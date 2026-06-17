package ui

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"termtalk/internal/client"
	"termtalk/internal/db"
	"termtalk/internal/protocol"
)

// Init triggers the initial listening command for background events.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.ListenForEvents(),
		textinput.Blink,
		presenceTick(),
	)
}

// ListenForEvents blocks waiting for a background event, then returns it.
func (m Model) ListenForEvents() tea.Cmd {
	return func() tea.Msg {
		return <-m.Client.Events()
	}
}

// SetStatus displays a status notification at the bottom of the screen.
func (m *Model) SetStatus(msg string, duration time.Duration) {
	m.StatusMessage = msg
	m.StatusExpiry = time.Now().Add(duration).Unix()
}

// RefreshContacts reloads the list of contacts from the database.
func (m *Model) RefreshContacts() {
	if m.Client == nil {
		return
	}
	contacts, err := m.Client.ListContacts()
	if err == nil {
		m.Contacts = contacts
		if m.SelectedIdx == -1 && len(contacts) > 0 {
			m.SelectedIdx = 0
			m.ReloadMessages()
		}
	}
	m.RefreshUnreadCounts()
}

// RefreshUnreadCounts queries unread counts for all contacts.
func (m *Model) RefreshUnreadCounts() {
	if m.Client == nil {
		return
	}
	for _, c := range m.Contacts {
		count, err := m.Client.GetUnreadCount(c.UUID)
		if err == nil {
			m.UnreadCounts[c.UUID] = count
		}
	}
}

// sendReadReceipts marks messages from the current contact as read and sends a read_ack.
func (m *Model) sendReadReceipts() {
	if m.Client == nil || m.SelectedIdx < 0 || m.SelectedIdx >= len(m.Contacts) || m.LocalUser == nil {
		return
	}
	contact := m.Contacts[m.SelectedIdx]
	var unreadIDs []string
	for _, msg := range m.ChatHistory {
		if msg.Sender == contact.UUID && msg.Status != string(db.StatusRead) {
			unreadIDs = append(unreadIDs, msg.ID)
		}
	}
	if len(unreadIDs) == 0 {
		return
	}
	// Mark locally as read
	if err := m.Client.MarkMessagesRead(unreadIDs); err != nil {
		log.Printf("ui: failed to mark messages as read: %v", err)
	}
	// Send read_ack to the sender via relay (fire-and-forget)
	_ = m.Client.SendReadAck(contact.UUID, unreadIDs)
	// Refresh unread counts
	m.UnreadCounts[contact.UUID] = 0
}

// PresenceTickMsg triggers periodic online presence refresh.
type PresenceTickMsg struct{}

func presenceTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return PresenceTickMsg{}
	})
}

// ReloadMessages fetches message logs for the active conversation.
func (m *Model) ReloadMessages() {
	if m.Client == nil || m.SelectedIdx < 0 || m.SelectedIdx >= len(m.Contacts) || m.LocalUser == nil {
		return
	}

	contact := m.Contacts[m.SelectedIdx]
	history, err := m.Client.GetChatHistory(contact.UUID)
	if err == nil {
		m.ChatHistory = history

		// Format chat transcript
		var builder strings.Builder
		width := m.Viewport.Width
		if width < 1 {
			width = 40
		}
		msgStyle := lipgloss.NewStyle().Width(width)

		for _, msg := range history {
			timestamp := msg.Timestamp.Format("15:04:05")
			statusStr := ""
			// Encryption indicator
			encIcon := ""
			if msg.Encrypted {
				encIcon = " 🔒"
			}
			var line string
			if msg.Sender == m.LocalUser.UUID {
				switch msg.Status {
				case string(db.StatusQueued), string(db.StatusStored):
					statusStr = " [Queued]"
				case string(db.StatusSynced):
					statusStr = " [✓]"
				case string(db.StatusAck), string(db.StatusRead):
					statusStr = " [✓✓]"
				}
				line = fmt.Sprintf("[%s] You: %s%s%s", timestamp, msg.Content, statusStr, encIcon)
			} else {
				line = fmt.Sprintf("[%s] %s: %s%s", timestamp, contact.Username, msg.Content, encIcon)
			}
			builder.WriteString(msgStyle.Render(line) + "\n")
		}

		m.Viewport.SetContent(builder.String())
		m.Viewport.GotoBottom()
	}
}

// Update processes Bubble Tea incoming events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Clear expired status messages
	if m.StatusMessage != "" && time.Now().Unix() > m.StatusExpiry {
		m.StatusMessage = ""
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.TerminalWidth = msg.Width
		m.TerminalHeight = msg.Height

		// Resize viewport and inputs based on layout sizing
		sidebarWidth := 25
		m.Viewport.Width = msg.Width - sidebarWidth - 1 // Leave room for chatBoxStyle padding

		overhead := 9
		if m.LocalUser != nil {
			overhead += 2
		}
		m.Viewport.Height = msg.Height - overhead
		if m.Viewport.Height < 1 {
			m.Viewport.Height = 1
		}
		m.MsgInput.Width = m.Viewport.Width
		m.PathInput.Width = msg.Width - 6

		m.ReloadMessages()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		}

		// Handle keys based on application state
		switch m.State {
		case StateRegister:
			var cmd tea.Cmd
			m.UsernameInput, cmd = m.UsernameInput.Update(msg)
			cmds = append(cmds, cmd)

			if msg.Type == tea.KeyEnter {
				username := strings.TrimSpace(m.UsernameInput.Value())
				if username != "" {
					profile, err := m.Client.Register(username)
					if err == nil {
						m.LocalUser = profile
						_ = m.Client.Start(context.Background())
						m.State = StateDashboard
						m.Focus = FocusSidebar
						m.RefreshContacts()
						m.MsgInput.Focus()
					}
				}
			}

		case StateDashboard:
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlQ:
				return m, tea.Quit

			case tea.KeyTab:
				// Toggle focus between sidebar and chat panes
				if m.Focus == FocusSidebar {
					m.Focus = FocusChat
					m.MsgInput.Focus()
				} else {
					m.Focus = FocusSidebar
					m.MsgInput.Blur()
				}

			case tea.KeyCtrlP:
				m.State = StateProfile

			case tea.KeyCtrlE:
				if m.SelectedIdx >= 0 {
					m.State = StateExport
					m.PathInput.Placeholder = "Enter path to save sync file (e.g. sync.json)..."
					m.PathInput.SetValue("")
					m.PathInput.Focus()
				} else {
					m.SetStatus("No contact selected to export messages for.", 3*time.Second)
				}

			case tea.KeyCtrlO:
				m.State = StateImport
				m.PathInput.Placeholder = "Enter path to import sync file from (e.g. sync.json)..."
				m.PathInput.SetValue("")
				m.PathInput.Focus()

			case tea.KeyCtrlN:
				m.State = StateAddContact
				m.AddContactInput.SetValue("")
				m.AddContactInput.Focus()

			case tea.KeyCtrlV:
				if m.Focus == FocusSidebar && m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
					m.State = StateVerify
				}

			case tea.KeyCtrlF:
				m.State = StateSearch
				m.SearchInput.SetValue("")
				m.SearchInput.Focus()
				m.SearchResults = nil
				m.SearchSelectedIdx = 0

			case tea.KeyCtrlD, tea.KeyDelete:
				if m.Focus == FocusSidebar && m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
					contact := m.Contacts[m.SelectedIdx]
					m.ConfirmAction = "delete_contact"
					m.ConfirmTarget = contact.UUID
					m.SetStatus(fmt.Sprintf("Delete @%s? (y/n)", contact.Username), 30*time.Second)
				}

			case tea.KeyCtrlL:
				m.State = StateUserList
				m.UserList = nil
				m.UserListSelected = 0
				if err := m.Client.ListUsers(); err != nil {
					m.SetStatus(fmt.Sprintf("Failed: %v", err), 3*time.Second)
				}

			case tea.KeyCtrlX:
				if m.Focus == FocusChat && m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) && m.LocalUser != nil {
					// Find the last message sent by the local user
					var lastSentID string
					for i := len(m.ChatHistory) - 1; i >= 0; i-- {
						if m.ChatHistory[i].Sender == m.LocalUser.UUID {
							lastSentID = m.ChatHistory[i].ID
							break
						}
					}
					if lastSentID != "" {
						m.ConfirmAction = "delete_message"
						m.ConfirmTarget = lastSentID
						m.SetStatus("Delete last message? (y: for me, e: for everyone, n: cancel)", 30*time.Second)
					}
				}

			case tea.KeyUp:
				if m.Focus == FocusSidebar {
					if m.SelectedIdx > 0 {
						m.SelectedIdx--
						m.ReloadMessages()
						m.sendReadReceipts()
					}
				}
				// When FocusChat, viewport scroll is handled below

			case tea.KeyDown:
				if m.Focus == FocusSidebar {
					if m.SelectedIdx < len(m.Contacts)-1 {
						m.SelectedIdx++
						m.ReloadMessages()
						m.sendReadReceipts()
					}
				}
				// When FocusChat, viewport scroll is handled below

			case tea.KeyEnter:
				if m.Focus == FocusSidebar {
					// Enter in sidebar selects the contact and switches to chat
					if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
						m.Focus = FocusChat
						m.MsgInput.Focus()
						m.ReloadMessages()
						m.sendReadReceipts()
					}
				} else {
					// Enter in chat sends a message
					text := strings.TrimSpace(m.MsgInput.Value())
					if text != "" && m.SelectedIdx >= 0 {
						contact := m.Contacts[m.SelectedIdx]
						_ = m.Client.SendMessage(contact.UUID, text)
						m.MsgInput.SetValue("")
						m.ReloadMessages()
					}
				}

			default:
				// Handle confirmation dialog responses
				if m.ConfirmAction != "" {
					switch m.ConfirmAction {
					case "delete_contact":
						if msg.String() == "y" || msg.String() == "Y" {
							if err := m.Client.DeleteContact(m.ConfirmTarget); err != nil {
								m.SetStatus(fmt.Sprintf("Failed to delete: %v", err), 3*time.Second)
							} else {
								m.SetStatus("Contact deleted.", 3*time.Second)
								m.RefreshContacts()
								if m.SelectedIdx >= len(m.Contacts) {
									m.SelectedIdx = len(m.Contacts) - 1
								}
								if m.SelectedIdx >= 0 {
									m.ReloadMessages()
								} else {
									m.ChatHistory = nil
									m.Viewport.SetContent("Select a contact to start messaging.")
								}
							}
						} else {
							m.SetStatus("Cancelled.", 2*time.Second)
						}
					case "delete_message":
						if msg.String() == "y" || msg.String() == "Y" {
							// Delete for me only
							if err := m.Client.DeleteMessagesLocal([]string{m.ConfirmTarget}); err != nil {
								m.SetStatus(fmt.Sprintf("Failed: %v", err), 3*time.Second)
							} else {
								m.SetStatus("Message deleted (for you).", 3*time.Second)
								m.ReloadMessages()
							}
						} else if msg.String() == "e" || msg.String() == "E" {
							// Delete for everyone
							contact := m.Contacts[m.SelectedIdx]
							if err := m.Client.DeleteMessagesForEveryone(contact.UUID, []string{m.ConfirmTarget}); err != nil {
								m.SetStatus(fmt.Sprintf("Failed: %v", err), 3*time.Second)
							} else {
								m.SetStatus("Message deleted for everyone.", 3*time.Second)
								m.ReloadMessages()
							}
						} else {
							m.SetStatus("Cancelled.", 2*time.Second)
						}
					default:
						m.SetStatus("Cancelled.", 2*time.Second)
					}
					m.ConfirmAction = ""
					m.ConfirmTarget = ""
					return m, nil
				}

				// '?' opens help overlay (only when not typing in chat)
				if m.Focus == FocusSidebar && msg.String() == "?" {
					m.State = StateHelp
					return m, nil
				}

				if m.Focus == FocusChat {
					// Forward input keypresses to active chat input
					var cmd tea.Cmd
					m.MsgInput, cmd = m.MsgInput.Update(msg)
					cmds = append(cmds, cmd)
				}
			}

		case StateProfile:
			if msg.Type == tea.KeyEsc {
				m.State = StateDashboard
				if m.Focus == FocusChat {
					m.MsgInput.Focus()
				}
				return m, nil
			}

		case StateExport:
			if msg.Type == tea.KeyEsc {
				m.State = StateDashboard
				m.MsgInput.Focus()
				return m, nil
			}

			var cmd tea.Cmd
			m.PathInput, cmd = m.PathInput.Update(msg)
			cmds = append(cmds, cmd)

			if msg.Type == tea.KeyEnter {
				path := strings.TrimSpace(m.PathInput.Value())
				if path != "" && m.SelectedIdx >= 0 {
					contact := m.Contacts[m.SelectedIdx]
					err := m.Client.ExportSync(contact.UUID, path)
					if err != nil {
						m.SetStatus(fmt.Sprintf("Export failed: %v", err), 4*time.Second)
					} else {
						m.SetStatus(fmt.Sprintf("Messages exported to %s", path), 4*time.Second)
					}
					m.State = StateDashboard
					m.ReloadMessages()
					m.MsgInput.Focus()
				}
			}

		case StateImport:
			if msg.Type == tea.KeyEsc {
				m.State = StateDashboard
				m.MsgInput.Focus()
				return m, nil
			}

			var cmd tea.Cmd
			m.PathInput, cmd = m.PathInput.Update(msg)
			cmds = append(cmds, cmd)

			if msg.Type == tea.KeyEnter {
				path := strings.TrimSpace(m.PathInput.Value())
				if path != "" {
					file, err := m.Client.ImportSync(path)
					if err != nil {
						m.SetStatus(fmt.Sprintf("Import failed: %v", err), 4*time.Second)
					} else {
						m.SetStatus(fmt.Sprintf("Imported %d messages from %s", len(file.Messages), file.SenderUsername), 4*time.Second)
						m.RefreshContacts()
					}
					m.State = StateDashboard
					m.MsgInput.Focus()
				}
			}

		case StateAddContact:
			if msg.Type == tea.KeyEsc {
				m.State = StateDashboard
				m.MsgInput.Focus()
				return m, nil
			}

			var cmd tea.Cmd
			m.AddContactInput, cmd = m.AddContactInput.Update(msg)
			cmds = append(cmds, cmd)

			if msg.Type == tea.KeyEnter {
				val := strings.TrimSpace(m.AddContactInput.Value())
				parts := strings.Split(val, ":")
				if len(parts) == 2 {
					err := m.Client.AddContact(parts[0], parts[1])
					if err != nil {
						m.SetStatus(fmt.Sprintf("Failed to add contact: %v", err), 3*time.Second)
					} else {
						m.SetStatus(fmt.Sprintf("Added contact %s", parts[0]), 3*time.Second)
						m.RefreshContacts()
					}
					m.State = StateDashboard
					m.MsgInput.Focus()
				} else {
					m.SetStatus("Format must be username:uuid", 3*time.Second)
				}
			}

		case StateSearch:
			switch msg.Type {
			case tea.KeyEsc:
				m.State = StateDashboard
				if m.Focus == FocusChat {
					m.MsgInput.Focus()
				}
				return m, nil

			case tea.KeyUp:
				if m.SearchSelectedIdx > 0 {
					m.SearchSelectedIdx--
				}

			case tea.KeyDown:
				if m.SearchSelectedIdx < len(m.SearchResults)-1 {
					m.SearchSelectedIdx++
				}

			case tea.KeyEnter:
				if len(m.SearchResults) > 0 && m.SearchSelectedIdx >= 0 && m.SearchSelectedIdx < len(m.SearchResults) {
					// Select and add the highlighted result
					result := m.SearchResults[m.SearchSelectedIdx]
					err := m.Client.AddContact(result.Username, result.UUID)
					if err != nil {
						m.SetStatus(fmt.Sprintf("Failed to add contact: %v", err), 3*time.Second)
					} else {
						m.SetStatus(fmt.Sprintf("Added contact %s", result.Username), 3*time.Second)
						m.RefreshContacts()
					}
					m.State = StateDashboard
					if m.Focus == FocusChat {
						m.MsgInput.Focus()
					}
				} else {
					// No results yet — send the search query
					query := strings.TrimSpace(m.SearchInput.Value())
					if err := m.Client.SearchUsers(query); err != nil {
						m.SetStatus(fmt.Sprintf("Search failed: %v", err), 3*time.Second)
					}
				}

			default:
				var cmd tea.Cmd
				m.SearchInput, cmd = m.SearchInput.Update(msg)
				cmds = append(cmds, cmd)

				// Live search: send query to relay on every keystroke
				query := strings.TrimSpace(m.SearchInput.Value())
				if query != "" {
					_ = m.Client.SearchUsers(query)
				}
			}

		case StateHelp:
			if msg.Type == tea.KeyEsc || msg.String() == "?" {
				m.State = StateDashboard
				if m.Focus == FocusChat {
					m.MsgInput.Focus()
				}
				return m, nil
			}

		case StateVerify:
			switch msg.Type {
			case tea.KeyEsc:
				m.State = StateDashboard
				if m.Focus == FocusChat {
					m.MsgInput.Focus()
				}
				return m, nil
			default:
				if msg.String() == "v" || msg.String() == "V" {
					if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
						contact := m.Contacts[m.SelectedIdx]
						_ = m.Client.SetContactVerified(contact.UUID, true)
						m.SetStatus("Contact verified ✓", 3*time.Second)
						m.RefreshContacts()
					}
				} else if msg.String() == "u" || msg.String() == "U" {
					if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
						contact := m.Contacts[m.SelectedIdx]
						_ = m.Client.SetContactVerified(contact.UUID, false)
						m.SetStatus("Contact marked unverified", 3*time.Second)
						m.RefreshContacts()
					}
				}
			}

		case StateUserList:
			switch msg.Type {
			case tea.KeyEsc:
				m.State = StateDashboard
				if m.Focus == FocusChat {
					m.MsgInput.Focus()
				}
				return m, nil
			case tea.KeyUp:
				if m.UserListSelected > 0 {
					m.UserListSelected--
				}
			case tea.KeyDown:
				if m.UserListSelected < len(m.UserList)-1 {
					m.UserListSelected++
				}
			case tea.KeyEnter:
				if len(m.UserList) > 0 && m.UserListSelected >= 0 && m.UserListSelected < len(m.UserList) {
					user := m.UserList[m.UserListSelected]
					if m.LocalUser != nil && user.UUID == m.LocalUser.UUID {
						m.SetStatus("That's you!", 2*time.Second)
					} else {
						err := m.Client.AddContact(user.Username, user.UUID)
						if err != nil {
							m.SetStatus(fmt.Sprintf("Failed: %v", err), 3*time.Second)
						} else {
							m.SetStatus(fmt.Sprintf("Added @%s", user.Username), 3*time.Second)
							m.RefreshContacts()
						}
						m.State = StateDashboard
						if m.Focus == FocusChat {
							m.MsgInput.Focus()
						}
					}
				}
			}
		}
	case client.PeerDiscoveredEvent:
		m.RefreshContacts()
		// Try to connect directly to peer over TCP if discovered
		if msg.Contact != nil {
			go func() {
				_ = m.Client.ConnectToPeer(context.Background(), msg.Contact)
			}()
		}
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())

	case client.MessageReceivedEvent:
		m.ReloadMessages()
		// If the message is in the currently open chat, send read receipt
		if m.State == StateDashboard && m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
			if msg.Message != nil && msg.Message.Sender == m.Contacts[m.SelectedIdx].UUID {
				m.sendReadReceipts()
			}
		}
		m.RefreshUnreadCounts()
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())

	case client.SearchResultEvent:
		// Convert protocol.UserInfo to local SearchResult
		results := make([]SearchResult, len(msg.Users))
		for i, u := range msg.Users {
			results[i] = SearchResult{UUID: u.UUID, Username: u.Username, Online: u.Online}
		}
		m.SearchResults = results
		m.SearchSelectedIdx = 0
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())

	case client.OnlineListEvent:
		// Update online presence map
		newMap := make(map[string]protocol.UserInfo, len(msg.Users))
		for _, u := range msg.Users {
			newMap[u.UUID] = u
		}
		m.OnlineUsers = newMap
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())

	case client.UserListEvent:
		m.UserList = msg.Users
		m.UserListSelected = 0
		cmds = append(cmds, m.ListenForEvents())

	case client.ReadAckEvent:
		// Incoming read receipt — update local message statuses
		if err := m.Client.MarkMessagesRead(msg.MessageIDs); err != nil {
			log.Printf("ui: failed to process read_ack: %v", err)
		}
		m.ReloadMessages()
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())

	case PresenceTickMsg:
		// Periodically refresh online presence
		if m.Client != nil {
			_ = m.Client.GetOnlineUsers()
		}
		cmds = append(cmds, presenceTick())
	}

	// Update viewport scroll position based on focus mode
	var vpCmd tea.Cmd
	shouldUpdateViewport := true
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Only suppress viewport scroll when in sidebar focus (arrows navigate contacts)
		if m.State == StateDashboard && m.Focus == FocusSidebar && (keyMsg.Type == tea.KeyUp || keyMsg.Type == tea.KeyDown) {
			shouldUpdateViewport = false
		}
	}

	if shouldUpdateViewport {
		m.Viewport, vpCmd = m.Viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

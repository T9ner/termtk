package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"termtalk/internal/db"
)

// Init triggers the initial listening command for background events.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.ListenForEvents(),
		textinput.Blink,
	)
}

// ListenForEvents blocks waiting for a background event, then returns it.
func (m Model) ListenForEvents() tea.Cmd {
	return func() tea.Msg {
		return <-m.EventChan
	}
}

// SetStatus displays a status notification at the bottom of the screen.
func (m *Model) SetStatus(msg string, duration time.Duration) {
	m.StatusMessage = msg
	m.StatusExpiry = time.Now().Add(duration).Unix()
}

// RefreshContacts reloads the list of contacts from the database.
func (m *Model) RefreshContacts() {
	if m.DB == nil {
		return
	}
	contacts, err := m.DB.ListContacts()
	if err == nil {
		m.Contacts = contacts
		if m.SelectedIdx == -1 && len(contacts) > 0 {
			m.SelectedIdx = 0
			m.ReloadMessages()
		}
	}
}

// ReloadMessages fetches message logs for the active conversation.
func (m *Model) ReloadMessages() {
	if m.DB == nil || m.SelectedIdx < 0 || m.SelectedIdx >= len(m.Contacts) || m.LocalUser == nil {
		return
	}

	contact := m.Contacts[m.SelectedIdx]
	history, err := m.DB.GetChatHistory(m.LocalUser.UUID, contact.UUID)
	if err == nil {
		m.ChatHistory = history

		// Format chat transcript
		var builder strings.Builder
		for _, msg := range history {
			timestamp := msg.Timestamp.Format("15:04:05")
			statusStr := ""
			if msg.Sender == m.LocalUser.UUID {
				switch msg.Status {
				case "draft":
					statusStr = " [Draft]"
				case "queued":
					statusStr = " [Queued]"
				case "synced":
					statusStr = " [✓]"
				}
				builder.WriteString(fmt.Sprintf("[%s] You: %s%s\n", timestamp, msg.Content, statusStr))
			} else {
				builder.WriteString(fmt.Sprintf("[%s] %s: %s\n", timestamp, contact.Username, msg.Content))
			}
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
		m.Viewport.Width = msg.Width - sidebarWidth - 4
		m.Viewport.Height = msg.Height - 6 // Padding for header, input and helper footer
		m.MsgInput.Width = msg.Width - sidebarWidth - 6
		m.PathInput.Width = msg.Width - 6

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
					userUUID := uuid.New().String()
					profile := &db.Profile{
						UUID:     userUUID,
						Username: username,
					}
					_ = m.DB.SaveProfile(profile)
					m.LocalUser = profile

					// Start network engines with profile info
					m.SyncManager.UpdateCredentials(userUUID, username)
					_ = m.SyncManager.Start()

					m.Discovery.UpdateCredentials(userUUID, username)
					_ = m.Discovery.Start()

					m.State = StateDashboard
					m.RefreshContacts()
					m.MsgInput.Focus()
				}
			}

		case StateDashboard:
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlQ:
				return m, tea.Quit

			case tea.KeyCtrlE:
				if m.SelectedIdx >= 0 {
					m.State = StateExport
					m.PathInput.Placeholder = "Enter path to save sync file (e.g. sync.json)..."
					m.PathInput.SetValue("")
					m.PathInput.Focus()
				} else {
					m.SetStatus("No contact selected to export messages for.", 3*time.Second)
				}

			case tea.KeyCtrlI:
				m.State = StateImport
				m.PathInput.Placeholder = "Enter path to import sync file from (e.g. sync.json)..."
				m.PathInput.SetValue("")
				m.PathInput.Focus()

			case tea.KeyCtrlN:
				m.State = StateAddContact
				m.AddContactInput.SetValue("")
				m.AddContactInput.Focus()

			case tea.KeyUp:
				if m.SelectedIdx > 0 {
					m.SelectedIdx--
					m.ReloadMessages()
				}

			case tea.KeyDown:
				if m.SelectedIdx < len(m.Contacts)-1 {
					m.SelectedIdx++
					m.ReloadMessages()
				}

			case tea.KeyEnter:
				text := strings.TrimSpace(m.MsgInput.Value())
				if text != "" && m.SelectedIdx >= 0 {
					contact := m.Contacts[m.SelectedIdx]
					_ = m.SyncManager.SendMessage(contact.UUID, text)
					m.MsgInput.SetValue("")
					m.ReloadMessages()
				}

			default:
				// Forward input keypresses to active chat input
				var cmd tea.Cmd
				m.MsgInput, cmd = m.MsgInput.Update(msg)
				cmds = append(cmds, cmd)
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
					err := m.DB.ExportSyncFile(contact.UUID, m.LocalUser, path)
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
					file, err := m.DB.ImportSyncFile(path)
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
					c := &db.Contact{
						Username: parts[0],
						UUID:     parts[1],
						IP:       "offline",
						Port:     0,
						LastSeen: time.Now(),
					}
					_ = m.DB.UpsertContact(c)
					m.SetStatus(fmt.Sprintf("Added contact %s", c.Username), 3*time.Second)
					m.RefreshContacts()
					m.State = StateDashboard
					m.MsgInput.Focus()
				} else {
					m.SetStatus("Format must be username:uuid", 3*time.Second)
				}
			}
		}

	case PeerDiscoveredMsg:
		m.RefreshContacts()
		// Try to connect directly to peer over TCP if discovered
		if msg.Contact != nil {
			go func() {
				_ = m.SyncManager.ConnectToPeer(msg.Contact)
			}()
		}
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())

	case MessageReceivedMsg:
		m.ReloadMessages()
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())
	}

	// Update viewport scroll position
	var vpCmd tea.Cmd
	m.Viewport, vpCmd = m.Viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

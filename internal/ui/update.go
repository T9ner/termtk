package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"termtalk/internal/client"
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
			var line string
			if msg.Sender == m.LocalUser.UUID {
				switch msg.Status {
				case "draft":
					statusStr = " [Draft]"
				case "queued":
					statusStr = " [Queued]"
				case "synced":
					statusStr = " [✓]"
				}
				line = fmt.Sprintf("[%s] You: %s%s", timestamp, msg.Content, statusStr)
			} else {
				line = fmt.Sprintf("[%s] %s: %s", timestamp, contact.Username, msg.Content)
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
						_ = m.Client.Start()
						m.State = StateDashboard
						m.RefreshContacts()
						m.MsgInput.Focus()
					}
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
					_ = m.Client.SendMessage(contact.UUID, text)
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
		}

	case client.PeerDiscoveredEvent:
		m.RefreshContacts()
		// Try to connect directly to peer over TCP if discovered
		if msg.Contact != nil {
			go func() {
				_ = m.Client.ConnectToPeer(msg.Contact)
			}()
		}
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())

	case client.MessageReceivedEvent:
		m.ReloadMessages()
		// Re-trigger listener
		cmds = append(cmds, m.ListenForEvents())
	}

	// Update viewport scroll position (ignoring Up/Down arrow keys on Dashboard to avoid selection conflict)
	var vpCmd tea.Cmd
	shouldUpdateViewport := true
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.State == StateDashboard && (keyMsg.Type == tea.KeyUp || keyMsg.Type == tea.KeyDown) {
			shouldUpdateViewport = false
		}
	}

	if shouldUpdateViewport {
		m.Viewport, vpCmd = m.Viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

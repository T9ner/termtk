package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#5F5FD7") // Sleek Indigo
	accentColor    = lipgloss.Color("#00FF87") // Neon Mint Green
	grayColor      = lipgloss.Color("#8A8A8A")
	errorColor     = lipgloss.Color("#D70000")
	darkGrayColor  = lipgloss.Color("#262626")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF")).
			Background(primaryColor).
			Padding(0, 1).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#3A3A3A")).
			PaddingRight(2).
			Width(25)

	chatBoxStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Background(darkGrayColor)

	normalContactStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF"))

	onlineBadge = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	offlineBadge = lipgloss.NewStyle().
			Foreground(grayColor)

	footerStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true).
			MarginTop(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAF00")).
			Bold(true)
)

// View renders the TUI screen based on the current state.
func (m Model) View() string {
	switch m.State {
	case StateRegister:
		return m.viewRegister()
	case StateDashboard:
		return m.viewDashboard()
	case StateExport:
		return m.viewExport()
	case StateImport:
		return m.viewImport()
	case StateAddContact:
		return m.viewAddContact()
	default:
		return "Unknown app state."
	}
}

func (m Model) viewRegister() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Welcome to TermTalk "))
	sb.WriteString("\n\n")
	sb.WriteString("TermTalk is a cross-platform, decentralized, offline-first messenger.\n")
	sb.WriteString("To begin, please choose a username. Your unique UUID will be auto-generated.\n\n")
	sb.WriteString(m.UsernameInput.View())
	sb.WriteString("\n\n(Press Enter to confirm and launch)")
	return sb.String()
}

func (m Model) viewDashboard() string {
	// 1. Header
	var headerStr string
	if m.LocalUser != nil {
		headerStr = titleStyle.Render(fmt.Sprintf(" TermTalk | User: %s (%s) ", m.LocalUser.Username, m.LocalUser.UUID[:8]))
	} else {
		headerStr = titleStyle.Render(" TermTalk ")
	}

	// 2. Sidebar (Contacts list)
	var sidebarBuilder strings.Builder
	sidebarBuilder.WriteString(headerStyle.Render("CONTACTS") + "\n\n")
	if len(m.Contacts) == 0 {
		sidebarBuilder.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("No contacts yet.\nUse Ctrl+N to add."))
	} else {
		for i, c := range m.Contacts {
			online := m.SyncManager != nil && m.SyncManager.IsPeerOnline(c.UUID)
			badge := offlineBadge.Render("[OFF]")
			if online {
				badge = onlineBadge.Render("[ON ]")
			}

			contactName := c.Username
			if len(contactName) > 12 {
				contactName = contactName[:9] + "..."
			}

			line := fmt.Sprintf("%s %s", badge, contactName)
			if i == m.SelectedIdx {
				sidebarBuilder.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				sidebarBuilder.WriteString(normalContactStyle.Render(line) + "\n")
			}
		}
	}
	sidebarView := sidebarStyle.Render(sidebarBuilder.String())

	// 3. Chat Pane (Messages history + Input field)
	var chatBuilder strings.Builder
	if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
		contact := m.Contacts[m.SelectedIdx]
		onlineStatus := offlineBadge.Render("offline")
		if m.SyncManager.IsPeerOnline(contact.UUID) {
			onlineStatus = onlineBadge.Render("online")
		}
		chatBuilder.WriteString(headerStyle.Render(fmt.Sprintf("Chatting with %s (%s)", contact.Username, onlineStatus)) + "\n\n")
		chatBuilder.WriteString(m.Viewport.View() + "\n\n")
		chatBuilder.WriteString(m.MsgInput.View())
	} else {
		chatBuilder.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("Welcome! Import a sync file (Ctrl+I) or add a friend (Ctrl+N) to start chatting."))
	}
	chatView := chatBoxStyle.Render(chatBuilder.String())

	// Combine Sidebar and Chat side-by-side
	bodyView := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, chatView)

	// 4. Status Bar & Footer
	var footerBuilder strings.Builder
	if m.StatusMessage != "" {
		footerBuilder.WriteString(statusStyle.Render(m.StatusMessage) + "\n")
	} else {
		footerBuilder.WriteString("\n")
	}
	footerBuilder.WriteString(footerStyle.Render("Ctrl+N: Add Peer | Ctrl+E: Export Sync | Ctrl+I: Import Sync | Ctrl+Q: Quit"))
	footerView := footerBuilder.String()

	return lipgloss.JoinVertical(lipgloss.Left, headerStr, bodyView, footerView)
}

func (m Model) viewExport() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Export Messages (Sneakernet) "))
	sb.WriteString("\n\n")
	if m.SelectedIdx >= 0 {
		contact := m.Contacts[m.SelectedIdx]
		sb.WriteString(fmt.Sprintf("Exporting pending offline messages to send to %s.\n", contact.Username))
	}
	sb.WriteString("Provide the destination JSON filepath:\n\n")
	sb.WriteString(m.PathInput.View())
	sb.WriteString("\n\n(Enter: Confirm | Esc: Cancel)")
	return sb.String()
}

func (m Model) viewImport() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Import Messages (Sneakernet) "))
	sb.WriteString("\n\n")
	sb.WriteString("Import messages from a sync JSON file received from a peer.\n")
	sb.WriteString("Provide the source JSON filepath:\n\n")
	sb.WriteString(m.PathInput.View())
	sb.WriteString("\n\n(Enter: Confirm | Esc: Cancel)")
	return sb.String()
}

func (m Model) viewAddContact() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Add Contact Manually "))
	sb.WriteString("\n\n")
	sb.WriteString("Ask your friend for their profile string, which looks like 'username:uuid'.\n")
	sb.WriteString("Paste it below:\n\n")
	sb.WriteString(m.AddContactInput.View())
	sb.WriteString("\n\n(Enter: Confirm | Esc: Cancel)")
	return sb.String()
}

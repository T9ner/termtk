package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"termtalk/internal/db"
)

var (
	// Colors
	primaryColor  = lipgloss.Color("#5F5FD7") // Sleek Indigo
	accentColor   = lipgloss.Color("#00FF87") // Neon Mint Green
	grayColor     = lipgloss.Color("#8A8A8A")
	errorColor    = lipgloss.Color("#D70000")
	darkGrayColor = lipgloss.Color("#262626")
	navyColor     = lipgloss.Color("#1A1A2E") // Dark navy for identity bar

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
			PaddingRight(1).
			Width(23) // 23 content + 1 padding + 1 border = 25 total

	chatBoxStyle = lipgloss.NewStyle().
			PaddingLeft(1)

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

	idBarStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Background(navyColor).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	// Active-pane accent styles
	sidebarActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, false, false).
				BorderForeground(accentColor).
				PaddingRight(1).
				Width(23)

	sidebarHeaderActiveStyle = lipgloss.NewStyle().
					Foreground(accentColor).
					Bold(true)

	chatHeaderActiveStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true).
				Underline(true)
)

// View renders the TUI screen based on the current state.
func (m Model) View() string {
	switch m.State {
	case StateRegister:
		return m.viewRegister()
	case StateDashboard:
		return m.viewDashboard()
	case StateProfile:
		return m.viewProfile()
	case StateExport:
		return m.viewExport()
	case StateImport:
		return m.viewImport()
	case StateAddContact:
		return m.viewAddContact()
	case StateSearch:
		return m.viewSearch()
	case StateHelp:
		return m.viewHelp()
	case StateVerify:
		return m.viewVerify()
	case StateUserList:
		return m.viewUserList()
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

func (m Model) viewProfile() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Your TermTalk Profile "))
	sb.WriteString("\n\n")

	if m.LocalUser != nil {
		boxWidth := 50
		border := strings.Repeat("═", boxWidth-2)
		divider := strings.Repeat("─", boxWidth-2)

		profileStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF"))

		accentTextStyle := lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

		handle := fmt.Sprintf("@%s", m.LocalUser.Username)
		shareID := fmt.Sprintf("%s:%s", m.LocalUser.Username, m.LocalUser.UUID)

		sb.WriteString(profileStyle.Render("╔"+border+"╗") + "\n")
		sb.WriteString(profileStyle.Render("║") + centerPad("Your TermTalk Profile", boxWidth-2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("╠"+divider+"╣") + "\n")
		sb.WriteString(profileStyle.Render("║") + padRight("", boxWidth-2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("║") + padRight(fmt.Sprintf("  Handle:    %s", handle), boxWidth-2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("║") + padRight(fmt.Sprintf("  Username:  %s", m.LocalUser.Username), boxWidth-2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("║") + padRight(fmt.Sprintf("  UUID:      %s", m.LocalUser.UUID), boxWidth-2) + profileStyle.Render("║") + "\n")
		fingerprint := m.LocalUser.Fingerprint()
		if fingerprint != "" {
			sb.WriteString(profileStyle.Render("║") + padRight(fmt.Sprintf("  Key:       %s", fingerprint), boxWidth-2) + profileStyle.Render("║") + "\n")
		}
		sb.WriteString(profileStyle.Render("║") + padRight("", boxWidth-2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("║") + padRight("  Share ID (give this to peers):", boxWidth-2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("║") + "  " + accentTextStyle.Render(shareID) + padRight("", boxWidth-2-2-len(shareID)) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("║") + padRight("", boxWidth-2) + profileStyle.Render("║") + "\n")
		findMeLine := fmt.Sprintf("  Find me on TermTalk: %s", handle)
		sb.WriteString(profileStyle.Render("║") + "  " + accentTextStyle.Render(fmt.Sprintf("Find me on TermTalk: %s", handle)) + padRight("", boxWidth-2-2-len(findMeLine)+2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("║") + padRight("", boxWidth-2) + profileStyle.Render("║") + "\n")
		sb.WriteString(profileStyle.Render("╚"+border+"╝") + "\n")
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("No profile loaded.") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(footerStyle.Render("Esc: Back to Dashboard"))
	return sb.String()
}

// centerPad centers text within the given width, padding with spaces.
func centerPad(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	left := (width - len(text)) / 2
	right := width - len(text) - left
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
}

// padRight pads text to the given width with trailing spaces.
func padRight(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}

func (m Model) viewDashboard() string {
	// 1. Header
	var headerStr string
	var identityBar string
	if m.LocalUser != nil {
		headerStr = titleStyle.Render(fmt.Sprintf(" TermTalk | User: %s ", m.LocalUser.Username))
		handle := fmt.Sprintf("@%s", m.LocalUser.Username)
		identityBar = idBarStyle.Render(fmt.Sprintf("%s  |  Ctrl+P: Profile  |  Ctrl+F: Find Users", handle))
	} else {
		headerStr = titleStyle.Render(" TermTalk ")
	}

	// 2. Sidebar (Contacts list)
	var sidebarBuilder strings.Builder

	// Sidebar header with contact count and focus indicator
	contactCount := len(m.Contacts)
	sidebarHeaderText := fmt.Sprintf("CONTACTS (%d)", contactCount)
	if m.Focus == FocusSidebar {
		sidebarBuilder.WriteString(sidebarHeaderActiveStyle.Render(sidebarHeaderText) + "\n")
		sidebarBuilder.WriteString(sidebarHeaderActiveStyle.Render(strings.Repeat("─", 15)) + "\n")
	} else {
		sidebarBuilder.WriteString(headerStyle.Render(sidebarHeaderText) + "\n")
		sidebarBuilder.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render(strings.Repeat("─", 15)) + "\n")
	}

	if len(m.Contacts) == 0 {
		sidebarBuilder.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("No contacts yet.\nUse Ctrl+N to add."))
	} else {
		maxVisible := m.Viewport.Height + 2
		if maxVisible < 1 {
			maxVisible = 1
		}
		start := 0
		end := len(m.Contacts)
		if len(m.Contacts) > maxVisible {
			start = m.SelectedIdx - maxVisible/2
			if start < 0 {
				start = 0
			}
			end = start + maxVisible
			if end > len(m.Contacts) {
				end = len(m.Contacts)
				start = end - maxVisible
				if start < 0 {
					start = 0
				}
			}
		}

		if start > 0 {
			sidebarBuilder.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("  ↑ more\n"))
		} else {
			sidebarBuilder.WriteString("\n")
		}

		for i := start; i < end; i++ {
			c := m.Contacts[i]
			// Check both local P2P connection and relay online presence
			_, relayOnline := m.OnlineUsers[c.UUID]
			online := (m.Client != nil && m.Client.IsPeerOnline(c.UUID)) || relayOnline
			badge := offlineBadge.Render("[OFF]")
			if online {
				badge = onlineBadge.Render("[ON ]")
			}

			contactName := c.Username
			if len(contactName) > 12 {
				contactName = contactName[:9] + "..."
			}

			// Show unread count badge
			unread := m.UnreadCounts[c.UUID]
			unreadStr := ""
			if unread > 0 {
				unreadStr = fmt.Sprintf(" (%d)", unread)
				contactName = lipgloss.NewStyle().Bold(true).Render(contactName)
			}

			verifiedBadge := ""
			if c.Verified {
				verifiedBadge = " ✓"
			}

			line := fmt.Sprintf("%s %s%s%s", badge, contactName, unreadStr, verifiedBadge)
			if i == m.SelectedIdx {
				sidebarBuilder.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				sidebarBuilder.WriteString(normalContactStyle.Render(line) + "\n")
			}
		}

		if end < len(m.Contacts) {
			sidebarBuilder.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("  ↓ more\n"))
		} else {
			sidebarBuilder.WriteString("\n")
		}
	}

	// Apply active-pane sidebar style
	var sidebarView string
	if m.Focus == FocusSidebar {
		sidebarView = sidebarActiveStyle.Render(sidebarBuilder.String())
	} else {
		sidebarView = sidebarStyle.Render(sidebarBuilder.String())
	}

	// 3. Chat Pane (Messages history + Input field)
	var chatBuilder strings.Builder
	if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Contacts) {
		contact := m.Contacts[m.SelectedIdx]
		_, relayOnline := m.OnlineUsers[contact.UUID]
		statusText := "offline"
		if m.Client.IsPeerOnline(contact.UUID) || relayOnline {
			statusText = "online"
		}
		// ICE connection status indicator
		connIndicator := ""
		if m.Client.IsPeerOnline(contact.UUID) || relayOnline {
			if m.ICEConnected[contact.UUID] {
				connIndicator = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(" [direct]")
			} else {
				connIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF00")).Render(" [relay]")
			}
		}
		chatHeaderText := fmt.Sprintf("Chatting with %s (%s)", contact.Username, statusText) + connIndicator
		if m.Focus == FocusChat {
			chatBuilder.WriteString(chatHeaderActiveStyle.Render(chatHeaderText) + "\n\n")
		} else {
			chatBuilder.WriteString(headerStyle.Render(chatHeaderText) + "\n\n")
		}

		// Show empty conversation state or messages
		if len(m.ChatHistory) == 0 {
			emptyMsg := lipgloss.NewStyle().Foreground(grayColor).Italic(true)
			chatBuilder.WriteString(emptyMsg.Render(fmt.Sprintf("No messages with %s yet.", contact.Username)) + "\n")
			chatBuilder.WriteString(emptyMsg.Render("Type a message below to start the conversation.") + "\n\n")
		} else {
			chatBuilder.WriteString(m.Viewport.View() + "\n\n")
		}
		chatBuilder.WriteString(m.MsgInput.View())
	} else {
		// No contact selected — welcome empty state
		welcomeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF"))
		accentText := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		grayText := lipgloss.NewStyle().Foreground(grayColor)

		chatBuilder.WriteString("\n")
		chatBuilder.WriteString(welcomeStyle.Render("  ┌─────────────────────────────────┐") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │                                 │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │   ") + accentText.Render("👋 Welcome to TermTalk!") + welcomeStyle.Render("      │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │                                 │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │   Get started:                  │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │   ") + grayText.Render("• Ctrl+N to add a peer") + welcomeStyle.Render("    │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │   ") + grayText.Render("• Ctrl+I to import sync") + welcomeStyle.Render("   │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │   ") + grayText.Render("• Ctrl+P to view profile") + welcomeStyle.Render("  │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  │                                 │") + "\n")
		chatBuilder.WriteString(welcomeStyle.Render("  └─────────────────────────────────┘") + "\n")
	}
	chatView := chatBoxStyle.Render(chatBuilder.String())

	// Combine Sidebar and Chat side-by-side
	bodyView := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, chatView)

	// 4. Status Bar & Context-Aware Footer
	var footerBuilder strings.Builder
	if m.StatusMessage != "" {
		footerBuilder.WriteString(statusStyle.Render(m.StatusMessage) + "\n")
	} else {
		footerBuilder.WriteString("\n")
	}
	footerBuilder.WriteString(footerStyle.Render(m.dashboardFooter()))
	footerView := footerBuilder.String()

	if identityBar != "" {
		return lipgloss.JoinVertical(lipgloss.Left, headerStr, identityBar, bodyView, footerView)
	}
	return lipgloss.JoinVertical(lipgloss.Left, headerStr, bodyView, footerView)
}

// dashboardFooter returns context-aware shortcut hints based on focus mode.
func (m Model) dashboardFooter() string {
	if m.ConfirmAction != "" {
		return ""
	}
	if m.Focus == FocusSidebar {
		if len(m.Contacts) == 0 {
			return "Ctrl+F: Find  ·  Ctrl+N: Add  ·  ?: Help"
		}
		return "↑↓: Navigate  ·  Enter: Chat  ·  ?: Help"
	}
	return "Enter: Send  ·  Tab: Contacts  ·  ?: Help"
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
	sb.WriteString("\n\n")
	sb.WriteString(footerStyle.Render("Enter: Confirm | Esc: Cancel"))
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
	sb.WriteString("\n\n")
	sb.WriteString(footerStyle.Render("Enter: Confirm | Esc: Cancel"))
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
	sb.WriteString("\n\n")
	sb.WriteString(footerStyle.Render("Enter: Confirm | Esc: Cancel"))
	return sb.String()
}

func (m Model) viewSearch() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Find Users on Relay "))
	sb.WriteString("\n\n")
	sb.WriteString(" Search: " + m.SearchInput.View())
	sb.WriteString("\n\n")

	// Separate online and offline results, online first
	var online, offline []SearchResult
	for _, r := range m.SearchResults {
		if r.Online {
			online = append(online, r)
		} else {
			offline = append(offline, r)
		}
	}
	sorted := append(online, offline...)

	if len(sorted) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(grayColor).Italic(true).Render(" No results.") + "\n")
	} else {
		sb.WriteString(" Results:\n")
		sb.WriteString(" " + strings.Repeat("─", 34) + "\n")

		for i, r := range sorted {
			var badge string
			if r.Online {
				badge = onlineBadge.Render("[ON ]")
			} else {
				badge = offlineBadge.Render("[OFF]")
			}

			username := r.Username
			if len(username) > 14 {
				username = username[:11] + "..."
			}

			line := fmt.Sprintf("  %s %-14s", badge, username)
			if i == 0 {
				line += "  Press Enter to add"
			}

			// Find original index for selection highlighting
			origIdx := -1
			for j, orig := range m.SearchResults {
				if orig.UUID == r.UUID {
					origIdx = j
					break
				}
			}

			if origIdx == m.SearchSelectedIdx {
				sb.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				sb.WriteString(normalContactStyle.Render(line) + "\n")
			}
		}
		sb.WriteString(" " + strings.Repeat("─", 34) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(footerStyle.Render("↑↓: Navigate | Enter: Add Contact | Esc: Cancel"))
	return sb.String()
}

func (m Model) viewHelp() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" TermTalk Help "))
	sb.WriteString("\n\n")

	sectionStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Bold(true).Width(14)
	descStyle := lipgloss.NewStyle().Foreground(grayColor)
	divStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3A3A3A"))

	writeSection := func(title string) {
		sb.WriteString(sectionStyle.Render("  "+title) + "\n")
		sb.WriteString(divStyle.Render("  "+strings.Repeat("─", 38)) + "\n")
	}

	writeKey := func(key, desc string) {
		sb.WriteString("  " + keyStyle.Render(key) + descStyle.Render(desc) + "\n")
	}

	writeSection("Navigation")
	writeKey("Tab", "Switch sidebar ↔ chat")
	writeKey("↑ ↓", "Navigate contacts / scroll")
	writeKey("Enter", "Open chat / send message")
	writeKey("Esc", "Quit TermTalk")
	sb.WriteString("\n")

	writeSection("Actions")
	writeKey("Ctrl+F", "Find users on relay")
	writeKey("Ctrl+N", "Add contact manually")
	writeKey("Del/Ctrl+D", "Delete selected contact")
	writeKey("Ctrl+V", "Verify contact")
	writeKey("Ctrl+P", "View your profile")
	writeKey("Ctrl+X", "Delete last sent message")
	writeKey("Ctrl+L", "User directory")
	sb.WriteString("\n")

	writeSection("Sync")
	writeKey("Ctrl+E", "Export sync file")
	writeKey("Ctrl+O", "Import sync file")
	sb.WriteString("\n")

	sb.WriteString(footerStyle.Render("  Press Esc or ? to close"))
	return sb.String()
}

func (m Model) viewVerify() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" Verify Contact "))
	sb.WriteString("\n\n")

	if m.SelectedIdx < 0 || m.SelectedIdx >= len(m.Contacts) {
		sb.WriteString("  No contact selected.\n")
		sb.WriteString("\n")
		sb.WriteString(footerStyle.Render("Esc: Back"))
		return sb.String()
	}

	contact := m.Contacts[m.SelectedIdx]

	if len(contact.PublicKey) == 0 || m.LocalUser == nil || len(m.LocalUser.PublicKey) == 0 {
		sb.WriteString("  Cannot verify — no cryptographic keys available.\n")
		sb.WriteString("  Both you and your contact need v0.4.0+ for verification.\n")
		sb.WriteString("\n")
		sb.WriteString(footerStyle.Render("Esc: Back"))
		return sb.String()
	}

	code := db.VerificationCode(m.LocalUser.PublicKey, contact.PublicKey)

	accentTextStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	grayText := lipgloss.NewStyle().Foreground(grayColor)

	sb.WriteString(fmt.Sprintf("  Contact: @%s\n\n", contact.Username))
	sb.WriteString("  Your verification code:\n\n")
	sb.WriteString("    " + accentTextStyle.Render(fmt.Sprintf("  %s  %s  ", code[:3], code[3:])) + "\n\n")
	sb.WriteString(grayText.Render("  Ask your contact to open this screen (Ctrl+V).\n"))
	sb.WriteString(grayText.Render("  If their code matches, press 'v' to mark as verified.\n"))
	sb.WriteString(grayText.Render("  If it doesn't match, press 'u' to mark as unverified.\n"))

	if contact.Verified {
		sb.WriteString("\n")
		sb.WriteString("  " + accentTextStyle.Render("✓ This contact is verified") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(footerStyle.Render("v: Verify  ·  u: Unverify  ·  Esc: Back"))
	return sb.String()
}

func formatTimeAgo(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
}

func (m Model) viewUserList() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(" User Directory "))
	sb.WriteString("\n\n")

	if len(m.UserList) == 0 {
		sb.WriteString("  Loading users...\n")
		sb.WriteString("\n")
		sb.WriteString(footerStyle.Render("Esc: Back"))
		return sb.String()
	}

	accentTextStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	grayText := lipgloss.NewStyle().Foreground(grayColor)

	sb.WriteString(accentTextStyle.Render(fmt.Sprintf("  %d registered users", len(m.UserList))))
	sb.WriteString("\n")
	sb.WriteString("  " + strings.Repeat("─", 40) + "\n")

	for i, u := range m.UserList {
		status := offlineBadge.Render("○")
		if u.Online {
			status = onlineBadge.Render("●")
		}

		lastSeen := ""
		if u.LastSeen != "" {
			if t, err := time.Parse(time.RFC3339, u.LastSeen); err == nil {
				lastSeen = grayText.Render(" · " + formatTimeAgo(t))
			}
		}

		line := fmt.Sprintf("  %s @%s%s", status, u.Username, lastSeen)

		if i == m.UserListSelected {
			sb.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			sb.WriteString(normalContactStyle.Render(line) + "\n")
		}
	}

	sb.WriteString("  " + strings.Repeat("─", 40) + "\n")
	sb.WriteString("\n")
	sb.WriteString(footerStyle.Render("↑↓: Navigate | Enter: Add as contact | Esc: Back"))
	return sb.String()
}

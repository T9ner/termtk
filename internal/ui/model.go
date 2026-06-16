package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"termtalk/internal/client"
	"termtalk/internal/db"
)

type AppState int

const (
	StateRegister AppState = iota
	StateDashboard
	StateExport
	StateImport
	StateAddContact
	StateProfile
)

// FocusMode indicates which pane has keyboard focus on the dashboard.
type FocusMode int

const (
	FocusSidebar FocusMode = iota
	FocusChat
)

// Model represents the state of the TermTalk TUI application.
type Model struct {
	State     AppState
	Client    *client.Client
	LocalUser *db.Profile

	// Focus tracks which dashboard pane owns keyboard input.
	Focus FocusMode

	// UI Component states
	UsernameInput   textinput.Model // For profile creation
	MsgInput        textinput.Model // For chat messages
	PathInput       textinput.Model // For import/export file paths
	AddContactInput textinput.Model // For manually adding contact by username/UUID
	Viewport        viewport.Model  // Scrollable chat message area

	// Chat Selection states
	Contacts       []db.Contact
	SelectedIdx    int
	ChatHistory    []db.Message
	TerminalWidth  int
	TerminalHeight int
	StatusMessage  string
	StatusExpiry   int64 // Unix time for when status should clear
}

// NewModel initializes the Bubble Tea model with the Client reference.
func NewModel(c *client.Client) *Model {
	m := &Model{
		Client:      c,
		SelectedIdx: -1,
		Focus:       FocusSidebar,
	}

	// Initialize inputs
	m.UsernameInput = textinput.New()
	m.UsernameInput.Placeholder = "Enter your username..."
	m.UsernameInput.Focus()

	m.MsgInput = textinput.New()
	m.MsgInput.Placeholder = "Type a message and press Enter..."

	m.PathInput = textinput.New()

	m.AddContactInput = textinput.New()
	m.AddContactInput.Placeholder = "username:uuid..."

	m.Viewport = viewport.New(0, 0)
	m.Viewport.SetContent("Select a contact to start messaging.")

	return m
}

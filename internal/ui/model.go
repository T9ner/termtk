package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"termtalk/internal/client"
	"termtalk/internal/db"
	"termtalk/internal/protocol"
)

type AppState int

const (
	StateRegister AppState = iota
	StateDashboard
	StateExport
	StateImport
	StateAddContact
	StateProfile
	StateSearch
	StateHelp
	StateVerify
	StateUserList
)

// FocusMode indicates which pane has keyboard focus on the dashboard.
type FocusMode int

const (
	FocusSidebar FocusMode = iota
	FocusChat
)

// SearchResult holds user info from relay search responses.
// Temporary local type until protocol package adds UserInfo.
type SearchResult struct {
	UUID     string
	Username string
	Online   bool
}

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

	// Confirmation dialog state
	ConfirmAction string
	ConfirmTarget string

	// Search state
	SearchResults     []SearchResult
	SearchInput       textinput.Model
	SearchSelectedIdx int

	// Unread and presence state
	UnreadCounts map[string]int               // contactUUID → unread count
	OnlineUsers  map[string]protocol.UserInfo // uuid → presence info

	// User directory state
	UserList         []protocol.UserInfo
	UserListSelected int

	// Reaction state
	EmojiPickerOpen bool
	EmojiPickerIdx  int
	SelectedMsgIdx  int
	ChatReactions   map[string][]db.Reaction
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

	m.SearchInput = textinput.New()
	m.SearchInput.Placeholder = "Search by username..."

	m.Viewport = viewport.New(0, 0)
	m.Viewport.SetContent("Select a contact to start messaging.")

	m.UnreadCounts = make(map[string]int)
	m.OnlineUsers = make(map[string]protocol.UserInfo)
	m.ChatReactions = make(map[string][]db.Reaction)
	m.SelectedMsgIdx = -1

	return m
}

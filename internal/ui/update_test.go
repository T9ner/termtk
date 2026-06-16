package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termtalk/internal/client"
	"termtalk/internal/db"
)

// newTestModel creates a Model backed by a real Client with a temp DB.
// The Client is not started (no networking), which is fine for UI unit tests.
func newTestModel(t *testing.T) (*Model, func()) {
	t.Helper()
	dir := t.TempDir()
	c, err := client.New(dir+"/test.db", 0)
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	m := NewModel(c)
	return m, func() { c.Stop() }
}

// ────────────────────────────────────────────────────────────────────
// Test 1: NewModel initial state
// ────────────────────────────────────────────────────────────────────

func TestNewModel_InitialState(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	if m.State != StateRegister {
		t.Errorf("expected initial state StateRegister (%d), got %d", StateRegister, m.State)
	}
	if m.SelectedIdx != -1 {
		t.Errorf("expected SelectedIdx -1, got %d", m.SelectedIdx)
	}
	if m.Client == nil {
		t.Error("expected Client to be non-nil")
	}
	if m.LocalUser != nil {
		t.Error("expected LocalUser to be nil before registration")
	}
	if len(m.Contacts) != 0 {
		t.Errorf("expected 0 contacts, got %d", len(m.Contacts))
	}
	if len(m.ChatHistory) != 0 {
		t.Errorf("expected 0 chat history, got %d", len(m.ChatHistory))
	}
	if m.Focus != FocusSidebar {
		t.Errorf("expected initial Focus FocusSidebar (%d), got %d", FocusSidebar, m.Focus)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 2: Ctrl+C triggers tea.Quit from any state
// ────────────────────────────────────────────────────────────────────

func TestUpdate_CtrlC_Quits(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected a non-nil cmd from Ctrl+C")
	}
	// tea.Quit returns a special quit message
	result := cmd()
	if _, ok := result.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", result)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 3: WindowSizeMsg updates terminal dimensions
// ────────────────────────────────────────────────────────────────────

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.TerminalWidth != 120 {
		t.Errorf("expected TerminalWidth 120, got %d", um.TerminalWidth)
	}
	if um.TerminalHeight != 40 {
		t.Errorf("expected TerminalHeight 40, got %d", um.TerminalHeight)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 4: Dashboard — Esc quits
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_EscQuits(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected a non-nil cmd from Esc on Dashboard")
	}
	result := cmd()
	if _, ok := result.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", result)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 5: Dashboard — Ctrl+N transitions to StateAddContact
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_CtrlN_AddContact(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	msg := tea.KeyMsg{Type: tea.KeyCtrlN}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateAddContact {
		t.Errorf("expected StateAddContact (%d), got %d", StateAddContact, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 6: Dashboard — Up/Down navigation with contacts (sidebar focus)
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_ContactNavigation(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	// Register a profile so ReloadMessages doesn't bail out
	m.State = StateDashboard
	m.Focus = FocusSidebar
	m.LocalUser = &db.Profile{UUID: "me", Username: "me"}
	m.Contacts = []db.Contact{
		{UUID: "a", Username: "Alice"},
		{UUID: "b", Username: "Bob"},
		{UUID: "c", Username: "Charlie"},
	}
	m.SelectedIdx = 0

	// Press Down
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	um := updated.(Model)
	if um.SelectedIdx != 1 {
		t.Errorf("expected SelectedIdx 1 after Down, got %d", um.SelectedIdx)
	}

	// Press Down again
	updated, _ = um.Update(msg)
	um = updated.(Model)
	if um.SelectedIdx != 2 {
		t.Errorf("expected SelectedIdx 2 after second Down, got %d", um.SelectedIdx)
	}

	// Press Down at bottom — should not overflow
	updated, _ = um.Update(msg)
	um = updated.(Model)
	if um.SelectedIdx != 2 {
		t.Errorf("expected SelectedIdx 2 (clamped), got %d", um.SelectedIdx)
	}

	// Press Up
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = um.Update(upMsg)
	um = updated.(Model)
	if um.SelectedIdx != 1 {
		t.Errorf("expected SelectedIdx 1 after Up, got %d", um.SelectedIdx)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 7: Dashboard — Ctrl+I transitions to StateImport
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_CtrlO_Import(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	msg := tea.KeyMsg{Type: tea.KeyCtrlO}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateImport {
		t.Errorf("expected StateImport (%d), got %d", StateImport, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 8: StateAddContact — Esc returns to Dashboard
// ────────────────────────────────────────────────────────────────────

func TestUpdate_AddContact_EscReturnsToDashboard(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateAddContact
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateDashboard {
		t.Errorf("expected StateDashboard (%d), got %d", StateDashboard, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 9: StateExport — Esc returns to Dashboard
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Export_EscReturnsToDashboard(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateExport
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateDashboard {
		t.Errorf("expected StateDashboard (%d), got %d", StateDashboard, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 10: StateImport — Esc returns to Dashboard
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Import_EscReturnsToDashboard(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateImport
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateDashboard {
		t.Errorf("expected StateDashboard (%d), got %d", StateDashboard, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 11: MessageReceivedEvent triggers event processing
// ────────────────────────────────────────────────────────────────────

func TestUpdate_MessageReceivedEvent(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	m.LocalUser = &db.Profile{UUID: "me", Username: "me"}

	msg := client.MessageReceivedEvent{
		Message: &db.Message{
			ID:        "test-msg-id",
			Sender:    "peer-uuid",
			Recipient: "me",
			Content:   "Hello!",
		},
	}

	// Should not panic and should return a cmd (ListenForEvents re-trigger)
	updated, cmd := m.Update(msg)
	_ = updated.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd after MessageReceivedEvent (re-trigger listener)")
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 12: PeerDiscoveredEvent triggers event processing
// ────────────────────────────────────────────────────────────────────

func TestUpdate_PeerDiscoveredEvent(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard

	msg := client.PeerDiscoveredEvent{
		Contact: &db.Contact{
			UUID:     "discovered-uuid",
			Username: "newpeer",
			IP:       "192.168.1.5",
			Port:     55555,
		},
	}

	// Should not panic and should return a cmd (ListenForEvents re-trigger)
	updated, cmd := m.Update(msg)
	_ = updated.(Model)

	if cmd == nil {
		t.Error("expected non-nil cmd after PeerDiscoveredEvent (re-trigger listener)")
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 13: View() renders without panic for each state
// ────────────────────────────────────────────────────────────────────

func TestView_AllStates_NoPanic(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	states := []AppState{
		StateRegister,
		StateDashboard,
		StateProfile,
		StateExport,
		StateImport,
		StateAddContact,
		StateSearch,
	}

	for _, state := range states {
		m.State = state
		// For export/addcontact views that access m.Contacts[m.SelectedIdx]
		if state == StateExport {
			m.Contacts = []db.Contact{{UUID: "a", Username: "Alice"}}
			m.SelectedIdx = 0
		}
		// For profile view
		if state == StateProfile {
			m.LocalUser = &db.Profile{UUID: "test-uuid-123", Username: "testuser"}
		}
		output := m.View()
		if output == "" {
			t.Errorf("expected non-empty View output for state %d", state)
		}
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 14: Dashboard — Ctrl+E with no contact sets status
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_CtrlE_NoContact_SetsStatus(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	m.SelectedIdx = -1

	msg := tea.KeyMsg{Type: tea.KeyCtrlE}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.StatusMessage == "" {
		t.Error("expected a status message when Ctrl+E with no contact selected")
	}
	// Should remain on Dashboard
	if um.State != StateDashboard {
		t.Errorf("expected to stay on StateDashboard, got %d", um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 15: Dashboard — Ctrl+E with contact transitions to StateExport
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_CtrlE_WithContact_GoesToExport(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	m.Contacts = []db.Contact{{UUID: "a", Username: "Alice"}}
	m.SelectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyCtrlE}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateExport {
		t.Errorf("expected StateExport (%d), got %d", StateExport, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 16: Tab key toggles FocusMode
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_Tab_TogglesFocus(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	m.Focus = FocusSidebar

	// Tab should switch to FocusChat
	msg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.Focus != FocusChat {
		t.Errorf("expected FocusChat (%d) after Tab, got %d", FocusChat, um.Focus)
	}

	// Tab again should switch back to FocusSidebar
	updated, _ = um.Update(msg)
	um = updated.(Model)

	if um.Focus != FocusSidebar {
		t.Errorf("expected FocusSidebar (%d) after second Tab, got %d", FocusSidebar, um.Focus)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 17: Ctrl+P transitions to StateProfile
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_CtrlP_GoesToProfile(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard

	msg := tea.KeyMsg{Type: tea.KeyCtrlP}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateProfile {
		t.Errorf("expected StateProfile (%d), got %d", StateProfile, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 18: Esc from StateProfile returns to StateDashboard
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Profile_EscReturnsToDashboard(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateProfile

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateDashboard {
		t.Errorf("expected StateDashboard (%d), got %d", StateDashboard, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 19: Up/Down in FocusChat does NOT change SelectedIdx
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_FocusChat_UpDown_NoContactChange(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	m.Focus = FocusChat
	m.LocalUser = &db.Profile{UUID: "me", Username: "me"}
	m.Contacts = []db.Contact{
		{UUID: "a", Username: "Alice"},
		{UUID: "b", Username: "Bob"},
	}
	m.SelectedIdx = 0

	// Down in FocusChat should NOT change SelectedIdx
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.SelectedIdx != 0 {
		t.Errorf("expected SelectedIdx 0 (unchanged in FocusChat), got %d", um.SelectedIdx)
	}

	// Up in FocusChat should NOT change SelectedIdx
	um.SelectedIdx = 1
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = um.Update(msg)
	um = updated.(Model)

	if um.SelectedIdx != 1 {
		t.Errorf("expected SelectedIdx 1 (unchanged in FocusChat), got %d", um.SelectedIdx)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 20: Enter in FocusSidebar selects contact and switches to chat
// ────────────────────────────────────────────────────────────────────

func TestUpdate_Dashboard_FocusSidebar_Enter_SelectsAndSwitches(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	m.Focus = FocusSidebar
	m.LocalUser = &db.Profile{UUID: "me", Username: "me"}
	m.Contacts = []db.Contact{
		{UUID: "a", Username: "Alice"},
	}
	m.SelectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.Focus != FocusChat {
		t.Errorf("expected FocusChat after Enter in sidebar, got %d", um.Focus)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 21: Ctrl+F from Dashboard transitions to StateSearch
// ────────────────────────────────────────────────────────────────────

func TestCtrlFTransitionsToSearch(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateDashboard
	m.Focus = FocusSidebar

	msg := tea.KeyMsg{Type: tea.KeyCtrlF}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateSearch {
		t.Errorf("expected StateSearch (%d), got %d", StateSearch, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 22: Esc from StateSearch returns to Dashboard
// ────────────────────────────────────────────────────────────────────

func TestEscFromSearchReturnsToDashboard(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateSearch

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	updated, _ := m.Update(msg)
	um := updated.(Model)

	if um.State != StateDashboard {
		t.Errorf("expected StateDashboard (%d), got %d", StateDashboard, um.State)
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 23: viewSearch() renders without panic on empty results
// ────────────────────────────────────────────────────────────────────

func TestSearchViewRendersWithoutPanic(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateSearch
	m.SearchResults = nil

	output := m.View()
	if output == "" {
		t.Error("expected non-empty View output for StateSearch with no results")
	}
}

// ────────────────────────────────────────────────────────────────────
// Test 24: viewSearch() with results shows usernames and badges
// ────────────────────────────────────────────────────────────────────

func TestSearchViewRendersResults(t *testing.T) {
	m, cleanup := newTestModel(t)
	defer cleanup()

	m.State = StateSearch
	m.SearchResults = []SearchResult{
		{UUID: "uuid-1", Username: "alice", Online: true},
		{UUID: "uuid-2", Username: "chioma", Online: false},
		{UUID: "uuid-3", Username: "amara", Online: true},
	}
	m.SearchSelectedIdx = 0

	output := m.View()

	// Should contain usernames
	if !containsStr(output, "alice") {
		t.Error("expected output to contain 'alice'")
	}
	if !containsStr(output, "chioma") {
		t.Error("expected output to contain 'chioma'")
	}
	if !containsStr(output, "amara") {
		t.Error("expected output to contain 'amara'")
	}

	// Should contain the title
	if !containsStr(output, "Find Users on Relay") {
		t.Error("expected output to contain 'Find Users on Relay'")
	}
}

// containsStr is a test helper to check substring membership.
func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}

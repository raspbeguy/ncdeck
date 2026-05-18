// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

func managerFixture() labelManager {
	return newLabelManager([]api.Label{
		{ID: 1, Title: "bug", Color: "ff0000"},
		{ID: 2, Title: "feature", Color: "00ff00"},
	}, nil, lipgloss.Color("#0082c9"))
}

func keyPress(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestLabelManager_SpaceTogglesFilter(t *testing.T) {
	m := managerFixture()
	m.cursor = 0
	if a := m.Update(keyPress(" ")); a != lmgrActionNone {
		t.Errorf("space should be a no-side-effect toggle, got action=%d", a)
	}
	if !m.filter[1] {
		t.Errorf("filter should now include label 1, got %v", m.filter)
	}
	_ = m.Update(keyPress(" "))
	if m.filter[1] {
		t.Errorf("second space should clear the filter")
	}
}

func TestLabelManager_EscFromListClosesManager(t *testing.T) {
	m := managerFixture()
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEsc}); a != lmgrActionClose {
		t.Errorf("esc on list should close, got %d", a)
	}
}

func TestLabelManager_CreateFlowNameThenColor(t *testing.T) {
	m := managerFixture()
	// 'n' kicks off the create flow, name first.
	_ = m.Update(keyPress("n"))
	if m.mode != lmgrModeName || m.intent != lmgrIntentCreate {
		t.Fatalf("after 'n': mode=%d intent=%d, want lmgrModeName+lmgrIntentCreate", m.mode, m.intent)
	}
	m.nameIn.SetValue("urgent")
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); a != lmgrActionNone {
		t.Errorf("enter on name shouldn't fire yet for create flow, got %d", a)
	}
	if m.mode != lmgrModeColor {
		t.Fatalf("mode should advance to color, got %d", m.mode)
	}
	if m.pendingName != "urgent" {
		t.Errorf("pendingName: got %q, want %q", m.pendingName, "urgent")
	}
	// Enter on color (default preset selected) fires create.
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); a != lmgrActionCreate {
		t.Errorf("enter on color should fire create, got %d", a)
	}
	if m.pendingColor == "" {
		t.Errorf("pendingColor not set after color enter")
	}
}

func TestLabelManager_EditNameFiresUpdateImmediately(t *testing.T) {
	m := managerFixture()
	m.cursor = 0
	_ = m.Update(keyPress("e"))
	if m.mode != lmgrModeName || m.intent != lmgrIntentEditName {
		t.Fatalf("after 'e': mode=%d intent=%d", m.mode, m.intent)
	}
	m.nameIn.SetValue("defect")
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); a != lmgrActionUpdateName {
		t.Errorf("enter on edit-name should fire updateName, got %d", a)
	}
	if m.pendingName != "defect" {
		t.Errorf("pendingName: got %q, want %q", m.pendingName, "defect")
	}
}

func TestLabelManager_DeleteRequiresConfirm(t *testing.T) {
	m := managerFixture()
	m.cursor = 0
	_ = m.Update(keyPress("x"))
	if m.mode != lmgrModeConfirmDel {
		t.Fatalf("after 'x': mode=%d, want lmgrModeConfirmDel", m.mode)
	}
	if a := m.Update(keyPress("n")); a != lmgrActionNone {
		t.Errorf("n cancels: got %d, want lmgrActionNone", a)
	}
	if m.mode != lmgrModeList {
		t.Errorf("cancel should restore list mode, got %d", m.mode)
	}
	_ = m.Update(keyPress("x"))
	if a := m.Update(keyPress("y")); a != lmgrActionDelete {
		t.Errorf("y confirms: got %d, want lmgrActionDelete", a)
	}
}

func TestLabelManager_OnLabelCreatedAppendsAndAdvancesCursor(t *testing.T) {
	m := managerFixture()
	m.onLabelCreated(api.Label{ID: 99, Title: "urgent", Color: "ff0000"})
	if len(m.all) != 3 {
		t.Fatalf("len after create: got %d, want 3", len(m.all))
	}
	if m.cursor != 2 {
		t.Errorf("cursor should land on the new label (idx 2), got %d", m.cursor)
	}
}

func TestLabelManager_OnLabelDeletedDropsAndClampsCursor(t *testing.T) {
	m := managerFixture()
	m.cursor = 1
	m.filter[1] = true
	m.onLabelDeleted(2) // remove "feature"
	if len(m.all) != 1 {
		t.Errorf("len after delete: got %d, want 1", len(m.all))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should clamp, got %d", m.cursor)
	}
	m.onLabelDeleted(1) // remove last label, "bug"
	if m.filter[1] {
		t.Errorf("filter should drop the deleted label's id")
	}
	if m.cursor != 0 {
		t.Errorf("cursor on empty: got %d, want 0", m.cursor)
	}
}

func TestMatchesLabelFilter(t *testing.T) {
	filter := map[int]bool{1: true, 3: true}
	if !matchesLabelFilter(api.Card{}, nil) {
		t.Errorf("empty filter should always match")
	}
	if !matchesLabelFilter(api.Card{Labels: []api.Label{{ID: 1}}}, filter) {
		t.Errorf("card with matching label should pass")
	}
	if matchesLabelFilter(api.Card{Labels: []api.Label{{ID: 2}}}, filter) {
		t.Errorf("card with non-matching label should not pass")
	}
	if matchesLabelFilter(api.Card{}, filter) {
		t.Errorf("card with no labels should not pass an active filter")
	}
}

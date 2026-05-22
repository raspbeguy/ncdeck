// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"strings"
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

// Pinned: plain ⏎ must cancel the delete confirm, not trigger it. A reflexive
// enter-press shouldn't destroy a label.
func TestLabelManager_DeleteConfirmEnterCancels(t *testing.T) {
	m := managerFixture()
	m.cursor = 0
	_ = m.Update(keyPress("x"))
	if m.mode != lmgrModeConfirmDel {
		t.Fatalf("setup: mode should be confirmDel after 'x'")
	}
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); a != lmgrActionNone {
		t.Errorf("⏎ on delete confirm must NOT fire lmgrActionDelete; got %d", a)
	}
	if m.mode != lmgrModeList {
		t.Errorf("⏎ should cancel and return to list mode; got %d", m.mode)
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

func TestLabelManager_EditColorFiresUpdateImmediately(t *testing.T) {
	m := managerFixture()
	m.cursor = 0
	_ = m.Update(keyPress("c"))
	if m.mode != lmgrModeColor || m.intent != lmgrIntentEditColor {
		t.Fatalf("after 'c': mode=%d intent=%d, want lmgrModeColor+lmgrIntentEditColor", m.mode, m.intent)
	}
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); a != lmgrActionUpdateColor {
		t.Errorf("enter on edit-color should fire updateColor, got %d", a)
	}
	if m.pendingColor == "" {
		t.Errorf("pendingColor should be set after color enter")
	}
}

func TestLabelManager_EscInCreateColorReturnsToName(t *testing.T) {
	m := managerFixture()
	_ = m.Update(keyPress("n"))
	m.nameIn.SetValue("urgent")
	_ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != lmgrModeColor {
		t.Fatalf("setup: expected to advance to color mode, got %d", m.mode)
	}
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEsc}); a != lmgrActionNone {
		t.Errorf("esc in create-color should be lmgrActionNone, got %d", a)
	}
	if m.mode != lmgrModeName {
		t.Errorf("esc in create-color should pop back to name mode, got %d", m.mode)
	}
	if m.intent != lmgrIntentCreate {
		t.Errorf("intent should remain lmgrIntentCreate, got %d", m.intent)
	}
}

func TestLabelManager_OnLabelDeletedNonCursorKeepsCursor(t *testing.T) {
	m := managerFixture()
	m.cursor = 0
	m.onLabelDeleted(2) // delete "feature", cursor was on "bug" (id=1)
	if len(m.all) != 1 {
		t.Errorf("len after delete: got %d, want 1", len(m.all))
	}
	if m.cursor != 0 {
		t.Errorf("cursor on non-cursor delete should stay at 0, got %d", m.cursor)
	}
	if m.all[0].ID != 1 {
		t.Errorf("remaining label should be bug (id=1), got id=%d", m.all[0].ID)
	}
}

func TestLabelManager_CreateRoundTripPreservesPickerState(t *testing.T) {
	m := managerFixture()
	_ = m.Update(keyPress("n"))
	m.nameIn.SetValue("urgent")
	_ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m.picker.input.SetValue("ff7f50")
	_ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != lmgrModeName {
		t.Fatalf("setup: esc should return to name mode, got %d", m.mode)
	}
	_ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != lmgrModeColor {
		t.Fatalf("setup: enter on name should advance to color, got %d", m.mode)
	}
	if got := m.picker.input.Value(); got != "ff7f50" {
		t.Errorf("picker input should preserve typed value across name<->color round-trip; got %q, want %q", got, "ff7f50")
	}
}

// Pinned bug: previously left/right in colour mode always called movePreset,
// so the textinput's cursor-left / cursor-right never reached the input when
// it was focused.
func TestLabelManager_LeftRightWhenInputFocusedDoesNotMovePalette(t *testing.T) {
	m := managerFixture()
	m.cursor = 0
	_ = m.Update(keyPress("c"))
	_ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.picker.focusInput {
		t.Fatalf("setup: tab should focus the input")
	}
	before := m.picker.presetIdx
	_ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	_ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.picker.presetIdx != before {
		t.Errorf("left/right with input focused must not move palette cursor; got %d, want %d", m.picker.presetIdx, before)
	}
}

func TestLabelManager_ResetSubdialogClearsPendingColor(t *testing.T) {
	m := managerFixture()
	m.pendingName = "x"
	m.pendingColor = "abcdef"
	m.picker = newColorPicker("ff0000", lipgloss.Color("#0082c9"))
	m.mode = lmgrModeColor
	m.intent = lmgrIntentEditColor
	m.resetSubdialog()
	if m.pendingName != "" || m.pendingColor != "" {
		t.Errorf("resetSubdialog should clear both pendingName and pendingColor, got %q / %q", m.pendingName, m.pendingColor)
	}
	if m.picker.initialised() {
		t.Errorf("resetSubdialog should zero the picker so the next create flow gets a fresh one")
	}
}

// Pinned: edit-color on a label whose hex isn't in the preset palette must
// preserve the original colour when the user presses ⏎ on the auto-focused
// input, instead of silently replacing it with the default preset.
func TestLabelManager_EditColorOnNonPresetHexPreservesOriginal(t *testing.T) {
	m := newLabelManager(
		[]api.Label{{ID: 1, Title: "coral", Color: "ff7f50"}},
		nil,
		lipgloss.Color("#0082c9"),
	)
	_ = m.Update(keyPress("c"))
	if m.mode != lmgrModeColor || m.intent != lmgrIntentEditColor {
		t.Fatalf("after 'c': mode=%d intent=%d", m.mode, m.intent)
	}
	if !m.picker.focusInput {
		t.Errorf("non-preset hex should default to input focus so ⏎ keeps the typed value")
	}
	if a := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); a != lmgrActionUpdateColor {
		t.Fatalf("enter on edit-color should fire update, got %d", a)
	}
	if m.pendingColor != "ff7f50" {
		t.Errorf("⏎ on auto-focused input must keep the original hex; got %q want %q", m.pendingColor, "ff7f50")
	}
}

func TestLabelManager_FilterStatusPluralisation(t *testing.T) {
	m := managerFixture()
	m.filter[1] = true
	out := m.viewList()
	if !strings.Contains(out, "1 label.") {
		t.Errorf("singular: expected '1 label.' in viewList output, got: %q", out)
	}
	if strings.Contains(out, "1 labels") {
		t.Errorf("singular: viewList must not say '1 labels', got: %q", out)
	}
	m.filter[2] = true
	out = m.viewList()
	if !strings.Contains(out, "2 labels.") {
		t.Errorf("plural: expected '2 labels.' in viewList output, got: %q", out)
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

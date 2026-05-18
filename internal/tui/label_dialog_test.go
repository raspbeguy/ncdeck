// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

func boardLabelsFixture() []api.Label {
	return []api.Label{
		{ID: 1, Title: "bug", Color: "ff0000"},
		{ID: 2, Title: "feature", Color: "00ff00"},
		{ID: 3, Title: "question", Color: "0000ff"},
		{ID: 4, Title: "blocked", Color: "ffaa00"},
	}
}

func TestNewLabelDialog_MarksCardLabelsAssigned(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), []api.Label{{ID: 2}, {ID: 4}}, lipgloss.Color("#0082c9"))
	if !d.assigned[2] || !d.assigned[4] {
		t.Errorf("card labels not marked assigned: %v", d.assigned)
	}
	if d.assigned[1] || d.assigned[3] {
		t.Errorf("unrelated labels marked assigned: %v", d.assigned)
	}
}

func TestRefilter_NarrowsToMatchingPrefix(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("b")
	d.refilter()
	// "bug" and "blocked" both contain 'b'.
	if len(d.filtered) != 2 {
		t.Fatalf("filter 'b': got %d matches, want 2 (%v)", len(d.filtered), d.filtered)
	}
}

func TestRefilter_CaseInsensitive(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("BUG")
	d.refilter()
	if len(d.filtered) != 1 || d.all[d.filtered[0]].Title != "bug" {
		t.Errorf("case-insensitive filter failed: %v", d.filtered)
	}
}

func TestRefilter_ResetsCursorWhenItFallsOff(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.cursor = 3 // last entry
	d.input.SetValue("bug")
	d.refilter()
	// Now only one match; cursor must clamp.
	if d.cursor != 0 {
		t.Errorf("cursor not reset: got %d, want 0", d.cursor)
	}
}

func TestMoveCursor_WrapsBothDirections(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	// 4 entries, no filter.
	d.cursor = 0
	d.moveCursor(-1)
	if d.cursor != 3 {
		t.Errorf("wrap backward from 0: got %d, want 3", d.cursor)
	}
	d.moveCursor(+1)
	if d.cursor != 0 {
		t.Errorf("wrap forward from 3: got %d, want 0", d.cursor)
	}
}

func TestMoveCursor_EmptyFilteredStaysAtZero(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("zzz")
	d.refilter()
	d.cursor = 0
	d.moveCursor(+1)
	if d.cursor != 0 {
		t.Errorf("cursor moved with no matches: got %d", d.cursor)
	}
}

func TestToggleAssigned_FlipsAndReportsPriorState(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), []api.Label{{ID: 1}}, lipgloss.Color("#0082c9"))
	if was := d.toggleAssigned(1); !was {
		t.Errorf("first toggle: wasAssigned=%v, want true", was)
	}
	if d.assigned[1] {
		t.Errorf("assigned[1] should be false after toggle")
	}
	if was := d.toggleAssigned(1); was {
		t.Errorf("second toggle: wasAssigned=%v, want false", was)
	}
	if !d.assigned[1] {
		t.Errorf("assigned[1] should be true after second toggle")
	}
}

func TestCurrentAction_OnLabelRowReturnsToggle(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.cursor = 0
	action, label, name := d.currentAction()
	if action != labelActionToggle {
		t.Errorf("action: got %d, want labelActionToggle", action)
	}
	if label == nil || label.ID != 1 {
		t.Errorf("label: got %v, want id=1", label)
	}
	if name != "" {
		t.Errorf("name should be empty for toggle, got %q", name)
	}
}

func TestCurrentAction_OnCreateRowReturnsCreate(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("urgent")
	d.refilter()
	// "urgent" doesn't match any existing label so canCreate is true.
	// Cursor at len(filtered) lands on the synthetic create row.
	d.cursor = len(d.filtered)
	action, label, name := d.currentAction()
	if action != labelActionCreate {
		t.Errorf("action: got %d, want labelActionCreate", action)
	}
	if label != nil {
		t.Errorf("label should be nil for create, got %v", label)
	}
	if name != "urgent" {
		t.Errorf("name: got %q, want %q", name, "urgent")
	}
}

func TestCanCreate_FalseWhenExactMatchExists(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("bug") // exact title match
	if d.canCreate() {
		t.Errorf("canCreate should be false when an existing label matches exactly")
	}
	d.input.SetValue("BUG") // case-insensitive
	if d.canCreate() {
		t.Errorf("canCreate should be case-insensitive")
	}
}

func TestCanCreate_TrueOnPartialMatch(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	// "bu" is a prefix of "bug" but not an exact match; user might still
	// want to create a separate label called "bu".
	d.input.SetValue("bu")
	if !d.canCreate() {
		t.Errorf("canCreate should be true for partial-match queries")
	}
}

func TestMoveCursor_VisitsCreateRow(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("urgent")
	d.refilter()
	// canCreate -> totalEntries = 0 + 1 = 1; cursor wraps within the single
	// synthetic create row.
	d.moveCursor(+1)
	if d.cursor != 0 {
		t.Errorf("with only the create row, cursor stays at 0, got %d", d.cursor)
	}
}

func TestAdoptCreated_AppendsAssignsAndClearsFilter(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("urgent")
	d.refilter()
	d.adoptCreated(api.Label{ID: 99, Title: "urgent", Color: "888888"})
	if !d.assigned[99] {
		t.Errorf("assigned[99] should be true after adoptCreated")
	}
	if d.input.Value() != "" {
		t.Errorf("filter should be cleared, got %q", d.input.Value())
	}
	// Re-filtered list should now contain all 5 entries (4 fixture + 1 new).
	if len(d.filtered) != 5 {
		t.Errorf("filtered list: got %d entries, want 5", len(d.filtered))
	}
}

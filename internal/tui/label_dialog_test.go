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

func TestToggleSelected_FlipsAssignedAndReportsPriorState(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), []api.Label{{ID: 1}}, lipgloss.Color("#0082c9"))
	d.cursor = 0 // "bug" is currently assigned
	l, was := d.toggleSelected()
	if l == nil || l.ID != 1 || !was {
		t.Errorf("first toggle: label=%v wasAssigned=%v, want id=1 wasAssigned=true", l, was)
	}
	if d.assigned[1] {
		t.Errorf("assigned[1] should be false after toggle")
	}
	l, was = d.toggleSelected()
	if l == nil || l.ID != 1 || was {
		t.Errorf("second toggle: label=%v wasAssigned=%v, want id=1 wasAssigned=false", l, was)
	}
	if !d.assigned[1] {
		t.Errorf("assigned[1] should be true after second toggle")
	}
}

func TestToggleSelected_EmptyFilteredReturnsNil(t *testing.T) {
	d := newLabelDialog(boardLabelsFixture(), nil, lipgloss.Color("#0082c9"))
	d.input.SetValue("zzz")
	d.refilter()
	if l, _ := d.toggleSelected(); l != nil {
		t.Errorf("expected nil when no labels match, got %v", l)
	}
}

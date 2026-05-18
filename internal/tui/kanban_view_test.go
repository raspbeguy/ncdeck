// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	"github.com/raspbeguy/ncdeck/internal/api"
)

func TestSetStacks_SortsByOrder(t *testing.T) {
	k := newKanbanModel(1)
	k.setStacks([]api.Stack{
		{ID: 3, Title: "C", Order: 2},
		{ID: 1, Title: "A", Order: 0},
		{ID: 2, Title: "B", Order: 1},
	})
	if k.stacks[0].Title != "A" || k.stacks[1].Title != "B" || k.stacks[2].Title != "C" {
		t.Errorf("stacks order: got %v %v %v, want A B C",
			k.stacks[0].Title, k.stacks[1].Title, k.stacks[2].Title)
	}
}

func TestSetStacks_SortsCardsWithinStack(t *testing.T) {
	k := newKanbanModel(1)
	k.setStacks([]api.Stack{{
		ID: 1, Title: "A", Order: 0, Cards: []api.Card{
			{ID: 10, Title: "z", Order: 2},
			{ID: 11, Title: "y", Order: 0},
			{ID: 12, Title: "x", Order: 1},
		},
	}})
	cards := k.stacks[0].Cards
	if cards[0].Title != "y" || cards[1].Title != "x" || cards[2].Title != "z" {
		t.Errorf("card order: got %s %s %s, want y x z", cards[0].Title, cards[1].Title, cards[2].Title)
	}
}

func TestSetStacks_ClampsCursorWhenStackShrinks(t *testing.T) {
	k := newKanbanModel(1)
	k.stackIdx = 2
	k.cardIdx = 5
	k.setStacks([]api.Stack{
		{ID: 1, Order: 0, Cards: []api.Card{{ID: 1}}},
	})
	if k.stackIdx != 0 {
		t.Errorf("stackIdx not reset: got %d, want 0", k.stackIdx)
	}
	if k.cardIdx != 0 {
		t.Errorf("cardIdx not reset: got %d, want 0", k.cardIdx)
	}
}

func TestPickFocusedWindow_CursorFitsAtTop(t *testing.T) {
	heights := []int{4, 4, 4, 4, 4}
	newTop, start, end := pickFocusedWindow(0, 0, heights, 20)
	if newTop != 0 || start != 0 || end != 5 {
		t.Errorf("got newTop=%d start=%d end=%d, want 0 0 5", newTop, start, end)
	}
}

func TestPickFocusedWindow_CursorBelowWindowAdvancesTop(t *testing.T) {
	heights := []int{4, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	// 20 rows fits 5 cards. Cursor at idx 7 must push topIdx to 3.
	newTop, start, end := pickFocusedWindow(0, 7, heights, 20)
	if newTop != 3 {
		t.Errorf("newTop: got %d, want 3", newTop)
	}
	if start != 3 || end != 8 {
		t.Errorf("window: got [%d, %d), want [3, 8)", start, end)
	}
}

func TestPickFocusedWindow_CursorAboveWindowPullsTopBack(t *testing.T) {
	heights := []int{4, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	// Already scrolled (topIdx=5) but cursor moved up to 2.
	newTop, start, end := pickFocusedWindow(5, 2, heights, 20)
	if newTop != 2 {
		t.Errorf("newTop: got %d, want 2", newTop)
	}
	if start != 2 || end != 7 {
		t.Errorf("window: got [%d, %d), want [2, 7)", start, end)
	}
}

func TestPickFocusedWindow_SingleTallCardAlwaysVisible(t *testing.T) {
	// Each card is taller than avail; the focused card still shows.
	heights := []int{30, 30, 30}
	newTop, start, end := pickFocusedWindow(0, 1, heights, 10)
	if newTop != 1 || start != 1 || end != 2 {
		t.Errorf("got newTop=%d start=%d end=%d, want 1 1 2", newTop, start, end)
	}
}

func TestPickTopWindow(t *testing.T) {
	heights := []int{4, 4, 4, 4, 4}
	if got := pickTopWindow(heights, 20); got != 5 {
		t.Errorf("avail=20: got %d, want 5", got)
	}
	if got := pickTopWindow(heights, 7); got != 1 {
		t.Errorf("avail=7 (one card fits): got %d, want 1", got)
	}
	if got := pickTopWindow(heights, 0); got != 1 {
		t.Errorf("avail=0: got %d, want 1 (always show at least one)", got)
	}
	if got := pickTopWindow(nil, 20); got != 0 {
		t.Errorf("empty: got %d, want 0", got)
	}
}

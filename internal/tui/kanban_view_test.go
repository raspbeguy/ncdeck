// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "testing"

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

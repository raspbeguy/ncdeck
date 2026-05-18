// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/raspbeguy/ncdeck/internal/api"
)

// Wires a Model + kanbanModel against an httptest server so tea.Cmd factories
// can be exercised without a live Nextcloud.
func newKanbanWithStubServer(t *testing.T, h http.HandlerFunc) (*Model, *kanbanModel) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	m := &Model{ctx: context.Background(), client: api.New(srv.URL, "u", "p")}
	k := newKanbanModel(7)
	k.stacks = []api.Stack{{ID: 1, Title: "A", Cards: []api.Card{{ID: 10, Title: "a"}}}}
	m.kanban = k
	return m, k
}

func isRefreshMsg(msg tea.Msg) bool { _, ok := msg.(refreshMsg); return ok }
func isErrMsg(msg tea.Msg) bool     { _, ok := msg.(errMsg); return ok }

// --- setStacks --------------------------------------------------------------

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

// End-to-end check that setStacks + pickFocusedWindow together survive a
// stale topIdx (setStacks doesn't clear it; the render-time picker clamps).
func TestSetStacksThenPickFocusedWindow_SelfCorrectsOnShrink(t *testing.T) {
	k := newKanbanModel(1)
	k.stacks = []api.Stack{{ID: 1, Cards: []api.Card{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}}}
	k.cardIdx = 3
	k.topIdx = 3
	k.setStacks([]api.Stack{
		{ID: 1, Order: 0, Cards: []api.Card{{ID: 1}}},
	})
	if k.cardIdx != 0 {
		t.Errorf("cardIdx: got %d, want 0", k.cardIdx)
	}
	heights := []int{4}
	newTop, _, _ := pickFocusedWindow(k.topIdx, k.cardIdx, heights, 20)
	if newTop != 0 {
		t.Errorf("pickFocusedWindow self-correction: got newTop=%d, want 0", newTop)
	}
}

// --- Update routing ---------------------------------------------------------

func TestKanbanUpdate_QuestionMarkTogglesHelp(t *testing.T) {
	m := &Model{ctx: context.Background()}
	k := newKanbanModel(1)
	m.kanban = k
	_, _ = k.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}, m)
	if !k.showHelp {
		t.Fatalf("first ? should open help")
	}
	_, _ = k.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}, m)
	if k.showHelp {
		t.Errorf("second ? should close help")
	}
}

// esc while the help overlay is open must close the overlay rather than
// fire backMsg (which would dump the user out of the kanban entirely).
func TestKanbanUpdate_EscClosesHelpInsteadOfGoingBack(t *testing.T) {
	m := &Model{ctx: context.Background()}
	k := newKanbanModel(1)
	k.showHelp = true
	m.kanban = k
	_, cmd := k.Update(tea.KeyMsg{Type: tea.KeyEsc}, m)
	if k.showHelp {
		t.Errorf("esc should have closed help")
	}
	if cmd != nil {
		t.Errorf("esc-closes-help should not return a cmd (no backMsg)")
	}
}

func TestKanbanUpdate_JKMovesCursor(t *testing.T) {
	m := &Model{ctx: context.Background()}
	k := newKanbanModel(1)
	k.stacks = []api.Stack{{ID: 1, Cards: []api.Card{
		{ID: 1}, {ID: 2}, {ID: 3},
	}}}
	m.kanban = k
	_, _ = k.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, m)
	if k.cardIdx != 1 {
		t.Errorf("j: cardIdx=%d, want 1", k.cardIdx)
	}
	_, _ = k.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, m)
	if k.cardIdx != 0 {
		t.Errorf("k: cardIdx=%d, want 0", k.cardIdx)
	}
}

// --- reorderWithin ----------------------------------------------------------

// Two rapid J presses must each move the cursor + locally swap the cards
// (so a future press operates on the correct card), and the in-flight gate
// must prevent the *second* API call from racing against the first.
func TestReorderWithin_OptimisticAndGated(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := &Model{
		ctx:    context.Background(),
		client: api.New(srv.URL, "u", "p"),
	}
	k := newKanbanModel(7)
	k.stacks = []api.Stack{{ID: 1, Title: "A", Cards: []api.Card{
		{ID: 10, Title: "a"},
		{ID: 11, Title: "b"},
		{ID: 12, Title: "c"},
	}}}
	k.stackIdx = 0
	k.cardIdx = 0
	m.kanban = k

	cmd1 := k.reorderWithin(m, +1)
	if k.cardIdx != 1 {
		t.Fatalf("first press: cardIdx=%d, want 1 (optimistic)", k.cardIdx)
	}
	if k.stacks[0].Cards[0].ID != 11 || k.stacks[0].Cards[1].ID != 10 {
		t.Errorf("first press: cards not swapped locally: %+v", k.stacks[0].Cards)
	}
	if !k.reorderInFlight {
		t.Errorf("inFlight should be true after the first press")
	}

	// Second press while the first is still in flight must be a no-op.
	cmd2 := k.reorderWithin(m, +1)
	if cmd2 != nil {
		t.Errorf("second press during in-flight: expected nil cmd")
	}
	if k.cardIdx != 1 {
		t.Errorf("second press shouldn't move cursor: cardIdx=%d", k.cardIdx)
	}

	// Drain the first cmd so the test doesn't leak goroutines.
	if cmd1 == nil {
		t.Fatal("first press: expected a non-nil cmd")
	}
	msg := cmd1()
	if _, ok := msg.(reorderedMsg); !ok {
		t.Errorf("expected reorderedMsg, got %T", msg)
	}
	if hits.Load() != 1 {
		t.Errorf("expected exactly 1 API call, got %d", hits.Load())
	}
}

func TestReorderWithin_BoundsRejectsOutOfRange(t *testing.T) {
	k := newKanbanModel(1)
	k.stacks = []api.Stack{{ID: 1, Cards: []api.Card{{ID: 1}, {ID: 2}}}}
	k.cardIdx = 0
	m := &Model{ctx: context.Background(), client: api.New("http://x", "u", "p")}
	if cmd := k.reorderWithin(m, -1); cmd != nil {
		t.Errorf("expected nil cmd when target < 0")
	}
	if k.reorderInFlight {
		t.Errorf("inFlight set after bounds-rejection at target < 0")
	}
	k.cardIdx = 1
	if cmd := k.reorderWithin(m, +1); cmd != nil {
		t.Errorf("expected nil cmd when target >= len(Cards)")
	}
	if k.reorderInFlight {
		t.Errorf("inFlight set after bounds-rejection at target >= len(Cards)")
	}
}

// --- command factories ------------------------------------------------------

func TestDoArchive_ReturnsRefreshOnSuccess(t *testing.T) {
	m, k := newKanbanWithStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	cmd := k.doArchive(m, &k.stacks[0].Cards[0])
	if msg := cmd(); !isRefreshMsg(msg) {
		t.Errorf("got %T, want refreshMsg", msg)
	}
}

func TestDoArchive_ReturnsErrOnFailure(t *testing.T) {
	m, k := newKanbanWithStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	cmd := k.doArchive(m, &k.stacks[0].Cards[0])
	if msg := cmd(); !isErrMsg(msg) {
		t.Errorf("got %T, want errMsg", msg)
	}
}

func TestDoDelete_ReturnsRefreshOnSuccess(t *testing.T) {
	m, k := newKanbanWithStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	cmd := k.doDelete(m, &k.stacks[0].Cards[0])
	if msg := cmd(); !isRefreshMsg(msg) {
		t.Errorf("got %T, want refreshMsg", msg)
	}
}

func TestDoMove_ReturnsRefreshOnSuccess(t *testing.T) {
	m, k := newKanbanWithStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	// doMove needs two stacks so moveTarget can refer to a different one.
	k.stacks = append(k.stacks, api.Stack{ID: 2, Title: "B"})
	k.stackIdx = 0
	k.moveTarget = 1
	k.moveMode = true
	cmd := k.doMove(m)
	if cmd == nil {
		t.Fatal("doMove returned nil")
	}
	if msg := cmd(); !isRefreshMsg(msg) {
		t.Errorf("got %T, want refreshMsg", msg)
	}
	if k.moveMode {
		t.Errorf("moveMode should be cleared after doMove")
	}
}

func TestCreateCard_ReturnsRefreshOnSuccess(t *testing.T) {
	m, k := newKanbanWithStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":99,"title":"new"}`))
	})
	cmd := k.createCard(m, "new")
	if msg := cmd(); !isRefreshMsg(msg) {
		t.Errorf("got %T, want refreshMsg", msg)
	}
}

func TestCreateStack_ReturnsRefreshOnSuccess(t *testing.T) {
	m, k := newKanbanWithStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":2,"title":"new"}`))
	})
	cmd := k.createStack(m, "new")
	if msg := cmd(); !isRefreshMsg(msg) {
		t.Errorf("got %T, want refreshMsg", msg)
	}
}

// --- window picking ---------------------------------------------------------

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

func TestFiltersEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b map[int]bool
		want bool
	}{
		{"both nil", nil, nil, true},
		{"nil vs empty", nil, map[int]bool{}, true},
		{"identical", map[int]bool{1: true, 2: true}, map[int]bool{1: true, 2: true}, true},
		{"different size", map[int]bool{1: true}, map[int]bool{1: true, 2: true}, false},
		{"same size, different ids", map[int]bool{1: true}, map[int]bool{2: true}, false},
	}
	for _, tc := range cases {
		if got := filtersEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: filtersEqual(%v, %v) = %v, want %v", tc.name, tc.a, tc.b, got, tc.want)
		}
	}
}

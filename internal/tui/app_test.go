// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

func newRoutingModel() *Model {
	return &Model{
		ctx:    context.Background(),
		client: api.New("http://example.invalid", "u", "p"),
		active: screenBoards,
		boards: newBoardsModel(),
	}
}

func TestUpdate_BoardOpenedSwitchesToKanban(t *testing.T) {
	m := newRoutingModel()
	_, _ = m.Update(boardOpenedMsg{boardID: 7, color: "0082c9"})
	if m.active != screenKanban {
		t.Errorf("active: got %d, want screenKanban (%d)", m.active, screenKanban)
	}
	if m.kanban == nil || m.kanban.boardID != 7 {
		t.Fatalf("kanban: %+v", m.kanban)
	}
	if m.kanban.boardColor != "0082c9" {
		t.Errorf("kanban.boardColor: got %q, want %q", m.kanban.boardColor, "0082c9")
	}
}

func TestUpdate_OpenCardMsgEntersDetail(t *testing.T) {
	m := newRoutingModel()
	m.active = screenKanban
	m.kanban = newKanbanModel(7)
	card := &api.Card{ID: 42, StackID: 9, Title: "x"}
	_, _ = m.Update(openCardMsg{boardID: 7, card: card})
	if m.active != screenCard {
		t.Errorf("active: got %d, want screenCard", m.active)
	}
	if m.card == nil {
		t.Fatal("card model not created")
	}
	if m.card.boardID != 7 {
		t.Errorf("card.boardID: got %d, want 7", m.card.boardID)
	}
	if m.card.cardID != 42 || m.card.stackID != 9 {
		t.Errorf("card ids: stack=%d card=%d, want 9/42", m.card.stackID, m.card.cardID)
	}
}

func TestUpdate_BackFromCardToKanban(t *testing.T) {
	m := newRoutingModel()
	m.active = screenCard
	m.card = &cardModel{}
	_, _ = m.Update(backMsg{})
	if m.active != screenKanban {
		t.Errorf("active: got %d, want screenKanban", m.active)
	}
	if m.card != nil {
		t.Errorf("card should be cleared, got %+v", m.card)
	}
}

func TestUpdate_BackFromCardRefreshesKanban(t *testing.T) {
	m := newRoutingModel()
	m.active = screenCard
	m.card = &cardModel{}
	m.kanban = newKanbanModel(7)
	_, cmd := m.Update(backMsg{})
	if cmd == nil {
		t.Errorf("expected a loadStacks cmd so the kanban shows fresh data on return")
	}
	if !m.loading {
		t.Errorf("loading should be true while the kanban refresh is in flight")
	}
}

func TestUpdate_OpenCardMsgFiresBackgroundRefresh(t *testing.T) {
	m := newRoutingModel()
	m.active = screenKanban
	m.kanban = newKanbanModel(7)
	card := &api.Card{ID: 42, StackID: 9, Title: "cached"}
	_, cmd := m.Update(openCardMsg{boardID: 7, card: card})
	if cmd == nil {
		t.Fatal("expected a batched cmd including the background loadCard")
	}
	if m.card == nil || m.card.card == nil || m.card.card.Title != "cached" {
		t.Errorf("cached card not displayed: %+v", m.card)
	}
}

func TestCardLoadedMsg_PatchesCardWithoutReentering(t *testing.T) {
	m := newRoutingModel()
	m.active = screenCard
	m.card = &cardModel{cardID: 42, stackID: 9, card: &api.Card{ID: 42, Title: "old"}, mode: cardModeEditDescription}
	m.card.vp.Width = 80
	m.card.vp.Height = 20
	fresh := &api.Card{ID: 42, StackID: 9, Title: "new"}
	_, _ = m.Update(cardLoadedMsg{boardID: 7, card: fresh})
	if m.card.card.Title != "new" {
		t.Errorf("card data not patched: got title=%q, want %q", m.card.card.Title, "new")
	}
	if m.card.mode != cardModeEditDescription {
		t.Errorf("mode must be preserved across background refresh, got %d", m.card.mode)
	}
}

func TestCardLoadedMsg_IgnoresStaleCardID(t *testing.T) {
	m := newRoutingModel()
	m.active = screenCard
	m.card = &cardModel{cardID: 42, card: &api.Card{ID: 42, Title: "current"}}
	stale := &api.Card{ID: 99, Title: "wrong"}
	_, _ = m.Update(cardLoadedMsg{boardID: 7, card: stale})
	if m.card.card.Title != "current" {
		t.Errorf("stale cardLoadedMsg shouldn't overwrite the current card: got %q", m.card.card.Title)
	}
}

func TestUpdate_BackFromKanbanToBoards(t *testing.T) {
	m := newRoutingModel()
	m.active = screenKanban
	m.kanban = newKanbanModel(1)
	_, _ = m.Update(backMsg{})
	if m.active != screenBoards {
		t.Errorf("active: got %d, want screenBoards", m.active)
	}
	if !m.loading {
		t.Errorf("loading should be true while we reload the boards list")
	}
}

func TestUpdate_BoardInfoMsgUpdatesKanbanColor(t *testing.T) {
	m := newRoutingModel()
	m.active = screenKanban
	m.kanban = newKanbanModel(7)
	_, _ = m.Update(boardInfoMsg{boardID: 7, color: "ff7f50"})
	if m.kanban.boardColor != "ff7f50" {
		t.Errorf("boardColor: got %q, want %q", m.kanban.boardColor, "ff7f50")
	}
}

func TestUpdate_BoardInfoMsgIgnoredForWrongBoard(t *testing.T) {
	m := newRoutingModel()
	m.active = screenKanban
	m.kanban = newKanbanModel(7)
	_, _ = m.Update(boardInfoMsg{boardID: 99, color: "ff7f50"})
	if m.kanban.boardColor != "" {
		t.Errorf("boardColor should stay empty when boardID mismatches, got %q", m.kanban.boardColor)
	}
}

func TestModel_Accent_FallsBackWhenNoKanban(t *testing.T) {
	m := newRoutingModel()
	if got := m.accent(); got != colSelected {
		t.Errorf("accent without kanban: got %q, want %q", got, colSelected)
	}
}

func TestModel_Accent_UsesBoardColorWhenAvailable(t *testing.T) {
	m := newRoutingModel()
	m.kanban = newKanbanModel(1)
	m.kanban.boardColor = "deadbe"
	if got := m.accent(); got != lipgloss.Color("#deadbe") {
		t.Errorf("accent: got %q, want %q", got, "#deadbe")
	}
}

func TestEnterCard_PopulatesAllFields(t *testing.T) {
	m := newRoutingModel()
	card := &api.Card{ID: 42, StackID: 9, Title: "x"}
	_ = m.enterCard(7, card)
	if m.active != screenCard {
		t.Errorf("active: got %d, want screenCard", m.active)
	}
	if m.card.boardID != 7 {
		t.Errorf("boardID: got %d, want 7", m.card.boardID)
	}
	if m.card.cardID != 42 || m.card.stackID != 9 {
		t.Errorf("ids: stack=%d card=%d, want 9/42", m.card.stackID, m.card.cardID)
	}
	if m.card.mode != cardModeView {
		t.Errorf("fresh card mode: got %d, want cardModeView", m.card.mode)
	}
	if m.loading {
		t.Errorf("loading should be false after enterCard")
	}
}

// Refresh path (m.card already exists) must preserve mode so a save+'e'
// race doesn't drop the user's in-progress editor.
func TestEnterCard_PreservesModeOnRefresh(t *testing.T) {
	m := newRoutingModel()
	m.card = &cardModel{mode: cardModeEditDescription}
	_ = m.enterCard(7, &api.Card{ID: 42, StackID: 9})
	if m.card.mode != cardModeEditDescription {
		t.Errorf("mode after refresh: got %d, want cardModeEditDescription", m.card.mode)
	}
}

func TestReorderedMsg_ClearsInFlightFlag(t *testing.T) {
	m := newRoutingModel()
	m.active = screenKanban
	m.kanban = newKanbanModel(7)
	m.kanban.reorderInFlight = true
	_, _ = m.Update(reorderedMsg{boardID: 7})
	if m.kanban.reorderInFlight {
		t.Errorf("reorderInFlight should be false after confirmation")
	}
}

func TestReorderedMsg_IgnoredForWrongBoard(t *testing.T) {
	m := newRoutingModel()
	m.kanban = newKanbanModel(7)
	m.kanban.reorderInFlight = true
	_, cmd := m.Update(reorderedMsg{boardID: 99})
	if !m.kanban.reorderInFlight {
		t.Errorf("inFlight should stay set when boardID mismatches")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd on mismatched boardID, got %v", cmd)
	}
}

func TestReorderFailedMsg_ResyncsKanban(t *testing.T) {
	m := newRoutingModel()
	m.active = screenKanban
	m.kanban = newKanbanModel(7)
	m.kanban.reorderInFlight = true
	_, cmd := m.Update(reorderFailedMsg{boardID: 7, err: errors.New("boom")})
	if m.kanban.reorderInFlight {
		t.Errorf("inFlight should clear on failure")
	}
	if cmd == nil {
		t.Errorf("expected a resync (loadStacks) cmd on failure")
	}
	if !m.loading {
		t.Errorf("expected loading=true so the spinner shows during the resync")
	}
	if m.errStr == "" {
		t.Errorf("expected an error message to surface to the user")
	}
}

func TestEnterCard_NilCardIsNoOp(t *testing.T) {
	m := newRoutingModel()
	startActive := m.active
	if cmd := m.enterCard(7, nil); cmd != nil {
		t.Errorf("expected nil cmd for nil card, got %v", cmd)
	}
	if m.active != startActive {
		t.Errorf("active changed: got %d, want %d (no-op)", m.active, startActive)
	}
}

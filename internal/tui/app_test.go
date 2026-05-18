// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"
	"testing"

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
	if got := m.accent(); string(got) != string(colSelected) {
		t.Errorf("accent without kanban: got %q, want %q", got, colSelected)
	}
}

func TestModel_Accent_UsesBoardColorWhenAvailable(t *testing.T) {
	m := newRoutingModel()
	m.kanban = newKanbanModel(1)
	m.kanban.boardColor = "deadbe"
	if got := string(m.accent()); got != "#deadbe" {
		t.Errorf("accent: got %q, want %q", got, "#deadbe")
	}
}

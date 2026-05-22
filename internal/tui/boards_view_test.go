// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/raspbeguy/ncdeck/internal/api"
)

func boardsKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}}

func boardsFixture() (boardsModel, *Model) {
	b := newBoardsModel()
	b.setBoards([]api.Board{
		{ID: 1, Title: "Alpha", Color: "ff0000"},
		{ID: 2, Title: "Beta", Color: "00ff00"},
	})
	return b, &Model{}
}

// Pinned: while the help overlay is up on the board picker, navigation,
// open-board and refresh must all be swallowed. `?` / `esc` close the
// overlay; `q` still quits unconditionally.
func TestBoardsView_HelpOverlayIsModal(t *testing.T) {
	b := newBoardsModel()
	b.setBoards([]api.Board{
		{ID: 1, Title: "Alpha"},
		{ID: 2, Title: "Beta"},
		{ID: 3, Title: "Gamma"},
	})
	root := &Model{}

	b, _ = b.Update(boardsKey("?"), root)
	if !b.showHelp {
		t.Fatalf("setup: '?' should open the help overlay")
	}
	startCursor := b.cursor

	// Navigation must not move the cursor.
	b, _ = b.Update(boardsKey("j"), root)
	if b.cursor != startCursor {
		t.Errorf("'j' must be swallowed while help is up; got cursor=%d", b.cursor)
	}
	b, _ = b.Update(boardsKey("G"), root)
	if b.cursor != startCursor {
		t.Errorf("'G' must be swallowed while help is up; got cursor=%d", b.cursor)
	}

	// Opening a board must not fire while help is up.
	var cmd tea.Cmd
	b, cmd = b.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if cmd != nil {
		t.Errorf("⏎ while help is up should be swallowed, not open the board")
	}
	b, cmd = b.Update(boardsKey("l"), root)
	if cmd != nil {
		t.Errorf("'l' while help is up should be swallowed, not open the board")
	}
	b, cmd = b.Update(boardsKey("r"), root)
	if cmd != nil {
		t.Errorf("'r' while help is up should be swallowed, not refresh")
	}

	// esc closes the overlay and does NOT also quit in the same keystroke.
	b, cmd = b.Update(tea.KeyMsg{Type: tea.KeyEsc}, root)
	if b.showHelp {
		t.Errorf("'esc' should close the help overlay")
	}
	if cmd != nil {
		t.Errorf("'esc' while help is up should only close the overlay, not also quit")
	}

	// q must still quit even while the help overlay is up.
	b, _ = b.Update(boardsKey("?"), root)
	if !b.showHelp {
		t.Fatalf("setup: '?' should reopen the overlay")
	}
	_, cmd = b.Update(boardsKey("q"), root)
	if cmd == nil {
		t.Fatalf("'q' must return a non-nil cmd while help is up")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("'q' while help is up must still quit; got %T", cmd())
	}
}

func TestBoardsView_NKeyOpensNewBoardForm(t *testing.T) {
	b, root := boardsFixture()
	b, _ = b.Update(boardsKey("n"), root)
	if b.mode != boardsModeNewTitle {
		t.Fatalf("'n' should enter boardsModeNewTitle, got %d", b.mode)
	}
	if b.titleInput.Value() != "" {
		t.Errorf("new-board title input should start empty; got %q", b.titleInput.Value())
	}
}

func TestBoardsView_EKeyOpensEditFlowPrefilledWithCurrentTitle(t *testing.T) {
	b, root := boardsFixture()
	b.cursor = 1
	b, _ = b.Update(boardsKey("e"), root)
	if b.mode != boardsModeEditTitle {
		t.Fatalf("'e' should enter boardsModeEditTitle, got %d", b.mode)
	}
	if b.titleInput.Value() != "Beta" {
		t.Errorf("edit title input must be pre-filled with the current title; got %q", b.titleInput.Value())
	}
}

// Pinned: the edit flow is two-stage. The first ⏎ on the title must NOT
// commit yet; it advances to the colour picker (pre-filled with the board's
// current colour) so the user can change both.
func TestBoardsView_EditFlowAdvancesTitleToColor(t *testing.T) {
	b, root := boardsFixture()
	b.cursor = 0
	b, _ = b.Update(boardsKey("e"), root)
	b.titleInput.SetValue("Alpha renamed")
	var cmd tea.Cmd
	b, cmd = b.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if cmd != nil {
		t.Errorf("first ⏎ in the edit flow must not fire an update cmd yet (advance to colour first)")
	}
	if b.mode != boardsModeEditColor {
		t.Fatalf("first ⏎ should advance to colour picker; got mode=%d", b.mode)
	}
	if b.pendingTitle != "Alpha renamed" {
		t.Errorf("pendingTitle should hold the typed value; got %q", b.pendingTitle)
	}
	// ⏎ on the picker (red preset already selected) must fire the update.
	_, cmd = b.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if cmd == nil {
		t.Errorf("⏎ on the colour picker should fire an update cmd")
	}
}

func TestBoardsView_XKeyOpensConfirmDialog(t *testing.T) {
	b, root := boardsFixture()
	b, _ = b.Update(boardsKey("x"), root)
	if b.mode != boardsModeConfirmDel {
		t.Fatalf("'x' should enter boardsModeConfirmDel, got %d", b.mode)
	}
	// Plain ⏎ must NOT delete.
	var cmd tea.Cmd
	b, cmd = b.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if cmd != nil {
		t.Errorf("⏎ on the delete confirm must cancel, not delete")
	}
	if b.mode != boardsModeList {
		t.Errorf("⏎ should return to list mode after cancelling")
	}
	// y must delete (fire a non-nil cmd against the API).
	b, _ = b.Update(boardsKey("x"), root)
	if b.mode != boardsModeConfirmDel {
		t.Fatalf("setup: 'x' should reopen the confirm")
	}
	_, cmd = b.Update(boardsKey("y"), root)
	if cmd == nil {
		t.Errorf("'y' on the confirm should fire a delete cmd")
	}
}

// Drives the create flow against a real httptest server to confirm the
// CreateBoard request fires with the right payload.
func TestBoardsView_CreateBoardEndToEnd(t *testing.T) {
	var got struct {
		method, path, body string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		got.body = string(buf[:n])
		w.Write([]byte(`{"id": 42, "title": "demo"}`))
	}))
	t.Cleanup(srv.Close)

	b, _ := boardsFixture()
	root := &Model{ctx: context.Background(), client: api.New(srv.URL, "u", "p")}

	b, _ = b.Update(boardsKey("n"), root)
	b.titleInput.SetValue("demo")
	_, cmd := b.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if cmd == nil {
		t.Fatal("expected a cmd from create")
	}
	if _, ok := cmd().(refreshMsg); !ok {
		t.Errorf("create cmd should ultimately return a refreshMsg; got %T", cmd())
	}
	if got.method != "POST" || got.path != "/index.php/apps/deck/api/v1.0/boards" {
		t.Errorf("expected POST /boards, got %s %s", got.method, got.path)
	}
	if !contains(got.body, `"title":"demo"`) {
		t.Errorf("CreateBoard body should carry the typed title; got %s", got.body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

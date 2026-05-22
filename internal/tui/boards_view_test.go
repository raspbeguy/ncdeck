// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/raspbeguy/ncdeck/internal/api"
)

func boardsKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}}

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

// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "github.com/raspbeguy/ncdeck/internal/api"

// Cross-screen messages.

type boardsLoadedMsg struct {
	boards []api.Board
}

type boardOpenedMsg struct {
	boardID int
	color   string // empty when launched directly without going through the picker
}

// boardInfoMsg carries the board's metadata (currently the colour) for the
// kanban screen to use for focus highlights when launched directly.
type boardInfoMsg struct {
	boardID int
	color   string
}

type stacksLoadedMsg struct {
	boardID int
	stacks  []api.Stack
}

type cardLoadedMsg struct {
	card *api.Card
}

// openCardMsg uses an already-loaded card to switch to the detail screen
// without re-fetching from the server.
type openCardMsg struct {
	boardID int
	card    *api.Card
}

type commentsLoadedMsg struct {
	cardID   int
	comments []api.Comment
}

type attachmentsLoadedMsg struct {
	cardID      int
	attachments []api.Attachment
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type backMsg struct{}

type refreshMsg struct{}

type statusMsg struct{ text string }

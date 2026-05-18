// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "github.com/raspbeguy/ncdeck/internal/api"

type boardsLoadedMsg struct {
	boards []api.Board
}

type boardOpenedMsg struct {
	boardID int
	color   string // empty when launched directly without going through the picker
}

type boardInfoMsg struct {
	boardID int
	color   string
}

type stacksLoadedMsg struct {
	boardID int
	stacks  []api.Stack
}

type cardLoadedMsg struct {
	boardID int
	card    *api.Card
}

// openCardMsg skips a GetCard round-trip by reusing the card already loaded
// for the kanban view.
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

// reorderedMsg lets the kanban move its cursor onto the moved card *after*
// the server confirms, instead of optimistically moving it first and trying
// to roll back on failure.
type reorderedMsg struct {
	boardID    int
	newCardIdx int
}

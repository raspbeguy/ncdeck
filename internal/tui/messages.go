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
	labels  []api.Label
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

// reorderedMsg clears reorderInFlight after the server confirms a reorder
// the kanban already applied optimistically. carries only boardID because
// the cursor + local stack were updated synchronously when the press fired.
type reorderedMsg struct {
	boardID int
}

// reorderFailedMsg triggers a full reload to repair the optimistic local
// state after a server rejection.
type reorderFailedMsg struct {
	boardID int
	err     error
}

// labelCreatedMsg lets the kanban + label dialog adopt a freshly-created
// label without a full board re-fetch.
type labelCreatedMsg struct {
	boardID int
	label   api.Label
}

// labelUpdatedMsg / labelDeletedMsg propagate manager mutations so the
// kanban's cached palette and the manager dialog stay in sync.
type labelUpdatedMsg struct {
	boardID int
	label   api.Label
}

type labelDeletedMsg struct {
	boardID int
	labelID int
}

type labelOpFailedMsg struct {
	boardID int
	err     error
}

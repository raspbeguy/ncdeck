// SPDX-License-Identifier: GPL-3.0-or-later

package api

// We import boards client-side (CreateBoard + many follow-ups) rather than
// POSTing the whole JSON to Deck's bulk import endpoints. Reasons:
//   - Deck's web-UI route (POST /index.php/apps/deck/boards/import) requires
//     session cookies + a CSRF requesttoken. ncdeck authenticates with app
//     passwords (basic auth), so we'd need a full login-and-session shim.
//   - Deck's documented API route (POST /api/v1.0/boards/import) accepts
//     basic auth but is broken on at least Nextcloud 32 / Deck 1.16: the
//     `data` field arrives at the controller as an int regardless of the
//     posted JSON, returning "must be of type stdClass, int given".
//   - Even when the web-UI route works, the server-side importer drops the
//     `done` timestamp on cards. Our client-side path restores it.
//
// If the documented API import endpoint gets fixed upstream, the natural
// move is to add a --via-server opt-in flag, keeping this orchestrator as
// the default.

import (
	"context"
	"fmt"
	"sort"
)

type ImportOptions struct {
	TitleOverride      string
	BoardIndex         int
	SkipAssignees      bool
	KeepDefaultLabels  bool
}

// ImportProgress receives a one-line status update per phase: pass nil to
// suppress. Errors during user-assignment do not abort the import; they're
// surfaced via this callback instead.
type ImportProgress func(line string)

// IDs are remapped on the fly: stack and label references inside the export
// don't match the new server's IDs, so we build oldID -> newID maps for
// labels and stacks before creating cards.
func (c *Client) ImportBoard(ctx context.Context, export *DeckExport, opts ImportOptions, progress ImportProgress) (*Board, error) {
	if export == nil || len(export.Boards) == 0 {
		return nil, fmt.Errorf("import: file has no boards")
	}
	if opts.BoardIndex < 0 || opts.BoardIndex >= len(export.Boards) {
		return nil, fmt.Errorf("import: --board-index %d out of range (file has %d boards)", opts.BoardIndex, len(export.Boards))
	}
	src := export.Boards[opts.BoardIndex]

	title := src.Title
	if opts.TitleOverride != "" {
		title = opts.TitleOverride
	}

	report := func(s string) {
		if progress != nil {
			progress(s)
		}
	}

	board, err := c.CreateBoard(ctx, CreateBoardInput{Title: title, Color: src.Color})
	if err != nil {
		return nil, fmt.Errorf("create board: %w", err)
	}
	report(fmt.Sprintf("created board %q (id=%d)", board.Title, board.ID))

	if !opts.KeepDefaultLabels {
		removed, err := c.wipeAutoLabels(ctx, board.ID)
		if err != nil {
			return board, fmt.Errorf("wipe default labels: %w", err)
		}
		report(fmt.Sprintf("removed %d default labels", removed))
	}

	labelMap, err := c.importLabels(ctx, board.ID, src.Labels)
	if err != nil {
		return board, fmt.Errorf("import labels: %w", err)
	}
	report(fmt.Sprintf("created %d/%d labels", len(labelMap), len(src.Labels)))

	// Map iteration is unordered: sort stacks by their Order field so we
	// recreate them in the same on-screen sequence (and so the server's
	// renormalised positions match the source layout).
	stacks := make([]ExportStack, 0, len(src.Stacks))
	for _, s := range src.Stacks {
		stacks = append(stacks, s)
	}
	sort.SliceStable(stacks, func(i, j int) bool { return stacks[i].Order < stacks[j].Order })

	for _, s := range stacks {
		ns, err := c.CreateStack(ctx, board.ID, StackInput{Title: s.Title, Order: s.Order})
		if err != nil {
			return board, fmt.Errorf("create stack %q: %w", s.Title, err)
		}

		cards := append([]ExportCard(nil), s.Cards...)
		sort.SliceStable(cards, func(i, j int) bool { return cards[i].Order < cards[j].Order })

		created := 0
		for _, card := range cards {
			if err := c.importCard(ctx, board.ID, ns.ID, card, labelMap, opts, report); err != nil {
				return board, fmt.Errorf("create card %q in %q: %w", card.Title, s.Title, err)
			}
			created++
		}
		report(fmt.Sprintf("%s: %d/%d cards", s.Title, created, len(cards)))
	}

	// CreateBoard's response is the server's first snapshot, taken before any
	// of our follow-up calls ran. Re-fetch so callers (especially --json) see
	// the post-import state. GetBoard returns labels but not stacks; callers
	// that want a full snapshot should follow up with `board show`.
	if fresh, err := c.GetBoard(ctx, board.ID); err == nil {
		board = fresh
	} else {
		report(fmt.Sprintf("warning: couldn't re-fetch the imported board: %v", err))
	}

	return board, nil
}

// wipeAutoLabels removes the four default labels Deck auto-creates with every
// new board, so the imported labels don't fight them by title. Aborts on the
// first delete failure: the cause is almost always auth/permissions, and a
// best-effort partial wipe would leave defaults coexisting with imports
// silently (worse than failing loudly).
func (c *Client) wipeAutoLabels(ctx context.Context, boardID int) (int, error) {
	b, err := c.GetBoard(ctx, boardID)
	if err != nil {
		return 0, err
	}
	for _, l := range b.Labels {
		if err := c.DeleteLabel(ctx, boardID, l.ID); err != nil {
			return 0, err
		}
	}
	return len(b.Labels), nil
}

func (c *Client) importLabels(ctx context.Context, boardID int, labels []ExportLabel) (map[int]int, error) {
	out := make(map[int]int, len(labels))
	for _, l := range labels {
		nl, err := c.CreateLabel(ctx, boardID, LabelInput{Title: l.Title, Color: l.Color})
		if err != nil {
			return out, fmt.Errorf("label %q: %w", l.Title, err)
		}
		out[l.ID] = nl.ID
	}
	return out, nil
}

func (c *Client) importCard(ctx context.Context, boardID, stackID int, card ExportCard, labelMap map[int]int, opts ImportOptions, report ImportProgress) error {
	in := CreateCardInput{
		Title:       card.Title,
		Type:        card.Type,
		Order:       card.Order,
		Description: card.Description,
	}
	if card.DueDate != nil {
		in.DueDate = *card.DueDate
	}
	created, err := c.CreateCard(ctx, boardID, stackID, in)
	if err != nil {
		return err
	}

	// Done is a timestamp string, Archived is a flag: both are post-create
	// state the CreateCardInput doesn't accept, so an UpdateCard is needed
	// when either is set. Build the input from the just-created card so we
	// don't accidentally clear the description or due-date we just sent.
	if card.Done != nil || card.Archived {
		upd := UpdateCardInput{
			Title:       created.Title,
			Description: created.Description,
			Type:        created.Type,
			Owner:       created.Owner.UID,
			Order:       created.Order,
			DueDate:     created.DueDate,
			Archived:    card.Archived,
			Done:        card.Done,
		}
		if _, err := c.UpdateCard(ctx, boardID, stackID, created.ID, upd); err != nil {
			return fmt.Errorf("set done/archived: %w", err)
		}
	}

	for _, l := range card.Labels {
		nid, ok := labelMap[l.ID]
		if !ok {
			// Card references a label id that wasn't in the file's labels[].
			// Surface a warning so silent data loss is visible (a Trello-
			// converted file, or a hand-edited export, can land here).
			report(fmt.Sprintf("warning: card %q references label id %d not in the file; assignment skipped", card.Title, l.ID))
			continue
		}
		if err := c.AssignLabelToCard(ctx, boardID, stackID, created.ID, nid); err != nil {
			return fmt.Errorf("assign label %q: %w", l.Title, err)
		}
	}

	if !opts.SkipAssignees {
		for _, a := range card.AssignedUsers {
			uid := a.Participant.UID
			if uid == "" {
				continue
			}
			if err := c.AssignUserToCard(ctx, boardID, stackID, created.ID, uid); err != nil {
				report(fmt.Sprintf("warning: skipped assignee %q on %q (%v)", uid, card.Title, err))
			}
		}
	}

	return nil
}

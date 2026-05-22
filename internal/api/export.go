// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
)

// DeckExport matches the JSON shape produced by `occ deck:export` and
// consumed by `occ deck:import`. Comments are intentionally absent: the
// server's exporter doesn't include them.
type DeckExport struct {
	Boards []ExportBoard `json:"boards"`
}

type ExportBoard struct {
	ID             int                    `json:"id"`
	Title          string                 `json:"title"`
	Owner          string                 `json:"owner"`
	Color          string                 `json:"color"`
	Archived       bool                   `json:"archived"`
	Labels         []ExportLabel          `json:"labels"`
	ACL            []any                  `json:"acl"`
	Permissions    []any                  `json:"permissions"`
	Users          []any                  `json:"users"`
	Stacks         map[string]ExportStack `json:"stacks"`
	ActiveSessions []any                  `json:"activeSessions"`
	DeletedAt      int64                  `json:"deletedAt"`
	LastModified   int64                  `json:"lastModified"`
	Settings       []any                  `json:"settings"`
	ETag           string                 `json:"ETag"`
}

type ExportLabel struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Color        string `json:"color"`
	BoardID      int    `json:"boardId"`
	CardID       *int   `json:"cardId"`
	LastModified int64  `json:"lastModified"`
	ETag         string `json:"ETag"`
}

type ExportStack struct {
	ID           int          `json:"id"`
	Title        string       `json:"title"`
	BoardID      int          `json:"boardId"`
	DeletedAt    int64        `json:"deletedAt"`
	LastModified int64        `json:"lastModified"`
	Cards        []ExportCard `json:"cards"`
	Order        int          `json:"order"`
	ETag         string       `json:"ETag"`
}

type ExportCard struct {
	ID              int                `json:"id"`
	Title           string             `json:"title"`
	Description     string             `json:"description"`
	DescriptionPrev *string            `json:"descriptionPrev"`
	StackID         int                `json:"stackId"`
	Type            string             `json:"type"`
	LastModified    int64              `json:"lastModified"`
	LastEditor      *string            `json:"lastEditor"`
	CreatedAt       int64              `json:"createdAt"`
	Labels          []ExportLabel      `json:"labels"`
	AssignedUsers   []ExportAssignment `json:"assignedUsers"`
	Attachments     []ExportAttachment `json:"attachments"`
	AttachmentCount *int               `json:"attachmentCount"`
	Owner           Owner              `json:"owner"`
	Order           int                `json:"order"`
	Archived        bool               `json:"archived"`
	Done            *string            `json:"done"`
	DueDate         *string            `json:"duedate"`
	Notified        bool               `json:"notified"`
	DeletedAt       int64              `json:"deletedAt"`
	CommentsUnread  int                `json:"commentsUnread"`
	CommentsCount   int                `json:"commentsCount"`
	RelatedStack    *json.RawMessage   `json:"relatedStack"`
	RelatedBoard    *json.RawMessage   `json:"relatedBoard"`
	ETag            string             `json:"ETag"`
}

type ExportAssignment struct {
	ID          int   `json:"id"`
	Participant Owner `json:"participant"`
	CardID      int   `json:"cardId"`
	Type        int   `json:"type"`
}

// ExportAttachment payload bytes aren't included by `occ deck:export` in
// any sample we have. Kept as a raw map so future schema details (e.g.
// base64-encoded data) round-trip without us guessing.
type ExportAttachment map[string]any

// ExportBoard fetches a board and its stacks/cards from the live Deck
// server and assembles them into the same JSON shape as `occ deck:export`.
func (c *Client) ExportBoard(ctx context.Context, boardID int) (*DeckExport, error) {
	board, err := c.GetBoard(ctx, boardID)
	if err != nil {
		return nil, err
	}
	stacks, err := c.ListStacks(ctx, boardID)
	if err != nil {
		return nil, err
	}

	eb := ExportBoard{
		ID:             board.ID,
		Title:          board.Title,
		Owner:          board.OwnerRaw.UID,
		Color:          board.Color,
		Archived:       board.Archived,
		Labels:         convertBoardLabels(board.Labels),
		ACL:            []any{},
		Permissions:    []any{},
		Users:          []any{},
		Stacks:         make(map[string]ExportStack, len(stacks)),
		ActiveSessions: []any{},
		DeletedAt:      board.DeletedAt,
		LastModified:   board.LastModified,
		Settings:       []any{},
		ETag:           board.ETag,
	}
	for _, s := range stacks {
		eb.Stacks[strconv.Itoa(s.ID)] = convertStack(s)
	}
	return &DeckExport{Boards: []ExportBoard{eb}}, nil
}

func convertBoardLabels(labels []Label) []ExportLabel {
	out := make([]ExportLabel, 0, len(labels))
	for _, l := range labels {
		out = append(out, ExportLabel{
			ID:           l.ID,
			Title:        l.Title,
			Color:        l.Color,
			BoardID:      l.BoardID,
			CardID:       nil,
			LastModified: l.LastModified,
			ETag:         l.ETag,
		})
	}
	return out
}

func convertCardLabels(labels []Label, cardID int) []ExportLabel {
	out := make([]ExportLabel, 0, len(labels))
	for _, l := range labels {
		id := cardID
		out = append(out, ExportLabel{
			ID:           l.ID,
			Title:        l.Title,
			Color:        l.Color,
			BoardID:      l.BoardID,
			CardID:       &id,
			LastModified: l.LastModified,
			ETag:         l.ETag,
		})
	}
	return out
}

func convertStack(s Stack) ExportStack {
	cards := make([]ExportCard, 0, len(s.Cards))
	for _, c := range s.Cards {
		cards = append(cards, convertCard(c))
	}
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].Order < cards[j].Order })
	return ExportStack{
		ID:           s.ID,
		Title:        s.Title,
		BoardID:      s.BoardID,
		DeletedAt:    s.DeletedAt,
		LastModified: s.LastModified,
		Cards:        cards,
		Order:        s.Order,
		ETag:         s.ETag,
	}
}

func convertCard(c Card) ExportCard {
	var attachments []ExportAttachment
	var attachmentCount *int
	assignments := make([]ExportAssignment, 0, len(c.AssignedUsers))
	for _, a := range c.AssignedUsers {
		assignments = append(assignments, ExportAssignment{
			ID: a.ID,
			Participant: Owner{
				PrimaryKey:  a.PrimaryKey,
				UID:         a.UID,
				DisplayName: a.DisplayName,
			},
			CardID: a.CardID,
			Type:   a.Type,
		})
	}
	return ExportCard{
		ID:              c.ID,
		Title:           c.Title,
		Description:     c.Description,
		DescriptionPrev: nil,
		StackID:         c.StackID,
		Type:            c.Type,
		LastModified:    c.LastModified,
		LastEditor:      nil,
		CreatedAt:       c.CreatedAt,
		Labels:          convertCardLabels(c.Labels, c.ID),
		AssignedUsers:   assignments,
		Attachments:     attachments,
		AttachmentCount: attachmentCount,
		Owner:           c.Owner,
		Order:           c.Order,
		Archived:        c.Archived,
		Done:            c.Done,
		DueDate:         c.DueDate,
		Notified:        false,
		DeletedAt:       0,
		CommentsUnread:  c.CommentsUnread,
		CommentsCount:   c.CommentsCount,
		RelatedStack:    nil,
		RelatedBoard:    nil,
		ETag:            c.ETag,
	}
}

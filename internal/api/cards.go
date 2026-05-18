// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"fmt"
)

type CreateCardInput struct {
	Title       string `json:"title"`
	Type        string `json:"type,omitempty"`
	Order       int    `json:"order"`
	Description string `json:"description,omitempty"`
	DueDate     string `json:"duedate,omitempty"`
}

func (c *Client) CreateCard(ctx context.Context, boardID, stackID int, in CreateCardInput) (*Card, error) {
	if in.Type == "" {
		in.Type = "plain"
	}
	var out Card
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards", boardID, stackID)
	if err := c.do(ctx, "POST", path, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetCard(ctx context.Context, boardID, stackID, cardID int) (*Card, error) {
	var out Card
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d", boardID, stackID, cardID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateCardInput captures fields supported by PUT /boards/{b}/stacks/{s}/cards/{c}.
//
// DueDate and Done are *string so callers can distinguish three cases:
//   - field omitted entirely from the wire format    , unsupported (use a fetched
//     value and pass it through)
//   - explicit JSON null                             , clears the value
//   - non-empty string                               , sets the value
//
// Empty-string semantics are server-defined; prefer nil to clear.
type UpdateCardInput struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Owner       string  `json:"owner"`
	Order       int     `json:"order"`
	DueDate     *string `json:"duedate"`
	Archived    bool    `json:"archived"`
	Done        *string `json:"done"`
}

func (c *Client) UpdateCard(ctx context.Context, boardID, stackID, cardID int, in UpdateCardInput) (*Card, error) {
	var out Card
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d", boardID, stackID, cardID)
	if err := c.do(ctx, "PUT", path, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteCard(ctx context.Context, boardID, stackID, cardID int) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d", boardID, stackID, cardID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

func (c *Client) ArchiveCard(ctx context.Context, boardID, stackID, cardID int) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/archive", boardID, stackID, cardID)
	return c.do(ctx, "PUT", path, nil, nil)
}

func (c *Client) UnarchiveCard(ctx context.Context, boardID, stackID, cardID int) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/unarchive", boardID, stackID, cardID)
	return c.do(ctx, "PUT", path, nil, nil)
}

func (c *Client) AssignLabelToCard(ctx context.Context, boardID, stackID, cardID, labelID int) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/assignLabel", boardID, stackID, cardID)
	return c.do(ctx, "PUT", path, map[string]int{"labelId": labelID}, nil)
}

func (c *Client) RemoveLabelFromCard(ctx context.Context, boardID, stackID, cardID, labelID int) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/removeLabel", boardID, stackID, cardID)
	return c.do(ctx, "PUT", path, map[string]int{"labelId": labelID}, nil)
}

func (c *Client) AssignUserToCard(ctx context.Context, boardID, stackID, cardID int, userID string) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/assignUser", boardID, stackID, cardID)
	return c.do(ctx, "PUT", path, map[string]string{"userId": userID}, nil)
}

func (c *Client) UnassignUserFromCard(ctx context.Context, boardID, stackID, cardID int, userID string) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/unassignUser", boardID, stackID, cardID)
	return c.do(ctx, "PUT", path, map[string]string{"userId": userID}, nil)
}

type ReorderInput struct {
	Order   int `json:"order"`
	StackID int `json:"stackId"`
}

// ReorderCard moves a card to (or within) the stack identified by in.StackID
// and sets its order. The Deck API's reorder route takes the destination stack
// ID in the URL path, not the source: passing the source silently no-ops the
// stack change.
func (c *Client) ReorderCard(ctx context.Context, boardID, cardID int, in ReorderInput) error {
	path := fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/reorder", boardID, in.StackID, cardID)
	return c.do(ctx, "PUT", path, in, nil)
}

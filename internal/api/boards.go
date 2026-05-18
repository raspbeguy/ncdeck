// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"fmt"
)

func (c *Client) ListBoards(ctx context.Context, details bool) ([]Board, error) {
	path := "/boards"
	if details {
		path += "?details=true"
	}
	var out []Board
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetBoard(ctx context.Context, id int) (*Board, error) {
	var out Board
	if err := c.do(ctx, "GET", fmt.Sprintf("/boards/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type CreateBoardInput struct {
	Title string `json:"title"`
	Color string `json:"color"`
}

func (c *Client) CreateBoard(ctx context.Context, in CreateBoardInput) (*Board, error) {
	var out Board
	if err := c.do(ctx, "POST", "/boards", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type UpdateBoardInput struct {
	Title    string `json:"title"`
	Color    string `json:"color"`
	Archived bool   `json:"archived"`
}

func (c *Client) UpdateBoard(ctx context.Context, id int, in UpdateBoardInput) (*Board, error) {
	var out Board
	if err := c.do(ctx, "PUT", fmt.Sprintf("/boards/%d", id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteBoard(ctx context.Context, id int) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/boards/%d", id), nil, nil)
}

func (c *Client) UndoDeleteBoard(ctx context.Context, id int) (*Board, error) {
	var out Board
	if err := c.do(ctx, "POST", fmt.Sprintf("/boards/%d/undo_delete", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"fmt"
)

type LabelInput struct {
	Title string `json:"title"`
	Color string `json:"color"`
}

func (c *Client) GetLabel(ctx context.Context, boardID, labelID int) (*Label, error) {
	var out Label
	path := fmt.Sprintf("/boards/%d/labels/%d", boardID, labelID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateLabel(ctx context.Context, boardID int, in LabelInput) (*Label, error) {
	var out Label
	path := fmt.Sprintf("/boards/%d/labels", boardID)
	if err := c.do(ctx, "POST", path, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateLabel(ctx context.Context, boardID, labelID int, in LabelInput) (*Label, error) {
	var out Label
	path := fmt.Sprintf("/boards/%d/labels/%d", boardID, labelID)
	if err := c.do(ctx, "PUT", path, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteLabel(ctx context.Context, boardID, labelID int) error {
	path := fmt.Sprintf("/boards/%d/labels/%d", boardID, labelID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

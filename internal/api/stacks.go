// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"fmt"
)

func (c *Client) ListStacks(ctx context.Context, boardID int) ([]Stack, error) {
	var out []Stack
	if err := c.do(ctx, "GET", fmt.Sprintf("/boards/%d/stacks", boardID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ListArchivedStacks(ctx context.Context, boardID int) ([]Stack, error) {
	var out []Stack
	if err := c.do(ctx, "GET", fmt.Sprintf("/boards/%d/stacks/archived", boardID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetStack(ctx context.Context, boardID, stackID int) (*Stack, error) {
	var out Stack
	if err := c.do(ctx, "GET", fmt.Sprintf("/boards/%d/stacks/%d", boardID, stackID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type StackInput struct {
	Title string `json:"title"`
	Order int    `json:"order"`
}

func (c *Client) CreateStack(ctx context.Context, boardID int, in StackInput) (*Stack, error) {
	var out Stack
	if err := c.do(ctx, "POST", fmt.Sprintf("/boards/%d/stacks", boardID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateStack(ctx context.Context, boardID, stackID int, in StackInput) (*Stack, error) {
	var out Stack
	if err := c.do(ctx, "PUT", fmt.Sprintf("/boards/%d/stacks/%d", boardID, stackID), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteStack(ctx context.Context, boardID, stackID int) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/boards/%d/stacks/%d", boardID, stackID), nil, nil)
}

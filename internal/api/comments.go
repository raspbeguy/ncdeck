// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ocsCommentsBase points at the OCS routes that wrap the Nextcloud comments app
// for Deck card objects.
const ocsCommentsBase = "/ocs/v2.php/apps/deck/api/v1.0"

// ocsEnvelope is the standard OCS response wrapper.
type ocsEnvelope[T any] struct {
	OCS struct {
		Meta struct {
			Status     string `json:"status"`
			StatusCode int    `json:"statuscode"`
			Message    string `json:"message"`
		} `json:"meta"`
		Data T `json:"data"`
	} `json:"ocs"`
}

func (c *Client) ocsURL(path string, q url.Values) string {
	u := c.BaseURL + ocsCommentsBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// ListComments returns the comments on a card. limit/offset paginate the result.
func (c *Client) ListComments(ctx context.Context, cardID, limit, offset int) ([]Comment, error) {
	q := url.Values{}
	q.Set("format", "json")
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	u := c.ocsURL(fmt.Sprintf("/cards/%d/comments", cardID), q)
	var env ocsEnvelope[json.RawMessage]
	if err := c.doRaw(ctx, "GET", u, nil, &env); err != nil {
		return nil, err
	}
	if env.OCS.Meta.Status != "" && env.OCS.Meta.Status != "ok" {
		return nil, &APIError{Status: env.OCS.Meta.StatusCode, Message: env.OCS.Meta.Message}
	}
	var out []Comment
	if len(env.OCS.Data) == 0 || strings.TrimSpace(string(env.OCS.Data)) == "[]" {
		return out, nil
	}
	if err := json.Unmarshal(env.OCS.Data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddComment posts a new comment to a card.
func (c *Client) AddComment(ctx context.Context, cardID int, message string, replyTo int) (*Comment, error) {
	body := map[string]any{"message": message}
	if replyTo > 0 {
		body["parentId"] = replyTo
	}
	q := url.Values{"format": []string{"json"}}
	u := c.ocsURL(fmt.Sprintf("/cards/%d/comments", cardID), q)
	var env ocsEnvelope[Comment]
	if err := c.doRaw(ctx, "POST", u, body, &env); err != nil {
		return nil, err
	}
	return &env.OCS.Data, nil
}

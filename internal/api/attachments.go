// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// ListAttachments returns the file attachments on a card.
func (c *Client) ListAttachments(ctx context.Context, cardID int) ([]Attachment, error) {
	var out []Attachment
	path := fmt.Sprintf("/cards/%d/attachments", cardID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UploadAttachment uploads a local file as an attachment on a card.
func (c *Client) UploadAttachment(ctx context.Context, cardID int, file string) (*Attachment, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("type", "file")
	fw, err := w.CreateFormFile("file", filepath.Base(file))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}
	w.Close()

	url := c.BaseURL + apiBase + fmt.Sprintf("/cards/%d/attachments", cardID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.User, c.Password)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	var out Attachment
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DownloadAttachment streams the raw bytes of an attachment to dst.
func (c *Client) DownloadAttachment(ctx context.Context, cardID, attachmentID int, dst io.Writer) error {
	url := c.BaseURL + apiBase + fmt.Sprintf("/cards/%d/attachments/%d", cardID, attachmentID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.User, c.Password)
	req.Header.Set("OCS-APIRequest", "true")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(body)}
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

// DeleteAttachment removes an attachment from a card.
func (c *Client) DeleteAttachment(ctx context.Context, cardID, attachmentID int) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/cards/%d/attachments/%d", cardID, attachmentID), nil, nil)
}

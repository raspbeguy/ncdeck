// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
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

// UploadAttachment uploads a local file as an attachment on a card. The file
// is streamed through an io.Pipe so the whole payload is never buffered in
// memory.
func (c *Client) UploadAttachment(ctx context.Context, cardID int, file string) (*Attachment, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	contentType := w.FormDataContentType()

	go func() {
		// Close pw last so any error in the writer surfaces on the read side.
		var werr error
		defer func() {
			if cerr := w.Close(); werr == nil {
				werr = cerr
			}
			pw.CloseWithError(werr)
		}()
		if werr = w.WriteField("type", "file"); werr != nil {
			return
		}
		fw, err := w.CreateFormFile("file", filepath.Base(file))
		if err != nil {
			werr = err
			return
		}
		if _, err := io.Copy(fw, f); err != nil {
			werr = err
			return
		}
	}()

	var out Attachment
	path := fmt.Sprintf("/cards/%d/attachments", cardID)
	if err := c.doMultipart(ctx, "POST", path, contentType, pr, &out); err != nil {
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

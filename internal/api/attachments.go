// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

// attachmentsBase returns the API path prefix for a card's attachments. The
// Deck API scopes attachments under their board+stack+card, not directly under
// /cards/{id} (which 405s on GET).
func attachmentsBase(boardID, stackID, cardID int) string {
	return fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/attachments", boardID, stackID, cardID)
}

// ListAttachments returns the file attachments on a card.
func (c *Client) ListAttachments(ctx context.Context, boardID, stackID, cardID int) ([]Attachment, error) {
	var out []Attachment
	if err := c.do(ctx, "GET", attachmentsBase(boardID, stackID, cardID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UploadAttachment uploads a local file as an attachment on a card. The file
// is streamed through an io.Pipe so the whole payload is never buffered in
// memory.
func (c *Client) UploadAttachment(ctx context.Context, boardID, stackID, cardID int, file string) (*Attachment, error) {
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
	if err := c.doMultipart(ctx, "POST", attachmentsBase(boardID, stackID, cardID), contentType, pr, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DownloadAttachment streams the raw bytes of an attachment to dst.
func (c *Client) DownloadAttachment(ctx context.Context, boardID, stackID, cardID, attachmentID int, dst io.Writer) error {
	return c.doStream(ctx, fmt.Sprintf("%s/%d", attachmentsBase(boardID, stackID, cardID), attachmentID), dst)
}

// DeleteAttachment removes an attachment from a card.
func (c *Client) DeleteAttachment(ctx context.Context, boardID, stackID, cardID, attachmentID int) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("%s/%d", attachmentsBase(boardID, stackID, cardID), attachmentID), nil, nil)
}

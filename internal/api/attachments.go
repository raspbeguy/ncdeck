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

// Deck's attachment routes live under board+stack+card; the shorter
// /cards/{id}/attachments path 405s on GET.
func attachmentsBase(boardID, stackID, cardID int) string {
	return fmt.Sprintf("/boards/%d/stacks/%d/cards/%d/attachments", boardID, stackID, cardID)
}

func (c *Client) ListAttachments(ctx context.Context, boardID, stackID, cardID int) ([]Attachment, error) {
	var out []Attachment
	if err := c.do(ctx, "GET", attachmentsBase(boardID, stackID, cardID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UploadAttachment streams the file through an io.Pipe so payload size
// is not bounded by available memory.
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
		// Close pw last so writer errors surface on the read side.
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

func (c *Client) DownloadAttachment(ctx context.Context, boardID, stackID, cardID, attachmentID int, dst io.Writer) error {
	return c.doStream(ctx, fmt.Sprintf("%s/%d", attachmentsBase(boardID, stackID, cardID), attachmentID), dst)
}

func (c *Client) DeleteAttachment(ctx context.Context, boardID, stackID, cardID, attachmentID int) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("%s/%d", attachmentsBase(boardID, stackID, cardID), attachmentID), nil, nil)
}

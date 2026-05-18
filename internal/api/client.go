// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const apiBase = "/index.php/apps/deck/api/v1.0"

// Client talks to the Nextcloud Deck REST API.
type Client struct {
	BaseURL  string
	User     string
	Password string
	HTTP     *http.Client
}

// New constructs a Client with sensible defaults.
//
// The HTTP client has no overall timeout: per-request cancellation is handled
// via the caller's context. A blanket 30s timeout would cap long attachment
// uploads/downloads against a slow link.
//
// Setting NCDECK_INSECURE_TLS=1 disables certificate verification. Only use
// this with self-hosted Nextcloud instances on internal networks; a warning
// is printed to stderr on construction.
func New(baseURL, user, password string) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if v := os.Getenv("NCDECK_INSECURE_TLS"); v == "1" || strings.EqualFold(v, "true") {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		fmt.Fprintln(os.Stderr, "ncdeck: WARNING: NCDECK_INSECURE_TLS is set, TLS certificate verification disabled")
	}
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		User:     user,
		Password: password,
		HTTP:     &http.Client{Transport: tr},
	}
}

// APIError represents a non-2xx response.
type APIError struct {
	Status  int
	Message string
	Body    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("nextcloud %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("nextcloud %d: %s", e.Status, e.Body)
}

// do executes a request against the Deck API. body and out may be nil.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	return c.doRaw(ctx, method, c.BaseURL+apiBase+path, body, out)
}

// doMultipart executes a multipart/form-data request. The request body is
// streamed from `bodyReader`; callers should produce the multipart payload in
// a separate goroutine writing to a pipe whose read end is `bodyReader`.
// `contentType` must be the value returned by multipart.Writer.FormDataContentType().
func (c *Client) doMultipart(ctx context.Context, method, path, contentType string, bodyReader io.Reader, out any) error {
	url := c.BaseURL + apiBase + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.User, c.Password)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// doRaw is like do but does not prepend the Deck API base path.
func (c *Client) doRaw(ctx context.Context, method, url string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.User, c.Password)
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		ae := &APIError{Status: resp.StatusCode, Body: string(raw)}
		var parsed struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(raw, &parsed); err == nil {
			if parsed.Message != "" {
				ae.Message = parsed.Message
			} else if parsed.Error != "" {
				ae.Message = parsed.Error
			}
		}
		return ae
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

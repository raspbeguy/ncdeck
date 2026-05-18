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

type Client struct {
	BaseURL  string
	User     string
	Password string
	HTTP     *http.Client
}

// New leaves http.Client.Timeout at zero so callers' contexts bound request
// duration; a blanket timeout would cap long attachment transfers.
// NCDECK_INSECURE_TLS=1 disables certificate verification for self-hosted
// instances using internal CAs.
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

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	return c.doRaw(ctx, method, c.BaseURL+apiBase+path, body, out)
}

func (c *Client) doStream(ctx context.Context, path string, dst io.Writer) error {
	url := c.BaseURL + apiBase + path
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

// doMultipart expects bodyReader to be the read end of a pipe being written
// by a goroutine producing the multipart payload, and contentType to be
// multipart.Writer.FormDataContentType() so the boundary matches.
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

// doRaw skips the Deck API base path so callers (OCS routes, login flow)
// can target a different prefix on the same host.
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

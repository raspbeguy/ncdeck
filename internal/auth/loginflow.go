// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type LoginInit struct {
	Login string   `json:"login"`
	Poll  PollInfo `json:"poll"`
}

type PollInfo struct {
	Token    string `json:"token"`
	Endpoint string `json:"endpoint"`
}

type LoginResult struct {
	Server      string `json:"server"`
	LoginName   string `json:"loginName"`
	AppPassword string `json:"appPassword"`
}

func Start(ctx context.Context, baseURL string) (*LoginInit, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/index.php/login/v2", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ncdeck")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login init: %s, %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out LoginInit
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// pollInterval is var, not const, so tests can shorten it without changing
// the Poll signature.
var pollInterval = 2 * time.Second

// Poll returns once the user finishes the browser flow (HTTP 200) or ctx is cancelled.
// 404 means "not yet, keep polling".
func Poll(ctx context.Context, info PollInfo) (*LoginResult, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	form := url.Values{"token": []string{info.Token}}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "POST", info.Endpoint, strings.NewReader(form.Encode()))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode == 200 {
				var out LoginResult
				err := json.NewDecoder(resp.Body).Decode(&out)
				resp.Body.Close()
				if err != nil {
					return nil, err
				}
				return &out, nil
			}
			resp.Body.Close()
			if resp.StatusCode != 404 {
				return nil, fmt.Errorf("login poll: unexpected status %s", resp.Status)
			}
		}
	}
}

// Returned errors are non-fatal: callers fall back to printing the URL.
func OpenBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if cmd == nil {
		return errors.New("no opener available")
	}
	return cmd.Start()
}

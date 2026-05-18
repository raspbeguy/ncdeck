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

// LoginInit is the response from POST /index.php/login/v2.
type LoginInit struct {
	Login string   `json:"login"`
	Poll  PollInfo `json:"poll"`
}

type PollInfo struct {
	Token    string `json:"token"`
	Endpoint string `json:"endpoint"`
}

// LoginResult comes back from the poll endpoint once the user completes the flow.
type LoginResult struct {
	Server      string `json:"server"`
	LoginName   string `json:"loginName"`
	AppPassword string `json:"appPassword"`
}

// Start initiates Login Flow v2 against the given Nextcloud URL.
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
		return nil, fmt.Errorf("login init: %s — %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out LoginInit
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Poll polls the endpoint with the token until the user completes login (200) or ctx is cancelled.
// Polling cadence: every 2 seconds.
func Poll(ctx context.Context, info PollInfo) (*LoginResult, error) {
	ticker := time.NewTicker(2 * time.Second)
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

// OpenBrowser tries to open the URL in the default browser; failure is non-fatal.
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

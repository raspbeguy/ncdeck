// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
)

// glamour.WithAutoStyle() issues an OSC 11 terminal query that blocks for
// the response, slowing down TUI startup on some terminals. We pick a style
// statically from env hints instead.
func pickStyle() string {
	if v := strings.TrimSpace(os.Getenv("NCDECK_THEME")); v != "" {
		return v
	}
	if v := os.Getenv("COLORFGBG"); v != "" {
		parts := strings.Split(v, ";")
		if len(parts) >= 2 {
			switch parts[len(parts)-1] {
			case "15", "7":
				return "light"
			}
		}
	}
	return "dark"
}

// Glamour renderer construction parses styles and builds a goldmark pipeline,
// so we cache one per word-wrap width.
var (
	mdMu     sync.Mutex
	mdCache  = map[int]*glamour.TermRenderer{}
	mdStyle  = pickStyle()
)

func renderMarkdown(s string, width int) string {
	if width < 20 {
		width = 80
	}
	mdMu.Lock()
	r, ok := mdCache[width]
	if !ok {
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle(mdStyle),
			glamour.WithWordWrap(width),
			glamour.WithEmoji(),
		)
		if err != nil {
			mdMu.Unlock()
			return s
		}
		mdCache[width] = r
	}
	mdMu.Unlock()
	out, err := r.Render(s)
	if err != nil {
		return s
	}
	return out
}

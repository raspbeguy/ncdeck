// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
)

// pickStyle resolves the glamour style once, without doing an OSC 11 terminal
// query (which is what makes glamour.WithAutoStyle() slow on some terminals).
// Respect $NCDECK_THEME, then $COLORFGBG, then default to "dark".
func pickStyle() string {
	if v := strings.TrimSpace(os.Getenv("NCDECK_THEME")); v != "" {
		return v
	}
	// COLORFGBG is "fg;bg" — a small bg number means a dark background.
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

// mdRenderer is the shared glamour renderer; constructing one is non-trivial
// (parses styles, builds the goldmark pipeline), so we cache by word-wrap width.
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

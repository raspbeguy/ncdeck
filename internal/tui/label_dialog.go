// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

type labelDialog struct {
	all      []api.Label
	assigned map[int]bool // label ID -> currently on the card
	filtered []int        // indices into all
	cursor   int
	input    textinput.Model
	accent   lipgloss.Color
}

func newLabelDialog(boardLabels, cardLabels []api.Label, accent lipgloss.Color) labelDialog {
	assigned := make(map[int]bool, len(cardLabels))
	for _, l := range cardLabels {
		assigned[l.ID] = true
	}
	ti := textinput.New()
	ti.Placeholder = "filter"
	ti.Focus()
	ti.Width = 40
	d := labelDialog{
		all:      boardLabels,
		assigned: assigned,
		input:    ti,
		accent:   accent,
	}
	d.refilter()
	return d
}

func (d *labelDialog) refilter() {
	q := strings.ToLower(strings.TrimSpace(d.input.Value()))
	d.filtered = d.filtered[:0]
	for i, l := range d.all {
		if q == "" || strings.Contains(strings.ToLower(l.Title), q) {
			d.filtered = append(d.filtered, i)
		}
	}
	if d.cursor >= len(d.filtered) {
		d.cursor = 0
	}
}

func (d *labelDialog) moveCursor(delta int) {
	n := len(d.filtered)
	if n == 0 {
		d.cursor = 0
		return
	}
	d.cursor = ((d.cursor+delta)%n + n) % n
}

func (d *labelDialog) selected() *api.Label {
	if d.cursor < 0 || d.cursor >= len(d.filtered) {
		return nil
	}
	return &d.all[d.filtered[d.cursor]]
}

// toggleSelected flips the assigned state for the cursor's label and returns
// the label plus whether it was assigned before the flip (caller uses that to
// decide between Assign vs Remove on the API side).
func (d *labelDialog) toggleSelected() (*api.Label, bool) {
	l := d.selected()
	if l == nil {
		return nil, false
	}
	was := d.assigned[l.ID]
	d.assigned[l.ID] = !was
	return l, was
}

func (d labelDialog) view() string {
	title := lipgloss.NewStyle().Foreground(d.accent).Bold(true).Render("Labels")
	parts := []string{title, inputBoxStyle.Render(d.input.View()), ""}

	if len(d.filtered) == 0 {
		empty := "(no matching labels)"
		if len(d.all) == 0 {
			empty = "(this board has no labels yet)"
		}
		parts = append(parts, subtleStyle.Italic(true).Render(empty))
	} else {
		for i, idx := range d.filtered {
			l := d.all[idx]
			marker := "  "
			if i == d.cursor {
				marker = lipgloss.NewStyle().Foreground(d.accent).Bold(true).Render("▌ ")
			}
			check := "[ ]"
			if d.assigned[l.ID] {
				check = lipgloss.NewStyle().Foreground(d.accent).Bold(true).Render("[✓]")
			}
			chip := chipStyle.
				Background(lipgloss.Color("#"+l.Color)).
				Foreground(lipgloss.Color("16")).
				Render(l.Title)
			parts = append(parts, marker+check+" "+chip)
		}
	}

	parts = append(parts, "", helpStyle.Render("type to filter   ↑/↓ navigate   ⏎ toggle   esc close"))
	return modalStyle.BorderForeground(d.accent).Padding(1, 3).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

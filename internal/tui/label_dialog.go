// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

type labelAction int

const (
	labelActionNone labelAction = iota
	labelActionToggle
	labelActionCreate
)

type labelDialog struct {
	all      []api.Label
	assigned map[int]bool // label ID -> currently on the card
	filtered []int        // indices into all
	cursor   int          // 0..len(filtered)-1 selects a label, == len(filtered) selects the create entry
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
	if d.cursor >= d.totalEntries() {
		d.cursor = 0
	}
}

// canCreate returns true when the typed query is non-empty and doesn't match
// any existing label title (case-insensitive). The dialog offers a synthetic
// "+ Create" row in that case.
func (d *labelDialog) canCreate() bool {
	name := strings.TrimSpace(d.input.Value())
	if name == "" {
		return false
	}
	for _, l := range d.all {
		if strings.EqualFold(l.Title, name) {
			return false
		}
	}
	return true
}

func (d *labelDialog) totalEntries() int {
	n := len(d.filtered)
	if d.canCreate() {
		n++
	}
	return n
}

func (d *labelDialog) moveCursor(delta int) {
	n := d.totalEntries()
	if n == 0 {
		d.cursor = 0
		return
	}
	d.cursor = ((d.cursor+delta)%n + n) % n
}

// currentAction inspects the cursor position to decide what enter should do.
// The returned label is non-nil only when the action is labelActionToggle;
// the returned name is set only for labelActionCreate.
func (d *labelDialog) currentAction() (action labelAction, label *api.Label, name string) {
	if d.cursor < len(d.filtered) {
		return labelActionToggle, &d.all[d.filtered[d.cursor]], ""
	}
	if d.canCreate() {
		return labelActionCreate, nil, strings.TrimSpace(d.input.Value())
	}
	return labelActionNone, nil, ""
}

// toggleAssigned flips the assigned state for a label already in d.all and
// reports the prior value so the caller can pick Assign vs Remove.
func (d *labelDialog) toggleAssigned(id int) bool {
	was := d.assigned[id]
	d.assigned[id] = !was
	return was
}

// adoptCreated appends a freshly-created label, marks it assigned, and clears
// the filter so the user sees the new entry surface in the list.
func (d *labelDialog) adoptCreated(l api.Label) {
	d.all = append(d.all, l)
	d.assigned[l.ID] = true
	d.input.SetValue("")
	d.cursor = 0
	d.refilter()
}

func (d labelDialog) view() string {
	title := lipgloss.NewStyle().Foreground(d.accent).Bold(true).Render("Labels")
	parts := []string{title, inputBoxStyle.Render(d.input.View()), ""}

	if len(d.filtered) == 0 && !d.canCreate() {
		empty := "(no matching labels)"
		if len(d.all) == 0 {
			empty = "(this board has no labels yet, type a name to create one)"
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
		if d.canCreate() {
			marker := "  "
			if d.cursor == len(d.filtered) {
				marker = lipgloss.NewStyle().Foreground(d.accent).Bold(true).Render("▌ ")
			}
			label := lipgloss.NewStyle().Foreground(d.accent).Italic(true).
				Render("+ Create \"" + strings.TrimSpace(d.input.Value()) + "\"")
			parts = append(parts, marker+label)
		}
	}

	parts = append(parts, "", helpStyle.Render("type to filter / create   ↑/↓ navigate   ⏎ toggle or create   esc close"))
	return modalStyle.BorderForeground(d.accent).Padding(1, 3).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

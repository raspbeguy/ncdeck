// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type confirmAction int

const (
	confirmActionNone confirmAction = iota
	confirmActionYes
	confirmActionNo
)

// Default-safe: only `y`/`Y` confirms; `n`/`N`/`esc`/`⏎` all cancel.
// Plain enter must not destroy data, matching the project rule for the
// label manager's delete confirm.
type confirmDialog struct {
	prompt string
	accent lipgloss.Color
}

func newConfirmDialog(prompt string, accent lipgloss.Color) confirmDialog {
	return confirmDialog{prompt: prompt, accent: accent}
}

func (d confirmDialog) Update(km tea.KeyMsg) confirmAction {
	switch km.String() {
	case "y", "Y":
		return confirmActionYes
	case "n", "N", "esc", "enter":
		return confirmActionNo
	}
	return confirmActionNone
}

func (d confirmDialog) view() string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(colDanger).Bold(true).Render("Confirm"),
		"",
		d.prompt,
		"",
		helpStyle.Render("y confirm   n / esc / ⏎ cancel"),
	)
	return modalStyle.BorderForeground(colDanger).Padding(1, 3).Render(body)
}

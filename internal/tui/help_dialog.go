// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "github.com/charmbracelet/lipgloss"

type helpEntry struct {
	keys, action string
}

// Caller wraps the result with lipgloss.Place for screen positioning.
func renderHelp(title string, entries []helpEntry, accent lipgloss.Color) string {
	// Widen the key column to the longest entry so action labels line up.
	keyW := 0
	for _, e := range entries {
		if n := lipgloss.Width(e.keys); n > keyW {
			keyW = n
		}
	}
	keyStyle := lipgloss.NewStyle().Foreground(accent).Bold(true).Width(keyW + 2)
	actionStyle := lipgloss.NewStyle().Foreground(colText)

	lines := []string{
		lipgloss.NewStyle().Foreground(accent).Bold(true).Render(title),
		"",
	}
	for _, e := range entries {
		lines = append(lines, keyStyle.Render(e.keys)+actionStyle.Render(e.action))
	}
	lines = append(lines, "", helpStyle.Render("? or esc to close"))
	return modalStyle.BorderForeground(accent).Padding(1, 3).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

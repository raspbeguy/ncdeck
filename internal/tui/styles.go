// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// foregroundFor picks a readable foreground colour (near-black or near-white)
// based on a background colour's perceived luminance. Accepts colours in
// "#RRGGBB" form; unknown forms (8-bit terminal palette, named colours) fall
// back to black, which is safe for the warm pastel-ish palette typical Deck
// boards use.
func foregroundFor(bg lipgloss.Color) lipgloss.Color {
	s := string(bg)
	if !strings.HasPrefix(s, "#") || len(s) != 7 {
		return lipgloss.Color("16")
	}
	parse := func(hi, lo int) int {
		v, err := strconv.ParseInt(s[hi:lo], 16, 0)
		if err != nil {
			return 0
		}
		return int(v)
	}
	r, g, b := parse(1, 3), parse(3, 5), parse(5, 7)
	// Rec. 601 relative luminance; cheap and good enough for our palette.
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if lum > 140 {
		return lipgloss.Color("16") // near-black on light bg
	}
	return lipgloss.Color("230") // near-white on dark bg
}

// Color palette loosely based on the official Deck web UI.
//
// colSelected is the fallback accent when the board colour hasn't been loaded
// yet; once it is, renderStack/renderCardWithAccent take the board's own
// colour instead.
var (
	colPrimary  = lipgloss.Color("#0082c9")
	colMuted    = lipgloss.Color("240")
	colSelected = lipgloss.Color("#ffe066")
	colDanger   = lipgloss.Color("#d9534f")
	colSubtle   = lipgloss.Color("245")
	colText     = lipgloss.Color("252")
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(colPrimary).
			Padding(0, 1).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().Foreground(colSubtle)

	// stackHeaderStyle is the base for both focused and unfocused columns.
	// renderStack derives the focused variant inline by chaining .Foreground()
	// (lipgloss returns a copy on each chained call, so this does not mutate
	// the package-level value).
	stackHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(colMuted).
				Padding(0, 1).
				Bold(true)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colMuted).
			Padding(0, 1).
			Margin(0, 0, 1, 0).
			Foreground(colText)

	chipStyle = lipgloss.NewStyle().Padding(0, 1).Bold(true)

	helpStyle = lipgloss.NewStyle().Foreground(colSubtle).Italic(true)

	dueOverdueStyle = lipgloss.NewStyle().Foreground(colDanger).Bold(true)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colPrimary).
			Padding(1, 2)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colPrimary).
			Padding(0, 1)
)

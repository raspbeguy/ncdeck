// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "github.com/charmbracelet/lipgloss"

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

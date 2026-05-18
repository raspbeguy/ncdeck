// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "github.com/charmbracelet/lipgloss"

// Color palette loosely based on the official Deck web UI.
var (
	colPrimary    = lipgloss.Color("#0082c9")
	colMuted      = lipgloss.Color("240")
	colSelected   = lipgloss.Color("#ffe066")
	colDanger     = lipgloss.Color("#d9534f")
	colSubtle     = lipgloss.Color("245")
	colBackground = lipgloss.Color("236")
	colText       = lipgloss.Color("252")
)

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(colPrimary).
			Padding(0, 1).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().Foreground(colSubtle)

	stackHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(colMuted).
				Padding(0, 1).
				Bold(true)

	stackHeaderSel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("16")).
			Background(colSelected).
			Padding(0, 1).
			Bold(true)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colMuted).
			Padding(0, 1).
			Margin(0, 0, 1, 0).
			Foreground(colText)

	cardStyleSel = cardStyle.
			BorderForeground(colSelected).
			Bold(true)

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

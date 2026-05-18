// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type paletteEntry struct{ name, hex string }

// Curated palette covering the colours Deck's web UI uses most often.
var defaultPalette = []paletteEntry{
	{"red", "ff0000"},
	{"orange", "ff8800"},
	{"yellow", "ffd700"},
	{"green", "00c853"},
	{"cyan", "00bcd4"},
	{"blue", "1976d2"},
	{"purple", "8e24aa"},
	{"pink", "e91e63"},
	{"brown", "8d6e63"},
	{"gray", "888888"},
}

type colorPicker struct {
	presets    []paletteEntry
	presetIdx  int
	input      textinput.Model
	focusInput bool
	accent     lipgloss.Color
}

func newColorPicker(currentHex string, accent lipgloss.Color) colorPicker {
	ti := textinput.New()
	ti.Placeholder = "rrggbb"
	ti.CharLimit = 7
	ti.Width = 10
	cur := strings.TrimPrefix(strings.TrimSpace(currentHex), "#")
	ti.SetValue(cur)
	p := colorPicker{presets: defaultPalette, input: ti, accent: accent}
	matched := false
	for i, e := range defaultPalette {
		if strings.EqualFold(e.hex, cur) {
			p.presetIdx = i
			matched = true
			break
		}
	}
	// Custom hex: -1 + focused input together stop ⏎ from silently
	// falling back to the first preset.
	if !matched && cur != "" {
		p.presetIdx = -1
		p.focusInput = true
		p.input.Focus()
	}
	return p
}

// Used by labelManager to preserve picker state across an
// esc-back-to-name round-trip in the create flow.
func (p colorPicker) initialised() bool { return p.presets != nil }

func (p *colorPicker) movePreset(delta int) {
	n := len(p.presets)
	if n == 0 {
		return
	}
	p.presetIdx = ((p.presetIdx+delta)%n + n) % n
}

func (p *colorPicker) toggleFocus() {
	p.focusInput = !p.focusInput
	if p.focusInput {
		p.input.Focus()
	} else {
		p.input.Blur()
	}
}

// pickedColor returns the chosen hex (without #) and whether it's valid. When
// the input is focused, the typed value wins; otherwise the cursor's preset.
func (p colorPicker) pickedColor() (string, bool) {
	if p.focusInput {
		return validateHex(p.input.Value())
	}
	if p.presetIdx < 0 || p.presetIdx >= len(p.presets) {
		return "", false
	}
	return p.presets[p.presetIdx].hex, true
}

func validateHex(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 3 && len(s) != 6 {
		return "", false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return "", false
		}
	}
	if len(s) == 3 {
		s = expandHexShorthand(s)
	}
	return strings.ToLower(s), true
}

// Byte indexing assumes the caller validated three ASCII hex digits.
func expandHexShorthand(s string) string {
	return string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
}

func (p colorPicker) view() string {
	const cellsPerRow = 5
	var rows []string
	var current []string
	for i, e := range p.presets {
		swatch := lipgloss.NewStyle().
			Background(lipgloss.Color("#"+e.hex)).
			Foreground(foregroundFor(lipgloss.Color("#"+e.hex))).
			Padding(0, 1).
			Render(e.name)
		if !p.focusInput && i == p.presetIdx {
			swatch = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(p.accent).Render(swatch)
		} else {
			// Padding matches the focused-swatch border thickness so rows don't jitter.
			swatch = lipgloss.NewStyle().Padding(1, 1).Render(swatch)
		}
		current = append(current, swatch)
		if len(current) == cellsPerRow {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, current...))
			current = nil
		}
	}
	if len(current) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, current...))
	}

	inputLabel := "rgb hex (tab to focus):"
	inputBox := inputBoxStyle.Render(p.input.View())
	if p.focusInput {
		inputLabel = "rgb hex (typing):"
		inputBox = inputBoxStyle.BorderForeground(p.accent).Render(p.input.View())
	}

	hint := "tab switch palette/hex   ←/→ pick preset   ⏎ confirm   esc cancel"
	if p.focusInput {
		hint = "tab switch palette/hex   type to edit hex   ⏎ confirm   esc cancel"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		subtleStyle.Render("palette:"),
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		"",
		subtleStyle.Render(inputLabel),
		inputBox,
		"",
		helpStyle.Render(hint),
	)
}

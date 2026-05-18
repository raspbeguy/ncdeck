// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestValidateHex(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"ff0000", "ff0000", true},
		{"#FF0000", "ff0000", true},
		{"  abcdef ", "abcdef", true},
		{"f00", "ff0000", true},
		{"#f0a", "ff00aa", true},
		{"FFF", "ffffff", true},
		{"12", "", false},
		{"gggggg", "", false},
		{"", "", false},
		{"#12345", "", false},
		{"gga", "", false},
	}
	for _, tc := range cases {
		got, ok := validateHex(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("validateHex(%q): got (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestColorPicker_TabTogglesFocus(t *testing.T) {
	p := newColorPicker("ff0000", lipgloss.Color("#0082c9"))
	if p.focusInput {
		t.Errorf("focusInput should default to false when the current colour matches a preset")
	}
	p.toggleFocus()
	if !p.focusInput {
		t.Errorf("toggleFocus should switch to input focus")
	}
	p.toggleFocus()
	if p.focusInput {
		t.Errorf("toggleFocus should flip back to palette focus")
	}
}

func TestColorPicker_AutoFocusForNonPresetColor(t *testing.T) {
	p := newColorPicker("ff7f50", lipgloss.Color("#0082c9"))
	if !p.focusInput {
		t.Fatalf("a colour outside the preset palette must auto-focus the input to preserve the typed hex on ⏎")
	}
	if p.presetIdx != -1 {
		t.Errorf("presetIdx for a non-preset colour must be -1 (no swatch selected); got %d", p.presetIdx)
	}
	hex, ok := p.pickedColor()
	if !ok || hex != "ff7f50" {
		t.Errorf("pickedColor with custom colour: got (%q, %v), want (ff7f50, true)", hex, ok)
	}
}

// Pinned bug: previously, opening edit-color on a custom-hex label and pressing
// tab would land on the (default-zero) red preset; ⏎ then overwrote the
// original colour. presetIdx=-1 means no preset is selected after the tab.
func TestColorPicker_TabFromNonPresetLeavesNoPaletteSelection(t *testing.T) {
	p := newColorPicker("ff7f50", lipgloss.Color("#0082c9"))
	p.toggleFocus() // tab to palette
	if p.focusInput {
		t.Fatalf("setup: tab should have moved focus to the palette")
	}
	if _, ok := p.pickedColor(); ok {
		t.Errorf("pickedColor on palette mode with no preset selected should return ok=false")
	}
	p.movePreset(+1)
	if p.presetIdx != 0 {
		t.Errorf("first right-arrow from no selection should land on preset 0, got %d", p.presetIdx)
	}
}

func TestColorPicker_MatchingPresetLeavesInputBlurred(t *testing.T) {
	p := newColorPicker("00c853", lipgloss.Color("#0082c9"))
	if p.focusInput {
		t.Errorf("matching preset should keep palette focus")
	}
	hex, ok := p.pickedColor()
	if !ok || hex != "00c853" {
		t.Errorf("pickedColor on preset match: got (%q, %v), want (00c853, true)", hex, ok)
	}
}

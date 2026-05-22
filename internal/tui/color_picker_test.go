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

// Pinned: a custom (non-preset) hex starts the picker on the INPUT with the
// value pre-filled, so ⏎ keeps the original colour. presetIdx stays at -1
// (no swatch highlighted). pickedColor's fallback also handles a quick
// tab-to-palette + ⏎ without losing the original.
func TestColorPicker_NonPresetStartsOnInputAndPreservesHex(t *testing.T) {
	p := newColorPicker("ff7f50", lipgloss.Color("#0082c9"))
	if !p.focusInput {
		t.Errorf("non-preset colour must default to input focus")
	}
	if p.presetIdx != -1 {
		t.Errorf("non-preset colour must have presetIdx=-1 (no swatch highlighted); got %d", p.presetIdx)
	}
	hex, ok := p.pickedColor()
	if !ok || hex != "ff7f50" {
		t.Errorf("⏎ on the focused input must return the typed value; got (%q, %v), want (ff7f50, true)", hex, ok)
	}
	p.toggleFocus()
	hex, ok = p.pickedColor()
	if !ok || hex != "ff7f50" {
		t.Errorf("tab to palette with no preset must still fall back to the input value; got (%q, %v), want (ff7f50, true)", hex, ok)
	}
}

// Pinned: tabbing to the palette from a non-preset start, then arrowing,
// should land on preset 0 (not stay at -1).
func TestColorPicker_FirstArrowFromNoSelectionLandsOnPresetZero(t *testing.T) {
	p := newColorPicker("ff7f50", lipgloss.Color("#0082c9"))
	p.toggleFocus() // input-focused by default for non-preset; flip to palette
	if p.focusInput {
		t.Fatalf("setup: tab should have moved focus to the palette")
	}
	p.movePreset(+1)
	if p.presetIdx != 0 {
		t.Errorf("first right-arrow from no selection should land on preset 0, got %d", p.presetIdx)
	}
}

// Pinned: when palette is focused and a preset is highlighted, the live
// preview must reflect the preset (what ⏎ would pick), not the input's
// (possibly stale) pre-filled value. Arrowing through presets must visibly
// change the swatch.
func TestColorPicker_PreviewTracksHighlightedPresetInPaletteMode(t *testing.T) {
	p := newColorPicker("ff0000", lipgloss.Color("#0082c9"))
	if p.focusInput {
		t.Fatalf("setup: matching preset must start on palette focus")
	}
	if got := p.previewColor(); got != "ff0000" {
		t.Errorf("preview should mirror the highlighted preset (idx 0 = red); got %q", got)
	}
	p.movePreset(+1) // arrow to orange
	if got := p.previewColor(); got != "ff8800" {
		t.Errorf("preview must follow the palette cursor; got %q after right-arrow", got)
	}
}

// Pinned: palette focused with NO preset highlighted (presetIdx=-1) falls
// back to the input's validated value. This is the seam between paths 2
// and 3 of pickedColor.
func TestColorPicker_PickedColorFallsBackToInputWhenPaletteFocusedAndNoPreset(t *testing.T) {
	p := newColorPicker("ff7f50", lipgloss.Color("#0082c9"))
	p.toggleFocus() // input is the default for non-preset; flip to palette
	if p.focusInput || p.presetIdx != -1 {
		t.Fatalf("setup: palette focused with no preset selected; got focusInput=%v presetIdx=%d", p.focusInput, p.presetIdx)
	}
	hex, ok := p.pickedColor()
	if !ok || hex != "ff7f50" {
		t.Errorf("⏎ on palette with no preset must fall back to the input value; got (%q, %v), want (ff7f50, true)", hex, ok)
	}
}

// Pinned: the live preview reflects whatever's currently in the input,
// validated. Invalid/incomplete input renders the placeholder.
func TestColorPicker_PreviewTracksInput(t *testing.T) {
	p := newColorPicker("ad730d", lipgloss.Color("#0082c9"))
	if got := p.previewColor(); got != "ad730d" {
		t.Errorf("preview on fresh open should mirror the pre-filled hex; got %q", got)
	}
	p.input.SetValue("00ff00")
	if got := p.previewColor(); got != "00ff00" {
		t.Errorf("preview must update live with the typed value; got %q", got)
	}
	p.input.SetValue("00f") // 3-digit shorthand
	if got := p.previewColor(); got != "0000ff" {
		t.Errorf("3-digit shorthand should expand in the preview; got %q", got)
	}
	p.input.SetValue("zz") // invalid
	if got := p.previewColor(); got != "" {
		t.Errorf("invalid input should yield empty preview; got %q", got)
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

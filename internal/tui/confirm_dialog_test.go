// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Pinned: a destructive confirm dialog must NOT treat plain ⏎ as confirm.
// y/Y is the only "yes"; n, N, esc and enter all cancel. Reflexive enter-
// presses are the most common way users accidentally delete things.
func TestConfirmDialog_EnterIsCancelNotConfirm(t *testing.T) {
	d := newConfirmDialog("Delete X?", lipgloss.Color("#0082c9"))

	cases := []struct {
		name string
		key  tea.KeyMsg
		want confirmAction
	}{
		{"y lowercase", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}, confirmActionYes},
		{"Y uppercase", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")}, confirmActionYes},
		{"n lowercase", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}, confirmActionNo},
		{"N uppercase", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")}, confirmActionNo},
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}, confirmActionNo},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, confirmActionNo},
		{"random rune", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, confirmActionNone},
	}
	for _, tc := range cases {
		if got := d.Update(tc.key); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

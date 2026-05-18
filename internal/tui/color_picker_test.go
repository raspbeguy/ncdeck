// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import "testing"

func TestValidateHex(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"ff0000", "ff0000", true},
		{"#FF0000", "ff0000", true},
		{"  abcdef ", "abcdef", true},
		{"123", "", false},
		{"gggggg", "", false},
		{"", "", false},
		{"#12345", "", false},
	}
	for _, tc := range cases {
		got, ok := validateHex(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("validateHex(%q): got (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

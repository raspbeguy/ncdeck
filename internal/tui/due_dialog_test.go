// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// newTestDialog returns a dueDialog with deterministic starting state so the
// tests below don't depend on time.Now.
func newTestDialog() dueDialog {
	return dueDialog{
		year: 2026, month: 5, day: 18, hour: 10, minute: 0,
		accent: lipgloss.Color("#0082c9"),
	}
}

func TestTypeDigit_BuildsValueLeftToRight(t *testing.T) {
	d := newTestDialog()
	d.focus = 0 // year
	d.typeDigit('2')
	d.typeDigit('0')
	d.typeDigit('2')
	d.typeDigit('7')
	// 4-digit year field auto-advances on the 4th digit.
	if d.year != 2027 {
		t.Errorf("year: got %d, want 2027", d.year)
	}
	if d.focus != 1 {
		t.Errorf("focus: got %d, want 1 (auto-advanced to month)", d.focus)
	}
	if d.buf != "" {
		t.Errorf("buf: got %q, want \"\" (cleared on auto-advance)", d.buf)
	}
}

func TestTypeDigit_OverflowReplacesBuffer(t *testing.T) {
	d := newTestDialog()
	d.focus = 3 // hour, 0..23
	d.typeDigit('2')
	if d.hour != 2 || d.buf != "2" {
		t.Errorf("after '2': hour=%d buf=%q, want 2 / \"2\"", d.hour, d.buf)
	}
	d.typeDigit('5') // 25 > 23, must roll back to just "5"
	if d.hour != 5 || d.buf != "5" {
		t.Errorf("after '25' overflow: hour=%d buf=%q, want 5 / \"5\"", d.hour, d.buf)
	}
}

func TestAdjust_ClearsTypingBuffer(t *testing.T) {
	// Reproducer for the bug fixed in this round: typing then nudging should
	// not concatenate the next digit onto a stale buffer.
	d := newTestDialog()
	d.focus = 4 // minute
	d.typeDigit('5')
	if d.minute != 5 || d.buf != "5" {
		t.Fatalf("setup: minute=%d buf=%q", d.minute, d.buf)
	}
	d.adjust(+1)
	if d.minute != 6 {
		t.Errorf("after adjust: minute=%d, want 6", d.minute)
	}
	if d.buf != "" {
		t.Errorf("buf: got %q, want \"\" (cleared on adjust)", d.buf)
	}
	d.typeDigit('9')
	if d.minute != 9 || d.buf != "9" {
		t.Errorf("after type-9: minute=%d buf=%q, want 9 / \"9\" (not 69)", d.minute, d.buf)
	}
}

func TestCommit_ClampsYearMonthDay(t *testing.T) {
	d := newTestDialog()
	// Simulate transient typed-zero state.
	d.year, d.month, d.day = 0, 0, 0
	d.buf = "0"
	d.commit()
	if d.year != 1 {
		t.Errorf("year after commit: got %d, want 1", d.year)
	}
	if d.month != 1 {
		t.Errorf("month after commit: got %d, want 1", d.month)
	}
	if d.day != 1 {
		t.Errorf("day after commit: got %d, want 1", d.day)
	}
	if d.buf != "" {
		t.Errorf("buf after commit: got %q, want \"\"", d.buf)
	}
}

func TestDaysInMonth_DefensiveForOutOfRangeMonth(t *testing.T) {
	d := newTestDialog()
	cases := []struct {
		month int
		want  int
	}{
		{1, 31}, {2, 28}, {3, 31}, {4, 30}, {12, 31},
		{0, 31},  // transient typed state
		{13, 31}, // shouldn't happen but defensive
	}
	for _, tc := range cases {
		d.month = tc.month
		got := d.daysInMonth()
		if got != tc.want {
			t.Errorf("month=%d: got %d days, want %d", tc.month, got, tc.want)
		}
	}
}

func TestDaysInMonth_LeapYear(t *testing.T) {
	d := newTestDialog()
	d.month = 2
	d.year = 2024
	if got := d.daysInMonth(); got != 29 {
		t.Errorf("Feb 2024: got %d days, want 29", got)
	}
	d.year = 2026
	if got := d.daysInMonth(); got != 28 {
		t.Errorf("Feb 2026: got %d days, want 28", got)
	}
}

func TestClampDay_ShrinksOnShorterMonth(t *testing.T) {
	d := newTestDialog()
	d.month = 1
	d.day = 31
	d.month = 2
	d.clampDay()
	if d.day != 28 && d.day != 29 {
		t.Errorf("day after switching Jan31 -> Feb: got %d, want 28 or 29", d.day)
	}
}

func TestBackspace_ShortensBufferAndValue(t *testing.T) {
	d := newTestDialog()
	d.focus = 0
	d.typeDigit('2')
	d.typeDigit('0')
	d.typeDigit('2')
	// buf="202", year=202 (still typing, no auto-advance yet)
	if d.year != 202 || d.buf != "202" {
		t.Fatalf("setup: year=%d buf=%q", d.year, d.buf)
	}
	d.backspace()
	if d.year != 20 || d.buf != "20" {
		t.Errorf("after backspace: year=%d buf=%q, want 20 / \"20\"", d.year, d.buf)
	}
	d.backspace()
	d.backspace()
	if d.year != 0 || d.buf != "" {
		t.Errorf("after 3 backspaces: year=%d buf=%q, want 0 / \"\"", d.year, d.buf)
	}
	// commit must rescue from year=0.
	d.commit()
	if d.year != 1 {
		t.Errorf("year after commit-rescue: got %d, want 1", d.year)
	}
}

func TestMoveFocus_CommitsBeforeMoving(t *testing.T) {
	d := newTestDialog()
	d.focus = 1   // month
	d.month = 0   // transient typing state
	d.buf = "0"
	d.moveFocus(+1)
	if d.focus != 2 {
		t.Errorf("focus: got %d, want 2", d.focus)
	}
	if d.month != 1 {
		t.Errorf("month after focus move: got %d, want 1 (clamped by commit)", d.month)
	}
	if d.buf != "" {
		t.Errorf("buf: got %q, want \"\"", d.buf)
	}
}

func TestRfc3339_RoundTripsLocalTime(t *testing.T) {
	d := newTestDialog()
	d.year, d.month, d.day = 2026, 12, 31
	d.hour, d.minute = 14, 30
	out := d.rfc3339()
	// We don't pin the offset (depends on host TZ) but the local-time fields
	// should appear verbatim and the year/month/day separators should be -.
	wantPrefix := "2026-12-31T14:30:00"
	if got := out[:len(wantPrefix)]; got != wantPrefix {
		t.Errorf("rfc3339: %q, want prefix %q", out, wantPrefix)
	}
}

func TestForegroundFor_PicksReadableColour(t *testing.T) {
	cases := []struct {
		bg   string
		want string
	}{
		{"#ffffff", "16"},   // white bg -> black fg
		{"#000000", "230"},  // black bg -> near-white fg
		{"#ffe066", "16"},   // pastel yellow -> black
		{"#0082c9", "230"},  // Deck primary blue -> near-white
		{"badinput", "16"},  // unparseable -> safe default
	}
	for _, tc := range cases {
		got := foregroundFor(lipgloss.Color(tc.bg))
		if string(got) != tc.want {
			t.Errorf("bg=%s: got fg=%s, want %s", tc.bg, got, tc.want)
		}
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// dueDialog is the foreground modal used to pick a due date+time. Each of the
// five fields (year, month, day, hour, minute) is edited independently with
// arrow keys or by typing digits directly. Day is automatically clamped to the
// chosen month's length.
type dueDialog struct {
	year, month, day int
	hour, minute     int
	focus            int    // 0=year, 1=month, 2=day, 3=hour, 4=minute
	buf              string // digits typed in the current field, reset on focus move
	accent           lipgloss.Color
}

// newDueDialog seeds the dialog from a card's existing due date (RFC3339) or,
// if none, from the current local time rounded to the next hour at minute 0.
func newDueDialog(current *string, accent lipgloss.Color) dueDialog {
	d := dueDialog{accent: accent}
	if current != nil && *current != "" {
		if t, err := time.Parse(time.RFC3339, *current); err == nil {
			t = t.Local()
			d.year, d.month, d.day = t.Year(), int(t.Month()), t.Day()
			d.hour, d.minute = t.Hour(), t.Minute()
			return d
		}
	}
	now := time.Now().Add(time.Hour).Local()
	d.year, d.month, d.day = now.Year(), int(now.Month()), now.Day()
	d.hour, d.minute = now.Hour(), 0
	return d
}

// daysInMonth returns the number of days in the dialog's current month/year,
// handling leap years via the standard library.
func (d dueDialog) daysInMonth() int {
	// Day 0 of (month+1) is the last day of month.
	return time.Date(d.year, time.Month(d.month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

// clampDay shrinks the day field if it now exceeds the days in the selected
// month (e.g. Jan 31 -> Feb gets clamped to Feb 28/29).
func (d *dueDialog) clampDay() {
	max := d.daysInMonth()
	if d.day > max {
		d.day = max
	}
	if d.day < 1 {
		d.day = 1
	}
}

// adjust nudges the focused field by delta, wrapping the value to its valid
// range. Day wraps within the current month, month wraps 1..12, hour wraps
// 0..23, minute wraps 0..59. Year is unbounded but kept >= 1900.
func (d *dueDialog) adjust(delta int) {
	switch d.focus {
	case 0:
		d.year += delta
		if d.year < 1900 {
			d.year = 1900
		}
		d.clampDay()
	case 1:
		d.month = wrap(d.month-1+delta, 12) + 1
		d.clampDay()
	case 2:
		d.day = wrap(d.day-1+delta, d.daysInMonth()) + 1
	case 3:
		d.hour = wrap(d.hour+delta, 24)
	case 4:
		d.minute = wrap(d.minute+delta, 60)
	}
}

func wrap(v, mod int) int {
	v %= mod
	if v < 0 {
		v += mod
	}
	return v
}

// moveFocus shifts the focus by delta (left/right), clamped to 0..4. The
// current field is committed first so partial typed values like "0" in the
// month field get normalised to "1".
func (d *dueDialog) moveFocus(delta int) {
	d.commit()
	d.focus += delta
	if d.focus < 0 {
		d.focus = 0
	}
	if d.focus > 4 {
		d.focus = 4
	}
}

// commit clears any pending typed digits and snaps month/day to at least 1
// (they can transiently be 0 during typing).
func (d *dueDialog) commit() {
	d.buf = ""
	if d.month < 1 {
		d.month = 1
	}
	if d.day < 1 {
		d.day = 1
	}
	d.clampDay()
}

// fieldMaxDigits returns how many digits the focused field can hold.
func (d *dueDialog) fieldMaxDigits(i int) int {
	if i == 0 {
		return 4
	}
	return 2
}

// typeDigit handles a "0".."9" keypress on the focused field. It builds up
// a typed value digit-by-digit (replacing the field's display), rejects
// values that overflow the field's range, and auto-advances to the next
// field once the current one is full.
func (d *dueDialog) typeDigit(c rune) {
	if c < '0' || c > '9' {
		return
	}
	digit := string(c)
	max := d.fieldMaxDigits(d.focus)

	// Append (or replace if the buffer is already full).
	if len(d.buf) >= max {
		d.buf = digit
	} else {
		d.buf = d.buf + digit
	}
	v, _ := strconv.Atoi(d.buf)

	// If accumulating the digits overflows the field's range, throw away
	// what we had and use just the new digit. Example: hour=14 + "5" tried as
	// 145 -> rejected -> reset to 5.
	if !d.acceptValue(v) {
		d.buf = digit
		_ = d.acceptValue(int(c - '0')) // single digit 0..9 always fits
	}

	// Auto-advance once a field is full so users can fluently type
	// "20271231 1430" to set everything.
	if len(d.buf) >= max && d.focus < 4 {
		d.moveFocus(+1)
	}
}

// acceptValue tries to set the focused field to v. Returns false if v is out
// of range for that field (caller can decide to retry with a smaller value).
func (d *dueDialog) acceptValue(v int) bool {
	switch d.focus {
	case 0:
		d.year = v
		d.clampDay()
		return true
	case 1:
		if v > 12 {
			return false
		}
		d.month = v
		if v >= 1 {
			d.clampDay()
		}
		return true
	case 2:
		if v > d.daysInMonth() {
			return false
		}
		d.day = v
		return true
	case 3:
		if v > 23 {
			return false
		}
		d.hour = v
		return true
	case 4:
		if v > 59 {
			return false
		}
		d.minute = v
		return true
	}
	return false
}

// backspace removes the last typed digit from the focused field. With no
// buffered digits it leaves the value alone.
func (d *dueDialog) backspace() {
	if d.buf == "" {
		return
	}
	d.buf = d.buf[:len(d.buf)-1]
	v := 0
	if d.buf != "" {
		v, _ = strconv.Atoi(d.buf)
	}
	d.acceptValue(v)
}

// rfc3339 formats the dialog's date+time as an RFC3339 string in the user's
// local timezone, ready to send to the Deck API.
func (d dueDialog) rfc3339() string {
	t := time.Date(d.year, time.Month(d.month), d.day, d.hour, d.minute, 0, 0, time.Local)
	return t.Format(time.RFC3339)
}

// view renders the dialog as a centered modal block. The caller is expected to
// place it on top of the screen with lipgloss.Place.
func (d dueDialog) view() string {
	// "mon" / "min" keep every label inside its 2-digit field's 4-char cell.
	labels := [5]string{"year", "mon", "day", "hr", "min"}

	field := lipgloss.NewStyle().Padding(0, 1).Bold(true)
	focused := field.
		Foreground(lipgloss.Color("16")).
		Background(d.accent)

	display := func(i int) string {
		max := d.fieldMaxDigits(i)
		// When the user is typing into this field, show the typed digits
		// right-aligned within the field width so partial input doesn't
		// look like leading zeros.
		if d.focus == i && d.buf != "" {
			return strings.Repeat(" ", max-len(d.buf)) + d.buf
		}
		// Special case: month/day can transiently be 0 during typing.
		var v int
		switch i {
		case 0:
			return fmt.Sprintf("%04d", d.year)
		case 1:
			v = d.month
		case 2:
			v = d.day
		case 3:
			v = d.hour
		case 4:
			v = d.minute
		}
		return fmt.Sprintf("%02d", v)
	}
	cell := func(i int) string {
		if d.focus == i {
			return focused.Render(display(i))
		}
		return field.Render(display(i))
	}
	label := func(i int) string {
		s := lipgloss.NewStyle().Foreground(colSubtle).Width(lipgloss.Width(cell(i))).Align(lipgloss.Center)
		if d.focus == i {
			s = s.Foreground(d.accent).Bold(true)
		}
		return s.Render(labels[i])
	}

	sep := lipgloss.NewStyle().Foreground(colSubtle).Render(" / ")
	colon := lipgloss.NewStyle().Foreground(colSubtle).Render(" : ")

	date := lipgloss.JoinHorizontal(lipgloss.Top, cell(0), sep, cell(1), sep, cell(2))
	dateLab := lipgloss.JoinHorizontal(lipgloss.Top, label(0), "   ", label(1), "   ", label(2))
	timeRow := lipgloss.JoinHorizontal(lipgloss.Top, cell(3), colon, cell(4))
	timeLab := lipgloss.JoinHorizontal(lipgloss.Top, label(3), "   ", label(4))

	body := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Foreground(d.accent).Bold(true).Render("Due date"),
		"",
		date,
		dateLab,
		"",
		timeRow,
		timeLab,
		"",
		helpStyle.Render("←/→ field   ↑/↓ value   type digits to set   ⏎ save   c clear   esc cancel"),
	)

	return modalStyle.BorderForeground(d.accent).Padding(1, 3).Render(body)
}

// placeModal centers the modal content inside the available area. Bubble Tea
// has no real overlay primitive, so the rest of the screen is replaced by
// blank space for the duration of the dialog.
func placeModal(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

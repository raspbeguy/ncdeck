// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type dueDialog struct {
	year, month, day int
	hour, minute     int
	focus            int // 0=year, 1=month, 2=day, 3=hour, 4=minute
	buf              string
	accent           lipgloss.Color
}

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

// Returns 31 when month is out of range so callers don't accidentally clamp
// day to a smaller value during transient typing states.
func (d dueDialog) daysInMonth() int {
	if d.month < 1 || d.month > 12 {
		return 31
	}
	return time.Date(d.year, time.Month(d.month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

func (d *dueDialog) clampDay() {
	max := d.daysInMonth()
	if d.day > max {
		d.day = max
	}
	if d.day < 1 {
		d.day = 1
	}
}

// Clears buf so a subsequent digit doesn't get concatenated onto the
// pre-nudge typed value.
func (d *dueDialog) adjust(delta int) {
	d.buf = ""
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

// Year=0 would serialise as "0000-01-01T..." which Deck rejects; month/day
// can transiently be 0 mid-typing.
func (d *dueDialog) commit() {
	d.buf = ""
	if d.year < 1 {
		d.year = 1
	}
	if d.month < 1 {
		d.month = 1
	}
	if d.day < 1 {
		d.day = 1
	}
	d.clampDay()
}

func (d *dueDialog) fieldMaxDigits(i int) int {
	if i == 0 {
		return 4
	}
	return 2
}

func (d *dueDialog) typeDigit(c rune) {
	if c < '0' || c > '9' {
		return
	}
	digit := string(c)
	max := d.fieldMaxDigits(d.focus)

	if len(d.buf) >= max {
		d.buf = digit
	} else {
		d.buf = d.buf + digit
	}
	v, _ := strconv.Atoi(d.buf)

	// Concatenated value out of range, start over with just the new digit
	// (e.g. hour=2 then '5' tried as 25 -> rejected -> hour=5).
	if !d.acceptValue(v) {
		d.buf = digit
		_ = d.acceptValue(int(c - '0'))
	}

	if len(d.buf) >= max && d.focus < 4 {
		d.moveFocus(+1)
	}
}

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

func (d dueDialog) rfc3339() string {
	t := time.Date(d.year, time.Month(d.month), d.day, d.hour, d.minute, 0, 0, time.Local)
	return t.Format(time.RFC3339)
}

// While typing, the buffer is shown right-aligned so partial input doesn't
// look like leading zeros.
func (d dueDialog) fieldDisplay(i int) string {
	max := d.fieldMaxDigits(i)
	if d.focus == i && d.buf != "" {
		return strings.Repeat(" ", max-len(d.buf)) + d.buf
	}
	switch i {
	case 0:
		return fmt.Sprintf("%04d", d.year)
	case 1:
		return fmt.Sprintf("%02d", d.month)
	case 2:
		return fmt.Sprintf("%02d", d.day)
	case 3:
		return fmt.Sprintf("%02d", d.hour)
	case 4:
		return fmt.Sprintf("%02d", d.minute)
	}
	return ""
}

// "mon" / "min" keep every label inside its 2-digit field's 4-char cell.
var dueDialogLabels = [5]string{"year", "mon", "day", "hr", "min"}

func (d dueDialog) view() string {
	field := lipgloss.NewStyle().Padding(0, 1).Bold(true)
	focused := field.
		Foreground(foregroundFor(d.accent)).
		Background(d.accent)

	var cells [5]string
	var widths [5]int
	for i := 0; i < 5; i++ {
		text := d.fieldDisplay(i)
		if d.focus == i {
			cells[i] = focused.Render(text)
		} else {
			cells[i] = field.Render(text)
		}
		widths[i] = lipgloss.Width(cells[i])
	}

	label := func(i int) string {
		s := lipgloss.NewStyle().Foreground(colSubtle).Width(widths[i]).Align(lipgloss.Center)
		if d.focus == i {
			s = s.Foreground(d.accent).Bold(true)
		}
		return s.Render(dueDialogLabels[i])
	}

	sep := lipgloss.NewStyle().Foreground(colSubtle).Render(" / ")
	colon := lipgloss.NewStyle().Foreground(colSubtle).Render(" : ")

	date := lipgloss.JoinHorizontal(lipgloss.Top, cells[0], sep, cells[1], sep, cells[2])
	dateLab := lipgloss.JoinHorizontal(lipgloss.Top, label(0), "   ", label(1), "   ", label(2))
	timeRow := lipgloss.JoinHorizontal(lipgloss.Top, cells[3], colon, cells[4])
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
		helpStyle.Render("←/→ field   ↑/↓ value   0-9 type   ⌫ erase   ⏎ save   c clear   esc cancel"),
	)

	return modalStyle.BorderForeground(d.accent).Padding(1, 3).Render(body)
}

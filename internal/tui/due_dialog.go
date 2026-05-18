// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// dueDialog is the foreground modal used to pick a due date+time. Each of the
// five fields (year, month, day, hour, minute) is edited independently with
// arrow keys. Day is automatically clamped to the chosen month's length.
type dueDialog struct {
	year, month, day int
	hour, minute     int
	focus            int // 0=year, 1=month, 2=day, 3=hour, 4=minute
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

// moveFocus shifts the focus by delta (left/right), clamped to 0..4.
func (d *dueDialog) moveFocus(delta int) {
	d.focus += delta
	if d.focus < 0 {
		d.focus = 0
	}
	if d.focus > 4 {
		d.focus = 4
	}
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
	values := [5]string{
		fmt.Sprintf("%04d", d.year),
		fmt.Sprintf("%02d", d.month),
		fmt.Sprintf("%02d", d.day),
		fmt.Sprintf("%02d", d.hour),
		fmt.Sprintf("%02d", d.minute),
	}
	labels := [5]string{"year", "month", "day", "hour", "min"}

	field := lipgloss.NewStyle().Padding(0, 1).Bold(true)
	focused := field.
		Foreground(lipgloss.Color("16")).
		Background(d.accent)

	cell := func(i int) string {
		if d.focus == i {
			return focused.Render(values[i])
		}
		return field.Render(values[i])
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
		helpStyle.Render("←/→ field   ↑/↓ value   ⏎ save   c clear   esc cancel"),
	)

	return modalStyle.BorderForeground(d.accent).Padding(1, 3).Render(body)
}

// placeModal centers the modal content inside the available area. Bubble Tea
// has no real overlay primitive, so the rest of the screen is replaced by
// blank space for the duration of the dialog.
func placeModal(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

type lmgrMode int

const (
	lmgrModeList lmgrMode = iota
	lmgrModeName       // typing a name (create or rename)
	lmgrModeColor      // picking a color (create after name, or recolor)
	lmgrModeConfirmDel // confirm before delete
)

// labelManagerIntent disambiguates lmgrModeName / lmgrModeColor between the
// new-label flow (need both fields, persist when both filled) and the edit
// flow (one field at a time, persist immediately).
type labelManagerIntent int

const (
	lmgrIntentNone labelManagerIntent = iota
	lmgrIntentCreate
	lmgrIntentEditName
	lmgrIntentEditColor
)

type labelManager struct {
	all     []api.Label
	filter  map[int]bool
	cursor  int
	mode    lmgrMode
	intent  labelManagerIntent
	accent  lipgloss.Color
	nameIn  textinput.Model
	picker  colorPicker
	// pendingName / pendingColor carry user input across the create flow's
	// two sub-dialogs (name -> color) and back to the caller for the API call.
	pendingName  string
	pendingColor string
}

type lmgrAction int

const (
	lmgrActionNone lmgrAction = iota
	lmgrActionClose
	lmgrActionCreate
	lmgrActionUpdateName
	lmgrActionUpdateColor
	lmgrActionDelete
)

func newLabelManager(labels []api.Label, filter map[int]bool, accent lipgloss.Color) labelManager {
	if filter == nil {
		filter = make(map[int]bool)
	}
	return labelManager{all: labels, filter: filter, accent: accent}
}

func (m *labelManager) current() *api.Label {
	if m.cursor < 0 || m.cursor >= len(m.all) {
		return nil
	}
	return &m.all[m.cursor]
}

func (m *labelManager) moveCursor(delta int) {
	n := len(m.all)
	if n == 0 {
		m.cursor = 0
		return
	}
	m.cursor = ((m.cursor+delta)%n + n) % n
}

func (m *labelManager) toggleFilter() {
	l := m.current()
	if l == nil {
		return
	}
	if m.filter[l.ID] {
		delete(m.filter, l.ID)
	} else {
		m.filter[l.ID] = true
	}
}

// beginCreate primes the name input for a fresh label.
func (m *labelManager) beginCreate() {
	m.mode = lmgrModeName
	m.intent = lmgrIntentCreate
	m.pendingName = ""
	m.pendingColor = ""
	m.picker = colorPicker{}
	m.nameIn = newNameInput("new label name", "")
}

// beginEditName primes the name input to rename the focused label.
func (m *labelManager) beginEditName() {
	l := m.current()
	if l == nil {
		return
	}
	m.mode = lmgrModeName
	m.intent = lmgrIntentEditName
	m.nameIn = newNameInput("rename label", l.Title)
}

// beginEditColor primes the color picker for the focused label.
func (m *labelManager) beginEditColor() {
	l := m.current()
	if l == nil {
		return
	}
	m.mode = lmgrModeColor
	m.intent = lmgrIntentEditColor
	m.picker = newColorPicker(l.Color, m.accent)
}

func (m *labelManager) beginConfirmDelete() {
	if m.current() == nil {
		return
	}
	m.mode = lmgrModeConfirmDel
}

// onLabelCreated finalises the new-label flow once the server confirms.
func (m *labelManager) onLabelCreated(l api.Label) {
	m.all = append(m.all, l)
	m.cursor = len(m.all) - 1
	m.resetSubdialog()
}

func (m *labelManager) onLabelUpdated(l api.Label) {
	for i := range m.all {
		if m.all[i].ID == l.ID {
			m.all[i] = l
			break
		}
	}
	m.resetSubdialog()
}

func (m *labelManager) onLabelDeleted(id int) {
	for i := range m.all {
		if m.all[i].ID == id {
			m.all = append(m.all[:i], m.all[i+1:]...)
			break
		}
	}
	delete(m.filter, id)
	if m.cursor >= len(m.all) {
		m.cursor = len(m.all) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
	m.resetSubdialog()
}

// onLabelOpFailed restores the list mode without applying changes; caller
// surfaces the error message separately.
func (m *labelManager) onLabelOpFailed() { m.resetSubdialog() }

func (m *labelManager) resetSubdialog() {
	m.mode = lmgrModeList
	m.intent = lmgrIntentNone
	m.pendingName = ""
	m.pendingColor = ""
	m.picker = colorPicker{}
}

func newNameInput(placeholder, initial string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(initial)
	ti.CursorEnd()
	ti.Focus()
	ti.Width = 40
	return ti
}

// Update processes a key event in whichever sub-mode the manager is in.
// It only mutates the manager's own state; the caller fires any side-effect
// based on the returned action.
func (m *labelManager) Update(km tea.KeyMsg) lmgrAction {
	switch m.mode {
	case lmgrModeName:
		return m.updateName(km)
	case lmgrModeColor:
		return m.updateColor(km)
	case lmgrModeConfirmDel:
		switch km.String() {
		case "y", "Y":
			return lmgrActionDelete
		case "n", "N", "esc":
			m.resetSubdialog()
		}
		return lmgrActionNone
	}
	return m.updateList(km)
}

func (m *labelManager) updateList(km tea.KeyMsg) lmgrAction {
	switch km.String() {
	case "esc":
		return lmgrActionClose
	case "j", "down":
		m.moveCursor(+1)
	case "k", "up":
		m.moveCursor(-1)
	case " ", "enter":
		m.toggleFilter()
	case "n":
		m.beginCreate()
	case "e":
		m.beginEditName()
	case "c":
		m.beginEditColor()
	case "x":
		m.beginConfirmDelete()
	}
	return lmgrActionNone
}

func (m *labelManager) updateName(km tea.KeyMsg) lmgrAction {
	switch km.String() {
	case "esc":
		m.resetSubdialog()
		return lmgrActionNone
	case "enter":
		v := strings.TrimSpace(m.nameIn.Value())
		if v == "" {
			m.resetSubdialog()
			return lmgrActionNone
		}
		m.pendingName = v
		switch m.intent {
		case lmgrIntentCreate:
			// Init only on the first transition so esc-back-to-name
			// and re-advance preserves any typed colour.
			m.mode = lmgrModeColor
			if !m.picker.initialised() {
				m.picker = newColorPicker(newLabelDefaultColor, m.accent)
			}
			return lmgrActionNone
		case lmgrIntentEditName:
			return lmgrActionUpdateName
		}
		return lmgrActionNone
	}
	var cmd tea.Cmd
	m.nameIn, cmd = m.nameIn.Update(km)
	_ = cmd
	return lmgrActionNone
}

func (m *labelManager) updateColor(km tea.KeyMsg) lmgrAction {
	// left/right with input focused fall out of the switch so textinput.Update below moves the cursor.
	switch km.String() {
	case "esc":
		// In create flow esc pops back to name so the typed name isn't lost.
		if m.intent == lmgrIntentCreate {
			m.mode = lmgrModeName
			return lmgrActionNone
		}
		m.resetSubdialog()
		return lmgrActionNone
	case "tab":
		m.picker.toggleFocus()
		return lmgrActionNone
	case "left":
		if !m.picker.focusInput {
			m.picker.movePreset(-1)
			return lmgrActionNone
		}
	case "right":
		if !m.picker.focusInput {
			m.picker.movePreset(+1)
			return lmgrActionNone
		}
	case "enter":
		hex, ok := m.picker.pickedColor()
		if !ok {
			return lmgrActionNone
		}
		m.pendingColor = hex
		switch m.intent {
		case lmgrIntentCreate:
			return lmgrActionCreate
		case lmgrIntentEditColor:
			return lmgrActionUpdateColor
		}
		return lmgrActionNone
	}
	if m.picker.focusInput {
		var cmd tea.Cmd
		m.picker.input, cmd = m.picker.input.Update(km)
		_ = cmd
	}
	return lmgrActionNone
}

func (m labelManager) view() string {
	switch m.mode {
	case lmgrModeName:
		title := "New label"
		if m.intent == lmgrIntentEditName {
			title = "Rename label"
		}
		body := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(m.accent).Bold(true).Render(title),
			"",
			inputBoxStyle.Render(m.nameIn.View()),
			"",
			helpStyle.Render("⏎ next   esc cancel"),
		)
		return modalStyle.BorderForeground(m.accent).Padding(1, 3).Render(body)
	case lmgrModeColor:
		title := "Pick a colour"
		if m.intent == lmgrIntentCreate {
			title = fmt.Sprintf("Colour for %q", m.pendingName)
		}
		body := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(m.accent).Bold(true).Render(title),
			"",
			m.picker.view(),
		)
		return modalStyle.BorderForeground(m.accent).Padding(1, 3).Render(body)
	case lmgrModeConfirmDel:
		l := m.current()
		var name string
		if l != nil {
			name = l.Title
		}
		body := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colDanger).Bold(true).Render("Delete label?"),
			"",
			fmt.Sprintf("This will remove %q from every card on the board.", name),
			"",
			helpStyle.Render("y confirm   n or esc cancel"),
		)
		return modalStyle.BorderForeground(colDanger).Padding(1, 3).Render(body)
	}
	return m.viewList()
}

func (m labelManager) viewList() string {
	title := lipgloss.NewStyle().Foreground(m.accent).Bold(true).Render("Manage labels")
	parts := []string{title, ""}

	if filtered := len(m.filter); filtered > 0 {
		noun := "label"
		if filtered != 1 {
			noun = "labels"
		}
		parts = append(parts, subtleStyle.Italic(true).Render(fmt.Sprintf("Filter active: %d %s. space toggles the focused label.", filtered, noun)))
		parts = append(parts, "")
	}

	if len(m.all) == 0 {
		parts = append(parts, subtleStyle.Italic(true).Render("(no labels yet, n to create one)"))
	} else {
		for i, l := range m.all {
			marker := "  "
			if i == m.cursor {
				marker = lipgloss.NewStyle().Foreground(m.accent).Bold(true).Render("▌ ")
			}
			check := "[ ]"
			if m.filter[l.ID] {
				check = lipgloss.NewStyle().Foreground(m.accent).Bold(true).Render("[F]")
			}
			chip := chipStyle.
				Background(lipgloss.Color("#"+l.Color)).
				Foreground(foregroundFor(lipgloss.Color("#"+l.Color))).
				Render(l.Title)
			parts = append(parts, marker+check+" "+chip)
		}
	}

	parts = append(parts, "", helpStyle.Render("↑/↓ navigate   space filter   n new   e rename   c color   x delete   esc close"))
	return modalStyle.BorderForeground(m.accent).Padding(1, 3).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

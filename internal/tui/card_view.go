// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

type cardMode int

const (
	cardModeView cardMode = iota
	cardModeEditDescription
	cardModeAddComment
	cardModeEditDue
	cardModeEditLabels
)

// modalStyle's rounded border (1 each side) + Padding(1, 2) total. Verified
// with lipgloss: a 1x1 cell renders as 7x5 wrapped.
const (
	cardModalPadH = 6
	cardModalPadV = 4
	// Inset inside the viewport so the glamour-rendered markdown has 1 col
	// of breathing room each side.
	cardBodyInset = 2
)

type cardModel struct {
	boardID  int
	stackID  int
	cardID   int
	card     *api.Card
	comments []api.Comment
	attach   []api.Attachment

	vp       viewport.Model
	vpInit   bool
	editor   textarea.Model
	commentI textinput.Model
	due      dueDialog
	labels   labelDialog
	mode     cardMode
}

func (m *cardModel) setCard(c *api.Card, w, h int) {
	if c == nil {
		return
	}
	m.stackID = c.StackID
	m.cardID = c.ID
	m.card = c
	if !m.vpInit {
		m.vp = viewport.New(w-cardModalPadH, h-cardModalPadV)
		m.vpInit = true
	} else {
		m.vp.Width = w - cardModalPadH
		m.vp.Height = h - cardModalPadV
	}
	m.refreshBody()
}

func (m *cardModel) resize(w, h int) {
	m.vp.Width = w - cardModalPadH
	m.vp.Height = h - cardModalPadV
	if m.editor.Width() > 0 {
		m.editor.SetWidth(m.vp.Width)
		m.editor.SetHeight(m.vp.Height - 2)
	}
	if m.commentI.Width > 0 {
		m.commentI.Width = m.vp.Width
	}
	if m.card != nil {
		m.refreshBody()
	}
}

func (m *cardModel) setComments(cs []api.Comment) {
	m.comments = cs
	m.refreshBody()
}

func (m *cardModel) setAttachments(as []api.Attachment) {
	m.attach = as
	m.refreshBody()
}

func (m *cardModel) refreshBody() {
	if m.card == nil {
		return
	}
	var b strings.Builder

	fmt.Fprintln(&b, lipgloss.NewStyle().Bold(true).Underline(true).Render(m.card.Title))
	if m.card.Archived {
		fmt.Fprintln(&b, chipStyle.Background(colDanger).Foreground(lipgloss.Color("230")).Render("ARCHIVED"))
	}
	if m.card.DueDate != nil && *m.card.DueDate != "" {
		raw := *m.card.DueDate
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			line := "Due: " + t.Format("2006-01-02 15:04")
			if t.Before(time.Now()) {
				line = dueOverdueStyle.Render(line + "  (overdue)")
			}
			fmt.Fprintln(&b, line)
		} else {
			fmt.Fprintln(&b, "Due: "+raw)
		}
	}
	if len(m.card.Labels) > 0 {
		chips := make([]string, 0, len(m.card.Labels))
		for _, l := range m.card.Labels {
			chips = append(chips, chipStyle.Background(lipgloss.Color("#"+l.Color)).Foreground(lipgloss.Color("16")).Render(l.Title))
		}
		fmt.Fprintln(&b, "Labels: "+lipgloss.JoinHorizontal(lipgloss.Top, chips...))
	}
	if len(m.card.AssignedUsers) > 0 {
		names := make([]string, 0, len(m.card.AssignedUsers))
		for _, a := range m.card.AssignedUsers {
			names = append(names, "@"+a.UID)
		}
		fmt.Fprintln(&b, "Assignees: "+strings.Join(names, ", "))
	}
	b.WriteString("\n")

	desc := strings.TrimSpace(m.card.Description)
	if desc == "" {
		desc = "_(no description)_"
	}
	b.WriteString(renderMarkdown(desc, m.vp.Width-cardBodyInset))

	if len(m.attach) > 0 {
		b.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render("Attachments") + "\n")
		for _, a := range m.attach {
			fmt.Fprintf(&b, "  📎 #%d  %s\n", a.ID, a.Data)
		}
	}

	b.WriteString("\n" + lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Comments (%d)", len(m.comments))) + "\n")
	for _, c := range m.comments {
		who := c.ActorDisplay
		if who == "" {
			who = c.ActorID
		}
		fmt.Fprintf(&b, "  %s, %s\n  %s\n\n",
			lipgloss.NewStyle().Bold(true).Render(who),
			subtleStyle.Render(c.CreationDT.Format("2006-01-02 15:04")),
			c.Message)
	}

	m.vp.SetContent(b.String())
}

func (m *cardModel) Update(msg tea.Msg, root *Model) (tea.Model, tea.Cmd) {
	switch m.mode {
	case cardModeEditLabels:
		var cmd tea.Cmd
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				m.mode = cardModeView
				return root, nil
			case "up":
				m.labels.moveCursor(-1)
				return root, nil
			case "down":
				m.labels.moveCursor(+1)
				return root, nil
			case "enter":
				action, label, name := m.labels.currentAction()
				switch action {
				case labelActionToggle:
					wasAssigned := m.labels.toggleAssigned(label.ID)
					return root, m.toggleLabel(root, label.ID, wasAssigned)
				case labelActionCreate:
					return root, m.createLabel(root, name)
				}
				return root, nil
			}
		}
		m.labels.input, cmd = m.labels.input.Update(msg)
		m.labels.refilter()
		return root, cmd
	case cardModeEditDue:
		if km, ok := msg.(tea.KeyMsg); ok {
			s := km.String()
			if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
				m.due.typeDigit(rune(s[0]))
				return root, nil
			}
			switch s {
			case "esc":
				m.mode = cardModeView
				return root, nil
			case "enter":
				m.due.commit()
				out := m.due.rfc3339()
				m.mode = cardModeView
				return root, m.saveDueDate(root, &out)
			case "c":
				m.mode = cardModeView
				return root, m.saveDueDate(root, nil)
			case "backspace":
				m.due.backspace()
			case "left", "h", "shift+tab":
				m.due.moveFocus(-1)
			case "right", "l", "tab":
				m.due.moveFocus(+1)
			case "up", "k":
				m.due.adjust(+1)
			case "down", "j":
				m.due.adjust(-1)
			case "pgup":
				m.due.adjust(+10)
			case "pgdown":
				m.due.adjust(-10)
			}
		}
		return root, nil
	case cardModeEditDescription:
		var cmd tea.Cmd
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				m.mode = cardModeView
				return root, nil
			case "ctrl+s":
				newDesc := m.editor.Value()
				m.mode = cardModeView
				return root, m.saveDescription(root, newDesc)
			}
		}
		m.editor, cmd = m.editor.Update(msg)
		return root, cmd
	case cardModeAddComment:
		var cmd tea.Cmd
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				m.mode = cardModeView
				return root, nil
			case "enter":
				text := strings.TrimSpace(m.commentI.Value())
				m.mode = cardModeView
				m.commentI.SetValue("")
				if text == "" {
					return root, nil
				}
				return root, m.postComment(root, text)
			}
		}
		m.commentI, cmd = m.commentI.Update(msg)
		return root, cmd
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "b", "q":
			return root, func() tea.Msg { return backMsg{} }
		case "r":
			return root, func() tea.Msg { return refreshMsg{} }
		case "c":
			ti := textinput.New()
			ti.Placeholder = "Type a comment, ⏎ to post, esc cancel"
			ti.Focus()
			ti.Width = m.vp.Width
			m.commentI = ti
			m.mode = cardModeAddComment
			return root, nil
		case "e":
			ta := textarea.New()
			ta.SetValue(m.card.Description)
			ta.SetWidth(m.vp.Width)
			ta.SetHeight(m.vp.Height - 2)
			ta.Focus()
			m.editor = ta
			m.mode = cardModeEditDescription
			return root, nil
		case "a":
			return root, m.toggleArchive(root)
		case "D":
			return root, m.toggleDone(root)
		case "d":
			m.due = newDueDialog(m.card.DueDate, root.accent())
			m.mode = cardModeEditDue
			return root, nil
		case "l":
			var pool []api.Label
			if root.kanban != nil {
				pool = root.kanban.boardLabels
			}
			m.labels = newLabelDialog(pool, m.card.Labels, root.accent())
			m.mode = cardModeEditLabels
			return root, nil
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return root, cmd
}

func (m *cardModel) View(width, height int) string {
	if m.card == nil {
		return ""
	}
	if m.mode == cardModeEditDue {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.due.view())
	}
	if m.mode == cardModeEditLabels {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.labels.view())
	}
	box := modalStyle.Render(m.vp.View())
	var footer string
	switch m.mode {
	case cardModeEditDescription:
		footer = inputBoxStyle.Render(m.editor.View()) + "\n" + helpStyle.Render("ctrl+s save  esc cancel")
		return lipgloss.JoinVertical(lipgloss.Left, box, footer)
	case cardModeAddComment:
		footer = inputBoxStyle.Render(m.commentI.View()) + "\n" + helpStyle.Render("⏎ post  esc cancel")
		return lipgloss.JoinVertical(lipgloss.Left, box, footer)
	default:
		footer = helpStyle.Render("e edit  c comment  l labels  d due  a archive  D done  r refresh  esc back  q quit")
		return lipgloss.JoinVertical(lipgloss.Left, box, footer)
	}
}

func (m *cardModel) baseInput() api.UpdateCardInput {
	return api.UpdateCardInput{
		Title:       m.card.Title,
		Description: m.card.Description,
		Type:        m.card.Type,
		Owner:       m.card.Owner.UID,
		Order:       m.card.Order,
		Archived:    m.card.Archived,
		DueDate:     m.card.DueDate,
		Done:        m.card.Done,
	}
}

func (m *cardModel) applyUpdate(root *Model, in api.UpdateCardInput) tea.Cmd {
	return func() tea.Msg {
		_, err := root.client.UpdateCard(root.ctx, m.boardID, m.card.StackID, m.card.ID, in)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (m *cardModel) saveDescription(root *Model, desc string) tea.Cmd {
	in := m.baseInput()
	in.Description = desc
	return m.applyUpdate(root, in)
}

// nil due sends JSON null (clear). The dialog guarantees RFC3339 output, so
// no re-parsing.
func (m *cardModel) saveDueDate(root *Model, due *string) tea.Cmd {
	in := m.baseInput()
	in.DueDate = due
	return m.applyUpdate(root, in)
}

func (m *cardModel) postComment(root *Model, text string) tea.Cmd {
	return func() tea.Msg {
		_, err := root.client.AddComment(root.ctx, m.card.ID, text, 0)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

// createLabel creates a fresh label on the board and assigns it to this card
// in one cmd. Color defaults to a muted grey; users can recolour via the web
// UI or `ncdeck label` if they want a brighter chip.
const newLabelDefaultColor = "888888"

func (m *cardModel) createLabel(root *Model, name string) tea.Cmd {
	boardID := m.boardID
	stackID := m.card.StackID
	cardID := m.card.ID
	return func() tea.Msg {
		l, err := root.client.CreateLabel(root.ctx, boardID, api.LabelInput{
			Title: name,
			Color: newLabelDefaultColor,
		})
		if err != nil {
			return errMsg{err}
		}
		if err := root.client.AssignLabelToCard(root.ctx, boardID, stackID, cardID, l.ID); err != nil {
			return errMsg{err}
		}
		return labelCreatedMsg{boardID: boardID, label: *l}
	}
}

// wasAssigned reflects the state *before* the optimistic flip the dialog
// already applied; we use it to decide between Assign and Remove.
func (m *cardModel) toggleLabel(root *Model, labelID int, wasAssigned bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if wasAssigned {
			err = root.client.RemoveLabelFromCard(root.ctx, m.boardID, m.card.StackID, m.card.ID, labelID)
		} else {
			err = root.client.AssignLabelToCard(root.ctx, m.boardID, m.card.StackID, m.card.ID, labelID)
		}
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (m *cardModel) toggleArchive(root *Model) tea.Cmd {
	return func() tea.Msg {
		var err error
		if m.card.Archived {
			err = root.client.UnarchiveCard(root.ctx, m.boardID, m.card.StackID, m.card.ID)
		} else {
			err = root.client.ArchiveCard(root.ctx, m.boardID, m.card.StackID, m.card.ID)
		}
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (m *cardModel) toggleDone(root *Model) tea.Cmd {
	in := m.baseInput()
	if m.card.Done == nil || *m.card.Done == "" {
		s := time.Now().UTC().Format(time.RFC3339)
		in.Done = &s
	} else {
		in.Done = nil
	}
	return m.applyUpdate(root, in)
}

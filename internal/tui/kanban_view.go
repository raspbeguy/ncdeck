// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

// kanbanModel renders horizontally scrolling columns of cards for a board.
type kanbanModel struct {
	boardID int
	stacks  []api.Stack
	// cursor: index into stacks for the focused column, index into that stack's
	// cards for the focused card.
	stackIdx int
	cardIdx  int
	// moveMode is true once the user pressed 'm' — next h/l + enter moves the card.
	moveMode   bool
	moveTarget int

	// inline form: "card" for new card, "stack" for new stack, "" for none.
	formKind string
	form     textinput.Model

	// horizontal scrolling offset (in columns)
	scroll int

	// topIdx is the index of the first visible card within the focused stack.
	// It only moves when the cursor would otherwise leave the visible window,
	// so cursor and card positions stay stable when navigating in the middle
	// of a long column.
	topIdx int

	colWidth int
}

func newKanbanModel(boardID int) *kanbanModel {
	return &kanbanModel{boardID: boardID, colWidth: 28}
}

func (k *kanbanModel) setStacks(stacks []api.Stack) {
	// Stable-sort stacks by Order so layout matches the web UI.
	sort.SliceStable(stacks, func(i, j int) bool { return stacks[i].Order < stacks[j].Order })
	for i := range stacks {
		sort.SliceStable(stacks[i].Cards, func(a, b int) bool {
			return stacks[i].Cards[a].Order < stacks[i].Cards[b].Order
		})
	}
	k.stacks = stacks
	if k.stackIdx >= len(stacks) {
		k.stackIdx = 0
	}
	if k.stackIdx < len(stacks) && k.cardIdx >= len(stacks[k.stackIdx].Cards) {
		k.cardIdx = 0
	}
}

func (k *kanbanModel) focusedCard() *api.Card {
	if k.stackIdx >= len(k.stacks) {
		return nil
	}
	s := k.stacks[k.stackIdx]
	if k.cardIdx >= len(s.Cards) {
		return nil
	}
	return &s.Cards[k.cardIdx]
}

func (k *kanbanModel) Update(msg tea.Msg, root *Model) (tea.Model, tea.Cmd) {
	if k.formKind != "" {
		var cmd tea.Cmd
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				k.formKind = ""
				root.statusf("")
				return root, nil
			case "enter":
				title := strings.TrimSpace(k.form.Value())
				kind := k.formKind
				k.formKind = ""
				k.form.SetValue("")
				root.statusf("")
				if title == "" {
					return root, nil
				}
				if kind == "card" {
					return root, k.createCard(root, title)
				}
				return root, k.createStack(root, title)
			}
		}
		k.form, cmd = k.form.Update(msg)
		return root, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return root, tea.Quit
		case "esc", "b":
			return root, func() tea.Msg { return backMsg{} }
		case "h", "left":
			if k.moveMode {
				if k.moveTarget > 0 {
					k.moveTarget--
				}
			} else if k.stackIdx > 0 {
				k.stackIdx--
				k.cardIdx = 0
				k.topIdx = 0
			}
		case "l", "right":
			if k.moveMode {
				if k.moveTarget < len(k.stacks)-1 {
					k.moveTarget++
				}
			} else if k.stackIdx < len(k.stacks)-1 {
				k.stackIdx++
				k.cardIdx = 0
				k.topIdx = 0
			}
		case "j", "down":
			if s := k.curStack(); s != nil && k.cardIdx < len(s.Cards)-1 {
				k.cardIdx++
			}
		case "k", "up":
			if k.cardIdx > 0 {
				k.cardIdx--
				if k.cardIdx < k.topIdx {
					k.topIdx = k.cardIdx
				}
			}
		case "J", "shift+down":
			return root, k.reorderWithin(root, +1)
		case "K", "shift+up":
			return root, k.reorderWithin(root, -1)
		case "enter":
			if k.moveMode {
				return root, k.doMove(root)
			}
			c := k.focusedCard()
			if c == nil {
				return root, nil
			}
			boardID := k.boardID
			card := *c
			return root, func() tea.Msg { return openCardMsg{boardID: boardID, card: &card} }
		case "r":
			return root, func() tea.Msg { return refreshMsg{} }
		case "m":
			if k.focusedCard() != nil {
				k.moveMode = true
				k.moveTarget = k.stackIdx
				root.statusf("move: ←/→ pick target stack, ⏎ confirm, esc cancel")
			}
		case "a":
			if c := k.focusedCard(); c != nil {
				return root, k.doArchive(root, c)
			}
		case "x":
			if c := k.focusedCard(); c != nil {
				return root, k.doDelete(root, c)
			}
		case "n":
			if k.curStack() != nil {
				k.openForm("card", "New card title")
			}
		case "N":
			k.openForm("stack", "New stack title")
		}
		if msg.String() == "esc" && k.moveMode {
			k.moveMode = false
			root.statusf("")
		}
	}
	return root, nil
}

func (k *kanbanModel) curStack() *api.Stack {
	if k.stackIdx >= len(k.stacks) {
		return nil
	}
	return &k.stacks[k.stackIdx]
}

// reorderWithin shifts the focused card up (-1) or down (+1) one slot in its
// own stack. The Deck server interprets ReorderInput.Order as the destination
// index in the sorted column, so passing cardIdx+delta is enough; the server
// renormalises every other card's order. The cursor follows the moved card so
// the user can chain J/K presses.
func (k *kanbanModel) reorderWithin(root *Model, delta int) tea.Cmd {
	s := k.curStack()
	if s == nil {
		return nil
	}
	target := k.cardIdx + delta
	if target < 0 || target >= len(s.Cards) {
		return nil
	}
	c := s.Cards[k.cardIdx]
	stackID := s.ID
	k.cardIdx = target
	if k.cardIdx < k.topIdx {
		k.topIdx = k.cardIdx
	}
	return func() tea.Msg {
		err := root.client.ReorderCard(root.ctx, k.boardID, c.ID, api.ReorderInput{
			Order:   target,
			StackID: stackID,
		})
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (k *kanbanModel) doMove(root *Model) tea.Cmd {
	c := k.focusedCard()
	src := k.curStack()
	if c == nil || src == nil || k.moveTarget >= len(k.stacks) {
		k.moveMode = false
		return nil
	}
	dst := k.stacks[k.moveTarget]
	k.moveMode = false
	return func() tea.Msg {
		err := root.client.ReorderCard(root.ctx, k.boardID, c.ID, api.ReorderInput{Order: 0, StackID: dst.ID})
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (k *kanbanModel) doArchive(root *Model, c *api.Card) tea.Cmd {
	s := k.curStack()
	return func() tea.Msg {
		err := root.client.ArchiveCard(root.ctx, k.boardID, s.ID, c.ID)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (k *kanbanModel) openForm(kind, placeholder string) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.Width = 60
	k.form = ti
	k.formKind = kind
}

func (k *kanbanModel) createCard(root *Model, title string) tea.Cmd {
	s := k.curStack()
	if s == nil {
		return nil
	}
	return func() tea.Msg {
		_, err := root.client.CreateCard(root.ctx, k.boardID, s.ID, api.CreateCardInput{Title: title, Order: 999})
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (k *kanbanModel) createStack(root *Model, title string) tea.Cmd {
	return func() tea.Msg {
		_, err := root.client.CreateStack(root.ctx, k.boardID, api.StackInput{Title: title, Order: 999})
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (k *kanbanModel) doDelete(root *Model, c *api.Card) tea.Cmd {
	s := k.curStack()
	return func() tea.Msg {
		err := root.client.DeleteCard(root.ctx, k.boardID, s.ID, c.ID)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (k *kanbanModel) View(width, height int) string {
	if len(k.stacks) == 0 {
		return subtleStyle.Render("\n  No stacks. Press 'N' to create one, b to go back.\n")
	}

	bodyHeight := height - 4
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	// Determine how many stacks fit horizontally.
	colW := k.colWidth
	maxCols := width / (colW + 2)
	if maxCols < 1 {
		maxCols = 1
	}
	// Auto-scroll so the focused stack is visible.
	if k.stackIdx < k.scroll {
		k.scroll = k.stackIdx
	}
	if k.stackIdx >= k.scroll+maxCols {
		k.scroll = k.stackIdx - maxCols + 1
	}

	visible := k.stacks[k.scroll:]
	if len(visible) > maxCols {
		visible = visible[:maxCols]
	}

	cols := make([]string, 0, len(visible))
	for vi, s := range visible {
		globalIdx := k.scroll + vi
		focused := globalIdx == k.stackIdx
		highlight := focused
		if k.moveMode && globalIdx == k.moveTarget {
			highlight = true
		}
		cols = append(cols, k.renderStack(s, focused, highlight, colW, bodyHeight))
	}

	help := helpStyle.Render("h/l col  j/k card  J/K reorder  ⏎ open  n new  N stack  m move  a archive  x delete  r refresh  b back  q quit")
	if k.moveMode {
		help = helpStyle.Render("MOVE: ←/→ pick target, ⏎ confirm, esc cancel")
	}
	if k.formKind != "" {
		help = inputBoxStyle.Render(k.form.View()) + "\n" + helpStyle.Render("⏎ submit  esc cancel")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...) + "\n" + help
}

func (k *kanbanModel) renderStack(s api.Stack, focused, highlight bool, w, h int) string {
	hdr := stackHeaderStyle.Width(w).Render(fmt.Sprintf("%s (%d)", s.Title, len(s.Cards)))
	if highlight {
		hdr = stackHeaderSel.Width(w).Render(fmt.Sprintf("%s (%d)", s.Title, len(s.Cards)))
	}

	// Render every card once and record its actual rendered height, so we can
	// pick a vertical window that keeps the focused card visible.
	rendered := make([]string, len(s.Cards))
	heights := make([]int, len(s.Cards))
	for i, c := range s.Cards {
		sel := focused && i == k.cardIdx
		rendered[i] = renderCard(c, w-2, sel)
		heights[i] = lipgloss.Height(rendered[i])
	}

	// Available rows for cards beneath the header. Reserve 2 rows for potential
	// "↑ N more" / "↓ N more" hints so they don't push content off-screen.
	avail := h - lipgloss.Height(hdr) - 2
	if avail < 3 {
		avail = 3
	}

	start, end := 0, len(rendered)
	switch {
	case len(rendered) == 0:
		// nothing to do
	case focused:
		// Anchor on k.topIdx, advance it forward only when the cursor would
		// otherwise fall off the bottom of the visible window. Pressing 'k'
		// already pulls topIdx back via the Update handler.
		if k.topIdx > k.cardIdx {
			k.topIdx = k.cardIdx
		}
		if k.topIdx >= len(rendered) {
			k.topIdx = len(rendered) - 1
		}
		for {
			used := 0
			fits := false
			last := k.topIdx
			for i := k.topIdx; i < len(rendered); i++ {
				if used+heights[i] > avail {
					break
				}
				used += heights[i]
				last = i
				if i == k.cardIdx {
					fits = true
				}
			}
			if fits || k.topIdx >= k.cardIdx {
				start, end = k.topIdx, last+1
				break
			}
			k.topIdx++
		}
	default:
		// Non-focused stacks: just show from the top, truncating if needed.
		used := 0
		end = 0
		for end < len(rendered) && used+heights[end] <= avail {
			used += heights[end]
			end++
		}
		if end == 0 && len(rendered) > 0 {
			end = 1 // always show at least one card
		}
	}

	parts := []string{hdr}
	if start > 0 {
		parts = append(parts, subtleStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
	}
	parts = append(parts, rendered[start:end]...)
	if end < len(rendered) {
		parts = append(parts, subtleStyle.Render(fmt.Sprintf("  ↓ %d more", len(rendered)-end)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return lipgloss.NewStyle().Width(w + 2).Padding(0, 1).Render(content)
}

func renderCard(c api.Card, w int, selected bool) string {
	style := cardStyle
	if selected {
		style = cardStyleSel
	}
	style = style.Width(w)

	title := c.Title
	if c.Done != nil && *c.Done != "" {
		title = "✓ " + title
	}
	parts := []string{lipgloss.NewStyle().Bold(true).Render(title)}

	if len(c.Labels) > 0 {
		chips := make([]string, 0, len(c.Labels))
		for _, l := range c.Labels {
			chips = append(chips, chipStyle.Background(lipgloss.Color("#"+l.Color)).Foreground(lipgloss.Color("16")).Render(l.Title))
		}
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, chips...))
	}

	var meta []string
	if c.DueDate != nil && *c.DueDate != "" {
		t, err := time.Parse(time.RFC3339, *c.DueDate)
		if err == nil {
			s := t.Format("Jan 2")
			if t.Before(time.Now()) {
				meta = append(meta, dueOverdueStyle.Render("⏰ "+s))
			} else {
				meta = append(meta, subtleStyle.Render("⏰ "+s))
			}
		}
	}
	if len(c.AssignedUsers) > 0 {
		ids := make([]string, 0, len(c.AssignedUsers))
		for _, a := range c.AssignedUsers {
			ids = append(ids, "@"+a.UID)
		}
		meta = append(meta, subtleStyle.Render(strings.Join(ids, " ")))
	}
	if c.CommentsCount > 0 {
		meta = append(meta, subtleStyle.Render(fmt.Sprintf("💬 %d", c.CommentsCount)))
	}
	if c.AttachmentCount > 0 {
		meta = append(meta, subtleStyle.Render(fmt.Sprintf("📎 %d", c.AttachmentCount)))
	}
	if len(meta) > 0 {
		parts = append(parts, strings.Join(meta, "  "))
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

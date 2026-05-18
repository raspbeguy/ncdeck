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

const (
	kanbanChromeRows      = 4 // header + help + spacing
	kanbanColGutter       = 2
	kanbanMinBodyRows     = 5
	kanbanScrollHintRows  = 2 // reserved for "↑/↓ N more"
	kanbanMinAvailRows    = 3
	kanbanDefaultColWidth = 28
)

type kanbanModel struct {
	boardID    int
	boardColor string
	stacks     []api.Stack
	stackIdx   int
	cardIdx    int
	moveMode   bool
	moveTarget int

	formKind string // "card" / "stack" / "" for none
	form     textinput.Model

	scroll int // horizontal column offset

	// topIdx only moves when the cursor would leave the visible window,
	// keeping cursor and card positions stable mid-column.
	topIdx int

	// width is the last terminal width seen via WindowSizeMsg, used to size
	// the inline form. 0 before the first resize event arrives.
	width int

	colWidth int
}

func newKanbanModel(boardID int) *kanbanModel {
	return &kanbanModel{boardID: boardID, colWidth: kanbanDefaultColWidth}
}

func (k *kanbanModel) setStacks(stacks []api.Stack) {
	// Order matches the web UI's stable sort.
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
				root.setStatus("")
				return root, nil
			case "enter":
				title := strings.TrimSpace(k.form.Value())
				kind := k.formKind
				k.formKind = ""
				k.form.SetValue("")
				root.setStatus("")
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
		case "esc":
			if k.moveMode {
				k.moveMode = false
				root.setStatus("")
				return root, nil
			}
			return root, func() tea.Msg { return backMsg{} }
		case "b":
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
				root.setStatus("move: ←/→ pick target stack, ⏎ confirm, esc cancel")
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
	}
	return root, nil
}

func (k *kanbanModel) curStack() *api.Stack {
	if k.stackIdx >= len(k.stacks) {
		return nil
	}
	return &k.stacks[k.stackIdx]
}

// Deck interprets Order as the destination index in the sorted column and
// renormalises everything else; passing cardIdx+delta is enough.
//
// Cursor moves only on success (via reorderedMsg) so a failed reorder doesn't
// leave the cursor pointing at a different card than the user sees.
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
	boardID := k.boardID
	return func() tea.Msg {
		err := root.client.ReorderCard(root.ctx, boardID, c.ID, api.ReorderInput{
			Order:   target,
			StackID: stackID,
		})
		if err != nil {
			return errMsg{err}
		}
		return reorderedMsg{boardID: boardID, newCardIdx: target}
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
	w := k.width - 6
	switch {
	case w <= 0:
		w = 60
	case w < 20:
		w = 20
	case w > 80:
		w = 80
	}
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.Width = w
	k.form = ti
	k.formKind = kind
}

func (k *kanbanModel) createCard(root *Model, title string) tea.Cmd {
	s := k.curStack()
	if s == nil {
		return nil
	}
	return func() tea.Msg {
		_, err := root.client.CreateCard(root.ctx, k.boardID, s.ID, api.CreateCardInput{Title: title, Order: api.OrderAtEnd})
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (k *kanbanModel) createStack(root *Model, title string) tea.Cmd {
	return func() tea.Msg {
		_, err := root.client.CreateStack(root.ctx, k.boardID, api.StackInput{Title: title, Order: api.OrderAtEnd})
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

	bodyHeight := height - kanbanChromeRows
	if bodyHeight < kanbanMinBodyRows {
		bodyHeight = kanbanMinBodyRows
	}

	colW := k.colWidth
	maxCols := width / (colW + kanbanColGutter)
	if maxCols < 1 {
		maxCols = 1
	}
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

func (k *kanbanModel) accentColor() lipgloss.Color {
	if k.boardColor != "" {
		return lipgloss.Color("#" + k.boardColor)
	}
	return colSelected
}

// Only advances topIdx forward; the 'k' handler pulls it back when the
// cursor moves above the window.
func pickFocusedWindow(topIdx, cardIdx int, heights []int, avail int) (newTop, start, end int) {
	newTop = topIdx
	if newTop > cardIdx {
		newTop = cardIdx
	}
	if newTop >= len(heights) {
		newTop = len(heights) - 1
	}
	if newTop < 0 {
		newTop = 0
	}
	for {
		used := 0
		fits := false
		last := newTop
		for i := newTop; i < len(heights); i++ {
			if used+heights[i] > avail {
				break
			}
			used += heights[i]
			last = i
			if i == cardIdx {
				fits = true
			}
		}
		if fits || newTop >= cardIdx {
			return newTop, newTop, last + 1
		}
		newTop++
	}
}

// Always returns at least 1 when len(heights) > 0 so non-focused columns
// never collapse to "↓ N more" with no cards visible.
func pickTopWindow(heights []int, avail int) int {
	if len(heights) == 0 {
		return 0
	}
	used := 0
	end := 0
	for end < len(heights) && used+heights[end] <= avail {
		used += heights[end]
		end++
	}
	if end == 0 {
		end = 1
	}
	return end
}

func (k *kanbanModel) renderStack(s api.Stack, focused, highlight bool, w, h int) string {
	accent := k.accentColor()
	hdr := stackHeaderStyle.Width(w).Render(fmt.Sprintf("%s (%d)", s.Title, len(s.Cards)))
	if highlight {
		hdr = stackHeaderStyle.Foreground(accent).Underline(true).Width(w).Render(fmt.Sprintf("%s (%d)", s.Title, len(s.Cards)))
	}

	if len(s.Cards) == 0 {
		return columnWrapStyle.Width(w + 2).Render(hdr)
	}

	rendered := make([]string, len(s.Cards))
	heights := make([]int, len(s.Cards))
	for i, c := range s.Cards {
		sel := focused && i == k.cardIdx
		rendered[i] = renderCard(c, w-2, sel, accent)
		heights[i] = lipgloss.Height(rendered[i])
	}

	avail := h - lipgloss.Height(hdr) - kanbanScrollHintRows
	if avail < kanbanMinAvailRows {
		avail = kanbanMinAvailRows
	}

	var start, end int
	if focused {
		var newTop int
		newTop, start, end = pickFocusedWindow(k.topIdx, k.cardIdx, heights, avail)
		k.topIdx = newTop
	} else {
		end = pickTopWindow(heights, avail)
	}

	parts := []string{hdr}
	if start > 0 {
		parts = append(parts, subtleStyle.Render(fmt.Sprintf("  ↑ %d more", start)))
	}
	parts = append(parts, rendered[start:end]...)
	if end < len(rendered) {
		parts = append(parts, subtleStyle.Render(fmt.Sprintf("  ↓ %d more", len(rendered)-end)))
	}

	return columnWrapStyle.Width(w + 2).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func renderCard(c api.Card, w int, selected bool, accent lipgloss.Color) string {
	style := cardStyle
	if selected {
		style = cardStyle.BorderForeground(accent).Bold(true)
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

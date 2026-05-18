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
	// kanbanColPadH is Padding(0, 1) on columnWrapStyle expressed as a total.
	// Cards inside the wrapper render at colWidth - kanbanColPadH.
	kanbanColPadH         = 2
	kanbanMinBodyRows     = 5
	kanbanScrollHintRows  = 2 // reserved for "↑/↓ N more"
	kanbanMinAvailRows    = 3
	kanbanDefaultColWidth = 28
)

type kanbanModel struct {
	boardID     int
	boardColor  string
	boardLabels []api.Label
	stacks      []api.Stack
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

	// reorderInFlight gates J/K so a press during an outstanding ReorderCard
	// call doesn't queue a second move against the just-moved card.
	reorderInFlight bool

	showHelp bool

	// labelFilter is the set of label IDs the kanban view is filtering by.
	// A card passes when it has at least one label in this set; empty = no
	// filter, show everything.
	labelFilter map[int]bool

	// labelMgr is the modal label-management dialog. labelMgrOpen mirrors
	// the "is the manager on screen" state so View can route through it.
	labelMgr     labelManager
	labelMgrOpen bool

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
	if len(k.labelFilter) == 0 {
		if k.cardIdx >= len(s.Cards) {
			return nil
		}
		return &s.Cards[k.cardIdx]
	}
	// Cursor is the index into the filtered subset.
	seen := 0
	for i := range s.Cards {
		if !matchesLabelFilter(s.Cards[i], k.labelFilter) {
			continue
		}
		if seen == k.cardIdx {
			return &s.Cards[i]
		}
		seen++
	}
	return nil
}

func (k *kanbanModel) visibleCardCount() int {
	s := k.curStack()
	if s == nil {
		return 0
	}
	if len(k.labelFilter) == 0 {
		return len(s.Cards)
	}
	n := 0
	for _, c := range s.Cards {
		if matchesLabelFilter(c, k.labelFilter) {
			n++
		}
	}
	return n
}

func (k *kanbanModel) Update(msg tea.Msg, root *Model) (tea.Model, tea.Cmd) {
	if k.labelMgrOpen {
		if km, ok := msg.(tea.KeyMsg); ok {
			return root, k.handleLabelMgrKey(root, km)
		}
		return root, nil
	}
	// When the inline form is open every msg is forwarded to its textinput;
	// the textinput ignores non-key messages so passing them through is safe.
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
		case "?":
			k.showHelp = !k.showHelp
			return root, nil
		case "q":
			return root, tea.Quit
		case "esc":
			if k.showHelp {
				k.showHelp = false
				return root, nil
			}
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
			if k.cardIdx < k.visibleCardCount()-1 {
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
			if len(k.labelFilter) > 0 {
				root.setStatus("J/K reorder is disabled while a label filter is active")
				return root, nil
			}
			return root, k.reorderWithin(root, +1)
		case "K", "shift+up":
			if len(k.labelFilter) > 0 {
				root.setStatus("J/K reorder is disabled while a label filter is active")
				return root, nil
			}
			return root, k.reorderWithin(root, -1)
		case "L":
			k.labelMgr = newLabelManager(k.boardLabels, copyFilter(k.labelFilter), k.accentColor())
			k.labelMgrOpen = true
			return root, nil
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
// Optimistic: cursor moves and the local stack swaps immediately so rapid
// J/K presses feel responsive. reorderInFlight gates against pressing again
// before the server confirms (which would queue a stale move). On failure
// the kanban resyncs by reloading stacks from the server.
//
// Result handling lives in app.Update: reorderedMsg clears the gate,
// reorderFailedMsg clears the gate + triggers a full resync.
func (k *kanbanModel) reorderWithin(root *Model, delta int) tea.Cmd {
	if k.reorderInFlight {
		return nil
	}
	if k.stackIdx >= len(k.stacks) {
		return nil
	}
	cards := k.stacks[k.stackIdx].Cards
	target := k.cardIdx + delta
	if target < 0 || target >= len(cards) {
		return nil
	}
	// Capture the card *before* the swap; the closure must move the card
	// the user was on, not the one that ends up at the old slot after.
	c := cards[k.cardIdx]
	stackID := k.stacks[k.stackIdx].ID
	boardID := k.boardID

	cards[k.cardIdx], cards[target] = cards[target], cards[k.cardIdx]
	k.cardIdx = target
	if k.cardIdx < k.topIdx {
		k.topIdx = k.cardIdx
	}
	k.reorderInFlight = true

	return func() tea.Msg {
		err := root.client.ReorderCard(root.ctx, boardID, c.ID, api.ReorderInput{
			Order:   target,
			StackID: stackID,
		})
		if err != nil {
			return reorderFailedMsg{boardID: boardID, err: err}
		}
		return reorderedMsg{boardID: boardID}
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
	if k.labelMgrOpen {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, k.labelMgr.view())
	}
	if k.showHelp {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, renderHelp("Kanban", []helpEntry{
			{"h / l or ←/→", "previous / next column"},
			{"j / k or ↓/↑", "navigate cards in column"},
			{"J / K", "reorder card down / up"},
			{"⏎", "open card"},
			{"n / N", "new card / new stack"},
			{"m", "move card to another stack"},
			{"a / x", "archive / delete card"},
			{"L", "manage / filter labels"},
			{"r", "refresh"},
			{"b / esc", "back to boards"},
			{"q", "quit"},
			{"?", "toggle this help"},
		}, k.accentColor()))
	}
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

	help := helpStyle.Render("h/l/j/k navigate   ⏎ open   ? help   b back")
	if k.moveMode {
		help = helpStyle.Render("MOVE: ←/→ pick target, ⏎ confirm, esc cancel")
	}
	if k.formKind != "" {
		help = inputBoxStyle.Render(k.form.View()) + "\n" + helpStyle.Render("⏎ submit  esc cancel")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...) + "\n" + help
}

// handleLabelMgrKey dispatches a key to the manager dialog and fires whatever
// command (create/update/delete) the manager asks for. Closing the manager
// commits its filter into the kanban.
func (k *kanbanModel) handleLabelMgrKey(root *Model, km tea.KeyMsg) tea.Cmd {
	switch action := k.labelMgr.Update(km); action {
	case lmgrActionClose:
		// Don't disturb the cursor when the user just glanced at labels.
		if !filtersEqual(k.labelFilter, k.labelMgr.filter) {
			k.cardIdx = 0
			k.topIdx = 0
		}
		k.labelFilter = k.labelMgr.filter
		k.labelMgrOpen = false
		return nil
	case lmgrActionCreate:
		return k.cmdCreateLabel(root, k.labelMgr.pendingName, k.labelMgr.pendingColor)
	case lmgrActionUpdateName:
		l := k.labelMgr.current()
		if l == nil {
			return nil
		}
		return k.cmdUpdateLabel(root, l.ID, k.labelMgr.pendingName, l.Color)
	case lmgrActionUpdateColor:
		l := k.labelMgr.current()
		if l == nil {
			return nil
		}
		return k.cmdUpdateLabel(root, l.ID, l.Title, k.labelMgr.pendingColor)
	case lmgrActionDelete:
		l := k.labelMgr.current()
		if l == nil {
			return nil
		}
		return k.cmdDeleteLabel(root, l.ID)
	}
	return nil
}

func (k *kanbanModel) cmdCreateLabel(root *Model, name, color string) tea.Cmd {
	boardID := k.boardID
	return func() tea.Msg {
		l, err := root.client.CreateLabel(root.ctx, boardID, api.LabelInput{Title: name, Color: color})
		if err != nil {
			return labelOpFailedMsg{boardID: boardID, err: err}
		}
		return labelCreatedMsg{boardID: boardID, label: *l}
	}
}

func (k *kanbanModel) cmdUpdateLabel(root *Model, labelID int, name, color string) tea.Cmd {
	boardID := k.boardID
	return func() tea.Msg {
		l, err := root.client.UpdateLabel(root.ctx, boardID, labelID, api.LabelInput{Title: name, Color: color})
		if err != nil {
			return labelOpFailedMsg{boardID: boardID, err: err}
		}
		return labelUpdatedMsg{boardID: boardID, label: *l}
	}
}

func (k *kanbanModel) cmdDeleteLabel(root *Model, labelID int) tea.Cmd {
	boardID := k.boardID
	return func() tea.Msg {
		if err := root.client.DeleteLabel(root.ctx, boardID, labelID); err != nil {
			return labelOpFailedMsg{boardID: boardID, err: err}
		}
		return labelDeletedMsg{boardID: boardID, labelID: labelID}
	}
}

func copyFilter(src map[int]bool) map[int]bool {
	out := make(map[int]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func filtersEqual(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}

// matchesLabelFilter is true when the card has at least one label in the
// active filter, or the filter is empty (no filter).
func matchesLabelFilter(c api.Card, filter map[int]bool) bool {
	if len(filter) == 0 {
		return true
	}
	for _, l := range c.Labels {
		if filter[l.ID] {
			return true
		}
	}
	return false
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
	visible := s.Cards
	if len(k.labelFilter) > 0 {
		visible = visible[:0:0]
		for _, c := range s.Cards {
			if matchesLabelFilter(c, k.labelFilter) {
				visible = append(visible, c)
			}
		}
	}

	headerText := fmt.Sprintf("%s (%d)", s.Title, len(visible))
	if len(visible) != len(s.Cards) {
		headerText = fmt.Sprintf("%s (%d/%d)", s.Title, len(visible), len(s.Cards))
	}
	hdr := stackHeaderStyle.Width(w).Render(headerText)
	if highlight {
		hdr = stackHeaderStyle.Foreground(accent).Underline(true).Width(w).Render(headerText)
	}

	if len(visible) == 0 {
		return columnWrapStyle.Width(w + kanbanColPadH).Render(hdr)
	}

	rendered := make([]string, len(visible))
	heights := make([]int, len(visible))
	for i, c := range visible {
		sel := focused && i == k.cardIdx
		rendered[i] = renderCard(c, w-kanbanColPadH, sel, accent)
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

	return columnWrapStyle.Width(w + kanbanColPadH).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
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

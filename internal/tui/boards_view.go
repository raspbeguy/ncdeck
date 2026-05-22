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

type boardsMode int

const (
	boardsModeList boardsMode = iota
	boardsModeNewTitle
	boardsModeEditTitle
	boardsModeEditColor
	boardsModeConfirmDel
)

type boardsModel struct {
	boards   []api.Board
	cursor   int
	showHelp bool

	mode         boardsMode
	titleInput   textinput.Model
	picker       colorPicker
	confirm      confirmDialog
	pendingTitle string
}

func newBoardsModel() boardsModel {
	return boardsModel{}
}

func (m *boardsModel) setBoards(bs []api.Board) {
	m.boards = bs
	if m.cursor >= len(bs) {
		m.cursor = 0
	}
}

func (b boardsModel) Update(msg tea.Msg, root *Model) (boardsModel, tea.Cmd) {
	if b.mode != boardsModeList {
		return b.updateSubmode(msg, root)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Help overlay is modal: while it's up, only `?` / `esc` (close) and
		// `q` (quit) respond. Everything else is swallowed so navigation/
		// open/etc. can't run silently underneath the overlay.
		if b.showHelp {
			switch msg.String() {
			case "?", "esc":
				b.showHelp = false
				return b, nil
			case "q":
				return b, tea.Quit
			}
			return b, nil
		}
		switch msg.String() {
		case "?":
			b.showHelp = true
			return b, nil
		case "esc":
			return b, tea.Quit
		case "q":
			return b, tea.Quit
		case "j", "down":
			if b.cursor < len(b.boards)-1 {
				b.cursor++
			}
		case "k", "up":
			if b.cursor > 0 {
				b.cursor--
			}
		case "g", "home":
			b.cursor = 0
		case "G", "end":
			if n := len(b.boards); n > 0 {
				b.cursor = n - 1
			}
		case "r":
			return b, func() tea.Msg { return refreshMsg{} }
		case "enter", "l", "right":
			if len(b.boards) > 0 {
				board := b.boards[b.cursor]
				return b, func() tea.Msg { return boardOpenedMsg{boardID: board.ID, color: board.Color} }
			}
		case "n":
			b.titleInput = newBoardTitleInput("new board title", "")
			b.mode = boardsModeNewTitle
		case "e":
			if len(b.boards) > 0 {
				b.titleInput = newBoardTitleInput("rename board", b.boards[b.cursor].Title)
				b.mode = boardsModeEditTitle
			}
		case "x":
			if len(b.boards) > 0 {
				name := b.boards[b.cursor].Title
				b.confirm = newConfirmDialog(fmt.Sprintf("Delete board %q and all its content?", name), colDanger)
				b.mode = boardsModeConfirmDel
			}
		}
	}
	return b, nil
}

func (b boardsModel) updateSubmode(msg tea.Msg, root *Model) (boardsModel, tea.Cmd) {
	km, isKey := msg.(tea.KeyMsg)

	switch b.mode {
	case boardsModeNewTitle, boardsModeEditTitle:
		if isKey {
			switch km.String() {
			case "esc":
				b.mode = boardsModeList
				return b, nil
			case "enter":
				title := strings.TrimSpace(b.titleInput.Value())
				if title == "" {
					return b, nil
				}
				if b.mode == boardsModeNewTitle {
					b.mode = boardsModeList
					return b, b.cmdCreate(root, title)
				}
				b.pendingTitle = title
				cur := b.boards[b.cursor].Color
				if cur == "" {
					cur = "0082c9"
				}
				b.picker = newColorPicker(cur, colSelected)
				b.mode = boardsModeEditColor
				return b, nil
			}
		}
		var cmd tea.Cmd
		b.titleInput, cmd = b.titleInput.Update(msg)
		return b, cmd

	case boardsModeEditColor:
		if !isKey {
			return b, nil
		}
		// left/right intentionally fall out of the switch when the input is
		// focused so textinput.Update below receives them for cursor movement.
		switch km.String() {
		case "esc":
			b.mode = boardsModeList
			b.pendingTitle = ""
			return b, nil
		case "tab":
			b.picker.toggleFocus()
			return b, nil
		case "enter":
			hex, ok := b.picker.pickedColor()
			if !ok {
				return b, nil
			}
			id := b.boards[b.cursor].ID
			title := b.pendingTitle
			b.mode = boardsModeList
			b.pendingTitle = ""
			return b, b.cmdUpdate(root, id, title, hex)
		case "left":
			if !b.picker.focusInput {
				b.picker.movePreset(-1)
				return b, nil
			}
		case "right":
			if !b.picker.focusInput {
				b.picker.movePreset(+1)
				return b, nil
			}
		}
		if b.picker.focusInput {
			var cmd tea.Cmd
			b.picker.input, cmd = b.picker.input.Update(km)
			_ = cmd
		}
		return b, nil

	case boardsModeConfirmDel:
		if !isKey {
			return b, nil
		}
		switch b.confirm.Update(km) {
		case confirmActionYes:
			id := b.boards[b.cursor].ID
			b.mode = boardsModeList
			return b, b.cmdDelete(root, id)
		case confirmActionNo:
			b.mode = boardsModeList
		}
		return b, nil
	}
	return b, nil
}

func newBoardTitleInput(placeholder, initial string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(initial)
	ti.CursorEnd()
	ti.Focus()
	ti.Width = 40
	return ti
}

func (b boardsModel) cmdCreate(root *Model, title string) tea.Cmd {
	return func() tea.Msg {
		_, err := root.client.CreateBoard(root.ctx, api.CreateBoardInput{Title: title, Color: "0082c9"})
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (b boardsModel) cmdUpdate(root *Model, id int, title, color string) tea.Cmd {
	return func() tea.Msg {
		_, err := root.client.UpdateBoard(root.ctx, id, api.UpdateBoardInput{Title: title, Color: color})
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (b boardsModel) cmdDelete(root *Model, id int) tea.Cmd {
	return func() tea.Msg {
		err := root.client.DeleteBoard(root.ctx, id)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{}
	}
}

func (b boardsModel) View(width, height int) string {
	if b.mode == boardsModeNewTitle || b.mode == boardsModeEditTitle {
		title := "New board"
		if b.mode == boardsModeEditTitle {
			title = "Rename board"
		}
		body := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colSelected).Bold(true).Render(title),
			"",
			inputBoxStyle.Render(b.titleInput.View()),
			"",
			helpStyle.Render("⏎ next   esc cancel"),
		)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			modalStyle.BorderForeground(colSelected).Padding(1, 3).Render(body))
	}
	if b.mode == boardsModeEditColor {
		body := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(colSelected).Bold(true).Render(fmt.Sprintf("Colour for %q", b.pendingTitle)),
			"",
			b.picker.view(),
		)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			modalStyle.BorderForeground(colSelected).Padding(1, 3).Render(body))
	}
	if b.mode == boardsModeConfirmDel {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, b.confirm.view())
	}
	if b.showHelp {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, renderHelp("Boards", []helpEntry{
			{"j / k or ↑/↓", "navigate"},
			{"g / G", "first / last"},
			{"⏎ or l", "open board"},
			{"n", "new board"},
			{"e", "edit board (title + colour)"},
			{"x", "delete board"},
			{"r", "refresh"},
			{"q / esc", "quit"},
			{"?", "toggle this help"},
		}, colSelected))
	}
	if len(b.boards) == 0 {
		return subtleStyle.Render("\n  No boards (press n to create one, r to refresh, q to quit).\n")
	}
	rows := []string{
		titleStyle.Background(colSubtle).Render(" Boards "),
		"",
	}
	for i, board := range b.boards {
		hex := board.Color
		if hex == "" {
			hex = "888888"
		}
		boardCol := lipgloss.Color("#" + hex)
		marker := "  "
		if i == b.cursor {
			marker = lipgloss.NewStyle().Foreground(boardCol).Render("▌ ")
		}
		swatch := lipgloss.NewStyle().Background(boardCol).Render("   ")
		title := board.Title
		if i == b.cursor {
			title = lipgloss.NewStyle().Foreground(boardCol).Bold(true).Render(title)
		}
		meta := subtleStyle.Render(fmt.Sprintf("  #%d  %s", board.ID, board.OwnerRaw.UID))
		if board.Archived {
			meta += subtleStyle.Render("  [archived]")
		}
		rows = append(rows, marker+swatch+" "+title+meta)
	}
	rows = append(rows, "", helpStyle.Render("↑/↓ ⏎ open   n new   e edit   x delete   ? help   q quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

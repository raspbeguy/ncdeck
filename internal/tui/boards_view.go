// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

type boardsModel struct {
	boards   []api.Board
	cursor   int
	showHelp bool
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
		}
	}
	return b, nil
}

func (b boardsModel) View(width, height int) string {
	if b.showHelp {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, renderHelp("Boards", []helpEntry{
			{"j / k or ↑/↓", "navigate"},
			{"g / G", "first / last"},
			{"⏎ or l", "open board"},
			{"r", "refresh"},
			{"q / esc", "quit"},
			{"?", "toggle this help"},
		}, colSelected))
	}
	if len(b.boards) == 0 {
		return subtleStyle.Render("\n  No boards (press q to quit, r to retry).\n")
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
	rows = append(rows, "", helpStyle.Render("↑/↓ ⏎ open   ? help   q quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

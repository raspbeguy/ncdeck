// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

type boardsModel struct {
	boards []api.Board
	cursor int
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
		switch msg.String() {
		case "q", "esc":
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
			b.cursor = len(b.boards) - 1
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
	if len(b.boards) == 0 {
		return subtleStyle.Render("\n  No boards (press q to quit, r to retry).\n")
	}
	rows := []string{
		titleStyle.Copy().Background(colSubtle).Render(" Boards "),
		"",
	}
	for i, board := range b.boards {
		boardCol := lipgloss.Color("#" + board.Color)
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
	rows = append(rows, "", helpStyle.Render("↑/↓ navigate  ⏎ open  r refresh  q quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

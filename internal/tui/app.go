// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

type screen int

const (
	screenBoards screen = iota
	screenKanban
	screenCard
)

// Model is the root bubbletea model.
type Model struct {
	ctx    context.Context
	client *api.Client

	width  int
	height int

	active screen
	boards boardsModel
	kanban *kanbanModel
	card   *cardModel

	spinner spinner.Model
	loading bool
	status  string
	errStr  string
}

// Run launches the TUI. initialBoardID may be 0 to start at the picker.
func Run(ctx context.Context, c *api.Client, initialBoardID int) error {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := &Model{
		ctx:     ctx,
		client:  c,
		active:  screenBoards,
		boards:  newBoardsModel(),
		spinner: sp,
		loading: true,
	}

	if initialBoardID > 0 {
		m.active = screenKanban
		m.kanban = newKanbanModel(initialBoardID)
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.active == screenBoards {
		cmds = append(cmds, m.loadBoards())
	} else if m.kanban != nil {
		cmds = append(cmds, m.loadStacks(m.kanban.boardID), m.loadBoardInfo(m.kanban.boardID))
	}
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case errMsg:
		m.loading = false
		m.errStr = msg.err.Error()
		return m, nil
	case statusMsg:
		m.status = msg.text
		return m, nil
	case boardsLoadedMsg:
		m.loading = false
		m.errStr = ""
		m.boards.setBoards(msg.boards)
		return m, nil
	case stacksLoadedMsg:
		m.loading = false
		m.errStr = ""
		if m.kanban == nil || m.kanban.boardID != msg.boardID {
			m.kanban = newKanbanModel(msg.boardID)
		}
		m.kanban.setStacks(msg.stacks)
		return m, nil
	case boardOpenedMsg:
		m.active = screenKanban
		m.kanban = newKanbanModel(msg.boardID)
		m.kanban.boardColor = msg.color
		m.loading = true
		cmds := []tea.Cmd{m.loadStacks(msg.boardID)}
		if msg.color == "" {
			cmds = append(cmds, m.loadBoardInfo(msg.boardID))
		}
		return m, tea.Batch(cmds...)
	case boardInfoMsg:
		if m.kanban != nil && m.kanban.boardID == msg.boardID {
			m.kanban.boardColor = msg.color
		}
		return m, nil
	case backMsg:
		switch m.active {
		case screenCard:
			m.active = screenKanban
			m.card = nil
		case screenKanban:
			m.active = screenBoards
			m.loading = true
			return m, m.loadBoards()
		}
		return m, nil
	case refreshMsg:
		m.loading = true
		switch m.active {
		case screenBoards:
			return m, m.loadBoards()
		case screenKanban:
			return m, m.loadStacks(m.kanban.boardID)
		case screenCard:
			if m.card != nil {
				return m, tea.Batch(
					m.loadCard(m.card.boardID, m.card.stackID, m.card.cardID),
					m.loadComments(m.card.cardID),
					m.loadAttachments(m.card.cardID),
				)
			}
		}
	case openCardMsg:
		m.loading = false
		m.errStr = ""
		m.active = screenCard
		if m.card == nil {
			m.card = &cardModel{}
		}
		m.card.boardID = msg.boardID
		m.card.setCard(msg.card, m.width, m.height)
		return m, tea.Batch(m.loadComments(msg.card.ID), m.loadAttachments(msg.card.ID))
	case cardLoadedMsg:
		m.loading = false
		m.errStr = ""
		m.active = screenCard
		if m.card == nil {
			m.card = &cardModel{}
		}
		m.card.setCard(msg.card, m.width, m.height)
		return m, tea.Batch(m.loadComments(msg.card.ID), m.loadAttachments(msg.card.ID))
	case commentsLoadedMsg:
		if m.card != nil && m.card.cardID == msg.cardID {
			m.card.setComments(msg.comments)
		}
		return m, nil
	case attachmentsLoadedMsg:
		if m.card != nil && m.card.cardID == msg.cardID {
			m.card.setAttachments(msg.attachments)
		}
		return m, nil
	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	// Route to active screen.
	switch m.active {
	case screenBoards:
		var cmd tea.Cmd
		m.boards, cmd = m.boards.Update(msg, m)
		return m, cmd
	case screenKanban:
		if m.kanban == nil {
			return m, nil
		}
		return m.kanban.Update(msg, m)
	case screenCard:
		if m.card == nil {
			return m, nil
		}
		return m.card.Update(msg, m)
	}
	return m, nil
}

func (m *Model) View() string {
	header := titleStyle.Render("ncdeck")
	if m.loading {
		header += "  " + m.spinner.View()
	}
	if m.status != "" {
		header += "  " + subtleStyle.Render(m.status)
	}

	var body string
	switch m.active {
	case screenBoards:
		body = m.boards.View(m.width, m.height-2)
	case screenKanban:
		if m.kanban != nil {
			body = m.kanban.View(m.width, m.height-2)
		}
	case screenCard:
		if m.card != nil {
			body = m.card.View(m.width, m.height-2)
		}
	}

	footer := ""
	if m.errStr != "" {
		footer = lipgloss.NewStyle().Foreground(colDanger).Render("error: " + m.errStr)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// --- async loaders -------------------------------------------------------

func (m *Model) loadBoards() tea.Cmd {
	return func() tea.Msg {
		bs, err := m.client.ListBoards(m.ctx, false)
		if err != nil {
			return errMsg{err}
		}
		// Filter archived for the picker by default.
		filt := bs[:0]
		for _, b := range bs {
			if !b.Archived {
				filt = append(filt, b)
			}
		}
		return boardsLoadedMsg{filt}
	}
}

func (m *Model) loadStacks(boardID int) tea.Cmd {
	return func() tea.Msg {
		ss, err := m.client.ListStacks(m.ctx, boardID)
		if err != nil {
			return errMsg{err}
		}
		return stacksLoadedMsg{boardID, ss}
	}
}

func (m *Model) loadCard(boardID, stackID, cardID int) tea.Cmd {
	return func() tea.Msg {
		k, err := m.client.GetCard(m.ctx, boardID, stackID, cardID)
		if err != nil {
			return errMsg{err}
		}
		return cardLoadedMsg{k}
	}
}

func (m *Model) loadComments(cardID int) tea.Cmd {
	return func() tea.Msg {
		cs, err := m.client.ListComments(m.ctx, cardID, 50, 0)
		if err != nil {
			// Comments are non-fatal — return empty.
			return commentsLoadedMsg{cardID, nil}
		}
		return commentsLoadedMsg{cardID, cs}
	}
}

func (m *Model) loadBoardInfo(boardID int) tea.Cmd {
	return func() tea.Msg {
		b, err := m.client.GetBoard(m.ctx, boardID)
		if err != nil {
			// Non-fatal: just keep the default accent.
			return boardInfoMsg{boardID: boardID, color: ""}
		}
		return boardInfoMsg{boardID: boardID, color: b.Color}
	}
}

func (m *Model) loadAttachments(cardID int) tea.Cmd {
	return func() tea.Msg {
		as, err := m.client.ListAttachments(m.ctx, cardID)
		if err != nil {
			return attachmentsLoadedMsg{cardID, nil}
		}
		return attachmentsLoadedMsg{cardID, as}
	}
}

// statusf is a convenience for setting a transient status line.
func (m *Model) statusf(format string, a ...any) {
	m.status = fmt.Sprintf(format, a...)
}

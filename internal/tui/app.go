// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"context"

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

// chromeRows is what View()'s header + footer subtract from height before
// handing the remainder to each screen.
const chromeRows = 2

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

// initialBoardID 0 starts at the picker; non-zero jumps straight to that board.
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
		if m.kanban != nil {
			m.kanban.width = msg.Width
		}
		if m.card != nil {
			m.card.resize(m.width, m.height-chromeRows)
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case errMsg:
		m.loading = false
		m.errStr = msg.err.Error()
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
			m.kanban.width = m.width
		}
		m.kanban.setStacks(msg.stacks)
		return m, nil
	case boardOpenedMsg:
		m.active = screenKanban
		m.kanban = newKanbanModel(msg.boardID)
		m.kanban.boardColor = msg.color
		m.kanban.width = m.width
		m.loading = true
		// loadBoardInfo also fetches the label palette for the label dialog;
		// the boards picker only carries the color, so we always re-fetch.
		return m, tea.Batch(m.loadStacks(msg.boardID), m.loadBoardInfo(msg.boardID))
	case boardInfoMsg:
		if m.onBoard(msg.boardID) {
			m.kanban.boardColor = msg.color
			m.kanban.boardLabels = msg.labels
		}
		return m, nil
	case backMsg:
		switch m.active {
		case screenCard:
			m.active = screenKanban
			m.card = nil
			// Refresh the kanban so changes made in the card detail (labels,
			// description, archive, etc.) show up immediately on return.
			if m.kanban != nil {
				m.loading = true
				return m, m.loadStacks(m.kanban.boardID)
			}
		case screenKanban:
			m.active = screenBoards
			m.kanban = nil
			m.loading = true
			return m, m.loadBoards()
		}
		return m, nil
	case refreshMsg:
		switch m.active {
		case screenBoards:
			m.loading = true
			return m, m.loadBoards()
		case screenKanban:
			m.loading = true
			return m, m.loadStacks(m.kanban.boardID)
		case screenCard:
			if m.card == nil {
				return m, nil
			}
			m.loading = true
			return m, tea.Batch(
				m.loadCard(m.card.boardID, m.card.stackID, m.card.cardID),
				m.loadComments(m.card.cardID),
				m.loadAttachments(m.card.boardID, m.card.stackID, m.card.cardID),
			)
		}
		return m, nil
	case openCardMsg:
		// Show the cached card immediately, then refresh from the server in
		// the background; cardLoadedMsg replaces m.card.card when it arrives.
		enter := m.enterCard(msg.boardID, msg.card)
		refresh := m.loadCard(msg.boardID, msg.card.StackID, msg.card.ID)
		return m, tea.Batch(enter, refresh)
	case cardLoadedMsg:
		// Background card refresh; refreshMsg fires comments/attachments
		// alongside this, so cardLoadedMsg only patches the card object.
		if m.card != nil && m.card.cardID == msg.card.ID {
			m.card.card = msg.card
			m.card.stackID = msg.card.StackID
			m.card.refreshBody()
		}
		return m, nil
	case reorderedMsg:
		if m.onBoard(msg.boardID) {
			m.kanban.reorderInFlight = false
		}
		return m, nil
	case reorderFailedMsg:
		m.errStr = msg.err.Error()
		if m.onBoard(msg.boardID) {
			m.kanban.reorderInFlight = false
			m.loading = true
			return m, m.loadStacks(msg.boardID)
		}
		return m, nil
	case labelCreatedMsg:
		if m.onBoard(msg.boardID) {
			m.kanban.boardLabels = append(m.kanban.boardLabels, msg.label)
			if m.kanban.labelMgrOpen {
				m.kanban.labelMgr.onLabelCreated(msg.label)
			}
		}
		if m.card != nil && m.card.mode == cardModeEditLabels {
			m.card.labels.adoptCreated(msg.label)
		}
		// loadCard -> cardLoadedMsg -> enterCard takes care of comments
		// and attachments, no need to double-fire here.
		if m.card != nil {
			return m, m.loadCard(m.card.boardID, m.card.stackID, m.card.cardID)
		}
		if m.kanban != nil {
			return m, m.loadStacks(m.kanban.boardID)
		}
		return m, nil
	case labelUpdatedMsg:
		if m.onBoard(msg.boardID) {
			for i := range m.kanban.boardLabels {
				if m.kanban.boardLabels[i].ID == msg.label.ID {
					m.kanban.boardLabels[i] = msg.label
					break
				}
			}
			if m.kanban.labelMgrOpen {
				m.kanban.labelMgr.onLabelUpdated(msg.label)
			}
			return m, m.loadStacks(msg.boardID)
		}
		return m, nil
	case labelDeletedMsg:
		if m.onBoard(msg.boardID) {
			for i := range m.kanban.boardLabels {
				if m.kanban.boardLabels[i].ID == msg.labelID {
					m.kanban.boardLabels = append(m.kanban.boardLabels[:i], m.kanban.boardLabels[i+1:]...)
					break
				}
			}
			delete(m.kanban.labelFilter, msg.labelID)
			if m.kanban.labelMgrOpen {
				m.kanban.labelMgr.onLabelDeleted(msg.labelID)
			}
			return m, m.loadStacks(msg.boardID)
		}
		return m, nil
	case labelOpFailedMsg:
		m.errStr = msg.err.Error()
		if m.onBoard(msg.boardID) && m.kanban.labelMgrOpen {
			m.kanban.labelMgr.onLabelOpFailed()
		}
		return m, nil
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
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}

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
		body = m.boards.View(m.width, m.height-chromeRows)
	case screenKanban:
		if m.kanban != nil {
			body = m.kanban.View(m.width, m.height-chromeRows)
		}
	case screenCard:
		if m.card != nil {
			body = m.card.View(m.width, m.height-chromeRows)
		}
	}

	footer := ""
	if m.errStr != "" {
		footer = lipgloss.NewStyle().Foreground(colDanger).Render("error: " + m.errStr)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *Model) loadBoards() tea.Cmd {
	return func() tea.Msg {
		bs, err := m.client.ListBoards(m.ctx, false)
		if err != nil {
			return errMsg{err}
		}
		filt := make([]api.Board, 0, len(bs))
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
		return cardLoadedMsg{boardID: boardID, card: k}
	}
}

func (m *Model) loadComments(cardID int) tea.Cmd {
	return func() tea.Msg {
		cs, err := m.client.ListComments(m.ctx, cardID, 50, 0)
		if err != nil {
			return commentsLoadedMsg{cardID, nil} // non-fatal
		}
		return commentsLoadedMsg{cardID, cs}
	}
}

func (m *Model) loadBoardInfo(boardID int) tea.Cmd {
	return func() tea.Msg {
		b, err := m.client.GetBoard(m.ctx, boardID)
		if err != nil {
			return boardInfoMsg{boardID: boardID} // non-fatal, keep defaults
		}
		return boardInfoMsg{boardID: boardID, color: b.Color, labels: b.Labels}
	}
}

func (m *Model) loadAttachments(boardID, stackID, cardID int) tea.Cmd {
	return func() tea.Msg {
		as, err := m.client.ListAttachments(m.ctx, boardID, stackID, cardID)
		if err != nil {
			// 404 means "no attachments"; anything else (auth, network) is
			// a real failure the user should see.
			if ae, ok := err.(*api.APIError); ok && ae.Status == 404 {
				return attachmentsLoadedMsg{cardID, nil}
			}
			return errMsg{err}
		}
		return attachmentsLoadedMsg{cardID, as}
	}
}

func (m *Model) setStatus(text string) {
	m.status = text
}

func (m *Model) accent() lipgloss.Color {
	if m.kanban != nil {
		return m.kanban.accentColor()
	}
	return colSelected
}

func (m *Model) onBoard(boardID int) bool {
	return m.kanban != nil && m.kanban.boardID == boardID
}

func (m *Model) enterCard(boardID int, card *api.Card) tea.Cmd {
	if card == nil {
		return nil
	}
	m.loading = false
	m.errStr = ""
	m.active = screenCard
	// Fresh cardModel zero-value already has mode = cardModeView. Refresh
	// (cardLoadedMsg) preserves whatever mode the user is in so a save +
	// quick 'e' to edit again doesn't lose the editor content when the
	// refresh response arrives.
	if m.card == nil {
		m.card = &cardModel{}
	}
	m.card.boardID = boardID
	m.card.accent = m.accent()
	m.card.setCard(card, m.width, m.height-chromeRows)
	return tea.Batch(
		m.loadComments(card.ID),
		m.loadAttachments(boardID, card.StackID, card.ID),
	)
}

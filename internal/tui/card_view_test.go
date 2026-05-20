// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

func cardModelFixture() (*cardModel, *Model) {
	m := &cardModel{}
	m.setCard(&api.Card{ID: 7, StackID: 3, Title: "do the thing", Description: "deets"}, 80, 22)
	return m, &Model{}
}

func cardKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}}

// renderedModalDims wraps a vp-sized cell in modalStyle and reports the
// resulting visible width/height (lipgloss strips ANSI for measurement)
// so the test can assert vp + chrome = parent.
func renderedModalDims(vpW, vpH int) (w, h int) {
	cell := strings.TrimRight(strings.Repeat(strings.Repeat("x", vpW)+"\n", vpH), "\n")
	out := modalStyle.Render(cell)
	return lipgloss.Width(out), lipgloss.Height(out)
}

func TestSetCard_ViewportFitsParentArea(t *testing.T) {
	m := &cardModel{}
	parentW, parentH := 80, 22 // m.height-chromeRows with m.height=24
	m.setCard(&api.Card{ID: 1, StackID: 1, Title: "t"}, parentW, parentH)
	// The modal must fit inside (parentW, parentH).
	w, h := renderedModalDims(m.vp.Width, m.vp.Height)
	if w > parentW {
		t.Errorf("modal width: got %d, parent %d (modal overflows the right edge by %d cols)", w, parentW, w-parentW)
	}
	if h > parentH {
		t.Errorf("modal height: got %d, parent %d (modal overflows the bottom by %d rows)", h, parentH, h-parentH)
	}
}

func TestResize_AgreesWithSetCard(t *testing.T) {
	m := &cardModel{}
	m.setCard(&api.Card{ID: 1, StackID: 1, Title: "t"}, 80, 22)
	openW, openH := m.vp.Width, m.vp.Height
	m.resize(80, 22)
	if m.vp.Width != openW || m.vp.Height != openH {
		t.Errorf("resize disagreed with setCard for the same dimensions: open=%dx%d resize=%dx%d",
			openW, openH, m.vp.Width, m.vp.Height)
	}
}

func TestCardView_TKeyOpensTitleEditorWithCurrentTitle(t *testing.T) {
	m, root := cardModelFixture()
	if _, _ = m.Update(cardKey("t"), root); m.mode != cardModeEditTitle {
		t.Fatalf("'t' should enter cardModeEditTitle, got mode=%d", m.mode)
	}
	if got := m.titleI.Value(); got != "do the thing" {
		t.Errorf("title input should be pre-filled with current title; got %q", got)
	}
}

func TestCardView_BKeyOpensDescriptionEditorWithCurrentDescription(t *testing.T) {
	m, root := cardModelFixture()
	if _, _ = m.Update(cardKey("b"), root); m.mode != cardModeEditDescription {
		t.Fatalf("'b' should enter cardModeEditDescription, got mode=%d", m.mode)
	}
	if got := m.editor.Value(); got != "deets" {
		t.Errorf("description editor should be pre-filled with current description; got %q", got)
	}
}

// Pinned: 'e' was the old description shortcut. It must no longer trigger anything.
func TestCardView_EKeyIsNoLongerBound(t *testing.T) {
	m, root := cardModelFixture()
	_, _ = m.Update(cardKey("e"), root)
	if m.mode != cardModeView {
		t.Errorf("'e' must not change mode anymore; got %d", m.mode)
	}
}

func TestCardView_QKeyQuits(t *testing.T) {
	m, root := cardModelFixture()
	_, cmd := m.Update(cardKey("q"), root)
	if cmd == nil {
		t.Fatalf("'q' should return a non-nil cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("'q' should return tea.Quit; got %T", cmd())
	}
}

func TestCardView_EscGoesBack(t *testing.T) {
	m, root := cardModelFixture()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc}, root)
	if cmd == nil {
		t.Fatalf("'esc' should return a non-nil cmd")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Errorf("'esc' should return backMsg; got %T", cmd())
	}
}

func TestCardView_EmptyTitleOnEnterIsRejected(t *testing.T) {
	m, root := cardModelFixture()
	_, _ = m.Update(cardKey("t"), root)
	m.titleI.SetValue("   ")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if m.mode != cardModeEditTitle {
		t.Errorf("empty title must keep editor open, got mode=%d", m.mode)
	}
	if cmd != nil {
		t.Errorf("empty title must not fire an update cmd")
	}
	if m.titleErr == "" {
		t.Errorf("empty title should set m.titleErr so the error renders inline in the form footer")
	}
}

// Pinned: inline error must clear on typing, not linger after the user has reacted.
func TestCardView_TitleErrorClearsOnTyping(t *testing.T) {
	m, root := cardModelFixture()
	_, _ = m.Update(cardKey("t"), root)
	m.titleI.SetValue("")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if m.titleErr == "" {
		t.Fatalf("setup: empty ⏎ should have set titleErr")
	}
	_, _ = m.Update(cardKey("x"), root)
	if m.titleErr != "" {
		t.Errorf("typing should clear the inline title error; got %q", m.titleErr)
	}
}

func TestCardView_NonEmptyTitleOnEnterFiresUpdateAndExitsEditor(t *testing.T) {
	m, root := cardModelFixture()
	_, _ = m.Update(cardKey("t"), root)
	m.titleI.SetValue("renamed")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}, root)
	if m.mode != cardModeEditTitle && m.mode != cardModeView {
		t.Errorf("after enter, mode should revert to view, got %d", m.mode)
	}
	if m.mode != cardModeView {
		t.Errorf("mode should revert to view after submitting non-empty title")
	}
	if cmd == nil {
		t.Errorf("non-empty title should fire an update cmd")
	}
}

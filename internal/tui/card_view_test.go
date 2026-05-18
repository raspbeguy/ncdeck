// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/raspbeguy/ncdeck/internal/api"
)

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

package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleCommentKey routes keys while the comment input (shown in place
// of the footer) is open. Printable keys are typed text; enter saves the
// comment on the selected unit and esc discards it.
func (m *Model) handleCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.closeCommentInput()
		return m, nil
	case "enter":
		if text := strings.TrimSpace(m.commentInput.Value()); text != "" {
			if key := m.selectedKey(); key != "" {
				m.state.AddComment(key, text)
				m.refreshContent()
			}
		}
		m.closeCommentInput()
		return m, nil
	}
	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
	return m, cmd
}

func (m *Model) closeCommentInput() {
	m.commenting = false
	m.commentInput.Blur()
	m.commentInput.SetValue("")
}

// hasComments reports whether the reviewer left comments on a unit.
func (m *Model) hasComments(key string) bool {
	return len(m.state.CommentsFor(key)) > 0
}

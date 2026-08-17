package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeClaude installs a stand-in for the claude CLI that ignores its
// arguments and prints a fixed answer.
func fakeClaude(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATHFINDER_CLAUDE_CMD", path)
}

func typeRunes(m *Model, s string) {
	for _, r := range s {
		m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// runCmd executes a tea.Cmd (unwrapping batches) until it yields an
// askResultMsg.
func runCmd(t *testing.T, cmd tea.Cmd) askResultMsg {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case askResultMsg:
			return msg
		case tea.BatchMsg:
			queue = append(queue, msg...)
		}
	}
	t.Fatal("command yielded no askResultMsg")
	return askResultMsg{}
}

func TestAskPaneQuestionFlow(t *testing.T) {
	fakeClaude(t, "echo モック回答です")
	m := exampleModel(t)

	// 'a' focuses the ask pane; typed keys go to the input, so 'q' must
	// not quit and 'j' must not move the selection.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.focus != paneAsk {
		t.Fatalf("a should focus the ask pane, focus = %d", m.focus)
	}
	before := m.cursor
	typeRunes(m, "qjこの関数は?")
	if m.cursor != before {
		t.Error("typing must not move the tree cursor")
	}
	if got := m.askInput.Value(); got != "qjこの関数は?" {
		t.Fatalf("input = %q", got)
	}

	// Enter submits: the question enters the log, the pane shows the
	// waiting state, and the returned command yields the answer.
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.asking {
		t.Fatal("enter should start a question")
	}
	if m.askInput.Value() != "" {
		t.Error("enter should clear the input")
	}
	if !strings.Contains(m.View(), "回答を待っています") {
		t.Error("view should show the waiting state")
	}

	msg := runCmd(t, cmd)
	if msg.err != nil {
		t.Fatalf("fake claude failed: %v", msg.err)
	}
	m.Update(msg)
	if m.asking {
		t.Error("result should clear the asking state")
	}
	view := m.View()
	for _, want := range []string{"この関数は?", "モック回答です"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q", want)
		}
	}

	// esc returns focus to the tree.
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.focus != paneTree {
		t.Errorf("esc should focus the tree, focus = %d", m.focus)
	}
}

func TestAskErrorIsShown(t *testing.T) {
	fakeClaude(t, "echo 認証に失敗しました >&2; exit 1")
	m := exampleModel(t)
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	typeRunes(m, "test")
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(runCmd(t, cmd))
	if !strings.Contains(m.View(), "認証に失敗しました") {
		t.Error("view should show the CLI error")
	}
}

func TestAskPromptCarriesSelection(t *testing.T) {
	m := exampleModel(t)
	p := m.askPrompt("なぜ?")
	for _, want := range []string{
		"ユーザー招待機能の追加",                  // plan title
		"internal/model/invitation.go", // selected file
		"なぜ?",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestAskIgnoresEmptyAndDuplicateSubmit(t *testing.T) {
	fakeClaude(t, "echo x")
	m := exampleModel(t)
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("empty input should not submit")
	}
	typeRunes(m, "q1")
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	typeRunes(m, "q2")
	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("a second question must wait for the first answer")
	}
	if len(m.askLog) != 1 {
		t.Errorf("log should hold one entry, got %d", len(m.askLog))
	}
}

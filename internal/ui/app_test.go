package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uho-wq/pathfinder/internal/plan"
)

func exampleModel(t *testing.T) *Model {
	t.Helper()
	p, err := plan.Load(filepath.Join("..", "..", "examples", "review.json"))
	if err != nil {
		t.Fatal(err)
	}
	st := plan.LoadState(filepath.Join(t.TempDir(), "review.json"))
	m := New(p, st, "")
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	return m
}

func TestViewRendersAllPanes(t *testing.T) {
	m := exampleModel(t)
	view := m.View()
	for _, want := range []string{
		"ファイル", "差分", "レビューガイド", // pane titles
		"ユーザー招待機能の追加",              // header title
		"0/3 レビュー済",                 // progress
		"internal/model/invitation.go", // first file selected
		"レビュー観点",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q", want)
		}
	}
}

func TestNavigationAndToggle(t *testing.T) {
	m := exampleModel(t)

	if f := m.selectedFile(); f == nil || f.Path != "internal/model/invitation.go" {
		t.Fatalf("initial selection = %v", f)
	}

	// space marks reviewed and advances to the next file.
	m.handleKey(tea.KeyMsg{Type: tea.KeySpace})
	if !m.state.Reviewed["internal/model/invitation.go"] {
		t.Error("space should mark the file reviewed")
	}
	if f := m.selectedFile(); f == nil || f.Path != "internal/service/invitation.go" {
		t.Errorf("space should advance selection, got %v", f)
	}

	// p goes back.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if f := m.selectedFile(); f == nil || f.Path != "internal/model/invitation.go" {
		t.Errorf("p should go back, got %v", f)
	}

	// G jumps to the last file.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if f := m.selectedFile(); f == nil || f.Path != "internal/handler/invitation.go" {
		t.Errorf("G should jump to last file, got %v", f)
	}
}

func TestEmbeddedDiffIsUsed(t *testing.T) {
	m := exampleModel(t)
	f := m.selectedFile()
	d := m.diffFor(f)
	if !strings.Contains(d, "type Invitation struct") {
		t.Errorf("embedded diff should be returned, got %q", d)
	}
}

func TestFocusCycles(t *testing.T) {
	m := exampleModel(t)
	if m.focus != paneTree {
		t.Fatal("initial focus should be tree")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != paneDiff {
		t.Error("tab should move focus to diff")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != paneTree {
		t.Error("focus should wrap around to tree")
	}
}

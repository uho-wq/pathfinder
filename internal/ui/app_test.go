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
		"ファイル", "差分", "レビューガイド", "PRディスクリプション", // pane titles
		"招待トークンのデータモデルを追加",             // PR description in the bottom-right pane
		"ユーザー招待機能の追加",                  // header title
		"0/4 レビュー済",                    // progress counts units: 2 files + 2 sections
		"internal/model/invitation.go", // first file selected
		"CreateInvitation",             // section row in the tree
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

	// space marks reviewed and advances to the next unit: the first
	// section of the service file.
	m.handleKey(tea.KeyMsg{Type: tea.KeySpace})
	if !m.state.Reviewed["internal/model/invitation.go"] {
		t.Error("space should mark the file reviewed")
	}
	f := m.selectedFile()
	sec, idx := m.selectedSection()
	if f == nil || f.Path != "internal/service/invitation.go" || sec == nil || idx != 0 {
		t.Fatalf("space should advance to first section, got file %v section %d", f, idx)
	}

	// space on a section marks that section only and moves to the next.
	m.handleKey(tea.KeyMsg{Type: tea.KeySpace})
	if !m.state.Reviewed["internal/service/invitation.go#0"] {
		t.Error("space should mark the section reviewed")
	}
	if m.state.Reviewed["internal/service/invitation.go"] {
		t.Error("section toggle should not mark the whole file")
	}
	if _, idx := m.selectedSection(); idx != 1 {
		t.Errorf("space should advance to next section, got %d", idx)
	}

	// p jumps back to the previous file's first unit.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if f := m.selectedFile(); f == nil || f.Path != "internal/model/invitation.go" {
		t.Errorf("p should go back a file, got %v", f)
	}

	// n skips over the remaining sections to the next file each time.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if f := m.selectedFile(); f == nil || f.Path != "internal/service/invitation.go" {
		t.Errorf("n should enter the service file, got %v", f)
	}
	if _, idx := m.selectedSection(); idx != 0 {
		t.Errorf("n should land on the file's first section, got %d", idx)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if f := m.selectedFile(); f == nil || f.Path != "internal/handler/invitation.go" {
		t.Errorf("n should skip sections to the handler file, got %v", f)
	}

	// G jumps to the last unit.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if f := m.selectedFile(); f == nil || f.Path != "internal/handler/invitation.go" {
		t.Errorf("G should jump to last unit, got %v", f)
	}
}

func TestSectionDiffScrollsToSection(t *testing.T) {
	m := exampleModel(t)
	// Move to the first section of the service file (line 3 of the
	// embedded diff) and check the diff viewport scrolled off the top.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if _, idx := m.selectedSection(); idx != 0 {
		t.Fatalf("expected a section selected, got %d", idx)
	}
	if m.diffOffset == 0 {
		t.Error("selecting a section should locate its rows in the diff")
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
	if m.focus != paneGuide {
		t.Error("tab should move focus to guide")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != paneDesc {
		t.Error("tab should move focus to the description pane")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != paneTree {
		t.Error("focus should wrap around to tree")
	}
}

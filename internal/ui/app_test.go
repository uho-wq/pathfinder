package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
		"ファイル", "差分", "呼び出し先", "レビューガイド", "PRディスクリプション", "Claudeに質問", // pane titles
		"招待トークンのデータモデルを追加",             // PR description in the bottom-right pane
		"ユーザー招待機能の追加",                  // header title
		"0/4 レビュー済",                    // progress counts units: 2 files + 2 sections
		"internal/model/invitation.go", // first file selected
		"CreateInvitation",             // section row in the tree
		"ステップ全体の変化",                    // guide sections
		"この項目の変化",
		"なぜこの処理か",
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
	for _, want := range []pane{paneDiff, paneCallee, paneGuide, paneDesc, paneAsk, paneTree} {
		m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
		if m.focus != want {
			t.Errorf("tab: focus = %d, want %d", m.focus, want)
		}
	}
}

func TestFocusedPaneExpands(t *testing.T) {
	m := exampleModel(t)

	// The tree starts focused, so its column holds its expanded width;
	// moving focus to the diff shrinks it back and widens the diff.
	treeFocusedW := m.treeW
	diffBeforeW := m.diff.Width
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // -> diff
	if m.treeW >= treeFocusedW {
		t.Errorf("unfocusing the tree should narrow it: %d -> %d", treeFocusedW, m.treeW)
	}
	if m.diff.Width <= diffBeforeW {
		t.Errorf("focusing the diff should widen it: %d -> %d", diffBeforeW, m.diff.Width)
	}

	calleeBeforeW := m.callee.Width
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // -> callee
	if m.callee.Width <= calleeBeforeW {
		t.Errorf("focusing the callee pane should widen it: %d -> %d", calleeBeforeW, m.callee.Width)
	}

	guideBeforeW := m.guide.Width
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // -> guide
	if m.guide.Width <= guideBeforeW {
		t.Errorf("focusing the guide should widen it: %d -> %d", guideBeforeW, m.guide.Width)
	}

	descBeforeH := m.desc.Height
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // -> desc
	if m.desc.Height <= descBeforeH {
		t.Errorf("focusing the description should heighten it: %d -> %d", descBeforeH, m.desc.Height)
	}

	askBeforeH := m.askView.Height
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // -> ask
	if m.askView.Height <= askBeforeH {
		t.Errorf("focusing the ask pane should heighten it: %d -> %d", askBeforeH, m.askView.Height)
	}
	if m.treeW != treeFocusedW {
		t.Errorf("the ask pane shares the left column: treeW = %d, want %d", m.treeW, treeFocusedW)
	}

	// The expanded layout still fits the terminal.
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab}) // -> tree
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > m.width {
			t.Fatalf("view line overflows terminal: %d > %d", w, m.width)
		}
	}
}

func TestCalleePaneShowsCalleeBody(t *testing.T) {
	m := exampleModel(t)
	// The first unit (the model file) has no callees of its own.
	view := m.View()
	if !strings.Contains(view, "呼び出し先") {
		t.Fatal("callee pane should be shown for plans with callees")
	}
	// Move to the first section of the service file, whose callee embeds
	// the mail sender's body.
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	view = m.View()
	for _, want := range []string{"mail.Sender.Send", "internal/mail/sender.go"} {
		if !strings.Contains(view, want) {
			t.Errorf("callee pane should contain %q", want)
		}
	}
}

func TestCalleePaneHiddenWithoutCallees(t *testing.T) {
	p := &plan.Plan{
		Title: "t",
		Steps: []plan.Step{{Name: "s", Files: []plan.File{{Path: "a.go", Diff: "x"}}}},
	}
	st := plan.LoadState(filepath.Join(t.TempDir(), "review.json"))
	m := New(p, st, "")
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 48})
	if strings.Contains(m.View(), "呼び出し先") {
		t.Error("callee pane should be hidden for plans without callees")
	}
	// The focus cycle skips the callee pane in both directions.
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != paneGuide {
		t.Errorf("tab should skip the callee pane, focus = %d", m.focus)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus != paneDiff {
		t.Errorf("shift+tab should skip the callee pane, focus = %d", m.focus)
	}
}

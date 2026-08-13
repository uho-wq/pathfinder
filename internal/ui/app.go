// Package ui implements the three-pane review TUI: an ordered file tree
// on the left, the file diff in the center, and the AI-authored review
// guide on the right.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"

	"github.com/uho-wq/pathfinder/internal/gitdiff"
	"github.com/uho-wq/pathfinder/internal/plan"
)

type pane int

const (
	paneTree pane = iota
	paneDiff
	paneGuide
	paneCount
)

type rowKind int

const (
	rowStep rowKind = iota
	rowFile
)

// row is one visible line of the tree pane, either a step header or a
// selectable file.
type row struct {
	kind rowKind
	step int
	file int
}

// Model is the Bubble Tea model for the whole application.
type Model struct {
	plan    *plan.Plan
	state   *plan.State
	repoDir string

	rows    []row
	cursor  int // index into rows; always on a rowFile once initialized
	treeTop int // first visible tree row (scroll offset)

	focus pane
	diff  viewport.Model
	guide viewport.Model

	width, height int
	treeW, treeH  int
	ready         bool

	diffCache map[string]string
}

// New builds the application model. repoDir overrides the plan's
// repo_dir when non-empty.
func New(p *plan.Plan, st *plan.State, repoDir string) *Model {
	if repoDir == "" {
		repoDir = p.RepoDir
	}
	m := &Model{
		plan:      p,
		state:     st,
		repoDir:   repoDir,
		focus:     paneTree,
		diffCache: map[string]string{},
	}
	for i, s := range p.Steps {
		m.rows = append(m.rows, row{kind: rowStep, step: i})
		for j := range s.Files {
			m.rows = append(m.rows, row{kind: rowFile, step: i, file: j})
		}
	}
	m.cursor = m.nextFileRow(-1, +1)
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		if !m.ready {
			m.ready = true
			m.loadSelection()
		} else {
			m.refreshContent()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab", "l", "right":
		m.focus = (m.focus + 1) % paneCount
		return m, nil
	case "shift+tab", "h", "left":
		m.focus = (m.focus + paneCount - 1) % paneCount
		return m, nil

	case "n":
		m.moveFile(+1)
		return m, nil
	case "p":
		m.moveFile(-1)
		return m, nil

	case " ", "r":
		if f := m.selectedFile(); f != nil {
			m.state.Toggle(f.Path)
			if msg.String() == " " {
				m.moveFile(+1)
			}
		}
		return m, nil

	case "enter":
		if m.focus == paneTree {
			m.focus = paneDiff
		}
		return m, nil
	}

	switch m.focus {
	case paneTree:
		switch msg.String() {
		case "j", "down":
			m.moveFile(+1)
		case "k", "up":
			m.moveFile(-1)
		case "g":
			m.cursor = m.nextFileRow(-1, +1)
			m.loadSelection()
		case "G":
			m.cursor = m.nextFileRow(len(m.rows), -1)
			m.loadSelection()
		}
	case paneDiff:
		m.diff = m.scrollKeys(m.diff, msg)
	case paneGuide:
		m.guide = m.scrollKeys(m.guide, msg)
	}
	return m, nil
}

func (m *Model) scrollKeys(vp viewport.Model, msg tea.KeyMsg) viewport.Model {
	switch msg.String() {
	case "j", "down":
		vp.ScrollDown(1)
	case "k", "up":
		vp.ScrollUp(1)
	case "d", "ctrl+d", "pgdown":
		vp.HalfPageDown()
	case "u", "ctrl+u", "pgup":
		vp.HalfPageUp()
	case "g":
		vp.GotoTop()
	case "G":
		vp.GotoBottom()
	}
	return vp
}

// nextFileRow returns the index of the first rowFile found walking from
// `from` (exclusive) in direction dir, or -1 wrapped to current position.
func (m *Model) nextFileRow(from, dir int) int {
	for i := from + dir; i >= 0 && i < len(m.rows); i += dir {
		if m.rows[i].kind == rowFile {
			return i
		}
	}
	return from
}

func (m *Model) moveFile(dir int) {
	next := m.nextFileRow(m.cursor, dir)
	if next != m.cursor && next >= 0 && next < len(m.rows) {
		m.cursor = next
		m.loadSelection()
	}
}

func (m *Model) selectedFile() *plan.File {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	r := m.rows[m.cursor]
	if r.kind != rowFile {
		return nil
	}
	return &m.plan.Steps[r.step].Files[r.file]
}

func (m *Model) selectedStep() *plan.Step {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.plan.Steps[m.rows[m.cursor].step]
}

// loadSelection refreshes both viewports for the file under the cursor
// and resets their scroll position.
func (m *Model) loadSelection() {
	if !m.ready {
		return
	}
	m.refreshContent()
	m.diff.GotoTop()
	m.guide.GotoTop()
	m.scrollTreeIntoView()
}

// refreshContent re-renders viewport contents at the current sizes
// without touching scroll positions (used on resize).
func (m *Model) refreshContent() {
	f := m.selectedFile()
	st := m.selectedStep()
	if f == nil || st == nil {
		m.guide.SetContent(renderOverview(m.plan, m.guide.Width))
		return
	}
	m.diff.SetContent(renderDiff(m.diffFor(f), m.diff.Width))
	m.guide.SetContent(renderGuide(st, f, m.guide.Width))
}

func (m *Model) diffFor(f *plan.File) string {
	if f.Diff != "" {
		return f.Diff
	}
	if d, ok := m.diffCache[f.Path]; ok {
		return d
	}
	d, err := gitdiff.FileDiff(m.repoDir, m.plan.Base, m.plan.Head, f.Path)
	if err != nil {
		d = fmt.Sprintf("差分を取得できませんでした:\n%v\n\nプランに diff を埋め込むか、base/head と実行ディレクトリを確認してください。", err)
	}
	m.diffCache[f.Path] = d
	return d
}

func (m *Model) scrollTreeIntoView() {
	if m.cursor < m.treeTop {
		m.treeTop = m.cursor
	}
	if m.cursor >= m.treeTop+m.treeH {
		m.treeTop = m.cursor - m.treeH + 1
	}
	if m.treeTop < 0 {
		m.treeTop = 0
	}
}

// layout recomputes pane sizes from the terminal size.
func (m *Model) layout() {
	headerH, footerH := 1, 1
	// Each pane draws a border (2 cols, 2 rows) plus a title line.
	contentH := m.height - headerH - footerH - 2 - 1
	if contentH < 3 {
		contentH = 3
	}

	treeW := m.width * 22 / 100
	if treeW < 20 {
		treeW = 20
	}
	if treeW > 40 {
		treeW = 40
	}
	guideW := m.width * 30 / 100
	if guideW < 24 {
		guideW = 24
	}
	if guideW > 56 {
		guideW = 56
	}
	diffW := m.width - treeW - guideW - 6 // 3 panes x 2 border cols
	if diffW < 20 {
		diffW = 20
	}

	m.treeW = treeW
	m.treeH = contentH

	m.diff.Width = diffW
	m.diff.Height = contentH
	m.guide.Width = guideW
	m.guide.Height = contentH
	m.scrollTreeIntoView()
}

func (m *Model) View() string {
	if !m.ready {
		return "loading..."
	}

	header := m.renderHeader()
	tree := m.renderPane(paneTree, "ファイル", m.renderTree(), m.treeW, m.treeH)
	diff := m.renderPane(paneDiff, "差分", m.diff.View(), m.diff.Width, m.diff.Height)
	guide := m.renderPane(paneGuide, "レビューガイド", m.guide.View(), m.guide.Width, m.guide.Height)

	body := lipgloss.JoinHorizontal(lipgloss.Top, tree, diff, guide)
	footer := styleFooter.Render(
		"tab/h/l: ペイン移動  j/k: 選択/スクロール  space: レビュー済→次へ  r: 済トグル  n/p: 次/前ファイル  q: 終了")

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *Model) renderHeader() string {
	done := m.state.CountReviewed(m.plan)
	total := m.plan.TotalFiles()
	progress := fmt.Sprintf(" %d/%d レビュー済 ", done, total)

	title := m.plan.Title
	if title == "" {
		title = "pathfinder"
	}
	left := styleHeader.Render("⌘ " + title)
	right := styleHeaderInfo.Render(progress)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	filler := styleHeaderInfo.Render(strings.Repeat(" ", gap))
	return left + filler + right
}

func (m *Model) renderPane(p pane, title, content string, w, h int) string {
	titleStyle := stylePaneTitle
	paneStyle := stylePane
	if m.focus == p {
		titleStyle = stylePaneTitleFocus
		paneStyle = stylePaneFocus
	}
	head := titleStyle.Render(truncate.StringWithTail(title, uint(w), "…"))
	inner := lipgloss.JoinVertical(lipgloss.Left, head, content)
	return paneStyle.Width(w).Height(h + 1).Render(inner)
}

func (m *Model) renderTree() string {
	var lines []string
	end := m.treeTop + m.treeH
	if end > len(m.rows) {
		end = len(m.rows)
	}
	for i := m.treeTop; i < end; i++ {
		r := m.rows[i]
		var line string
		switch r.kind {
		case rowStep:
			line = styleStep.Render(truncate.StringWithTail(
				fmt.Sprintf("%d. %s", r.step+1, m.plan.Steps[r.step].Name), uint(m.treeW), "…"))
		case rowFile:
			f := m.plan.Steps[r.step].Files[r.file]
			name := truncate.StringWithTail(f.Path, uint(m.treeW-6), "…")
			if i == m.cursor {
				// The cursor line is styled as a whole; nested styles
				// would reset its background mid-line.
				mark := "  "
				if m.state.Reviewed[f.Path] {
					mark = "✓ "
				}
				line = fmt.Sprintf(" %s%s %s", mark, plainStatus(f.Status), name)
				if pad := m.treeW - lipgloss.Width(line); pad > 0 {
					line += strings.Repeat(" ", pad)
				}
				line = styleCursor.Render(line)
			} else {
				mark := "  "
				if m.state.Reviewed[f.Path] {
					mark = styleReviewed.Render("✓ ")
				}
				line = fmt.Sprintf(" %s%s %s", mark, statusIcon(f.Status), name)
			}
		}
		lines = append(lines, line)
	}
	for len(lines) < m.treeH {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// Run starts the TUI and blocks until the user quits.
func Run(p *plan.Plan, st *plan.State, repoDir string) error {
	prog := tea.NewProgram(New(p, st, repoDir), tea.WithAltScreen())
	_, err := prog.Run()
	return err
}

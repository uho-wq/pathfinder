// Package ui implements the review TUI: an ordered file tree on the
// left, the file diff in the center, the bodies of functions the change
// calls next to it (when the plan carries callee info), and on the
// right the AI-authored review guide with the PR description below it.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	paneCallee
	paneGuide
	paneDesc
	paneAsk
	paneCount

	paneNone pane = -1
)

// wheelLines is how many rows one mouse-wheel tick scrolls.
const wheelLines = 3

type rowKind int

const (
	rowStep rowKind = iota
	rowFile
	rowSection
)

// row is one visible line of the tree pane: a step header, a file, or a
// section inside a file. The cursor rests on units — sections, or files
// without sections; a file that has sections is just a header.
type row struct {
	kind rowKind
	step int
	file int
	sec  int
}

// Model is the Bubble Tea model for the whole application.
type Model struct {
	plan    *plan.Plan
	state   *plan.State
	repoDir string

	rows    []row
	cursor  int // index into rows; always on a rowFile once initialized
	treeTop int // first visible tree row (scroll offset)

	focus  pane
	diff   viewport.Model
	callee viewport.Model // callee bodies, right of the diff
	guide  viewport.Model
	desc   viewport.Model // PR description, bottom right

	// The comment input replaces the footer while the reviewer writes a
	// comment on the selected unit (key: c). Comments live in the state
	// file next to the reviewed marks.
	commentInput textinput.Model
	commenting   bool

	// The ask pane (bottom left) sends questions to the claude CLI with
	// the current selection as context and shows the transcript.
	askView  viewport.Model
	askInput textinput.Model
	askSpin  spinner.Model
	askLog   []askEntry
	asking   bool
	askID    int

	// showCallees keeps the callee pane out of the layout (and the
	// focus cycle) for plans that carry no callee info at all.
	showCallees bool

	width, height int
	treeW, treeH  int
	ready         bool

	diffCache  map[string]string
	srcCache   map[string]string // head-side file contents for callees
	diffOffset int               // row where the selected section starts in the diff
}

// New builds the application model. repoDir overrides the plan's
// repo_dir when non-empty.
func New(p *plan.Plan, st *plan.State, repoDir string) *Model {
	if repoDir == "" {
		repoDir = p.RepoDir
	}
	m := &Model{
		plan:        p,
		state:       st,
		repoDir:     repoDir,
		focus:       paneTree,
		diffCache:   map[string]string{},
		srcCache:    map[string]string{},
		showCallees: p.HasCallees(),
	}
	m.askInput = textinput.New()
	m.askInput.Prompt = "> "
	m.askInput.Placeholder = "Claudeに質問..."
	m.commentInput = textinput.New()
	m.commentInput.Prompt = " コメント> "
	m.commentInput.Placeholder = "Enterで保存 / escでキャンセル"
	m.askSpin = spinner.New(spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(colorAccent)))
	for i, s := range p.Steps {
		m.rows = append(m.rows, row{kind: rowStep, step: i})
		for j, f := range s.Files {
			m.rows = append(m.rows, row{kind: rowFile, step: i, file: j})
			for k := range f.Sections {
				m.rows = append(m.rows, row{kind: rowSection, step: i, file: j, sec: k})
			}
		}
	}
	m.cursor = m.nextUnitRow(-1, +1)
	return m
}

// isUnit reports whether the row at index i is a reviewable unit the
// cursor can rest on.
func (m *Model) isUnit(i int) bool {
	if i < 0 || i >= len(m.rows) {
		return false
	}
	r := m.rows[i]
	switch r.kind {
	case rowSection:
		return true
	case rowFile:
		return len(m.plan.Steps[r.step].Files[r.file].Sections) == 0
	}
	return false
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

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case editorFinishedMsg:
		// The edit may have changed the working tree; re-derive the diff
		// and, when callees are read from the working tree, their code.
		delete(m.diffCache, msg.path)
		if m.plan.Head == "" {
			m.srcCache = map[string]string{}
		}
		m.refreshContent()
		return m, nil

	case askResultMsg:
		if msg.id != m.askID || len(m.askLog) == 0 {
			return m, nil
		}
		m.asking = false
		e := &m.askLog[len(m.askLog)-1]
		if msg.err != nil {
			e.errText = msg.err.Error()
		} else {
			e.answer = msg.answer
		}
		m.refreshAskView(true)
		return m, nil

	case spinner.TickMsg:
		if !m.asking {
			return m, nil
		}
		var cmd tea.Cmd
		m.askSpin, cmd = m.askSpin.Update(msg)
		m.refreshAskView(false)
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While an input is open keys are typed text, not bindings.
	if m.commenting {
		return m.handleCommentKey(msg)
	}
	if m.focus == paneAsk {
		return m.handleAskKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab", "l", "right":
		return m, m.cycleFocus(+1)
	case "shift+tab", "h", "left":
		return m, m.cycleFocus(-1)

	case "a":
		m.setFocus(paneAsk)
		return m, m.askInput.Focus()

	case "c":
		if m.selectedKey() != "" {
			m.commenting = true
			m.commentInput.SetValue("")
			return m, m.commentInput.Focus()
		}
		return m, nil
	case "C":
		if key := m.selectedKey(); key != "" {
			m.state.RemoveLastComment(key)
			m.refreshContent()
		}
		return m, nil

	case "n":
		m.moveOtherFile(+1)
		return m, nil
	case "p":
		m.moveOtherFile(-1)
		return m, nil

	case " ", "r":
		if key := m.selectedKey(); key != "" {
			m.state.Toggle(key)
			if msg.String() == " " {
				m.moveUnit(+1)
			}
		}
		return m, nil

	case "e":
		return m, m.openEditor()

	case "enter":
		if m.focus == paneTree {
			m.setFocus(paneDiff)
		}
		return m, nil
	}

	switch m.focus {
	case paneTree:
		switch msg.String() {
		case "j", "down":
			m.moveUnit(+1)
		case "k", "up":
			m.moveUnit(-1)
		case "g":
			m.cursor = m.nextUnitRow(-1, +1)
			m.loadSelection()
		case "G":
			m.cursor = m.nextUnitRow(len(m.rows), -1)
			m.loadSelection()
		}
	case paneDiff:
		m.diff = m.scrollKeys(m.diff, msg)
	case paneCallee:
		m.callee = m.scrollKeys(m.callee, msg)
	case paneGuide:
		m.guide = m.scrollKeys(m.guide, msg)
	case paneDesc:
		m.desc = m.scrollKeys(m.desc, msg)
	}
	return m, nil
}

// cycleFocus moves focus to the neighboring pane, skipping the callee
// pane when it is not part of the layout, and keeps the ask input's
// focus state (its cursor) in sync.
func (m *Model) cycleFocus(dir int) tea.Cmd {
	next := (m.focus + pane(dir) + paneCount) % paneCount
	if next == paneCallee && !m.showCallees {
		next = (next + pane(dir) + paneCount) % paneCount
	}
	m.setFocus(next)
	if m.focus == paneAsk {
		return m.askInput.Focus()
	}
	m.askInput.Blur()
	return nil
}

// setFocus moves focus and recomputes the layout: pane sizes depend on
// which pane is focused, so the viewports must be resized and their
// contents re-rendered for the new widths.
func (m *Model) setFocus(p pane) {
	if m.focus == p {
		return
	}
	m.focus = p
	if m.ready {
		m.layout()
		m.refreshContent()
	}
}

// handleMouse routes wheel events to the pane under the pointer, not
// the focused one, so e.g. the diff scrolls under the wheel while the
// tree keeps focus. Focus is left untouched.
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.ready {
		return m, nil
	}
	lines := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		lines = -wheelLines
	case tea.MouseButtonWheelDown:
		lines = wheelLines
	default:
		return m, nil
	}
	switch m.paneAt(msg.X, msg.Y) {
	case paneTree:
		m.scrollTree(lines)
	case paneDiff:
		m.diff = scrollLines(m.diff, lines)
	case paneCallee:
		m.callee = scrollLines(m.callee, lines)
	case paneGuide:
		m.guide = scrollLines(m.guide, lines)
	case paneDesc:
		m.desc = scrollLines(m.desc, lines)
	case paneAsk:
		m.askView = scrollLines(m.askView, lines)
	}
	return m, nil
}

// paneAt maps terminal coordinates to the pane drawn there, mirroring
// the column widths and row splits that layout computed (each pane's
// border adds 2 columns and, with its title row, 3 extra rows).
func (m *Model) paneAt(x, y int) pane {
	if y < 1 || y >= m.height-1 { // header / footer rows
		return paneNone
	}
	right := m.treeW + 2
	if x < right {
		if y < 1+m.treeH+3 {
			return paneTree
		}
		return paneAsk
	}
	if right += m.diff.Width + 2; x < right {
		return paneDiff
	}
	if m.showCallees {
		if right += m.callee.Width + 2; x < right {
			return paneCallee
		}
	}
	if right += m.guide.Width + 2; x < right {
		if y < 1+m.guide.Height+3 {
			return paneGuide
		}
		return paneDesc
	}
	return paneNone
}

// scrollTree shifts the visible window of the tree without moving the
// cursor; the next cursor move snaps it back into view.
func (m *Model) scrollTree(lines int) {
	m.treeTop += lines
	if max := len(m.rows) - m.treeH; m.treeTop > max {
		m.treeTop = max
	}
	if m.treeTop < 0 {
		m.treeTop = 0
	}
}

func scrollLines(vp viewport.Model, lines int) viewport.Model {
	if lines < 0 {
		vp.ScrollUp(-lines)
	} else {
		vp.ScrollDown(lines)
	}
	return vp
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

// nextUnitRow returns the index of the first unit row found walking from
// `from` (exclusive) in direction dir, or `from` when there is none.
func (m *Model) nextUnitRow(from, dir int) int {
	for i := from + dir; i >= 0 && i < len(m.rows); i += dir {
		if m.isUnit(i) {
			return i
		}
	}
	return from
}

func (m *Model) moveUnit(dir int) {
	next := m.nextUnitRow(m.cursor, dir)
	if next != m.cursor && next >= 0 && next < len(m.rows) {
		m.cursor = next
		m.loadSelection()
	}
}

// moveOtherFile jumps to the first unit of the next (or previous) file,
// skipping the remaining sections of the current one.
func (m *Model) moveOtherFile(dir int) {
	cur := m.cursor
	if cur < 0 || cur >= len(m.rows) {
		return
	}
	from := m.rows[cur]
	for i := cur + dir; i >= 0 && i < len(m.rows); i += dir {
		r := m.rows[i]
		if !m.isUnit(i) || (r.step == from.step && r.file == from.file) {
			continue
		}
		// Walking backwards lands on a file's last section; rewind to
		// its first unit so files are always entered from the top.
		for dir < 0 && i > 0 && m.isUnit(i-1) &&
			m.rows[i-1].step == r.step && m.rows[i-1].file == r.file {
			i--
		}
		m.cursor = i
		m.loadSelection()
		return
	}
}

func (m *Model) selectedFile() *plan.File {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	r := m.rows[m.cursor]
	if r.kind != rowFile && r.kind != rowSection {
		return nil
	}
	return &m.plan.Steps[r.step].Files[r.file]
}

// selectedSection returns the section under the cursor and its index,
// or nil when the cursor is on a whole-file unit.
func (m *Model) selectedSection() (*plan.Section, int) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil, -1
	}
	r := m.rows[m.cursor]
	if r.kind != rowSection {
		return nil, -1
	}
	return &m.plan.Steps[r.step].Files[r.file].Sections[r.sec], r.sec
}

// selectedKey is the state key of the unit under the cursor.
func (m *Model) selectedKey() string {
	f := m.selectedFile()
	if f == nil {
		return ""
	}
	if _, i := m.selectedSection(); i >= 0 {
		return f.SectionKey(i)
	}
	return f.Path
}

func (m *Model) selectedStep() *plan.Step {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return &m.plan.Steps[m.rows[m.cursor].step]
}

// loadSelection refreshes both viewports for the unit under the cursor,
// scrolls the diff to the selected section, and resets the guide.
func (m *Model) loadSelection() {
	if !m.ready {
		return
	}
	m.refreshContent()
	m.diff.GotoTop()
	if m.diffOffset > 0 {
		// Leave a couple of context rows above the section start.
		off := m.diffOffset - 2
		if off < 0 {
			off = 0
		}
		m.diff.SetYOffset(off)
	}
	m.callee.GotoTop()
	m.guide.GotoTop()
	m.scrollTreeIntoView()
}

// refreshContent re-renders viewport contents at the current sizes
// without touching scroll positions (used on resize).
func (m *Model) refreshContent() {
	f := m.selectedFile()
	st := m.selectedStep()
	m.diffOffset = 0
	m.desc.SetContent(renderDescription(m.plan, m.desc.Width))
	m.refreshAskView(false)
	if f == nil || st == nil {
		m.guide.SetContent(renderOverview(m.plan, m.guide.Width))
		if m.showCallees {
			m.callee.SetContent(renderCallees(nil, m.sourceFor, m.callee.Width))
		}
		return
	}
	sec, secIdx := m.selectedSection()
	if sec != nil {
		content, off := renderDiffSection(f.Path, m.diffFor(f), m.diff.Width,
			sec.StartLine, sec.EndLine, f.Status == "deleted")
		m.diff.SetContent(content)
		m.diffOffset = off
	} else {
		m.diff.SetContent(renderDiff(f.Path, m.diffFor(f), m.diff.Width))
	}
	if m.showCallees {
		callees := f.Callees
		if sec != nil && len(sec.Callees) > 0 {
			callees = sec.Callees
		}
		m.callee.SetContent(renderCallees(callees, m.sourceFor, m.callee.Width))
	}
	m.guide.SetContent(renderGuide(st, f, sec, secIdx, m.state.CommentsFor(m.selectedKey()), m.guide.Width))
}

// sourceFor loads (and caches) the head-side content of a repo-relative
// file for the callee pane.
func (m *Model) sourceFor(path string) (string, error) {
	if s, ok := m.srcCache[path]; ok {
		return s, nil
	}
	s, err := gitdiff.FileAt(m.repoDir, m.plan.Head, path)
	if err != nil {
		return "", err
	}
	m.srcCache[path] = s
	return s, nil
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

// layout recomputes pane sizes from the terminal size and the focus:
// the focused pane's column (and, for the stacked panes, its row) gets
// a larger share so whatever is being read has room, while the other
// columns fall back toward their minimums. The diff always absorbs the
// remaining width, so it grows whenever focus leaves the side panes.
func (m *Model) layout() {
	headerH, footerH := 1, 1
	// Each pane draws a border (2 cols, 2 rows) plus a title line.
	contentH := m.height - headerH - footerH - 2 - 1
	if contentH < 3 {
		contentH = 3
	}

	treePct, treeMax := 22, 40
	switch m.focus {
	case paneTree, paneAsk:
		treePct, treeMax = 30, 48
	case paneDiff:
		treePct = 14
	}
	treeW := m.width * treePct / 100
	if treeW < 20 {
		treeW = 20
	}
	if treeW > treeMax {
		treeW = treeMax
	}
	// With the callee pane in the layout, the guide narrows and every
	// column pays two more border columns.
	guidePct, guideMax, borders, calleeW := 30, 56, 6, 0
	if m.showCallees {
		guidePct, borders = 24, 8
		calleePct, calleeMax := 26, 48
		switch m.focus {
		case paneCallee:
			calleePct, calleeMax = 36, 72
		case paneDiff:
			calleePct = 16
		}
		calleeW = m.width * calleePct / 100
		if calleeW < 24 {
			calleeW = 24
		}
		if calleeW > calleeMax {
			calleeW = calleeMax
		}
	}
	switch m.focus {
	case paneGuide, paneDesc:
		guidePct, guideMax = 36, 72
	case paneDiff:
		guidePct = 16
	}
	guideW := m.width * guidePct / 100
	if guideW < 24 {
		guideW = 24
	}
	if guideW > guideMax {
		guideW = guideMax
	}
	diffW := m.width - treeW - guideW - calleeW - borders
	if diffW < 20 {
		// Give the boosted column's extra width back before letting
		// the row overflow the terminal.
		short := 20 - diffW
		switch {
		case (m.focus == paneGuide || m.focus == paneDesc) && guideW-short >= 24:
			guideW -= short
			diffW = 20
		case m.focus == paneCallee && calleeW-short >= 24:
			calleeW -= short
			diffW = 20
		case (m.focus == paneTree || m.focus == paneAsk) && treeW-short >= 20:
			treeW -= short
			diffW = 20
		}
	}
	if diffW < 20 {
		diffW = 20
	}

	// The left column stacks the tree over the ask pane; like the
	// description pane, the ask pane costs 3 extra rows for its own
	// border and title.
	askPct, askMax := 30, 14
	if m.focus == paneAsk {
		askPct, askMax = 60, contentH
	}
	askH := contentH * askPct / 100
	if askH < 5 {
		askH = 5
	}
	if askH > askMax {
		askH = askMax
	}
	treeH := contentH - askH - 3
	if treeH < 3 {
		treeH = 3
		if askH = contentH - treeH - 3; askH < 2 {
			askH = 2
		}
	}

	m.treeW = treeW
	m.treeH = treeH
	m.askView.Width = treeW
	m.askView.Height = askH - 1 // the input line takes the last row
	m.askInput.Width = treeW - 4
	m.commentInput.Width = m.width - 16

	// The right column stacks the guide over the PR description; the
	// description pane costs 3 extra rows for its own border and title.
	descPct, descMax := 30, 12
	if m.focus == paneDesc {
		descPct, descMax = 60, contentH
	}
	descH := contentH * descPct / 100
	if descH < 3 {
		descH = 3
	}
	if descH > descMax {
		descH = descMax
	}
	guideH := contentH - descH - 3
	if guideH < 3 {
		guideH = 3
		if descH = contentH - guideH - 3; descH < 1 {
			descH = 1
		}
	}

	m.diff.Width = diffW
	m.diff.Height = contentH
	m.callee.Width = calleeW
	m.callee.Height = contentH
	m.guide.Width = guideW
	m.guide.Height = guideH
	m.desc.Width = guideW
	m.desc.Height = descH
	m.scrollTreeIntoView()
}

func (m *Model) View() string {
	if !m.ready {
		return "loading..."
	}

	header := m.renderHeader()
	tree := m.renderPane(paneTree, "ファイル", m.renderTree(), m.treeW, m.treeH)
	askBody := lipgloss.JoinVertical(lipgloss.Left, m.askView.View(), m.renderAskInput())
	ask := m.renderPane(paneAsk, "Claudeに質問", askBody, m.treeW, m.askView.Height+1)
	left := lipgloss.JoinVertical(lipgloss.Left, tree, ask)
	diff := m.renderPane(paneDiff, "差分", m.diff.View(), m.diff.Width, m.diff.Height)
	guide := m.renderPane(paneGuide, "レビューガイド", m.guide.View(), m.guide.Width, m.guide.Height)
	desc := m.renderPane(paneDesc, "PRディスクリプション", m.desc.View(), m.desc.Width, m.desc.Height)
	right := lipgloss.JoinVertical(lipgloss.Left, guide, desc)

	cols := []string{left, diff}
	if m.showCallees {
		cols = append(cols, m.renderPane(paneCallee, "呼び出し先", m.callee.View(), m.callee.Width, m.callee.Height))
	}
	cols = append(cols, right)
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	footer := styleFooter.Render(
		"tab/h/l: ペイン移動  j/k: 選択/スクロール  space: レビュー済→次へ  r: 済トグル  n/p: 次/前ファイル  c: コメント  a: Claudeに質問  e: エディタで開く  q: 終了")
	if m.commenting {
		// The comment input takes over the footer row while open.
		footer = m.commentInput.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *Model) renderHeader() string {
	done := m.state.CountReviewed(m.plan)
	total := m.plan.TotalUnits()
	progress := fmt.Sprintf(" %d/%d レビュー済 ", done, total)
	if n := m.state.TotalComments(); n > 0 {
		progress = fmt.Sprintf(" コメント%d件 ·%s", n, progress)
	}

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
			f := &m.plan.Steps[r.step].Files[r.file]
			name := truncate.StringWithTail(f.Path, uint(m.treeW-8), "…")
			commented := m.hasComments(f.Path)
			if i == m.cursor {
				// The cursor line is styled as a whole; nested styles
				// would reset its background mid-line.
				mark := "  "
				if m.state.FileReviewed(f) {
					mark = "✓ "
				}
				cmark := "  "
				if commented {
					cmark = "c "
				}
				line = fmt.Sprintf(" %s%s%s %s", mark, cmark, plainStatus(f.Status), name)
				if pad := m.treeW - lipgloss.Width(line); pad > 0 {
					line += strings.Repeat(" ", pad)
				}
				line = styleCursor.Render(line)
			} else {
				mark := "  "
				if m.state.FileReviewed(f) {
					mark = styleReviewed.Render("✓ ")
				}
				cmark := "  "
				if commented {
					cmark = styleComment.Render("c ")
				}
				line = fmt.Sprintf(" %s%s%s %s", mark, cmark, statusIcon(f.Status), name)
			}
		case rowSection:
			f := &m.plan.Steps[r.step].Files[r.file]
			sec := f.Sections[r.sec]
			name := truncate.StringWithTail(sec.Title, uint(m.treeW-10), "…")
			reviewed := m.state.Reviewed[f.SectionKey(r.sec)]
			commented := m.hasComments(f.SectionKey(r.sec))
			if i == m.cursor {
				mark := "  "
				if reviewed {
					mark = "✓ "
				}
				cmark := "  "
				if commented {
					cmark = "c "
				}
				line = fmt.Sprintf("    %s%s· %s", mark, cmark, name)
				if pad := m.treeW - lipgloss.Width(line); pad > 0 {
					line += strings.Repeat(" ", pad)
				}
				line = styleCursor.Render(line)
			} else {
				mark := "  "
				if reviewed {
					mark = styleReviewed.Render("✓ ")
				}
				cmark := "  "
				if commented {
					cmark = styleComment.Render("c ")
				}
				line = fmt.Sprintf("    %s%s%s %s", mark, cmark, styleFaint.Render("·"), name)
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
	prog := tea.NewProgram(New(p, st, repoDir), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := prog.Run()
	return err
}

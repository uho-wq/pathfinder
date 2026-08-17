package ui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// askEntry is one question/answer pair shown in the ask pane. answer is
// empty while the question is in flight; errText replaces it on failure.
type askEntry struct {
	question string
	answer   string
	errText  string
}

// askResultMsg carries the claude CLI's answer back into Update. id ties
// the result to the question that spawned it so a stale run (e.g. after
// the user somehow re-submits) cannot overwrite a newer answer.
type askResultMsg struct {
	id     int
	answer string
	err    error
}

// askTimeout bounds one claude run; without it a hung CLI would leave
// the pane spinning forever.
const askTimeout = 3 * time.Minute

// askCommand returns the argv used to answer questions. It is the
// claude CLI by default; PATHFINDER_CLAUDE_CMD overrides it (mainly for
// tests, but also for wrappers), keeping any arguments it carries.
func askCommand() []string {
	if parts := strings.Fields(os.Getenv("PATHFINDER_CLAUDE_CMD")); len(parts) > 0 {
		return parts
	}
	return []string{"claude"}
}

// askPrompt wraps the user's question with the review context claude
// needs to answer it: which repo state is under review and which unit is
// selected. claude explores the repo itself via its allowed tools.
func (m *Model) askPrompt(question string) string {
	var b strings.Builder
	b.WriteString("あなたはPRレビュー中のレビュアーを支援するアシスタントです。リポジトリのルートで実行されています。\n")
	if m.plan.Title != "" {
		fmt.Fprintf(&b, "レビュー中のPR: %s\n", m.plan.Title)
	}
	if m.plan.Base != "" {
		head := m.plan.Head
		if head == "" {
			head = "作業ツリー"
		}
		fmt.Fprintf(&b, "差分の範囲: base=%s head=%s\n", m.plan.Base, head)
	}
	if f := m.selectedFile(); f != nil {
		fmt.Fprintf(&b, "選択中のファイル: %s\n", f.Path)
		if sec, _ := m.selectedSection(); sec != nil {
			loc := sec.Title
			if sec.StartLine > 0 {
				loc = fmt.Sprintf("%s (L%d-%d)", sec.Title, sec.StartLine, sec.EndLine)
			}
			fmt.Fprintf(&b, "選択中の箇所: %s\n", loc)
		}
	}
	b.WriteString("必要ならgitやコード検索で調べた上で、次の質問に日本語で簡潔に答えてください。\n\n質問: ")
	b.WriteString(question)
	return b.String()
}

// runAsk executes the claude CLI outside the UI loop and reports back
// via askResultMsg. The prompt is captured now so later cursor moves
// don't change the context the question was asked in.
func (m *Model) runAsk(question string, id int) tea.Cmd {
	prompt := m.askPrompt(question)
	dir := m.repoDir
	return func() tea.Msg {
		argv := askCommand()
		if _, err := exec.LookPath(argv[0]); err != nil {
			return askResultMsg{id: id, err: fmt.Errorf("%s コマンドが見つかりません (PATHを確認してください)", argv[0])}
		}
		ctx, cancel := context.WithTimeout(context.Background(), askTimeout)
		defer cancel()
		args := append(append([]string{}, argv[1:]...),
			"-p", prompt, "--allowedTools", "Bash(git:*) Read Grep Glob")
		cmd := exec.CommandContext(ctx, argv[0], args...)
		cmd.Dir = dir
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			detail := strings.TrimSpace(errb.String())
			if detail == "" {
				detail = err.Error()
			}
			return askResultMsg{id: id, err: fmt.Errorf("%s", detail)}
		}
		return askResultMsg{id: id, answer: strings.TrimSpace(out.String())}
	}
}

// submitAsk sends the current input as a question, if there is one and
// no question is already running (the CLI is one conversation at a time).
func (m *Model) submitAsk() tea.Cmd {
	q := strings.TrimSpace(m.askInput.Value())
	if q == "" || m.asking {
		return nil
	}
	m.askLog = append(m.askLog, askEntry{question: q})
	m.asking = true
	m.askID++
	m.askInput.SetValue("")
	m.refreshAskView(true)
	return tea.Batch(m.askSpin.Tick, m.runAsk(q, m.askID))
}

// handleAskKey routes keys while the ask pane is focused. Printable keys
// belong to the text input, so global single-letter bindings (q, j, e,
// ...) are suspended here; only control keys keep their app meaning.
func (m *Model) handleAskKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		return m, m.cycleFocus(+1)
	case "shift+tab":
		return m, m.cycleFocus(-1)
	case "esc":
		m.focus = paneTree
		m.askInput.Blur()
		return m, nil
	case "enter":
		return m, m.submitAsk()
	case "up", "ctrl+u", "pgup":
		m.askView.ScrollUp(1)
		return m, nil
	case "down", "ctrl+d", "pgdown":
		m.askView.ScrollDown(1)
		return m, nil
	}
	var cmd tea.Cmd
	m.askInput, cmd = m.askInput.Update(msg)
	return m, cmd
}

// refreshAskView re-renders the transcript; bottom pins the view to the
// newest entry (on submit and on answers, but not on resize).
func (m *Model) refreshAskView(bottom bool) {
	m.askView.SetContent(m.renderAskLog())
	if bottom {
		m.askView.GotoBottom()
	}
}

// renderAskLog builds the transcript shown above the input line.
func (m *Model) renderAskLog() string {
	width := m.askView.Width
	if len(m.askLog) == 0 {
		return styleFaint.Render(wrapText("aで入力欄へ。Enterで選択中の箇所を文脈にしてClaudeに質問します。", width))
	}
	var b strings.Builder
	for i, e := range m.askLog {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(styleGuideHead.Render("Q "))
		b.WriteString(wrapText(e.question, width-2))
		b.WriteString("\n")
		switch {
		case e.errText != "":
			b.WriteString(styleGuideWarn.Render(wrapText(e.errText, width)))
			b.WriteString("\n")
		case e.answer != "":
			b.WriteString(wrapText(e.answer, width))
			b.WriteString("\n")
		case m.asking && i == len(m.askLog)-1:
			b.WriteString(m.askSpin.View())
			b.WriteString(styleFaint.Render(" 回答を待っています..."))
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderAskInput is the one-line prompt at the bottom of the ask pane.
func (m *Model) renderAskInput() string {
	return lipgloss.NewStyle().MaxWidth(m.treeW).Render(m.askInput.View())
}

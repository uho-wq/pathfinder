package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"

	"github.com/uho-wq/pathfinder/internal/plan"
)

// The callee pane sits to the right of the diff and shows the bodies of
// the functions the selected change calls, so the reviewer can check
// what a call actually does without leaving the diff. Which callees to
// show — and where their bodies live — comes from the plan; the source
// itself is read from the head revision (or embedded in the plan) via
// the load callback.

// renderCallees builds the callee pane content. load resolves a
// repo-relative path to full file content and is only invoked for
// callees that do not embed their source.
func renderCallees(callees []plan.Callee, load func(string) (string, error), width int) string {
	if len(callees) == 0 {
		return styleFaint.Render("(この箇所の呼び出し先情報がありません)")
	}
	var b strings.Builder
	for i, c := range callees {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(styleGuideHead.Render(truncate.StringWithTail("■ "+c.Name, uint(width), "…")))
		b.WriteByte('\n')
		if loc := calleeLocation(c); loc != "" {
			b.WriteString(styleFaint.Render(truncate.StringWithTail(loc, uint(width), "…")))
			b.WriteByte('\n')
		}
		if c.Description != "" {
			b.WriteString(styleFaint.Render(wrapText(c.Description, width)))
			b.WriteByte('\n')
		}

		src, first, err := calleeSource(c, load)
		if err != nil {
			b.WriteString(styleFaint.Render(wrapText(fmt.Sprintf("コードを取得できませんでした: %v", err), width)))
			b.WriteByte('\n')
			continue
		}
		b.WriteString(renderCode(c.Path, src, first, width))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func calleeLocation(c plan.Callee) string {
	if c.Path == "" {
		return ""
	}
	switch {
	case c.StartLine > 0 && c.EndLine > c.StartLine:
		return fmt.Sprintf("%s (L%d-%d)", c.Path, c.StartLine, c.EndLine)
	case c.StartLine > 0:
		return fmt.Sprintf("%s (L%d)", c.Path, c.StartLine)
	}
	return c.Path
}

// calleeSource returns the callee's body and the file line number of its
// first line. Embedded source wins; otherwise the body is cut out of the
// loaded file at StartLine..EndLine.
func calleeSource(c plan.Callee, load func(string) (string, error)) (string, int, error) {
	if c.Source != "" {
		first := c.StartLine
		if first <= 0 {
			first = 1
		}
		return strings.TrimRight(c.Source, "\n"), first, nil
	}
	full, err := load(c.Path)
	if err != nil {
		return "", 0, err
	}
	lines := strings.Split(strings.TrimRight(full, "\n"), "\n")
	start, end := c.StartLine, c.EndLine
	if start <= 0 {
		start = 1
	}
	if start > len(lines) {
		return "", 0, fmt.Errorf("%s には %d 行目がありません(全%d行)", c.Path, start, len(lines))
	}
	if end < start {
		end = start
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start-1:end], "\n"), start, nil
}

// renderCode lays out source lines with a line-number gutter and the
// same per-line syntax highlighting the diff pane uses.
func renderCode(path, code string, firstLine, width int) string {
	lines := strings.Split(code, "\n")
	numW := len(strconv.Itoa(firstLine + len(lines) - 1))
	if numW < 2 {
		numW = 2
	}
	cw := width - numW - 1
	if cw < 4 {
		cw = 4
	}
	hl := highlighterFor(path)
	var b strings.Builder
	for i, ln := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(styleDiffCtxGut.Render(fmt.Sprintf("%*d ", numW, firstLine+i)))
		b.WriteString(renderCodeLine(expandTabs(ln), hl, cw))
	}
	return b.String()
}

// renderCodeLine syntax-colors one source line and truncates it
// ANSI-safely to the content width.
func renderCodeLine(text string, hl *highlighter, cw int) string {
	r := []rune(text)
	var b strings.Builder
	pos := 0
	for _, sp := range hl.spans(text) {
		if sp.lo > pos {
			b.WriteString(string(r[pos:sp.lo]))
		}
		b.WriteString(lipgloss.NewStyle().Foreground(sp.color).Render(string(r[sp.lo:sp.hi])))
		pos = sp.hi
	}
	if pos < len(r) {
		b.WriteString(string(r[pos:]))
	}
	return truncate.StringWithTail(b.String(), uint(cw), "…")
}

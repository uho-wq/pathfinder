package ui

import (
	"fmt"
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"

	"github.com/uho-wq/pathfinder/internal/plan"
)

// renderGuide builds the right pane: the AI-authored explanation of the
// selected file within its review step.
func renderGuide(st *plan.Step, f *plan.File, width int) string {
	var b strings.Builder

	section := func(title string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(styleGuideHead.Render("■ " + title))
		b.WriteString("\n")
	}
	para := func(s string) {
		b.WriteString(wrapText(s, width))
		b.WriteString("\n")
	}

	section("ステップ")
	para(st.Name)
	if st.Description != "" {
		b.WriteString(styleFaint.Render(wrapText(st.Description, width)))
		b.WriteString("\n")
	}

	if f.Summary != "" {
		section("この差分で起こっている変化")
		para(f.Summary)
	}

	if len(f.ReviewPoints) > 0 {
		section("レビュー観点")
		for _, p := range f.ReviewPoints {
			b.WriteString(bullet(p, width, styleGuideWarn.Render("✔ ")))
		}
	}

	if len(f.Dependencies) > 0 {
		section("依存先")
		for _, d := range f.Dependencies {
			b.WriteString(bullet(d, width, styleFaint.Render("→ ")))
		}
	}

	if len(f.Dependents) > 0 {
		section("呼び出し元 / 影響範囲")
		for _, d := range f.Dependents {
			b.WriteString(bullet(d, width, styleFaint.Render("← ")))
		}
	}

	if f.Notes != "" {
		section("メモ")
		para(f.Notes)
	}

	return b.String()
}

// renderOverview is shown in the guide pane before any file is selected,
// and summarizes the whole plan.
func renderOverview(p *plan.Plan, width int) string {
	var b strings.Builder
	b.WriteString(styleGuideHead.Render("■ PR概要"))
	b.WriteString("\n")
	if p.Summary != "" {
		b.WriteString(wrapText(p.Summary, width))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styleGuideHead.Render("■ レビューの流れ"))
	b.WriteString("\n")
	for i, st := range p.Steps {
		b.WriteString(bullet(fmt.Sprintf("%d. %s", i+1, st.Name), width, ""))
	}
	return b.String()
}

// bullet renders one wrapped list item with a hanging indent.
func bullet(text string, width int, marker string) string {
	indent := "  "
	body := wrapText(text, width-len(indent))
	lines := strings.Split(body, "\n")
	var b strings.Builder
	for i, l := range lines {
		if i == 0 {
			b.WriteString(marker)
			if marker == "" {
				b.WriteString(indent)
			}
		} else {
			b.WriteString(indent)
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// wrapText word-wraps where possible and hard-wraps otherwise, which is
// what makes long CJK runs (no spaces) fit the pane.
func wrapText(s string, width int) string {
	if width < 4 {
		width = 4
	}
	return wrap.String(wordwrap.String(s, width), width)
}

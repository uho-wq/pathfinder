package ui

import (
	"fmt"
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"

	"github.com/uho-wq/pathfinder/internal/plan"
)

// renderGuide builds the right pane: the AI-authored guide for the
// selected unit — a section of a file, or the whole file — in four fixed
// parts: what changed across the step, what changed in this unit, why
// the change took this form, and the review points. sec is nil (and
// secIdx < 0) for whole-file units.
func renderGuide(st *plan.Step, f *plan.File, sec *plan.Section, secIdx, width int) string {
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
	faint := func(s string) {
		b.WriteString(styleFaint.Render(wrapText(s, width)))
		b.WriteString("\n")
	}

	section("ステップ全体の変化")
	faint(st.Name)
	if st.Summary != "" {
		para(st.Summary)
	} else if st.Description != "" {
		para(st.Description)
	}

	section("この項目の変化")
	if sec != nil {
		title := sec.Title
		if sec.StartLine > 0 {
			if sec.EndLine > sec.StartLine {
				title = fmt.Sprintf("%s (L%d-%d)", title, sec.StartLine, sec.EndLine)
			} else {
				title = fmt.Sprintf("%s (L%d)", title, sec.StartLine)
			}
		}
		faint(fmt.Sprintf("箇所 %d/%d: %s", secIdx+1, len(f.Sections), title))
		if sec.Summary != "" {
			para(sec.Summary)
		}
	} else if f.Summary != "" {
		para(f.Summary)
	}

	rationale := f.Rationale
	if sec != nil {
		rationale = sec.Rationale
	}
	if rationale != "" {
		section("なぜこの処理か")
		para(rationale)
	}

	points := f.ReviewPoints
	if sec != nil {
		points = sec.ReviewPoints
	}
	if len(points) > 0 {
		section("レビュー観点")
		for _, p := range points {
			b.WriteString(bullet(p, width, styleGuideWarn.Render("✔ ")))
		}
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

// renderDescription builds the bottom-right pane: the PR's own
// description, falling back to the plan summary when the plan does not
// carry one.
func renderDescription(p *plan.Plan, width int) string {
	text := p.Description
	if text == "" {
		text = p.Summary
	}
	if text == "" {
		return styleFaint.Render("(ディスクリプションがありません)")
	}
	return wrapText(text, width)
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

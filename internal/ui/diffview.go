package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

// The diff pane renders a unified diff the way GitHub does: a line-number
// gutter for the old and new file, full-width background tints for added
// and deleted lines, word-level emphasis on the changed part of paired
// -/+ lines, and the noisy file header (diff --git, index, ---/+++)
// hidden.

type diffLineKind int

const (
	diffMeta diffLineKind = iota
	diffHunk
	diffContext
	diffAdd
	diffDel
)

// diffLine is one parsed row of a unified diff.
type diffLine struct {
	kind  diffLineKind
	oldNo int    // 1-based line number in the old file; 0 when absent
	newNo int    // 1-based line number in the new file; 0 when absent
	text  string // content without the leading diff marker, tabs expanded
	hiLo  int    // rune range within text emphasized as the changed part
	hiHi  int
}

var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// hiddenMeta matches file-header lines that carry no review value on
// their own; everything they say is already visible in the tree pane.
func hiddenMeta(line string) bool {
	for _, p := range []string{"diff --git", "index ", "similarity ", "dissimilarity ", "--- ", "+++ "} {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

func expandTabs(s string) string { return strings.ReplaceAll(s, "\t", "    ") }

// parseDiff turns a unified diff into rows with resolved line numbers.
func parseDiff(diff string) []diffLine {
	var out []diffLine
	var oldNo, newNo int
	inHunk := false
	for _, raw := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		switch {
		case hunkRe.MatchString(raw):
			m := hunkRe.FindStringSubmatch(raw)
			oldNo, _ = strconv.Atoi(m[1])
			newNo, _ = strconv.Atoi(m[2])
			inHunk = true
			out = append(out, diffLine{kind: diffHunk, text: expandTabs(raw)})
		case strings.HasPrefix(raw, "diff --git"):
			inHunk = false
			fallthrough
		case !inHunk:
			if !hiddenMeta(raw) {
				out = append(out, diffLine{kind: diffMeta, text: expandTabs(raw)})
			}
		case strings.HasPrefix(raw, "+"):
			out = append(out, diffLine{kind: diffAdd, newNo: newNo, text: expandTabs(raw[1:])})
			newNo++
		case strings.HasPrefix(raw, "-"):
			out = append(out, diffLine{kind: diffDel, oldNo: oldNo, text: expandTabs(raw[1:])})
			oldNo++
		case strings.HasPrefix(raw, `\`):
			// "\ No newline at end of file"
			out = append(out, diffLine{kind: diffMeta, text: expandTabs(raw)})
		default:
			out = append(out, diffLine{kind: diffContext, oldNo: oldNo, newNo: newNo,
				text: expandTabs(strings.TrimPrefix(raw, " "))})
			oldNo++
			newNo++
		}
	}
	markInlineChanges(out)
	return out
}

// markInlineChanges pairs each run of deletions with the run of
// additions that immediately follows it (the shape git emits for a
// modification) and marks the changed middle of each -/+ line pair.
func markInlineChanges(lines []diffLine) {
	i := 0
	for i < len(lines) {
		if lines[i].kind != diffDel {
			i++
			continue
		}
		delStart := i
		for i < len(lines) && lines[i].kind == diffDel {
			i++
		}
		delEnd := i
		if i >= len(lines) || lines[i].kind != diffAdd {
			continue
		}
		addStart := i
		for i < len(lines) && lines[i].kind == diffAdd {
			i++
		}
		n := delEnd - delStart
		if an := i - addStart; an < n {
			n = an
		}
		for j := 0; j < n; j++ {
			markPair(&lines[delStart+j], &lines[addStart+j])
		}
	}
}

// markPair emphasizes what actually changed between a deleted line and
// its replacement: everything between the common prefix and suffix.
func markPair(d, a *diffLine) {
	dr, ar := []rune(d.text), []rune(a.text)
	p := 0
	for p < len(dr) && p < len(ar) && dr[p] == ar[p] {
		p++
	}
	s := 0
	for s < len(dr)-p && s < len(ar)-p && dr[len(dr)-1-s] == ar[len(ar)-1-s] {
		s++
	}
	if p == 0 && s == 0 {
		// Nothing in common: the whole line changed, emphasis is noise.
		return
	}
	d.hiLo, d.hiHi = p, len(dr)-s
	a.hiLo, a.hiHi = p, len(ar)-s
}

// renderDiff renders a unified diff GitHub-style into a string sized for
// the diff pane, syntax-highlighting content by the file type of path.
// Text that is not a unified diff (error messages, binary notices) is
// passed through as-is.
func renderDiff(path, diff string, width int) string {
	s, _ := renderDiffSection(path, diff, width, 0, 0, false)
	return s
}

// renderDiffSection renders a diff like renderDiff, additionally marking
// the file lines start..end (new-file numbers, or old-file when useOld is
// set) with a bar in the left margin. The second return value is the row
// index where the marked range begins, for scrolling it into view.
// start == 0 disables marking.
func renderDiffSection(path, diff string, width, start, end int, useOld bool) (string, int) {
	if strings.TrimSpace(diff) == "" {
		return styleFaint.Render("(差分がありません)"), 0
	}
	lines := parseDiff(diff)
	if !hasHunk(lines) {
		raw := strings.Split(strings.TrimRight(diff, "\n"), "\n")
		for i := range raw {
			raw[i] = truncate.StringWithTail(expandTabs(raw[i]), uint(width), "…")
		}
		return strings.Join(raw, "\n"), 0
	}

	var marked []bool
	offset := 0
	if start > 0 {
		marked = markSection(lines, start, end, useOld)
		for i, m := range marked {
			if m {
				offset = i
				break
			}
		}
		// One column goes to the section bar.
		width--
	}

	numW := numberWidth(lines)
	// Two number columns like GitHub when the pane is wide enough,
	// one column on narrow panes, none when there is barely any room.
	cols := 2
	if width < 40 {
		cols = 1
	}
	if width-gutterWidth(numW, cols) < 10 {
		cols = 0
	}

	hl := highlighterFor(path)
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if marked != nil {
			if marked[i] {
				b.WriteString(styleSectionBar.Render("▎"))
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString(renderDiffLine(l, hl, numW, cols, width))
	}
	return b.String(), offset
}

// markSection flags the rows whose file line number falls in start..end.
// Line numbers are taken from the new file (or the old file when useOld
// is set); pure deletion rows in new-file mode (and pure additions in
// old-file mode) carry no such number, so they inherit membership from
// the position they sit at. end == 0 means a single line.
func markSection(lines []diffLine, start, end int, useOld bool) []bool {
	if end < start {
		end = start
	}
	in := func(n int) bool { return n >= start && n <= end }
	marked := make([]bool, len(lines))
	last := 0 // last numbered line seen on the tracked side
	for i, l := range lines {
		n := l.newNo
		other := l.oldNo
		if useOld {
			n, other = l.oldNo, l.newNo
		}
		switch {
		case l.kind == diffHunk, l.kind == diffMeta:
			// never marked
		case n != 0:
			marked[i] = in(n)
			last = n
		case other != 0:
			// A row on the opposite side only: it sits between tracked
			// lines `last` and `last+1`.
			marked[i] = in(last + 1)
		}
	}
	return marked
}

func hasHunk(lines []diffLine) bool {
	for _, l := range lines {
		if l.kind == diffHunk {
			return true
		}
	}
	return false
}

func numberWidth(lines []diffLine) int {
	max := 0
	for _, l := range lines {
		if l.oldNo > max {
			max = l.oldNo
		}
		if l.newNo > max {
			max = l.newNo
		}
	}
	w := len(strconv.Itoa(max))
	if w < 2 {
		w = 2
	}
	return w
}

func gutterWidth(numW, cols int) int {
	switch cols {
	case 2:
		return numW*2 + 2
	case 1:
		return numW + 1
	}
	return 0
}

// gutterText formats the line-number gutter without styling.
func gutterText(l diffLine, numW, cols int) string {
	num := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	switch cols {
	case 2:
		return fmt.Sprintf("%*s %*s ", numW, num(l.oldNo), numW, num(l.newNo))
	case 1:
		n := l.newNo
		if n == 0 {
			n = l.oldNo
		}
		return fmt.Sprintf("%*s ", numW, num(n))
	}
	return ""
}

func renderDiffLine(l diffLine, hl *highlighter, numW, cols, width int) string {
	gw := gutterWidth(numW, cols)
	cw := width - gw

	switch l.kind {
	case diffHunk:
		// The hunk header spans the whole row, gutter included.
		s := truncate.StringWithTail(l.text, uint(width), "…")
		if pad := width - lipgloss.Width(s); pad > 0 {
			s += strings.Repeat(" ", pad)
		}
		return styleDiffHunk.Render(s)

	case diffMeta:
		return strings.Repeat(" ", gw) +
			styleDiffMeta.Render(truncate.StringWithTail(l.text, uint(cw), "…"))

	case diffAdd:
		return styleDiffAddGut.Render(gutterText(l, numW, cols)) +
			diffContentRow("+", l, hl, styleDiffAddLine, styleDiffAddWord, cw)

	case diffDel:
		return styleDiffDelGut.Render(gutterText(l, numW, cols)) +
			diffContentRow("-", l, hl, styleDiffDelLine, styleDiffDelWord, cw)

	default: // diffContext
		return styleDiffCtxGut.Render(gutterText(l, numW, cols)) +
			diffContentRow(" ", l, hl, styleDiffCtxLine, styleDiffCtxLine, cw)
	}
}

// diffContentRow styles a line's content — sign, syntax-colored text,
// and the emphasized changed segment — truncates it to the content width
// ANSI-safely, and pads the remainder so the background tint spans the
// full row. The line is cut wherever a syntax span or the emphasis range
// starts or ends; each piece gets the word-emphasis background when it
// falls inside hiLo..hiHi and the syntax foreground of its span.
func diffContentRow(sign string, l diffLine, hl *highlighter, lineSt, wordSt lipgloss.Style, cw int) string {
	r := []rune(l.text)
	spans := hl.spans(l.text)

	cutSet := map[int]bool{0: true, len(r): true}
	if l.hiLo < l.hiHi {
		cutSet[l.hiLo] = true
		cutSet[l.hiHi] = true
	}
	for _, sp := range spans {
		if sp.lo > 0 && sp.lo < len(r) {
			cutSet[sp.lo] = true
		}
		if sp.hi > 0 && sp.hi < len(r) {
			cutSet[sp.hi] = true
		}
	}
	cuts := make([]int, 0, len(cutSet))
	for c := range cutSet {
		cuts = append(cuts, c)
	}
	sort.Ints(cuts)

	var b strings.Builder
	b.WriteString(lineSt.Render(sign))
	for k := 0; k+1 < len(cuts); k++ {
		lo, hi := cuts[k], cuts[k+1]
		st := lineSt
		if l.hiLo < l.hiHi && lo >= l.hiLo && lo < l.hiHi {
			st = wordSt
		}
		for _, sp := range spans {
			if lo >= sp.lo && lo < sp.hi {
				st = st.Foreground(sp.color)
				break
			}
		}
		b.WriteString(st.Render(string(r[lo:hi])))
	}

	s := truncate.StringWithTail(b.String(), uint(cw), "…")
	if pad := cw - lipgloss.Width(s); pad > 0 {
		s += lineSt.Render(strings.Repeat(" ", pad))
	}
	return s
}

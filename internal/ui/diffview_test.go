package ui

import (
	"strings"
	"testing"
)

const sampleDiff = `diff --git a/foo.go b/foo.go
index 111..222 100644
--- a/foo.go
+++ b/foo.go
@@ -10,4 +10,5 @@ func main() {
 	ctx := context.Background()
-	fmt.Println("hello")
+	fmt.Println("world")
+	fmt.Println("extra")
 	return
`

func TestParseDiffLineNumbers(t *testing.T) {
	lines := parseDiff(sampleDiff)

	// Hidden header lines (diff --git, index, ---/+++) must be dropped.
	for _, l := range lines {
		if strings.HasPrefix(l.text, "index ") || strings.HasPrefix(l.text, "+++ ") {
			t.Errorf("header line should be hidden: %q", l.text)
		}
	}

	want := []struct {
		kind  diffLineKind
		oldNo int
		newNo int
	}{
		{diffHunk, 0, 0},
		{diffContext, 10, 10},
		{diffDel, 11, 0},
		{diffAdd, 0, 11},
		{diffAdd, 0, 12},
		{diffContext, 12, 13},
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(lines), len(want), lines)
	}
	for i, w := range want {
		l := lines[i]
		if l.kind != w.kind || l.oldNo != w.oldNo || l.newNo != w.newNo {
			t.Errorf("line %d = kind %d old %d new %d, want kind %d old %d new %d",
				i, l.kind, l.oldNo, l.newNo, w.kind, w.oldNo, w.newNo)
		}
	}
}

func TestInlineChangeIsMarked(t *testing.T) {
	lines := parseDiff(sampleDiff)
	var del, add *diffLine
	for i := range lines {
		switch lines[i].kind {
		case diffDel:
			del = &lines[i]
		case diffAdd:
			if add == nil {
				add = &lines[i]
			}
		}
	}
	if del == nil || add == nil {
		t.Fatal("sample diff should contain a -/+ pair")
	}
	// The changed segment of `fmt.Println("hello")` → `fmt.Println("world")`
	// is exactly the differing word.
	if got := string([]rune(del.text)[del.hiLo:del.hiHi]); got != "hello" {
		t.Errorf("del emphasis = %q, want %q", got, "hello")
	}
	if got := string([]rune(add.text)[add.hiLo:add.hiHi]); got != "world" {
		t.Errorf("add emphasis = %q, want %q", got, "world")
	}
}

func TestUnpairedAddHasNoEmphasis(t *testing.T) {
	lines := parseDiff(sampleDiff)
	var adds []*diffLine
	for i := range lines {
		if lines[i].kind == diffAdd {
			adds = append(adds, &lines[i])
		}
	}
	if len(adds) != 2 {
		t.Fatalf("want 2 added lines, got %d", len(adds))
	}
	// The second + line has no - counterpart, so nothing is emphasized.
	if last := adds[1]; last.hiLo < last.hiHi {
		t.Errorf("unpaired add should have no emphasis, got %d..%d", last.hiLo, last.hiHi)
	}
}

func TestRenderDiffShowsGutterNumbers(t *testing.T) {
	out := renderDiff(sampleDiff, 80)
	for _, want := range []string{"10 10", "11   ", "   11", "   12", "12 13", "@@ -10,4 +10,5 @@"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered diff should contain %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "index 111") || strings.Contains(out, "+++ b/foo.go") {
		t.Error("rendered diff should hide file header noise")
	}
}

func TestRenderDiffNarrowPaneUsesSingleColumn(t *testing.T) {
	out := renderDiff(sampleDiff, 24)
	if strings.Contains(out, "10 10") {
		t.Error("narrow pane should not render two number columns")
	}
	if !strings.Contains(out, "10") {
		t.Error("narrow pane should still render line numbers")
	}
}

func TestRenderDiffPassesThroughNonDiffText(t *testing.T) {
	msg := "差分を取得できませんでした:\ngit diff: exit status 128"
	out := renderDiff(msg, 80)
	for _, want := range []string{"差分を取得できませんでした", "exit status 128"} {
		if !strings.Contains(out, want) {
			t.Errorf("non-diff text should pass through, missing %q in %q", want, out)
		}
	}
}

func TestMarkSectionNewSide(t *testing.T) {
	lines := parseDiff(sampleDiff)
	marked := markSection(lines, 11, 12, false)

	// New-file lines 11-12 are the two added lines; the deletion they
	// replace sits at the same position and is marked with them.
	want := []bool{false, false, true, true, true, false}
	if len(marked) != len(want) {
		t.Fatalf("got %d marks, want %d", len(marked), len(want))
	}
	for i := range want {
		if marked[i] != want[i] {
			t.Errorf("marked[%d] = %v, want %v (line %+v)", i, marked[i], want[i], lines[i])
		}
	}
}

func TestMarkSectionOldSide(t *testing.T) {
	lines := parseDiff(sampleDiff)
	marked := markSection(lines, 11, 0, true)
	// Old-file line 11 is exactly the deleted line.
	for i, l := range lines {
		want := l.kind == diffDel
		if marked[i] != want {
			t.Errorf("marked[%d] = %v, want %v (line %+v)", i, marked[i], want, l)
		}
	}
}

func TestRenderDiffSectionBarAndOffset(t *testing.T) {
	out, off := renderDiffSection(sampleDiff, 80, 11, 12, false)
	if off != 2 {
		t.Errorf("offset = %d, want 2 (first marked row)", off)
	}
	bars := strings.Count(out, "▎")
	if bars != 3 {
		t.Errorf("marked rows = %d, want 3:\n%s", bars, out)
	}
}

func TestRenderDiffSectionWithoutRangeMatchesRenderDiff(t *testing.T) {
	plain := renderDiff(sampleDiff, 80)
	sect, off := renderDiffSection(sampleDiff, 80, 0, 0, false)
	if plain != sect || off != 0 {
		t.Error("start_line 0 should render exactly like renderDiff")
	}
}

func TestRenderDiffKeepsMeaningfulMeta(t *testing.T) {
	d := "diff --git a/new.go b/new.go\nnew file mode 100644\nindex 000..111\n--- /dev/null\n+++ b/new.go\n@@ -0,0 +1,1 @@\n+package main\n"
	out := renderDiff(d, 80)
	if !strings.Contains(out, "new file mode 100644") {
		t.Errorf("new-file marker should stay visible:\n%s", out)
	}
}

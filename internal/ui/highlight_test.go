package ui

import (
	"strings"
	"testing"
)

func spanAt(spans []hlSpan, pos int) *hlSpan {
	for i := range spans {
		if pos >= spans[i].lo && pos < spans[i].hi {
			return &spans[i]
		}
	}
	return nil
}

func TestHighlighterForKnownAndUnknownFiles(t *testing.T) {
	if highlighterFor("foo.go") == nil {
		t.Error("foo.go should get a highlighter")
	}
	if highlighterFor("main.py") == nil {
		t.Error("main.py should get a highlighter")
	}
	if hl := highlighterFor("data.xyzzy-unknown"); hl != nil {
		t.Errorf("unknown extension should get no highlighter, got %+v", hl)
	}
	if hl := highlighterFor(""); hl != nil {
		t.Error("empty path should get no highlighter")
	}
}

func TestGoLineSpans(t *testing.T) {
	hl := highlighterFor("foo.go")
	line := `if x := 42; ok { return "done" } // fin`
	spans := hl.spans(line)

	runes := []rune(line)
	find := func(sub string) int {
		for i := 0; i+len([]rune(sub)) <= len(runes); i++ {
			if string(runes[i:i+len([]rune(sub))]) == sub {
				return i
			}
		}
		t.Fatalf("substring %q not in test line", sub)
		return -1
	}

	for _, tc := range []struct {
		sub  string
		want interface{}
	}{
		{"if", colorSynKeyword},
		{"return", colorSynKeyword},
		{"42", colorSynNumber},
		{`"done"`, colorSynString},
		{"// fin", colorSynComment},
	} {
		sp := spanAt(spans, find(tc.sub))
		if sp == nil {
			t.Errorf("%q should be inside a colored span", tc.sub)
			continue
		}
		if sp.color != tc.want {
			t.Errorf("%q colored %v, want %v", tc.sub, sp.color, tc.want)
		}
	}

	// Plain identifiers keep the default foreground.
	if sp := spanAt(spans, find("ok")); sp != nil {
		t.Errorf("identifier should not be colored, got %v", sp.color)
	}
}

func TestNilHighlighterYieldsNoSpans(t *testing.T) {
	var hl *highlighter
	if spans := hl.spans("anything at all"); spans != nil {
		t.Errorf("nil highlighter should yield no spans, got %v", spans)
	}
}

// Highlighting must not disturb layout: same visible text, gutter
// numbers, and padding as before, colors aside.
func TestHighlightedRenderKeepsGutterLayout(t *testing.T) {
	out := renderDiff("foo.go", sampleDiff, 80)
	lines := parseDiff(sampleDiff)
	if len(lines) == 0 {
		t.Fatal("sample diff should parse")
	}
	for _, want := range []string{"10 10", "11   ", "   11", "fmt.Println"} {
		if !strings.Contains(out, want) {
			t.Errorf("highlighted render should still contain %q\n%s", want, out)
		}
	}
}

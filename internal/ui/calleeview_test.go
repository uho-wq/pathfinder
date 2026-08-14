package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/uho-wq/pathfinder/internal/plan"
)

func TestRenderCalleesEmbeddedSource(t *testing.T) {
	callees := []plan.Callee{{
		Name:        "service.Create",
		Path:        "internal/service/s.go",
		StartLine:   10,
		EndLine:     12,
		Description: "本体の説明",
		Source:      "func Create() error {\n\treturn nil\n}\n",
	}}
	load := func(string) (string, error) {
		t.Fatal("embedded source should not trigger a load")
		return "", nil
	}
	out := renderCallees(callees, load, 60)
	for _, want := range []string{
		"service.Create",
		"internal/service/s.go (L10-12)",
		"本体の説明",
		"func Create() error {",
		"10", "12", // gutter numbers start at the callee's start_line
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got:\n%s", want, out)
		}
	}
}

func TestRenderCalleesLoadsRange(t *testing.T) {
	callees := []plan.Callee{{
		Name:      "helper",
		Path:      "pkg/h.go",
		StartLine: 2,
		EndLine:   3,
	}}
	load := func(path string) (string, error) {
		if path != "pkg/h.go" {
			t.Errorf("load path = %q", path)
		}
		return "line1\nline2\nline3\nline4\n", nil
	}
	out := renderCallees(callees, load, 60)
	if !strings.Contains(out, "line2") || !strings.Contains(out, "line3") {
		t.Errorf("output should contain the requested range, got:\n%s", out)
	}
	if strings.Contains(out, "line1") || strings.Contains(out, "line4") {
		t.Errorf("output should not contain lines outside the range, got:\n%s", out)
	}
}

func TestRenderCalleesLoadError(t *testing.T) {
	callees := []plan.Callee{{Name: "gone", Path: "no/such.go", StartLine: 1}}
	load := func(string) (string, error) { return "", fmt.Errorf("boom") }
	out := renderCallees(callees, load, 60)
	if !strings.Contains(out, "boom") {
		t.Errorf("load errors should be surfaced, got:\n%s", out)
	}
}

func TestRenderCalleesEmpty(t *testing.T) {
	out := renderCallees(nil, nil, 60)
	if !strings.Contains(out, "呼び出し先情報がありません") {
		t.Errorf("empty callees should render a placeholder, got:\n%s", out)
	}
}

func TestCalleeSourceRangeClamped(t *testing.T) {
	c := plan.Callee{Name: "f", Path: "a.go", StartLine: 3, EndLine: 99}
	load := func(string) (string, error) { return "1\n2\n3\n4\n", nil }
	src, first, err := calleeSource(c, load)
	if err != nil {
		t.Fatal(err)
	}
	if first != 3 || src != "3\n4" {
		t.Errorf("got first=%d src=%q", first, src)
	}

	c.StartLine = 10
	if _, _, err := calleeSource(c, load); err == nil {
		t.Error("start beyond EOF should error")
	}
}

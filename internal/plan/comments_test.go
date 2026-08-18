package plan

import (
	"path/filepath"
	"testing"
)

func TestCommentsPersist(t *testing.T) {
	planPath := filepath.Join(t.TempDir(), "review.json")
	st := LoadState(planPath)
	st.AddComment("a.go", "ファイルへのコメント")
	st.AddComment("b.go#0", "セクションへのコメント")
	st.AddComment("b.go#0", "消されるコメント")
	st.RemoveLastComment("b.go#0")

	re := LoadState(planPath)
	if got := re.CommentsFor("a.go"); len(got) != 1 || got[0] != "ファイルへのコメント" {
		t.Errorf("CommentsFor(a.go) = %v", got)
	}
	if got := re.CommentsFor("b.go#0"); len(got) != 1 || got[0] != "セクションへのコメント" {
		t.Errorf("CommentsFor(b.go#0) = %v", got)
	}
	if re.TotalComments() != 2 {
		t.Errorf("TotalComments = %d, want 2", re.TotalComments())
	}

	re.RemoveLastComment("a.go")
	if _, ok := re.Comments["a.go"]; ok {
		t.Error("removing the only comment should drop the key")
	}
	re.RemoveLastComment("a.go") // no-op on empty
}

func TestFormatComments(t *testing.T) {
	p := &Plan{Steps: []Step{{Name: "s", Files: []File{
		{Path: "a.go"},
		{Path: "b.go", Sections: []Section{
			{Title: "関数X", StartLine: 3, EndLine: 9},
			{Title: "関数Y", StartLine: 12},
		}},
		{Path: "c.go"},
	}}}}
	st := LoadState(filepath.Join(t.TempDir(), "review.json"))

	if out := FormatComments(p, st); out != "" {
		t.Errorf("no comments should format to empty, got %q", out)
	}

	st.AddComment("a.go", "ファイルへのコメント")
	st.AddComment("b.go#0", "セクションへのコメント")
	st.AddComment("b.go#1", "1行だけの箇所")

	want := `## a.go
- ファイルへのコメント

## b.go
- 関数X (L3-L9)
  - セクションへのコメント
- 関数Y (L12)
  - 1行だけの箇所
`
	if out := FormatComments(p, st); out != want {
		t.Errorf("FormatComments =\n%q\nwant\n%q", out, want)
	}
}

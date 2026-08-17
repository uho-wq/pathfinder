package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExample(t *testing.T) {
	p, err := Load(filepath.Join("..", "..", "examples", "review.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Title == "" {
		t.Error("title should not be empty")
	}
	if got := p.TotalFiles(); got != 3 {
		t.Errorf("TotalFiles = %d, want 3", got)
	}
	// Two section-less files plus two sections in the service file.
	if got := p.TotalUnits(); got != 4 {
		t.Errorf("TotalUnits = %d, want 4", got)
	}
	if !p.HasCallees() {
		t.Error("example plan should carry callees")
	}
}

func TestLoadRejectsBadSections(t *testing.T) {
	dir := t.TempDir()
	for name, files := range map[string]string{
		"notitle.json":      `[{"path":"a.go","sections":[{"summary":"x"}]}]`,
		"badrange.json":     `[{"path":"a.go","sections":[{"title":"t","start_line":10,"end_line":5}]}]`,
		"calleenoname.json": `[{"path":"a.go","callees":[{"path":"b.go"}]}]`,
		"calleenopath.json": `[{"path":"a.go","callees":[{"name":"f"}]}]`,
		"calleerange.json":  `[{"path":"a.go","sections":[{"title":"t","callees":[{"name":"f","path":"b.go","start_line":9,"end_line":3}]}]}]`,
	} {
		path := filepath.Join(dir, name)
		data := `{"version":1,"title":"x","steps":[{"name":"s","files":` + files + `}]}`
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestSectionProgress(t *testing.T) {
	p := &Plan{Steps: []Step{{Name: "s", Files: []File{
		{Path: "a.go", Sections: []Section{{Title: "one"}, {Title: "two"}}},
		{Path: "b.go"},
	}}}}
	st := LoadState(filepath.Join(t.TempDir(), "review.json"))

	f := &p.Steps[0].Files[0]
	st.Toggle(f.SectionKey(0))
	if got := st.CountReviewed(p); got != 1 {
		t.Errorf("CountReviewed = %d, want 1", got)
	}
	if st.FileReviewed(f) {
		t.Error("file should not be reviewed with one section left")
	}
	st.Toggle(f.SectionKey(1))
	if !st.FileReviewed(f) {
		t.Error("file should be reviewed once all sections are")
	}
	if got := st.CountReviewed(p); got != 2 {
		t.Errorf("CountReviewed = %d, want 2", got)
	}
	if st.FileReviewed(&p.Steps[0].Files[1]) {
		t.Error("section-less file should follow its own mark")
	}
}

func TestLoadRejectsEmptyPlan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"title":"x","steps":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected error for plan without steps")
	}
}

func TestLoadRejectsMissingPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nopath.json")
	data := `{"version":1,"title":"x","steps":[{"name":"s","files":[{"summary":"no path"}]}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected error for file entry without path")
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "review.json")

	st := LoadState(planPath)
	st.Toggle("a.go")
	if !st.Reviewed["a.go"] {
		t.Fatal("a.go should be reviewed after toggle")
	}

	st2 := LoadState(planPath)
	if !st2.Reviewed["a.go"] {
		t.Error("reviewed state should persist across loads")
	}
	st2.Toggle("a.go")
	if st2.Reviewed["a.go"] {
		t.Error("second toggle should clear the mark")
	}
}

func TestStatePath(t *testing.T) {
	if got := StatePath("dir/review.json"); got != "dir/review.state.json" {
		t.Errorf("StatePath = %q", got)
	}
}

func TestBranchFileName(t *testing.T) {
	for branch, want := range map[string]string{
		"main":          "main.json",
		"feature/login": "feature-login.json",
		"a/b/c":         "a-b-c.json",
	} {
		if got := BranchFileName(branch); got != want {
			t.Errorf("BranchFileName(%q) = %q, want %q", branch, got, want)
		}
	}
}

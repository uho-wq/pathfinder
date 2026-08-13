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

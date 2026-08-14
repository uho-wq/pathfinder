package ui

import (
	"reflect"
	"testing"
)

func TestEditorCommand(t *testing.T) {
	tests := []struct {
		editor string
		path   string
		line   int
		want   []string
	}{
		{"vim", "a/b.go", 12, []string{"vim", "+12", "a/b.go"}},
		{"nvim", "a/b.go", 0, []string{"nvim", "a/b.go"}},
		{"", "a/b.go", 5, []string{"vi", "+5", "a/b.go"}},
		{"code --wait", "a/b.go", 7, []string{"code", "--wait", "--goto", "a/b.go:7"}},
		{"/usr/local/bin/subl", "a/b.go", 3, []string{"/usr/local/bin/subl", "a/b.go:3"}},
	}
	for _, tt := range tests {
		t.Setenv("EDITOR", tt.editor)
		c := editorCommand(tt.path, tt.line)
		if !reflect.DeepEqual(c.Args, tt.want) {
			t.Errorf("EDITOR=%q line=%d: args = %v, want %v", tt.editor, tt.line, c.Args, tt.want)
		}
	}
}

func TestOpenEditorTargetsSelection(t *testing.T) {
	t.Setenv("EDITOR", "vim")
	m := exampleModel(t)
	if cmd := m.openEditor(); cmd == nil {
		t.Error("e should produce an editor command for the selected file")
	}
}

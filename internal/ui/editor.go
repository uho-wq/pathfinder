package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// editorFinishedMsg reports that the external editor exited; path is the
// plan-relative path whose working-tree diff may now be stale.
type editorFinishedMsg struct {
	path string
	err  error
}

// openEditor suspends the TUI and opens the selected file in $EDITOR,
// jumping to the selected section's first line when there is one.
func (m *Model) openEditor() tea.Cmd {
	f := m.selectedFile()
	if f == nil {
		return nil
	}
	line := 0
	if sec, _ := m.selectedSection(); sec != nil {
		line = sec.StartLine
	}
	path := f.Path
	if m.repoDir != "" {
		path = filepath.Join(m.repoDir, f.Path)
	}
	relPath := f.Path
	return tea.ExecProcess(editorCommand(path, line), func(err error) tea.Msg {
		return editorFinishedMsg{path: relPath, err: err}
	})
}

// editorCommand builds the $EDITOR invocation for path, using the
// line-jump syntax of the configured editor (vi-style +N by default).
// $EDITOR may carry arguments ("code --wait"); they are kept.
func editorCommand(path string, line int) *exec.Cmd {
	parts := strings.Fields(os.Getenv("EDITOR"))
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	args := append([]string{}, parts[1:]...)
	if line <= 0 {
		return exec.Command(parts[0], append(args, path)...)
	}
	switch filepath.Base(parts[0]) {
	case "code", "code-insiders", "codium", "cursor":
		args = append(args, "--goto", fmt.Sprintf("%s:%d", path, line))
	case "subl", "sublime_text", "zed", "hx":
		args = append(args, fmt.Sprintf("%s:%d", path, line))
	default: // vi, vim, nvim, nano, emacs, micro, ...
		args = append(args, fmt.Sprintf("+%d", line), path)
	}
	return exec.Command(parts[0], args...)
}

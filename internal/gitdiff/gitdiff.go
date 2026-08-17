// Package gitdiff resolves per-file unified diffs from a git repository.
package gitdiff

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileDiff returns the unified diff for a single file.
//
// With both base and head set, it uses the three-dot form
// (base...head) so the diff matches what a PR shows: changes on head
// since it diverged from base. With only base set, the working tree is
// compared against base. With neither, the working tree is compared
// against HEAD.
func FileDiff(dir, base, head, path string) (string, error) {
	args := []string{"diff", "--no-color", "--find-renames"}
	switch {
	case base != "" && head != "":
		args = append(args, base+"..."+head)
	case base != "":
		args = append(args, base)
	default:
		args = append(args, "HEAD")
	}
	args = append(args, "--", path)

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr == "" {
			stderr = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr)
	}
	return string(out), nil
}

// CurrentBranch returns the name of the checked-out branch, or "" when
// HEAD is detached or dir is not a git repository.
func CurrentBranch(dir string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// FileAt returns the content of a file at the given revision. With rev
// empty, the working tree copy is read instead, matching FileDiff's
// convention that an empty head means "compare against the working
// tree".
func FileAt(dir, rev, path string) (string, error) {
	if rev == "" {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	cmd := exec.Command("git", "show", rev+":"+path)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr == "" {
			stderr = err.Error()
		}
		return "", fmt.Errorf("git show %s:%s: %s", rev, path, stderr)
	}
	return string(out), nil
}

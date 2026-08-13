// Package gitdiff resolves per-file unified diffs from a git repository.
package gitdiff

import (
	"fmt"
	"os/exec"
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

// Package plan defines the review plan format that AI tools (e.g. Claude
// Code) generate and pathfinder consumes. The plan carries the semantic
// layer of a review: in which order to read files, what changed in each
// file, and what to look out for. Diffs themselves are usually resolved
// from git at runtime, though a plan may embed them.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
)

// Plan is the top-level document loaded from a review plan file.
type Plan struct {
	Version int    `json:"version"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	// Base and Head are git revisions used to compute diffs when a file
	// entry does not embed one. Base is typically the merge target
	// (e.g. "main"), Head the PR branch. With Head empty, the working
	// tree is compared against Base.
	Base    string `json:"base,omitempty"`
	Head    string `json:"head,omitempty"`
	RepoDir string `json:"repo_dir,omitempty"`
	Steps   []Step `json:"steps"`
}

// Step groups files that should be reviewed together, in order.
type Step struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Files       []File `json:"files"`
}

// File is one reviewable unit inside a step.
type File struct {
	Path   string `json:"path"`
	Status string `json:"status,omitempty"` // added | modified | deleted | renamed
	// Summary explains what is happening in this file's diff.
	Summary string `json:"summary,omitempty"`
	// ReviewPoints are concrete things the reviewer should check.
	ReviewPoints []string `json:"review_points,omitempty"`
	// Dependencies lists what this file's changes depend on.
	Dependencies []string `json:"dependencies,omitempty"`
	// Dependents lists callers / places affected by this change.
	Dependents []string `json:"dependents,omitempty"`
	Notes      string   `json:"notes,omitempty"`
	// Diff optionally embeds a unified diff. When empty, pathfinder
	// runs `git diff` using the plan's Base/Head.
	Diff string `json:"diff,omitempty"`
}

// Load reads and validates a plan file.
func Load(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
	}
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &p, nil
}

func (p *Plan) validate() error {
	if p.Version > 1 {
		return fmt.Errorf("unsupported plan version %d (this build supports 1)", p.Version)
	}
	if len(p.Steps) == 0 {
		return fmt.Errorf("plan has no steps")
	}
	total := 0
	for i, s := range p.Steps {
		for j, f := range s.Files {
			if f.Path == "" {
				return fmt.Errorf("steps[%d].files[%d]: path is required", i, j)
			}
			total++
		}
	}
	if total == 0 {
		return fmt.Errorf("plan has no files")
	}
	return nil
}

// TotalFiles returns the number of file entries across all steps.
func (p *Plan) TotalFiles() int {
	n := 0
	for _, s := range p.Steps {
		n += len(s.Files)
	}
	return n
}

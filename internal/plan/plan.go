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
	// Description is the PR's own description (e.g. the PR body on
	// GitHub), shown in the bottom-right pane. When empty, Summary is
	// shown there instead.
	Description string `json:"description,omitempty"`
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

// File groups the sections of one file inside a step. When Sections is
// empty the file itself is the reviewable unit (v1 plans).
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
	// Sections split the file's diff into ordered spots to review one
	// by one. When present, they replace the file as the unit the
	// reviewer steps through.
	Sections []Section `json:"sections,omitempty"`
}

// Section is one spot inside a file's diff: a hunk, a function, or any
// contiguous range worth reviewing as a unit.
type Section struct {
	Title string `json:"title"`
	// StartLine / EndLine locate the section in the new file (head
	// side). For deleted files they refer to the old file. Zero means
	// the section has no anchor and the diff is shown from the top.
	StartLine int `json:"start_line,omitempty"`
	EndLine   int `json:"end_line,omitempty"`
	// Summary explains what is happening in this section.
	Summary string `json:"summary,omitempty"`
	// ReviewPoints are concrete things to check in this section.
	ReviewPoints []string `json:"review_points,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

// SectionKey returns the state key marking one section reviewed. It
// includes the index so the same path may appear in several sections.
func (f *File) SectionKey(i int) string {
	return fmt.Sprintf("%s#%d", f.Path, i)
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
			for k, sec := range f.Sections {
				if sec.Title == "" {
					return fmt.Errorf("steps[%d].files[%d].sections[%d]: title is required", i, j, k)
				}
				if sec.EndLine != 0 && sec.EndLine < sec.StartLine {
					return fmt.Errorf("steps[%d].files[%d].sections[%d]: end_line %d < start_line %d",
						i, j, k, sec.EndLine, sec.StartLine)
				}
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

// TotalUnits returns the number of reviewable units: each section
// counts as one, and a file without sections counts as one.
func (p *Plan) TotalUnits() int {
	n := 0
	for _, s := range p.Steps {
		for _, f := range s.Files {
			if len(f.Sections) > 0 {
				n += len(f.Sections)
			} else {
				n++
			}
		}
	}
	return n
}

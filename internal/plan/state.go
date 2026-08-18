package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// State holds the reviewer's progress, persisted next to the plan file so
// a review can be resumed across sessions.
type State struct {
	Reviewed map[string]bool `json:"reviewed"`
	// Comments are the reviewer's own notes, keyed by unit key (a file
	// path, or path#index for a section) like Reviewed.
	Comments map[string][]string `json:"comments,omitempty"`

	path string
}

// StatePath derives the state file location from a plan file path:
// review.json -> review.state.json.
func StatePath(planPath string) string {
	ext := filepath.Ext(planPath)
	return strings.TrimSuffix(planPath, ext) + ".state" + ext
}

// LoadState reads existing state for a plan, or returns empty state if
// none has been saved yet.
func LoadState(planPath string) *State {
	s := &State{Reviewed: map[string]bool{}, path: StatePath(planPath)}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, s)
	if s.Reviewed == nil {
		s.Reviewed = map[string]bool{}
	}
	return s
}

// AddComment appends a reviewer comment to a unit and persists the state.
func (s *State) AddComment(key, text string) {
	if s.Comments == nil {
		s.Comments = map[string][]string{}
	}
	s.Comments[key] = append(s.Comments[key], text)
	s.save()
}

// RemoveLastComment drops a unit's newest comment (the undo for a typo)
// and persists the state. It is a no-op when the unit has none.
func (s *State) RemoveLastComment(key string) {
	cs := s.Comments[key]
	if len(cs) == 0 {
		return
	}
	if cs = cs[:len(cs)-1]; len(cs) == 0 {
		delete(s.Comments, key)
	} else {
		s.Comments[key] = cs
	}
	s.save()
}

// CommentsFor returns a unit's comments in the order they were written.
func (s *State) CommentsFor(key string) []string {
	return s.Comments[key]
}

// TotalComments counts comments across all units.
func (s *State) TotalComments() int {
	n := 0
	for _, cs := range s.Comments {
		n += len(cs)
	}
	return n
}

// Toggle flips the reviewed mark for a file and persists the state.
func (s *State) Toggle(path string) {
	s.Reviewed[path] = !s.Reviewed[path]
	if !s.Reviewed[path] {
		delete(s.Reviewed, path)
	}
	s.save()
}

// CountReviewed returns how many of the plan's units (sections, or
// whole files when a file has no sections) are marked reviewed.
func (s *State) CountReviewed(p *Plan) int {
	n := 0
	for _, st := range p.Steps {
		for _, f := range st.Files {
			if len(f.Sections) == 0 {
				if s.Reviewed[f.Path] {
					n++
				}
				continue
			}
			for i := range f.Sections {
				if s.Reviewed[f.SectionKey(i)] {
					n++
				}
			}
		}
	}
	return n
}

// FileReviewed reports whether a file is fully reviewed: its own mark
// for section-less files, all section marks otherwise.
func (s *State) FileReviewed(f *File) bool {
	if len(f.Sections) == 0 {
		return s.Reviewed[f.Path]
	}
	for i := range f.Sections {
		if !s.Reviewed[f.SectionKey(i)] {
			return false
		}
	}
	return true
}

func (s *State) save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0o644)
}

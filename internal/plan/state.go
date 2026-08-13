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

// Toggle flips the reviewed mark for a file and persists the state.
func (s *State) Toggle(path string) {
	s.Reviewed[path] = !s.Reviewed[path]
	if !s.Reviewed[path] {
		delete(s.Reviewed, path)
	}
	s.save()
}

// CountReviewed returns how many of the plan's files are marked reviewed.
func (s *State) CountReviewed(p *Plan) int {
	n := 0
	for _, st := range p.Steps {
		for _, f := range st.Files {
			if s.Reviewed[f.Path] {
				n++
			}
		}
	}
	return n
}

func (s *State) save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0o644)
}

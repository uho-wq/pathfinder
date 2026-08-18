package plan

import (
	"fmt"
	"strings"
)

// FormatComments renders the reviewer's comments as markdown, grouped by
// file in plan (review) order, so they can be pasted into a PR review.
// It returns "" when the state carries no comments for this plan.
func FormatComments(p *Plan, s *State) string {
	if s.TotalComments() == 0 {
		return ""
	}
	var b strings.Builder
	for _, st := range p.Steps {
		for fi := range st.Files {
			f := &st.Files[fi]
			var fb strings.Builder
			for _, c := range s.CommentsFor(f.Path) {
				fmt.Fprintf(&fb, "- %s\n", c)
			}
			for i := range f.Sections {
				sec := &f.Sections[i]
				cs := s.CommentsFor(f.SectionKey(i))
				if len(cs) == 0 {
					continue
				}
				fmt.Fprintf(&fb, "- %s%s\n", sec.Title, lineRange(sec))
				for _, c := range cs {
					fmt.Fprintf(&fb, "  - %s\n", c)
				}
			}
			if fb.Len() == 0 {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "## %s\n", f.Path)
			b.WriteString(fb.String())
		}
	}
	return b.String()
}

func lineRange(sec *Section) string {
	switch {
	case sec.StartLine <= 0:
		return ""
	case sec.EndLine > sec.StartLine:
		return fmt.Sprintf(" (L%d-L%d)", sec.StartLine, sec.EndLine)
	default:
		return fmt.Sprintf(" (L%d)", sec.StartLine)
	}
}

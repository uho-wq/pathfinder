package ui

import (
	"path/filepath"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
)

// Syntax highlighting for diff content. Each line is tokenized on its
// own with the lexer chroma picks for the file name, and tokens become
// foreground-color spans that renderDiffLine lays over the add/del
// background tints. Per-line lexing loses multi-line state (a block
// comment opened on an earlier line), which is the usual trade-off for
// diff views: hunks only carry fragments of the file anyway.

// hlSpan colors the rune range lo..hi of a line.
type hlSpan struct {
	lo, hi int
	color  lipgloss.TerminalColor
}

type highlighter struct {
	lexer chroma.Lexer
}

// highlighterFor returns a highlighter for the given file path, or nil
// when chroma has no lexer for it (then the diff renders unhighlighted).
func highlighterFor(path string) *highlighter {
	if path == "" {
		return nil
	}
	lexer := lexers.Match(filepath.Base(path))
	if lexer == nil {
		return nil
	}
	return &highlighter{lexer: chroma.Coalesce(lexer)}
}

// spans tokenizes a single line and returns the colored ranges.
func (h *highlighter) spans(line string) []hlSpan {
	if h == nil || line == "" {
		return nil
	}
	it, err := h.lexer.Tokenise(nil, line)
	if err != nil {
		return nil
	}
	var out []hlSpan
	pos := 0
	for _, tok := range it.Tokens() {
		n := utf8.RuneCountInString(tok.Value)
		if c := tokenColor(tok.Type); c != nil {
			out = append(out, hlSpan{lo: pos, hi: pos + n, color: c})
		}
		pos += n
	}
	return out
}

// tokenColor maps chroma token categories to the syntax palette; nil
// means the default foreground.
func tokenColor(t chroma.TokenType) lipgloss.TerminalColor {
	switch {
	case t.Category() == chroma.Comment:
		return colorSynComment
	case t.SubCategory() == chroma.LiteralString:
		return colorSynString
	case t.SubCategory() == chroma.LiteralNumber:
		return colorSynNumber
	case t.Category() == chroma.Keyword,
		t == chroma.NameBuiltin, t == chroma.NameTag:
		return colorSynKeyword
	case t == chroma.NameFunction, t == chroma.NameClass,
		t == chroma.NameDecorator, t == chroma.NameAttribute:
		return colorSynFunc
	case t == chroma.NameConstant:
		return colorSynNumber
	default:
		return nil
	}
}

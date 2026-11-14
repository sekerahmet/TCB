package ast

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// LoopName is one name a loop binds (5.7) with the token where it is written; the token is zero
// when the name has no place in the source.
type LoopName struct {
	Name  string
	Token lexer.Token
}

// Names gives every name the loop binds, in the written order: the index or key (KeyVar), then
// the element (ValueVar) or the names of its tuple pattern (Pattern). A name the loop does not
// write is left out. Every walker that reads the loop's names takes them from here, so a pattern's
// names are bound wherever the loop's names are (checker, compiler, scope walkers).
func (fs *ForInStatement) Names() []LoopName {
	var out []LoopName
	if fs.KeyVar != "" {
		out = append(out, LoopName{fs.KeyVar, fs.KeyToken})
	}
	if fs.Pattern != nil {
		return append(out, fs.Pattern...)
	}
	if fs.ValueVar != "" {
		out = append(out, LoopName{fs.ValueVar, fs.ValueToken})
	}
	return out
}

// PatternText is the pattern's spelling: "(a, b)", names joined by ", " in parentheses.
func PatternText(names []string) string {
	return "(" + strings.Join(names, ", ") + ")"
}

// loopHeader is the loop's header with value in the element's place, in the file's identity
// spelling: TR_for i, (a, b) TR_in l. The iterable is its source text (IterableToken).
func (fs *ForInStatement) loopHeader(value string) string {
	head := value
	if fs.KeyVar != "" {
		head = fs.KeyVar + ", " + value
	}
	return codes.Spelling(codes.KwFor) + " " + head + " " + codes.Spelling(codes.KwIn) + " " + fs.IterableToken.Literal
}

// PatternSpelling is the written pattern: "(a, b)".
func (fs *ForInStatement) PatternSpelling() string { return PatternText(fs.patternNames()) }

// PatternHead is the loop's names with the element taken by one name, for a message that suggests
// dropping the pattern: the pattern's first name other than "_" ("x" when every name is "_"), after
// the index when the loop has one ("i, a").
func (fs *ForInStatement) PatternHead() string {
	head := "x"
	for _, n := range fs.Pattern {
		if n.Name != "_" {
			head = n.Name
			break
		}
	}
	if fs.KeyVar != "" {
		return fs.KeyVar + ", " + head
	}
	return head
}

// TupleSuggestion is what H2730 suggests for a two-name loop over a tuple of count elements (5.7):
// the two written names as a pattern filled up with "_" ("(a, b)", "(a, b, _)") and a free index
// name for the indexed form ("i"; "j" when the pattern has i, then "k").
func (fs *ForInStatement) TupleSuggestion(count int) (pattern, index string) {
	names := PadNames([]string{fs.KeyVar, fs.ValueVar}, count)
	index = "i"
	for _, free := range []string{"i", "j", "k"} {
		if free != fs.KeyVar && free != fs.ValueVar {
			index = free
			break
		}
	}
	return PatternText(names), index
}

// patternNames are the pattern's names in order.
func (fs *ForInStatement) patternNames() []string {
	names := make([]string, len(fs.Pattern))
	for i, n := range fs.Pattern {
		names[i] = n.Name
	}
	return names
}

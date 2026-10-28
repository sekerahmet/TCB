package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Where the checker's errors and warnings are (docs/spec/02_sozcuk_yapisi.md 2.13): at a node of the
// tree, from its first token to the end of its last (ast.RangeOf), or at one token. The record
// carries a start column and an end, so the text shows "Satır 3, Sütun 5:" and a tool reads the
// range. A new check puts its error at the node it is about (addErrorAt, addWarningAt), at a
// name's token when it is about a name (addErrorAtToken), and adds a fix to the record that comes
// back (WithFix) when the check knows the text that would put it right.

// addErrorAt records err at node n and returns the record.
func (c *Checker) addErrorAt(n ast.Node, err error) *messages.PosError {
	e := nodeError(n, err)
	c.errors = append(c.errors, e)
	return e
}

// addErrorAtToken records err at the token tok and returns the record. A token with no column (one
// the tree did not keep: siteOf) gives only the line.
func (c *Checker) addErrorAtToken(tok lexer.Token, err error) *messages.PosError {
	e := tokenError(tok, err)
	c.errors = append(c.errors, e)
	return e
}

// addWarningAt records the warning err at node n and returns the record.
func (c *Checker) addWarningAt(n ast.Node, err error) *messages.PosError {
	e := nodeError(n, err)
	c.warnings = append(c.warnings, e)
	return e
}

// addWarningAtToken records the warning err at the token tok and returns the record.
func (c *Checker) addWarningAtToken(tok lexer.Token, err error) *messages.PosError {
	e := tokenError(tok, err)
	c.warnings = append(c.warnings, e)
	return e
}

func nodeError(n ast.Node, err error) *messages.PosError {
	if r := ast.RangeOf(n); r.Line > 0 {
		return messages.AtRange(r, err)
	}
	return messages.At(n.Line(), 0, err)
}

func tokenError(tok lexer.Token, err error) *messages.PosError {
	return messages.AtRange(tok.Range(), err)
}

// siteOf is the token a name is written at, or when the tree kept none (a parameter the parser
// made, a catch variable that was not written) a token with only the line.
func siteOf(tok lexer.Token, line int) lexer.Token {
	if tok.Line > 0 {
		return tok
	}
	return lexer.Token{Line: line}
}

// replaceToken is the fix that replaces the name at tok with text. The tree keeps a name without
// its '$' escape; the range of the token includes it, so the replacement puts it back.
func replaceToken(tok lexer.Token, name, text string, applicable bool) messages.Fix {
	r := tok.Range()
	if r.EndLine == r.Line && r.EndCol-r.Col == len([]rune(name))+1 {
		text = "$" + text
	}
	return messages.Fix{Range: r, Replacement: text, Applicable: applicable}
}

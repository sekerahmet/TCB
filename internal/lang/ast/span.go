package ast

import (
	"reflect"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

var tokenKind = reflect.TypeOf(lexer.Token{})

// RangeOf is the place of node n in the file (2.13): from the first of its tokens to the end of the
// last, among the tokens of the node and everything under it. The tree keeps the tokens the parser
// needs, not every character: a closing ')' or '}' is not among them, so the range of a call ends at its
// last argument. A node with no token of its own place (a synthesized one) has no range.
func RangeOf(n any) messages.Range {
	var out messages.Range
	var rec func(v reflect.Value)
	rec = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			if !v.IsNil() {
				rec(v.Elem())
			}
		case reflect.Struct:
			if v.Type() == tokenKind {
				if tok := v.Interface().(lexer.Token); tok.Column > 0 {
					out = union(out, tok.Range())
				}
				return
			}
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					rec(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				rec(v.Index(i))
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				rec(v.MapIndex(k))
			}
		}
	}
	rec(reflect.ValueOf(n))
	return out
}

// union is the range from the first start to the last end of a and b; a range with no line is
// nothing.
func union(a, b messages.Range) messages.Range {
	switch {
	case a.Line == 0:
		return b
	case b.Line == 0:
		return a
	}
	out := a
	if b.Line < a.Line || b.Line == a.Line && b.Col < a.Col {
		out.Line, out.Col = b.Line, b.Col
	}
	if b.EndLine > a.EndLine || b.EndLine == a.EndLine && b.EndCol > a.EndCol {
		out.EndLine, out.EndCol = b.EndLine, b.EndCol
	}
	return out
}

// UnpackSuggestion builds the select key ("extra" when there are more names than elements,
// "missing" when fewer) and the corrected statement example for a tuple unpack count mismatch
// (5.3, 6.3). The right-hand side comes from ValueToken, cut from the source by token positions.
func UnpackSuggestion(stmt Node, count int) (kind, example string) {
	var prefix, rhs string
	var names []string
	switch s := stmt.(type) {
	case *MultiVarStatement:
		kw := codes.KwVar
		if s.IsConst {
			kw = codes.KwConst
		}
		prefix = codes.Spelling(kw) + " "
		for _, n := range s.Names {
			names = append(names, n.Value)
		}
		rhs = s.ValueToken.Literal
	case *MultiAssignStatement:
		for _, t := range s.Targets {
			names = append(names, targetName(t))
		}
		rhs = s.ValueToken.Literal
	}
	if len(names) > count {
		return "extra", prefix + strings.Join(names[:count], ", ") + " = " + rhs
	}
	padded := append([]string(nil), names...)
	for len(padded) < count {
		padded = append(padded, "_")
	}
	return "missing", prefix + strings.Join(padded, ", ") + " = " + rhs
}

func targetName(e Expression) string {
	switch x := e.(type) {
	case *Identifier:
		return x.Value
	case *MemberExpression:
		dot := "."
		if x.Safe {
			dot = "?."
		}
		return targetName(x.Object) + dot + x.Property
	}
	return e.TokenLiteral()
}

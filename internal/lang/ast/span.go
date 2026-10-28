package ast

import (
	"reflect"

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

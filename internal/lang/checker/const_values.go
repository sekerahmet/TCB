package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// constScope knows the top-level names of a module or a test file for the constant-expression rule
// (1.1): constants, functions and types are constant, a TR_var is not. A name the file does not
// define (an imported or a global name) is taken as constant: the checker cannot know it, and it
// does not raise a false alarm.
type constScope struct {
	vars    map[string]bool
	structs map[string]*ast.StructStatement
}

func newConstScope(prog *ast.Program) *constScope {
	k := &constScope{vars: map[string]bool{}, structs: map[string]*ast.StructStatement{}}
	for _, stmt := range prog.Statements {
		if s, ok := stmt.(*ast.StructStatement); ok {
			k.structs[s.Name] = s
		}
	}
	return k
}

// define records a top-level TR_var (not constant) or TR_const once its value is checked.
func (k *constScope) define(name string, isConst bool) {
	if isConst {
		delete(k.vars, name)
	} else {
		k.vars[name] = true
	}
}

// numberBuiltins are the builtins whose result depends only on their number arguments (TR_sqrt(64.0));
// valueBuiltins return a value or a function without running code: TR_Bytes(...) of constants, and
// TR_fuse, TR_replay, TR_custom_grad, which return a function without calling it.
var numberBuiltins = map[codes.Code]bool{codes.FnAbs: true, codes.FnAcos: true, codes.FnAsin: true,
	codes.FnAtan: true, codes.FnAtan2: true, codes.FnCeil: true, codes.FnClamp: true, codes.FnCos: true,
	codes.FnCosh: true, codes.FnExp: true, codes.FnExpm1: true, codes.FnFloor: true, codes.FnIsFinite: true,
	codes.FnLog: true, codes.FnLog10: true, codes.FnLog1p: true, codes.FnLog2: true, codes.FnMax: true,
	codes.FnMin: true, codes.FnRound: true, codes.FnSign: true, codes.FnSin: true, codes.FnSinh: true,
	codes.FnSqrt: true, codes.FnTan: true, codes.FnTanh: true}

var valueBuiltins = map[codes.Code]bool{codes.TypeBytes: true, codes.FnFuse: true, codes.FnReplay: true,
	codes.FnCustomGrad: true}

// nonConstant is the part of e that is not a constant expression, nil when e is constant (1.1): a
// number, a text whose inserted values are constant, a truth value, TR_none, a constant and their
// operations; lists and dicts of constants; a struct without a constructor and an enum state with
// constant fields; the builtins above with constant arguments; a function name, and a function
// expression that reads no top-level TR_var.
func (k *constScope) nonConstant(e ast.Expression) ast.Node {
	switch e := e.(type) {
	case nil, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.NullLiteral:
		return nil
	case *ast.Identifier:
		if k.vars[e.Value] {
			return e
		}
		return nil
	case *ast.InterpolatedString:
		return k.first(e.Parts...)
	case *ast.PrefixExpression:
		return k.nonConstant(e.Right)
	case *ast.InfixExpression:
		return k.first(e.Left, e.Right)
	case *ast.RangeExpression:
		return k.first(e.Start, e.End)
	case *ast.ListLiteral:
		return k.first(e.Elements...)
	case *ast.TupleExpression:
		return k.first(e.Elements...)
	case *ast.DictLiteral:
		for _, p := range e.Pairs {
			if bad := k.first(p.Key, p.Value); bad != nil {
				return bad
			}
		}
		return nil
	case *ast.IndexExpression:
		return k.first(e.Left, e.Index)
	case *ast.ElementExpression:
		return k.nonConstant(e.Object)
	case *ast.MemberExpression:
		if k.typeName(e.Object) {
			return nil
		}
		return k.nonConstant(e.Object)
	case *ast.FunctionLiteral:
		return k.captured(e)
	case *ast.CallExpression:
		return k.call(e)
	}
	return e
}

func (k *constScope) first(es ...ast.Expression) ast.Node {
	for _, e := range es {
		if bad := k.nonConstant(e); bad != nil {
			return bad
		}
	}
	return nil
}

// call is constant when its arguments are and it calls a builtin above, a struct of the file
// without a constructor, or an enum's state (Renk.Özel(1)).
func (k *constScope) call(c *ast.CallExpression) ast.Node {
	if bad := k.first(c.Arguments...); bad != nil {
		return bad
	}
	for _, na := range c.NamedArgs {
		if bad := k.nonConstant(na.Value); bad != nil {
			return bad
		}
	}
	switch f := c.Function.(type) {
	case *ast.Identifier:
		if numberBuiltins[f.Token.Code] || valueBuiltins[f.Token.Code] {
			return nil
		}
		if s := k.structs[f.Value]; s != nil && s.Kind == ast.KindStruct && !hasConstructor(s) {
			return nil
		}
	case *ast.MemberExpression:
		if k.enumName(f.Object) {
			return nil
		}
	}
	return c
}

// enumName says e names an enum of the file: calling its member builds a state (Renk.Özel(1)).
// A call on any other type's member (TR_Tensor.TR_zeros) runs code (1.1: no other call).
func (k *constScope) enumName(e ast.Expression) bool {
	id, ok := e.(*ast.Identifier)
	if !ok {
		return false
	}
	s := k.structs[id.Value]
	return s != nil && s.Kind == ast.KindEnum
}

// typeName says e names an enum of the file or a type of the language or its library (a state
// value: Renk.Kırmızı).
func (k *constScope) typeName(e ast.Expression) bool {
	id, ok := e.(*ast.Identifier)
	if !ok {
		return false
	}
	if s := k.structs[id.Value]; s != nil {
		return s.Kind == ast.KindEnum
	}
	return id.Token.Code != codes.None && id.Token.Code.Namespace() == codes.NamespaceType
}

func hasConstructor(s *ast.StructStatement) bool {
	for _, m := range s.Methods {
		if m.NameToken.Code == codes.CtxInit {
			return true
		}
	}
	return false
}

// captured is the first top-level TR_var a function expression reads (one it does not define
// itself as a parameter, a variable or a nested function's parameter); nil when it reads none.
func (k *constScope) captured(f *ast.FunctionLiteral) ast.Node {
	own := ownNames(f)
	var found ast.Node
	ast.Walk(f.Body, func(n any) bool {
		if id, ok := n.(*ast.Identifier); ok && found == nil && k.vars[id.Value] && !own[id.Value] {
			found = id
		}
		return found == nil
	})
	return found
}

// ownNames are the names f defines itself: its parameters, its variables and the parameters of
// the function expressions nested in it.
func ownNames(f *ast.FunctionLiteral) map[string]bool {
	own := map[string]bool{}
	for _, p := range f.Parameters {
		own[p.Name] = true
	}
	ast.Walk(f.Body, func(n any) bool {
		switch s := n.(type) {
		case *ast.FunctionLiteral:
			// iç fonksiyon ifadesinin parametresi de kendi adıdır (en dıştaki aynı adlı TR_var değil)
			for _, p := range s.Parameters {
				own[p.Name] = true
			}
		case *ast.VarStatement:
			own[s.Name.Value] = true
		case *ast.MultiVarStatement:
			for _, name := range s.Names {
				own[name.Value] = true
			}
		case *ast.ForInStatement:
			own[s.KeyVar], own[s.ValueVar] = true, true
		}
		return true
	})
	return own
}

// sharedVars are the names functions assign to without defining them (ownNames): a top-level
// TR_var among them is shared state (H1114).
func sharedVars(prog *ast.Program) map[string]bool {
	out := map[string]bool{}
	ast.Walk(prog, func(n any) bool {
		f, ok := n.(*ast.FunctionLiteral)
		if !ok || f.Body == nil {
			return true
		}
		own := ownNames(f)
		ast.Walk(f.Body, func(m any) bool {
			var targets []ast.Expression
			switch s := m.(type) {
			case *ast.AssignStatement:
				targets = []ast.Expression{s.Target}
			case *ast.MultiAssignStatement:
				targets = s.Targets
			}
			for _, t := range targets {
				if id, ok := t.(*ast.Identifier); ok && !own[id.Value] {
					out[id.Value] = true
				}
			}
			return true
		})
		return true
	})
	return out
}

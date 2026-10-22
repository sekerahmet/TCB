// Package checker is TR's type checker that runs before execution: it walks the syntax tree
// and finds type mismatches (declaration, assignment, return value, call argument),
// assignments to constants, undefined names and shadowed builtin names; it reports errors
// and warnings with their lines ('tr kontrol', and before running or loading a module).
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TypeInfo is what the checker knows about a name's type.
type TypeInfo struct {
	TypeName   string
	IsConst    bool
	ParamTypes []string
	ReturnType string
}

// Scope is one level of the checker's type scope.
type Scope struct {
	parent  *Scope
	symbols map[string]TypeInfo
}

// NewScope returns a scope nested in parent (nil for the outermost scope).
func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]TypeInfo),
	}
}

// Define records name in this scope.
func (s *Scope) Define(name string, info TypeInfo) {
	s.symbols[name] = info
}

// Resolve looks name up in this scope and its parents.
func (s *Scope) Resolve(name string) (TypeInfo, bool) {
	if info, ok := s.symbols[name]; ok {
		return info, true
	}
	if s.parent != nil {
		return s.parent.Resolve(name)
	}
	return TypeInfo{}, false
}

// Checker checks programs; in the REPL one checker keeps its scope between lines.
type Checker struct {
	scope  *Scope
	errors []error
	// warnings programın çalışmasını engellemeyen bulgulardır (ör. yerleşik ad gölgeleme).
	warnings []error
	// interactive kabukta açıktır: ifade deyiminin değeri gösterildiği için etkisiz sayılmaz.
	interactive bool
	// İçinde bulunulan fonksiyonun bildirilmiş dönüş tipi ("" = fonksiyon dışı veya tipsiz)
	returnType string
	// globalNames tanımsız ad denetiminin genel kapsamıdır (kabukta satırlar arasında kalır);
	// adHatalari son CheckProgram çağrısının tanımsız ad hatalarıdır (undefined_names.go).
	globalNames *nameScope
	nameErrors  []error
}

// New returns a checker with an empty global scope.
func New() *Checker {
	return &Checker{
		scope:  NewScope(nil),
		errors: []error{},
	}
}

// Errs returns every error found so far, each at its line (messages.Pos).
func (c *Checker) Errs() []error {
	return c.errors
}

// Errors returns the texts of Errs ("Satır N: ...").
func (c *Checker) Errors() []string {
	return messages.Texts(c.errors)
}

// addError records err at the given position (col 0 when unknown).
func (c *Checker) addError(line, col int, err error) {
	c.errors = append(c.errors, messages.At(line, col, err))
}

// Warnings returns the checker's warnings, each at its line (messages.Pos). Warnings are not errors: the program runs and
// the exit code is unaffected; the command line prints them under a separate heading.
func (c *Checker) Warnings() []error {
	return c.warnings
}

// defineName bir adı kapsama tanımlar: önce gölgeleme uyarısı, sonra yazım kuralı denetimi, en
// sonda kapsam kaydı. Değeri çıkarılacak tanımlarda (değişken bildirimi) kayıt değerden sonra
// yapılır; onlar bu yardımcıyı kullanmaz.
func (c *Checker) defineName(name string, kind nameKind, line int, info TypeInfo) {
	c.checkShadowing(name, line)
	c.checkNameStyle(name, kind, line)
	c.scope.Define(name, info)
}

// checkShadowing ad bir yerleşik fonksiyonun adıysa (sözlükte G/T/V/W yerleşik kodu) uyarı
// ekler: kullanıcı tanımı o kapsamda yerleşiği gölgeler; yerleşik orada artık çağrılamaz.
func (c *Checker) checkShadowing(name string, line int) {
	if k, ok := codes.Find(codes.NamespaceGlobal, name); ok && k.IsBuiltin() {
		c.warnings = append(c.warnings, messages.At(line, 0, messages.New(messages.H1009, name)))
	}
}

// CheckProgram checks prog and returns the errors it found.
func (c *Checker) CheckProgram(prog *ast.Program) []string {
	c.check(prog)
	return c.Errors()
}

// check checks prog and records the errors it found.
func (c *Checker) check(prog *ast.Program) {
	start := len(c.errors)
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	// Tanımsız adlar (5.14) tip hatalarıyla birlikte, satır sırasıyla bildirilir.
	c.nameErrors = c.nameErrors[:0]
	c.checkNames(prog)
	c.errors = append(c.errors, c.nameErrors...)
	sortByLine(c.errors[start:])
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.VarStatement:
		c.checkShadowing(s.Name.Value, s.Line())
		c.checkNameStyle(s.Name.Value, varKind(s.IsConst), s.Line())
		declared := c.checkedTypeName(s.TypeName, s.Line())
		inferred := c.inferExpression(s.Value)
		if s.TypeName != "" && !typesCompatible(declared, inferred) {
			c.addError(s.Line(), 0, messages.New(messages.H1001,
				s.Name.Value, declared, inferred, numberHint(declared, inferred)))
		}
		// Dil dinamiktir: açıklamasız bir değişkenin ilk değerden çıkarılan tipi kalıcı bir
		// bildirim değildir (değişken sonra başka tipte değer alabilir), bu yüzden tipsiz
		// sayılır. Sabitler yeniden atanamadığı için çıkarılan tipleri kalır.
		finalType := typeAny
		switch {
		case s.TypeName != "":
			finalType = declared
		case s.IsConst:
			finalType = inferred
		}
		c.scope.Define(s.Name.Value, TypeInfo{
			TypeName: finalType,
			IsConst:  s.IsConst,
		})

	case *ast.AssignStatement:
		c.checkAssign(s)

	case *ast.BlockStatement:
		prev := c.scope
		c.scope = NewScope(prev)
		for _, bs := range s.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *ast.ForInStatement:
		c.inferExpression(s.Iterable)
		prev := c.scope
		c.scope = NewScope(prev)
		if s.KeyVar != "" {
			c.defineName(s.KeyVar, kindVariable, s.Line(), TypeInfo{TypeName: typeAny})
		}
		c.defineName(s.ValueVar, kindVariable, s.Line(), TypeInfo{TypeName: typeAny})
		for _, bs := range s.Body.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *ast.WhileStatement:
		c.inferExpression(s.Condition)
		c.checkStatement(s.Body)

	case *ast.TryCatchStatement:
		c.checkStatement(s.TryBody)
		prev := c.scope
		c.scope = NewScope(prev)
		catchLine := s.Line()
		if s.CatchBody != nil {
			catchLine = s.CatchBody.Line()
		}
		// Yakalanan hata Metin'dir, ama değişken açıklamasızdır: sonra başka tipte değer alabilir.
		c.defineName(s.ErrorVar, kindVariable, catchLine, TypeInfo{TypeName: typeAny})
		for _, bs := range s.CatchBody.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *ast.StructStatement:
		c.checkShadowing(s.Name, s.Line())
		c.checkNameStyle(s.Name, kindType, s.Line())
		for _, f := range s.Fields {
			c.checkNameStyle(f.Name, kindValue, s.Line())
		}
		for name, m := range s.Methods {
			c.checkNameStyle(name, kindValue, m.Line())
		}
		c.scope.Define(s.Name, TypeInfo{TypeName: s.Name})
		c.checkStruct(s)

	case *ast.MultiVarStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, name := range s.Names {
			c.defineName(name.Value, varKind(s.IsConst), s.Line(), TypeInfo{TypeName: typeAny, IsConst: s.IsConst})
		}

	case *ast.MultiAssignStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, t := range s.Targets {
			ident, ok := t.(*ast.Identifier)
			if !ok {
				c.inferExpression(t)
				continue
			}
			if info, found := c.scope.Resolve(ident.Value); found && info.IsConst {
				c.addError(s.Line(), 0, messages.New(messages.H1002, ident.Value, codes.DisplayName(codes.A031)))
			}
		}

	case *ast.DeferStatement:
		if s.Body != nil {
			c.checkStatement(s.Body)
		}
		if s.Call != nil {
			c.inferExpression(s.Call)
		}

	case *ast.ReturnStatement:
		valType := c.inferExpression(s.ReturnValue)
		if c.returnType != "" && !typesCompatible(c.returnType, valType) {
			c.addError(s.Line(), 0, messages.New(messages.H1003,
				c.returnType, valType, numberHint(c.returnType, valType)))
		}

	case *ast.ThrowStatement:
		c.inferExpression(s.Value)

	case *ast.AsyncStatement:
		if s.Body != nil {
			c.checkStatement(s.Body)
		}
		if s.Call != nil {
			c.inferExpression(s.Call)
		}

	case *ast.BreakpointStatement:
		c.inferExpression(s.Condition)

	case *ast.TestStatement:
		c.checkStatement(s.Body)

	case *ast.ExpressionStatement:
		c.inferExpression(s.Expression)
		c.warnUnusedNegation(s)
	}
}

// SetInteractive marks the checker as the shell's: an expression statement's value is shown
// there, so it is not unused.
func (c *Checker) SetInteractive() { c.interactive = true }

// warnUnusedNegation değeri kullanılmayan '-x' deyimini bildirir (2.8): satır başındaki '-'
// önceki satırı sürdürmediği için 'a = 1' ardından gelen '- b' sessizce etkisiz kalırdı.
func (c *Checker) warnUnusedNegation(s *ast.ExpressionStatement) {
	if c.interactive {
		return
	}
	if p, ok := s.Expression.(*ast.PrefixExpression); ok && p.Operator == "-" {
		c.warnings = append(c.warnings, messages.At(p.Token.Line, 0, messages.New(messages.H1011)))
	}
}

func (c *Checker) inferExpression(expr ast.Expression) string {
	if expr == nil {
		return typeNull
	}
	if t, ok := c.inferNested(expr); ok {
		return t
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return typeInt
	case *ast.FloatLiteral:
		return typeFloat
	case *ast.StringLiteral:
		return typeString
	case *ast.BooleanLiteral:
		return typeBool
	case *ast.NullLiteral:
		return typeNull
	case *ast.Identifier:
		if info, ok := c.scope.Resolve(e.Value); ok {
			return info.TypeName
		}
		return typeAny
	case *ast.PrefixExpression:
		right := c.inferExpression(e.Right)
		if e.Operator == "!" {
			return typeBool
		}
		return right
	case *ast.InfixExpression:
		left := c.inferExpression(e.Left)
		right := c.inferExpression(e.Right)
		c.checkTypeTextComparison(e)
		switch e.Operator {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return typeBool
		case "??":
			if left != typeNull && left != typeAny {
				return left
			}
			return right
		case "|>":
			return typeAny
		case "@":
			return typeTensor
		case "+", "-", "*", "/", "%", "**":
			lit, intConst := e.Right.(*ast.IntegerLiteral)
			return arithmeticType(e.Operator, left, right, intConst && lit.Value >= 0)
		}
		return typeAny
	case *ast.FunctionLiteral:
		paramTypes, retType := c.functionSignature(e)
		if e.Name != "" {
			c.defineName(e.Name, kindValue, e.Line(), TypeInfo{
				TypeName:   typeFunction,
				ParamTypes: paramTypes,
				ReturnType: retType,
			})
		}
		c.functionBody(e, retType)
		return typeFunction
	case *ast.CallExpression:
		var info TypeInfo
		var found bool
		ident, isIdent := e.Function.(*ast.Identifier)
		if isIdent {
			info, found = c.scope.Resolve(ident.Value)
		} else {
			c.inferExpression(e.Function)
		}
		for i, arg := range e.Arguments {
			argType := c.inferExpression(arg)
			if found && i < len(info.ParamTypes) && !typesCompatible(info.ParamTypes[i], argType) {
				c.addError(e.Line(), 0, messages.New(messages.H1004,
					ident.Value, i+1, info.ParamTypes[i], argType))
			}
		}
		for _, na := range e.NamedArgs {
			c.inferExpression(na.Value)
		}
		if found && info.TypeName == typeFunction && info.ReturnType != "" {
			return info.ReturnType
		}
		return typeAny
	case *ast.IfExpression:
		c.inferExpression(e.Condition)
		c.checkStatement(e.Consequence)
		for _, elif := range e.ElseIfs {
			c.inferExpression(elif.Condition)
			c.checkStatement(elif.Consequence)
		}
		if e.Alternative != nil {
			c.checkStatement(e.Alternative)
		}
		return typeAny
	case *ast.MatchExpression:
		c.inferExpression(e.Subject)
		for _, mc := range e.Cases {
			for _, p := range mc.Patterns {
				c.inferExpression(p)
			}
			if mc.Body != nil {
				c.checkStatement(mc.Body)
			}
			if mc.ValueExpr != nil {
				c.inferExpression(mc.ValueExpr)
			}
		}
		return typeAny
	}
	return typeAny
}

// checkAssign atamayı denetler: sabite atama ve açıklamalı değişkene uyumsuz tipte değer
// hatadır; indeks ve alan hedeflerinin içi de denetlenir.
func (c *Checker) checkAssign(s *ast.AssignStatement) {
	valType := c.inferExpression(s.Value)
	if ident, ok := s.Target.(*ast.Identifier); ok {
		if info, found := c.scope.Resolve(ident.Value); found {
			// Bileşik atamada (x += y) atanan değer 'x + y' ifadesinin sonucudur
			if op := strings.TrimSuffix(s.Operator, "="); op != "" {
				valType = arithmeticType(op, info.TypeName, valType, false)
			}
			if info.IsConst {
				c.addError(s.Line(), 0, messages.New(messages.H1002, ident.Value, codes.DisplayName(codes.A031)))
			}
			if info.TypeName != typeAny && info.TypeName != typeNull && !typesCompatible(info.TypeName, valType) {
				c.addError(s.Line(), 0, messages.New(messages.H1005,
					ident.Value, info.TypeName, valType, numberHint(info.TypeName, valType)))
			}
		}
	} else {
		// indeks ve alan hedefleri (l[i] = ..., p.x = ...) okunan ifadeler içerir
		c.inferExpression(s.Target)
	}
}

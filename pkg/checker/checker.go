package checker

import (
	"fmt"
	"strings"
	"tr_dili/pkg/ast"
)

type TypeInfo struct {
	TypeName string
	IsConst  bool
	ParamTypes []string
	ReturnType string
}

type Scope struct {
	parent  *Scope
	symbols map[string]TypeInfo
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]TypeInfo),
	}
}

func (s *Scope) Define(name string, info TypeInfo) {
	s.symbols[name] = info
}

func (s *Scope) Resolve(name string) (TypeInfo, bool) {
	if info, ok := s.symbols[name]; ok {
		return info, true
	}
	if s.parent != nil {
		return s.parent.Resolve(name)
	}
	return TypeInfo{}, false
}

type Checker struct {
	scope  *Scope
	errors []string
}

func New() *Checker {
	return &Checker{
		scope:  NewScope(nil),
		errors: []string{},
	}
}

func (c *Checker) Errors() []string {
	return c.errors
}

func normalizeTypeName(t string) string {
	switch strings.ToLower(t) {
	case "tam", "tamsayı", "tamsayi", "int":
		return "Tam"
	case "ondalık", "ondalik", "sayı", "sayi", "float":
		return "Ondalık"
	case "metin", "yazı", "yazi", "str", "string":
		return "Metin"
	case "mantık", "mantik", "mantıksal", "mantiksal", "bool":
		return "Mantık"
	case "liste", "dizi", "list":
		return "Liste"
	case "sözlük", "sozluk", "harita", "dict", "map":
		return "Sözlük"
	case "matris", "matrix":
		return "Matris"
	case "tensör", "tensor":
		return "Tensör"
	case "yok", "boş", "bos", "null":
		return "Yok"
	case "":
		return "Herhangi"
	default:
		return t
	}
}

func typesCompatible(expected, actual string) bool {
	exp := normalizeTypeName(expected)
	act := normalizeTypeName(actual)
	if exp == "Herhangi" || act == "Herhangi" || act == "Yok" {
		return true
	}
	// Tam sayı, Ondalık beklenen yere atanabilir
	if exp == "Ondalık" && act == "Tam" {
		return true
	}
	return exp == act
}

func (c *Checker) CheckProgram(prog *ast.Program) []string {
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	return c.errors
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.VarStatement:
		inferred := c.inferExpression(s.Value)
		declared := normalizeTypeName(s.TypeName)
		if s.TypeName != "" && !typesCompatible(declared, inferred) {
			c.errors = append(c.errors, fmt.Sprintf(
				"Satır %d: Tip uyuşmazlığı — %q değişkeni %q tipinde tanımlandı ancak %q tipinde değer atandı",
				s.Line(), s.Name.Value, declared, inferred,
			))
		}
		finalType := inferred
		if s.TypeName != "" {
			finalType = declared
		}
		c.scope.Define(s.Name.Value, TypeInfo{
			TypeName: finalType,
			IsConst:  s.IsConst,
		})

	case *ast.AssignStatement:
		valType := c.inferExpression(s.Value)
		if ident, ok := s.Target.(*ast.Identifier); ok {
			if info, found := c.scope.Resolve(ident.Value); found {
				if info.IsConst {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Sabit hatası — %q bir sabittir ('sabit') ve değeri değiştirilemez",
						s.Line(), ident.Value,
					))
				}
				if info.TypeName != "Herhangi" && info.TypeName != "Yok" && !typesCompatible(info.TypeName, valType) {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Tip uyuşmazlığı — %q değişkeni (%s) içine %q tipinde değer atanamaz",
						s.Line(), ident.Value, info.TypeName, valType,
					))
				}
			}
		}

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
			c.scope.Define(s.KeyVar, TypeInfo{TypeName: "Herhangi"})
		}
		c.scope.Define(s.ValueVar, TypeInfo{TypeName: "Herhangi"})
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
		c.scope.Define(s.ErrorVar, TypeInfo{TypeName: "Metin"})
		for _, bs := range s.CatchBody.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *ast.StructStatement:
		c.scope.Define(s.Name, TypeInfo{TypeName: s.Name})

	case *ast.MultiVarStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, name := range s.Names {
			c.scope.Define(name.Value, TypeInfo{TypeName: "Herhangi", IsConst: s.IsConst})
		}

	case *ast.MultiAssignStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, t := range s.Targets {
			if ident, ok := t.(*ast.Identifier); ok {
				if info, found := c.scope.Resolve(ident.Value); found && info.IsConst {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Sabit hatası — %q bir sabittir ('sabit') ve değeri değiştirilemez",
						s.Line(), ident.Value,
					))
				}
			}
		}

	case *ast.DeferStatement:
		if s.Body != nil {
			c.checkStatement(s.Body)
		}
		if s.Call != nil {
			c.inferExpression(s.Call)
		}

	case *ast.ExpressionStatement:
		c.inferExpression(s.Expression)
	}
}

func (c *Checker) inferExpression(expr ast.Expression) string {
	if expr == nil {
		return "Yok"
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return "Tam"
	case *ast.FloatLiteral:
		return "Ondalık"
	case *ast.StringLiteral, *ast.InterpolatedString:
		return "Metin"
	case *ast.BooleanLiteral:
		return "Mantık"
	case *ast.NullLiteral:
		return "Yok"
	case *ast.ListLiteral, *ast.ListComprehension:
		return "Liste"
	case *ast.DictLiteral:
		return "Sözlük"
	case *ast.RangeExpression:
		return "Liste"
	case *ast.Identifier:
		if info, ok := c.scope.Resolve(e.Value); ok {
			return info.TypeName
		}
		return "Herhangi"
	case *ast.PrefixExpression:
		right := c.inferExpression(e.Right)
		if e.Operator == "!" {
			return "Mantık"
		}
		return right
	case *ast.InfixExpression:
		left := c.inferExpression(e.Left)
		right := c.inferExpression(e.Right)
		switch e.Operator {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return "Mantık"
		case "??":
			if left != "Yok" && left != "Herhangi" {
				return left
			}
			return right
		case "|>":
			return "Herhangi"
		case "@":
			return "Matris"
		case "+", "-", "*", "/", "%", "**":
			if left == "Metin" && e.Operator == "+" {
				return "Metin"
			}
			if left == "Ondalık" || right == "Ondalık" {
				return "Ondalık"
			}
			if left == "Tam" && right == "Tam" {
				return "Tam"
			}
		}
		return "Herhangi"
	case *ast.FunctionLiteral:
		var paramTypes []string
		for _, p := range e.Parameters {
			paramTypes = append(paramTypes, normalizeTypeName(p.TypeName))
		}
		retType := normalizeTypeName(e.ReturnType)
		if e.Name != "" {
			c.scope.Define(e.Name, TypeInfo{
				TypeName:   "Fonksiyon",
				ParamTypes: paramTypes,
				ReturnType: retType,
			})
		}
		prev := c.scope
		c.scope = NewScope(prev)
		for _, p := range e.Parameters {
			c.scope.Define(p.Name, TypeInfo{TypeName: normalizeTypeName(p.TypeName)})
		}
		for _, stmt := range e.Body.Statements {
			c.checkStatement(stmt)
		}
		c.scope = prev
		return "Fonksiyon"
	case *ast.CallExpression:
		if ident, ok := e.Function.(*ast.Identifier); ok {
			if info, found := c.scope.Resolve(ident.Value); found && len(info.ParamTypes) > 0 {
				for i, arg := range e.Arguments {
					if i < len(info.ParamTypes) {
						argType := c.inferExpression(arg)
						if !typesCompatible(info.ParamTypes[i], argType) {
							c.errors = append(c.errors, fmt.Sprintf(
								"Satır %d: Fonksiyon çağrısı tip hatası — %q fonksiyonunun %d. parametresi %q bekliyor ancak %q verildi",
								e.Line(), ident.Value, i+1, info.ParamTypes[i], argType,
							))
						}
					}
				}
				return info.ReturnType
			}
		}
		for _, arg := range e.Arguments {
			c.inferExpression(arg)
		}
		return "Herhangi"
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
		return "Herhangi"
	case *ast.MatchExpression:
		c.inferExpression(e.Subject)
		for _, mc := range e.Cases {
			if mc.Body != nil {
				c.checkStatement(mc.Body)
			}
			if mc.ValueExpr != nil {
				c.inferExpression(mc.ValueExpr)
			}
		}
		return "Herhangi"
	}
	return "Herhangi"
}

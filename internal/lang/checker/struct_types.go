package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Yapının alanlarının tipi (7.2, 7.5): tipi bilinen yapı nesnesinin alanına atanan değer ve yapı
// kurulurken alana verilen değer alanın yazılan tipindedir (3.13). Çalışma anı alanın tipini
// denetlemez; bu hataların çalışma anında kodu yoktur (H1124, H1125).

// structField tipi bilinen yapının adı name olan, tipi yazılmış alanıdır.
func (c *Checker) structField(structName, name string) (ast.StructField, bool) {
	s := c.structs[structName]
	if s == nil {
		return ast.StructField{}, false
	}
	for _, f := range s.Fields {
		if f.Name == name && f.Type != nil {
			return f, true
		}
	}
	return ast.StructField{}, false
}

// checkFieldAssign tipi bilinen yapı nesnesinin alanına atamayı denetler (7.5, H1124): p.x = ifade,
// p.x += ifade. Nesne bir ad ya da TR_self olmalıdır (çıkarımı yan etkisizdir); öbür hedefler denetlenmez.
func (c *Checker) checkFieldAssign(s *ast.AssignStatement, valType string) {
	m, ok := s.Target.(*ast.MemberExpression)
	if !ok || m.Safe {
		return
	}
	if _, isName := m.Object.(*ast.Identifier); !isName {
		return
	}
	owner := c.inferExpression(m.Object)
	f, found := c.structField(owner, m.Property)
	if !found {
		return
	}
	declared := c.localType(normalizedType(f.Type))
	op := strings.TrimSuffix(string(s.Operator), "=")
	if op != "" {
		valType = arithmeticType(op, declared, valType)
	}
	if declared != typeAny && valType != typeAny && !c.assignable(declared, valType, s.Value) {
		c.addErrorAt(s, messages.New(messages.H1124, m.Property, c.typeArg(owner), c.typeArg(declared), c.typeArg(valType),
			c.typeHint(declared, valType, s.Value, op == "/" || op == "" && dividesAtTop(s.Value))))
	}
}

// checkConstruction yapı kurulurken alanlara verilen değerlerin tipini denetler (7.2, H1125): konumlu
// argümanlar alanlara tanım sırasıyla, adlı argümanlar adıyla gider. Kurucusu (TR_init, 7.4) olan
// yapı kurucunun imzasıyla kurulur; o burada denetlenmez.
func (c *Checker) checkConstruction(e *ast.CallExpression, name string, argTypes, namedTypes []string) {
	s := c.structs[name]
	if s == nil || s.Methods[codes.Name(codes.CtxInit)] != nil {
		return
	}
	for i, arg := range e.Arguments {
		if i < len(s.Fields) && s.Fields[i].Type != nil {
			c.checkFieldValue(name, s.Fields[i], arg, argTypes[i])
		}
	}
	for i, na := range e.NamedArgs {
		if f, ok := c.structField(name, na.Name); ok {
			c.checkFieldValue(name, f, na.Value, namedTypes[i])
		}
	}
}

func (c *Checker) checkFieldValue(structName string, f ast.StructField, value ast.Expression, given string) {
	declared := c.localType(normalizedType(f.Type))
	if declared != typeAny && given != typeAny && !c.assignable(declared, given, value) {
		c.addErrorAt(value, messages.New(messages.H1125, f.Name, c.typeArg(structName), c.typeArg(declared), c.typeArg(given),
			c.typeHint(declared, given, value, dividesAtTop(value))))
	}
}

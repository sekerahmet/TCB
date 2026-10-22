package checker

import (
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// Tip denetimi her düğümün içine iner (undefined_names.go'daki gezintiyle aynı düğümler):
// bileşik ifadelerin çocukları, liste üreteci, yapı alanları ve metotları, fonksiyon
// gövdeleri. İçteki bir hata (ör. listedeki bir çağrının tip hatası) sessizce geçmez.

// inferNested bileşik ifadelerin (metin içi ifade, liste, liste üreteci, sözlük, aralık,
// indeks, üye) çocuklarını denetler ve tipini verir; ifade bunlardan değilse ok false'tur.
func (c *Checker) inferNested(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.InterpolatedString:
		for _, p := range e.Parts {
			c.inferExpression(p)
		}
		return typeString, true
	case *ast.ListLiteral:
		for _, el := range e.Elements {
			c.inferExpression(el)
		}
		return typeList, true
	case *ast.ListComprehension:
		c.inferExpression(e.Iterable)
		prev := c.scope
		c.scope = NewScope(prev)
		c.scope.Define(e.VarName, TypeInfo{TypeName: typeAny})
		c.inferExpression(e.Output)
		if e.Condition != nil {
			c.inferExpression(e.Condition)
		}
		c.scope = prev
		return typeList, true
	case *ast.DictLiteral:
		for _, p := range e.Pairs {
			c.inferExpression(p.Key)
			c.inferExpression(p.Value)
		}
		return typeDict, true
	case *ast.RangeExpression:
		c.inferExpression(e.Start)
		c.inferExpression(e.End)
		return typeRange, true
	case *ast.IndexExpression:
		c.inferExpression(e.Left)
		c.inferExpression(e.Index)
		return typeAny, true
	case *ast.MemberExpression:
		c.inferExpression(e.Object)
		return typeAny, true
	}
	return "", false
}

// checkStruct yapının alan varsayılanlarını ve metot gövdelerini denetler.
func (c *Checker) checkStruct(s *ast.StructStatement) {
	for _, f := range s.Fields {
		c.inferExpression(f.DefaultValue)
	}
	for _, name := range sortedMethodNames(s) {
		// metodun adı kapsama tanımlanmaz: yalnızca 'nesne.metot' ile çağrılır
		m := s.Methods[name]
		_, retType := c.functionSignature(m)
		c.functionBody(m, retType)
	}
}

// functionSignature parametre ve dönüş tiplerinin yazımını denetler; parametre tiplerini ve
// dönüş tipini (yoksa Herhangi) verir.
func (c *Checker) functionSignature(e *ast.FunctionLiteral) (paramTypes []string, retType string) {
	for _, p := range e.Parameters {
		paramTypes = append(paramTypes, c.checkedTypeName(p.TypeName, e.Line()))
	}
	return paramTypes, c.checkedTypeName(e.ReturnType, e.Line())
}

// functionBody parametrelerin tanımlı olduğu yeni bir kapsamda varsayılan değerleri ve
// gövdeyi denetler; gövdedeki 'sonuç'lar retType'a göre denetlenir (açıklama yoksa denetlenmez).
func (c *Checker) functionBody(e *ast.FunctionLiteral, retType string) {
	prev, prevRet := c.scope, c.returnType
	c.scope = NewScope(prev)
	c.returnType = ""
	if e.ReturnType != "" {
		c.returnType = retType
	}
	for _, p := range e.Parameters {
		c.defineName(p.Name, kindValue, e.Line(), TypeInfo{TypeName: normalizeTypeName(p.TypeName)})
	}
	for _, p := range e.Parameters {
		if p.DefaultValue != nil {
			c.inferExpression(p.DefaultValue)
		}
	}
	if e.Body != nil {
		for _, stmt := range e.Body.Statements {
			c.checkStatement(stmt)
		}
	}
	c.scope, c.returnType = prev, prevRet
}

// sortedMethodNames yapının metot adlarını sıralı verir; hatalar kararlı sırada çıkar.
func sortedMethodNames(s *ast.StructStatement) []string {
	names := make([]string, 0, len(s.Methods))
	for name := range s.Methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

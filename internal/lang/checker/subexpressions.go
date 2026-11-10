package checker

import (
	"sort"

	"github.com/sekerahmet/TCB/internal/lang/ast"
)

// Tip denetimi her düğümün içine iner (undefined_names.go'daki gezintiyle aynı düğümler):
// bileşik ifadelerin çocukları, liste üreteci, yapı alanları ve metotları, fonksiyon
// gövdeleri. İçteki bir hata (ör. listedeki bir çağrının tip hatası) sessizce geçmez.

// inferNested bileşik ifadelerin (metin içi ifade, liste, demet, liste üreteci, sözlük, aralık,
// indeks, üye, demetin öğesi) çocuklarını denetler ve tipini verir; ifade bunlardan değilse ok false'tur.
func (c *Checker) inferNested(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.InterpolatedString:
		for _, p := range e.Parts {
			c.inferExpression(p)
		}
		return typeString, true
	case *ast.ListLiteral:
		elems := make([]string, len(e.Elements))
		for i, el := range e.Elements {
			elems[i] = c.itemType(el)
		}
		return c.listLiteralType(elems, e.Elements), true
	case *ast.TupleExpression:
		elems := make([]string, len(e.Elements))
		for i, el := range e.Elements {
			elems[i] = c.itemType(el)
		}
		return c.tupleLiteralType(elems), true
	case *ast.ListComprehension:
		_, value := loopTypes(c.inferExpression(e.Iterable), false)
		prev := c.scope
		c.scope = NewScope(prev)
		c.scope.Define(e.VarName, TypeInfo{TypeName: value})
		out := c.itemType(e.Output)
		c.condition(e.Condition)
		c.scope = prev
		if c.knownType(out) {
			// Öğe tipi ifadenin tipidir (1.2, 3.13): süzgeç her öğeyi atsa da boş listenin tipi bilinir
			// (15.18'in boş tensör listesi, 13'ün boş TR_List[TR_Float]'ı; noteListElem).
			return c.listLiteralType([]string{out}, []ast.Expression{e.Output}), true
		}
		return typeList, true
	case *ast.DictLiteral:
		keys, values := make([]string, len(e.Pairs)), make([]string, len(e.Pairs))
		keyExprs, valueExprs := make([]ast.Expression, len(e.Pairs)), make([]ast.Expression, len(e.Pairs))
		for i, p := range e.Pairs {
			keys[i] = c.itemType(p.Key)
			values[i] = c.itemType(p.Value)
			keyExprs[i], valueExprs[i] = p.Key, p.Value
		}
		c.checkLiteralKeys(keyExprs, keys) // 3.7 (dict_keys.go)
		return c.dictLiteralType(keys, values, keyExprs, valueExprs), true
	case *ast.RangeExpression:
		c.inferExpression(e.Start)
		c.inferExpression(e.End)
		return typeRange, true
	case *ast.IndexExpression:
		c.checkModuleIndex(e) // modül değeri köşeli ayraçla okunmaz ve atanmaz (11.2)
		left, index := c.itemType(e.Left), c.itemType(e.Index)
		c.checkDictKey(e.Index, left, index) // 3.7 (dict_keys.go)
		return indexType(left, index), true
	case *ast.MemberExpression:
		obj := c.inferExpression(e.Object)
		if t := c.variantType(e, false); t != "" {
			return t, true // durumlu tipin alansız durumu: C.Yok (7.7)
		}
		if t := c.memberValueType(e, obj, false); t != "" {
			return t, true // tipi bilinen çekirdek alıcının özelliği (1.2, 13.9)
		}
		return c.fieldType(obj, e), true // tipi bilinen yapının alanı (7.5)
	case *ast.ElementExpression:
		obj := c.inferExpression(e.Object)
		c.checkTupleIndex(e, obj)
		return elementType(obj, e.Index), true // demetin öğesi (3.6a)
	}
	return "", false
}

// checkStruct yapının alan varsayılanlarını ve metot gövdelerini denetler.
func (c *Checker) checkStruct(s *ast.StructStatement) {
	c.defineStruct(s)
	c.noteTypeDef(s)
	for _, f := range s.Fields {
		c.expect(f.Type, f.DefaultValue)
		c.noteDefault(f.DefaultValue, c.inferExpression(f.DefaultValue))
	}
	// durumlu tipin durumu BüyükDeve bir addır (2.3b, 7.7; uymayan ad uyarıdır), alanları değerdir
	for _, vr := range s.Variants {
		c.checkNameStyle(vr.Name, kindType, siteOf(vr.NameToken, s.Line()))
		for _, f := range vr.Fields {
			c.checkNameStyle(f.Name, kindValue, siteOf(f.NameToken, s.Line()))
			c.noteDefault(f.DefaultValue, c.inferExpression(f.DefaultValue))
		}
	}
	// Metotta TR_self yapının tipindedir (7.3); durumlu tip ve arayüz kurulmaz, bilinmez.
	prevSelf := c.selfType
	if s.Kind == ast.KindStruct {
		c.selfType = s.Name
	}
	for _, name := range sortedMethodNames(s) {
		// metodun adı kapsama tanımlanmaz: yalnızca 'nesne.metot' ile çağrılır
		m := s.Methods[name]
		_, retType := c.functionSignature(m)
		c.functionBody(m, retType)
	}
	c.selfType = prevSelf
}

// functionSignature parametre ve dönüş tiplerinin yazımını denetler; parametre tiplerini ve
// dönüş tipini (yoksa Herhangi) verir.
func (c *Checker) functionSignature(e *ast.FunctionLiteral) (paramTypes []string, retType string) {
	for _, p := range e.Parameters {
		paramTypes = append(paramTypes, c.writtenType(p.Type, e.Line()))
	}
	return paramTypes, c.writtenType(e.ReturnType, e.Line())
}

// functionBody parametrelerin tanımlı olduğu yeni bir kapsamda varsayılan değerleri ve
// gövdeyi denetler; gövdedeki 'sonuç'lar retType'a göre denetlenir (açıklama yoksa denetlenmez).
func (c *Checker) functionBody(e *ast.FunctionLiteral, retType string) {
	prev, prevRet, prevFunc := c.scope, c.returnType, c.facts.current
	c.scope = NewScope(prev)
	c.facts.current = e
	c.returnType = ""
	if e.ReturnType != nil {
		c.returnType = retType
	}
	for _, p := range e.Parameters {
		c.defineName(p.Name, kindValue, siteOf(p.NameToken, e.Line()), TypeInfo{TypeName: normalizedType(p.Type), Written: p.Type})
	}
	for _, p := range e.Parameters {
		if p.DefaultValue != nil {
			c.expect(p.Type, p.DefaultValue)
			c.noteDefault(p.DefaultValue, c.inferExpression(p.DefaultValue))
		}
	}
	if e.Body != nil {
		// Gövdenin son ifadesi fonksiyonun değeridir (sondaki blok: liste.filtrele { o > 2 }).
		c.markBlockValue(e.Body, true)
		c.checkStatements(e.Body.Statements)
	}
	c.scope, c.returnType, c.facts.current = prev, prevRet, prevFunc
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

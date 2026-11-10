package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Değer döndürmeyen çağrı (3.5, 6.6, 6.7, 10.1, 13.10). Bir çağrı dört kaynaktan değersiz olur:
// gövdesinde değerli TR_return olmayan fonksiyon (`->` ölçüt değildir: kitaplığın istisnaları `->`
// yazmadan değer döndürür, 6.1), değer vermeyen gövdeli fonksiyon ifadesi ve sondaki blok, tipi
// `(T) -> TR_NoneType` yazılmış değer, tanım dosyasında dönüşü TR_NoneType olan yerleşik. Tipi
// bilinmeyen çağrı (modülün adı, fonksiyon değeri tutan öğe) değersiz sayılmaz: yanlış alarm yoktur.

// hoist fonksiyonu kapsamına kaydeder: ad bulunduğu bloğun her yerinden görünür (6.1).
func (s *Scope) hoist(f *ast.FunctionLiteral) {
	if s.hoisted == nil {
		s.hoisted = map[string]*ast.FunctionLiteral{}
	}
	s.hoisted[f.Name] = f
}

// binding adın en yakın bağıdır: adın tanımı ya da bloğunda henüz görülmemiş fonksiyonu.
func (s *Scope) binding(name string) (TypeInfo, bool) {
	for ; s != nil; s = s.parent {
		if info, ok := s.symbols[name]; ok {
			return info, true
		}
		if f := s.hoisted[name]; f != nil {
			return TypeInfo{Fn: f}, true
		}
	}
	return TypeInfo{}, false
}

// checkStatements bloğun deyimlerini denetler; bloğun fonksiyonları önce kaydedilir.
func (c *Checker) checkStatements(list []ast.Statement) {
	for _, s := range list {
		if es, ok := s.(*ast.ExpressionStatement); ok {
			if f, ok := es.Expression.(*ast.FunctionLiteral); ok && f.Name != "" {
				c.scope.hoist(f)
			}
		}
	}
	for _, s := range list {
		c.checkStatement(s)
	}
}

// noteAssigned programda yeniden atanan adları kaydeder: böyle bir değişkenin fonksiyonu bilinmez.
func (c *Checker) noteAssigned(prog *ast.Program) {
	if c.reassigned == nil {
		c.reassigned = map[string]bool{}
	}
	note := func(e ast.Expression) {
		if id, ok := e.(*ast.Identifier); ok {
			c.reassigned[id.Value] = true
		}
	}
	ast.Walk(prog, func(n any) bool {
		switch s := n.(type) {
		case *ast.AssignStatement:
			note(s.Target)
		case *ast.MultiAssignStatement:
			for _, t := range s.Targets {
				note(t)
			}
		}
		return true
	})
}

// noValueType t, dönüşü TR_NoneType yazılmış bir fonksiyon tipi mi (6.9).
func noValueType(t *ast.TypeRef) bool {
	return t != nil && !t.Optional && t.Return != nil && t.Return.Token.Code == codes.TypeNoneType && !t.Return.Optional
}

// boundFunction değişkenin ilk değeri fonksiyon ifadesiyse onu verir (yeniden atanırsa bilinmez).
func boundFunction(e ast.Expression) *ast.FunctionLiteral {
	f, _ := e.(*ast.FunctionLiteral)
	return f
}

// madeType e, tipi denetleyicinin çekirdek tipleri dışındaki bir yerleşik değeri kuran çağrıysa o
// tiptir (TR_Progress(3)): alıcının metodu tipe göre ayrışır (TR_update, 13.10, 20.3).
func (c *Checker) madeType(e ast.Expression) string {
	call, ok := e.(*ast.CallExpression)
	if !ok {
		return ""
	}
	if ret, ok := SingleReturn(c.builtinCallCode(call, c.receivers[call])); ok {
		return codes.Name(ret)
	}
	return ""
}

// funcNoValue f değer döndürmeyen bir fonksiyon mu. Fonksiyon ifadesinin ve sondaki bloğun gövdesi
// denetlenmiş olmalıdır; TR_func'ta yalnız yazım okunur, ileri başvuruda da bilinir.
func (c *Checker) funcNoValue(f *ast.FunctionLiteral) bool {
	if f.Token.Type == lexer.TokLBrace {
		return c.blockNoValue(f.Body)
	}
	if e := arrowBody(f); e != nil {
		return c.exprNoValue(e)
	}
	return f.ReturnType == nil && !hasValueReturn(f.Body)
}

// arrowBody fonksiyon ifadesinin ifade gövdesidir; ayrıştırıcı onu '=>' belirteçli bir TR_return
// olarak kurar. Öbür fonksiyonlarda nil.
func arrowBody(f *ast.FunctionLiteral) ast.Expression {
	if f.Body == nil || len(f.Body.Statements) != 1 {
		return nil
	}
	if r, ok := f.Body.Statements[0].(*ast.ReturnStatement); ok && r.Token.Type == lexer.TokArrow {
		return r.ReturnValue
	}
	return nil
}

// bareReturn TR_return değersiz mi: ayrıştırıcı onu kendi belirteciyle bir TR_none olarak kurar.
func bareReturn(s *ast.ReturnStatement) bool {
	null, bare := s.ReturnValue.(*ast.NullLiteral)
	return s.ReturnValue == nil || bare && null.Token == s.Token
}

// hasValueReturn gövdede değerli bir TR_return var mı; iç içe fonksiyonun TR_return'ü sayılmaz.
func hasValueReturn(body *ast.BlockStatement) bool {
	found := false
	ast.Walk(body, func(n any) bool {
		switch s := n.(type) {
		case *ast.FunctionLiteral:
			return false
		case *ast.ReturnStatement:
			found = found || !bareReturn(s)
		}
		return !found
	})
	return found
}

// blockNoValue gövdenin ya da dalın değer vermediğini söyler (3.5): son deyimi değer vermeyen ifade,
// değersiz TR_return ya da ifade olmayan bir deyimdir. TR_throw ya da değerli TR_return ile biten dal
// sayılmaz; bütün dalları böyle biten iç TR_if de sayılmaz (exprNoValue).
func (c *Checker) blockNoValue(b *ast.BlockStatement) bool {
	if b == nil || len(b.Statements) == 0 {
		return true
	}
	switch s := b.Statements[len(b.Statements)-1].(type) {
	case *ast.ExpressionStatement:
		return c.exprNoValue(s.Expression)
	case *ast.ReturnStatement:
		return bareReturn(s)
	case *ast.ThrowStatement, *ast.BreakStatement, *ast.ContinueStatement:
		return false
	case *ast.TryCatchStatement:
		return c.blockNoValue(s.TryBody) || c.blockNoValue(s.CatchBody)
	}
	return true
}

// exprNoValue ifade değer vermiyor mu: değersiz çağrı, TR_else'i olmayan ya da dallarından biri
// değer vermeyen TR_if, TR_match ve TR_try.
func (c *Checker) exprNoValue(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.CallExpression:
		return c.noValue[e]
	case *ast.IfExpression:
		if e.Alternative == nil || c.blockNoValue(e.Consequence) || c.blockNoValue(e.Alternative) {
			return true
		}
		for _, elif := range e.ElseIfs {
			if c.blockNoValue(elif.Consequence) {
				return true
			}
		}
	case *ast.MatchExpression:
		for _, mc := range e.Cases {
			if mc.Body != nil && c.blockNoValue(mc.Body) || mc.ValueExpr != nil && c.exprNoValue(mc.ValueExpr) {
				return true
			}
		}
	case *ast.TryExpression:
		return c.blockNoValue(e.Statement.TryBody) || c.blockNoValue(e.Statement.CatchBody)
	}
	return false
}

// valueThrough bloğun değerini geçiren yerleşiklerdir: blok değer vermiyorsa çağrı da vermez (3.5).
var valueThrough = map[codes.Code]bool{codes.FnNoGrad: true, codes.FnMixedPrecision: true, codes.FnQuantizedActivations: true}

// callNoValue çağrının değer vermediğini söyler; objType metodun nesnesinin tipidir.
func (c *Checker) callNoValue(e *ast.CallExpression, objType string) bool {
	switch f := e.Function.(type) {
	case *ast.FunctionLiteral:
		return c.funcNoValue(f)
	case *ast.Identifier:
		if info, user := c.scope.binding(f.Value); user {
			return c.bindingNoValue(f.Value, info)
		}
		if valueThrough[f.Token.Code] && len(e.Arguments) > 0 {
			return c.functionArgNoValue(e.Arguments[len(e.Arguments)-1])
		}
		if f.Token.Code == codes.FnWaitAll {
			return len(e.Arguments) == 0 // TR_wait_all(görevler) sonuçları verir (10.5)
		}
		return builtinNoValue(f.Token.Code, typeAny)
	case *ast.MemberExpression:
		return c.memberNoValue(e, f, objType)
	}
	return false
}

// bindingNoValue adın çağrısı değer vermiyor mu: tipi yazılmış değer ya da bilinen fonksiyon.
func (c *Checker) bindingNoValue(name string, info TypeInfo) bool {
	if noValueType(info.Written) {
		return true
	}
	return info.Fn != nil && !c.reassigned[name] && c.funcNoValue(info.Fn)
}

// memberNoValue metot çağrısı değer vermiyor mu: yapının metodu ya da fonksiyon tipli alanı, değeri
// olmayan görevin TR_wait'i (10.1), yerleşik metot.
func (c *Checker) memberNoValue(e *ast.CallExpression, m *ast.MemberExpression, objType string) bool {
	if s := c.structs[objType]; s != nil && !m.Safe {
		if f := s.Methods[m.Property]; f != nil {
			return c.funcNoValue(f)
		}
		for _, fl := range s.Fields {
			if fl.Name == m.Property {
				return noValueType(fl.Type)
			}
		}
		return false
	}
	if t := shapeOf(objType); c.builtinCallCode(e, objType) == codes.MemWait && t.base == typeTask && len(t.args) == 1 {
		return t.args[0] == typeNull
	}
	if id, ok := m.Object.(*ast.Identifier); ok && objType == typeAny {
		if info, found := c.scope.Resolve(id.Value); found && info.Made != "" {
			objType = info.Made
		}
	}
	return builtinNoValue(c.builtinCallCode(e, objType), objType)
}

// builtinNoValue yerleşiğin dönüşü tanım dosyasında TR_NoneType mı (13.10). `returns` yazılmamış
// tanım değer döndürmez demek değildir. Alıcısı bilinmeyen metot bilinmez: kitaplığın yapısı aynı
// adla değer döndürebilir (lib/layers.tr, TR_cache.TR_append).
func builtinNoValue(k codes.Code, objType string) bool {
	def, ok := codes.DefinitionOf(k)
	if !ok || k.IsLibrary() {
		return false
	}
	if def.Kind != codes.KindMethod {
		return noneType(def.Signature.Return)
	}
	for _, u := range def.Members {
		if codes.Name(u.Type) == baseType(objType) {
			return noneType(u.Signature.Return)
		}
	}
	return false
}

func noneType(t codes.TypeExpr) bool {
	return len(t.Types) == 1 && t.Types[0] == codes.TypeNoneType && !t.Optional
}

// functionArgNoValue argüman olarak verilen fonksiyon değer döndürmüyor mu: fonksiyon ifadesi ya da
// sondaki blok, adıyla verilen fonksiyon, yerleşik.
func (c *Checker) functionArgNoValue(arg ast.Expression) bool {
	switch a := arg.(type) {
	case *ast.FunctionLiteral:
		return c.funcNoValue(a)
	case *ast.Identifier:
		if info, user := c.scope.binding(a.Value); user {
			return c.bindingNoValue(a.Value, info)
		}
		return builtinNoValue(a.Token.Code, typeAny)
	}
	return false
}

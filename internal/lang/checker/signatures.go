package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tipli yazımın kapısı (1.2, 6.1, 6.6, 7.1): her parametre ve her alan tipini yazar, değer döndüren
// TR_func dönüş tipini '->' ile yazar. Tipsiz yer denetim hatasıdır ve ileti çalışan yazımı verir:
// tip varsayılandan (H1109) ya da dönüşlerin ortak tipinden (H1112) çıkar, düzeltme uygulanabilir;
// dosyadaki çağrılar aynı tipi veriyorsa o tip tahmin olarak önerilir, düzeltme uygulanamaz (H1110);
// tip bulunamazsa ileti yazım biçimini ve nedenini söyler (H1111, H1113), tip uydurulmaz. Hatalar
// program bütünüyle denetlendikten sonra verilir: çağrılar ve dönüşler ancak o zaman bilinir.
//
// Fonksiyon ifadesinin parametresi tipini beklenen fonksiyon tipinden alır (6.6). Denetleyici
// yerleşik ve kitaplık imzalarını henüz bilmediği için (13.9, 4a2) her çağrı argümanı, tipli
// değişken, alan ve dönüş beklenen tipi olan yer sayılır; hata yalnız tipi açıklanmamış bir
// değişkenin ilk değerinde (doğrudan ya da liste, sözlük, demet değişmezinin içinde) verilir:
// orada beklenen tip yoktur.

// callArg dosyadaki bir çağrının bir argümanıdır: index sırası (adlı argümanda -1), name adı.
type callArg struct {
	index int
	name  string
	expr  ast.Expression
	typ   string
}

// returned bir TR_return'dür: expr nil ise değersizdir.
type returned struct {
	expr ast.Expression
	typ  string
}

// signatureFacts kapının denetim boyunca topladıklarıdır: çağrıların argümanları (çağrılan adın
// anahtarıyla: fonksiyon, yapı, Yapı.metot), fonksiyonların dönüşleri, varsayılanların tipleri ve
// o an gövdesi denetlenen fonksiyon.
type signatureFacts struct {
	calls    map[string][]callArg
	returns  map[*ast.FunctionLiteral][]returned
	defaults map[ast.Expression]string
	current  *ast.FunctionLiteral
}

// noteCall çağrının argümanlarını kaydeder; anahtarı olmayan çağrı (tipi bilinmeyen nesnenin
// metodu, fonksiyon değeri) kaydedilmez.
func (c *Checker) noteCall(e *ast.CallExpression, objType string, i int, name string, arg ast.Expression, typ string) {
	key := ""
	switch f := e.Function.(type) {
	case *ast.Identifier:
		key = f.Value
	case *ast.MemberExpression:
		if c.knownStruct(objType) && !f.Safe {
			key = objType + "." + f.Property
		}
	}
	if key == "" {
		return
	}
	if c.facts.calls == nil {
		c.facts.calls = map[string][]callArg{}
	}
	c.facts.calls[key] = append(c.facts.calls[key], callArg{index: i, name: name, expr: arg, typ: typ})
}

// noteReturn o an denetlenen fonksiyonun bir TR_return'ünü kaydeder. Ayrıştırıcı değersiz
// TR_return'ü kendi belirteciyle bir TR_none olarak kurar.
func (c *Checker) noteReturn(s *ast.ReturnStatement, typ string) {
	if c.facts.current == nil {
		return
	}
	r := returned{expr: s.ReturnValue, typ: typ}
	if null, bare := s.ReturnValue.(*ast.NullLiteral); s.ReturnValue == nil || bare && null.Token == s.Token {
		r.expr = nil
	}
	if c.facts.returns == nil {
		c.facts.returns = map[*ast.FunctionLiteral][]returned{}
	}
	c.facts.returns[c.facts.current] = append(c.facts.returns[c.facts.current], r)
}

// noteDefault parametrenin ya da alanın varsayılanının tipini kaydeder.
func (c *Checker) noteDefault(value ast.Expression, typ string) {
	if c.facts.defaults == nil {
		c.facts.defaults = map[ast.Expression]string{}
	}
	c.facts.defaults[value] = typ
}

// checkSignatures programın bütün tipsiz parametrelerini, alanlarını ve dönüşlerini bildirir.
func (c *Checker) checkSignatures(prog *ast.Program) {
	owners := map[*ast.FunctionLiteral]string{}
	open := untypedArrows(prog)
	ast.Walk(prog, func(n any) bool {
		switch n := n.(type) {
		case *ast.StructStatement:
			c.checkStructSignature(n, owners)
		case *ast.FunctionLiteral:
			if n.Token.Type == lexer.TokFunction {
				owner := owners[n]
				if owner == "" {
					owner = n.Name
				}
				c.checkFunctionSignature(n, owner, owner)
			} else if open[n] {
				c.checkParams(n, "", "", placeArrow)
			}
		}
		return true
	})
}

// checkStructSignature yapının ve durumlu tipin alanlarını denetler, metotlarının sahibini
// (Yapı.metot) kaydeder. Kurucusuz yapının alanları kurma çağrılarından tahmin edilir; kurucunun
// parametreleri de yapının adıyla yapılan çağrılardan.
func (c *Checker) checkStructSignature(s *ast.StructStatement, owners map[*ast.FunctionLiteral]string) {
	init := codes.Name(codes.CtxInit)
	for name, m := range s.Methods {
		owners[m] = s.Name + "." + name
	}
	fieldCalls := s.Name
	if s.Methods[init] != nil || s.Kind != ast.KindStruct {
		fieldCalls = ""
	}
	for i, f := range s.Fields {
		if f.Type != nil {
			continue
		}
		c.checkUntyped(f.NameToken, s.Name+"."+f.Name, f.Name, placeField, f.DefaultValue, c.argsFor(fieldCalls, i, f.Name))
	}
	for _, v := range s.Variants {
		for _, f := range v.Fields {
			if f.Type != nil {
				continue
			}
			c.checkUntyped(f.NameToken, s.Name+"."+v.Name+"."+f.Name, f.Name, placeField, f.DefaultValue, nil)
		}
	}
}

// checkFunctionSignature TR_func'ın parametrelerini ve dönüşünü denetler; owner istisnanın ve
// çağrıların anahtarıdır (fonksiyonun adı ya da Yapı.metot). Kurucunun parametreleri yapının
// adıyla yapılan çağrılardan tahmin edilir; kurucu değer döndürmez (7.4).
func (c *Checker) checkFunctionSignature(f *ast.FunctionLiteral, owner, calls string) {
	if s, name, ok := cutLast(owner); ok && name == codes.Name(codes.CtxInit) {
		c.checkParams(f, owner, s, placeParam)
		return
	}
	c.checkParams(f, owner, calls, placeParam)
	if f.ReturnType == nil {
		c.checkReturnType(f, owner)
	}
}

// checkParams f'nin tipsiz parametrelerini bildirir; `_` okunmadığı için tipsiz kalabilir (6.6).
func (c *Checker) checkParams(f *ast.FunctionLiteral, owner, calls, place string) {
	for i, p := range f.Parameters {
		if p.Type != nil || p.NameToken.Line == 0 || p.Name == "_" {
			continue
		}
		c.checkUntyped(p.NameToken, owner+"."+p.Name, p.Name, place, p.DefaultValue, c.argsFor(calls, i, p.Name))
	}
}

// Tipsiz yerin türü, iletinin seçim anahtarıdır.
const (
	placeParam = "param"
	placeField = "field"
	placeArrow = "arrow"
)

// argsFor key'in çağrılarında i. sıradaki ya da name adlı argümanlardır.
func (c *Checker) argsFor(key string, i int, name string) []callArg {
	if key == "" {
		return nil
	}
	var out []callArg
	for _, a := range c.facts.calls[key] {
		if a.index == i || a.index < 0 && a.name == name {
			out = append(out, a)
		}
	}
	return out
}

// checkUntyped tipsiz bir parametreyi ya da alanı bildirir: tip varsayılandan (uygulanabilir
// düzeltme) ya da çağrılardan (tahmin) çıkar; bulunamazsa nedeni söylenir.
func (c *Checker) checkUntyped(tok lexer.Token, key, name, place string, def ast.Expression, args []callArg) {
	if c.excepted(key) {
		return
	}
	if def != nil {
		if t := c.facts.defaults[def]; c.completeType(t) {
			c.reportTyped(tok, place, name, t, 0)
			return
		}
		c.addErrorAtToken(tok, messages.New(messages.H1111, place, name, "default"))
		return
	}
	reason := "none"
	switch {
	case place == placeArrow:
		reason = "arrow"
	case len(args) > 0:
		types, exprs := make([]string, len(args)), make([]ast.Expression, len(args))
		for i, a := range args {
			types[i], exprs[i] = a.typ, a.expr
		}
		if t := c.commonType(types, exprs); c.completeType(t) {
			c.reportTyped(tok, place, name, t, len(args))
			return
		}
		reason = "calls"
	}
	c.addErrorAtToken(tok, messages.New(messages.H1111, place, name, reason))
}

// reportTyped tipi bulunan tipsiz yeri bildirir: düzeltme adı "ad: T" yapar, ileti düzeltilmiş
// satırı (ya da yalnız yazımı) verir. calls 0 ise tip varsayılandandır (H1109, uygulanabilir),
// değilse o kadar çağrıdan tahmindir (H1110, uygulanamaz).
func (c *Checker) reportTyped(tok lexer.Token, place, name, typ string, calls int) {
	fix := replaceToken(tok, name+": "+c.typeSpelled(typ), calls == 0)
	line := c.fixedLine(tok, fix.Replacement)
	err := messages.New(messages.H1109, place, name, c.typeArg(typ), line)
	if calls > 0 {
		err = messages.New(messages.H1110, place, name, c.typeArg(typ), calls, line)
	}
	c.addErrorAtToken(tok, err).WithFix(fix)
}

// checkReturnType değer döndürüp '->' yazmayan fonksiyonu bildirir: tip değerli TR_return'lerin
// ortak tipidir (uygulanabilir düzeltme: gövdenin '{'ı "-> T {" olur); değersiz ya da yokluk
// döndüren yol varsa ya da tipler farklıysa tip bulunamaz.
func (c *Checker) checkReturnType(f *ast.FunctionLiteral, owner string) {
	var types []string
	var exprs []ast.Expression
	bare := false
	for _, r := range c.facts.returns[f] {
		if r.expr == nil || r.typ == typeNull {
			bare = true
			continue
		}
		types, exprs = append(types, r.typ), append(exprs, r.expr)
	}
	if len(types) == 0 || c.excepted(owner+" ->") {
		return
	}
	at := siteOf(f.NameToken, f.Line())
	t := c.commonType(types, exprs)
	if bare || !c.completeType(t) || f.Body == nil {
		reason := "mixed"
		if bare {
			reason = "none"
		}
		c.addErrorAtToken(at, messages.New(messages.H1113, f.Name, c.word(codes.KwReturn), reason))
		return
	}
	fix := replaceToken(f.Body.Token, "-> "+c.typeSpelled(t)+" {", true)
	c.addErrorAtToken(at, messages.New(messages.H1112, f.Name, c.word(codes.KwReturn), c.typeArg(t),
		c.fixedLine(f.Body.Token, fix.Replacement))).WithFix(fix)
}

// completeType t bir tip yazımı olarak eksiksiz mi: denetleyicinin tanıdığı, genel olmayan
// (TR_Any, çıplak TR_Function) ve kapları öğe tipini taşıyan bir tip (3.6, 3.7, 3.11).
func (c *Checker) completeType(t string) bool {
	if t == "" || t == typeAny || t == typeNull || t == typeFunction || !c.knownType(t) {
		return false
	}
	s := shapeOf(t)
	if (s.base == typeList || s.base == typeDict) && len(s.args) == 0 {
		return false
	}
	for _, a := range append(s.args, s.elems...) {
		if !c.completeType(a) {
			return false
		}
	}
	return true
}

// typeSpelled t'nin düzeltmedeki yazımıdır: dosyada kimlik yazımıyla, yazılan satırda satırın
// dilindeki adıyla.
func (c *Checker) typeSpelled(t string) string {
	if c.language() == nil {
		return typeSpelling(t).MessageText(messages.ActiveLanguage())
	}
	return typeText(t).MessageText(c.fileLanguage)
}

// fixedLine tok'un satırının düzeltilmiş hâlidir (tok'un yerine replacement); satır bilinmiyorsa
// yalnız replacement.
func (c *Checker) fixedLine(tok lexer.Token, replacement string) string {
	r := tok.Range()
	if r.Line < 1 || r.Line > len(c.fileLines) || r.EndLine != r.Line || r.Col < 1 {
		return replacement
	}
	line := []rune(c.fileLines[r.Line-1])
	if r.EndCol-1 > len(line) {
		return replacement
	}
	return strings.TrimSpace(string(line[:r.Col-1]) + replacement + string(line[r.EndCol-1:]))
}

// untypedArrows tipi açıklanmamış değişkenlerin ilk değerindeki fonksiyon ifadeleridir (doğrudan
// ya da liste, sözlük, demet değişmezinin öğesi): orada beklenen fonksiyon tipi yoktur (6.6).
func untypedArrows(prog *ast.Program) map[*ast.FunctionLiteral]bool {
	out := map[*ast.FunctionLiteral]bool{}
	var add func(e ast.Expression)
	add = func(e ast.Expression) {
		switch e := e.(type) {
		case *ast.FunctionLiteral:
			if e.Token.Type != lexer.TokFunction {
				out[e] = true
			}
		case *ast.ListLiteral:
			for _, el := range e.Elements {
				add(el)
			}
		case *ast.TupleExpression:
			for _, el := range e.Elements {
				add(el)
			}
		case *ast.DictLiteral:
			for _, p := range e.Pairs {
				add(p.Value)
			}
		}
	}
	ast.Walk(prog, func(n any) bool {
		switch s := n.(type) {
		case *ast.VarStatement:
			if s.Type == nil {
				add(s.Value)
			}
		case *ast.MultiVarStatement:
			for _, v := range s.Values {
				add(v)
			}
		}
		return true
	})
	return out
}

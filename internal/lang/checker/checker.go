// Package checker is TR's type checker that runs before execution: it walks the syntax tree
// and finds type mismatches (declaration, assignment, return value, call argument),
// assignments to constants, undefined names and shadowed builtin names; it reports errors
// and warnings with their lines ('tr kontrol', and before running or loading a module).
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package checker

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"strings"
	"unicode"
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
	errors []string
	// uyarilar programın çalışmasını engellemeyen bulgulardır (ör. yerleşik ad gölgeleme).
	uyarilar []string
	// İçinde bulunulan fonksiyonun bildirilmiş dönüş tipi ("" = fonksiyon dışı veya tipsiz)
	returnType string
	// genelAdlar tanımsız ad denetiminin genel kapsamıdır (kabukta satırlar arasında kalır);
	// adHatalari son CheckProgram çağrısının tanımsız ad hatalarıdır (undefined_names.go).
	genelAdlar *adKapsami
	adHatalari []string
}

// New returns a checker with an empty global scope.
func New() *Checker {
	return &Checker{
		scope:  NewScope(nil),
		errors: []string{},
	}
}

// Errors returns every error found so far.
func (c *Checker) Errors() []string {
	return c.errors
}

// Warnings returns the checker's warnings. Warnings are not errors: the program runs and
// the exit code is unaffected; the command line prints them under a separate heading.
func (c *Checker) Warnings() []string {
	return c.uyarilar
}

// golgeDenetle ad bir yerleşik fonksiyonun adıysa (sözlükte G/T/V/W yerleşik kodu) uyarı
// ekler: kullanıcı tanımı o kapsamda yerleşiği gölgeler; yerleşik orada artık çağrılamaz.
func (c *Checker) golgeDenetle(ad string, satir int) {
	if k, ok := codes.Find(codes.NamespaceGlobal, ad); ok && k.IsBuiltin() {
		c.uyarilar = append(c.uyarilar, fmt.Sprintf(
			"Satır %d: '%s' yerleşik bir ad; bu kapsamda yerleşik '%s' artık kullanılamaz", satir, ad, ad))
	}
}

// Denetleyicinin kullandığı tip adları kod listesindedir (internal/lang/codes, A2xx).
var (
	tipTam       = codes.Name(codes.A200)
	tipOndalik   = codes.Name(codes.A201)
	tipMetin     = codes.Name(codes.A202)
	tipMantik    = codes.Name(codes.A203)
	tipYok       = codes.Name(codes.A204)
	tipListe     = codes.Name(codes.A205)
	tipSozluk    = codes.Name(codes.A206)
	tipAralik    = codes.Name(codes.A207)
	tipFonksiyon = codes.Name(codes.A208)
	tipTensor    = codes.Name(codes.A212)
	tipHerhangi  = codes.Name(codes.A213)
)

// tipSayi denetleyicinin iç tipidir: değer çalışma anında Tam da Ondalık da olabilir (ör.
// Tam / Tam: 6 / 3 = 2, 7 / 2 = 3.5). Kullanıcı açıklamasında yazılamaz; yalnızca Ondalık
// (Tam da Ondalığa atanabildiği için) ve tipsiz yerlere atanabilir.
var tipSayi = tipTam + " ya da " + tipOndalik

// aciklamaTipleri tip açıklamasında (değişken x: Tam) kullanılabilen tiplerin kodlarıdır:
// 3.11'deki çekirdek tip adları.
var aciklamaTipleri = []codes.Code{codes.A200, codes.A201, codes.A202, codes.A203,
	codes.A205, codes.A206, codes.A207, codes.A208, codes.A209, codes.A212, codes.A204, codes.A215}

// tipYazimlari tip açıklamasında kabul edilen yazımlar → tip adıdır. Yazımlar sözlükten
// gelir ve tektir (3.14): yalnızca tip adının tam yazımı (Tam, Ondalık); büyük/küçük harfi
// farklı bir yazım (tam, MANTIK) kabul edilmez. Matris 2 boyutlu tensördür; ayrı bir tip
// değildir.
var tipYazimlari = func() map[string]string {
	m := map[string]string{}
	for _, k := range aciklamaTipleri {
		for _, y := range codes.Spellings(k) {
			m[y] = codes.Name(k)
		}
	}
	return m
}()

// tipHarfYazimlari kabul edilen yazımların Türkçe kuralla küçültülmüş biçimi → tip adıdır.
// Yalnızca ipucu içindir: yazım bir tip adından yalnızca büyük/küçük harfle ayrılıyorsa
// (tam, ONDALIK) denetleyici asıl yazımı söyler.
var tipHarfYazimlari = func() map[string]string {
	m := map[string]string{}
	for y, ad := range tipYazimlari {
		m[kucukHarf(y)] = ad
	}
	return m
}()

// tipOnerileri kabul edilmeyen ama bir tip adına işaret eden yazımların (Türkçe
// karaktersiz biçim: ondalik; kaldırılmış adlar) küçük harfli biçimi → önerilecek tip
// adıdır. Bu yazımlar tip açıklamasında hata verir ve asıl adı önerir.
var tipOnerileri = func() map[string]string {
	m := map[string]string{}
	ekle := func(y, ad string) {
		if _, gecerli := tipHarfYazimlari[kucukHarf(y)]; !gecerli {
			m[kucukHarf(y)] = ad
		}
	}
	for _, k := range aciklamaTipleri {
		g, _ := codes.Active().Entry(k)
		ekle(turkish.ASCII(g.Name), g.Name)
		for _, e := range g.Old {
			ekle(e, g.Name)
			ekle(turkish.ASCII(e), g.Name)
		}
	}
	return m
}()

// kucukHarf tip adlarını Türkçe kurala göre küçültür (MANTIK → mantık, METİN → metin);
// yalnızca kabul edilmeyen bir yazımın ipucunu bulmak için kullanılır.
func kucukHarf(s string) string { return strings.ToLowerSpecial(unicode.TurkishCase, s) }

// tipAdiDenetle tip açıklamasındaki ad bir tip adının kabul edilmeyen bir yazımıysa
// (tam, ondalik, int) asıl adı öneren bir hata ekler.
func (c *Checker) tipAdiDenetle(ad string, satir int) {
	if ad == "" {
		return
	}
	if _, gecerli := tipYazimlari[ad]; gecerli {
		return
	}
	if asil, ok := tipHarfYazimlari[kucukHarf(ad)]; ok {
		c.errors = append(c.errors, fmt.Sprintf(
			"Satır %d: '%s' TR'de bir tip adı değil; tip adının tek yazımı vardır: '%s' yazın", satir, ad, asil))
		return
	}
	if oneri, ok := tipOnerileri[kucukHarf(ad)]; ok {
		c.errors = append(c.errors, fmt.Sprintf(
			"Satır %d: '%s' TR'de bir tip adı değil; '%s' yazmak mı istediniz?", satir, ad, oneri))
	}
}

func normalizeTypeName(t string) string {
	if t == "" {
		return tipHerhangi
	}
	if ad, ok := tipYazimlari[t]; ok {
		return ad
	}
	// Kabul edilmeyen yazım (tam, ondalik) tipAdiDenetle'de hata verir; aynı satırda bir de
	// tip uyuşmazlığı çıkmasın diye önerilen tip varsayılır.
	if ad, ok := tipHarfYazimlari[kucukHarf(t)]; ok {
		return ad
	}
	if ad, ok := tipOnerileri[kucukHarf(t)]; ok {
		return ad
	}
	return t
}

func typesCompatible(expected, actual string) bool {
	exp := normalizeTypeName(expected)
	act := normalizeTypeName(actual)
	if exp == tipHerhangi || act == tipHerhangi || act == tipYok {
		return true
	}
	// Tam sayı (ve Tam ya da Ondalık olabilen değer), Ondalık beklenen yere atanabilir
	if exp == tipOndalik && (act == tipTam || act == tipSayi) {
		return true
	}
	return exp == act
}

// CheckProgram checks prog and returns the errors it found.
func (c *Checker) CheckProgram(prog *ast.Program) []string {
	bas := len(c.errors)
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	// Tanımsız adlar (5.14) tip hatalarıyla birlikte, satır sırasıyla bildirilir.
	c.adHatalari = c.adHatalari[:0]
	c.adlariDenetle(prog)
	c.errors = append(c.errors, c.adHatalari...)
	satiraGoreSirala(c.errors[bas:])
	return c.errors
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.VarStatement:
		c.golgeDenetle(s.Name.Value, s.Line())
		c.tipAdiDenetle(s.TypeName, s.Line())
		inferred := c.inferExpression(s.Value)
		declared := normalizeTypeName(s.TypeName)
		if s.TypeName != "" && !typesCompatible(declared, inferred) {
			c.errors = append(c.errors, fmt.Sprintf(
				"Satır %d: Tip uyuşmazlığı — %q değişkeni %q tipinde tanımlandı ancak %q tipinde değer atandı%s",
				s.Line(), s.Name.Value, declared, inferred, sayiIpucu(declared, inferred),
			))
		}
		// Dil dinamiktir: açıklamasız bir değişkenin ilk değerden çıkarılan tipi kalıcı bir
		// bildirim değildir (değişken sonra başka tipte değer alabilir), bu yüzden tipsiz
		// sayılır. Sabitler yeniden atanamadığı için çıkarılan tipleri kalır.
		finalType := tipHerhangi
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
		valType := c.inferExpression(s.Value)
		if ident, ok := s.Target.(*ast.Identifier); ok {
			if info, found := c.scope.Resolve(ident.Value); found {
				// Bileşik atamada (x += y) atanan değer 'x + y' ifadesinin sonucudur
				if op := strings.TrimSuffix(s.Operator, "="); op != "" {
					valType = arithmeticType(op, info.TypeName, valType, false)
				}
				if info.IsConst {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Sabit hatası — %q bir sabittir ('%s') ve değeri değiştirilemez",
						s.Line(), ident.Value, codes.Name(codes.A031),
					))
				}
				if info.TypeName != tipHerhangi && info.TypeName != tipYok && !typesCompatible(info.TypeName, valType) {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Tip uyuşmazlığı — %q değişkeni (%s) içine %q tipinde değer atanamaz%s",
						s.Line(), ident.Value, info.TypeName, valType, sayiIpucu(info.TypeName, valType),
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
			c.golgeDenetle(s.KeyVar, s.Line())
			c.scope.Define(s.KeyVar, TypeInfo{TypeName: tipHerhangi})
		}
		c.golgeDenetle(s.ValueVar, s.Line())
		c.scope.Define(s.ValueVar, TypeInfo{TypeName: tipHerhangi})
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
		yakalaSatiri := s.Line()
		if s.CatchBody != nil {
			yakalaSatiri = s.CatchBody.Line()
		}
		c.golgeDenetle(s.ErrorVar, yakalaSatiri)
		// Yakalanan hata Metin'dir, ama değişken açıklamasızdır: sonra başka tipte değer alabilir.
		c.scope.Define(s.ErrorVar, TypeInfo{TypeName: tipHerhangi})
		for _, bs := range s.CatchBody.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *ast.StructStatement:
		c.golgeDenetle(s.Name, s.Line())
		c.scope.Define(s.Name, TypeInfo{TypeName: s.Name})

	case *ast.MultiVarStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, name := range s.Names {
			c.golgeDenetle(name.Value, s.Line())
			c.scope.Define(name.Value, TypeInfo{TypeName: tipHerhangi, IsConst: s.IsConst})
		}

	case *ast.MultiAssignStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, t := range s.Targets {
			if ident, ok := t.(*ast.Identifier); ok {
				if info, found := c.scope.Resolve(ident.Value); found && info.IsConst {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Sabit hatası — %q bir sabittir ('%s') ve değeri değiştirilemez",
						s.Line(), ident.Value, codes.Name(codes.A031),
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

	case *ast.ReturnStatement:
		valType := c.inferExpression(s.ReturnValue)
		if c.returnType != "" && !typesCompatible(c.returnType, valType) {
			c.errors = append(c.errors, fmt.Sprintf(
				"Satır %d: Dönüş tipi uyuşmazlığı — fonksiyon %q döndürmeli ancak %q tipinde değer döndürülüyor%s",
				s.Line(), c.returnType, valType, sayiIpucu(c.returnType, valType),
			))
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
	}
}

// sayiIpucu Tam beklenen yere Tam ya da Ondalık olabilen bir değer verildiğinde yol gösterir.
func sayiIpucu(beklenen, verilen string) string {
	if normalizeTypeName(beklenen) == tipTam && verilen == tipSayi {
		return fmt.Sprintf("; '/' bölüm tam çıkmazsa, '**' üs negatifse %s verir: %s için %s(...) kullanın ya da tipi %s yapın",
			tipOndalik, tipTam, codes.Name(codes.G210), tipOndalik)
	}
	return ""
}

// sayisalTip tip bir sayı tipi mi (Tam, Ondalık ya da ikisinden biri)?
func sayisalTip(t string) bool {
	return t == tipTam || t == tipOndalik || t == tipSayi
}

// arithmeticType bir aritmetik işlemin sonuç tipini çalışma zamanının kurallarıyla çıkarır
// (4.2, 4.3, 4.10): denetleyici çalışma zamanından farklı bir tip iddia etmez. Tam / Tam
// tam bölünmezse Ondalık verir, Tam ** Tam üs negatifse Ondalık verir; bu yüzden ikisi de
// "Tam ya da Ondalık"tır. tamUs, '**'in sağının negatif olmayan bir Tam sabiti olduğunu
// söyler (2 ** 3 her zaman Tam'dır).
func arithmeticType(op, left, right string, tamUs bool) string {
	if op == "+" && (left == tipMetin || right == tipMetin) {
		return tipMetin
	}
	if !sayisalTip(left) || !sayisalTip(right) {
		return tipHerhangi
	}
	if left == tipOndalik || right == tipOndalik {
		return tipOndalik
	}
	if op == "/" || (op == "**" && !(tamUs && left == tipTam)) {
		return tipSayi
	}
	if left == tipSayi || right == tipSayi {
		return tipSayi
	}
	return tipTam
}

func (c *Checker) inferExpression(expr ast.Expression) string {
	if expr == nil {
		return tipYok
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return tipTam
	case *ast.FloatLiteral:
		return tipOndalik
	case *ast.StringLiteral, *ast.InterpolatedString:
		return tipMetin
	case *ast.BooleanLiteral:
		return tipMantik
	case *ast.NullLiteral:
		return tipYok
	case *ast.ListLiteral, *ast.ListComprehension:
		return tipListe
	case *ast.DictLiteral:
		return tipSozluk
	case *ast.RangeExpression:
		return tipAralik
	case *ast.Identifier:
		if info, ok := c.scope.Resolve(e.Value); ok {
			return info.TypeName
		}
		return tipHerhangi
	case *ast.PrefixExpression:
		right := c.inferExpression(e.Right)
		if e.Operator == "!" {
			return tipMantik
		}
		return right
	case *ast.InfixExpression:
		left := c.inferExpression(e.Left)
		right := c.inferExpression(e.Right)
		c.tipMetinDenetle(e)
		switch e.Operator {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return tipMantik
		case "??":
			if left != tipYok && left != tipHerhangi {
				return left
			}
			return right
		case "|>":
			return tipHerhangi
		case "@":
			return tipTensor
		case "+", "-", "*", "/", "%", "**":
			lit, tamSabit := e.Right.(*ast.IntegerLiteral)
			return arithmeticType(e.Operator, left, right, tamSabit && lit.Value >= 0)
		}
		return tipHerhangi
	case *ast.FunctionLiteral:
		var paramTypes []string
		for _, p := range e.Parameters {
			c.tipAdiDenetle(p.TypeName, e.Line())
			paramTypes = append(paramTypes, normalizeTypeName(p.TypeName))
		}
		c.tipAdiDenetle(e.ReturnType, e.Line())
		retType := normalizeTypeName(e.ReturnType)
		if e.Name != "" {
			c.golgeDenetle(e.Name, e.Line())
			c.scope.Define(e.Name, TypeInfo{
				TypeName:   tipFonksiyon,
				ParamTypes: paramTypes,
				ReturnType: retType,
			})
		}
		prev, prevRet := c.scope, c.returnType
		c.scope = NewScope(prev)
		c.returnType = ""
		if e.ReturnType != "" {
			c.returnType = retType
		}
		for _, p := range e.Parameters {
			c.golgeDenetle(p.Name, e.Line())
			c.scope.Define(p.Name, TypeInfo{TypeName: normalizeTypeName(p.TypeName)})
		}
		for _, stmt := range e.Body.Statements {
			c.checkStatement(stmt)
		}
		c.scope, c.returnType = prev, prevRet
		return tipFonksiyon
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
				c.errors = append(c.errors, fmt.Sprintf(
					"Satır %d: Fonksiyon çağrısı tip hatası — %q fonksiyonunun %d. parametresi %q bekliyor ancak %q verildi",
					e.Line(), ident.Value, i+1, info.ParamTypes[i], argType,
				))
			}
		}
		for _, na := range e.NamedArgs {
			c.inferExpression(na.Value)
		}
		if found && info.TypeName == tipFonksiyon && info.ReturnType != "" {
			return info.ReturnType
		}
		return tipHerhangi
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
		return tipHerhangi
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
		return tipHerhangi
	}
	return tipHerhangi
}

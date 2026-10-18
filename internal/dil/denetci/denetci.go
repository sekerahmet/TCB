// Package denetci TR'nin çalıştırmadan önceki tip denetleyicisidir: sözdizimi ağacını
// dolaşıp tip uyuşmazlıklarını (tanım, atama, dönüş değeri, çağrı argümanı), sabite
// atamayı ve yerleşik adların gölgelenmesini bulur; hataları ve uyarıları Türkçe ve
// satırlı olarak bildirir ('tr kontrol', çalıştırma ve modül yükleme öncesi).
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package denetci

import (
	"fmt"
	"strings"
	"tr_dili/internal/dil/agac"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"unicode"
)

type TypeInfo struct {
	TypeName   string
	IsConst    bool
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
	// uyarilar programın çalışmasını engellemeyen bulgulardır (ör. yerleşik ad gölgeleme).
	uyarilar []string
	// İçinde bulunulan fonksiyonun bildirilmiş dönüş tipi ("" = fonksiyon dışı veya tipsiz)
	returnType string
	// genelAdlar tanımsız ad denetiminin genel kapsamıdır (kabukta satırlar arasında kalır);
	// adHatalari son CheckProgram çağrısının tanımsız ad hatalarıdır (tanimsiz_adlar.go).
	genelAdlar *adKapsami
	adHatalari []string
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

// Warnings denetimin uyarılarıdır. Uyarılar hata değildir: program çalışır, çıkış kodunu
// etkilemezler; komut satırı onları hatalardan ayrı bir başlıkla yazar.
func (c *Checker) Warnings() []string {
	return c.uyarilar
}

// golgeDenetle ad bir yerleşik fonksiyonun adıysa (sözlükte G/T/V/W yerleşik kodu) uyarı
// ekler: kullanıcı tanımı o kapsamda yerleşiği gölgeler; yerleşik orada artık çağrılamaz.
func (c *Checker) golgeDenetle(ad string, satir int) {
	if k, ok := kod.Bul(kod.AlanGenel, ad); ok && k.Yerlesik() {
		c.uyarilar = append(c.uyarilar, fmt.Sprintf(
			"Satır %d: '%s' yerleşik bir ad; bu kapsamda yerleşik '%s' artık kullanılamaz", satir, ad, ad))
	}
}

// Denetleyicinin kullandığı tip adları kod listesindedir (internal/dil/kod, A2xx).
var (
	tipTam       = kod.Ad(kod.A200)
	tipOndalik   = kod.Ad(kod.A201)
	tipMetin     = kod.Ad(kod.A202)
	tipMantik    = kod.Ad(kod.A203)
	tipYok       = kod.Ad(kod.A204)
	tipListe     = kod.Ad(kod.A205)
	tipSozluk    = kod.Ad(kod.A206)
	tipAralik    = kod.Ad(kod.A207)
	tipFonksiyon = kod.Ad(kod.A208)
	tipTensor    = kod.Ad(kod.A212)
	tipHerhangi  = kod.Ad(kod.A213)
)

// tipSayi denetleyicinin iç tipidir: değer çalışma anında Tam da Ondalık da olabilir (ör.
// Tam / Tam: 6 / 3 = 2, 7 / 2 = 3.5). Kullanıcı açıklamasında yazılamaz; yalnızca Ondalık
// (Tam da Ondalığa atanabildiği için) ve tipsiz yerlere atanabilir.
const tipSayi = "Tam ya da Ondalık"

// aciklamaTipleri tip açıklamasında (değişken x: Tam) kullanılabilen tiplerin kodlarıdır:
// 3.11'deki çekirdek tip adları.
var aciklamaTipleri = []kod.Kod{kod.A200, kod.A201, kod.A202, kod.A203,
	kod.A205, kod.A206, kod.A207, kod.A208, kod.A209, kod.A212, kod.A204}

// tipYazimlari tip açıklamasında kabul edilen yazımların küçük harfli biçimi → tip adıdır.
// Yazımlar sözlükten gelir (yalnızca Türkçe biçim; büyük/küçük harf fark etmez). Matris 2
// boyutlu tensördür; ayrı bir tip değildir.
var tipYazimlari = func() map[string]string {
	m := map[string]string{}
	for _, k := range aciklamaTipleri {
		for _, y := range kod.Yazimlar(k) {
			m[kucukHarf(y)] = kod.Ad(k)
		}
	}
	return m
}()

// tipOnerileri kabul edilmeyen ama bir tip adına işaret eden yazımların (Türkçe
// karaktersiz biçim: ondalik; kaldırılmış adlar) küçük harfli biçimi → önerilecek tip
// adıdır. Bu yazımlar tip açıklamasında hata verir ve asıl adı önerir.
var tipOnerileri = func() map[string]string {
	m := map[string]string{}
	ekle := func(y, ad string) {
		if _, gecerli := tipYazimlari[kucukHarf(y)]; !gecerli {
			m[kucukHarf(y)] = ad
		}
	}
	for _, k := range aciklamaTipleri {
		g, _ := kod.Etkin().Girdisi(k)
		ekle(turkce.ASCII(g.Ad), g.Ad)
		for _, e := range g.Eski {
			ekle(e, g.Ad)
			ekle(turkce.ASCII(e), g.Ad)
		}
	}
	return m
}()

// kucukHarf tip adlarını Türkçe kurala göre küçültür (MANTIK → mantık, METİN → metin);
// tip adları büyük/küçük harfe duyarsızdır.
func kucukHarf(s string) string { return strings.ToLowerSpecial(unicode.TurkishCase, s) }

// tipAdiDenetle tip açıklamasındaki ad bir tip adının kabul edilmeyen bir yazımıysa
// (ondalik, int) asıl adı öneren bir hata ekler.
func (c *Checker) tipAdiDenetle(ad string, satir int) {
	if ad == "" {
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
	if ad, ok := tipYazimlari[kucukHarf(t)]; ok {
		return ad
	}
	// Kabul edilmeyen yazım (ondalik) tipAdiDenetle'de hata verir; aynı satırda bir de tip
	// uyuşmazlığı çıkmasın diye önerilen tip varsayılır.
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

func (c *Checker) CheckProgram(prog *agac.Program) []string {
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

func (c *Checker) checkStatement(stmt agac.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *agac.VarStatement:
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

	case *agac.AssignStatement:
		valType := c.inferExpression(s.Value)
		if ident, ok := s.Target.(*agac.Identifier); ok {
			if info, found := c.scope.Resolve(ident.Value); found {
				// Bileşik atamada (x += y) atanan değer 'x + y' ifadesinin sonucudur
				if op := strings.TrimSuffix(s.Operator, "="); op != "" {
					valType = arithmeticType(op, info.TypeName, valType, false)
				}
				if info.IsConst {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Sabit hatası — %q bir sabittir ('sabit') ve değeri değiştirilemez",
						s.Line(), ident.Value,
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

	case *agac.BlockStatement:
		prev := c.scope
		c.scope = NewScope(prev)
		for _, bs := range s.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *agac.ForInStatement:
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

	case *agac.WhileStatement:
		c.inferExpression(s.Condition)
		c.checkStatement(s.Body)

	case *agac.TryCatchStatement:
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

	case *agac.StructStatement:
		c.golgeDenetle(s.Name, s.Line())
		c.scope.Define(s.Name, TypeInfo{TypeName: s.Name})

	case *agac.MultiVarStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, name := range s.Names {
			c.golgeDenetle(name.Value, s.Line())
			c.scope.Define(name.Value, TypeInfo{TypeName: tipHerhangi, IsConst: s.IsConst})
		}

	case *agac.MultiAssignStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, t := range s.Targets {
			if ident, ok := t.(*agac.Identifier); ok {
				if info, found := c.scope.Resolve(ident.Value); found && info.IsConst {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Sabit hatası — %q bir sabittir ('sabit') ve değeri değiştirilemez",
						s.Line(), ident.Value,
					))
				}
			}
		}

	case *agac.DeferStatement:
		if s.Body != nil {
			c.checkStatement(s.Body)
		}
		if s.Call != nil {
			c.inferExpression(s.Call)
		}

	case *agac.ReturnStatement:
		valType := c.inferExpression(s.ReturnValue)
		if c.returnType != "" && !typesCompatible(c.returnType, valType) {
			c.errors = append(c.errors, fmt.Sprintf(
				"Satır %d: Dönüş tipi uyuşmazlığı — fonksiyon %q döndürmeli ancak %q tipinde değer döndürülüyor%s",
				s.Line(), c.returnType, valType, sayiIpucu(c.returnType, valType),
			))
		}

	case *agac.ThrowStatement:
		c.inferExpression(s.Value)

	case *agac.AsyncStatement:
		if s.Body != nil {
			c.checkStatement(s.Body)
		}
		if s.Call != nil {
			c.inferExpression(s.Call)
		}

	case *agac.BreakpointStatement:
		c.inferExpression(s.Condition)

	case *agac.TestStatement:
		c.checkStatement(s.Body)

	case *agac.ExpressionStatement:
		c.inferExpression(s.Expression)
	}
}

// sayiIpucu Tam beklenen yere Tam ya da Ondalık olabilen bir değer verildiğinde yol gösterir.
func sayiIpucu(beklenen, verilen string) string {
	if normalizeTypeName(beklenen) == tipTam && verilen == tipSayi {
		return "; '/' bölüm tam çıkmazsa, '**' üs negatifse Ondalık verir: Tam için tam(...) kullanın ya da tipi Ondalık yapın"
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

func (c *Checker) inferExpression(expr agac.Expression) string {
	if expr == nil {
		return tipYok
	}
	switch e := expr.(type) {
	case *agac.IntegerLiteral:
		return tipTam
	case *agac.FloatLiteral:
		return tipOndalik
	case *agac.StringLiteral, *agac.InterpolatedString:
		return tipMetin
	case *agac.BooleanLiteral:
		return tipMantik
	case *agac.NullLiteral:
		return tipYok
	case *agac.ListLiteral, *agac.ListComprehension:
		return tipListe
	case *agac.DictLiteral:
		return tipSozluk
	case *agac.RangeExpression:
		return tipAralik
	case *agac.Identifier:
		if info, ok := c.scope.Resolve(e.Value); ok {
			return info.TypeName
		}
		return tipHerhangi
	case *agac.PrefixExpression:
		right := c.inferExpression(e.Right)
		if e.Operator == "!" {
			return tipMantik
		}
		return right
	case *agac.InfixExpression:
		left := c.inferExpression(e.Left)
		right := c.inferExpression(e.Right)
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
			lit, tamSabit := e.Right.(*agac.IntegerLiteral)
			return arithmeticType(e.Operator, left, right, tamSabit && lit.Value >= 0)
		}
		return tipHerhangi
	case *agac.FunctionLiteral:
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
	case *agac.CallExpression:
		var info TypeInfo
		var found bool
		ident, isIdent := e.Function.(*agac.Identifier)
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
	case *agac.IfExpression:
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
	case *agac.MatchExpression:
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

package checker

import (
	"fmt"
	"strings"
	"tr_dili/pkg/ast"
	"tr_dili/pkg/sozluk"
	"tr_dili/pkg/turkce"
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
	if k, ok := sozluk.Bul(sozluk.AlanGenel, ad); ok && k.Yerlesik() {
		c.uyarilar = append(c.uyarilar, fmt.Sprintf(
			"Satır %d: '%s' yerleşik bir ad; bu kapsamda yerleşik '%s' artık kullanılamaz", satir, ad, ad))
	}
}

// Denetleyicinin kullandığı tip adları sözlüktedir (pkg/sozluk, A2xx).
var (
	tipTam       = sozluk.Ad(sozluk.A + 200)
	tipOndalik   = sozluk.Ad(sozluk.A + 201)
	tipMetin     = sozluk.Ad(sozluk.A + 202)
	tipMantik    = sozluk.Ad(sozluk.A + 203)
	tipYok       = sozluk.Ad(sozluk.A + 204)
	tipListe     = sozluk.Ad(sozluk.A + 205)
	tipSozluk    = sozluk.Ad(sozluk.A + 206)
	tipFonksiyon = sozluk.Ad(sozluk.A + 208)
	tipTensor    = sozluk.Ad(sozluk.A + 212)
	tipHerhangi  = sozluk.Ad(sozluk.A + 213)
)

// aciklamaTipleri tip açıklamasında (değişken x: Tam) kullanılabilen tiplerin kodlarıdır.
var aciklamaTipleri = []sozluk.Kod{sozluk.A + 200, sozluk.A + 201, sozluk.A + 202, sozluk.A + 203,
	sozluk.A + 205, sozluk.A + 206, sozluk.A + 212, sozluk.A + 204}

// tipYazimlari tip açıklamasında kabul edilen yazımların küçük harfli biçimi → tip adıdır.
// Yazımlar sözlükten gelir (yalnızca Türkçe biçim; büyük/küçük harf fark etmez). Matris 2
// boyutlu tensördür; ayrı bir tip değildir.
var tipYazimlari = func() map[string]string {
	m := map[string]string{}
	for _, k := range aciklamaTipleri {
		for _, y := range sozluk.Yazimlar(k) {
			m[kucukHarf(y)] = sozluk.Ad(k)
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
		g, _ := sozluk.Etkin().Girdisi(k)
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
	// Tam sayı, Ondalık beklenen yere atanabilir
	if exp == tipOndalik && act == tipTam {
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
		c.golgeDenetle(s.Name.Value, s.Line())
		c.tipAdiDenetle(s.TypeName, s.Line())
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
				// Bileşik atamada (x += y) atanan değer 'x + y' ifadesinin sonucudur
				if op := strings.TrimSuffix(s.Operator, "="); op != "" {
					valType = arithmeticType(op, info.TypeName, valType)
				}
				if info.IsConst {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Sabit hatası — %q bir sabittir ('sabit') ve değeri değiştirilemez",
						s.Line(), ident.Value,
					))
				}
				if info.TypeName != tipHerhangi && info.TypeName != tipYok && !typesCompatible(info.TypeName, valType) {
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
		c.scope.Define(s.ErrorVar, TypeInfo{TypeName: tipMetin})
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

	case *ast.ReturnStatement:
		valType := c.inferExpression(s.ReturnValue)
		if c.returnType != "" && !typesCompatible(c.returnType, valType) {
			c.errors = append(c.errors, fmt.Sprintf(
				"Satır %d: Dönüş tipi uyuşmazlığı — fonksiyon %q döndürmeli ancak %q tipinde değer döndürülüyor",
				s.Line(), c.returnType, valType,
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

// arithmeticType bir aritmetik işlemin sonuç tipini çıkarır.
func arithmeticType(op, left, right string) string {
	if left == tipMetin && op == "+" {
		return tipMetin
	}
	if left == tipOndalik || right == tipOndalik {
		return tipOndalik
	}
	if left == tipTam && right == tipTam {
		return tipTam
	}
	return tipHerhangi
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
		return tipListe
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
			return arithmeticType(e.Operator, left, right)
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

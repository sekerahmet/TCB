// Package checker is TR's type checker that runs before execution: it walks the syntax tree
// and finds type mismatches (declaration, assignment, return value, call argument),
// assignments to constants, undefined names and shadowed builtin names; it reports errors
// and warnings with their lines ('tr kontrol', and before running or loading a module).
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package checker

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
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
	// warnings programın çalışmasını engellemeyen bulgulardır (ör. yerleşik ad gölgeleme).
	warnings []string
	// İçinde bulunulan fonksiyonun bildirilmiş dönüş tipi ("" = fonksiyon dışı veya tipsiz)
	returnType string
	// globalNames tanımsız ad denetiminin genel kapsamıdır (kabukta satırlar arasında kalır);
	// adHatalari son CheckProgram çağrısının tanımsız ad hatalarıdır (undefined_names.go).
	globalNames *nameScope
	nameErrors  []string
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
	return c.warnings
}

// checkShadowing ad bir yerleşik fonksiyonun adıysa (sözlükte G/T/V/W yerleşik kodu) uyarı
// ekler: kullanıcı tanımı o kapsamda yerleşiği gölgeler; yerleşik orada artık çağrılamaz.
func (c *Checker) checkShadowing(name string, line int) {
	if k, ok := codes.Find(codes.NamespaceGlobal, name); ok && k.IsBuiltin() {
		c.warnings = append(c.warnings, fmt.Sprintf(
			"Satır %d: '%s' yerleşik bir ad; bu kapsamda yerleşik '%s' artık kullanılamaz", line, name, name))
	}
}

// Denetleyicinin kullandığı tip adları kod listesindedir (internal/lang/codes, A2xx).
var (
	typeInt      = codes.Name(codes.A200)
	typeFloat    = codes.Name(codes.A201)
	typeString   = codes.Name(codes.A202)
	typeBool     = codes.Name(codes.A203)
	typeNull     = codes.Name(codes.A204)
	typeList     = codes.Name(codes.A205)
	typeDict     = codes.Name(codes.A206)
	typeRange    = codes.Name(codes.A207)
	typeFunction = codes.Name(codes.A208)
	typeTensor   = codes.Name(codes.A212)
	typeAny      = codes.Name(codes.A213)
)

// typeNumber denetleyicinin iç tipidir: değer çalışma anında Tam da Ondalık da olabilir (ör.
// Tam / Tam: 6 / 3 = 2, 7 / 2 = 3.5). Kullanıcı açıklamasında yazılamaz; yalnızca Ondalık
// (Tam da Ondalığa atanabildiği için) ve tipsiz yerlere atanabilir.
var typeNumber = typeInt + " ya da " + typeFloat

// annotationTypes tip açıklamasında (değişken x: Tam) kullanılabilen tiplerin kodlarıdır:
// 3.11'deki çekirdek tip adları.
var annotationTypes = []codes.Code{codes.A200, codes.A201, codes.A202, codes.A203,
	codes.A205, codes.A206, codes.A207, codes.A208, codes.A209, codes.A212, codes.A204, codes.A215}

// typeSpellings tip açıklamasında kabul edilen yazımlar → tip adıdır. Yazımlar sözlükten
// gelir ve tektir (3.14): yalnızca tip adının tam yazımı (Tam, Ondalık); büyük/küçük harfi
// farklı bir yazım (tam, MANTIK) kabul edilmez. Matris 2 boyutlu tensördür; ayrı bir tip
// değildir.
var typeSpellings = func() map[string]string {
	m := map[string]string{}
	for _, k := range annotationTypes {
		for _, y := range codes.Spellings(k) {
			m[y] = codes.Name(k)
		}
	}
	return m
}()

// typeCaseSpellings kabul edilen yazımların Türkçe kuralla küçültülmüş biçimi → tip adıdır.
// Yalnızca ipucu içindir: yazım bir tip adından yalnızca büyük/küçük harfle ayrılıyorsa
// (tam, ONDALIK) denetleyici asıl yazımı söyler.
var typeCaseSpellings = func() map[string]string {
	m := map[string]string{}
	for y, name := range typeSpellings {
		m[lowerCase(y)] = name
	}
	return m
}()

// typeSuggestions kabul edilmeyen ama bir tip adına işaret eden yazımların (Türkçe
// karaktersiz biçim: ondalik; kaldırılmış adlar) küçük harfli biçimi → önerilecek tip
// adıdır. Bu yazımlar tip açıklamasında hata verir ve asıl adı önerir.
var typeSuggestions = func() map[string]string {
	m := map[string]string{}
	add := func(y, name string) {
		if _, valid := typeCaseSpellings[lowerCase(y)]; !valid {
			m[lowerCase(y)] = name
		}
	}
	for _, k := range annotationTypes {
		g, _ := codes.Active().Entry(k)
		add(turkish.ASCII(g.Name), g.Name)
		for _, e := range g.Old {
			add(e, g.Name)
			add(turkish.ASCII(e), g.Name)
		}
	}
	return m
}()

// lowerCase tip adlarını Türkçe kurala göre küçültür (MANTIK → mantık, METİN → metin);
// yalnızca kabul edilmeyen bir yazımın ipucunu bulmak için kullanılır.
func lowerCase(s string) string { return strings.ToLowerSpecial(unicode.TurkishCase, s) }

// checkTypeName tip açıklamasındaki ad bir tip adının kabul edilmeyen bir yazımıysa
// (tam, ondalik, int) asıl adı öneren bir hata ekler.
func (c *Checker) checkTypeName(name string, line int) {
	if name == "" {
		return
	}
	if _, valid := typeSpellings[name]; valid {
		return
	}
	if canonical, ok := typeCaseSpellings[lowerCase(name)]; ok {
		c.errors = append(c.errors, fmt.Sprintf(
			"Satır %d: '%s' TR'de bir tip adı değil; tip adının tek yazımı vardır: '%s' yazın", line, name, canonical))
		return
	}
	if suggestion, ok := typeSuggestions[lowerCase(name)]; ok {
		c.errors = append(c.errors, fmt.Sprintf(
			"Satır %d: '%s' TR'de bir tip adı değil; '%s' yazmak mı istediniz?", line, name, suggestion))
	}
}

func normalizeTypeName(t string) string {
	if t == "" {
		return typeAny
	}
	if name, ok := typeSpellings[t]; ok {
		return name
	}
	// Kabul edilmeyen yazım (tam, ondalik) tipAdiDenetle'de hata verir; aynı satırda bir de
	// tip uyuşmazlığı çıkmasın diye önerilen tip varsayılır.
	if name, ok := typeCaseSpellings[lowerCase(t)]; ok {
		return name
	}
	if name, ok := typeSuggestions[lowerCase(t)]; ok {
		return name
	}
	return t
}

func typesCompatible(expected, actual string) bool {
	exp := normalizeTypeName(expected)
	act := normalizeTypeName(actual)
	if exp == typeAny || act == typeAny || act == typeNull {
		return true
	}
	// Tam sayı (ve Tam ya da Ondalık olabilen değer), Ondalık beklenen yere atanabilir
	if exp == typeFloat && (act == typeInt || act == typeNumber) {
		return true
	}
	return exp == act
}

// CheckProgram checks prog and returns the errors it found.
func (c *Checker) CheckProgram(prog *ast.Program) []string {
	start := len(c.errors)
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	// Tanımsız adlar (5.14) tip hatalarıyla birlikte, satır sırasıyla bildirilir.
	c.nameErrors = c.nameErrors[:0]
	c.checkNames(prog)
	c.errors = append(c.errors, c.nameErrors...)
	sortByLine(c.errors[start:])
	return c.errors
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.VarStatement:
		c.checkShadowing(s.Name.Value, s.Line())
		c.checkTypeName(s.TypeName, s.Line())
		inferred := c.inferExpression(s.Value)
		declared := normalizeTypeName(s.TypeName)
		if s.TypeName != "" && !typesCompatible(declared, inferred) {
			c.errors = append(c.errors, fmt.Sprintf(
				"Satır %d: Tip uyuşmazlığı — %q değişkeni %q tipinde tanımlandı ancak %q tipinde değer atandı%s",
				s.Line(), s.Name.Value, declared, inferred, numberHint(declared, inferred),
			))
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
				if info.TypeName != typeAny && info.TypeName != typeNull && !typesCompatible(info.TypeName, valType) {
					c.errors = append(c.errors, fmt.Sprintf(
						"Satır %d: Tip uyuşmazlığı — %q değişkeni (%s) içine %q tipinde değer atanamaz%s",
						s.Line(), ident.Value, info.TypeName, valType, numberHint(info.TypeName, valType),
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
			c.checkShadowing(s.KeyVar, s.Line())
			c.scope.Define(s.KeyVar, TypeInfo{TypeName: typeAny})
		}
		c.checkShadowing(s.ValueVar, s.Line())
		c.scope.Define(s.ValueVar, TypeInfo{TypeName: typeAny})
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
		c.checkShadowing(s.ErrorVar, catchLine)
		// Yakalanan hata Metin'dir, ama değişken açıklamasızdır: sonra başka tipte değer alabilir.
		c.scope.Define(s.ErrorVar, TypeInfo{TypeName: typeAny})
		for _, bs := range s.CatchBody.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *ast.StructStatement:
		c.checkShadowing(s.Name, s.Line())
		c.scope.Define(s.Name, TypeInfo{TypeName: s.Name})

	case *ast.MultiVarStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		for _, name := range s.Names {
			c.checkShadowing(name.Value, s.Line())
			c.scope.Define(name.Value, TypeInfo{TypeName: typeAny, IsConst: s.IsConst})
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
				s.Line(), c.returnType, valType, numberHint(c.returnType, valType),
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

// numberHint Tam beklenen yere Tam ya da Ondalık olabilen bir değer verildiğinde yol gösterir.
func numberHint(expected, given string) string {
	if normalizeTypeName(expected) == typeInt && given == typeNumber {
		return fmt.Sprintf("; '/' bölüm tam çıkmazsa, '**' üs negatifse %s verir: %s için %s(...) kullanın ya da tipi %s yapın",
			typeFloat, typeInt, codes.Name(codes.G210), typeFloat)
	}
	return ""
}

// numericType tip bir sayı tipi mi (Tam, Ondalık ya da ikisinden biri)?
func numericType(t string) bool {
	return t == typeInt || t == typeFloat || t == typeNumber
}

// arithmeticType bir aritmetik işlemin sonuç tipini çalışma zamanının kurallarıyla çıkarır
// (4.2, 4.3, 4.10): denetleyici çalışma zamanından farklı bir tip iddia etmez. Tam / Tam
// tam bölünmezse Ondalık verir, Tam ** Tam üs negatifse Ondalık verir; bu yüzden ikisi de
// "Tam ya da Ondalık"tır. tamUs, '**'in sağının negatif olmayan bir Tam sabiti olduğunu
// söyler (2 ** 3 her zaman Tam'dır).
func arithmeticType(op, left, right string, intExponent bool) string {
	if op == "+" && (left == typeString || right == typeString) {
		return typeString
	}
	if !numericType(left) || !numericType(right) {
		return typeAny
	}
	if left == typeFloat || right == typeFloat {
		return typeFloat
	}
	if op == "/" || (op == "**" && !(intExponent && left == typeInt)) {
		return typeNumber
	}
	if left == typeNumber || right == typeNumber {
		return typeNumber
	}
	return typeInt
}

func (c *Checker) inferExpression(expr ast.Expression) string {
	if expr == nil {
		return typeNull
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return typeInt
	case *ast.FloatLiteral:
		return typeFloat
	case *ast.StringLiteral, *ast.InterpolatedString:
		return typeString
	case *ast.BooleanLiteral:
		return typeBool
	case *ast.NullLiteral:
		return typeNull
	case *ast.ListLiteral, *ast.ListComprehension:
		return typeList
	case *ast.DictLiteral:
		return typeDict
	case *ast.RangeExpression:
		return typeRange
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
		var paramTypes []string
		for _, p := range e.Parameters {
			c.checkTypeName(p.TypeName, e.Line())
			paramTypes = append(paramTypes, normalizeTypeName(p.TypeName))
		}
		c.checkTypeName(e.ReturnType, e.Line())
		retType := normalizeTypeName(e.ReturnType)
		if e.Name != "" {
			c.checkShadowing(e.Name, e.Line())
			c.scope.Define(e.Name, TypeInfo{
				TypeName:   typeFunction,
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
			c.checkShadowing(p.Name, e.Line())
			c.scope.Define(p.Name, TypeInfo{TypeName: normalizeTypeName(p.TypeName)})
		}
		for _, stmt := range e.Body.Statements {
			c.checkStatement(stmt)
		}
		c.scope, c.returnType = prev, prevRet
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
				c.errors = append(c.errors, fmt.Sprintf(
					"Satır %d: Fonksiyon çağrısı tip hatası — %q fonksiyonunun %d. parametresi %q bekliyor ancak %q verildi",
					e.Line(), ident.Value, i+1, info.ParamTypes[i], argType,
				))
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

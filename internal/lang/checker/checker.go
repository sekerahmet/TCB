// Package checker is TR's type checker that runs before execution: it walks the syntax tree
// and finds type mismatches (declaration, assignment, return value, call argument),
// assignments to constants, undefined names and shadowed builtin names; it reports errors
// and warnings with their lines ('tr check', and before running or loading a module).
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TypeInfo is what the checker knows about a name's type.
type TypeInfo struct {
	TypeName   string
	IsConst    bool
	ParamTypes []string
	ReturnType string
	// TupleLen dönüşü demet tipi olarak yazılmış fonksiyonun öğe sayısıdır (6.3; unpack.go); 0:
	// bilinmiyor ya da demet değil.
	TupleLen int
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
	errors []error
	// warnings programın çalışmasını engellemeyen bulgulardır (ör. yerleşik ad gölgeleme).
	warnings []error
	// interactive kabukta açıktır: ifade deyiminin değeri gösterildiği için etkisiz sayılmaz.
	interactive bool
	// blockValues değeri kullanılan bir bloğun (değer olarak kullanılan eğer ya da eşle kolu) son ifade
	// deyimleridir: bloğun değeridir, etkisiz sayılmaz. statementExpr o an denetlenen, değeri kullanılmayan
	// ifade deyiminin ifadesidir (eğer deyimi olarak yazılan eğer).
	blockValues   map[*ast.ExpressionStatement]bool
	statementExpr ast.Expression
	// İçinde bulunulan fonksiyonun bildirilmiş dönüş tipi ("" = fonksiyon dışı veya tipsiz)
	returnType string
	// globalNames tanımsız ad denetiminin genel kapsamıdır (kabukta satırlar arasında kalır);
	// adHatalari son CheckProgram çağrısının tanımsız ad hatalarıdır (undefined_names.go).
	globalNames *nameScope
	nameErrors  []error
	// fileLanguage denetlenen yazılan satırın (kabuk) yazım dilinin kodudur; "": dosya, dili yoktur
	// (2.1). İletiler dilin sözcüklerini dosyada kimlik yazımıyla, satırda o dilde adlandırır (word).
	fileLanguage string
	// fileKind ve fileLines denetlenen dosyanın türü ve satırlarıdır (1.1; module_rules.go);
	// valueTypes son CheckProgram'daki TR_var ve TR_const ilk değerlerinin (iç içe olanlar da)
	// çıkarılan tipleridir: en dıştakiler iletinin önerisi için (H1108), hepsi çıkarımın sayacı için.
	fileKind   FileKind
	fileLines  []string
	valueTypes map[ast.Expression]string
}

// New returns a checker with an empty global scope.
func New() *Checker {
	return &Checker{
		scope:  NewScope(nil),
		errors: []error{},
	}
}

// Errs returns every error found so far, each at its line (messages.Pos).
func (c *Checker) Errs() []error {
	return c.errors
}

// Errors returns the texts of Errs ("Satır N: ...").
func (c *Checker) Errors() []string {
	return messages.Texts(c.errors)
}

// Warnings returns the checker's warnings, each at its line (messages.Pos). Warnings are not errors: the program runs and
// the exit code is unaffected; the command line prints them under a separate heading.
func (c *Checker) Warnings() []error {
	return c.warnings
}

// defineName bir adı kapsama tanımlar: önce gölgeleme uyarısı, sonra yazım kuralı denetimi, en
// sonda kapsam kaydı. Değeri çıkarılacak tanımlarda (değişken bildirimi) kayıt değerden sonra
// yapılır; onlar bu yardımcıyı kullanmaz.
func (c *Checker) defineName(name string, kind nameKind, at lexer.Token, info TypeInfo) {
	c.checkShadowing(name, at)
	c.checkNameStyle(name, kind, at)
	c.scope.Define(name, info)
}

// checkShadowing ad bir yerleşik fonksiyonun adıysa (sözlükte fn yerleşik kodu) uyarı
// ekler: kullanıcı tanımı o kapsamda yerleşiği gölgeler; yerleşik orada artık çağrılamaz.
// Yalnız kişinin dilinde yazılan satırda (kabuk, SetFileLanguage) olur: dosyanın dili yoktur,
// yerleşik TR_ ile yazılır ve hiçbir kullanıcı adı onu gölgelemez (2.1; 'toplam' TR_sum değildir).
func (c *Checker) checkShadowing(name string, at lexer.Token) {
	if c.fileLanguage == "" {
		return
	}
	lang, err := codes.LoadLanguage(c.fileLanguage)
	if err != nil {
		return
	}
	if k, ok := lang.Find(codes.NamespaceGlobal, name); ok && k.IsBuiltin() {
		c.addWarningAtToken(at, messages.New(messages.H1009, name))
	}
}

// CheckProgram checks prog and returns the errors it found.
func (c *Checker) CheckProgram(prog *ast.Program) []string {
	c.check(prog)
	return c.Errors()
}

// check checks prog and records the errors it found.
func (c *Checker) check(prog *ast.Program) {
	start := len(c.errors)
	c.valueTypes = nil
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	// Tanımsız adlar (5.14) tip hatalarıyla birlikte, satır sırasıyla bildirilir.
	c.nameErrors = c.nameErrors[:0]
	c.checkNames(prog)
	c.errors = append(c.errors, c.nameErrors...)
	c.checkSkeletons(prog)
	c.checkTopLevel(prog)
	sortByLine(c.errors[start:])
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.VarStatement:
		c.checkShadowing(s.Name.Value, s.Name.Token)
		c.checkNameStyle(s.Name.Value, c.varKind(s.IsConst), s.Name.Token)
		declared := c.writtenType(s.Type, s.Line())
		inferred := c.inferExpression(s.Value)
		c.recordValueType(s.Value, inferred)
		if s.Type != nil && !typesCompatible(declared, inferred) {
			c.addErrorAt(s, messages.New(messages.H1001,
				s.Name.Value, c.typeArg(declared), c.typeArg(inferred), c.divisionHint(declared, inferred, dividesAtTop(s.Value))))
		}
		// Dil dinamiktir: açıklamasız bir değişkenin ilk değerden çıkarılan tipi kalıcı bir
		// bildirim değildir (değişken sonra başka tipte değer alabilir), bu yüzden tipsiz
		// sayılır. Sabitler yeniden atanamadığı için çıkarılan tipleri kalır.
		finalType := typeAny
		switch {
		case s.Type != nil:
			finalType = declared
		case s.IsConst:
			finalType = inferred
		}
		c.scope.Define(s.Name.Value, TypeInfo{
			TypeName: finalType,
			IsConst:  s.IsConst,
		})

	case *ast.AssignStatement:
		c.checkAssign(s)

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
			c.defineName(s.KeyVar, kindVariable, siteOf(s.KeyToken, s.Line()), TypeInfo{TypeName: typeAny})
		}
		c.defineName(s.ValueVar, kindVariable, siteOf(s.ValueToken, s.Line()), TypeInfo{TypeName: typeAny})
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
		// Yakalanan hata Metin'dir, ama değişken açıklamasızdır: sonra başka tipte değer alabilir.
		c.defineName(s.ErrorVar, kindVariable, siteOf(s.ErrorToken, catchLine), TypeInfo{TypeName: typeAny})
		for _, bs := range s.CatchBody.Statements {
			c.checkStatement(bs)
		}
		c.scope = prev

	case *ast.StructStatement:
		c.checkShadowing(s.Name, siteOf(s.NameToken, s.Line()))
		c.checkNameStyle(s.Name, kindType, siteOf(s.NameToken, s.Line()))
		for _, f := range s.Fields {
			c.checkNameStyle(f.Name, kindValue, siteOf(f.NameToken, s.Line()))
		}
		for name, m := range s.Methods {
			c.checkNameStyle(name, kindValue, siteOf(m.NameToken, m.Line()))
		}
		// Yapının adı değer olarak bir tip değeridir (3.11): tipi TR_Type'tır, yapının kendisi değil.
		c.scope.Define(s.Name, TypeInfo{TypeName: typeType})
		c.checkStruct(s)

	case *ast.MultiVarStatement:
		for _, v := range s.Values {
			c.recordValueType(v, c.inferExpression(v))
		}
		c.checkUnpackCount(s, s.Values, len(s.Names))
		for _, name := range s.Names {
			c.defineName(name.Value, c.varKind(s.IsConst), siteOf(name.Token, s.Line()), TypeInfo{TypeName: typeAny, IsConst: s.IsConst})
		}

	case *ast.MultiAssignStatement:
		for _, v := range s.Values {
			c.inferExpression(v)
		}
		c.checkUnpackCount(s, s.Values, len(s.Targets))
		for _, t := range s.Targets {
			ident, ok := t.(*ast.Identifier)
			if !ok {
				c.inferExpression(t)
				continue
			}
			if info, found := c.scope.Resolve(ident.Value); found && info.IsConst {
				c.addErrorAt(ident, messages.New(messages.H1002, ident.Value, c.word(codes.KwConst)))
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
			c.addErrorAt(s, messages.New(messages.H1003,
				c.typeArg(c.returnType), c.typeArg(valType), c.divisionHint(c.returnType, valType, dividesAtTop(s.ReturnValue))))
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
		prev := c.statementExpr
		c.statementExpr = s.Expression
		if c.blockValues[s] {
			c.statementExpr = nil // bloğun değeri: içindeki eğer'in değeri de kullanılır
		}
		c.inferExpression(s.Expression)
		c.statementExpr = prev
		c.checkUnusedValue(s)
	}
}

// SetFileLanguage sets the writing language of the file being checked (source.Parts.Language):
// a value's text is in it, so comparisons with another language's value names are warned about.
func (c *Checker) SetFileLanguage(code string) { c.fileLanguage = code }

// language denetlenen yazılan satırın yazım dilidir (nil: dosya).
func (c *Checker) language() *codes.Language {
	if l, err := codes.LoadLanguage(c.fileLanguage); err == nil && c.fileLanguage != "" {
		return l
	}
	return nil
}

// word dilin k sözcüğünün iletideki adıdır: dosyada yazımı (TR_const), yazılan satırda iletinin
// dilindeki sözcük (codes.WordRef).
func (c *Checker) word(k codes.Code) any { return codes.WordRef(k, c.language()) }

// SetInteractive marks the checker as the shell's: an expression statement's value is shown
// there, so it is not unused.
func (c *Checker) SetInteractive() { c.interactive = true }

func (c *Checker) inferExpression(expr ast.Expression) string {
	if expr == nil {
		return typeNull
	}
	if t, ok := c.inferNested(expr); ok {
		return t
	}
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return typeInt
	case *ast.FloatLiteral:
		return typeFloat
	case *ast.StringLiteral:
		c.checkForeignInterpolation(e)
		return typeString
	case *ast.BooleanLiteral:
		return typeBool
	case *ast.NullLiteral:
		return typeNull
	case *ast.Identifier:
		if info, ok := c.scope.Resolve(e.Value); ok {
			return info.TypeName
		}
		return typeAny
	case *ast.PrefixExpression:
		right := c.inferExpression(e.Right)
		switch e.Operator {
		case "!":
			return typeBool
		case "~":
			if right == typeInt {
				return typeInt
			}
			return typeAny
		}
		return right
	case *ast.InfixExpression:
		left := c.inferExpression(e.Left)
		right := c.inferExpression(e.Right)
		c.checkTypeTextComparison(e)
		c.checkBoolTextComparison(e)
		switch e.Operator {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return typeBool
		case "??":
			if left != typeNull && left != typeAny {
				return left
			}
			return right
		case "@":
			return typeTensor
		case "+", "-", "*", "/", "//", "%", "**", "&", "|", "^", "<<", ">>":
			return arithmeticType(string(e.Operator), left, right)
		}
		return typeAny
	case *ast.FunctionLiteral:
		paramTypes, retType := c.functionSignature(e)
		if e.Name != "" {
			c.defineName(e.Name, kindValue, siteOf(e.NameToken, e.Line()), TypeInfo{
				TypeName:   typeFunction,
				ParamTypes: paramTypes,
				ReturnType: retType,
				TupleLen:   returnTupleLen(e),
			})
		}
		c.functionBody(e, retType)
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
				c.addErrorAt(arg, messages.New(messages.H1004,
					ident.Value, i+1, c.typeArg(info.ParamTypes[i]), c.typeArg(argType)))
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
		return c.inferIf(e, expr != c.statementExpr)
	case *ast.MatchExpression:
		return c.inferMatch(e, expr != c.statementExpr)
	}
	return typeAny
}

// checkAssign atamayı denetler: sabite atama ve açıklamalı değişkene uyumsuz tipte değer
// hatadır; indeks ve alan hedeflerinin içi de denetlenir.
func (c *Checker) checkAssign(s *ast.AssignStatement) {
	valType := c.inferExpression(s.Value)
	if ident, ok := s.Target.(*ast.Identifier); ok {
		if info, found := c.scope.Resolve(ident.Value); found {
			// Bileşik atamada (x += y) atanan değer 'x + y' ifadesinin sonucudur
			op := strings.TrimSuffix(string(s.Operator), "=")
			if op != "" {
				valType = arithmeticType(op, info.TypeName, valType)
			}
			// Listede += ve *= listeyi yerinde değiştirir, bağ değişmez: sabit listede de geçerlidir
			// (5.4, 5.2); tipi bilinmeyen sabitte karar çalışma anında verilir.
			inPlace := (op == "+" || op == "*") && (info.TypeName == typeList || info.TypeName == typeAny)
			if info.IsConst && !inPlace {
				c.addErrorAt(ident, messages.New(messages.H1002, ident.Value, c.word(codes.KwConst)))
			}
			if info.TypeName != typeAny && info.TypeName != typeNull && !typesCompatible(info.TypeName, valType) {
				c.addErrorAt(s, messages.New(messages.H1005,
					ident.Value, c.typeArg(info.TypeName), c.typeArg(valType), c.divisionHint(info.TypeName, valType, op == "/" || op == "" && dividesAtTop(s.Value))))
			}
		}
	} else {
		// indeks ve alan hedefleri (l[i] = ..., p.x = ...) okunan ifadeler içerir
		c.inferExpression(s.Target)
	}
}

package interp

import "github.com/sekerahmet/TCB/internal/lang/ast"

// Blok kapsam bayrakları (derleyicinin blockFlagCache'inde önbelleğe alınır).
const (
	// blockDefines: blok, çalıştığı ortama doğrudan bir ad tanımlar (değişken, sabit,
	// adlı fonksiyon, yapı, aktar). Böyle olmayan bloklar için yeni ortam açılmaz.
	blockDefines uint8 = 1 << iota
	// blockCaptures: blok (iç içe herhangi bir yerinde) ortamı sonradan kullanılmak üzere
	// yakalayabilecek bir yapı içerir (fonksiyon, yapı metodu, ertele, eşzamanlı). Böyle
	// olmayan döngü gövdelerinin ortamı turlar arasında temizlenip yeniden kullanılır.
	blockCaptures
)

// blockFlagCache blokların kapsam bayraklarını bir derleme işi boyunca saklar. Önbellek
// derleyicinindir, ağaçta değil: ağaç yalnızca veri taşır ve aynı ayrıştırılmış programı
// aynı anda derleyen VM'ler ortak bir şey yazmaz (yarış olmaz).
type blockFlagCache map[*ast.BlockStatement]uint8

// flags bloğun kapsam bayraklarını döndürür; ilk çağrıda hesaplayıp saklar.
func (fc blockFlagCache) flags(b *ast.BlockStatement) uint8 {
	if f, ok := fc[b]; ok {
		return f
	}
	var f uint8
	if statementsDefine(b.Statements) {
		f |= blockDefines
	}
	for _, st := range b.Statements {
		if fc.statementCaptures(st) {
			f |= blockCaptures
			break
		}
	}
	fc[b] = f
	return f
}

// statementsDefine deyimlerden biri çalıştığı ortama doğrudan bir ad tanımlıyor mu?
func statementsDefine(stmts []ast.Statement) bool {
	var var_ bool
	for _, st := range stmts {
		statementNames(st, func(string) { var_ = true })
		if var_ {
			return true
		}
	}
	return false
}

// statementNames deyimin, çalıştığı ortama doğrudan tanımladığı adları ad'a verir. Takma adsız
// 'aktar' (adları çalışma anında belli olur) için ad("") çağrılır. İç bloklar (kendi
// kararlarını verdikleri için) ve fonksiyon gövdeleri incelenmez.
func statementNames(st ast.Statement, name func(string)) {
	switch s := st.(type) {
	case *ast.VarStatement:
		name(s.Name.Value)
	case *ast.MultiVarStatement:
		for _, n := range s.Names {
			name(n.Value)
		}
	case *ast.StructStatement:
		name(s.Name)
	case *ast.ImportStatement:
		name(s.Alias)
	case *ast.ExpressionStatement:
		exprNames(s.Expression, name)
	case *ast.AssignStatement:
		exprNames(s.Target, name)
		exprNames(s.Value, name)
	case *ast.MultiAssignStatement:
		for _, e := range s.Targets {
			exprNames(e, name)
		}
		for _, e := range s.Values {
			exprNames(e, name)
		}
	case *ast.ReturnStatement:
		exprNames(s.ReturnValue, name)
	case *ast.ThrowStatement:
		exprNames(s.Value, name)
	case *ast.ForInStatement:
		exprNames(s.Iterable, name)
	case *ast.WhileStatement:
		exprNames(s.Condition, name)
	case *ast.DeferStatement:
		// 'ertele f()' çağrısı sonradan aynı ortamda değerlendirilir
		exprNames(s.Call, name)
	case *ast.BreakpointStatement:
		exprNames(s.Condition, name)
	}
}

// exprNames ifadenin, değerlendirildiği ortama tanımladığı adları (fonksiyon f() { ... }
// biçimindeki adlı fonksiyonlar) ad'a verir.
func exprNames(e ast.Expression, name func(string)) {
	switch x := e.(type) {
	case nil:
	case *ast.FunctionLiteral:
		if x.Name != "" {
			name(x.Name)
		}
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			exprNames(p, name)
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			exprNames(el, name)
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			exprNames(p.Key, name)
			exprNames(p.Value, name)
		}
	case *ast.RangeExpression:
		exprNames(x.Start, name)
		exprNames(x.End, name)
	case *ast.PrefixExpression:
		exprNames(x.Right, name)
	case *ast.InfixExpression:
		exprNames(x.Left, name)
		exprNames(x.Right, name)
	case *ast.IfExpression:
		exprNames(x.Condition, name)
		for _, ei := range x.ElseIfs {
			exprNames(ei.Condition, name)
		}
	case *ast.MatchExpression:
		exprNames(x.Subject, name)
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				exprNames(p, name)
			}
			exprNames(mc.ValueExpr, name)
		}
	case *ast.CallExpression:
		exprNames(x.Function, name)
		for _, a := range x.Arguments {
			exprNames(a, name)
		}
		for _, na := range x.NamedArgs {
			exprNames(na.Value, name)
		}
	case *ast.IndexExpression:
		exprNames(x.Left, name)
		exprNames(x.Index, name)
	case *ast.MemberExpression:
		exprNames(x.Object, name)
	case *ast.ListComprehension:
		// Çıktı ve koşul kendi ortamında değerlendirilir; yalnızca kaynak bu ortamdadır
		exprNames(x.Iterable, name)
	}
}

// statementCaptures deyimin (iç içe her yerinde) ortamı yakalayabilecek bir yapı içerip
// içermediğini söyler.
func (fc blockFlagCache) statementCaptures(st ast.Statement) bool {
	switch s := st.(type) {
	case nil:
		return false
	case *ast.StructStatement, *ast.DeferStatement, *ast.AsyncStatement:
		return true
	case *ast.VarStatement:
		return fc.exprCaptures(s.Value)
	case *ast.MultiVarStatement:
		for _, e := range s.Values {
			if fc.exprCaptures(e) {
				return true
			}
		}
	case *ast.AssignStatement:
		return fc.exprCaptures(s.Target) || fc.exprCaptures(s.Value)
	case *ast.MultiAssignStatement:
		for _, e := range s.Targets {
			if fc.exprCaptures(e) {
				return true
			}
		}
		for _, e := range s.Values {
			if fc.exprCaptures(e) {
				return true
			}
		}
	case *ast.ReturnStatement:
		return fc.exprCaptures(s.ReturnValue)
	case *ast.ThrowStatement:
		return fc.exprCaptures(s.Value)
	case *ast.ExpressionStatement:
		return fc.exprCaptures(s.Expression)
	case *ast.BlockStatement:
		return fc.doesBlockCapture(s)
	case *ast.ForInStatement:
		return fc.exprCaptures(s.Iterable) || fc.doesBlockCapture(s.Body)
	case *ast.WhileStatement:
		return fc.exprCaptures(s.Condition) || fc.doesBlockCapture(s.Body)
	case *ast.TryCatchStatement:
		return fc.doesBlockCapture(s.TryBody) || fc.doesBlockCapture(s.CatchBody)
	case *ast.BreakpointStatement:
		return fc.exprCaptures(s.Condition)
	}
	return false
}

func (fc blockFlagCache) doesBlockCapture(b *ast.BlockStatement) bool {
	if b == nil {
		return false
	}
	return fc.flags(b)&blockCaptures != 0
}

// exprCaptures ifadenin içinde (iç içe her yerde) bir fonksiyon değeri oluşturulup
// oluşturulmadığını söyler.
func (fc blockFlagCache) exprCaptures(e ast.Expression) bool {
	switch x := e.(type) {
	case nil:
		return false
	case *ast.FunctionLiteral:
		return true
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			if fc.exprCaptures(p) {
				return true
			}
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			if fc.exprCaptures(el) {
				return true
			}
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			if fc.exprCaptures(p.Key) || fc.exprCaptures(p.Value) {
				return true
			}
		}
	case *ast.RangeExpression:
		return fc.exprCaptures(x.Start) || fc.exprCaptures(x.End)
	case *ast.PrefixExpression:
		return fc.exprCaptures(x.Right)
	case *ast.InfixExpression:
		return fc.exprCaptures(x.Left) || fc.exprCaptures(x.Right)
	case *ast.IfExpression:
		if fc.exprCaptures(x.Condition) || fc.doesBlockCapture(x.Consequence) || fc.doesBlockCapture(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if fc.exprCaptures(ei.Condition) || fc.doesBlockCapture(ei.Consequence) {
				return true
			}
		}
	case *ast.MatchExpression:
		if fc.exprCaptures(x.Subject) {
			return true
		}
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				if fc.exprCaptures(p) {
					return true
				}
			}
			if fc.exprCaptures(mc.ValueExpr) || fc.doesBlockCapture(mc.Body) {
				return true
			}
		}
	case *ast.CallExpression:
		if fc.exprCaptures(x.Function) {
			return true
		}
		for _, a := range x.Arguments {
			if fc.exprCaptures(a) {
				return true
			}
		}
		for _, na := range x.NamedArgs {
			if fc.exprCaptures(na.Value) {
				return true
			}
		}
	case *ast.IndexExpression:
		return fc.exprCaptures(x.Left) || fc.exprCaptures(x.Index)
	case *ast.MemberExpression:
		return fc.exprCaptures(x.Object)
	case *ast.ListComprehension:
		return fc.exprCaptures(x.Iterable) || fc.exprCaptures(x.Output) || fc.exprCaptures(x.Condition)
	}
	return false
}

// bodyHasDefer fonksiyon gövdesinde (iç fonksiyonlar ve eşzamanlı bloklar hariç; onların
// kendi kökleri vardır) bir 'ertele' deyimi olup olmadığını söyler.
func bodyHasDefer(stmts []ast.Statement) bool {
	for _, st := range stmts {
		if statementHasDefer(st) {
			return true
		}
	}
	return false
}

func statementHasDefer(st ast.Statement) bool {
	switch s := st.(type) {
	case *ast.DeferStatement:
		return true
	case *ast.BlockStatement:
		return s != nil && bodyHasDefer(s.Statements)
	case *ast.ForInStatement:
		return s.Body != nil && bodyHasDefer(s.Body.Statements)
	case *ast.WhileStatement:
		return s.Body != nil && bodyHasDefer(s.Body.Statements)
	case *ast.TryCatchStatement:
		return statementHasDefer(s.TryBody) || statementHasDefer(s.CatchBody)
	case *ast.ExpressionStatement:
		return exprHasDefer(s.Expression)
	}
	return false
}

// exprHasDefer 'eğer' ve 'eşle' dallarındaki blokları inceler (dallar deyim olarak çalışır).
func exprHasDefer(e ast.Expression) bool {
	switch x := e.(type) {
	case *ast.IfExpression:
		if statementHasDefer(x.Consequence) || statementHasDefer(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if statementHasDefer(ei.Consequence) {
				return true
			}
		}
	case *ast.MatchExpression:
		for _, mc := range x.Cases {
			if statementHasDefer(mc.Body) {
				return true
			}
		}
	}
	return false
}

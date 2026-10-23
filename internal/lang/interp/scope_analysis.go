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
		return
	case *ast.FunctionLiteral:
		if x.Name != "" {
			name(x.Name)
		}
		return
	case *ast.MatchExpression:
		// Kollar kendi kapsamını açar (5.12): yalnızca konu ve desenler bu ortamdadır.
		exprNames(x.Subject, name)
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				exprNames(p, name)
			}
		}
		return
	}
	// Liste üretecinin çıktısı ve koşulu kendi ortamında değerlendirilir; yalnızca kaynağı bu
	// ortamdadır (subExpressions yalnızca kaynağı verir). if/eşle blokları kendi kapsamlarıdır.
	subExpressions(e, func(child ast.Expression) bool {
		exprNames(child, name)
		return false
	})
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
	case *ast.IfExpression:
		if fc.doesBlockCapture(x.Consequence) || fc.doesBlockCapture(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if fc.doesBlockCapture(ei.Consequence) {
				return true
			}
		}
	case *ast.MatchExpression:
		for _, mc := range x.Cases {
			if fc.doesBlockCapture(mc.Body) {
				return true
			}
		}
	case *ast.ListComprehension:
		if fc.exprCaptures(x.Output) || fc.exprCaptures(x.Condition) {
			return true
		}
	}
	return subExpressions(e, fc.exprCaptures)
}

// subExpressions ifadenin, hem ad hem yakalama çözümlemesinin gezdiği doğrudan alt ifadelerini
// sırayla visit'e verir; visit true dönerse durur ve true döner. Bloklar ve liste üretecinin
// çıktısı ile koşulu burada yoktur: onları çözümlemeler kendi kurallarıyla ele alır.
func subExpressions(e ast.Expression, visit func(ast.Expression) bool) bool {
	each := func(list ...ast.Expression) bool {
		for _, c := range list {
			if visit(c) {
				return true
			}
		}
		return false
	}
	switch x := e.(type) {
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			if visit(p) {
				return true
			}
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			if visit(el) {
				return true
			}
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			if each(p.Key, p.Value) {
				return true
			}
		}
	case *ast.RangeExpression:
		return each(x.Start, x.End)
	case *ast.PrefixExpression:
		return visit(x.Right)
	case *ast.InfixExpression:
		return each(x.Left, x.Right)
	case *ast.IfExpression:
		if visit(x.Condition) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if visit(ei.Condition) {
				return true
			}
		}
	case *ast.MatchExpression:
		if visit(x.Subject) {
			return true
		}
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				if visit(p) {
					return true
				}
			}
			if visit(mc.ValueExpr) {
				return true
			}
		}
	case *ast.CallExpression:
		if visit(x.Function) {
			return true
		}
		for _, a := range x.Arguments {
			if visit(a) {
				return true
			}
		}
		for _, na := range x.NamedArgs {
			if visit(na.Value) {
				return true
			}
		}
	case *ast.IndexExpression:
		return each(x.Left, x.Index)
	case *ast.MemberExpression:
		return visit(x.Object)
	case *ast.ListComprehension:
		return visit(x.Iterable)
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

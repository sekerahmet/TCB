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
		ast.StatementDefinedNames(st, func(string) { var_ = true })
		if var_ {
			return true
		}
	}
	return false
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
	return ast.SubExpressions(e, fc.exprCaptures)
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

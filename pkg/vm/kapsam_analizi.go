package vm

import "tr_dili/pkg/ast"

// Blok kapsam bayrakları (ast.BlockStatement.Bayraklar içinde önbelleğe alınır).
const (
	blokHesaplandi uint8 = 1 << iota
	// blokTanimlar: blok, çalıştığı ortama doğrudan bir ad tanımlar (değişken, sabit,
	// adlı fonksiyon, yapı, aktar). Böyle olmayan bloklar için yeni ortam açılmaz.
	blokTanimlar
	// blokYakalar: blok (iç içe herhangi bir yerinde) ortamı sonradan kullanılmak üzere
	// yakalayabilecek bir yapı içerir (fonksiyon, yapı metodu, ertele, eşzamanlı). Böyle
	// olmayan döngü gövdelerinin ortamı turlar arasında temizlenip yeniden kullanılır.
	blokYakalar
)

// blokBayraklari bloğun kapsam bayraklarını döndürür; ilk çağrıda hesaplayıp saklar.
func blokBayraklari(b *ast.BlockStatement) uint8 {
	if b.Bayraklar != 0 {
		return b.Bayraklar
	}
	f := blokHesaplandi
	if deyimlerTanimlar(b.Statements) {
		f |= blokTanimlar
	}
	for _, st := range b.Statements {
		if deyimYakalar(st) {
			f |= blokYakalar
			break
		}
	}
	b.Bayraklar = f
	return f
}

// deyimlerTanimlar deyimlerden biri çalıştığı ortama doğrudan bir ad tanımlıyor mu?
func deyimlerTanimlar(stmts []ast.Statement) bool {
	var var_ bool
	for _, st := range stmts {
		deyimAdlari(st, func(string) { var_ = true })
		if var_ {
			return true
		}
	}
	return false
}

// deyimAdlari deyimin, çalıştığı ortama doğrudan tanımladığı adları ad'a verir. Takma adsız
// 'aktar' (adları çalışma anında belli olur) için ad("") çağrılır. İç bloklar (kendi
// kararlarını verdikleri için) ve fonksiyon gövdeleri incelenmez.
func deyimAdlari(st ast.Statement, ad func(string)) {
	switch s := st.(type) {
	case *ast.VarStatement:
		ad(s.Name.Value)
	case *ast.MultiVarStatement:
		for _, n := range s.Names {
			ad(n.Value)
		}
	case *ast.StructStatement:
		ad(s.Name)
	case *ast.ImportStatement:
		ad(s.Alias)
	case *ast.ExpressionStatement:
		ifadeAdlari(s.Expression, ad)
	case *ast.AssignStatement:
		ifadeAdlari(s.Target, ad)
		ifadeAdlari(s.Value, ad)
	case *ast.MultiAssignStatement:
		for _, e := range s.Targets {
			ifadeAdlari(e, ad)
		}
		for _, e := range s.Values {
			ifadeAdlari(e, ad)
		}
	case *ast.ReturnStatement:
		ifadeAdlari(s.ReturnValue, ad)
	case *ast.ThrowStatement:
		ifadeAdlari(s.Value, ad)
	case *ast.ForInStatement:
		ifadeAdlari(s.Iterable, ad)
	case *ast.WhileStatement:
		ifadeAdlari(s.Condition, ad)
	case *ast.DeferStatement:
		// 'ertele f()' çağrısı sonradan aynı ortamda değerlendirilir
		ifadeAdlari(s.Call, ad)
	case *ast.BreakpointStatement:
		ifadeAdlari(s.Condition, ad)
	}
}

// ifadeAdlari ifadenin, değerlendirildiği ortama tanımladığı adları (fonksiyon f() { ... }
// biçimindeki adlı fonksiyonlar) ad'a verir.
func ifadeAdlari(e ast.Expression, ad func(string)) {
	switch x := e.(type) {
	case nil:
	case *ast.FunctionLiteral:
		if x.Name != "" {
			ad(x.Name)
		}
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			ifadeAdlari(p, ad)
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			ifadeAdlari(el, ad)
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			ifadeAdlari(p.Key, ad)
			ifadeAdlari(p.Value, ad)
		}
	case *ast.RangeExpression:
		ifadeAdlari(x.Start, ad)
		ifadeAdlari(x.End, ad)
	case *ast.PrefixExpression:
		ifadeAdlari(x.Right, ad)
	case *ast.InfixExpression:
		ifadeAdlari(x.Left, ad)
		ifadeAdlari(x.Right, ad)
	case *ast.IfExpression:
		ifadeAdlari(x.Condition, ad)
		for _, ei := range x.ElseIfs {
			ifadeAdlari(ei.Condition, ad)
		}
	case *ast.MatchExpression:
		ifadeAdlari(x.Subject, ad)
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				ifadeAdlari(p, ad)
			}
			ifadeAdlari(mc.ValueExpr, ad)
		}
	case *ast.CallExpression:
		ifadeAdlari(x.Function, ad)
		for _, a := range x.Arguments {
			ifadeAdlari(a, ad)
		}
		for _, na := range x.NamedArgs {
			ifadeAdlari(na.Value, ad)
		}
	case *ast.IndexExpression:
		ifadeAdlari(x.Left, ad)
		ifadeAdlari(x.Index, ad)
	case *ast.MemberExpression:
		ifadeAdlari(x.Object, ad)
	case *ast.ListComprehension:
		// Çıktı ve koşul kendi ortamında değerlendirilir; yalnızca kaynak bu ortamdadır
		ifadeAdlari(x.Iterable, ad)
	}
}

// deyimYakalar deyimin (iç içe her yerinde) ortamı yakalayabilecek bir yapı içerip
// içermediğini söyler.
func deyimYakalar(st ast.Statement) bool {
	switch s := st.(type) {
	case nil:
		return false
	case *ast.StructStatement, *ast.DeferStatement, *ast.AsyncStatement:
		return true
	case *ast.VarStatement:
		return ifadeYakalar(s.Value)
	case *ast.MultiVarStatement:
		for _, e := range s.Values {
			if ifadeYakalar(e) {
				return true
			}
		}
	case *ast.AssignStatement:
		return ifadeYakalar(s.Target) || ifadeYakalar(s.Value)
	case *ast.MultiAssignStatement:
		for _, e := range s.Targets {
			if ifadeYakalar(e) {
				return true
			}
		}
		for _, e := range s.Values {
			if ifadeYakalar(e) {
				return true
			}
		}
	case *ast.ReturnStatement:
		return ifadeYakalar(s.ReturnValue)
	case *ast.ThrowStatement:
		return ifadeYakalar(s.Value)
	case *ast.ExpressionStatement:
		return ifadeYakalar(s.Expression)
	case *ast.BlockStatement:
		return blokYakalarMi(s)
	case *ast.ForInStatement:
		return ifadeYakalar(s.Iterable) || blokYakalarMi(s.Body)
	case *ast.WhileStatement:
		return ifadeYakalar(s.Condition) || blokYakalarMi(s.Body)
	case *ast.TryCatchStatement:
		return blokYakalarMi(s.TryBody) || blokYakalarMi(s.CatchBody)
	case *ast.BreakpointStatement:
		return ifadeYakalar(s.Condition)
	}
	return false
}

func blokYakalarMi(b *ast.BlockStatement) bool {
	if b == nil {
		return false
	}
	return blokBayraklari(b)&blokYakalar != 0
}

// ifadeYakalar ifadenin içinde (iç içe her yerde) bir fonksiyon değeri oluşturulup
// oluşturulmadığını söyler.
func ifadeYakalar(e ast.Expression) bool {
	switch x := e.(type) {
	case nil:
		return false
	case *ast.FunctionLiteral:
		return true
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			if ifadeYakalar(p) {
				return true
			}
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			if ifadeYakalar(el) {
				return true
			}
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			if ifadeYakalar(p.Key) || ifadeYakalar(p.Value) {
				return true
			}
		}
	case *ast.RangeExpression:
		return ifadeYakalar(x.Start) || ifadeYakalar(x.End)
	case *ast.PrefixExpression:
		return ifadeYakalar(x.Right)
	case *ast.InfixExpression:
		return ifadeYakalar(x.Left) || ifadeYakalar(x.Right)
	case *ast.IfExpression:
		if ifadeYakalar(x.Condition) || blokYakalarMi(x.Consequence) || blokYakalarMi(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if ifadeYakalar(ei.Condition) || blokYakalarMi(ei.Consequence) {
				return true
			}
		}
	case *ast.MatchExpression:
		if ifadeYakalar(x.Subject) {
			return true
		}
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				if ifadeYakalar(p) {
					return true
				}
			}
			if ifadeYakalar(mc.ValueExpr) || blokYakalarMi(mc.Body) {
				return true
			}
		}
	case *ast.CallExpression:
		if ifadeYakalar(x.Function) {
			return true
		}
		for _, a := range x.Arguments {
			if ifadeYakalar(a) {
				return true
			}
		}
		for _, na := range x.NamedArgs {
			if ifadeYakalar(na.Value) {
				return true
			}
		}
	case *ast.IndexExpression:
		return ifadeYakalar(x.Left) || ifadeYakalar(x.Index)
	case *ast.MemberExpression:
		return ifadeYakalar(x.Object)
	case *ast.ListComprehension:
		return ifadeYakalar(x.Iterable) || ifadeYakalar(x.Output) || ifadeYakalar(x.Condition)
	}
	return false
}

// govdedeErteleVar fonksiyon gövdesinde (iç fonksiyonlar ve eşzamanlı bloklar hariç; onların
// kendi kökleri vardır) bir 'ertele' deyimi olup olmadığını söyler.
func govdedeErteleVar(stmts []ast.Statement) bool {
	for _, st := range stmts {
		if deyimErteleVar(st) {
			return true
		}
	}
	return false
}

func deyimErteleVar(st ast.Statement) bool {
	switch s := st.(type) {
	case *ast.DeferStatement:
		return true
	case *ast.BlockStatement:
		return s != nil && govdedeErteleVar(s.Statements)
	case *ast.ForInStatement:
		return s.Body != nil && govdedeErteleVar(s.Body.Statements)
	case *ast.WhileStatement:
		return s.Body != nil && govdedeErteleVar(s.Body.Statements)
	case *ast.TryCatchStatement:
		return deyimErteleVar(s.TryBody) || deyimErteleVar(s.CatchBody)
	case *ast.ExpressionStatement:
		return ifadeErteleVar(s.Expression)
	}
	return false
}

// ifadeErteleVar 'eğer' ve 'eşle' dallarındaki blokları inceler (dallar deyim olarak çalışır).
func ifadeErteleVar(e ast.Expression) bool {
	switch x := e.(type) {
	case *ast.IfExpression:
		if deyimErteleVar(x.Consequence) || deyimErteleVar(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if deyimErteleVar(ei.Consequence) {
				return true
			}
		}
	case *ast.MatchExpression:
		for _, mc := range x.Cases {
			if deyimErteleVar(mc.Body) {
				return true
			}
		}
	}
	return false
}

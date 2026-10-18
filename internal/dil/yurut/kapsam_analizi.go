package yurut

import "tr_dili/internal/dil/agac"

// Blok kapsam bayrakları (agac.BlockStatement.Bayraklar içinde önbelleğe alınır).
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
func blokBayraklari(b *agac.BlockStatement) uint8 {
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
func deyimlerTanimlar(stmts []agac.Statement) bool {
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
func deyimAdlari(st agac.Statement, ad func(string)) {
	switch s := st.(type) {
	case *agac.VarStatement:
		ad(s.Name.Value)
	case *agac.MultiVarStatement:
		for _, n := range s.Names {
			ad(n.Value)
		}
	case *agac.StructStatement:
		ad(s.Name)
	case *agac.ImportStatement:
		ad(s.Alias)
	case *agac.ExpressionStatement:
		ifadeAdlari(s.Expression, ad)
	case *agac.AssignStatement:
		ifadeAdlari(s.Target, ad)
		ifadeAdlari(s.Value, ad)
	case *agac.MultiAssignStatement:
		for _, e := range s.Targets {
			ifadeAdlari(e, ad)
		}
		for _, e := range s.Values {
			ifadeAdlari(e, ad)
		}
	case *agac.ReturnStatement:
		ifadeAdlari(s.ReturnValue, ad)
	case *agac.ThrowStatement:
		ifadeAdlari(s.Value, ad)
	case *agac.ForInStatement:
		ifadeAdlari(s.Iterable, ad)
	case *agac.WhileStatement:
		ifadeAdlari(s.Condition, ad)
	case *agac.DeferStatement:
		// 'ertele f()' çağrısı sonradan aynı ortamda değerlendirilir
		ifadeAdlari(s.Call, ad)
	case *agac.BreakpointStatement:
		ifadeAdlari(s.Condition, ad)
	}
}

// ifadeAdlari ifadenin, değerlendirildiği ortama tanımladığı adları (fonksiyon f() { ... }
// biçimindeki adlı fonksiyonlar) ad'a verir.
func ifadeAdlari(e agac.Expression, ad func(string)) {
	switch x := e.(type) {
	case nil:
	case *agac.FunctionLiteral:
		if x.Name != "" {
			ad(x.Name)
		}
	case *agac.InterpolatedString:
		for _, p := range x.Parts {
			ifadeAdlari(p, ad)
		}
	case *agac.ListLiteral:
		for _, el := range x.Elements {
			ifadeAdlari(el, ad)
		}
	case *agac.DictLiteral:
		for _, p := range x.Pairs {
			ifadeAdlari(p.Key, ad)
			ifadeAdlari(p.Value, ad)
		}
	case *agac.RangeExpression:
		ifadeAdlari(x.Start, ad)
		ifadeAdlari(x.End, ad)
	case *agac.PrefixExpression:
		ifadeAdlari(x.Right, ad)
	case *agac.InfixExpression:
		ifadeAdlari(x.Left, ad)
		ifadeAdlari(x.Right, ad)
	case *agac.IfExpression:
		ifadeAdlari(x.Condition, ad)
		for _, ei := range x.ElseIfs {
			ifadeAdlari(ei.Condition, ad)
		}
	case *agac.MatchExpression:
		ifadeAdlari(x.Subject, ad)
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				ifadeAdlari(p, ad)
			}
			ifadeAdlari(mc.ValueExpr, ad)
		}
	case *agac.CallExpression:
		ifadeAdlari(x.Function, ad)
		for _, a := range x.Arguments {
			ifadeAdlari(a, ad)
		}
		for _, na := range x.NamedArgs {
			ifadeAdlari(na.Value, ad)
		}
	case *agac.IndexExpression:
		ifadeAdlari(x.Left, ad)
		ifadeAdlari(x.Index, ad)
	case *agac.MemberExpression:
		ifadeAdlari(x.Object, ad)
	case *agac.ListComprehension:
		// Çıktı ve koşul kendi ortamında değerlendirilir; yalnızca kaynak bu ortamdadır
		ifadeAdlari(x.Iterable, ad)
	}
}

// deyimYakalar deyimin (iç içe her yerinde) ortamı yakalayabilecek bir yapı içerip
// içermediğini söyler.
func deyimYakalar(st agac.Statement) bool {
	switch s := st.(type) {
	case nil:
		return false
	case *agac.StructStatement, *agac.DeferStatement, *agac.AsyncStatement:
		return true
	case *agac.VarStatement:
		return ifadeYakalar(s.Value)
	case *agac.MultiVarStatement:
		for _, e := range s.Values {
			if ifadeYakalar(e) {
				return true
			}
		}
	case *agac.AssignStatement:
		return ifadeYakalar(s.Target) || ifadeYakalar(s.Value)
	case *agac.MultiAssignStatement:
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
	case *agac.ReturnStatement:
		return ifadeYakalar(s.ReturnValue)
	case *agac.ThrowStatement:
		return ifadeYakalar(s.Value)
	case *agac.ExpressionStatement:
		return ifadeYakalar(s.Expression)
	case *agac.BlockStatement:
		return blokYakalarMi(s)
	case *agac.ForInStatement:
		return ifadeYakalar(s.Iterable) || blokYakalarMi(s.Body)
	case *agac.WhileStatement:
		return ifadeYakalar(s.Condition) || blokYakalarMi(s.Body)
	case *agac.TryCatchStatement:
		return blokYakalarMi(s.TryBody) || blokYakalarMi(s.CatchBody)
	case *agac.BreakpointStatement:
		return ifadeYakalar(s.Condition)
	}
	return false
}

func blokYakalarMi(b *agac.BlockStatement) bool {
	if b == nil {
		return false
	}
	return blokBayraklari(b)&blokYakalar != 0
}

// ifadeYakalar ifadenin içinde (iç içe her yerde) bir fonksiyon değeri oluşturulup
// oluşturulmadığını söyler.
func ifadeYakalar(e agac.Expression) bool {
	switch x := e.(type) {
	case nil:
		return false
	case *agac.FunctionLiteral:
		return true
	case *agac.InterpolatedString:
		for _, p := range x.Parts {
			if ifadeYakalar(p) {
				return true
			}
		}
	case *agac.ListLiteral:
		for _, el := range x.Elements {
			if ifadeYakalar(el) {
				return true
			}
		}
	case *agac.DictLiteral:
		for _, p := range x.Pairs {
			if ifadeYakalar(p.Key) || ifadeYakalar(p.Value) {
				return true
			}
		}
	case *agac.RangeExpression:
		return ifadeYakalar(x.Start) || ifadeYakalar(x.End)
	case *agac.PrefixExpression:
		return ifadeYakalar(x.Right)
	case *agac.InfixExpression:
		return ifadeYakalar(x.Left) || ifadeYakalar(x.Right)
	case *agac.IfExpression:
		if ifadeYakalar(x.Condition) || blokYakalarMi(x.Consequence) || blokYakalarMi(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if ifadeYakalar(ei.Condition) || blokYakalarMi(ei.Consequence) {
				return true
			}
		}
	case *agac.MatchExpression:
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
	case *agac.CallExpression:
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
	case *agac.IndexExpression:
		return ifadeYakalar(x.Left) || ifadeYakalar(x.Index)
	case *agac.MemberExpression:
		return ifadeYakalar(x.Object)
	case *agac.ListComprehension:
		return ifadeYakalar(x.Iterable) || ifadeYakalar(x.Output) || ifadeYakalar(x.Condition)
	}
	return false
}

// govdedeErteleVar fonksiyon gövdesinde (iç fonksiyonlar ve eşzamanlı bloklar hariç; onların
// kendi kökleri vardır) bir 'ertele' deyimi olup olmadığını söyler.
func govdedeErteleVar(stmts []agac.Statement) bool {
	for _, st := range stmts {
		if deyimErteleVar(st) {
			return true
		}
	}
	return false
}

func deyimErteleVar(st agac.Statement) bool {
	switch s := st.(type) {
	case *agac.DeferStatement:
		return true
	case *agac.BlockStatement:
		return s != nil && govdedeErteleVar(s.Statements)
	case *agac.ForInStatement:
		return s.Body != nil && govdedeErteleVar(s.Body.Statements)
	case *agac.WhileStatement:
		return s.Body != nil && govdedeErteleVar(s.Body.Statements)
	case *agac.TryCatchStatement:
		return deyimErteleVar(s.TryBody) || deyimErteleVar(s.CatchBody)
	case *agac.ExpressionStatement:
		return ifadeErteleVar(s.Expression)
	}
	return false
}

// ifadeErteleVar 'eğer' ve 'eşle' dallarındaki blokları inceler (dallar deyim olarak çalışır).
func ifadeErteleVar(e agac.Expression) bool {
	switch x := e.(type) {
	case *agac.IfExpression:
		if deyimErteleVar(x.Consequence) || deyimErteleVar(x.Alternative) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if deyimErteleVar(ei.Consequence) {
				return true
			}
		}
	case *agac.MatchExpression:
		for _, mc := range x.Cases {
			if deyimErteleVar(mc.Body) {
				return true
			}
		}
	}
	return false
}

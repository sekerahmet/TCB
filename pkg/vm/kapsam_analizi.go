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
	for _, st := range b.Statements {
		if deyimTanimlar(st) {
			f |= blokTanimlar
			break
		}
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

// deyimTanimlar deyimin, çalıştığı ortama doğrudan ad tanımlayıp tanımlamadığını söyler.
// İç bloklar (kendi kararlarını verdikleri için) ve fonksiyon gövdeleri incelenmez.
func deyimTanimlar(st ast.Statement) bool {
	switch s := st.(type) {
	case *ast.VarStatement, *ast.MultiVarStatement, *ast.StructStatement, *ast.ImportStatement:
		return true
	case *ast.ExpressionStatement:
		return ifadeTanimlar(s.Expression)
	case *ast.AssignStatement:
		return ifadeTanimlar(s.Target) || ifadeTanimlar(s.Value)
	case *ast.MultiAssignStatement:
		for _, e := range s.Targets {
			if ifadeTanimlar(e) {
				return true
			}
		}
		for _, e := range s.Values {
			if ifadeTanimlar(e) {
				return true
			}
		}
	case *ast.ReturnStatement:
		return ifadeTanimlar(s.ReturnValue)
	case *ast.ThrowStatement:
		return ifadeTanimlar(s.Value)
	case *ast.ForInStatement:
		return ifadeTanimlar(s.Iterable)
	case *ast.WhileStatement:
		return ifadeTanimlar(s.Condition)
	case *ast.DeferStatement:
		return ifadeTanimlar(s.Call)
	case *ast.BreakpointStatement:
		return ifadeTanimlar(s.Condition)
	}
	return false
}

// ifadeTanimlar ifadenin, değerlendirildiği ortama ad tanımlayan bir adlı fonksiyon
// içerip içermediğini söyler (fonksiyon f() { ... } ifadesi adı tanımlar).
func ifadeTanimlar(e ast.Expression) bool {
	switch x := e.(type) {
	case nil:
		return false
	case *ast.FunctionLiteral:
		return x.Name != ""
	case *ast.InterpolatedString:
		for _, p := range x.Parts {
			if ifadeTanimlar(p) {
				return true
			}
		}
	case *ast.ListLiteral:
		for _, el := range x.Elements {
			if ifadeTanimlar(el) {
				return true
			}
		}
	case *ast.DictLiteral:
		for _, p := range x.Pairs {
			if ifadeTanimlar(p.Key) || ifadeTanimlar(p.Value) {
				return true
			}
		}
	case *ast.RangeExpression:
		return ifadeTanimlar(x.Start) || ifadeTanimlar(x.End)
	case *ast.PrefixExpression:
		return ifadeTanimlar(x.Right)
	case *ast.InfixExpression:
		return ifadeTanimlar(x.Left) || ifadeTanimlar(x.Right)
	case *ast.IfExpression:
		if ifadeTanimlar(x.Condition) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if ifadeTanimlar(ei.Condition) {
				return true
			}
		}
	case *ast.MatchExpression:
		if ifadeTanimlar(x.Subject) {
			return true
		}
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				if ifadeTanimlar(p) {
					return true
				}
			}
			if ifadeTanimlar(mc.ValueExpr) {
				return true
			}
		}
	case *ast.CallExpression:
		if ifadeTanimlar(x.Function) {
			return true
		}
		for _, a := range x.Arguments {
			if ifadeTanimlar(a) {
				return true
			}
		}
		for _, na := range x.NamedArgs {
			if ifadeTanimlar(na.Value) {
				return true
			}
		}
	case *ast.IndexExpression:
		return ifadeTanimlar(x.Left) || ifadeTanimlar(x.Index)
	case *ast.MemberExpression:
		return ifadeTanimlar(x.Object)
	case *ast.ListComprehension:
		// Çıktı ve koşul kendi ortamında değerlendirilir; yalnızca kaynak bu ortamdadır
		return ifadeTanimlar(x.Iterable)
	}
	return false
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

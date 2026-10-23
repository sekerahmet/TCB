package ast

// SubExpressions ifadenin doğrudan alt ifadelerini kaynak sırasıyla visit'e verir; visit true
// dönerse durur ve true döner. Kapsam çözümlemelerinin (denetleyicideki tanımsız ad taraması,
// yorumlayıcıdaki ad ve yakalama analizi) ortak gezintisidir. Bloklar (eğer ve eşle gövdeleri,
// fonksiyon gövdesi) ve liste üretecinin çıktısı ile koşulu burada yoktur: onları çözümlemeler kendi
// kapsam kurallarıyla ele alır. Eşle kollarının tek satırlık ifadesi (ValueExpr) vardır; kol
// kendi kapsamını açtığı için adlandırma çözümlemeleri ona ExprDefinedNames ile inmez.
func SubExpressions(e Expression, visit func(Expression) bool) bool {
	each := func(list ...Expression) bool {
		for _, c := range list {
			if visit(c) {
				return true
			}
		}
		return false
	}
	switch x := e.(type) {
	case *InterpolatedString:
		for _, p := range x.Parts {
			if visit(p) {
				return true
			}
		}
	case *ListLiteral:
		for _, el := range x.Elements {
			if visit(el) {
				return true
			}
		}
	case *DictLiteral:
		for _, p := range x.Pairs {
			if each(p.Key, p.Value) {
				return true
			}
		}
	case *RangeExpression:
		return each(x.Start, x.End)
	case *PrefixExpression:
		return visit(x.Right)
	case *InfixExpression:
		return each(x.Left, x.Right)
	case *IfExpression:
		if visit(x.Condition) {
			return true
		}
		for _, ei := range x.ElseIfs {
			if visit(ei.Condition) {
				return true
			}
		}
	case *MatchExpression:
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
	case *CallExpression:
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
	case *IndexExpression:
		return each(x.Left, x.Index)
	case *MemberExpression:
		return visit(x.Object)
	case *ListComprehension:
		return visit(x.Iterable)
	}
	return false
}

// ExprDefinedNames ifadenin, değerlendirildiği kapsama tanımladığı adları (fonksiyon f() { ... }
// biçimindeki adlı fonksiyonlar) emit'e verir. Fonksiyon gövdelerine, bloklara ve liste
// üretecinin çıktısı ile koşuluna inmez (5.12); eşle kollarının tek satırlık ifadesi de kolun
// kapsamındadır, yalnızca konu ve desenler bu kapsamdadır.
func ExprDefinedNames(e Expression, emit func(string)) {
	switch x := e.(type) {
	case nil:
		return
	case *FunctionLiteral:
		if x.Name != "" {
			emit(x.Name)
		}
		return
	case *MatchExpression:
		ExprDefinedNames(x.Subject, emit)
		for _, mc := range x.Cases {
			for _, p := range mc.Patterns {
				ExprDefinedNames(p, emit)
			}
		}
		return
	}
	SubExpressions(e, func(child Expression) bool {
		ExprDefinedNames(child, emit)
		return false
	})
}

// StatementDefinedNames deyimin, çalıştığı kapsama doğrudan tanımladığı adları emit'e verir:
// değişken, sabit, yapı, takma adlı aktar ve ifadelerin içindeki adlı fonksiyonlar. Takma adsız
// 'aktar' (adları çalışma anında belli olur) için emit("") çağrılır. İç bloklar ve fonksiyon
// gövdeleri kendi kapsamlarıdır; 'eşzamanlı' deyimi görevin kendi kapsamını açar, ad vermez.
func StatementDefinedNames(st Statement, emit func(string)) {
	switch s := st.(type) {
	case *VarStatement:
		emit(s.Name.Value)
		ExprDefinedNames(s.Value, emit)
	case *MultiVarStatement:
		for _, n := range s.Names {
			emit(n.Value)
		}
		for _, v := range s.Values {
			ExprDefinedNames(v, emit)
		}
	case *StructStatement:
		emit(s.Name)
	case *ImportStatement:
		emit(s.Alias)
	case *ExpressionStatement:
		ExprDefinedNames(s.Expression, emit)
	case *AssignStatement:
		ExprDefinedNames(s.Target, emit)
		ExprDefinedNames(s.Value, emit)
	case *MultiAssignStatement:
		for _, e := range s.Targets {
			ExprDefinedNames(e, emit)
		}
		for _, e := range s.Values {
			ExprDefinedNames(e, emit)
		}
	case *ReturnStatement:
		ExprDefinedNames(s.ReturnValue, emit)
	case *ThrowStatement:
		ExprDefinedNames(s.Value, emit)
	case *ForInStatement:
		ExprDefinedNames(s.Iterable, emit)
	case *WhileStatement:
		ExprDefinedNames(s.Condition, emit)
	case *DeferStatement:
		// 'ertele f()' çağrısı sonradan aynı kapsamda değerlendirilir
		ExprDefinedNames(s.Call, emit)
	case *BreakpointStatement:
		ExprDefinedNames(s.Condition, emit)
	}
}

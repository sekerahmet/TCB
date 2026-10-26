package codeindex

import "github.com/sekerahmet/TCB/internal/lang/ast"

// Bu dosya TR ifadelerinin yapı türünü çıkarır (deneme 7 R4; kod-grafigi-40 tasarımı, TCB onaylı). Pyright'ın tür
// çıkarımının küçük bir alt kümesidir: tek tür ya da bilinmez, birleşim yoktur. Kaynaklar:
//   - yapıcı çağrısı `Yapı(...)` → Yapı;
//   - TR işlevi ya da metodu çağrısı → işlevin bütün `sonuç` ifadeleri aynı yapı türündeyse o tür (dönüş türü);
//   - tek bağlamalı, yeniden atanmayan yerel → atandığı ifadenin türü.
// Tür biliniyorsa `x.alan` o yapının alanına ya da metoduna INFERRED bağlanır (receiver); bilinmezse bugünkü aday
// kuralları sürer. Dönüş türü düğümün InferredType alanına yazılır: artımlı yenilemede değişmeyen dosyanın işlevlerinin
// gövdesi okunmaz, türleri önbellekteki düğümden gelir.

// fnBody bir işlevin ya da metodun gövdesidir (dönüş türü çıkarımı için).
type fnBody struct {
	f  *file
	fl *ast.FunctionLiteral
	o  owner
}

// inferReturns verilen dosyalardaki işlev ve metotların dönüş türlerini çıkarır (çözümden önce).
func (x *indexer) inferReturns(rels []string) {
	x.bodies = map[string]fnBody{}
	x.retDone = map[string]bool{}
	for _, rel := range rels {
		f := x.files[rel]
		if f == nil || f.res == nil || f.res.program == nil {
			continue
		}
		for _, s := range f.res.program.Statements {
			switch s := s.(type) {
			case *ast.ExpressionStatement:
				if fl, ok := s.Expression.(*ast.FunctionLiteral); ok && fl.Name != "" {
					id := NodeID(KindFunction, f.rel, fl.Name)
					x.bodies[id] = fnBody{f, fl, owner{id, KindFunction}}
				}
			case *ast.StructStatement:
				for name, fl := range s.Methods {
					id := NodeID(KindMethod, f.rel, s.Name+"."+name)
					x.bodies[id] = fnBody{f, fl, owner{id, KindMethod}}
				}
			}
		}
	}
	for id := range x.bodies {
		x.returnType(id)
	}
	for _, rel := range rels { // üst düzey değişken ve sabit: değerinin türü (her deyim kendi kapsamında taranır)
		f := x.files[rel]
		if f == nil || f.res == nil || f.res.program == nil {
			continue
		}
		for _, s := range f.res.program.Statements {
			if v, ok := s.(*ast.VarStatement); ok && v.Value != nil {
				kind := KindVariable
				if v.IsConst {
					kind = KindConstant
				}
				if n := x.nodes[NodeID(kind, f.rel, v.Name.Value)]; n != nil {
					n.InferredType, _ = x.exprType(f, collectScope(v.Value), v.Value)
				}
			}
		}
	}
}

// globalType üst düzey bir değişkenin ya da sabitin çıkarılan türüdür (yerel değilse).
func (x *indexer) globalType(f *file, sc *scope, name string) string {
	if sc.locals[name] {
		return ""
	}
	id, _ := x.lookupVisible(f, name)
	if n := x.nodes[id]; n != nil && (n.Kind == KindVariable || n.Kind == KindConstant) {
		return n.InferredType
	}
	return ""
}

// returnType bir işlevin dönüş yapı türüdür ("" bilinmez). Gövdesi bu kurulumda okunmadıysa önbellekteki düğümün
// InferredType alanı geçerlidir. Özyineleme döngüsünde bilinmez sayılır.
func (x *indexer) returnType(id string) string {
	n := x.nodes[id]
	if n == nil {
		return ""
	}
	b, ok := x.bodies[id]
	if !ok || x.retDone[id] {
		return n.InferredType
	}
	x.retDone[id] = true // döngüye karşı: hesaplanırken kendisine varan çağrı bilinmez görür
	n.InferredType = ""
	sc := collectScope(b.fl)
	x.bindOwner(b.f, b.o, sc, b.fl)
	t, seen := "", false
	ast.Walk(b.fl.Body, func(node any) bool {
		if _, nested := node.(*ast.FunctionLiteral); nested {
			return false // iç işlevin `sonuç`u bu işlevin değil
		}
		r, ok := node.(*ast.ReturnStatement)
		if !ok {
			return true
		}
		rt, _ := x.exprType(b.f, sc, r.ReturnValue)
		switch {
		case !seen:
			t, seen = rt, true
		case rt != t:
			t = ""
		}
		return true
	})
	n.InferredType = t
	return t
}

// exprType ifadenin yapı türü ve dayanağıdır ("" bilinmez).
func (x *indexer) exprType(f *file, sc *scope, e ast.Expression) (string, Basis) {
	switch e := e.(type) {
	case *ast.CallExpression:
		if id, _ := x.calleeStruct(f, sc, e.Function); id != "" {
			return id, BasisLocalType
		}
		if fn := x.calleeFunc(f, sc, e.Function); fn != "" {
			if t := x.returnType(fn); t != "" {
				return t, BasisInferredType
			}
		}
	case *ast.Identifier:
		if t := sc.localType[e.Value]; t != "" && sc.locals[e.Value] {
			return t, sc.localBasis[e.Value]
		}
		if t := x.globalType(f, sc, e.Value); t != "" {
			return t, BasisInferredType
		}
	}
	return "", ""
}

// calleeFunc çağrılan ifade bir TR işleviyse (bu dosyada, takma adsız aktarılan modülde ya da `a.işlev`) kimliğidir.
func (x *indexer) calleeFunc(f *file, sc *scope, fn ast.Expression) string {
	id := ""
	switch fn := fn.(type) {
	case *ast.Identifier:
		if !sc.locals[fn.Value] {
			id, _ = x.lookupVisible(f, fn.Value)
		}
	case *ast.MemberExpression:
		if obj, ok := fn.Object.(*ast.Identifier); ok && !sc.locals[obj.Value] {
			if target := f.aliases[obj.Value]; target != nil {
				id = target.defs[fn.Property]
			}
		}
	}
	if n := x.nodes[id]; n == nil || n.Kind != KindFunction {
		return ""
	}
	return id
}

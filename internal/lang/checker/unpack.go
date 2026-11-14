package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Açmada öğe sayısı (5.3, 6.3): sağ yanı tek bir demet olan çoklu tanım ve atamada demetin öğe sayısı
// ad sayısını tutmazsa denetim hatasıdır; demetin uzunluğu tipindedir (3.6a). Sayı üç yerde bilinir:
// sağ yan bir demet yazımıysa, dönüşü '-> (A, B, ...)' diye yazılmış bir fonksiyonun doğrudan
// çağrısıysa (TypeInfo.TupleLen) ve tipi bilinen bir demet ifadesiyse (değişken, tipli parametre,
// alan). Tipi bilinmeyen yerde (fonksiyon değeri, isteğe bağlı demet, liste) sayı çalışırken
// denetlenir (H2700, H2709).

// returnTupleLen dönüşü demet tipi olarak yazılmış fonksiyonun öğe sayısıdır; değilse 0.
func returnTupleLen(e *ast.FunctionLiteral) int {
	if r := e.ReturnType; r != nil && !r.Optional && len(r.Elems) >= 2 {
		return len(r.Elems)
	}
	return 0
}

// tupleLen sağ yanın bilinen öğe sayısıdır: demet yazımı, dönüşü demet tipli fonksiyonun doğrudan
// çağrısı ya da tipi isteğe bağlı olmayan bir demet olan ifade; bilinmiyorsa 0.
func (c *Checker) tupleLen(e ast.Expression) int {
	switch x := e.(type) {
	case *ast.TupleExpression:
		return len(x.Elements)
	case *ast.CallExpression:
		if id, ok := x.Function.(*ast.Identifier); ok {
			if info, found := c.scope.Resolve(id.Value); found {
				if info.TupleLen > 0 {
					return info.TupleLen
				}
			}
		}
	}
	if s := shapeOf(c.itemTypes[e]); s.elems != nil && !s.optional {
		return len(s.elems)
	}
	return 0
}

// checkTupleIndex tipi bilinen demetin sınır dışı indeksini denetler (3.6a): demetin uzunluğu
// tipindedir, t.5 çalışma anının hatasıdır (H2774). Tipi bilinmeyen ve isteğe bağlı demet çalışırken.
func (c *Checker) checkTupleIndex(e *ast.ElementExpression, objType string) {
	s := shapeOf(objType)
	if s.elems == nil || s.optional || e.Index < len(s.elems) {
		return
	}
	c.addErrorAt(e, messages.New(messages.H2774, e.Index, len(s.elems), len(s.elems)-1))
}

// checkUnpackCount n öğeli demeti names ada açan deyimin öğe sayısını denetler (H1026): çoklu tanım ve
// atama (n tek sağ yanın tupleLen'i) ve döngü deseni (n öğenin demet tipinden, loops.go); n bilinmiyorsa
// 0. İleti tek yerde kurulur; örnek düzeltilmiş deyimdir, desende düzeltilmiş döngü başlığı. Bilinmeyen
// sayı çalışırken aynı örnekle denetlenir (H2709, interp.unpackCountError).
func (c *Checker) checkUnpackCount(stmt ast.Node, n, names int) {
	if n > 0 && n != names {
		kind, example := ast.UnpackSuggestion(stmt, n)
		c.addErrorAt(stmt, messages.New(messages.H1026, n, names, kind, example))
	}
}

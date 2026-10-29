package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Açmada öğe sayısı (5.3, 6.3): sağ yanı tek bir demet olan çoklu tanım ve atamada demetin öğe sayısı
// ad sayısını tutmazsa denetim hatasıdır. Sayı yalnız iki yerde bilinir: sağ yan bir demet yazımıysa
// ve dönüşü '-> (A, B, ...)' diye yazılmış bir fonksiyonun doğrudan çağrısıysa (TypeInfo.TupleLen).
// Öbür yerlerde (fonksiyon değeri, metot, liste) sayı çalışırken denetlenir (H2700, H2709). Demet
// tipinin öğeleri tip dizesine girmez (writtenType onu Herhangi yapar); öğe tipleri 4a2'nin işidir.

// returnTupleLen dönüşü demet tipi olarak yazılmış fonksiyonun öğe sayısıdır; değilse 0.
func returnTupleLen(e *ast.FunctionLiteral) int {
	if r := e.ReturnType; r != nil && !r.Optional && len(r.Elems) >= 2 {
		return len(r.Elems)
	}
	return 0
}

// tupleLen sağ yanın bilinen öğe sayısıdır: demet yazımı ya da dönüşü demet tipli fonksiyonun
// doğrudan çağrısı; bilinmiyorsa 0.
func (c *Checker) tupleLen(e ast.Expression) int {
	switch x := e.(type) {
	case *ast.TupleExpression:
		return len(x.Elements)
	case *ast.CallExpression:
		if id, ok := x.Function.(*ast.Identifier); ok {
			if info, found := c.scope.Resolve(id.Value); found {
				return info.TupleLen
			}
		}
	}
	return 0
}

// checkUnpackCount çoklu tanım ve atamanın (names ad) tek sağ yanının öğe sayısını denetler.
func (c *Checker) checkUnpackCount(stmt ast.Node, values []ast.Expression, names int) {
	if len(values) != 1 {
		return
	}
	if n := c.tupleLen(values[0]); n > 0 && n != names {
		kind, example := ast.UnpackSuggestion(stmt, n)
		c.addErrorAt(stmt, messages.New(messages.H1026, n, names, kind, example))
	}
}

package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Çoklu atamanın sırası (5.3): önce hedeflerin yerleri (indeks, alanın sahibi; 5.4) soldan sağa,
// sonra sağdaki ifadelerin hepsi, sonra yazmalar soldan sağa. Bir hedefin yeri aynı atamada
// solunda atanan bir adı ya da yolu okuyorsa ('i, l[i] = 1, 5'; 'a, a.b = x, y') yer eski değerle
// hesaplanır, Python'da yeniyle: sessizce ayrılırdı, denetim hatasıdır (H1160). Sağında atanan adı
// okumak serbesttir, iki sıra aynı sonucu verir. Ad ve yol düzeyindedir: takma adla aynı nesneyi
// gösteren iki ad yakalanmaz.

// checkTargetOrder hedeflerin yerlerini solundaki hedeflere göre denetler.
func (c *Checker) checkTargetOrder(s *ast.MultiAssignStatement) {
	for j, t := range s.Targets {
		for i := 0; i < j; i++ {
			left := s.Targets[i]
			if !assignedPath(left) || !placeReads(t, left) {
				continue
			}
			target, _ := c.exprText(t)
			name, _ := c.exprText(left)
			c.addErrorAt(t, messages.New(messages.H1160, target, name, c.splitAssignment(s, j)))
			return
		}
	}
}

// assignedPath hedef bir ad ya da yol mu ('_' atanmaz).
func assignedPath(e ast.Expression) bool {
	switch x := e.(type) {
	case *ast.Identifier:
		return x.Value != "_"
	case *ast.MemberExpression, *ast.IndexExpression:
		return true
	}
	return false
}

// placeReads hedefin yerini hesaplayan ifadeler (indeksin kabı ve indeksi, alanın sahibi) path'i
// okuyor mu: yazımın anlamı (konumsuz döküm) aynı olan bir alt ifade.
func placeReads(target, path ast.Expression) bool {
	var parts []ast.Expression
	switch x := target.(type) {
	case *ast.IndexExpression:
		parts = []ast.Expression{x.Left, x.Index}
	case *ast.MemberExpression:
		parts = []ast.Expression{x.Object}
	default:
		return false
	}
	want := ast.Dump(path)
	found := false
	for _, p := range parts {
		ast.Walk(p, func(n any) bool {
			switch e := n.(type) {
			case *ast.Identifier, *ast.MemberExpression, *ast.IndexExpression:
				found = found || ast.Dump(e.(ast.Node)) == want
			}
			return !found
		})
	}
	return found
}

// splitAssignment iletinin önerisi: atama j. hedeften önce ikiye ayrılır ('i = 1; l[i] = 5'). Öneri
// yalnız değerler hedeflerle bire bir yazılmışsa ve ikinci yarının değerleri ilk yarıda atanan bir
// adı ya da yolu okumuyorsa verilir (yoksa ayırmak değerlerin anlamını da değiştirirdi); boş metin
// iletinin genel biçimini seçer.
func (c *Checker) splitAssignment(s *ast.MultiAssignStatement, j int) string {
	if len(s.Values) != len(s.Targets) {
		return ""
	}
	for _, v := range s.Values[j:] {
		for _, t := range s.Targets[:j] {
			if assignedPath(t) && reads(v, t) {
				return ""
			}
		}
	}
	half := func(targets, values []ast.Expression) (string, bool) {
		var ts, vs []string
		for k := range targets {
			t, ok1 := c.exprText(targets[k])
			v, ok2 := c.exprText(values[k])
			if !ok1 || !ok2 {
				return "", false
			}
			ts, vs = append(ts, t), append(vs, v)
		}
		return strings.Join(ts, ", ") + " = " + strings.Join(vs, ", "), true
	}
	first, ok1 := half(s.Targets[:j], s.Values[:j])
	second, ok2 := half(s.Targets[j:], s.Values[j:])
	if !ok1 || !ok2 {
		return ""
	}
	return first + "; " + second
}

// reads e'nin path'i okuyan bir alt ifadesi var mı.
func reads(e, path ast.Expression) bool {
	want := ast.Dump(path)
	found := false
	ast.Walk(e, func(n any) bool {
		switch x := n.(type) {
		case *ast.Identifier, *ast.MemberExpression, *ast.IndexExpression:
			found = found || ast.Dump(x.(ast.Node)) == want
		}
		return !found
	})
	return found
}

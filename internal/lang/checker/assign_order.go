package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Çoklu atamanın sırası (5.3): önce hedeflerin yerleri (indeks, alanın sahibi; 5.4) soldan sağa,
// sonra sağdaki ifadelerin hepsi, sonra yazmalar soldan sağa. Bir hedefin yeri aynı atamada
// solunda atanan bir adı ya da yolu okuyorsa ('i, l[i] = 1, 5'; 'a, a.b = x, y') yer eski değerle
// hesaplanır, Python'da yeniyle: sessizce ayrılırdı, denetim hatasıdır (H1160). Kap öğesinde indeks
// fark etmez (DENETİM 393): solda 'p[a]'ya yazılıyorsa sağdaki yer 'p[...]'yi okuyamaz
// ('p[a], p[p[b]] = 2, 9'; a == b iken birebir yol karşılaştırması yakalamazdı). O öğenin içine
// yazmak ('p[a][0]') 'p[...]'yi okumayı yasaklamaz: 'p[a][0], p[b][1] = 1, 2' geçerlidir. Sağında
// atanan adı okumak serbesttir, iki sıra aynı sonucu verir. Ad ve yol düzeyindedir: takma adla aynı
// nesneyi gösteren iki ad ve yerdeki çağrının okuduğu ad ('i, l[f()]') yakalanmaz, orada kural
// Go'nunkidir.

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
// okuyor mu (samePlace).
func placeReads(target, path ast.Expression) bool {
	switch x := target.(type) {
	case *ast.IndexExpression:
		return reads(x.Left, path) || reads(x.Index, path)
	case *ast.MemberExpression:
		return reads(x.Object, path)
	}
	return false
}

// reads e'nin path'in yerine bakan bir alt ifadesi var mı (samePlace).
func reads(e, path ast.Expression) bool {
	found := false
	ast.Walk(e, func(n any) bool {
		if x, ok := n.(ast.Expression); ok && samePlace(x, path) {
			found = true
		}
		return !found
	})
	return found
}

// samePlace e, path'in yazdığı yere bakıyor mu: ad aynı addır; alan aynı alanın adıyla aynı yere
// bakan sahiptir; indekste kap aynı yere bakar, indeks fark etmez (p[b], p[a]'nın yerine bakabilir).
// Öbür ifadeler konumsuz dökümleri aynıysa.
func samePlace(e, path ast.Expression) bool {
	switch p := path.(type) {
	case *ast.Identifier:
		x, ok := e.(*ast.Identifier)
		return ok && x.Value == p.Value
	case *ast.MemberExpression:
		x, ok := e.(*ast.MemberExpression)
		return ok && x.Property == p.Property && samePlace(x.Object, p.Object)
	case *ast.IndexExpression:
		x, ok := e.(*ast.IndexExpression)
		return ok && samePlace(x.Left, p.Left)
	}
	return ast.Dump(e) == ast.Dump(path)
}

// splitAssignment iletinin önerisi: atama j. hedeften önce ikiye ayrılır ('i = 1; l[i] = 5'). Öneri
// yalnız değerler hedeflerle bire bir yazılmışsa ve ikinci yarının değerleri ilk yarıda atanan bir
// adın ya da yolun yerine bakmıyorsa verilir (yoksa ayırmak değerlerin anlamını da değiştirirdi); boş
// metin iletinin genel biçimini seçer.
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

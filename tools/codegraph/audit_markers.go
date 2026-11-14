package main

import (
	"go/ast"

	"github.com/sekerahmet/TCB/internal/devtools/goload"
)

// İşaret arayüzü: değeri atılan bir tip iddiasında (`_, ok := x.(I)`; `if _, copies := left.(AssignCopier); !copies`)
// ya da bağladığı değişkeni o kolda kullanılmayan bir tip seçimi kolunda (`case I:`) geçen modül arayüzü. Program
// metodu çağırmadan yalnız tipin arayüzü sağlayıp sağlamadığına bakıp yol seçer: metot hiç çağrılmaz ama silinirse
// tip arayüzü sağlamaz ve program sessizce öbür yoldan gider (interp.AssignCopier, ml.TensorValue.CopiesAssigned).
// Onu sağlayan metotlar bu yüzden assertion-only sayılmaz. Değeri bağlanan iddia (`r, ok := x.(I)`) işaret
// değildir: bağlanan değerin metodu çağrılmıyorsa metot gerçekten ölü olabilir (RawBackend.FillRaw). Yalnız test
// dışı dosyalar sayılır: testteki bir iddia üretimde yol seçmez.

// collectMarkers modülün işaret arayüzlerini tip anahtarıyla toplar.
func (a *methodAudit) collectMarkers() {
	a.markers = map[string]bool{}
	forEachFile(a.m, func(u *goload.Unit, f *ast.File, rel string) {
		if isTestFile(rel) {
			return
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if len(s.Rhs) == 1 && isBlankIdent(s.Lhs[0]) {
					a.markAsserted(u, s.Rhs[0])
				}
			case *ast.ValueSpec:
				if len(s.Values) == 1 && isBlankIdent(s.Names[0]) {
					a.markAsserted(u, s.Values[0])
				}
			case *ast.TypeSwitchStmt:
				a.markSwitch(u, s)
			}
			return true
		})
	})
}

// markAsserted e bir tip iddiasıysa (`x.(I)`) I'yi işaret sayar.
func (a *methodAudit) markAsserted(u *goload.Unit, e ast.Expr) {
	if ta, ok := ast.Unparen(e).(*ast.TypeAssertExpr); ok && ta.Type != nil {
		a.markIface(u, ta.Type)
	}
}

// markSwitch tip seçiminin bağlanan değişkeni kullanmayan kollarındaki arayüzleri işaret sayar (`switch x.(type)`
// ya da değişkenin o kolda geçmediği `switch v := x.(type)`).
// Her kolun bağlanan değişkeni ayrı bir nesnedir ve konumu başlıktaki tanımlayıcınınkidir (go/types; goload
// Info.Implicits'i doldurmaz): kol, Uses'ı o konumu taşıyan bir tanımlayıcıyla değişkeni kullanır.
func (a *methodAudit) markSwitch(u *goload.Unit, s *ast.TypeSwitchStmt) {
	var bound *ast.Ident
	if as, ok := s.Assign.(*ast.AssignStmt); ok && len(as.Lhs) == 1 {
		bound, _ = as.Lhs[0].(*ast.Ident)
	}
	for _, c := range s.Body.List {
		cc := c.(*ast.CaseClause)
		if bound != nil {
			used := false
			ast.Inspect(cc, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == bound.Name && u.Info.Uses[id] != nil && u.Info.Uses[id].Pos() == bound.Pos() {
					used = true
				}
				return !used
			})
			if used {
				continue // kol bağlanan değeri kullanıyor: metodunu çağırabilir
			}
		}
		for _, e := range cc.List {
			a.markIface(u, e)
		}
	}
}

// markIface tip ifadesi bir modül arayüzüyse onu işaret sayar.
func (a *methodAudit) markIface(u *goload.Unit, e ast.Expr) {
	if i := a.ifaceOfType(u, e); i != nil {
		a.markers[typeKey(i)] = true
	}
}

// satisfiesMarker anahtarı verilen yöntem bir işaret arayüzünün yöntemini karşılıyor mu.
func (a *methodAudit) satisfiesMarker(key string) bool {
	for _, s := range a.satisfies[key] {
		if a.markers[typeKey(s.iface)] {
			return true
		}
	}
	return false
}

// isBlankIdent ifade boş tanımlayıcı (`_`) mı.
func isBlankIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}

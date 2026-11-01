package checker

import (
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Modül değeri (11.2, 11.8): TR_import "yol" TR_as m ile bağlanan ad. Denetleyici modülün adlarının
// tipini bugün bilmez (modüller arası tip denetimi, 4b), ama m'nin bir modül değeri olduğunu bilir:
// modül değerinin adına atama (m.PI = 7) ve köşeli ayraçla erişim (m["PI"]) çalışma anının koduyla
// denetim hatasıdır (13.11'in emsali). Değerin içi değişebilir: m.yer.x = 1, m.kayit.TR_append(x).

// defineModuleAlias takma adlı aktarımın adını kapsamda modül değeri olarak tanımlar.
func (c *Checker) defineModuleAlias(s *ast.ImportStatement) {
	if s.Alias != "" {
		c.scope.Define(s.Alias, TypeInfo{TypeName: typeAny, Module: true})
	}
}

// moduleValue e, kapsamda bir modül değerine çözülen ad mı (yerel bir ad onu gölgeleyebilir).
func (c *Checker) moduleValue(e ast.Expression) (string, bool) {
	id, ok := e.(*ast.Identifier)
	if !ok {
		return "", false
	}
	info, found := c.scope.Resolve(id.Value)
	return id.Value, found && info.Module
}

// checkModuleAssign modül değerinin adına atamayı (m.PI = 7, m.sayac += 1) bildirir (11.8, H2901).
func (c *Checker) checkModuleAssign(target ast.Expression) {
	m, ok := target.(*ast.MemberExpression)
	if !ok || m.Safe {
		return
	}
	if _, isModule := c.moduleValue(m.Object); isModule {
		c.addErrorAt(target, messages.New(messages.H2901, m.Property))
	}
}

// checkModuleIndex modül değerinin köşeli ayraçla okunmasını ya da atanmasını bildirir (11.2, H2904):
// ileti noktayla erişimi önerir (m["PI"] → m.PI). İndeks ad gibi yazılmış bir metin değilse (m[k],
// m["a b"]) önerilecek ad bilinmez; hata çalışma anında kalır.
func (c *Checker) checkModuleIndex(e *ast.IndexExpression) {
	module, isModule := c.moduleValue(e.Left)
	s, isText := e.Index.(*ast.StringLiteral)
	if isModule && isText && nameLike(s.Value) {
		c.addErrorAt(e, messages.New(messages.H2904, module, s.Value))
	}
}

// nameLike s bir ad gibi mi yazılmış: harf ya da alt çizgiyle başlar, harf, rakam ve alt çizgiyle sürer.
func nameLike(s string) bool {
	for i, r := range s {
		if !(r == '_' || unicode.IsLetter(r) || i > 0 && unicode.IsDigit(r)) {
			return false
		}
	}
	return s != ""
}

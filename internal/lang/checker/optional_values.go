package checker

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Açılmamış isteğe bağlı değer (3.5): T? tipli bir ifade, isteğe bağlı olmayan T beklenen yere
// verilemez; '??' ile varsayılan verilerek, '?.' ile ya da TR_none'la karşılaştırılarak açılır. Bu
// dosya kuralın adı olmayan ifadeler için olan kısmıdır: l.TR_first, l.TR_get(5), l[i]
// (TR_List[T?]). Adın daraltması (TR_if x != TR_none { ... }) 4b'nin işidir; o gelene kadar ad
// tabandaki gibi geçer. Kural assignable'ın her yerinde (tanım, atama, dönüş, argüman, kurucu,
// alan, yerleşiğin argümanı, .TR_append) ve liste, sözlük, demet değişmezinin öğelerinde geçer; öğesi
// isteğe bağlı olmayan kabın indeksine atama H1135'tir.

// itemType bir değişmezin öğesinin tipini çıkarır ve kaydeder: kuralın değişmezin içine bakması
// öğeyi yeniden çıkarmadan (hatası iki kez yazılmadan) olur.
func (c *Checker) itemType(e ast.Expression) string {
	t := c.inferExpression(e)
	if c.itemTypes == nil {
		c.itemTypes = map[ast.Expression]string{}
	}
	c.itemTypes[e] = t
	return t
}

// requiredPlace beklenen tip isteğe bağlı olmayan, anlamı bilinen bir tip mi: TR_Any, yokluk ve
// denetleyicinin bilmediği ad (tip parametresi, arayüz; isteğe bağlı olabilir) değildir.
func (c *Checker) requiredPlace(expected string) bool {
	e := shapeOf(expected)
	if e.optional {
		return false
	}
	if e.elems != nil {
		return true
	}
	base := normalizeTypeName(e.base)
	return base != typeAny && base != typeNull && !(compat{isStruct: c.knownStruct}).unknownName(base)
}

// unopened, expected beklenen yere verilen value'nun (tipi actual) ya da bir değişmezse içindeki
// bir öğenin açılmamış isteğe bağlı değer olduğunu söyler; döndürdüğü ifade o değer, tip onun
// beklendiği tiptir. Ad (*ast.Identifier) 4b'ye kalır.
func (c *Checker) unopened(expected, actual string, value ast.Expression) (ast.Expression, string) {
	if value == nil || !c.requiredPlace(expected) {
		return nil, ""
	}
	if _, isName := value.(*ast.Identifier); !isName && shapeOf(actual).optional {
		return value, expected
	}
	e := shapeOf(expected)
	switch v := value.(type) {
	case *ast.ListLiteral:
		if normalizeTypeName(e.base) == typeList && len(e.args) == 1 {
			for _, el := range v.Elements {
				if bad, t := c.unopened(e.args[0], c.itemTypes[el], el); bad != nil {
					return bad, t
				}
			}
		}
	case *ast.DictLiteral:
		if normalizeTypeName(e.base) == typeDict && len(e.args) == 2 {
			for _, p := range v.Pairs {
				if bad, t := c.unopened(e.args[0], c.itemTypes[p.Key], p.Key); bad != nil {
					return bad, t
				}
				if bad, t := c.unopened(e.args[1], c.itemTypes[p.Value], p.Value); bad != nil {
					return bad, t
				}
			}
		}
	case *ast.TupleExpression:
		if len(e.elems) == len(v.Elements) {
			for i, el := range v.Elements {
				if bad, t := c.unopened(e.elems[i], c.itemTypes[el], el); bad != nil {
					return bad, t
				}
			}
		}
	}
	return nil, ""
}

// typeHint tip uyuşmazlığının ipucudur: bölmenin (H1008) ya da açılmamış değerin (H1130).
func (c *Checker) typeHint(expected, given string, value ast.Expression, divides bool) any {
	if h := c.divisionHint(expected, given, divides); h != "" {
		return h
	}
	return c.openHint(expected, given, value)
}

// openHint H1130'dur: açılmamış değeri ve onu açan yazımı programın kendi yazımıyla söyler
// (l.TR_first ?? 0). Değerin dosyadaki yazımı bilinmiyorsa ipucu yoktur.
func (c *Checker) openHint(expected, actual string, value ast.Expression) any {
	return c.openHintAs("", expected, actual, value)
}

// openHintAs openHint'tir; kind boşsa iletinin select anahtarı beklenen tipten gelir.
func (c *Checker) openHintAs(kind, expected, actual string, value ast.Expression) any {
	bad, want := c.unopened(expected, actual, value)
	if bad == nil {
		return ""
	}
	text, ok := c.exprText(bad)
	if !ok {
		return ""
	}
	given := actual
	if bad != value {
		given = c.itemTypes[bad]
	}
	if kind == "" {
		kind = defaultKind(want)
	}
	return messages.New(messages.H1130, text, c.typeArg(given), kind)
}

// defaultKind '??'nin varsayılanı yazılabilen tipin anahtarıdır (iletinin select'i); öbür tiplerde
// (mantık değeri: kod yuvası yalnız her dalda geçebilir; sözlük: dalın içinde '}' dalı kapatır; tensör, yapı, demet) "other": ileti varsayılanı yazmaz.
func defaultKind(t string) string {
	switch normalizeTypeName(baseType(t)) {
	case typeInt:
		return "int"
	case typeFloat:
		return "float"
	case typeString:
		return "string"
	case typeList:
		return "list"
	}
	return "other"
}

// coalesceType 'a ?? b'nin tipini verir ve iki yanını denetler (4.7). Sol yan T? ise sağ yanın tipi
// tam T ya da tam T?'dir: tam sayı TR_Float'ın, değişmez TR_JSON'ın yerine geçmez, çünkü '??'
// çalışma anında çevirmez; öğesiz [] ve {} beklenen kabın yerine geçer (compat); TR_none hiçbir şeyi
// açmaz (H1030). Sonuç b T ise T, T? ise T?; sağ yanın '?'i uyumdan değil yazılı tipinden okunur,
// çünkü T? tipli bir ad 4b'ye kadar T yerine geçer. Sol yan tipi bilinen ve isteğe bağlı olmayan bir
// tipse sağ yan hiç kullanılmaz (H1031); tipi bilinmeyen ad (arayüz, başka modülün tipi) isteğe
// bağlı olabilir, hata vermez. Sol yan TR_Any ya da TR_none ise sonuç sağ yanın tipidir.
func (c *Checker) coalesceType(e *ast.InfixExpression, left, right string) string {
	if left == typeNull || left == typeAny {
		return right
	}
	if !shapeOf(left).optional {
		if c.requiredPlace(left) {
			value, _ := c.exprText(e.Left)
			c.addErrorAt(e.Left, messages.New(messages.H1031, c.typeArg(left), value))
		}
		return left
	}
	t := requiredType(left)
	if right == typeAny {
		return t
	}
	want := t
	if shapeOf(right).optional {
		want = left
	}
	if right == typeNull || !(compat{isStruct: c.knownStruct, exact: true}).types(want, right) {
		c.addErrorAt(e.Right, messages.New(messages.H1030, c.typeArg(right), c.typeArg(left), c.typeArg(t), c.coalesceHint(e, t)))
		return t
	}
	return want
}

// coalesceHint H1030'un önerisidir, sol yanın asıl tipinden (t): TR_JSON'da sağ yanı JSON değerine
// çeviren H1033, öbür tiplerde varsayılanı yazan H1032. Sol ya da (JSON'da) sağ yanın dosyadaki
// yazımı bilinmiyorsa öneri yoktur; ana ileti yine verilir.
func (c *Checker) coalesceHint(e *ast.InfixExpression, t string) any {
	value, ok := c.exprText(e.Left)
	if !ok {
		return ""
	}
	if normalizeTypeName(baseType(t)) == typeJSON {
		right, ok := c.exprText(e.Right)
		if !ok {
			return ""
		}
		return messages.New(messages.H1033, value, right)
	}
	return messages.New(messages.H1032, value, coalesceKind(t))
}

// coalesceKind H1032'nin select anahtarıdır, '??'nin sol yanının asıl tipinden: varsayılanı yazılan
// tiplerde o tip, TR_Bool'da "other" (kod yuvası {#false} yalnız other dalında olabilir), öbür
// tiplerde (sözlük: dalın içinde '}' dalı kapatır; yapı, demet, tensör, bilinmeyen ad) "generic".
func coalesceKind(t string) string {
	switch normalizeTypeName(baseType(t)) {
	case typeInt:
		return "int"
	case typeFloat:
		return "float"
	case typeString:
		return "string"
	case typeList:
		return "list"
	case typeBool:
		return "other"
	}
	return "generic"
}

// checkIndexAssign öğesi isteğe bağlı olmayan kabın indeksine açılmamış isteğe bağlı değer
// atanmasını denetler (3.5, H1135): m[0] = l.TR_first. Öğe tipinin genel denetimi (yanlış tipli
// değer, 3.6) burada yoktur.
func (c *Checker) checkIndexAssign(s *ast.AssignStatement, valType string) {
	ix, ok := s.Target.(*ast.IndexExpression)
	if !ok || string(s.Operator) != "=" {
		return
	}
	container := c.itemTypes[ix.Left] // inferNested kaydeder
	shape := shapeOf(container)
	elem := ""
	switch {
	case shape.optional:
		return
	case normalizeTypeName(shape.base) == typeList && len(shape.args) == 1:
		elem = shape.args[0]
	case normalizeTypeName(shape.base) == typeDict && len(shape.args) == 2:
		elem = shape.args[1]
	default:
		return
	}
	if bad, _ := c.unopened(elem, valType, s.Value); bad != nil {
		c.addErrorAt(s.Value, messages.New(messages.H1135, c.typeArg(valType), c.typeArg(container), c.typeArg(elem), c.openHint(elem, valType, s.Value)))
	}
}

// Ad olmayan açılmamış değerin kullanımı (3.5, H1180): T? tipli bir ifadenin indeksi, alanı (okuma
// ve yazma) ve üyesi (özellik, metot çağrısının alıcısı) asıl tipin kullanımıdır. Kullanım çıkarımda
// kaydedilir, deyim bitince bildirilir: ipucu deyimin metnini yazar (H1181, H1182), 5.4'ün aynı
// adımda verdiği ileti de (H2741, H2740) onun yerine geçer, çünkü onun önerisi ikisini birden
// düzeltir. Ad (*ast.Identifier) 4b'ye kalır; '?.' değeri açar.

// unopenedUse use'un (indeks, alan ya da metot çağrısı) value'su açılmamış isteğe bağlıdır; typ
// value'nun tipi, op iletinin select anahtarıdır (index, field, member).
type unopenedUse struct {
	use, value ast.Expression
	typ, op    string
}

// unopenedUses deyim bitmeden kaydedilen kullanımlar ve 5.4'ün bildirdiği adımlardır.
type unopenedUses struct {
	pending []unopenedUse
	copied  map[ast.Expression]bool
}

// noteUnopened u'nun değeri ad olmayan açılmamış isteğe bağlıysa kaydeder.
func (c *Checker) noteUnopened(u unopenedUse) {
	if _, isName := u.value.(*ast.Identifier); !isName && shapeOf(u.typ).optional {
		c.opt.pending = append(c.opt.pending, u)
	}
}

// noteMember mem'in nesnesinin (tipi typ) kullanımını kaydeder; '?.' açar. op boşsa yapının alanı
// "field", öbür üye "member"dır.
func (c *Checker) noteMember(use ast.Expression, mem *ast.MemberExpression, typ, op string) {
	if mem.Safe {
		return
	}
	if op == "" {
		op = "member"
		if c.knownStruct(shapeOf(typ).base) {
			op = "field"
		}
	}
	c.noteUnopened(unopenedUse{use, mem.Object, typ, op})
}

// noteCopyStep 5.4'ün bildirdiği adımı kaydeder: orada H1180 verilmez.
func (c *Checker) noteCopyStep(e ast.Expression) {
	if c.opt.copied == nil {
		c.opt.copied = map[ast.Expression]bool{}
	}
	c.opt.copied[e] = true
}

// reportUnopened stmt'nin içinde kaydedilen kullanımları (mark'tan sonrakiler) bildirir; içteki
// deyimler kendi kullanımlarını önce bildirmiştir.
func (c *Checker) reportUnopened(stmt ast.Statement, mark int) {
	uses := append([]unopenedUse(nil), c.opt.pending[mark:]...)
	c.opt.pending = c.opt.pending[:mark]
	seen := map[ast.Expression]bool{}
	for _, u := range uses {
		if seen[u.value] || c.opt.copied[u.value] {
			continue
		}
		seen[u.value] = true
		value, ok := c.exprText(u.value)
		if !ok {
			value = c.nodeText(u.value)
		}
		c.addErrorAt(u.value, messages.New(messages.H1180, value, c.typeArg(u.typ), u.op, c.unopenedHint(stmt, u)))
	}
}

// unopenedHint H1180'in ipucudur: kabın öğesini veren üyede kabın indeksi (H1181), öbür değerde
// değeri yerel ada alıp sınamak (H1182). İfade deyimi, atama, tanım ve dönüş bütünüyle yeniden
// yazılır; tanımda sınama erken çıkıştır (TR_none iken ifadenin metni fırlatılır, tanımlanan ad
// sonraki satırlarda görünür; 3.5'in daraltması, TCB 2026-11-15); öbür deyimde (koşul, döngü başı: bloğu vardır) H1181 yalnız kullanımı yazar, H1182'nin
// sınaması "..." sarar, çünkü kullanım tek başına deyim olamaz (2.8). Deyim ya da kullanım tek
// satıra sığmıyorsa ipucu yoktur.
func (c *Checker) unopenedHint(stmt ast.Statement, u unopenedUse) any {
	var part ast.Node = u.use
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		if !blockExpression(s.Expression) {
			part = stmt
		}
	case *ast.AssignStatement, *ast.MultiAssignStatement, *ast.VarStatement, *ast.MultiVarStatement,
		*ast.ReturnStatement:
		part = stmt
	}
	if step := (ast.WriteStep{Expr: u.value}); c.containerAccessor(step) {
		if d := accessorDirect(c.fileLines, part, part, step); d != "..." {
			return messages.New(messages.H1181, d)
		}
		return ""
	}
	value, ok1 := c.exprText(u.value)
	name := ast.FreshName(c.fileLines, "c", func(n string) bool { _, ok := c.scope.Resolve(n); return ok })
	s, ok2 := "...", true
	if part == stmt {
		s, ok2 = ast.StatementWith(c.fileLines, part, part, u.value, name)
	}
	if !ok1 || !ok2 {
		return ""
	}
	switch stmt.(type) {
	case *ast.VarStatement, *ast.MultiVarStatement: // erken çıkış: tanımlanan ad sonraki satırlarda görünür
		return messages.New(messages.H1182, name, value, trStringLiteral(value), "guard", "; "+s)
	}
	return messages.New(messages.H1182, name, value, s, "test", "")
}

// trStringLiteral metni TR'nin dizgi yazımıyla verir (2.6: ters bölü ve tırnak kaçar, 2.7'nin
// yerleştirmesi de ters bölüyle başladığı için kaçmış olur); H1182'nin
// erken çıkışı ifadenin metnini fırlatır.
func trStringLiteral(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// blockExpression ifade deyim olarak yazılan bloklu ifade mi (TR_if, TR_match): bütünü öneriye girmez.
func blockExpression(e ast.Expression) bool {
	switch e.(type) {
	case *ast.IfExpression, *ast.MatchExpression:
		return true
	}
	return false
}

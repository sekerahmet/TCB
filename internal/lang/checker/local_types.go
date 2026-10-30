package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Yerel tip çıkarımı (1.2, 3.13): fonksiyon parametrelerinin ve dönüşünün tipini yazar, yerel
// değişkenin tipi ilk değerinden çıkar ve değişmez. Denetleyici bir tipi ancak tanıyorsa
// (çekirdek tip, programın yapısı) değişkene verir; tanımadığı tip (tip parametresi T, arayüz, başka modülün tipi)
// ve bilinemeyen değer TR_Any kalır: yarım çıkarım yanlış alarm vermez.

// coreTypes denetleyicinin anlamını bildiği tip adlarıdır (3.11'in çekirdek tipleri, TR_NoneType hariç).
var coreTypes = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range annotationTypes {
		if k != codes.TypeNoneType {
			m[codes.Name(k)] = true
		}
	}
	return m
}()

// knownType denetleyici t tipini tanıyor mu: değişkene verilebilir bir tip mi? Çekirdek tipler,
// programın yapıları, öğe tipleri tanınan kaplar ve demetler.
func (c *Checker) knownType(t string) bool {
	s := shapeOf(t)
	if s.elems == nil && !coreTypes[s.base] && !c.knownStruct(s.base) {
		return false
	}
	for _, a := range append(s.args, s.elems...) {
		if !c.knownType(a) {
			return false
		}
	}
	return true
}

// commonType değişmezdeki öğelerin ortak tipidir (3.6): hepsi aynı tanınan tipse o; tam sayı ve
// ondalık karışıksa ve her tam sayı öğe bir tam sayı yazımıysa TR_Float (3.13: yazım ondalık olur);
// öğelerden bazıları TR_none ise öbürlerinin ortak tipinin isteğe bağlısı (T?, 3.5); boşsa,
// karışıksa (tam sayı değişkenle ondalık da) ya da bir öğenin tipi bilinmiyorsa "".
func (c *Checker) commonType(types []string, exprs []ast.Expression) string {
	var rest []string
	var restExprs []ast.Expression
	for i, t := range types {
		if t != typeNull {
			rest, restExprs = append(rest, t), append(restExprs, exprs[i])
		}
	}
	if len(rest) < len(types) {
		if t := c.commonType(rest, restExprs); t != "" {
			return optionalType(t)
		}
		return ""
	}
	if len(types) == 0 {
		return ""
	}
	t := types[0]
	for _, u := range types[1:] {
		switch {
		case u == t:
		case numericType(t) && numericType(u):
			t = typeFloat
		default:
			return ""
		}
	}
	if t == typeFloat {
		for i, u := range types {
			if u == typeInt && !integerSpelling(exprs[i]) {
				return ""
			}
		}
	}
	if !c.knownType(t) {
		return ""
	}
	return t
}

// listLiteralType liste değişmezinin tipidir: öğelerin ortak tipi varsa TR_List[T], yoksa
// (boş, karışık, bilinmeyen öğe) öğe tipsiz TR_List.
func (c *Checker) listLiteralType(elems []string, exprs []ast.Expression) string {
	if t := c.commonType(elems, exprs); t != "" {
		return containerType(typeList, []string{t})
	}
	return typeList
}

// dictLiteralType sözlük değişmezinin tipidir (3.7): anahtarların ve değerlerin ortak tipi varsa
// TR_Dict[K, V], yoksa öğe tipsiz TR_Dict.
func (c *Checker) dictLiteralType(keys, values []string, keyExprs, valueExprs []ast.Expression) string {
	k, v := c.commonType(keys, keyExprs), c.commonType(values, valueExprs)
	if k != "" && v != "" {
		return containerType(typeDict, []string{k, v})
	}
	return typeDict
}

// integerSpelling e'nin sayı olarak yazılmış bir tam sayı olduğunu söyler (3, -3): 3.13'e göre
// ondalıkların yanında TR_Float olan yazımdır; tam sayı değişkeni öyle değildir.
func integerSpelling(e ast.Expression) bool {
	if p, ok := e.(*ast.PrefixExpression); ok && p.Operator == "-" {
		e = p.Right
	}
	_, ok := e.(*ast.IntegerLiteral)
	return ok
}

// localType açıklamasız değişkenin ilk değerinden çıkan kalıcı tipidir: tanınan tip olduğu gibi,
// öbürü (bilinmeyen, TR_none) TR_Any.
func (c *Checker) localType(inferred string) string {
	if c.knownType(inferred) {
		return inferred
	}
	return typeAny
}

// signType önek '-' ve '+'nın sonucunun tipidir: sayı ve tensör kendi tipinde kalır, başka
// işlenenin hatası çalışma anındadır (denetleyici tip iddia etmez).
func signType(operand string) string {
	if numericType(operand) || operand == typeTensor {
		return operand
	}
	return typeAny
}

// sequenceType metnin ve listenin '+' ve '*' sonucudur (4.10): iki liste uç uca eklenir, metin ve
// liste bir TR_Int kadar yinelenir. Öğe tipleri ayrı iki listenin toplamı öğe tipsizdir.
func sequenceType(op, left, right string) (string, bool) {
	switch {
	case op == "*" && (left == typeString && right == typeInt || left == typeInt && right == typeString):
		return typeString, true
	case op == "*" && baseType(left) == typeList && right == typeInt:
		return left, true
	case op == "*" && left == typeInt && baseType(right) == typeList:
		return right, true
	case op == "+" && baseType(left) == typeList && baseType(right) == typeList:
		if left == right {
			return left, true
		}
		return typeList, true
	}
	return "", false
}

// tensorArithmetic işlem bir tensörün öğe öğe aritmetiği mi (15.6): bir yan tensör, öbür yan tensör
// ya da sayı. Sonuç bir tensördür.
func tensorArithmetic(op, left, right string) bool {
	switch op {
	case "+", "-", "*", "/", "//", "%", "**":
	default:
		return false
	}
	return left == typeTensor && (right == typeTensor || numericType(right)) ||
		right == typeTensor && numericType(left)
}

// defineStruct yapıyı kaydeder: kurucusunun çağrısı yapının tipini, metodunun çağrısı metodun
// yazılan dönüş tipini verir (callType). Durumlu tip ve arayüz kurulmaz; kaydedilmez.
func (c *Checker) defineStruct(s *ast.StructStatement) {
	if s.Kind != ast.KindStruct {
		return
	}
	if c.structs == nil {
		c.structs = map[string]*ast.StructStatement{}
	}
	c.structs[s.Name] = s
}

// knownStruct ad denetleyicinin bildiği bir yapının adı mı?
func (c *Checker) knownStruct(name string) bool {
	return c.structs[name] != nil
}

// callType çağrının tipidir: adıyla çağrılan tipli fonksiyonun dönüş tipi, yapı kurucusunun
// çağrısında yapının kendisi (Nokta(1, 2) → Nokta), tipi bilinen bir yapı nesnesinin metodunda
// metodun yazılan dönüş tipi (objType nesnenin tipidir). Öbür çağrılar TR_Any'dir.
func (c *Checker) callType(e *ast.CallExpression, info TypeInfo, found bool, objType string) string {
	if found && info.TypeName == typeFunction && info.ReturnType != "" {
		return info.ReturnType
	}
	if ident, ok := e.Function.(*ast.Identifier); ok && found && info.TypeName == typeType && c.knownStruct(ident.Value) {
		return ident.Value
	}
	if m, ok := e.Function.(*ast.MemberExpression); ok && !m.Safe && c.knownStruct(objType) {
		if f := c.structs[objType].Methods[m.Property]; f != nil && f.ReturnType != nil {
			return c.localType(normalizedType(f.ReturnType))
		}
	}
	return typeAny
}

// fieldType tipi bilinen yapı nesnesinin alanının yazılan tipidir (p.x, TR_self.x; 7.5): denetleyicinin
// tanıdığı tipse o, değilse (metot, tipsiz alan, bilinmeyen nesne, ?.) TR_Any.
func (c *Checker) fieldType(objType string, e *ast.MemberExpression) string {
	s := c.structs[objType]
	if s == nil || e.Safe {
		return typeAny
	}
	for _, f := range s.Fields {
		if f.Name == e.Property && f.Type != nil {
			return c.localType(normalizedType(f.Type))
		}
	}
	return typeAny
}

// indexType indekslemenin tipidir (3.6, 3.7, 15.7): listenin tam sayıyla indeksi öğe tipini, aralıkla
// dilimi listenin kendi tipini verir; metinde ikisi de metin; sözlükte değer tipi; tensörde tensör.
// İsteğe bağlı kap, tipi bilinmeyen kap ya da indeks TR_Any'dir.
func indexType(left, index string) string {
	s := shapeOf(left)
	switch {
	case s.optional || s.elems != nil:
		return typeAny
	case left == typeTensor:
		return typeTensor
	case index == typeRange && (s.base == typeList || left == typeString):
		return left
	case index == typeInt && s.base == typeList && len(s.args) == 1:
		return s.args[0]
	case index == typeInt && left == typeString:
		return typeString
	case index != typeRange && s.base == typeDict && len(s.args) == 2:
		return s.args[1]
	}
	return typeAny
}

// tupleLiteralType demet değişmezinin tipidir (3.6a): (1, "a") → (TR_Int, TR_String). Bir öğesinin
// tipi bilinmiyorsa demet TR_Any'dir.
func (c *Checker) tupleLiteralType(elems []string) string {
	for _, t := range elems {
		if !c.knownType(t) {
			return typeAny
		}
	}
	return tupleType(elems)
}

// elementType demetin i. öğesinin tipidir (t.0); nesne tipi bilinen bir demet değilse ya da indeks
// sınır dışındaysa TR_Any.
func elementType(objType string, i int) string {
	s := shapeOf(objType)
	if s.elems == nil || i < 0 || i >= len(s.elems) {
		return typeAny
	}
	return s.elems[i]
}

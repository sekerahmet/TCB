package checker

import "github.com/sekerahmet/TCB/internal/lang/codes"

// Yerel tip çıkarımı (1.2, 3.13): fonksiyon parametrelerinin ve dönüşünün tipini yazar, yerel
// değişkenin tipi ilk değerinden çıkar ve değişmez. Denetleyici bir tipi ancak tanıyorsa
// (çekirdek tip) değişkene verir; tanımadığı tip (tip parametresi T, arayüz, başka modülün tipi)
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

// knownType denetleyici t tipini tanıyor mu: değişkene verilebilir bir tip mi?
func (c *Checker) knownType(t string) bool {
	return coreTypes[t]
}

// localType açıklamasız değişkenin ilk değerinden çıkan kalıcı tipidir: tanınan tip olduğu gibi,
// öbürü (bilinmeyen, TR_none) TR_Any.
func (c *Checker) localType(inferred string) string {
	if c.knownType(inferred) {
		return inferred
	}
	return typeAny
}

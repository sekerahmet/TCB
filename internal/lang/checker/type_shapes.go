package checker

// İki tipin uyumu: beklenen yere verilen tipte bir değer konabilir mi (3.13). Denetleyici yalnız
// tanıdığı tiplerde karar verir: çekirdek tipler ve programın yapıları. Arayüz, durumlu tip, tip
// parametresi ya da başka bir modülün tipi denetleyiciye yalnız bir addır; böyle bir ad başka bir
// kullanıcı tipiyle karşılaşınca uyumlu sayılır (yanlış alarm vermez).

// compat iki tipin uyumuna karar verir. isStruct adın denetleyicinin bildiği bir yapı olup
// olmadığını söyler (nil: hiçbir yapı bilinmiyor).
type compat struct {
	isStruct func(string) bool
}

func typesCompatible(expected, actual string) bool {
	return compat{}.types(expected, actual)
}

// assignable actual tipinde bir değerin beklenen tipin yerine verilip verilemeyeceğidir; programın
// yapılarını bilir.
func (c *Checker) assignable(expected, actual string) bool {
	return compat{isStruct: c.knownStruct}.types(expected, actual)
}

func (k compat) types(expected, actual string) bool {
	exp := normalizeTypeName(expected)
	act := normalizeTypeName(actual)
	if exp == act || exp == typeAny || act == typeAny || act == typeNull {
		return true
	}
	// Tam sayı Ondalık beklenen yere atanabilir
	if exp == typeFloat && act == typeInt {
		return true
	}
	// İki kullanıcı tipi: ikisi de bilinen yapıysa ayrıdır; biri arayüz ya da bilinmeyen bir tipse
	// değer onu karşılayabilir.
	return !coreTypes[exp] && exp != typeNull && !coreTypes[act] && !(k.structName(exp) && k.structName(act))
}

// structName ad bilinen bir yapının adı mı?
func (k compat) structName(name string) bool {
	return k.isStruct != nil && k.isStruct(name)
}

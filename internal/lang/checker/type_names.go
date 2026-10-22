package checker

import (
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Denetleyicinin kullandığı tip adları kod listesindedir (internal/lang/codes, A2xx).
var (
	typeInt      = codes.Name(codes.A200)
	typeFloat    = codes.Name(codes.A201)
	typeString   = codes.Name(codes.A202)
	typeBool     = codes.Name(codes.A203)
	typeNull     = codes.Name(codes.A204)
	typeList     = codes.Name(codes.A205)
	typeDict     = codes.Name(codes.A206)
	typeRange    = codes.Name(codes.A207)
	typeFunction = codes.Name(codes.A208)
	typeTensor   = codes.Name(codes.A212)
	typeAny      = codes.Name(codes.A213)
)

// typeNumber denetleyicinin iç tipidir: değer çalışma anında Tam da Ondalık da olabilir (ör.
// Tam / Tam: 6 / 3 = 2, 7 / 2 = 3.5). Kullanıcı açıklamasında yazılamaz; yalnızca Ondalık
// (Tam da Ondalığa atanabildiği için) ve tipsiz yerlere atanabilir.
var typeNumber = typeInt + " ya da " + typeFloat

// annotationTypes tip açıklamasında (değişken x: Tam) kullanılabilen tiplerin kodlarıdır:
// 3.11'deki çekirdek tip adları.
var annotationTypes = []codes.Code{codes.A200, codes.A201, codes.A202, codes.A203,
	codes.A205, codes.A206, codes.A207, codes.A208, codes.A209, codes.A212, codes.A204, codes.A215}

// typeSpellings tip açıklamasında kabul edilen yazımlar → tip adıdır. Yazımlar sözlükten
// gelir ve tektir (3.14): yalnızca tip adının tam yazımı (Tam, Ondalık); büyük/küçük harfi
// farklı bir yazım (tam, MANTIK) kabul edilmez. Matris 2 boyutlu tensördür; ayrı bir tip
// değildir.
var typeSpellings = func() map[string]string {
	m := map[string]string{}
	for _, k := range annotationTypes {
		for _, y := range codes.Spellings(k) {
			m[y] = codes.Name(k)
		}
	}
	return m
}()

// typeCaseSpellings kabul edilen yazımların Türkçe kuralla küçültülmüş biçimi → tip adıdır.
// Yalnızca ipucu içindir: yazım bir tip adından yalnızca büyük/küçük harfle ayrılıyorsa
// (tam, ONDALIK) denetleyici asıl yazımı söyler.
var typeCaseSpellings = func() map[string]string {
	m := map[string]string{}
	for y, name := range typeSpellings {
		m[turkish.Lower(y)] = name
	}
	return m
}()

// typeSuggestions kabul edilmeyen ama bir tip adına işaret eden yazımların (Türkçe
// karaktersiz biçim: ondalik; başka dillerden alışılmış adlar: int, str) küçük harfli biçimi → önerilecek tip
// adıdır. Bu yazımlar tip açıklamasında hata verir ve asıl adı önerir.
var typeSuggestions = func() map[string]string {
	m := map[string]string{}
	add := func(y, name string) {
		if _, valid := typeCaseSpellings[turkish.Lower(y)]; !valid {
			m[turkish.Lower(y)] = name
		}
	}
	for _, k := range annotationTypes {
		g, _ := codes.Active().Entry(k)
		add(turkish.ASCII(g.Name), g.Name)
		for _, e := range g.Foreign {
			add(e, g.Name)
		}
	}
	return m
}()

// resolveTypeAnnotation tip açıklamasındaki adı tek aramada çözer: asıl tip adını ve adın
// kabul edilmeyen bir yazım olup olmadığını verir. Kabul edilen yazımda code 0'dır; yalnızca
// büyük/küçük harfi farklı yazımda H1006 (asıl yazım söylenir), Türkçe karaktersiz biçimde ya da
// başka dillerden alışılmış adta (ondalik, int, str) H1007 (öneri söylenir). Tanınmayan adın
// kendisi döner. Kabul edilmeyen yazımda da önerilen tip varsayılır: aynı satırda bir de tip
// uyuşmazlığı çıkmasın.
func resolveTypeAnnotation(name string) (canonical string, code messages.Code) {
	if name == "" {
		return typeAny, 0
	}
	if canonical, valid := typeSpellings[name]; valid {
		return canonical, 0
	}
	lower := turkish.Lower(name)
	if canonical, ok := typeCaseSpellings[lower]; ok {
		return canonical, messages.H1006
	}
	if suggestion, ok := typeSuggestions[lower]; ok {
		return suggestion, messages.H1007
	}
	return name, 0
}

// checkedTypeName tip açıklamasındaki adı çözer ve kabul edilmeyen bir yazımsa (tam, ondalik,
// int) asıl adı öneren bir hata ekler; asıl tip adını döndürür.
func (c *Checker) checkedTypeName(name string, line int) string {
	canonical, code := resolveTypeAnnotation(name)
	if code != 0 {
		c.addError(line, 0, messages.New(code, name, canonical))
	}
	return canonical
}

// normalizeTypeName tip açıklamasındaki adı asıl tip adına çevirir (hata eklemeden).
func normalizeTypeName(t string) string {
	canonical, _ := resolveTypeAnnotation(t)
	return canonical
}

func typesCompatible(expected, actual string) bool {
	exp := normalizeTypeName(expected)
	act := normalizeTypeName(actual)
	if exp == typeAny || act == typeAny || act == typeNull {
		return true
	}
	// Tam sayı (ve Tam ya da Ondalık olabilen değer), Ondalık beklenen yere atanabilir
	if exp == typeFloat && (act == typeInt || act == typeNumber) {
		return true
	}
	return exp == act
}

// numberHint Tam beklenen yere Tam ya da Ondalık olabilen bir değer verildiğinde yol gösterir.
func numberHint(expected, given string) string {
	if normalizeTypeName(expected) == typeInt && given == typeNumber {
		return messages.Text(messages.H1008, typeFloat, typeInt, codes.DisplayName(codes.G210))
	}
	return ""
}

// numericType tip bir sayı tipi mi (Tam, Ondalık ya da ikisinden biri)?
func numericType(t string) bool {
	return t == typeInt || t == typeFloat || t == typeNumber
}

// arithmeticType bir aritmetik işlemin sonuç tipini çalışma zamanının kurallarıyla çıkarır
// (4.2, 4.3, 4.10): denetleyici çalışma zamanından farklı bir tip iddia etmez. Tam / Tam
// tam bölünmezse Ondalık verir, Tam ** Tam üs negatifse Ondalık verir; bu yüzden ikisi de
// "Tam ya da Ondalık"tır. tamUs, '**'in sağının negatif olmayan bir Tam sabiti olduğunu
// söyler (2 ** 3 her zaman Tam'dır).
func arithmeticType(op, left, right string, intExponent bool) string {
	if op == "+" && (left == typeString || right == typeString) {
		return typeString
	}
	if !numericType(left) || !numericType(right) {
		return typeAny
	}
	if left == typeFloat || right == typeFloat {
		return typeFloat
	}
	if op == "/" || (op == "**" && !(intExponent && left == typeInt)) {
		return typeNumber
	}
	if left == typeNumber || right == typeNumber {
		return typeNumber
	}
	return typeInt
}

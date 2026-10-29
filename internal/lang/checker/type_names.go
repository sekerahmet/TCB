package checker

import (
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Denetleyicinin kullandığı tip adları kod listesindedir (internal/lang/codes, type).
var (
	typeInt      = codes.Name(codes.TypeInt)
	typeFloat    = codes.Name(codes.TypeFloat)
	typeString   = codes.Name(codes.TypeString)
	typeBool     = codes.Name(codes.TypeBool)
	typeNull     = codes.Name(codes.TypeNoneType)
	typeList     = codes.Name(codes.TypeList)
	typeDict     = codes.Name(codes.TypeDict)
	typeRange    = codes.Name(codes.TypeRange)
	typeFunction = codes.Name(codes.TypeFunction)
	typeTensor   = codes.Name(codes.TypeTensor)
	typeAny      = codes.Name(codes.TypeAny)
)

// typeText denetleyicinin tip adıdır, ileti parametresi olarak (messages.Localized): tip adı
// iletinin dilinde yazılır (Tam → Int); kullanıcının sınıf adı olduğu gibi kalır. Denetleyici
// tipleri kanonik adla taşır (karşılaştırma için); ad yalnız iletide dile çevrilir.
type typeText string

func (t typeText) MessageText(lang string) string {
	if k, ok := codes.Find(codes.NamespaceType, string(t)); ok {
		return codes.NameRef(k).MessageText(lang)
	}
	return string(t)
}

// typeSpelling typeText'in dosyadaki hâlidir: tip adı dosyada yazıldığı gibi, kimlik yazımıyla
// (TR_Int); cümle iletinin dilindedir.
type typeSpelling string

func (t typeSpelling) MessageText(lang string) string {
	if k, ok := codes.Find(codes.NamespaceType, string(t)); ok {
		return codes.Spelling(k)
	}
	return string(t)
}

// typeArg denetleyicinin t tipinin ileti parametresidir: dosyada kimlik yazımıyla (typeSpelling),
// yazılan satırda iletinin dilinde (typeText).
func (c *Checker) typeArg(t string) messages.Localized {
	if c.language() == nil {
		return typeSpelling(t)
	}
	return typeText(t)
}

// annotationTypes tip açıklamasında (değişken x: Tam) kullanılabilen tiplerin kodlarıdır:
// 3.11'deki çekirdek tip adları.
var annotationTypes = []codes.Code{codes.TypeInt, codes.TypeFloat, codes.TypeString, codes.TypeBool,
	codes.TypeList, codes.TypeDict, codes.TypeRange, codes.TypeFunction, codes.TypeBuiltinFunction, codes.TypeTensor, codes.TypeNoneType, codes.TypeType}

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

// typeHints bir yazım dilinin tip açıklaması ipuçlarıdır: caseSpellings tip adlarının
// küçültülmüş yazımı → tip adıdır (yazım bir tip adından yalnızca büyük/küçük harfle ayrılıyorsa,
// tam, ONDALIK, int: asıl yazım söylenir); suggestions kabul edilmeyen ama bir tip adına işaret
// eden yazımların (Türkçe karaktersiz biçim: ondalik; başka dillerden alışılmış adlar: int, str)
// küçültülmüş biçimi → önerilecek tip adıdır; otherNames başka bir yazım dilinin tip adı (tam
// yazımıyla) → önerilecek tip adıdır (İngilizce dosyada Tam → Int). İpucu adın yazıldığı dosyanın
// dilinden gelir: aynı yazım İngilizce dosyada Int'in harf yazımı, Türkçe dosyada Tam'ın yabancı adıdır.
type typeHints struct{ caseSpellings, suggestions, otherNames map[string]string }

// typeHintsByLanguage her yazım dilinin ipuçlarıdır (dil kodu → ipuçları); "" dili bilinmeyen
// metnin, bütün dillerin birleşimidir (orada her dilin adı harf yazımıdır, otherNames boştur). Yalnızca hata yolunda, ilk kullanımda kurulur.
var typeHintsByLanguage = sync.OnceValue(func() map[string]typeHints {
	var all []*codes.Language
	for _, lc := range codes.Languages() {
		if l, err := codes.LoadLanguage(lc); err == nil {
			all = append(all, l)
		}
	}
	out := map[string]typeHints{"": buildTypeHints(all...)}
	for _, l := range all {
		h := buildTypeHints(l)
		for _, o := range all {
			for _, k := range annotationTypes {
				if g, ok := o.Entry(k); ok && o != l {
					h.otherNames[g.Name] = codes.Name(k)
				}
			}
		}
		out[l.Code] = h
	}
	return out
})

// typeHintsIn lang'in ipuçlarıdır; lang nil ise (dil bilinmiyor) bütün dillerinki.
func typeHintsIn(lang *codes.Language) typeHints {
	if lang != nil {
		if h, ok := typeHintsByLanguage()[lang.Code]; ok {
			return h
		}
	}
	return typeHintsByLanguage()[""]
}

// buildTypeHints langs'in tip adlarından ipuçlarını kurar. Bir yazımın harf yazımı olması öneri
// olmasından önce gelir.
func buildTypeHints(langs ...*codes.Language) typeHints {
	h := typeHints{caseSpellings: map[string]string{}, suggestions: map[string]string{}, otherNames: map[string]string{}}
	for y, name := range typeSpellings {
		for _, f := range lowerForms(y) {
			h.caseSpellings[f] = name
		}
	}
	for _, l := range langs {
		for _, k := range annotationTypes {
			if g, ok := l.Entry(k); ok {
				for _, f := range lowerForms(g.Name) {
					h.caseSpellings[f] = codes.Name(k)
				}
			}
		}
	}
	for _, l := range langs {
		for _, k := range annotationTypes {
			g, ok := l.Entry(k)
			if !ok {
				continue
			}
			for _, y := range append([]string{turkish.ASCII(g.Name)}, g.Foreign...) {
				for _, f := range lowerForms(y) {
					if _, valid := h.caseSpellings[f]; !valid {
						h.suggestions[f] = codes.Name(k)
					}
				}
			}
		}
	}
	return h
}

// lowerForms yazımın küçültülmüş biçimleridir: Türkçe kuralla (METİN → metin) ve Unicode kuralıyla
// (INT → int). İkisi de anahtar olduğu için her dilin adı kendi harf kuralıyla bulunur; Go
// tarafında bir dilin adı geçmez.
func lowerForms(s string) [2]string {
	return [2]string{turkish.Lower(s), strings.ToLower(s)}
}

// resolveTypeAnnotation tip açıklamasındaki adı tek aramada çözer: asıl tip adını ve adın
// kabul edilmeyen bir yazım olup olmadığını verir; ipucu lang'den gelir (adın yazıldığı dosyanın
// dili; nil: bütün yazım dilleri, typeHints). Kabul edilen yazımda code 0'dır; yalnızca
// büyük/küçük harfi farklı yazımda H1006 (asıl yazım söylenir), Türkçe karaktersiz biçimde ya da
// başka dillerden alışılmış adta (ondalik, int, str) H1007 (öneri söylenir). Tanınmayan adın
// kendisi döner. Kabul edilmeyen yazımda da önerilen tip varsayılır: aynı satırda bir de tip
// uyuşmazlığı çıkmasın.
func resolveTypeAnnotation(name string, lang *codes.Language) (canonical string, code messages.Code) {
	if name == "" {
		return typeAny, 0
	}
	if canonical, valid := typeSpellings[name]; valid {
		return canonical, 0
	}
	h := typeHintsIn(lang)
	forms := lowerForms(name)
	for _, f := range forms {
		if canonical, ok := h.caseSpellings[f]; ok {
			return canonical, messages.H1006
		}
	}
	for _, f := range forms {
		if suggestion, ok := h.suggestions[f]; ok {
			return suggestion, messages.H1007
		}
	}
	if suggestion, ok := h.otherNames[name]; ok {
		return suggestion, messages.H1007
	}
	return name, 0
}

// checkedTypeName tip açıklamasındaki adı çözer ve kabul edilmeyen bir yazımsa (tam, ondalik,
// int) asıl adı öneren bir hata ekler; asıl tip adını döndürür.
func (c *Checker) checkedTypeName(name string, at lexer.Token) string {
	canonical, code := resolveTypeAnnotation(name, c.language())
	if code != 0 {
		c.addErrorAtToken(at, messages.New(code, name, c.typeArg(canonical)))
	}
	return canonical
}

// writtenType yazılan tipin denetleyicideki adını verir (yazılmamışsa Herhangi); adın kabul
// edilmeyen yazımı checkedTypeName'deki gibi hatadır.
func (c *Checker) writtenType(t *ast.TypeRef, line int) string {
	if t == nil {
		return typeAny
	}
	return c.checkedTypeName(t.Name, siteOf(t.Token, line))
}

// normalizedType yazılan tipin denetleyicideki adını hata eklemeden verir (yazılmamışsa Herhangi).
func normalizedType(t *ast.TypeRef) string {
	if t == nil {
		return typeAny
	}
	return normalizeTypeName(t.Name)
}

// normalizeTypeName tip açıklamasındaki adı asıl tip adına çevirir (hata eklemeden).
func normalizeTypeName(t string) string {
	canonical, _ := resolveTypeAnnotation(t, nil)
	return canonical
}

func typesCompatible(expected, actual string) bool {
	exp := normalizeTypeName(expected)
	act := normalizeTypeName(actual)
	if exp == typeAny || act == typeAny || act == typeNull {
		return true
	}
	// Tam sayı Ondalık beklenen yere atanabilir
	if exp == typeFloat && act == typeInt {
		return true
	}
	return exp == act
}

// divisionHint Tam beklenen yere '/'nin sonucu verildiğinde yol gösterir: '/' her zaman Ondalık
// verir (4.2), tam bölme '//'dir. İletinin bir parçasıdır (dıştaki iletinin dilinde yazılır);
// yol yoksa boş metin.
func (c *Checker) divisionHint(expected, given string, divides bool) any {
	if divides && normalizeTypeName(expected) == typeInt && given == typeFloat {
		return messages.New(messages.H1008, c.typeArg(typeFloat))
	}
	return ""
}

// dividesAtTop ifadenin en dıştaki işlemi '/' mi?
func dividesAtTop(e ast.Expression) bool {
	in, ok := e.(*ast.InfixExpression)
	return ok && in.Operator == "/"
}

// numericType tip bir sayı tipi mi (Tam ya da Ondalık)?
func numericType(t string) bool {
	return t == typeInt || t == typeFloat
}

// bitOperator işleç bir bit işleci mi (4.12)?
func bitOperator(op string) bool {
	switch op {
	case "&", "|", "^", "<<", ">>":
		return true
	}
	return false
}

// arithmeticType bir aritmetik işlemin sonuç tipini çalışma zamanının kurallarıyla çıkarır
// (4.2, 4.2a, 4.3, 4.10, 4.12): denetleyici çalışma zamanından farklı bir tip iddia etmez.
// '/' her zaman Ondalık verir; '+', '-', '*', '//', '%' ve '**' iki Tam'da Tam (Tam'ın eksi
// üssü çalışma hatasıdır), bir yanı Ondalıksa Ondalık verir. Bit işleçleri yalnız iki Tam'da
// tanımlıdır; başka işlenenin hatası çalışma anındadır, denetleyici tip iddia etmez.
func arithmeticType(op, left, right string) string {
	if op == "+" && (left == typeString || right == typeString) {
		return typeString
	}
	if bitOperator(op) {
		if left == typeInt && right == typeInt {
			return typeInt
		}
		return typeAny
	}
	if !numericType(left) || !numericType(right) {
		return typeAny
	}
	if op == "/" || left == typeFloat || right == typeFloat {
		return typeFloat
	}
	return typeInt
}

package codes

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kimliğin program metnindeki yazımı (dil tanımı 2.3a): TR_ ve kimliğin adı (TR_print, x.TR_map,
// TR_lr: 0.003, TR_Int). Sınıf yazılmaz, sözcüğün yeri onu belirler: noktadan sonra üye, çağrıda ve
// parametre listesinde iki noktadan önce argüman, öbür yerlerde anahtar kelime, bağlamsal sözcük,
// değer adı, tip, fonksiyon ya da sabit. Kod içeride sınıflı kalır (TR_fn_print); yazımdan koda
// geçiş yerle birlikte yapılır (FromSpelling). Aynı yerde aynı yazımlı iki kod olmaz
// (TestNoTwoCodesSharePlaceAndSpelling); komut, seçenek, ortam değişkeni ve hata ayıklayıcı komutu
// program metninde yazılmaz, belgelerde sınıflarıyla anılır (TR_cmd_test).

// Place is where a word stands in program text; it decides the class of an identity written
// TR_<name> (2.3a).
type Place uint8

const (
	// PlaceOther: keywords, contextual words, value names, types, functions and constants.
	PlaceOther Place = iota
	// PlaceMember: after '.' or '?.', and a struct's field and method definitions.
	PlaceMember
	// PlaceArg: a named argument before ':' in a call, and a function's parameter.
	PlaceArg
	placeCount
)

func (p Place) String() string {
	switch p {
	case PlaceMember:
		return "member"
	case PlaceArg:
		return "named argument"
	}
	return "other"
}

// spellings yer başına yazım → kod çizelgesidir (identities'ten bir kez kurulur).
var spellings = buildSpellings()

func buildSpellings() [placeCount]map[string]Code {
	var t [placeCount]map[string]Code
	for p := range t {
		t[p] = map[string]Code{}
	}
	for k := range identities {
		code := Code(k)
		if p, ok := PlaceOf(code); ok {
			t[p][Spelling(code)] = code
		}
	}
	return t
}

// PlaceOf is the place where the code is written in program text; ok is false for codes that are
// not written in programs (commands, options, environment variables, debugger commands).
func PlaceOf(k Code) (Place, bool) {
	switch k.Class() {
	case ClassKeyword, ClassContextual, ClassValueName, ClassType, ClassFunction, ClassConstant:
		return PlaceOther, true
	case ClassMember:
		return PlaceMember, true
	case ClassNamedArg:
		return PlaceArg, true
	}
	return PlaceOther, false
}

// Spelling is the code's spelling in program text: TR_ and the identity's name (TR_fn_print →
// TR_print). It is the same in every writing language.
func Spelling(k Code) string {
	id := k.String()
	cls := IdentityClass(id)
	if cls == "" {
		return id
	}
	return IdentityPrefix + strings.TrimPrefix(id, IdentityPrefix+string(cls)+"_")
}

// FromSpelling returns the code written as s (TR_print) at place p.
func FromSpelling(p Place, s string) (Code, bool) {
	if p >= placeCount {
		return None, false
	}
	k, ok := spellings[p][s]
	return k, ok
}

// SuggestIdentity returns the identity to suggest for a name written in program text's other place
// without TR_ or familiar from another language (2.3a): print → TR_print, True → TR_true, None and
// nil → TR_none, def → TR_func, elif → TR_else TR_if; a familiar member is suggested where it is
// written, len → x.TR_length. ok is false when the name is neither.
func SuggestIdentity(name string) (string, bool) {
	familiar, isFamiliar := FamiliarCode(name)
	exact, isExact := FromSpelling(PlaceOther, IdentityPrefix+name)
	switch {
	case isFamiliar && familiar == KwElse:
		// elif ve elsif başka dillerin "değilse eğer"idir: TR'de iki sözcüktür.
		return Spelling(KwElse) + " " + Spelling(KwIf), true
	case isFamiliar && (familiar.IsKeyword() || !isExact):
		// Anahtar kelimeyi gösteren tanıdık ad önce gelir: Python'un None'ı değerdir (TR_none),
		// None tipi (TR_None) değil.
		return Spelling(familiar), true
	case isExact:
		return Spelling(exact), true
	}
	if k, ok := FamiliarMember(name); ok {
		return "x." + Spelling(k), true
	}
	return "", false
}

// ValueLanguage is the code of ValueNames (it is no writing language; messages.ValueLanguage).
const ValueLanguage = messages.ValueLanguage

// valueNames ValueNames'in listesidir (bir kez kurulur).
var valueNames = buildValueNames()

// ValueNames returns the list a value's text is written with (3.14): every code is named by its
// identity's name, TR_ and the class dropped (TR_kw_true → true, TR_type_Int → Int, TR_kw_none →
// none, TR_mem_lr → lr). A value's text is the same on every machine and for every reader: it
// depends on no file's and no person's language; only messages, the shell and the debugger are
// in the person's language.
func ValueNames() *Language { return valueNames }

func buildValueNames() *Language {
	d := identityLanguage(ValueLanguage)
	for i := range d.names {
		d.names[i], d.canonical[i] = map[string]Code{}, map[string]string{}
	}
	for _, k := range d.codeList {
		name := strings.TrimPrefix(Spelling(k), IdentityPrefix)
		if k.Class() == ClassEnvironment {
			name = k.String() // ortam değişkeninin adı kimliğidir (TR_DIL'in kimliği TR_env_LANG değil)
		}
		ns := k.Namespace()
		d.entries[k].Name, d.nameArr[k], d.spellings[k] = name, name, []string{name}
		d.names[ns][name], d.canonical[ns][name] = k, name
	}
	return d
}

// SpellingsAt returns every spelling valid at place p (for suggestions), in no particular order.
func SpellingsAt(p Place) []string {
	if p >= placeCount {
		return nil
	}
	out := make([]string, 0, len(spellings[p]))
	for s := range spellings[p] {
		out = append(out, s)
	}
	return out
}

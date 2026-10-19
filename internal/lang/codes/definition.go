package codes

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Language-independent definitions (definitions.json): each code's kind (keyword, type,
// function, constant, method ...), the types a method or property applies to, its signature
// (order and type of positional parameters; code, type and default of named arguments), its
// return type, related codes, and for removed codes the new code the job moved to. No names
// appear here: signatures are lists of types and codes, and the language file (tr.json) gives
// the names. docs/kodlar.md is generated from these two files.
//
// File format (each entry is one line or a short block, codes in ascending order):
//
//	"G305": { "tür": "fonksiyon", "param": ["A200", "A200?"], "dönüş": "A205" },
//	"G400": { "tür": "fonksiyon", "param": ["A205|A212"], "adlı": [["T902", "A200"], ["T906", "A203", false]], "dönüş": "A200|A201|A212" },
//	"Y001": { "tür": "özellik", "tipler": ["A202", "A205", "A206"], "dönüş": "A200" },
//	"Y103": { "tür": "metot", "imzalar": { "A202": { "param": ["A202?"], "dönüş": "A205" }, "A212": { ... } } },
//	"G200": { "tür": "fonksiyon", "taşındı": "Y001" },
//
// A type expression is a union of type codes joined with "|" (A200|A201: Tam or Ondalık). On
// a positional parameter the suffix "?" means optional, "*" zero or more, "+" one or more.
// Type codes are A2xx (the type of AdamW and SGD is a type code too: İyileştirici, A214). A
// named argument is a [code, type, default] triple; the default is a JSON value (null: none)
// and is omitted when there is no single default. "taşındı" gives the new code when a code was
// removed and its job moved to another form; the old code's number is never reused and its
// name stays in the language file only for hints.

//go:embed definitions.json
var tanimJSON []byte

// DefinitionData returns the embedded definitions.json (for tests and tools).
func DefinitionData() []byte { return tanimJSON }

// Kind is the kind of a code.
type Kind string

const (
	KindKeyword    Kind = "anahtar_kelime"
	KindContextual Kind = "bağlamsal"
	KindType       Kind = "tip"
	KindFunction   Kind = "fonksiyon"
	// KindConstant is a builtin that has a value and is not called (pi); its signature is
	// only a return type.
	KindConstant Kind = "sabit"
	KindMethod   Kind = "metot"
	KindProperty Kind = "özellik"
	KindNamedArg Kind = "adlı_argüman"
)

// ExpectedKind is the kind implied by the code's family and group (for Y members it returns
// KindMethod since a member may be a method or a property; both are valid).
func (k Code) ExpectedKind() Kind {
	switch {
	case k.Letter() == 'A' && k.Group() == 0:
		return KindKeyword
	case k.Letter() == 'A' && k.Group() == 1:
		return KindContextual
	case k.Letter() == 'A':
		return KindType
	case k.Letter() == 'Y':
		return KindMethod
	case k.Namespace() == NamespaceParam:
		return KindNamedArg
	}
	return KindFunction
}

// TypeExpr is the type of a parameter or return value: one of Types (unspecified when
// empty).
type TypeExpr struct {
	Types []Code
	// Optional is an optional argument (?), Many zero or more (*), AtLeastOne one or more (+).
	Optional, Many, AtLeastOne bool
}

// Empty reports whether the type expression is unspecified.
func (t TypeExpr) Empty() bool { return len(t.Types) == 0 }

// NamedArg is a named argument.
type NamedArg struct {
	Code Code
	Type TypeExpr
	// Default is a JSON value (bool, float64, string or nil = none); when HasDefault is
	// false there is no single default.
	Default    any
	HasDefault bool
}

// Signature holds the positional parameters, named arguments and return type.
type Signature struct {
	Params []TypeExpr
	Named  []NamedArg
	Return TypeExpr
}

// TypeSignature is a member's signature on one type.
type TypeSignature struct {
	Type      Code
	Signature Signature
}

// Definition is one entry of definitions.json.
type Definition struct {
	Code Code
	Kind Kind
	// Signature is the signature of a function or of a member whose signature does not
	// depend on the type.
	Signature Signature
	// Members lists the types a method or property applies to and its signature on each
	// (in type order).
	Members []TypeSignature
	Related []Code
	// MovedTo is the new code when the code was removed and its job moved; Removed is true
	// when it was removed with no replacement.
	MovedTo Code
	Removed bool
}

// Types returns the types the member applies to.
func (t Definition) Types() []Code {
	out := make([]Code, len(t.Members))
	for i, u := range t.Members {
		out[i] = u.Type
	}
	return out
}

// TypeSpecific reports whether the member's signature depends on the type (written with
// "imzalar").
func (t Definition) TypeSpecific() bool {
	return len(t.Members) > 1 && !imzaEsit(t.Members[0].Signature, t.Members[1:]...)
}

func imzaEsit(a Signature, digerleri ...TypeSignature) bool {
	for _, d := range digerleri {
		if fmt.Sprint(a) != fmt.Sprint(d.Signature) {
			return false
		}
	}
	return true
}

var (
	tanimOnce   sync.Once
	tanimlar    map[Code]Definition
	tanimHatasi error
	tanimSirasi []Code
)

// DefinitionOf returns the code's definition. The definitions file is read on the first call
// (the interpreter does not need it while running; tools, tests and error paths use it).
func DefinitionOf(k Code) (Definition, bool) {
	tanimYukle()
	t, ok := tanimlar[k]
	return t, ok
}

// DefinitionError returns the format error found while reading definitions.json, or nil.
func DefinitionError() error {
	tanimYukle()
	return tanimHatasi
}

// DefinitionCodes returns every code in the definitions file, in ascending order.
func DefinitionCodes() []Code {
	tanimYukle()
	return append([]Code(nil), tanimSirasi...)
}

func tanimYukle() {
	tanimOnce.Do(func() {
		tanimlar, tanimSirasi, tanimHatasi = ParseDefinitions(tanimJSON)
	})
}

type hamImza struct {
	Param []string            `json:"param"`
	Adli  [][]json.RawMessage `json:"adlı"`
	Donus string              `json:"dönüş"`
}

type hamTanim struct {
	Tur      string              `json:"tür"`
	Tipler   []string            `json:"tipler"`
	Param    []string            `json:"param"`
	Adli     [][]json.RawMessage `json:"adlı"`
	Donus    string              `json:"dönüş"`
	Imzalar  map[string]hamImza  `json:"imzalar"`
	Iliskili []string            `json:"ilişkili"`
	Tasindi  string              `json:"taşındı"`
	Kaldir   bool                `json:"kaldırıldı"`
}

// ParseDefinitions parses a definitions file; it collects every format error into one error.
func ParseDefinitions(veri []byte) (map[Code]Definition, []Code, error) {
	var ham map[string]json.RawMessage
	if err := json.Unmarshal(veri, &ham); err != nil {
		return nil, nil, fmt.Errorf("definitions.json: %v", err)
	}
	var hatalar []string
	hata := func(bicim string, a ...any) { hatalar = append(hatalar, fmt.Sprintf(bicim, a...)) }
	out := make(map[Code]Definition, len(ham))
	var sira []Code
	for anahtar, satir := range ham {
		k, err := Parse(anahtar)
		if err != nil {
			hata("%v", err)
			continue
		}
		var h hamTanim
		dec := json.NewDecoder(bytes.NewReader(satir))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&h); err != nil {
			hata("%s: %v", k, err)
			continue
		}
		t := Definition{Code: k, Kind: Kind(h.Tur)}
		kodlar := func(alan string, ss []string) []Code {
			var ks []Code
			for _, s := range ss {
				x, err := Parse(s)
				if err != nil {
					hata("%s %s: %v", k, alan, err)
					continue
				}
				ks = append(ks, x)
			}
			return ks
		}
		imza := func(param []string, adli [][]json.RawMessage, donus string) Signature {
			var im Signature
			for _, p := range param {
				ti, err := ParseType(p, true)
				if err != nil {
					hata("%s param: %v", k, err)
				}
				im.Params = append(im.Params, ti)
			}
			for _, a := range adli {
				ad, err := adliCoz(a)
				if err != nil {
					hata("%s adlı: %v", k, err)
					continue
				}
				im.Named = append(im.Named, ad)
			}
			if donus != "" {
				ti, err := ParseType(donus, false)
				if err != nil {
					hata("%s dönüş: %v", k, err)
				}
				im.Return = ti
			}
			return im
		}
		t.Signature = imza(h.Param, h.Adli, h.Donus)
		for _, tip := range kodlar("tipler", h.Tipler) {
			t.Members = append(t.Members, TypeSignature{Type: tip, Signature: t.Signature})
		}
		if len(h.Imzalar) > 0 {
			if len(h.Tipler) > 0 || len(h.Param) > 0 || len(h.Adli) > 0 || h.Donus != "" {
				hata("%s: \"imzalar\" varken \"tipler\", \"param\", \"adlı\" ve \"dönüş\" yazılmaz", k)
			}
			for s, hi := range h.Imzalar {
				tip, err := Parse(s)
				if err != nil {
					hata("%s imzalar: %v", k, err)
					continue
				}
				t.Members = append(t.Members, TypeSignature{Type: tip, Signature: imza(hi.Param, hi.Adli, hi.Donus)})
			}
			sort.Slice(t.Members, func(i, j int) bool { return t.Members[i].Type < t.Members[j].Type })
		}
		t.Related = kodlar("ilişkili", h.Iliskili)
		if h.Tasindi != "" {
			if y := kodlar("taşındı", []string{h.Tasindi}); len(y) == 1 {
				t.MovedTo = y[0]
			}
		}
		t.Removed = h.Kaldir
		out[k] = t
		sira = append(sira, k)
	}
	sort.Slice(sira, func(i, j int) bool { return sira[i] < sira[j] })
	if len(hatalar) > 0 {
		sort.Strings(hatalar)
		return out, sira, fmt.Errorf("definitions.json:\n  %s", strings.Join(hatalar, "\n  "))
	}
	return out, sira, nil
}

// ParseType parses a type expression such as "A200|A201?"; suffixes are valid only on
// positional parameters (when parametre is true).
func ParseType(s string, parametre bool) (TypeExpr, error) {
	var t TypeExpr
	if parametre && s != "" {
		switch s[len(s)-1] {
		case '?':
			t.Optional, s = true, s[:len(s)-1]
		case '*':
			t.Many, s = true, s[:len(s)-1]
		case '+':
			t.AtLeastOne, s = true, s[:len(s)-1]
		}
	}
	for _, p := range strings.Split(s, "|") {
		k, err := Parse(p)
		if err != nil {
			return t, fmt.Errorf("tip ifadesi %q: %v", s, err)
		}
		t.Types = append(t.Types, k)
	}
	return t, nil
}

func adliCoz(a []json.RawMessage) (NamedArg, error) {
	var ad NamedArg
	if len(a) < 2 || len(a) > 3 {
		return ad, fmt.Errorf("adlı argüman [kod, tip] ya da [kod, tip, varsayılan] olmalı")
	}
	var ks, ts string
	if err := json.Unmarshal(a[0], &ks); err != nil {
		return ad, err
	}
	if err := json.Unmarshal(a[1], &ts); err != nil {
		return ad, err
	}
	k, err := Parse(ks)
	if err != nil {
		return ad, err
	}
	ti, err := ParseType(ts, false)
	if err != nil {
		return ad, err
	}
	ad.Code, ad.Type = k, ti
	if len(a) == 3 {
		if err := json.Unmarshal(a[2], &ad.Default); err != nil {
			return ad, err
		}
		ad.HasDefault = true
	}
	return ad, nil
}

// ReceiverName is the name written for a type as a receiver in error messages and docs: the
// type's name in lower case (Liste → liste, Sözlük → sözlük).
func ReceiverName(tip Code) string {
	return strings.ToLowerSpecial(unicode.TurkishCase, Name(tip))
}

// MemberName is a member's name written with its receiver: liste.filtrele, x.uzunluk. If the
// member is on a single type the receiver is that type's name (ReceiverName), otherwise x.
func MemberName(y Code) string {
	t, _ := DefinitionOf(y)
	a := "x"
	if len(t.Members) == 1 {
		a = ReceiverName(t.Members[0].Type)
	}
	return a + "." + Name(y)
}

// MemberSpelling returns how a member is written: x.uzunluk, liste.filtrele(...),
// liste.sırala(). The receiver is as in MemberName. Error messages and docs use it to show
// the new form of moved codes.
func MemberSpelling(y Code) string {
	t, _ := DefinitionOf(y)
	s := MemberName(y)
	if t.Kind != KindMethod {
		return s
	}
	for _, u := range t.Members {
		if len(u.Signature.Params) > 0 {
			return s + "(...)"
		}
	}
	return s + "()"
}

// SourceLanguage is the code of the source language file: it keeps the names of removed and
// moved (dead) codes for hints; in other language files dead codes are optional.
const SourceLanguage = "tr"

// CheckDefinitions checks that the definitions agree with a language file: every code (moved
// ones included; dead codes are optional in languages other than TR) has a definition, every
// defined code is in the language file, each kind fits the code's family, type expressions
// consist of type codes (A2xx), named-argument codes are x9xx, related and moved-to codes
// exist, a moved code points to a live code in another namespace, and the move information
// in the language file matches the definition.
func CheckDefinitions(d *Language) []error {
	var hatalar []error
	hata := func(bicim string, a ...any) { hatalar = append(hatalar, fmt.Errorf(bicim, a...)) }
	if err := DefinitionError(); err != nil {
		hata("%v", err)
	}
	canli := func(k Code) bool { g, ok := d.girdiler[k]; return ok && !g.Dead() }
	tumu := append(d.Codes(), d.DeadCodes()...)
	for _, k := range tumu {
		if _, ok := DefinitionOf(k); !ok {
			hata("%s (%s) dil dosyasında var, definitions.json'da tanımı yok", k, d.Name(k))
		}
	}
	tipKodu := func(k Code) bool {
		g, ok := d.girdiler[k]
		return ok && k.Letter() == 'A' && k.Group() >= 2 && !g.Dead()
	}
	tipDenetle := func(k Code, yer string, t TypeExpr) {
		for _, x := range t.Types {
			if !tipKodu(x) {
				hata("%s %s: %s bir tip kodu değil", k, yer, x)
			}
		}
	}
	imzaDenetle := func(k Code, yer string, im Signature) {
		for i, p := range im.Params {
			tipDenetle(k, fmt.Sprintf("%s%d. parametre", yer, i+1), p)
			if (p.Many || p.AtLeastOne) && i != len(im.Params)-1 {
				hata("%s %s: '*' ya da '+' yalnızca son parametrede olabilir", k, yer)
			}
		}
		for _, a := range im.Named {
			if a.Code.Namespace() != NamespaceParam || !canli(a.Code) {
				hata("%s %sadlı argüman %s bir adlı argüman kodu (x9xx) değil", k, yer, a.Code)
			}
			tipDenetle(k, yer+"adlı argüman", a.Type)
		}
		if !im.Return.Empty() {
			tipDenetle(k, yer+"dönüş", im.Return)
		}
	}
	for _, k := range DefinitionCodes() {
		t, _ := DefinitionOf(k)
		g, ok := d.girdiler[k]
		if !ok {
			// Ölü kodların adları yalnızca ipucu içindir: kaynak dil (TR) onları tutar, öbür dil
			// dosyalarında isteğe bağlıdırlar (o dilde hiç var olmamış adların karşılığı gerekmez).
			if d.Code != SourceLanguage && (t.Removed || t.MovedTo != None) {
				continue
			}
			hata("%s definitions.json'da var, dil dosyasında yok", k)
			continue
		}
		bek := k.ExpectedKind()
		switch {
		case bek == KindMethod && t.Kind != KindMethod && t.Kind != KindProperty:
			hata("%s: tür %q olamaz; üyeler (Y) metot ya da özelliktir", k, t.Kind)
		case bek == KindFunction && t.Kind == KindConstant:
			// Yerleşik bir sabit (G/T/V/W ailesinde fonksiyonun yerinde).
		case bek != KindMethod && t.Kind != bek:
			hata("%s: tür %q, ama kodun ailesi %q gerektirir", k, t.Kind, bek)
		}
		if g.MovedTo != t.MovedTo {
			hata("%s: dil dosyasının taşınma bilgisi (%s) tanımla (%s) aynı değil; go run ./tools/gen/codeconsts", k, g.MovedTo, t.MovedTo)
		}
		if g.Removed != t.Removed {
			hata("%s: dil dosyasının kaldırılma bilgisi tanımla aynı değil; go run ./tools/gen/codeconsts", k)
		}
		if t.Removed {
			if t.MovedTo != None || len(t.Members) > 0 || len(t.Signature.Params) > 0 || len(t.Signature.Named) > 0 || !t.Signature.Return.Empty() || len(t.Related) > 0 {
				hata("%s kaldırıldı; yalnızca tür yazılır", k)
			}
			continue
		}
		if t.MovedTo != None {
			if len(t.Members) > 0 || len(t.Signature.Params) > 0 || len(t.Signature.Named) > 0 || !t.Signature.Return.Empty() {
				hata("%s taşındı; imza yazılmaz (imza yeni kodda, %s)", k, t.MovedTo)
			}
			y, ok := d.girdiler[t.MovedTo]
			if !ok || y.Dead() {
				hata("%s %s'e taşındı, ama %s canlı bir kod değil", k, t.MovedTo, t.MovedTo)
			} else if t.MovedTo.Namespace() == k.Namespace() {
				hata("%s %s'e taşındı, ama ikisi aynı ad alanında (%s); taşıma biçim değiştirir (fonksiyon ↔ üye)", k, t.MovedTo, k.Namespace())
			}
			continue
		}
		switch t.Kind {
		case KindMethod, KindProperty:
			if len(t.Members) == 0 {
				hata("%s: üyenin geçerli olduğu tipler (tipler ya da imzalar) yazılmalı", k)
			}
			for _, u := range t.Members {
				if !tipKodu(u.Type) {
					hata("%s: %s bir tip kodu değil", k, u.Type)
				}
				imzaDenetle(k, u.Type.String()+" ", u.Signature)
				if t.Kind == KindProperty && (len(u.Signature.Params) > 0 || len(u.Signature.Named) > 0) {
					hata("%s: özellik parametre almaz", k)
				}
			}
		case KindFunction:
			imzaDenetle(k, "", t.Signature)
		case KindConstant:
			if len(t.Signature.Params) > 0 || len(t.Signature.Named) > 0 || len(t.Members) > 0 || t.Signature.Return.Empty() {
				hata("%s: sabit parametre almaz; yalnızca dönüş (değerin tipi) yazılır", k)
			}
			imzaDenetle(k, "", t.Signature)
		default:
			if len(t.Signature.Params) > 0 || len(t.Signature.Named) > 0 || !t.Signature.Return.Empty() || len(t.Members) > 0 {
				hata("%s: %s imza almaz", k, t.Kind)
			}
		}
		for _, x := range t.Related {
			if !canli(x) {
				hata("%s: ilişkili kod %s dil dosyasında canlı bir kod değil", k, x)
			} else if x == k {
				hata("%s kendisiyle ilişkili yazılmış", k)
			}
		}
	}
	return hatalar
}

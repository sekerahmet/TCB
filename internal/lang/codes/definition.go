package codes

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/turkish"
)

// Language-independent definitions (definitions.json): each code's kind (keyword, type,
// function, constant, method ...), the types a method or property applies to, its signature
// (order and type of positional parameters; code, type and default of named arguments), its
// return type, related codes, and for removed codes the new code the job moved to. No names
// appear here: signatures are lists of types and codes, and the language file (tr.json) gives
// the names. docs/codes.md is generated from these two files.
//
// File format (each entry is one line or a short block, codes in ascending order):
//
//	"G305": { "tür": "fonksiyon", "param": ["A200", "A200?"], "dönüş": "A205" },
//	"G400": { "tür": "fonksiyon", "param": ["A205|A212"], "adlı": [["T902", "A200"], ["T906", "A203", false]], "dönüş": "A200|A201|A212" },
//	"Y001": { "tür": "özellik", "tipler": ["A202", "A205", "A206"], "dönüş": "A200" },
//	"Y103": { "tür": "metot", "imzalar": { "A202": { "param": ["A202?"], "dönüş": "A205" }, "A212": { ... } } },
//
// A type expression is a union of type codes joined with "|" (A200|A201: Tam or Ondalık). On
// a positional parameter the suffix "?" means optional, "*" zero or more, "+" one or more.
// Type codes are A2xx (the type of AdamW and SGD is a type code too: İyileştirici, A214). A
// named argument is a [code, type, default] triple; the default is a JSON value (null: none)
// and is omitted when there is no single default.

//go:embed definitions.json
var definitionsJSON []byte

// DefinitionData returns the embedded definitions.json (for tests and tools).
func DefinitionData() []byte { return definitionsJSON }

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
	// KindCommand is a command or option of tr, the shell or the debugger (K); it is not a
	// name in the language, so programs cannot use it.
	KindCommand Kind = "komut"
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
	case k.Letter() == 'K':
		return KindCommand
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
	return len(t.Members) > 1 && !sigEqual(t.Members[0].Signature, t.Members[1:]...)
}

func sigEqual(a Signature, others ...TypeSignature) bool {
	for _, d := range others {
		if fmt.Sprint(a) != fmt.Sprint(d.Signature) {
			return false
		}
	}
	return true
}

var (
	defOnce  sync.Once
	defs     map[Code]Definition
	defErr   error
	defOrder []Code
)

// DefinitionOf returns the code's definition. The definitions file is read on the first call
// (the interpreter does not need it while running; tools, tests and error paths use it).
func DefinitionOf(k Code) (Definition, bool) {
	loadDefinitions()
	t, ok := defs[k]
	return t, ok
}

// DefinitionError returns the format error found while reading definitions.json, or nil.
func DefinitionError() error {
	loadDefinitions()
	return defErr
}

// DefinitionCodes returns every code in the definitions file, in ascending order.
func DefinitionCodes() []Code {
	loadDefinitions()
	return append([]Code(nil), defOrder...)
}

func loadDefinitions() {
	defOnce.Do(func() {
		defs, defOrder, defErr = ParseDefinitions(definitionsJSON)
	})
}

type rawSignature struct {
	Param  []string            `json:"param"`
	Named  [][]json.RawMessage `json:"adlı"`
	Return string              `json:"dönüş"`
}

type rawDefinition struct {
	Kind       string                  `json:"tür"`
	Types      []string                `json:"tipler"`
	Param      []string                `json:"param"`
	Named      [][]json.RawMessage     `json:"adlı"`
	Return     string                  `json:"dönüş"`
	Signatures map[string]rawSignature `json:"imzalar"`
	Related    []string                `json:"ilişkili"`
}

// ParseDefinitions parses a definitions file; it collects every format error into one error.
func ParseDefinitions(data []byte) (map[Code]Definition, []Code, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, internalf("definitions.json: %v", err)
	}
	var errs []string
	errorf := func(form string, a ...any) { errs = append(errs, fmt.Sprintf(form, a...)) }
	out := make(map[Code]Definition, len(raw))
	var order []Code
	for key, line := range raw {
		k, err := Parse(key)
		if err != nil {
			errorf("%v", err)
			continue
		}
		var h rawDefinition
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&h); err != nil {
			errorf("%s: %v", k, err)
			continue
		}
		t := Definition{Code: k, Kind: Kind(h.Kind)}
		codeList := func(field string, ss []string) []Code {
			var ks []Code
			for _, s := range ss {
				x, err := Parse(s)
				if err != nil {
					errorf("%s %s: %v", k, field, err)
					continue
				}
				ks = append(ks, x)
			}
			return ks
		}
		sig := func(param []string, named [][]json.RawMessage, ret string) Signature {
			var im Signature
			for _, p := range param {
				ti, err := ParseType(p, true)
				if err != nil {
					errorf("%s param: %v", k, err)
				}
				im.Params = append(im.Params, ti)
			}
			for _, a := range named {
				name, err := parseNamed(a)
				if err != nil {
					errorf("%s adlı: %v", k, err)
					continue
				}
				im.Named = append(im.Named, name)
			}
			if ret != "" {
				ti, err := ParseType(ret, false)
				if err != nil {
					errorf("%s dönüş: %v", k, err)
				}
				im.Return = ti
			}
			return im
		}
		t.Signature = sig(h.Param, h.Named, h.Return)
		for _, typ := range codeList("tipler", h.Types) {
			t.Members = append(t.Members, TypeSignature{Type: typ, Signature: t.Signature})
		}
		if len(h.Signatures) > 0 {
			if len(h.Types) > 0 || len(h.Param) > 0 || len(h.Named) > 0 || h.Return != "" {
				errorf("%s: \"tipler\", \"param\", \"adlı\" and \"dönüş\" must not be written when \"imzalar\" is present", k)
			}
			for s, hi := range h.Signatures {
				typ, err := Parse(s)
				if err != nil {
					errorf("%s imzalar: %v", k, err)
					continue
				}
				t.Members = append(t.Members, TypeSignature{Type: typ, Signature: sig(hi.Param, hi.Named, hi.Return)})
			}
			sort.Slice(t.Members, func(i, j int) bool { return t.Members[i].Type < t.Members[j].Type })
		}
		t.Related = codeList("ilişkili", h.Related)
		out[k] = t
		order = append(order, k)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	if len(errs) > 0 {
		sort.Strings(errs)
		return out, order, internalf("definitions.json:\n  %s", strings.Join(errs, "\n  "))
	}
	return out, order, nil
}

// ParseType parses a type expression such as "A200|A201?"; suffixes are valid only on
// positional parameters (when parametre is true).
func ParseType(s string, isParam bool) (TypeExpr, error) {
	var t TypeExpr
	if isParam && s != "" {
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
			return t, internalf("type expression %q: %v", s, err)
		}
		t.Types = append(t.Types, k)
	}
	return t, nil
}

func parseNamed(a []json.RawMessage) (NamedArg, error) {
	var name NamedArg
	if len(a) < 2 || len(a) > 3 {
		return name, internalf("a named argument must be [code, type] or [code, type, default]")
	}
	var ks, ts string
	if err := json.Unmarshal(a[0], &ks); err != nil {
		return name, err
	}
	if err := json.Unmarshal(a[1], &ts); err != nil {
		return name, err
	}
	k, err := Parse(ks)
	if err != nil {
		return name, err
	}
	ti, err := ParseType(ts, false)
	if err != nil {
		return name, err
	}
	name.Code, name.Type = k, ti
	if len(a) == 3 {
		if err := json.Unmarshal(a[2], &name.Default); err != nil {
			return name, err
		}
		name.HasDefault = true
	}
	return name, nil
}

// ReceiverName is the name written for a type as a receiver in error messages and docs: the
// type's name in lower case (Liste → liste, Sözlük → sözlük).
func ReceiverName(typ Code) string {
	return turkish.Lower(Name(typ))
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

// SourceLanguage is the code of the source language file (Turkish, the canonical language).
const SourceLanguage = "tr"

// CheckDefinitions checks that the definitions agree with a language file: every code has a
// definition, every defined code is in the language file, each kind fits the code's family,
// type expressions consist of type codes (A2xx), named-argument codes are x9xx and related
// codes exist.
func CheckDefinitions(d *Language) []error {
	var errs []error
	errorf := func(form string, a ...any) { errs = append(errs, internalf(form, a...)) }
	if err := DefinitionError(); err != nil {
		errorf("%v", err)
	}
	live := func(k Code) bool { _, ok := d.entries[k]; return ok }
	for _, k := range d.Codes() {
		if _, ok := DefinitionOf(k); !ok {
			errorf("%s (%s) is in the language file but has no definition in definitions.json", k, d.Name(k))
		}
	}
	typeCode := func(k Code) bool {
		_, ok := d.entries[k]
		return ok && k.Letter() == 'A' && k.Group() >= 2
	}
	checkType := func(k Code, pos string, t TypeExpr) {
		for _, x := range t.Types {
			if !typeCode(x) {
				errorf("%s %s: %s is not a type code", k, pos, x)
			}
		}
	}
	checkSig := func(k Code, pos string, im Signature) {
		for i, p := range im.Params {
			checkType(k, fmt.Sprintf("%s%d. parametre", pos, i+1), p)
			if (p.Many || p.AtLeastOne) && i != len(im.Params)-1 {
				errorf("%s %s: '*' or '+' is only allowed on the last parameter", k, pos)
			}
		}
		for _, a := range im.Named {
			if a.Code.Namespace() != NamespaceParam || !live(a.Code) {
				errorf("%s %snamed argument %s is not a named-argument code (x9xx)", k, pos, a.Code)
			}
			checkType(k, pos+"named argument", a.Type)
		}
		if !im.Return.Empty() {
			checkType(k, pos+"return", im.Return)
		}
	}
	for _, k := range DefinitionCodes() {
		t, _ := DefinitionOf(k)
		if _, ok := d.entries[k]; !ok {
			errorf("%s is in definitions.json but not in the language file", k)
			continue
		}
		exp := k.ExpectedKind()
		switch {
		case exp == KindMethod && t.Kind != KindMethod && t.Kind != KindProperty:
			errorf("%s: kind %q is not allowed; members (Y) are methods or properties", k, t.Kind)
		case exp == KindFunction && t.Kind == KindConstant:
			// Yerleşik bir sabit (G/T/V/W ailesinde fonksiyonun yerinde).
		case exp != KindMethod && t.Kind != exp:
			errorf("%s: kind %q, but the code's family requires %q", k, t.Kind, exp)
		}
		switch t.Kind {
		case KindMethod, KindProperty:
			if len(t.Members) == 0 {
				errorf("%s: the types a member applies to (tipler or imzalar) must be written", k)
			}
			for _, u := range t.Members {
				if !typeCode(u.Type) {
					errorf("%s: %s is not a type code", k, u.Type)
				}
				checkSig(k, u.Type.String()+" ", u.Signature)
				if t.Kind == KindProperty && (len(u.Signature.Params) > 0 || len(u.Signature.Named) > 0) {
					errorf("%s: a property takes no parameters", k)
				}
			}
		case KindFunction:
			checkSig(k, "", t.Signature)
		case KindConstant:
			if len(t.Signature.Params) > 0 || len(t.Signature.Named) > 0 || len(t.Members) > 0 || t.Signature.Return.Empty() {
				errorf("%s: a constant takes no parameters; only the return (the value's type) is written", k)
			}
			checkSig(k, "", t.Signature)
		default:
			if len(t.Signature.Params) > 0 || len(t.Signature.Named) > 0 || !t.Signature.Return.Empty() || len(t.Members) > 0 {
				errorf("%s: %s takes no signature", k, t.Kind)
			}
		}
		for _, x := range t.Related {
			if !live(x) {
				errorf("%s: related code %s is not in the language file", k, x)
			} else if x == k {
				errorf("%s is listed as related to itself", k)
			}
		}
	}
	return errs
}

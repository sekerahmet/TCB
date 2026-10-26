package codes

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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
//	"TR_fn_range": { "tür": "fonksiyon", "param": ["TR_type_Int", "TR_type_Int?"], "dönüş": "TR_type_List" },
//	"TR_fn_sum": { "tür": "fonksiyon", "param": ["TR_type_List|TR_type_Tensor"], "adlı": [["TR_arg_dim", "TR_type_Int"], ["TR_arg_keepdim", "TR_type_Bool", false]], "dönüş": "TR_type_Int|TR_type_Float|TR_type_Tensor" },
//	"TR_mem_length": { "tür": "özellik", "tipler": ["TR_type_String", "TR_type_List", "TR_type_Dict"], "dönüş": "TR_type_Int" },
//	"TR_mem_split": { "tür": "metot", "imzalar": { "TR_type_String": { "param": ["TR_type_String?"], "dönüş": "TR_type_List" }, "TR_type_Tensor": { ... } } },
//
// A type expression is a union of type codes joined with "|" (TR_type_Int|TR_type_Float: Tam or
// Ondalık). On a positional parameter the suffix "?" means optional, "*" zero or more, "+" one
// or more. Type codes are the type class (the type of AdamW and SGD is a type code too:
// İyileştirici, TR_type_Optimizer). A named argument is a [code, type, default] triple; the
// default is a JSON value (null: none) and is omitted when there is no single default.

//go:embed definitions.json
var definitionsJSON []byte

// DefinitionData returns the embedded definitions.json (for tests and tools).
func DefinitionData() []byte { return definitionsJSON }

// Kind is the kind of a code.
type Kind string

const (
	KindKeyword    Kind = "anahtar_kelime"
	KindContextual Kind = "bağlamsal"
	// KindValueName is the name of a value that is not a word of the source (inf, nan: written
	// ondalık("sonsuz")); it is never coded in a file.
	KindValueName Kind = "değer_adı"
	KindType      Kind = "tip"
	KindFunction  Kind = "fonksiyon"
	// KindConstant is a builtin that has a value and is not called (pi); its signature is
	// only a return type.
	KindConstant Kind = "sabit"
	KindMethod   Kind = "metot"
	KindProperty Kind = "özellik"
	KindNamedArg Kind = "adlı_argüman"
	// KindCommand is a command, option, shell or debugger command or environment variable; it is
	// not a name in the language, so programs cannot use it.
	KindCommand Kind = "komut"
)

// ExpectedKind is the kind implied by the class of the code's identity (for members it returns
// KindMethod since a member may be a method or a property; both are valid).
func (k Code) ExpectedKind() Kind {
	switch k.Class() {
	case ClassKeyword:
		return KindKeyword
	case ClassContextual:
		return KindContextual
	case ClassValueName:
		return KindValueName
	case ClassType:
		return KindType
	case ClassFunction:
		return KindFunction
	case ClassConstant:
		return KindConstant
	case ClassNamedArg:
		return KindNamedArg
	case ClassMember:
		return KindMethod
	}
	return KindCommand
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
	// Common is the code's common name ("ortak_ad"): a concept written with international
	// notation or a proper name (sin, AdamW, json) has the same name in every writing language. It
	// is written only here, never in a language file; Load gives it to every language.
	Common string
}

// Types returns the types the member applies to.
func (t Definition) Types() []Code {
	out := make([]Code, len(t.Members))
	for i, u := range t.Members {
		out[i] = u.Type
	}
	return out
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

// commonNames ortak adı olan kodlardır (kod → ortak ad).
func commonNames() map[Code]string {
	loadDefinitions()
	out := map[Code]string{}
	for _, k := range defOrder {
		if c := defs[k].Common; c != "" {
			out[k] = c
		}
	}
	return out
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
	Common     string                  `json:"ortak_ad"`
}

// ParseDefinitions parses a definitions file; it collects every format error into one error.
func ParseDefinitions(data []byte) (map[Code]Definition, []Code, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, messages.Internalf("definitions.json: %v", err)
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
		t.Common = h.Common
		out[k] = t
		order = append(order, k)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	if len(errs) > 0 {
		sort.Strings(errs)
		return out, order, messages.Internalf("definitions.json:\n  %s", strings.Join(errs, "\n  "))
	}
	return out, order, nil
}

// ParseType parses a type expression such as "TR_type_Int|TR_type_Float?"; suffixes are valid only on
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
			return t, messages.Internalf("type expression %q: %v", s, err)
		}
		t.Types = append(t.Types, k)
	}
	return t, nil
}

func parseNamed(a []json.RawMessage) (NamedArg, error) {
	var name NamedArg
	if len(a) < 2 || len(a) > 3 {
		return name, messages.Internalf("a named argument must be [code, type] or [code, type, default]")
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

// SourceLanguage is the code of the source language file (Turkish, the canonical language).
const SourceLanguage = "tr"

// CheckDefinitions checks that the definitions agree with a language file: every code has a
// definition, every defined code is in the language file, each kind fits the class of the
// code's identity, type expressions consist of type codes, named-argument codes are of the arg class and related
// codes exist.
func CheckDefinitions(d *Language) []error {
	var errs []error
	errorf := func(form string, a ...any) { errs = append(errs, messages.Internalf(form, a...)) }
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
		return ok && k.Class() == ClassType
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
				errorf("%s %snamed argument %s is not a named-argument code (arg class)", k, pos, a.Code)
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
		case exp != KindMethod && t.Kind != exp:
			errorf("%s: kind %q, but the class of the code's identity requires %q", k, t.Kind, exp)
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

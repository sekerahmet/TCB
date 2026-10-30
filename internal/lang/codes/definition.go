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
)

// Language-independent definitions (definitions.json): each code's kind (keyword, type,
// function, constant, method ...), the types a method or property applies to, its signature
// (order and type of positional parameters; code, type and default of named arguments), its
// return type, related codes, the example written with identities, and for removed codes the new
// code the job moved to. No names appear here: signatures are lists of types and codes, and the language file (tr.json) gives
// the names. docs/codes.md is generated from these two files.
//
// File format (each entry is one line or a short block, codes in ascending order):
//
//	"TR_fn_round": { "kind": "function", "params": ["TR_type_Int|TR_type_Float", "TR_type_Int?"], "returns": "TR_type_Int|TR_type_Float" },
//	"TR_fn_sum": { "kind": "function", "params": ["TR_type_List|TR_type_Tensor"], "named": [["TR_arg_dim", "TR_type_Int"], ["TR_arg_keepdim", "TR_type_Bool", false]], "returns": "TR_type_Int|TR_type_Float|TR_type_Tensor" },
//	"TR_mem_length": { "kind": "property", "types": ["TR_type_String", "TR_type_List", "TR_type_Dict"], "returns": "TR_type_Int" },
//	"TR_mem_split": { "kind": "method", "signatures": { "TR_type_String": { "params": ["TR_type_String?"], "returns": "TR_type_List" }, "TR_type_Tensor": { ... } } },
//
// A type expression is a union of type codes joined with "|" (TR_type_Int|TR_type_Float: Tam or
// Ondalık). On a positional parameter the suffix "?" means optional, "*" zero or more, "+" one
// or more. Type codes are the type class (the type of AdamW and SGD is a type code too:
// İyileştirici, TR_type_Optimizer). A named argument is a [code, type, default] triple; the
// default is a JSON value (null: none) and is omitted when there is no single default. A public
// name of a standard library module also carries "module" (the module, "tr/layers"); the code is
// then a library code, not a builtin.

//go:embed definitions.json
var definitionsJSON []byte

// DefinitionData returns the embedded definitions.json (for tests and tools).
func DefinitionData() []byte { return definitionsJSON }

// Kind is the kind of a code.
type Kind string

const (
	KindKeyword    Kind = "keyword"
	KindContextual Kind = "contextual"
	// KindValueName is the name of a value of the language that is no keyword (TR_inf, TR_nan;
	// 3.2): bound in the global scope like a constant, and the same text in every language.
	KindValueName Kind = "value_name"
	KindType      Kind = "type"
	KindFunction  Kind = "function"
	// KindConstant is a builtin that has a value and is not called (pi); its signature is
	// only a return type.
	KindConstant Kind = "constant"
	KindMethod   Kind = "method"
	KindProperty Kind = "property"
	KindNamedArg Kind = "named_argument"
	// KindCommand is a command, option, shell or debugger command or environment variable; it is
	// not a name in the language, so programs cannot use it.
	KindCommand Kind = "command"
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
	// Common is the code's common name ("common_name"): a concept written with international
	// notation or a proper name (sin, AdamW, json) has the same name in every writing language. It
	// is written only here, never in a language file; Load gives it to every language.
	Common string
	// Module is the standard library module ("module": tr/layers) whose public name the code is;
	// empty for a core code. codes_generated.go carries it as the libraryModules table.
	Module string
	// Familiar holds names familiar from other languages and libraries ("familiar": print's
	// println, none's None, null and nil, func's def): they are not accepted, a message only
	// suggests the code (2.3a). They belong to no writing language; Load gives them to every
	// language, except one equal to that language's own name.
	Familiar []string
	// Example is the code's one-line working example written with identities ("example":
	// TR_print(1 < 2)); every language shows it in its view (17.4, 17.7). A command-line code's
	// example is a command line with the identities' names (tr check eğit.tr).
	Example string
	// Pure ("pure") marks a builtin whose call only gives a new value (13.10): it uses no input or
	// output, file, clock or randomness, changes neither its receiver nor its arguments and does
	// not wait. Such a call written as a statement on its own is a check error (H1115).
	Pure bool
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

// familiarNames tanıdık adı olan kodlardır (kod → tanıdık adlar).
func familiarNames() map[Code][]string {
	loadDefinitions()
	out := map[Code][]string{}
	for _, k := range defOrder {
		if f := defs[k].Familiar; len(f) > 0 {
			out[k] = f
		}
	}
	return out
}

// FamiliarCode returns the code a name familiar from another language stands for in program text's
// other place (keywords, values, types, functions: None → TR_kw_none, def → TR_kw_func,
// println → TR_fn_print; 2.3a), for a message's suggestion.
func FamiliarCode(name string) (Code, bool) { return familiarAt(PlaceOther, name) }

// FamiliarMember returns the member a name familiar from another language stands for (len →
// TR_mem_length): in TR it is written after a dot (x.TR_length).
func FamiliarMember(name string) (Code, bool) { return familiarAt(PlaceMember, name) }

// familiarAt p yerindeki kodların tanıdık adının kodudur (aynı adı taşıyan iki koddan küçüğü).
func familiarAt(p Place, name string) (Code, bool) {
	familiarOnce.Do(func() {
		for k, names := range familiarNames() {
			if at, ok := PlaceOf(k); ok && at < placeCount {
				if familiarByPlace[at] == nil {
					familiarByPlace[at] = map[string]Code{}
				}
				for _, n := range names {
					if prev, ok := familiarByPlace[at][n]; !ok || k < prev {
						familiarByPlace[at][n] = k
					}
				}
			}
		}
	})
	k, ok := familiarByPlace[p][name]
	return k, ok
}

var (
	familiarOnce    sync.Once
	familiarByPlace [placeCount]map[string]Code
)

func loadDefinitions() {
	defOnce.Do(func() {
		defs, defOrder, defErr = ParseDefinitions(definitionsJSON)
	})
}

type rawSignature struct {
	Param  []string            `json:"params"`
	Named  [][]json.RawMessage `json:"named"`
	Return string              `json:"returns"`
}

type rawDefinition struct {
	Kind       string                  `json:"kind"`
	Types      []string                `json:"types"`
	Param      []string                `json:"params"`
	Named      [][]json.RawMessage     `json:"named"`
	Return     string                  `json:"returns"`
	Signatures map[string]rawSignature `json:"signatures"`
	Related    []string                `json:"related"`
	Common     string                  `json:"common_name"`
	Module     string                  `json:"module"`
	Familiar   []string                `json:"familiar"`
	Example    string                  `json:"example"`
	Pure       bool                    `json:"pure"`
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
					errorf("%s params: %v", k, err)
				}
				im.Params = append(im.Params, ti)
			}
			for _, a := range named {
				name, err := parseNamed(a)
				if err != nil {
					errorf("%s named: %v", k, err)
					continue
				}
				im.Named = append(im.Named, name)
			}
			if ret != "" {
				ti, err := ParseType(ret, false)
				if err != nil {
					errorf("%s returns: %v", k, err)
				}
				im.Return = ti
			}
			return im
		}
		t.Signature = sig(h.Param, h.Named, h.Return)
		for _, typ := range codeList("types", h.Types) {
			t.Members = append(t.Members, TypeSignature{Type: typ, Signature: t.Signature})
		}
		if len(h.Signatures) > 0 {
			if len(h.Types) > 0 || len(h.Param) > 0 || len(h.Named) > 0 || h.Return != "" {
				errorf("%s: \"types\", \"params\", \"named\" and \"returns\" must not be written when \"signatures\" is present", k)
			}
			for s, hi := range h.Signatures {
				typ, err := Parse(s)
				if err != nil {
					errorf("%s signatures: %v", k, err)
					continue
				}
				t.Members = append(t.Members, TypeSignature{Type: typ, Signature: sig(hi.Param, hi.Named, hi.Return)})
			}
			sort.Slice(t.Members, func(i, j int) bool { return t.Members[i].Type < t.Members[j].Type })
		}
		t.Related = codeList("related", h.Related)
		t.Common, t.Familiar, t.Example, t.Pure = h.Common, h.Familiar, h.Example, h.Pure
		if t.Module = h.Module; t.Module != "" {
			switch c := k.Class(); {
			case c != ClassType && c != ClassFunction && c != ClassConstant && c != ClassMember && c != ClassNamedArg:
				errorf("%s: a module is written only on the public names of a library (type, fn, const, mem and arg codes)", k)
			case !strings.HasPrefix(t.Module, "tr/"):
				errorf("%s: module %q is not a standard library module (tr/…)", k, t.Module)
			}
		}
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

// SourceLanguage Türkçe dil dosyasının kodudur; Türkçe yazım dillerinden biridir (bkz. Source).
const SourceLanguage = "tr"

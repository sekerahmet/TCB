package codes

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

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
		for _, x := range t.Related {
			if !live(x) {
				errorf("%s: related code %s is not in the language file", k, x)
			} else if x == k {
				errorf("%s is listed as related to itself", k)
			}
		}
		if t.Pure && (k.IsLibrary() || t.Kind != KindFunction && t.Kind != KindMethod) {
			errorf("%s: only a builtin function or method is marked pure (13.10)", k)
		}
		if k.IsLibrary() {
			if len(t.Members) > 0 || len(t.Signature.Params) > 0 || len(t.Signature.Named) > 0 || !t.Signature.Return.Empty() {
				errorf("%s: a library code's signature and types are written only in the library's source", k)
			}
			continue
		}
		switch t.Kind {
		case KindMethod, KindProperty:
			if len(t.Members) == 0 {
				errorf("%s: the types a member applies to (types or signatures) must be written", k)
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
		case KindType:
			// Tip çağrılınca kurar ve çevirir (3.12): çağrının imzası tipindir, dönüşü tipin kendisi.
			sig := t.Signature
			if len(t.Members) > 0 {
				errorf("%s: a type has no member signatures", k)
			}
			if len(sig.Params) > 0 || len(sig.Named) > 0 || !sig.Return.Empty() {
				if r := sig.Return; len(r.Types) != 1 || r.Types[0] != k || r.Optional {
					errorf("%s: a type's call returns the type itself (3.12)", k)
				}
				checkSig(k, "", sig)
			}
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
	}
	return errs
}

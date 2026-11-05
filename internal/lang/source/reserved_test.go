package source

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// reservedDefinitions are the places of 2.3a with a name that is a valid identity: the name the
// message speaks of and the place's text.
var reservedDefinitions = []struct{ name, code string }{
	{"TR_print", "TR_var TR_print = 1\n"},
	{"TR_min", "TR_var TR_min, b = 1, 2\n"},
	{"TR_pi", "TR_const TR_pi = 3\n"},
	{"TR_sum", "TR_func TR_sum() { }\n"},
	{"TR_init", "TR_func TR_init() { }\n"},
	{"TR_Tensor", "TR_struct TR_Tensor { }\n"},
	{"TR_Int", "TR_enum TR_Int { A }\n"},
	{"TR_Tensor", "TR_interface TR_Tensor {\n    TR_func a() -> TR_Int\n}\n"},
	{"TR_step", "TR_enum R { TR_step(a: TR_Int) }\n"},
	{"TR_lr", "TR_func f(TR_lr: TR_Float) { }\n"},
	{"TR_lr", "TR_struct K {\n    TR_func TR_init(TR_lr: TR_Int) { }\n}\n"},
	{"TR_abs", "TR_func f() {\n    TR_var TR_abs = 1\n}\n"},
	{"TR_abs", "TR_import \"tr/layers\" TR_as TR_abs\n"},
	{"TR_max", "TR_var s = \"\\(TR_func TR_max() { })\"\n"},
	{"TR_weight", "TR_struct K {\n    TR_weight: TR_Int\n}\n"},
	{"TR_next", "TR_struct K {\n    TR_next: TR_Int = 1\n}\n"},
	{"TR_forward", "TR_struct K {\n    TR_func TR_forward() { }\n}\n"},
	{"TR_length", "TR_interface I {\n    TR_func TR_length() -> TR_Int\n}\n"},
	{"TR_next", "TR_interface I {\n    TR_func TR_next() -> TR_Int?\n}\n"},
	{"TR_forward", "TR_enum E {\n    A\n    TR_func TR_forward() { }\n}\n"},
	{"TR_next", "TR_enum E {\n    A\n    TR_func TR_next() -> TR_Int? { TR_return TR_none }\n}\n"},
	{"TR_x", "TR_enum E { A(TR_x: TR_Int) }\n"},
}

// Kural (2.3a): kullanıcının adı TR_ ile başlamaz, geçerli bir kimlik olsa da. Kapı sözdizimi
// denetiminden sonra ve tip denetiminden bağımsızdır (modül çalışma anında tip denetimsiz okunur);
// ileti tanımın adını yazıldığı gibi söyler, görünüm adı sızmaz.
func TestReservedNamesStopEveryDefinition(t *testing.T) {
	for _, c := range reservedDefinitions {
		for _, typed := range []bool{true, false} {
			r := Prepare(c.code, Options{Body: true, TypeCheck: typed})
			if r.Stage != StageType || len(r.Errors) != 1 || !messages.Has(r.Errors[0], messages.H0004) {
				t.Errorf("%q: H0004 bekleniyordu: %q %v", c.code, r.Stage, r.Errors)
				continue
			}
			if !strings.Contains(r.Errors[0].Error(), "'"+c.name+"'") {
				t.Errorf("%q: ileti adı yazıldığı gibi söylemeli: %v", c.code, r.Errors[0])
			}
			e, ok := messages.Find(r.Errors[0])
			if !ok || len(e.Fixes) != 1 {
				t.Errorf("%q: düzeltme yok: %v", c.code, r.Errors[0])
			}
		}
	}
}

// Kural (2.3a, 2.3b): H0004'ün önerisi kopyalanınca çalışır: bir sonraki denetim onu ne hata ne de
// yazılış uyarısıyla H0004'e geri götürür. Bütün kimlikler için: tanıdık bir ad (sum, print, if)
// tanımlanınca o yerde sıradan addır.
func TestTheReservedNameSuggestionRuns(t *testing.T) {
	for _, k := range codes.Codes() {
		spelling := codes.Spelling(k)
		for _, place := range []string{"TR_var %s = 1\n", "TR_func %s() { }\n", "TR_func f(%s: TR_Int) { }\n"} {
			first := Prepare(fmt.Sprintf(place, spelling), Options{Body: true, TypeCheck: true})
			if len(first.Errors) == 0 {
				continue // kimlik o yerde tanım değildir (TR_if)
			}
			e, ok := messages.Find(first.Errors[0])
			if !ok || !messages.Has(e, messages.H0004) {
				continue
			}
			fixed := Prepare(fmt.Sprintf(place, e.Fixes[0].Replacement), Options{Body: true, TypeCheck: true})
			if fixed.Stage != "" {
				t.Errorf("%s: önerisi %q çalışmıyor: %v", spelling, e.Fixes[0].Replacement, fixed.Errors)
			}
			for _, w := range fixed.Warnings { // TR_Int → Int yerin yazılışına uymayabilir, H0004'e dönmez
				if e, ok := messages.Find(w); ok && len(e.Fixes) > 0 && codes.IsReservedName(e.Fixes[0].Replacement) {
					t.Errorf("%s: uyarının önerisi ayrılmış bir ad: %v", spelling, w)
				}
			}
		}
	}
	for _, c := range []struct{ code, name string }{
		{"TR_const TR_PI2 = 6.28\n", "PI2"},
		{"TR_struct TR_Foo { }\n", "Foo"},
		{"TR_func TR_sum() { }\n", "sum"},
	} {
		r := Prepare(c.code, Options{Body: true, TypeCheck: true})
		fixed := strings.Replace(c.code, "TR_"+c.name, c.name, 1)
		if e, ok := messages.Find(r.Errors[0]); !ok || e.Fixes[0].Replacement != c.name {
			t.Errorf("%q: önerisi %s olmalı: %v", c.code, c.name, r.Errors)
		}
		if s := Prepare(fixed, Options{Body: true, TypeCheck: true}); s.Stage != "" || len(s.Warnings) != 0 {
			t.Errorf("%q: %v %v", fixed, s.Errors, s.Warnings)
		}
	}
}

// Kural (2.3a): TR_ tek başına da kullanıcının adı değildir. Kırpılınca geçerli ad kalmayan yazımın
// iletisi öneri vermez: düzeltme de yoktur, metinde örnek de.
func TestAPrefixAloneHasNoSuggestion(t *testing.T) {
	for _, code := range []string{
		"TR_var TR_ = 2\n", "TR_const TR_ = 2\n", "TR_func TR_() { }\n", "TR_struct TR_ { }\n", "TR_func f(TR_: TR_Int) { }\n",
		"TR_var a, TR_ = 1, 2\n", "TR_for TR_ TR_in [1] { }\n", "TR_try { } TR_catch TR_ { }\n", "TR_var l = [1 TR_for TR_ TR_in [1]]\n",
		"TR_var g = (TR_) => 1\n", "TR_var TR_1 = 1\n", "TR_var TR__ = 1\n", "TR_import \"m\" TR_as TR_\n",
	} {
		r := Prepare(code, Options{Body: true, TypeCheck: true})
		if r.Stage == "" || !messages.Has(r.Errors[0], messages.H0004) {
			t.Errorf("%q: H0004 bekleniyordu: %v", code, r.Errors)
			continue
		}
		e, _ := messages.Find(r.Errors[0])
		if len(e.Fixes) != 0 || strings.Contains(e.Error(), "(ör.") {
			t.Errorf("%q: öneri olmamalı: %v %v", code, e, e.Fixes)
		}
	}
	if r := Prepare("TR_var TR_x = 1\n", Options{Body: true}); !strings.Contains(r.Errors[0].Error(), "(ör. x)") {
		t.Errorf("önerisi olan ad örneğini söyler: %v", r.Errors)
	}
}

// Kural (17): kullanıcının yapısında dilin çağırdığı iki üye kodla yazılır: kurucu ve dolaşmanın
// TR_next()'i. Dışlarında kod yoktur; yukarıdaki tablo öbür yerlerini sınar.
func TestAStructHoldsTheMembersTheLanguageCalls(t *testing.T) {
	code := "TR_struct S {\n    n: TR_Int = 0\n    TR_func TR_init() { }\n    TR_func TR_next() -> TR_Int? { TR_return TR_none }\n}\n"
	if r := Prepare(code, Options{Body: true}); r.Stage != "" {
		t.Errorf("TR_init ve TR_next yapıda yazılır: %v", r.Errors)
	}
}

// Kural (17.1): kitaplık kendi adlarını tanımlar; kapı yalnız onun metnine açılır.
func TestTheLibraryDefinesItsOwnNames(t *testing.T) {
	for _, c := range reservedDefinitions {
		if r := Prepare(c.code, Options{Body: true, Library: "layers.tr"}); r.Stage != "" {
			t.Errorf("%q: kitaplığın adıdır: %v", c.code, r.Errors)
		}
	}
}

// Kural: hata ayıklayıcının konsolu programın kapsamında çalışır; yerleşik oradan da ezilmez.
func TestParseTypedKeepsTheGate(t *testing.T) {
	if _, errs := ParseTyped("TR_var TR_print = 1"); len(errs) != 1 || !messages.Has(errs[0], messages.H0004) {
		t.Errorf("H0004 bekleniyordu: %v", errs)
	}
	if prog, errs := ParseTyped("TR_print(1)"); len(errs) != 0 || len(prog.Statements) != 1 {
		t.Errorf("çağrı tanım değildir: %v", errs)
	}
}

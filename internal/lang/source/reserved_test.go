package source

import (
	"strings"
	"testing"

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

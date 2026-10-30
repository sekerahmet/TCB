package interp

import (
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (6.1, 7.1; ileti ölçütü): tipsiz parametrenin, alanın ve dönüşün iletisi düzeltilmiş satırı
// verir; satır olduğu gibi kopyalanınca modül denetimden geçer ve program istenen sonucu verir.
// Tahmin (H1110, çağrılardan) de çalışan bir yazımdır; uygulanamaz sayılması tahmin olduğu içindir.
func TestTypedSignatureSuggestionRuns(t *testing.T) {
	for _, c := range []struct {
		name, module, wrong, call, want string
		code                            messages.Code
	}{
		{"varsayılanlı parametre", "TR_func f(a: TR_Int, b = 2) -> TR_Int { # ERROR H1109\n    TR_return a * b\n}\n",
			"TR_func f(a: TR_Int, b = 2) -> TR_Int { # ERROR H1109", "TR_print(f(3))", "6\n", messages.H1109},
		{"çağrılardan parametre", "TR_func f(a) -> TR_Float { # ERROR H1110\n    TR_return a * 2.0\n}\nTR_func g() -> TR_Float {\n    TR_return f(1.5)\n}\n",
			"TR_func f(a) -> TR_Float { # ERROR H1110", "TR_print(g())", "3.0\n", messages.H1110},
		{"varsayılanlı alan", "TR_struct S {\n    n = 4 # ERROR H1109\n}\n", "n = 4 # ERROR H1109", "TR_print(S().n + 1)", "5\n", messages.H1109},
		{"dönüş", "TR_func f(a: TR_Int) { # ERROR H1112\n    TR_return a + 1\n}\n", "TR_func f(a: TR_Int) { # ERROR H1112", "TR_print(f(1))", "2\n", messages.H1112},
	} {
		_, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": c.module, "ana.tr": "TR_import \"m\"\n"})
		var mh *ModuleError
		if !errors.As(err, &mh) || len(mh.Errors) != 1 || !messages.Has(mh.Errors[0], c.code) {
			t.Fatalf("%s: modülün %v hatası bekleniyordu: %v", c.name, c.code, err)
		}
		_, fix, ok := strings.Cut(mh.Errors[0].Error(), ":\n")
		if !ok {
			t.Fatalf("%s: iletide önerilen yazım yok: %s", c.name, mh.Errors[0])
		}
		fixed := strings.Replace(c.module, c.wrong, fix, 1)
		out, err := runWithModules(t, t.TempDir(), map[string]string{"m.tr": fixed, "ana.tr": "TR_import \"m\"\n" + c.call + "\n"})
		if err != nil || out != c.want {
			t.Errorf("%s: önerilen yazım çalışmalı:\n%s\nçıktı %q, hata %v", c.name, fixed, out, err)
		}
	}
}

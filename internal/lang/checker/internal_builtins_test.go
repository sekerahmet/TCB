package checker

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// internalErrors kaynağı library adlı gömülü dosya sayarak (boş: kullanıcının programı) denetler ve
// H6998'lerin sayısını verir.
func internalErrors(t *testing.T, src, library string) int {
	t.Helper()
	p := parser.New(lexer.NewBody(src, true))
	prog := p.ParseModule()
	if len(p.Errs()) > 0 {
		t.Fatalf("sözdizimi: %v", p.Errs())
	}
	c := New()
	c.SetFile(ModuleFile, src)
	c.SetLibraryFile(library)
	c.CheckProgram(prog)
	n := 0
	for _, err := range c.Errs() {
		if messages.Has(err, messages.H6998) {
			n++
		}
	}
	return n
}

// Kural (11.7a): kitaplığın iç yerleşiğini yalnız tanım dosyasında adı yazılı modülün gömülü
// kaynağı çağırır ve anar; kullanıcının programında, aynı metnin kopyasında ve başka kitaplık
// dosyasında çağrı da anma da H6998'dir.
func TestInternalBuiltinOnlyInItsLibrary(t *testing.T) {
	src := "TR_func f() -> TR_Tensor { TR_return TR_cache_buffer([2, 3]) }\nTR_func g() { TR_var h = TR_cache_buffer }\n"
	for library, want := range map[string]int{"layers.tr": 0, "": 2, "optimizers.tr": 2, "layers_test.tr": 2} {
		if got := internalErrors(t, src, library); got != want {
			t.Errorf("dosya %q: %d H6998 bekleniyordu, %d", library, want, got)
		}
	}
	if got := internalErrors(t, "TR_func f() -> TR_Tensor { TR_return TR_zeros([2]) }\n", ""); got != 0 {
		t.Errorf("çekirdek yerleşiği H6998 verdi: %d", got)
	}
}

// İleti iç yerleşiği önermez: önerilen yazım kullanıcının programında çalışmazdı (11.7a).
func TestInternalBuiltinIsNeverSuggested(t *testing.T) {
	p := parser.New(lexer.NewBody("TR_var t = TR_cache_bufer([2])\n", true))
	p.ParseProgram()
	if got := strings.Join(p.Errors(), "\n"); got == "" || strings.Contains(got, "TR_cache_buffer") {
		t.Errorf("yakın kimlik önerisi: %q", got)
	}
	for _, src := range []string{"TR_var t = cache_buffer([2])\n", "TR_var t = onbellek_tamponu([2])\n"} {
		if got := strings.Join(check(t, src), "\n"); got == "" || strings.Contains(got, "TR_cache_buffer") || strings.Contains(got, "önbellek_tamponu") {
			t.Errorf("%q: öneri %q", src, got)
		}
	}
}

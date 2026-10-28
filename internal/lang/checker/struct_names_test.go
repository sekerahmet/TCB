package checker

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Metotta alan ya da metot adıyla yazılmış düz ad (7.3) dıştaki bir ada çözülse de (H1024) çözülmese de
// (H1025) hatadır; düzeltme adı TR_self ile yazar ve uygulanabilir. Metodun yereli, parametresi ve
// içindeki fonksiyonun parametresi alanla karışmaz.
func TestPlainMemberNameInAMethod(t *testing.T) {
	const src = `TR_var x = 99
TR_struct N {
    x = 1
    y = 2
    TR_func m() { TR_return x }
    TR_func k() { TR_return y }
    TR_func j() { m() }
    TR_func a() { x = 5 }
    TR_func yerel(y) {
        TR_var x = 3
        TR_var f = (m) => m + x
        TR_return f(y) + TR_self.x + TR_self.m()
    }
}`
	want := []struct {
		code messages.Code
		fix  string
	}{
		{messages.H1024, "TR_self.x"},
		{messages.H1025, "TR_self.y"},
		{messages.H1025, "TR_self.m"},
		{messages.H1024, "TR_self.x"},
	}
	errs := checkCoded(t, src)
	if len(errs) != len(want) {
		t.Fatalf("want %d errors, got %v", len(want), errs)
	}
	for i, w := range want {
		pos, ok := messages.Find(errs[i])
		if !ok || !messages.Has(errs[i], w.code) || len(pos.Fixes) != 1 {
			t.Errorf("error %d: want %v with a fix, got %v", i, w.code, errs[i])
			continue
		}
		if f := pos.Fixes[0]; f.Replacement != w.fix || !f.Applicable {
			t.Errorf("error %d: fix %+v, want %q applicable", i, f, w.fix)
		}
	}
	// önerilen yazım denetimden geçer
	fixed := `TR_var x = 99
TR_struct N {
    x = 1
    TR_func m() { TR_return TR_self.x }
    TR_func j() { TR_self.m() }
    TR_func a() { TR_self.x = 5 }
}`
	if errs := checkCoded(t, fixed); len(errs) != 0 {
		t.Errorf("the suggested spelling: %v", errs)
	}
}

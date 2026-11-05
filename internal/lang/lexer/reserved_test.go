package lexer

import "testing"

// Kural (2.3a, 2.3b): önerilen ad öneki kırpar, harf büyüklüğünü korur: yerin yazılışına uyar. Kalan
// geçerli bir ad değilse öneri yoktur.
func TestUserNameDropsThePrefix(t *testing.T) {
	for lit, want := range map[string]string{
		"TR_PI2": "PI2", "TR_Foo": "Foo", "TR_sum": "sum", "TR_x": "x", "TR_TR_x": "x", "TR_tr_x": "tr_x",
		"TR__x": "_x", "TR_": "", "TR__": "", "TR_1": "", "TR_TR_": "", "TR_öğrenci": "öğrenci",
		"TR__1": "_1", "TR_" + "\u0663x": "", "TR_" + "\u0301x": "", "TR_" + "\u0422R_x": "", "TR_" + "\u0430": "\u0430",
	} {
		if got := UserName(lit); got != want {
			t.Errorf("UserName(%q) = %q, beklenen %q", lit, got, want)
		}
		if want == "" {
			continue
		}
		l := NewBody("TR_var "+want+" = 1\n", true)
		if allOf(l) == nil || len(l.Errs()) != 0 {
			t.Errorf("UserName(%q) = %q: önerilen ad okunmuyor: %v", lit, want, l.Errors())
		}
	}
}

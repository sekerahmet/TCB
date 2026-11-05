package lexer

import "testing"

// Kural (2.3a, 2.3b): önerilen ad öneki kırpar, harf büyüklüğünü korur: yerin yazılışına uyar ve
// kendisi ad olur.
func TestUserNameDropsThePrefix(t *testing.T) {
	for lit, want := range map[string]string{
		"TR_PI2": "PI2", "TR_Foo": "Foo", "TR_sum": "sum", "TR_x": "x", "TR_TR_x": "x", "TR_tr_x": "tr_x",
		"TR__x": "_x", "TR_": "x", "TR__": "x", "TR_1": "x1", "TR_öğrenci": "öğrenci",
	} {
		if got := UserName(lit); got != want {
			t.Errorf("UserName(%q) = %q, beklenen %q", lit, got, want)
		}
	}
}

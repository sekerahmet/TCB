package main

import "testing"

// Kural (5.1): ayrıştırıcının reddettiği eski yazım (x := 5) çevrilmez, elle çevrilmek üzere
// bildirilir; yeni yazımdaki metne deyim kuralları dokunmaz.
func TestStatementRulesLeaveNewTextAlone(t *testing.T) {
	in := "# TR 4\nTR_var x = 5\nTR_func f() {\n    TR_return x\n}\n"
	if out, probs := convertValues("", in); out != in || len(probs) > 0 {
		t.Errorf("yeni yazım değişti:\n%s\n%v", out, probs)
	}
	old := "# TR 4\nx := 5\n"
	if out, probs := convertValues("", old); out != old || len(probs) != 1 {
		t.Errorf("':=' elle çevrilmek üzere bildirilmeliydi: %q %v", out, probs)
	}
}

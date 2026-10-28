package main

import "testing"

// Kural (5.1, 6.1, 6.7): ayrıştırıcının reddettiği eski yazım (x := 5, f(): T, f(() => x)) çevrilmez, elle çevrilmek
// üzere bildirilir; yeni yazımdaki metne deyim kuralları dokunmaz.
func TestStatementRulesLeaveNewTextAlone(t *testing.T) {
	in := "# TR 4\nTR_var x = 5\nTR_func f() -> TR_Int {\n    TR_return x\n}\n"
	if out, probs := convertValues("", in); out != in || len(probs) > 0 {
		t.Errorf("yeni yazım değişti:\n%s\n%v", out, probs)
	}
	for _, old := range []string{"# TR 4\nx := 5\n", "# TR 4\nTR_func f(): TR_Int { TR_return 1 }\n", "# TR 4\nTR_print(TR_error_code(() => 1))\n"} {
		if out, probs := convertValues("", old); out != old || len(probs) != 1 {
			t.Errorf("eski yazım elle çevrilmek üzere bildirilmeliydi: %q %v", out, probs)
		}
	}
}

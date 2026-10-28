package main

import "testing"

// Kural (5.1, 6.1): deyim ve fonksiyon kurallarının her biri bir giriş ve beklenen çıktıyla; çevrilmiş
// metin bir daha çevrilmez, Go testlerindeki parçada da (members yanlış) aynı kurallar uygulanır.
// Ayrıştırıcının reddettiği eski yazım (x := 5) çevrilmez, elle çevrilmek üzere bildirilir.
func TestStatementRulesConvert(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		// 6.1: dönüş tipi '->' ile yazılır
		{"TR_func f(x: TR_Int): TR_Int { TR_return x }\n", "TR_func f(x: TR_Int) -> TR_Int { TR_return x }\n"},
		{"TR_func f() -> TR_Int { TR_return 1 }\n", "TR_func f() -> TR_Int { TR_return 1 }\n"},
		{"TR_func f(a: TR_Int = 2) { TR_return a }\n", "TR_func f(a: TR_Int = 2) { TR_return a }\n"},
	} {
		in, want := "# TR 4\n"+c.in, "# TR 4\n"+c.want
		out, probs := convertValues("", in)
		if len(probs) > 0 || out != want {
			t.Errorf("%q:\n%s\nbeklenen:\n%s\nsorun: %v", c.in, out, want, probs)
		}
		if again, _ := convertValues("", out); again != out {
			t.Errorf("ikinci çeviri metni değiştirdi:\n%s", again)
		}
		if part, _ := convertValuesWith("", c.in, false); part != c.want {
			t.Errorf("Go parçası %q:\n%s", c.in, part)
		}
	}
	old := "# TR 4\nx := 5\n"
	if out, probs := convertValues("", old); out != old || len(probs) != 1 {
		t.Errorf("':=' elle çevrilmek üzere bildirilmeliydi: %q %v", out, probs)
	}
}

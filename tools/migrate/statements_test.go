package main

import "testing"

// Kural (5.1, 6.x): deyim ve fonksiyon kurallarının her biri bir giriş ve beklenen çıktıyla; çevrilmiş
// metin bir daha çevrilmez, Go testlerindeki parçada da (members yanlış) aynı kurallar uygulanır.
func TestStatementRulesConvert(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		// 5.1: kısa tanım değişken tanımı olur
		{"x := 5\nTR_print(x)\n", "TR_var x = 5\nTR_print(x)\n"},
		{"TR_func f() {\n    toplam := [1] + [2]\n    TR_return toplam\n}\n", "TR_func f() {\n    TR_var toplam = [1] + [2]\n    TR_return toplam\n}\n"},
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
}

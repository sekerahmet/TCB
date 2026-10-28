package main

import "testing"

// Kural (6.6): fonksiyon ifadesinin tek yazımı oktur; adsız ve ifade yerindeki adlı TR_func ile parantezsiz
// ok çevrilir, deyim olarak yazılmış fonksiyon ve yapı metodu çevrilmez. Çevrilmiş metin bir daha
// çevrilmez, Go testlerindeki parçada da aynı kurallar uygulanır. Ayrıştırıcının reddettiği eski yazım
// (x := 5, f(): T, f(() => x)) çevrilmez, elle çevrilmek üzere bildirilir.
func TestStatementRulesConvert(t *testing.T) {
	for _, c := range []struct {
		in, want string
		probs    int
	}{
		{"TR_var f = TR_func(x) { TR_return x * 2 }\n", "TR_var f = (x) => { TR_return x * 2 }\n", 0},
		{"TR_var g = x => x + 1\nTR_print([1].TR_map(y => y))\n", "TR_var g = (x) => x + 1\nTR_print([1].TR_map((y) => y))\n", 0},
		{"TR_var r = (TR_func yedi() { TR_return 7 })()\n", "TR_var r = (() => { TR_return 7 })()\n", 0},
		{"TR_func f(g) { TR_return g() }\nTR_print(f(TR_func() { TR_return 5 }))\n", "TR_func f(g) { TR_return g() }\nTR_print(f { 5 })\n", 0},
		{"TR_func f(a, g) { TR_return g() }\nTR_print(f(1, TR_func() { TR_return [a] }))\n", "TR_func f(a, g) { TR_return g() }\nTR_print(f(1) { [a] })\n", 0},
		{"TR_func f(x) { TR_return x }\nTR_struct A {\n    TR_func m() { TR_return 1 }\n}\n", "TR_func f(x) { TR_return x }\nTR_struct A {\n    TR_func m() { TR_return 1 }\n}\n", 0},
		{"TR_var h = TR_func(x) -> TR_Int { TR_return x }\n", "TR_var h = TR_func(x) -> TR_Int { TR_return x }\n", 1},
		{"TR_var k = TR_func kendi(n) { TR_return kendi(n) }\n", "TR_var k = TR_func kendi(n) { TR_return kendi(n) }\n", 1},
	} {
		in, want := "# TR 4\n"+c.in, "# TR 4\n"+c.want
		out, probs := convertValues("", in)
		if len(probs) != c.probs || out != want {
			t.Errorf("%q:\n%s\nbeklenen:\n%s\nsorun: %v", c.in, out, want, probs)
		}
		if again, _ := convertValues("", out); again != out {
			t.Errorf("ikinci çeviri metni değiştirdi:\n%s", again)
		}
		if part, _ := convertValuesWith("", c.in, false); part != c.want {
			t.Errorf("Go parçası %q:\n%s", c.in, part)
		}
	}
	for _, old := range []string{"# TR 4\nx := 5\n", "# TR 4\nTR_func f(): TR_Int { TR_return 1 }\n", "# TR 4\nTR_print(TR_error_code(() => 1))\n"} {
		if out, probs := convertValues("", old); out != old || len(probs) != 1 {
			t.Errorf("eski yazım elle çevrilmek üzere bildirilmeliydi: %q %v", out, probs)
		}
	}
}

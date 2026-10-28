package main

import "testing"

// Kural (6.7): TR_it ya da TR_acc okuyan sondaki blok son argümandaki oka döner (iç içede x2), son
// argümandaki parametresiz ok sondaki blok olur. Çevrilmiş metin bir daha çevrilmez, Go testlerindeki
// parçada da aynı kurallar uygulanır. Ayrıştırıcının reddettiği eski yazım (x := 5, f(): T) çevrilmez,
// elle çevrilmek üzere bildirilir.
func TestStatementRulesConvert(t *testing.T) {
	for _, c := range []struct {
		in, want string
		probs    int
	}{
		{"TR_var l = [1, 2]\nTR_print(l.TR_map { TR_it * 2 })\n", "TR_var l = [1, 2]\nTR_print(l.TR_map((x) => x * 2))\n", 0},
		{"TR_var l = [1]\nTR_print(l.TR_reduce(0) { TR_acc + TR_it })\n", "TR_var l = [1]\nTR_print(l.TR_reduce(0, (acc, x) => acc + x))\n", 0},
		{"TR_func f(a, g) { TR_return g(a) }\nTR_print(f(5) { TR_it * 2 })\n", "TR_func f(a, g) { TR_return g(a) }\nTR_print(f(5, (x) => x * 2))\n", 0},
		{"TR_print([[1, 2]].TR_map { TR_it.TR_map { TR_it * 10 } })\n", "TR_print([[1, 2]].TR_map((x) => x.TR_map((x2) => x2 * 10)))\n", 1},
		{"TR_var x = 3\nTR_print([1].TR_map { TR_it * x })\n", "TR_var x = 3\nTR_print([1].TR_map((x2) => x2 * x))\n", 1},
		{"TR_print(TR_error_code(() => { TR_var z = 1 / 0 }))\n", "TR_print(TR_error_code { TR_var z = 1 / 0 })\n", 0},
		{"TR_func f(a, g) { TR_return g() }\nTR_print(f(1, () => a))\n", "TR_func f(a, g) { TR_return g() }\nTR_print(f(1) { a })\n", 0},
		{"TR_print(TR_error_code(() => {\n    TR_var z = 1 / 0\n}))\n", "TR_print(TR_error_code {\n    TR_var z = 1 / 0\n})\n", 0},
		{"TR_print(TR_error_code(() => { TR_return 1 }))\n", "TR_print(TR_error_code(() => { TR_return 1 }))\n", 1},
		{"TR_print(TR_error_code(() => { TR_print([1].TR_map { TR_it }) }))\n", "TR_print(TR_error_code { TR_print([1].TR_map((x) => x)) })\n", 0},
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
	for _, old := range []string{"# TR 4\nx := 5\n", "# TR 4\nTR_func f(): TR_Int { TR_return 1 }\n"} {
		if out, probs := convertValues("", old); out != old || len(probs) != 1 {
			t.Errorf("eski yazım elle çevrilmek üzere bildirilmeliydi: %q %v", out, probs)
		}
	}
}

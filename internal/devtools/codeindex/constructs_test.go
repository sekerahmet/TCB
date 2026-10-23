package codeindex

import (
	"os"
	"path/filepath"
	"testing"
)

// Kural: her bildirimin kullandığı dil yapıları (çağrı, metin yerleştirmesi, indeks, liste ...) bildirim başına bir kez
// kaydedilir; ad ve değişmez değer kaydedilmez. Kod grafiği bunları yorumlayıcının Go işlevlerine bağlar (deneme 9 H1,
// deneme 10; değer dili tasarımı 3. tur).
func TestConstructsAreRecordedPerDeclaration(t *testing.T) {
	dir := t.TempDir()
	src := "fonksiyon f(x) {\n    değişken l = [x, x]\n    yazdır(\"{x} {l[0]}\")\n    sonuç g(x)\n}\n\nfonksiyon g(y) {\n    sonuç y\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]map[string]int{}
	for _, c := range idx.Constructs {
		if kinds[c.In] == nil {
			kinds[c.In] = map[string]int{}
		}
		kinds[c.In][c.Kind]++
	}
	f := kinds["function:m.tr#f"]
	for _, k := range []string{"CallExpression", "InterpolatedString", "ListLiteral", "IndexExpression"} {
		if f[k] != 1 {
			t.Errorf("f: %s recorded %d times, want once: %v", k, f[k], f)
		}
	}
	if f["Identifier"] != 0 || len(kinds["function:m.tr#g"]) != 0 {
		t.Errorf("names and a body without constructs record nothing: %v", kinds)
	}
}

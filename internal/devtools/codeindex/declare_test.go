package codeindex

import (
	"os"
	"path/filepath"
	"testing"
)

// Kural: test bloğunda ya da gövdede yazılan `aktar` ifadesi de modülden modüle `imports` bağıdır (testler yardımcı
// modülünü böyle alır) ve takma adı dosyada tek bir modülü gösteriyorsa çözülür; aynı takma ad iki modülü gösteriyorsa
// (blok kapsamı izlenmediği için) yanlış çözülmesin diye bağlanmaz. Denetim: bu test.
func TestImportsInsideTestBlocksAreModuleImports(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"tests/yardim.tr": "fonksiyon iki() {\n    sonuç 2\n}\n",
		"tests/baska.tr":  "fonksiyon iki() {\n    sonuç 22\n}\n",
		"tests/a_test.tr": `test "birinci" {
    aktar "yardim" olarak m
    yazdır(m.iki())
}

test "ikinci" {
    aktar "yardim" olarak y
    yazdır(y.iki())
}

test "üçüncü" {
    aktar "baska" olarak k
    aktar "yardim" olarak k
    yazdır(k.iki())
}
`,
	}
	for rel, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]bool{}
	for _, e := range idx.Edges {
		if e.Kind == EdgeImports {
			imports[e.From+">"+e.To] = true
		}
	}
	for _, want := range []string{"module:tests/a_test.tr>module:tests/yardim.tr", "module:tests/a_test.tr>module:tests/baska.tr"} {
		if !imports[want] {
			t.Errorf("missing imports edge %s in %v", want, imports)
		}
	}
	res := NewReader(idx).Impact("tests/yardim.tr", QueryOptions{})
	if res.Status != StatusOK || res.Affected == nil || len(res.Affected.Modules) == 0 || res.Affected.Modules[0].Name != "tests/a_test.tr" {
		t.Errorf("the importing test file must be a module that imports the helper: %+v", res.Affected)
	}
}

// Kural: bir dosyada görünen ad önce kendi tanımıdır, sonra takma adsız aktarılan modüllerinki, dolaylı olanlar dahil;
// aktarma sırasıyla, derinlik önce, bir modülün kendi tanımları aktardıklarından önce; ilk bulunan kazanır
// (lookupVisible). ana.tr b'yi sonra c'yi aktarır, b d'yi aktarır: x hem d'de hem c'de vardır, ana.tr'deki x d'ninkidir
// (derinlik önce); y hem b'de hem d'de vardır, b'ninkidir (modül aktardığından önce). Takma adlı aktarmanın adları
// görünür kılınmaz.
func TestAVisibleNameIsTheFirstInImportOrderDepthFirst(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"ana.tr": "aktar \"b\"\naktar \"c\"\naktar \"e\" olarak e\nyazdır(x, y, z)\n",
		"b.tr":   "aktar \"d\"\nsabit y = 2\n",
		"c.tr":   "sabit x = 3\n",
		"d.tr":   "sabit x = 4\nsabit y = 4\n",
		"e.tr":   "sabit z = 5\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	used := func(id string) bool { // ana.tr id'ye başvuruyor mu
		for _, o := range idx.Occurrences {
			if o.Node == id && o.Pos.File == "ana.tr" {
				return true
			}
		}
		return false
	}
	if !used("constant:d.tr#x") || used("constant:c.tr#x") || !used("constant:b.tr#y") || used("constant:d.tr#y") || used("constant:e.tr#z") {
		t.Error("ana.tr's x is d's (depth first), y is b's (a module before its imports), z of an aliased import is not visible")
	}
}

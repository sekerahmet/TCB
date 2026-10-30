package codeindex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

func TestAtFindsTheNameUnderTheCursor(t *testing.T) {
	r := NewReader(buildFixture(t))
	// "    yaz_alan(k)": yaz_alan'ın ilk, ortadaki ve son karakteri; ardındaki sütun adın dışında
	line, start := fixtureAt(t, "lib/geo_test.tr", "yaz_alan(k)")
	end := start + len([]rune("yaz_alan")) - 1
	for _, col := range []int{start, start + 4, end} {
		o, n := r.At("lib/geo_test.tr", line, col)
		if o == nil || n == nil || n.ID != "function:lib/geo.tr#yaz_alan" || o.Role != RoleTest {
			t.Errorf("col %d: %+v %+v", col, o, n)
		}
	}
	if o, _ := r.At("lib/geo_test.tr", line, end+1); o != nil && o.Node == "function:lib/geo.tr#yaz_alan" {
		t.Error("the column after the name is outside it")
	}
	if o, _ := r.At("lib/geo_test.tr", 99, 1); o != nil {
		t.Error("a line without references has nothing under the cursor")
	}
	// Bildirimin adı da bir başvurudur (tanım rolüyle): "tanıma git" tanımın kendisinde de çalışır.
	line, col := fixtureAt(t, "lib/geo.tr", "çevre_hesapla(k:")
	o, n := r.At("lib/geo.tr", line, col+2)
	if o == nil || o.Role != RoleDefinition || n.ID != "function:lib/geo.tr#çevre_hesapla" {
		t.Errorf("definition under cursor: %+v %+v", o, n)
	}
}

func TestWriteRoleOnAliasedVariable(t *testing.T) {
	r := NewReader(buildFixture(t))
	found := false
	for _, o := range r.Occurrences("variable:lib/geo.tr#sayaç") {
		found = found || o.Role == RoleWrite && o.Pos.File == "examples/ana.tr" && o.Pos.Line == fixtureLine(t, "examples/ana.tr", "g.sayaç = 7")
	}
	if !found {
		t.Errorf("g.sayaç = 7 writes the variable: %+v", r.Occurrences("variable:lib/geo.tr#sayaç"))
	}
}

func TestUnresolvedNamesAreCounted(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, trText(p, body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := "TR_func f() { # ERROR H1113\n    TR_return bilinmeyen_ad(1)\n}\n"
	write("a.tr", src)
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Unresolved) != 1 || idx.Unresolved[0].Name != "bilinmeyen_ad" ||
		fmt.Sprintf("%d:%d", idx.Unresolved[0].Pos.Line, idx.Unresolved[0].Pos.Col) != srcAt(t, src, "bilinmeyen_ad(1)", "bilinmeyen_ad", 1) {
		t.Errorf("unresolved: %+v", idx.Unresolved)
	}
}

// Projede lib/ yoksa `aktar "tr/ad"` gömülü kitaplığa çözülür; modül sanal yoldadır.
func TestEmbeddedLibraryModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tr"), trText(filepath.Join(dir, "main.tr"), "TR_import \"tr/layers\"\n\nTR_GPT(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := fstest.MapFS{"layers.tr": {Data: trText("layers.tr", "TR_struct TR_GPT {\n    n: TR_Int = 1\n}\n")}}
	idx, err := Build(dir, Options{Library: lib})
	if err != nil {
		t.Fatal(err)
	}
	fs := facts(idx)
	for _, want := range []string{
		"edge module:main.tr imports module:tr:lib/layers.tr EXTRACTED x1",
		"edge module:main.tr calls struct:tr:lib/layers.tr#TR_type_GPT EXTRACTED x1",
	} {
		if !hasFact(fs, want) {
			t.Errorf("missing %s in\n%v", want, fs)
		}
	}
	// Kitaplık verilmezse modül görünmez ve aktarma çözülmez.
	bare, _ := Build(dir, Options{})
	for _, e := range bare.Edges {
		if e.Kind == EdgeImports {
			t.Errorf("no library, no import edge: %+v", e)
		}
	}
}

// Her üst düzey bildirim dizinde bulunur: bu deponun kendi .tr dosyaları için, ast'yi bağımsız
// sayan bir döküm dizindeki düğümlerle karşılaştırılır.
func TestEveryDeclarationIndexed(t *testing.T) {
	root := reporoot.Root()
	idx, err := Build(root, Options{Dirs: []string{"lib", "examples", "tools"}, Tracked: true}) // geliştiricinin izlenmeyen dosyaları sayılmaz
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, n := range idx.Nodes {
		have[n.ID] = true
	}
	checked := 0
	for _, rel := range idx.Files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range declaredIDs(rel, readFile(string(data), false)) {
			checked++
			if !have[want] {
				t.Errorf("%s: declaration %s is not in the index", rel, want)
			}
		}
	}
	if checked < 100 {
		t.Errorf("only %d declarations checked; the repository's .tr files were not found", checked)
	}
}

// Aynı modül hem takma adsız hem takma adla aktarılabilir (examples/tr/18_moduller.tr): takma ad
// önceki aktarma yüzünden yutulmaz.
func TestAliasAfterPlainImportOfTheSameModule(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(dir, rel), trText(filepath.Join(dir, rel), body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("geo.tr", "TR_const PI = 3\nTR_func alan(r: TR_Int) -> TR_Int {\n    TR_return PI * r\n}\n")
	write("main.tr", "TR_import \"geo\"\nTR_import \"geo\" TR_as g\n\nTR_func f() -> TR_Int {\n    TR_return g.alan(1) + g.PI\n}\n")
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Unresolved) != 0 {
		t.Errorf("g must resolve as an alias: %+v", idx.Unresolved)
	}
	fs := facts(idx)
	for _, want := range []string{
		"edge function:main.tr#f calls function:geo.tr#alan EXTRACTED x1",
		"edge function:main.tr#f uses constant:geo.tr#PI EXTRACTED x1",
	} {
		if !hasFact(fs, want) {
			t.Errorf("missing %s", want)
		}
	}
}

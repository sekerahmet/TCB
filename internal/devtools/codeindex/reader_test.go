package codeindex

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

func TestAtFindsTheNameUnderTheCursor(t *testing.T) {
	r := NewReader(buildFixture(t))
	// lib/geo_test.tr:5 "    yaz_alan(k)": yaz_alan 5. sütundan 12. sütuna kadar
	for _, col := range []int{5, 9, 12} {
		o, n := r.At("lib/geo_test.tr", 5, col)
		if o == nil || n == nil || n.ID != "function:lib/geo.tr#yaz_alan" || o.Role != RoleTest {
			t.Errorf("col %d: %+v %+v", col, o, n)
		}
	}
	if o, _ := r.At("lib/geo_test.tr", 5, 13); o != nil && o.Node == "function:lib/geo.tr#yaz_alan" {
		t.Error("the column after the name is outside it")
	}
	if o, _ := r.At("lib/geo_test.tr", 99, 1); o != nil {
		t.Error("a line without references has nothing under the cursor")
	}
	// Bildirimin adı da bir başvurudur (tanım rolüyle): "tanıma git" tanımın kendisinde de çalışır.
	o, n := r.At("lib/geo.tr", 15, 11)
	if o == nil || o.Role != RoleDefinition || n.ID != "function:lib/geo.tr#çevre_hesapla" {
		t.Errorf("definition under cursor: %+v %+v", o, n)
	}
}

func TestWriteRoleOnAliasedVariable(t *testing.T) {
	r := NewReader(buildFixture(t))
	found := false
	for _, o := range r.Occurrences("variable:lib/geo.tr#sayaç") {
		found = found || o.Role == RoleWrite && o.Pos.File == "examples/ana.tr" && o.Pos.Line == 6
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
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.tr", "fonksiyon f() {\n    sonuç bilinmeyen_ad(1)\n}\n")
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Unresolved) != 1 || idx.Unresolved[0].Name != "bilinmeyen_ad" || idx.Unresolved[0].Pos.Line != 2 {
		t.Errorf("unresolved: %+v", idx.Unresolved)
	}
}

// Projede lib/ yoksa `aktar "tr/ad"` gömülü kitaplığa çözülür; modül sanal yoldadır.
func TestEmbeddedLibraryModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tr"), []byte("aktar \"tr/katman\"\n\nGPT(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := fstest.MapFS{"katman.tr": {Data: []byte("yapı GPT {\n    n = 1\n}\n")}}
	idx, err := Build(dir, Options{Library: lib})
	if err != nil {
		t.Fatal(err)
	}
	fs := facts(idx)
	for _, want := range []string{
		"edge module:main.tr imports module:tr:lib/katman.tr EXTRACTED x1",
		"edge module:main.tr calls struct:tr:lib/katman.tr#GPT EXTRACTED x1",
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
	idx, err := Build(root, Options{Dirs: []string{"lib", "examples", "tools"}})
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
		for _, want := range declaredIDs(rel, readFile(string(data))) {
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

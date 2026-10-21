package codeindex

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

// fixtureDir deneme projesidir: modül, test, takma adlı aktarma, kodlu dosya, bozuk dosya.
var fixtureDir = filepath.Join("testdata", "proj")

func buildFixture(t *testing.T) *Index {
	t.Helper()
	idx, err := Build(fixtureDir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

// facts dizini sıralı metin olgularına döker (düğümler, kenarlar, başvurular; konumlarıyla).
func facts(idx *Index) []string {
	var out []string
	for _, n := range idx.Nodes {
		pos := ""
		if n.Pos != nil {
			pos = fmt.Sprintf(" @%d:%d-%d:%d", n.Pos.Line, n.Pos.Col, n.Pos.EndLine, n.Pos.EndCol)
		}
		out = append(out, fmt.Sprintf("node %s %s %q%s", n.ID, n.Kind, n.Qualified, pos))
	}
	for _, e := range idx.Edges {
		out = append(out, fmt.Sprintf("edge %s %s %s %s x%d", e.From, e.Kind, e.To, e.Confidence, e.Count))
	}
	for _, o := range idx.Occurrences {
		out = append(out, fmt.Sprintf("occ %s %s %s @%s in %s", o.Node, o.Role, o.Confidence, o.Pos, o.In))
	}
	sort.Strings(out)
	return out
}

func hasFact(fs []string, want string) bool {
	for _, f := range fs {
		if f == want {
			return true
		}
	}
	return false
}

func TestBuildDeclarations(t *testing.T) {
	idx := buildFixture(t)
	want := map[string]Pos{
		"struct:lib/geo.tr#Kare":            {File: "lib/geo.tr", Line: 5, Col: 1, EndLine: 13},
		"method:lib/geo.tr#Kare.alan":       {File: "lib/geo.tr", Line: 10, Col: 5, EndLine: 12},
		"field:lib/geo.tr#Kare.kenar":       {File: "lib/geo.tr", Line: 6, Col: 5, EndLine: 6},
		"function:lib/geo.tr#çevre_hesapla": {File: "lib/geo.tr", Line: 15, Col: 1, EndLine: 17},
		"constant:lib/geo.tr#PI2":           {File: "lib/geo.tr", Line: 2, Col: 1, EndLine: 2},
		"variable:lib/geo.tr#sayaç":         {File: "lib/geo.tr", Line: 3, Col: 1, EndLine: 3},
		"constant:lib/geo.tr#KATSAYILAR":    {File: "lib/geo.tr", Line: 24, Col: 1, EndLine: 27}, // çok satırlı değer
		"test_block:lib/geo_test.tr#3":      {File: "lib/geo_test.tr", Line: 3, Col: 1, EndLine: 6},
	}
	got := map[string]Node{}
	for _, n := range idx.Nodes {
		got[n.ID] = n
	}
	for id, p := range want {
		n, ok := got[id]
		switch {
		case !ok:
			t.Errorf("missing node %s", id)
		case n.Pos == nil || n.Pos.File != p.File || n.Pos.Line != p.Line || n.Pos.Col != p.Col || p.EndLine != 0 && n.Pos.EndLine != p.EndLine:
			t.Errorf("%s at %+v, want %+v", id, n.Pos, p)
		}
	}
	if m := got["module:lib/geo.tr"]; len(m.Aliases) != 2 || m.Aliases[0] != "geo" || m.Aliases[1] != "tr/geo" {
		t.Errorf("module aliases: %+v", m.Aliases)
	}
	if got["function:examples/coded.tr#kodlu_işlev"].Pos == nil || !got["module:examples/coded.tr"].Coded {
		t.Errorf("the coded file is read in its Turkish display: %+v", got["function:examples/coded.tr#kodlu_işlev"])
	}
	if sig := got["method:lib/geo.tr#Kare.kurucu"].Signature; !strings.HasSuffix(sig, "kurucu(kenar = …)") {
		t.Errorf("method signature: %q", sig)
	}
}

func TestBuildEdgesAndConfidence(t *testing.T) {
	idx := buildFixture(t)
	fs := facts(idx)
	for _, want := range []string{
		"edge function:lib/geo.tr#yaz_alan calls function:lib/geo.tr#çevre_hesapla EXTRACTED x1",
		"edge function:lib/geo.tr#yaz_alan calls method:lib/geo.tr#Kare.alan INFERRED x1", // k.alan(): alıcının tipi bilinmez
		"edge function:lib/geo.tr#yaz_alan uses builtin:G100 EXTRACTED x1",
		"edge module:examples/ana.tr imports module:lib/geo.tr EXTRACTED x1",
		"edge test_block:lib/geo_test.tr#3 calls function:lib/geo.tr#yaz_alan EXTRACTED x1",
		"edge test_block:lib/geo_test.tr#3 tests function:lib/geo.tr#yaz_alan EXTRACTED x1",
		"edge variable:examples/ana.tr#k calls struct:lib/geo.tr#Kare EXTRACTED x1", // g.Kare(2): takma adlı modül
		"edge module:examples/ana.tr uses constant:lib/geo.tr#PI2 EXTRACTED x1",
	} {
		if !hasFact(fs, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, f := range fs {
		if strings.HasPrefix(f, "edge test_block:lib/geo_test.tr#8 uses builtin:G100") {
			t.Errorf("a local variable shadows the built-in: %s", f)
		}
	}
}

func TestSkippedFileKeepsItsModuleNode(t *testing.T) {
	idx := buildFixture(t)
	if len(idx.Skipped) != 1 || idx.Skipped[0].File != "examples/broken.tr" || idx.Skipped[0].Reason == "" {
		t.Fatalf("skipped: %+v", idx.Skipped)
	}
	found := false
	for _, n := range idx.Nodes {
		found = found || n.ID == "module:examples/broken.tr"
	}
	if !found {
		t.Error("a file that does not parse still has its module node")
	}
}

// Dizin yazım dilinden bağımsızdır: aynı proje TR_DIL=tr ve TR_DIL=en ile aynı olguları verir.
func TestIndexIndependentOfActiveLanguage(t *testing.T) {
	tr, en := facts(buildWithLanguage(t, "tr")), facts(buildWithLanguage(t, "en"))
	if len(tr) < 40 {
		t.Fatalf("too few facts to compare: %d", len(tr))
	}
	if strings.Join(tr, "\n") != strings.Join(en, "\n") {
		have := map[string]bool{}
		for _, f := range en {
			have[f] = true
		}
		var missing []string
		for _, f := range tr {
			if !have[f] {
				missing = append(missing, f)
			}
		}
		t.Errorf("TR_DIL=en changes the index (%d facts vs %d); only in tr:\n%s", len(tr), len(en), strings.Join(missing, "\n"))
	}
}

// TestHelperBuildFixture bir test değil, alt süreçtir: çıktı yolu verilirse fikstürün dizinini oraya yazar.
func TestHelperBuildFixture(t *testing.T) {
	out := os.Getenv("CODEINDEX_HELPER_OUT")
	if out == "" {
		t.Skip("alt süreç yardımcısı")
	}
	data, err := EncodeIndex(buildFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// buildWithLanguage fikstürün dizinini TR_DIL=lang ortamında bir alt süreçte kurar.
func buildWithLanguage(t *testing.T, lang string) *Index {
	t.Helper()
	out := filepath.Join(t.TempDir(), "index_"+lang+".json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperBuildFixture$")
	cmd.Env = append(os.Environ(), "TR_DIL="+lang, "CODEINDEX_HELPER_OUT="+out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("TR_DIL=%s: %v\n%s", lang, err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := DecodeIndex(data)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

// Kodlu dosya kodsuzla aynı dizini verir: konumlar Türkçe gösterime göredir.
func TestCodedEqualsPlain(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{"lib/geo.tr", "lib/geo_test.tr"} {
		data, err := os.ReadFile(filepath.Join(fixtureDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		coded, err := source.Encode(string(data))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(coded, source.Header) {
			t.Fatalf("%s did not become coded", rel)
		}
		out := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, []byte(coded), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plain, err := Build(fixtureDir, Options{Dirs: []string{"lib"}})
	if err != nil {
		t.Fatal(err)
	}
	coded, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := strings.Join(facts(plain), "\n"), strings.Join(facts(coded), "\n"); a != b {
		t.Errorf("coded and plain differ:\nplain:\n%s\ncoded:\n%s", a, b)
	}
}

func TestBuildDeterministic(t *testing.T) {
	a, err1 := EncodeIndex(buildFixture(t))
	b, err2 := EncodeIndex(buildFixture(t))
	if err1 != nil || err2 != nil || string(a) != string(b) {
		t.Errorf("two builds of the same project differ (%v %v)", err1, err2)
	}
}

// Sütunlar Türkçe harfli satırlarda da kod noktası sayar ve adın yerini gösterir.
func TestOccurrencePositionsPointAtNames(t *testing.T) {
	idx := buildFixture(t)
	for _, o := range idx.Occurrences {
		if o.Pos.Col == 0 || o.Pos.EndCol == 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(fixtureDir, filepath.FromSlash(o.Pos.File)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(string(data), "//!") {
			continue // kodlu dosya: gösterim metni dosya metni değildir
		}
		line := []rune(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")[o.Pos.Line-1])
		text := string(line[o.Pos.Col-1 : o.Pos.EndCol-1])
		if text == "" || strings.ContainsAny(text, " (){}") {
			t.Errorf("%s: %q is not a name (%s %s)", o.Pos, text, o.Role, o.Node)
		}
	}
}

func TestFindRootAndClearCache(t *testing.T) {
	top := t.TempDir()
	if err := os.MkdirAll(filepath.Join(top, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(top, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(top)
	if got, _ := filepath.EvalSymlinks(FindRoot(deep)); got != want {
		t.Errorf("FindRoot(%s) = %s, want the folder with .git %s", deep, got, want)
	}
	plain := t.TempDir()
	if got, _ := filepath.EvalSymlinks(FindRoot(plain)); got == "" {
		t.Error("without .git the starting folder itself is the root")
	}
	if removed, err := ClearCache(top); err != nil || removed {
		t.Errorf("no cache yet: %v %v", removed, err)
	}
	if err := os.MkdirAll(filepath.Join(top, CacheDir, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if removed, err := ClearCache(top); err != nil || !removed {
		t.Errorf("the cache is removed: %v %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(top, CacheDir)); err == nil {
		t.Error("the cache folder is gone")
	}
}

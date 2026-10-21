package codeindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// hitSummary eşleşmeleri "dosya:satır:sütun yer/biçim" olarak verir.
func hitSummary(hits []TextHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.Pos.String()+" "+h.Where+"/"+h.Match)
	}
	return out
}

// Olası bölümün yanlış pozitifleri: başka, başla gibi sözcükler listelenmez ama sayılır; ekli ve
// bileşik biçimler ayrı etiketlenir; kodda, yorumda ve metinde bulunduğu yer söylenir.
func TestScanTextClassifiesMatches(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"doc.md":     "Dikkat başı ve başları. Başka şey. Başla! kv_baş ve baş_boyutu. Her baş için.\n",
		"ileti.json": "{\"H1\": \"baş sayısı\"}\n",
		"m.tr":       "değişken baş = 1 // baş sayısı\nyazdır(\"baş {baş}\")\nfonksiyon başla() {\n    sonuç 1\n}\n",
	})
	hits, other := ScanText(dir, "baş", "bakış", nil)
	want := []string{
		"doc.md:1:8 doc/suffixed", "doc.md:1:16 doc/suffixed", "doc.md:1:43 doc/compound", "doc.md:1:53 doc/compound", "doc.md:1:69 doc/exact",
		"ileti.json:1:9 data/exact",
		"m.tr:1:10 code/exact", "m.tr:1:21 comment/exact", "m.tr:2:9 string/exact", "m.tr:2:14 string/exact",
	}
	if got := hitSummary(hits); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("hits:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if other != 3 {
		t.Errorf("başka, başla and the function başla are other words, not matches: %d", other)
	}
	for _, h := range hits {
		if h.Match == MatchSuffixed && h.Suggestion != "bakışı" && h.Suggestion != "bakışları" {
			t.Errorf("suggestion keeps the suffix: %+v", h)
		}
		if h.Match == MatchCompound && h.Suggestion != "kv_bakış" && h.Suggestion != "bakış_boyutu" {
			t.Errorf("suggestion replaces the part: %+v", h)
		}
	}
}

func TestScanTextExcludesKnownPositionsAndWarnsAboutVowelHarmony(t *testing.T) {
	dir := writeTree(t, map[string]string{"a.md": "baş ve başı\n"})
	hits, _ := ScanText(dir, "baş", "göz", map[string]bool{"a.md:1:1": true})
	if len(hits) != 1 || hits[0].Word != "başı" || hits[0].Check != "vowel_harmony" || hits[0].Suggestion != "gözı" {
		t.Errorf("an excluded position is skipped; a different vowel class is flagged: %+v", hits)
	}
}

// .git varsa yalnızca izlenen dosyalar taranır (izlenmeyenler kullanıcınındır).
func TestScanTextOnlyScansTrackedFilesInAGitRepository(t *testing.T) {
	dir := writeTree(t, map[string]string{"tracked.md": "baş\n", "private.md": "baş\n"})
	for _, args := range [][]string{{"init", "-q"}, {"add", "tracked.md"}} {
		cmd := execGit(dir, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git is unavailable: %v %s", err, out)
		}
	}
	hits, _ := ScanText(dir, "baş", "x", nil)
	if len(hits) != 1 || hits[0].Pos.File != "tracked.md" {
		t.Errorf("only the tracked file: %+v", hits)
	}
}

func execGit(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	return cmd
}

// Türemiş sıfatlar (çok başlı) ayrı etiketlenir; -lık gibi anlamı değişenler listelenmez. Go
// dosyalarında yorum ve metin sabitleri ayrılır (blok yorum ve ham metin satırlar boyunca sürer).
func TestScanTextDerivedFormsAndGoLexicalState(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"a.md": "çok başlı model, başsız, başlık\n",
		"x.go": "// baş sayısı\nvar s = \"baş\"\n/* baş\n baş */\nvar t = `\nbaş`\nvar baş = 1\n",
	})
	hits, other := ScanText(dir, "baş", "bakış", nil)
	got := strings.Join(hitSummary(hits), "\n")
	want := strings.Join([]string{"a.md:1:5 doc/derived", "a.md:1:18 doc/derived",
		"x.go:1:4 comment/exact", "x.go:2:10 string/exact", "x.go:3:4 comment/exact", "x.go:4:2 comment/exact",
		"x.go:6:1 string/exact"}, "\n")
	if got != want || other != 1 {
		t.Errorf("hits:\n%s\nwant:\n%s\nother=%d", got, want, other)
	}
}

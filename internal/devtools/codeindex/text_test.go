package codeindex

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, trText(path, body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// hitSummary eşleşmeleri "dosya:satır:sütun yer/biçim" olarak verir.
func hitSummary(hits []TextHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, hitLine(h.Pos, h.Where, h.Match))
	}
	return out
}

// hitLine bir eşleşmeyi "konum nerede/eşleşme" dizgesine çevirir (testlerin karşılaştırma biçimi).
func hitLine(pos Pos, where, match string) string { return pos.String() + " " + where + "/" + match }

// Olası bölümün yanlış pozitifleri: başka, başla gibi sözcükler listelenmez ama sayılır; ekli ve
// bileşik biçimler ayrı etiketlenir; kodda, yorumda ve metinde bulunduğu yer söylenir.
func TestScanTextClassifiesMatches(t *testing.T) {
	src := "TR_var baş = 1 # baş sayısı\nTR_print(\"baş \\(baş)\")\nTR_func başla() {\n    TR_return 1\n}\n"
	dir := writeTree(t, map[string]string{
		"doc.md":     "Dikkat başı ve başları. Başka şey. Başla! kv_baş ve baş_boyutu. Her baş için.\n",
		"ileti.json": "{\"H1\": \"baş sayısı\"}\n",
		"m.tr":       src,
	})
	hits, other := ScanText(dir, "baş", "bakış", nil)
	at := func(line string, n int) string { return "m.tr:" + srcAt(t, src, line, "baş", n) }
	want := []string{
		"doc.md:1:8 doc/suffixed", "doc.md:1:16 doc/suffixed", "doc.md:1:43 doc/compound", "doc.md:1:53 doc/compound", "doc.md:1:69 doc/exact",
		"ileti.json:1:9 data/exact",
		at("TR_var baş", 1) + " code/exact", at("TR_var baş", 2) + " comment/exact", // .tr'de yorum # ile başlar (2.2)
		at("TR_print(", 1) + " string/exact", at("TR_print(", 2) + " string/exact",
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

// Kural: GitTracked izlenen dosyaları verir ve yalnızca onları (izlenmeyen dosya yok); .git yoksa ok=false (çağıran klasörü dolaşır).
func TestGitTrackedListsOnlyTrackedFiles(t *testing.T) {
	dir := t.TempDir()
	if _, ok := GitTracked(dir); ok {
		t.Error("a folder without .git has no tracked list")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	for _, args := range [][]string{{"init", "-q"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	for name, text := range map[string]string{"a.tr": "x", "b.tr": "y"} {
		if err := os.WriteFile(filepath.Join(dir, name), trText(filepath.Join(dir, name), text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "add", "a.tr").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	got, ok := GitTracked(dir)
	if !ok || len(got) != 1 || got[0] != "a.tr" {
		t.Errorf("GitTracked = %v, %v; want only a.tr", got, ok)
	}
}

// Kural: araçların tek dosya listesinin (ListFiles) iki kipi vardır. TrackedFiles (depo araçları): git'in izlediği
// dosyalar. AllFiles (tr dizin): kökün altındaki her dosya, .gitignore'a bakılmadan; scratch/ gibi yok sayılan bir
// klasörde yazılan .tr da girer (DENETİM: 0fb3351e'de tr dizin scratch/deneme.tr'yi görmüyordu). İki kipte de atlama
// listesindeki klasörün (testdata) altı girmez, izlense de; dirs ve keep süzer; git yoksa iki kip de gezintidir.
// Dizin kipi Options.Tracked'tan alır: tr dizin yok sayılan klasördeki .tr'yi bulur, codegraph'ın dizini bulmaz.
func TestListFilesKeepsTwoRules(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"lib/a.tr": "TR_func a() {\n    TR_return 1\n}\n", "lib/new.tr": "y",
		"scratch/deneme.tr": "TR_func deneme_fonk() {\n    TR_return 1\n}\n", "lib/testdata/f.tr": "w", "docs/d.md": "m",
		".gitignore": "scratch/\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), trText(filepath.Join(dir, name), text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tr := func(rel string) bool { return strings.HasSuffix(rel, ".tr") }
	for _, set := range []FileSet{TrackedFiles, AllFiles} {
		if got := strings.Join(ListFiles(dir, nil, DefaultSkipDirs, set, tr), " "); got != "lib/a.tr lib/new.tr scratch/deneme.tr" {
			t.Errorf("without git, set %d walks: %q", set, got)
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "lib/a.tr", "docs/d.md", ".gitignore"}, {"add", "-f", "lib/testdata/f.tr"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	for _, c := range []struct {
		set  FileSet
		dirs []string
		want string
	}{
		{TrackedFiles, nil, "lib/a.tr"},                          // depo aracı: yalnızca izlenen
		{AllFiles, nil, "lib/a.tr lib/new.tr scratch/deneme.tr"}, // kullanıcı aracı: yok sayılan klasör dahil
		{AllFiles, []string{"scratch"}, "scratch/deneme.tr"},     // dirs süzer
		{AllFiles, []string{"docs"}, ""},
	} {
		if got := strings.Join(ListFiles(dir, c.dirs, DefaultSkipDirs, c.set, tr), " "); got != c.want {
			t.Errorf("ListFiles(set=%d, dirs=%v) = %q, want %q", c.set, c.dirs, got, c.want)
		}
	}
	for _, c := range []struct {
		tracked bool
		want    int
	}{{false, 1}, {true, 0}} {
		idx, err := Build(dir, Options{Tracked: c.tracked})
		if err != nil {
			t.Fatal(err)
		}
		if got := len(NewReader(idx).Resolve("deneme_fonk", "")); got != c.want {
			t.Errorf("Build(Tracked: %v) finds deneme_fonk %d times, want %d", c.tracked, got, c.want)
		}
	}
}

// Kural (Ahmet, 2026-10-26): tr dizin git'in yok saydığı klasördeki .tr'yi de dizinler (kişinin scratch/ denemeleri), ama
// cevapta o satırlar depodakilerden sonra gelir ve "yok sayılan klasör" diye işaretlenir (H8870); belirsiz adda depodaki
// aday önce gelir. ortak hem lib/'de hem yok sayılan deneme/'de tanımlı (alfabede önce gelir); deneme/'deki dosya lib/'deki işlevi de çağırır.
func TestAnIgnoredFileComesAfterTheRepositoryAndIsMarked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git yok")
	}
	dir := t.TempDir()
	for name, text := range map[string]string{
		".gitignore":    "deneme/\n",
		"lib/m.tr":      "TR_func ortak() {\n    TR_return 1\n}\n\nTR_func kitaplık_işi() {\n    TR_return 2\n}\n",
		"examples/a.tr": "TR_import \"tr/m\"\nTR_print(kitaplık_işi())\n",
		"deneme/d.tr":   "TR_import \"tr/m\"\nTR_func ortak() {\n    TR_return kitaplık_işi()\n}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), trText(filepath.Join(dir, filepath.FromSlash(name)), text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	def := r.Definition("ortak", QueryOptions{})
	if def.Status != StatusAmbiguous || len(def.Candidates) != 2 || def.Candidates[0].Pos.File != "lib/m.tr" ||
		def.Candidates[0].Ignored || !def.Candidates[1].Ignored {
		t.Fatalf("the repository's ortak first, the ignored folder's second and marked: %+v", def.Candidates)
	}
	text := Text(def)
	mark := messages.Text(messages.H8870)
	if lines := strings.Split(strings.TrimSpace(text), "\n"); len(lines) != 3 || strings.Contains(lines[1], mark) || !strings.Contains(lines[2], mark) {
		t.Errorf("only the ignored candidate's line is marked:\n%s", text)
	}
	refs := r.References("kitaplık_işi", QueryOptions{})
	if len(refs.Users) != 2 || refs.Users[0].Pos.File != "examples/a.tr" || refs.Users[1].Pos.File != "deneme/d.tr" || !refs.Users[1].In.Ignored {
		t.Errorf("the ignored folder's use comes after the repository's and is marked: %+v", refs.Users)
	}
	if tracked, _ := Build(dir, Options{Tracked: true}); len(tracked.Ignored) != 0 {
		t.Errorf("the repository tool reads no ignored file: %v", tracked.Ignored)
	}
}

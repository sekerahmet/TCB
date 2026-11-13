package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural: gömülü dil listesi kurulamayınca (bozuk ya da yarım değişmiş tr.json) codegraph düşmez:
// Go sorusu cevaplanır ve cevap .tr tarafının eksik olduğunu söyler. Dil paketleri codegraph'ın
// içine derlendiği için ikili bozuk listeyle derlenir; kökün kendi tr.json'u da bozuktur (ad
// kodlarını codegraph oradan okur).
func TestABrokenLanguageFileLeavesTheGoAnswers(t *testing.T) {
	if testing.Short() {
		t.Skip("codegraph'ı yeniden derler")
	}
	const broken = `{ "TR_kw_if": `
	exe := testkit.BuildWithFile(t, "./tools/codegraph", "internal/lang/codes/tr.json", broken)
	root := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), root)
	testkit.WritePath(t, filepath.Join(root, "internal", "lang", "codes", "tr.json"), broken)

	built := testkit.RunBinary(t, exe, root, "", nil, "build", "--root", root)
	testkit.Wait(t, built, exitOK, []string{"warnings:", trSkipped}, nil)
	answer := testkit.RunBinary(t, exe, root, "", nil, "tests-for", "--root", root, "shape/shape.go")
	testkit.Wait(t, answer, exitOK, []string{"shape.TestDescribe"}, []string{"answers about .tr code are missing", "language list tr"})
}

// Kural: dil dosyasında (internal/lang/codes/tr.json) definitions.json'da olmayan geçici bir kod varken
// trSkipped uyarısı iç hata başlığında kesilmez; eksik kodun adını, dosyasını ve çaresini söyler.
func TestMissingCodeDefinitionInLanguageFileNamesCodeAndRemedy(t *testing.T) {
	repoRoot, err := resolveRoot("")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "internal", "lang", "codes", "tr.json"))
	if err != nil {
		t.Fatal(err)
	}
	tempCode := "TR_fn_" + "temp_missing_def"
	withTemp := strings.Replace(string(raw), `"TR_kw_if": {`, `"`+tempCode+`": {"name": "gecici_deneme"},`+"\n  "+`"TR_kw_if": {`, 1)
	if withTemp == string(raw) {
		t.Fatal("TR_kw_if not found in internal/lang/codes/tr.json")
	}
	exe := testkit.BuildWithFile(t, "./tools/codegraph", "internal/lang/codes/tr.json", withTemp)
	root := t.TempDir()
	testkit.CopyDir(t, filepath.Join("testdata", "mod"), root)
	testkit.WritePath(t, filepath.Join(root, "internal", "lang", "codes", "tr.json"), withTemp)

	answer := testkit.RunBinary(t, exe, root, "", nil, "tests-for", "--root", root, "shape/shape.go")
	testkit.Wait(t, answer, exitOK, []string{"shape.TestDescribe"}, []string{
		"answers about .tr code are missing",
		tempCode,
		"internal/lang/codes/tr.json but not in definitions.json",
		"add it to definitions.json and run go generate ./internal/lang/codes",
	})
	// Eksik olan geliştiricinin tanımıdır: iç hata başlığı ("TR'nin hatası, bildirin") yoktur.
	for _, bad := range []string{"lütfen bildirin", "TR'nin hatası"} {
		if strings.Contains(answer.Stderr, bad) {
			t.Errorf("warning still carries the internal error header %q:\n%s", bad, answer.Stderr)
		}
	}
}

// Kural: gömülü ileti dosyasında tanımsız yuva varken (messages.load sync.Once içinde panikler) var olan
// grafikle sorulan tests-for tek satır uyarı yazar, eski grafikle cevabı verir ve çıkış 0 döner; build ya da
// grafik yokken tek satır hata yazar ve sıfır dışı çıkar.
func TestUndefinedMessageSlotKeepsStaleGraphAndExitsOnBuild(t *testing.T) {
	repoRoot, err := resolveRoot("")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, "internal", "lang", "messages", "tr.json"))
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(raw), `"H7012": "`, `"H7012": "{#undefined_slot} `, 1)
	if broken == string(raw) {
		t.Fatal("H7012 not found in internal/lang/messages/tr.json")
	}
	exe := testkit.BuildWithFile(t, "./tools/codegraph", "internal/lang/messages/tr.json", broken)
	root := gitFixture(t)
	seeded := *fixture(t)
	seeded.Stamp = "content:stale"
	graph := filepath.Join(root, ".codegraph", "graph.json")
	if err := writeGraph(graph, &seeded); err != nil {
		t.Fatal(err)
	}
	// libraryImports -> codeindex.Snippet -> PosError.Error -> messages.load zincirini de tetikle.
	appendTo(t, filepath.Join(root, "shape", "shape_test.go"), "\nconst _ = `\"tr/missing\" @`\n")

	stale := testkit.RunBinary(t, exe, root, "", nil, "tests-for", "--root", root, "shape/shape.go")
	testkit.Wait(t, stale, exitOK, []string{"shape.TestDescribe"}, []string{"warning: rebuild failed", "H7012", "no such name slot", "using the existing, stale graph"})
	if strings.Count(stale.Stderr, "\n") != 1 || strings.Contains(stale.Stderr, "panic:") {
		t.Errorf("want single-line warning without panic traceback, got:\n%s", stale.Stderr)
	}

	built := testkit.RunBinary(t, exe, root, "", nil, "build", "--root", root)
	if built.Code == exitOK {
		t.Errorf("build with broken messages must fail, got exit 0")
	}
	if !strings.Contains(built.Stderr, "no such name slot") || strings.Count(built.Stderr, "\n") != 1 || strings.Contains(built.Stderr, "panic:") {
		t.Errorf("build stderr want single line without panic, got:\n%s", built.Stderr)
	}

	missing := testkit.RunBinary(t, exe, root, "", nil, "tests-for", "--root", root, "--graph", filepath.Join(root, "missing.json"), "shape/shape.go")
	if missing.Code == exitOK {
		t.Errorf("tests-for without an existing graph must fail, got exit 0")
	}
	if !strings.Contains(missing.Stderr, "no such name slot") || strings.Count(missing.Stderr, "\n") != 1 || strings.Contains(missing.Stderr, "panic:") {
		t.Errorf("missing-graph stderr want single line without panic, got:\n%s", missing.Stderr)
	}
}

// Kural (K7): `who-uses <ad> --mode read|write` bir alanın erişimlerini sorar; ad yalnız alana çözülür. Fikstürde
// Square'in alanı Side'dır, `Square.area` yalnız harf katlamasıyla Area metoduna uyar: --mode ile cevap "not found"dur
// (çıkış 4), metodun kullanımları verilmez. Gerçek alan --mode ile cevaplanır.
func TestWhoUsesModeResolvesOnlyFields(t *testing.T) {
	code, out, errOut := cli(t, "who-uses", "Square.area", "--mode", "write")
	if code != exitNotFound {
		t.Fatalf("exit %d, want %d (no field matches)\n%s%s", code, exitNotFound, out, errOut)
	}
	testkit.MustContain(t, errOut, `"Square.area" not found`)
	if strings.Contains(out, "shape.Square.Area") {
		t.Errorf("the method is not the answer:\n%s", out)
	}
	code, out, _ = cli(t, "who-uses", "Counter.n", "--mode", "write")
	if code != exitOK {
		t.Fatalf("a real field: exit %d\n%s", code, out)
	}
	testkit.MustContain(t, out, "shape.Counter.n [field]", "write accesses (", "shape/shape.go:84  write")
}

// Kural (K7): `fields` bir tipin alan tablosudur; bir alanı (`Tip.alan` ya da alana çözülen çıplak ad) soran `fields`
// çıkış 2 verir ve aynı soruyu soran `who-uses` satırını yazar (--mode verildiyse onunla). İletideki komut koşulunca
// alanın erişimleri gelir: eskiden `fields <alan>`'ın verdiği çıktı (`who-uses`'la aynı yazıcı).
func TestFieldsOfAFieldNamesTheWhoUsesLine(t *testing.T) {
	const prefix = "go run ./tools/codegraph "
	for _, tc := range []struct {
		args []string
		line string
	}{
		{[]string{"Counter.n"}, "who-uses " + counterN},
		{[]string{"Counter.n", "--mode", "write"}, "who-uses " + counterN + " --mode write"},
		{[]string{"list", "--mode", "read"}, "who-uses " + counterList + " --mode read"},
	} {
		code, out, errOut := cli(t, append([]string{"fields"}, tc.args...)...)
		if code != exitUsage || out != "" {
			t.Errorf("fields %v: exit %d, want %d, stdout %q", tc.args, code, exitUsage, out)
			continue
		}
		testkit.MustContain(t, errOut, "codegraph: fields takes a type; the reads and writes of the field ", prefix+tc.line+"\n")
		args := strings.Fields(strings.TrimSuffix(errOut[strings.Index(errOut, prefix)+len(prefix):], "\n"))
		code, got, _ := cli(t, args...)
		_, old, _ := cli(t, append([]string{"who-uses"}, tc.args...)...)
		if code != exitOK || got != old || !strings.Contains(got, "[field]") {
			t.Errorf("%s: exit %d; its answer is not the field's accesses:\n%s\nwant:\n%s", strings.Join(args, " "), code, got, old)
		}
	}
	if code, out, _ := cli(t, "fields", "shape.Counter"); code != exitOK || !strings.Contains(out, "list [[]int]: 0 reads, 1 writes") {
		t.Errorf("fields of a type is the table: exit %d\n%s", code, out)
	}
}

// Kural (K7): `--mode`'suz `who-uses Tip.ad` adı yalnız harf katlamasıyla alan olmayan bir düğüme çözerse cevabı verir
// (çıkış 0) ve bunu stderr'e yazar; alan sorusu sanılan cevap bir metodun çağıranlarıydı. Tam harfle uyan ad (alan ya
// da metot) not almaz.
func TestWhoUsesSaysAFoldedMatchIsNotAField(t *testing.T) {
	code, out, errOut := cli(t, "who-uses", "Square.area")
	if code != exitOK || !strings.HasPrefix(out, "shape.Square.Area [method]") {
		t.Fatalf("exit %d: the answer is still given\n%s", code, out)
	}
	const note = "codegraph: Square.area matched shape.Square.Area only by letter case, and it is a method, not a field; a field's reads and writes: go run ./tools/codegraph who-uses <Type.field> --mode write (or read)\n"
	if !strings.Contains(errOut, note) {
		t.Errorf("stderr lacks the note:\n%s", errOut)
	}
	for _, q := range []string{"Counter.n", "Square.Area"} {
		if _, _, errOut := cli(t, "who-uses", q); strings.Contains(errOut, "only by letter case") {
			t.Errorf("%s matches by its own spelling, no note:\n%s", q, errOut)
		}
	}
}

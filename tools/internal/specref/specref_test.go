package specref

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// specFile test için bir dil tanımı dosyası kurar: her madde `**N.M** Başlık: gövde` satırıdır.
func specFile(chapter int, ids ...int) File {
	var b strings.Builder
	for _, n := range ids {
		fmt.Fprintf(&b, "**%d.%d** Madde %d: gövde.\n", chapter, n, n)
	}
	return File{Path: fmt.Sprintf("docs/spec/%d_x.md", chapter), Text: b.String()}
}

func build(files ...File) (*Index, []Mention) {
	ix := BuildIndex(files)
	return ix, Scan(files, ix)
}

// Kural: ek, numaranın okunuşuna uyar; ünlü uyumu, d/t benzeşmesi ve kaynaştırma ünsüzü denetlenir.
func TestCheckSuffixFollowsTheReadingOfTheNumber(t *testing.T) {
	good := [][2]string{
		{"15.9", "daki"}, {"15.14", "teki"}, {"13.7", "dedir"}, {"15.3", "ün"}, {"15.24", "teki"},
		{"15.6", "nın"}, {"15.2", "ye"}, {"15.1", "in"}, {"15.4", "te"}, {"15.30", "da"}, {"15.8", "de"},
		{"15.40", "ta"}, {"15.50", "de"}, {"15.100", "de"}, {"15.0", "dır"}, {"15.3b", "xyz"},
	}
	for _, c := range good {
		if want, ok := CheckSuffix(c[0], c[1]); !ok {
			t.Errorf("%s'%s should pass, wants %q", c[0], c[1], want)
		}
	}
	bad := [][3]string{
		{"15.9", "taki", "daki"}, {"15.24", "deki", "teki"}, {"15.3", "in", "ün"}, {"15.4", "de", "te"},
		{"15.5", "dır", "tir"}, {"15.6", "nin", "nın"},
	}
	for _, c := range bad {
		if want, ok := CheckSuffix(c[0], c[1]); ok || want != c[2] {
			t.Errorf("%s'%s: got (%q,%v), want (%q,false)", c[0], c[1], want, ok, c[2])
		}
	}
	for _, c := range [][2]string{{"15.2", "e"}, {"15.3", "ye"}, {"15.3", "nın"}} {
		if want, ok := CheckSuffix(c[0], c[1]); ok || want != "" {
			t.Errorf("%s'%s should be an unfixable buffer error, got (%q,%v)", c[0], c[1], want, ok)
		}
	}
}

// Kural: açık atıf biçimleri taranır; olmayan maddeye atıf hata, spec içi zayıf anma not, sürüm gibi 15.0 yazıları
// da yalnızca açık biçimdeyse (bulgu çıkar ama renumber'a dokunmaz).
func TestCheckFindsMissingItemsAndBadSuffixes(t *testing.T) {
	spec := specFile(15, 1, 2, 3)
	other := File{Path: "docs/x.md", Text: "dil tanımı 15.2'de, **15.9**'daki, dil tanımı 15.3'in ve [15.1](spec/15_x.md) ile dil tanımı 15.77\n"}
	ix, ms := build(spec, other)
	var got []string
	for _, f := range Check(ms, ix) {
		got = append(got, fmt.Sprintf("%s:%d %s %s %s", f.File, f.Line, f.Check, f.Severity, f.Ref))
	}
	has := func(s string) bool {
		for _, g := range got {
			if g == s {
				return true
			}
		}
		return false
	}
	for _, s := range []string{
		"docs/x.md:1 missing error 15.9",
		"docs/x.md:1 missing error 15.77",
		"docs/x.md:1 suffix error 15.3",
	} {
		if !has(s) {
			t.Errorf("missing finding %q in %v", s, got)
		}
	}
	if has("docs/x.md:1 suffix error 15.2") || has("docs/x.md:1 missing error 15.1") {
		t.Errorf("unexpected finding in %v", got)
	}
}

// Kural: spec dosyalarındaki parantez içi anma zayıf anmadır; yok olan madde "note" olur, 1. ve 14. bölüm gibi
// numarasız bölümler sayılmaz.
func TestWeakMentionsAreNotesAndOnlyInSpecFiles(t *testing.T) {
	spec := specFile(15, 1, 2, 3)
	spec.Text += "bkz. (karışık hassasiyet, 15.3) ve (15.99) ve 14.2 ve 15.3.1\n"
	ix, ms := build(spec, File{Path: "docs/y.md", Text: "(bkz 15.99)\n"})
	var weak []string
	for _, m := range ms {
		if m.Kind == "weak" {
			weak = append(weak, m.Tokens[0].ID)
		}
	}
	if strings.Join(weak, ",") != "15.3,15.99" {
		t.Errorf("weak mentions = %v", weak)
	}
	for _, f := range Check(ms, ix) {
		if f.Severity != "note" {
			t.Errorf("weak finding must be a note: %+v", f)
		}
	}
}

// Kural: eşleme zincirleme uygulanmaz (15.3→15.4 ve 15.4→15.5 birlikte); tanımlar, kalın atıflar, "dil tanımı"
// atıfları, spec içi zayıf anmalar ve ekler özgün konumdan bir kez değişir; 15.0 gibi madde olmayan yazıya dokunulmaz.
func TestPlanRenumbersWithoutChainingAndFixesSuffixes(t *testing.T) {
	spec := specFile(15, 1, 2, 3, 4, 5)
	spec.Text += "karışık hassasiyet (… 15.3) ve **15.9**'daki\n"
	spec.Text = strings.Replace(spec.Text, "**15.5** Madde 5", "**15.5** Madde 5 (bkz. **15.0**)", 1)
	doc := File{Path: "docs/a.md", Text: "dil tanımı 15.3'ün ve **15.4**'te, dil tanımı 15.2'de, sürüm 15.0\n"}
	files := []File{spec, doc}
	ix, ms := build(files...)
	m := map[string]string{"15.3": "15.4", "15.4": "15.5", "15.5": "15.3"}
	if err := Validate(m, ix); err != nil {
		t.Fatal(err)
	}
	edits, manual := Plan(ms, files, ix, m, false)
	if len(manual) != 0 {
		t.Fatalf("manual = %v", manual)
	}
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.Path))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(f.Text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Apply(root, edits); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string { b, _ := os.ReadFile(filepath.Join(root, p)); return string(b) }
	if got := read("docs/a.md"); got != "dil tanımı 15.4'ün ve **15.5**'te, dil tanımı 15.2'de, sürüm 15.0\n" {
		t.Errorf("a.md = %q", got)
	}
	s := read("docs/spec/15_x.md")
	for _, w := range []string{"**15.4** Madde 3", "**15.5** Madde 4", "**15.3** Madde 5 (bkz. **15.0**)", "(… 15.4)"} {
		if !strings.Contains(s, w) {
			t.Errorf("spec lacks %q:\n%s", w, s)
		}
	}
}

// Kural: numara değişince ek yeni okunuşa uydurulur (15.9'daki → 15.24 olunca 'teki).
func TestPlanFixesTheSuffixOfARenumberedReference(t *testing.T) {
	spec := specFile(15, 9, 24)
	doc := File{Path: "docs/a.md", Text: "**15.9**'daki ve [15.24](spec/15_x.md)'teki\n"}
	files := []File{spec, doc}
	ix, ms := build(files...)
	edits, _ := Plan(ms, files, ix, map[string]string{"15.9": "15.30", "15.24": "15.9"}, false)
	var got []string
	for _, e := range edits {
		if e.File == "docs/a.md" {
			got = append(got, e.Old+">"+e.New)
		}
	}
	if strings.Join(got, " ") != "15.9>15.30 15.24>15.9 teki>daki" {
		t.Errorf("edits = %v", got)
	}
}

// Kural: ardışık olmayan yeni numaralara giden aralık parçalara bölünür ve ek yeni son numaraya uyar
// ("15.31–15.38" → "15.11–15.15, 15.20–15.22").
func TestPlanSplitsARangeOverNonConsecutiveTargets(t *testing.T) {
	spec := specFile(15, 11, 12, 13, 14, 15, 20, 21, 22, 31, 32, 33, 34, 35, 36, 37, 38)
	doc := File{Path: "docs/a.md", Text: "Bkz. dil tanımı 15.31–15.38'deki ve **15.32–15.34**'te ve **15.31**–**15.32**'de\n"}
	files := []File{spec, doc}
	ix, ms := build(files...)
	m := map[string]string{"15.31": "15.11", "15.32": "15.12", "15.33": "15.13", "15.34": "15.14", "15.35": "15.15",
		"15.36": "15.20", "15.37": "15.21", "15.38": "15.22"}
	// Hedef numaralar çakışmasın: eski 15.11–15.22 maddeleri de taşınır.
	m["15.11"], m["15.12"], m["15.13"], m["15.14"], m["15.15"] = "15.51", "15.52", "15.53", "15.54", "15.55"
	m["15.20"], m["15.21"], m["15.22"] = "15.60", "15.61", "15.62"
	if err := Validate(m, ix); err != nil {
		t.Fatal(err)
	}
	edits, manual := Plan(ms, files, ix, m, false)
	if len(manual) != 0 {
		t.Fatalf("manual = %v", manual)
	}
	root := t.TempDir()
	p := filepath.Join(root, "docs", "a.md")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(doc.Text), 0o644)
	var docEdits []Edit
	for _, e := range edits {
		if e.File == doc.Path {
			docEdits = append(docEdits, e)
		}
	}
	if err := Apply(root, docEdits); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	want := "Bkz. dil tanımı 15.11–15.15, 15.20–15.22'deki ve **15.12–15.14**'te ve **15.11**–**15.12**'de\n"
	if string(b) != want {
		t.Errorf("got  %q\nwant %q", b, want)
	}
}

// Kural: eşleme çakışması ve olmayan eski numara reddedilir.
func TestValidateRejectsCollisionsAndUnknownItems(t *testing.T) {
	ix, _ := build(specFile(15, 1, 2, 3))
	if err := Validate(map[string]string{"15.1": "15.2"}, ix); err == nil {
		t.Error("15.1→15.2 collides with the unmoved 15.2")
	}
	if err := Validate(map[string]string{"15.9": "15.1"}, ix); err == nil {
		t.Error("15.9 is not an item")
	}
	if err := Validate(map[string]string{"15.1": "15.2", "15.2": "15.1"}, ix); err != nil {
		t.Errorf("a swap is fine: %v", err)
	}
}

// Kural: Apply CRLF'yi korur ve dosya plandan sonra değiştiyse hiçbir şey yazmaz.
func TestApplyKeepsCRLFAndRefusesStalePlans(t *testing.T) {
	root := t.TempDir()
	spec := specFile(15, 1, 2)
	spec.Text = strings.ReplaceAll(spec.Text, "\n", "\r\n")
	p := filepath.Join(root, "docs", "spec", "15_x.md")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(spec.Text), 0o644)
	ix, ms := build(spec)
	edits, _ := Plan(ms, []File{spec}, ix, map[string]string{"15.1": "15.7"}, false)
	_ = os.WriteFile(p, []byte("x"+spec.Text), 0o644)
	if err := Apply(root, edits); err == nil {
		t.Fatal("stale plan must be refused")
	}
	_ = os.WriteFile(p, []byte(spec.Text), 0o644)
	if err := Apply(root, edits); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(b), "**15.7** Madde 1: gövde.\r\n") {
		t.Errorf("got %q", b)
	}
}

// Kural: Collect git'siz klasörde .md/.go/.tr dosyalarını okur, testdata ve .git'i atlar.
func TestCollectReadsDocsAndSkipsTestdata(t *testing.T) {
	root := t.TempDir()
	for name, text := range map[string]string{"a.md": "x", "b/c.go": "y", "d/e.tr": "z", "t/testdata/f.md": "no", "q.txt": "no"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(text), 0o644)
	}
	files, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Path)
	}
	if strings.Join(names, ",") != "a.md,b/c.go,d/e.tr" {
		t.Errorf("collected %v", names)
	}
}

// Kural: kod içindeki numaralar atıf değildir: Markdown'da `...` aralıkları ve çit blokları, Go'da // öncesi
// (dize sabitleri); Go yorumları ve düz metin atıftır.
func TestCodeIsNotScannedForReferences(t *testing.T) {
	spec := specFile(15, 1)
	doc := File{Path: "docs/a.md", Text: "örnek `**15.77**` ve dil tanımı 15.88\n```\n**15.99**\n```\n"}
	code := File{Path: "x/a.go", Text: "package x\n\nvar s = \"**15.55**\" /" + "/ dil tanımı 15.66\n/" + "/ dil tanımı 15.1\n"}
	ix, ms := build(spec, doc, code)
	var got []string
	for _, f := range Check(ms, ix) {
		got = append(got, f.Ref)
	}
	if strings.Join(got, ",") != "15.88,15.66" {
		t.Errorf("findings = %v", got)
	}
}

// Kural: spec dışında çıplak bir sayı yalnızca var olan bir maddenin numarasıysa anmadır ("(17.1, 15.5 ms)"deki 15.5 ms olmayan maddeyi
// anmaz); böyle bir anma yeniden numaralamada otomatik değişmez, elle bakılacaklara gider (ondalık sayı da olabilir).
func TestBareNumbersOutsideTheSpecAreMentionsOnlyOfExistingItems(t *testing.T) {
	spec := specFile(15, 1, 2, 3)
	doc := File{Path: "docs/a.md", Text: "Süre 15.9 ms, bkz. 15.3 ve sürüm 1.20.\n"}
	ix, ms := build(spec, doc)
	var got []string
	for _, m := range ms {
		if m.File == "docs/a.md" {
			got = append(got, m.Tokens[0].ID)
		}
	}
	if strings.Join(got, ",") != "15.3" {
		t.Errorf("mentions = %v", got)
	}
	edits, manual := Plan(ms, []File{spec, doc}, ix, map[string]string{"15.3": "15.7"}, false)
	for _, e := range edits {
		if e.File == "docs/a.md" {
			t.Errorf("a bare number outside the spec must not be edited automatically: %+v", e)
		}
	}
	if len(manual) != 1 || manual[0].File != "docs/a.md" {
		t.Errorf("manual = %v", manual)
	}
}

// Kural: Markdown kod çiti CommonMark kuralıyla okunur (Fence): ``` ve ~~~ açar, aynı karakterden en az o kadar uzun
// çit kapatır; açık ``` içindeki ~~~ içeriktir; dört boşluk girintili ve satır içi kod çit değildir. Atıf taraması ve
// codegraph'ın başlık ayrıştırıcısı bu tek kuralı kullanır (bağımsız denetim T6).
func TestFenceFollowsCommonMark(t *testing.T) {
	lines := []string{"a", "~~~", "**15.3**", "```", "~~~~", "b", "```go", "x", "````", "    ```", "```", "c", "```", "`` ``` ``", "d"}
	want := []bool{false, true, true, true, false, false, true, true, false, false, true, true, false, false, false} // satırdan sonra çitin içinde mi
	var f Fence
	for i, l := range lines {
		f.Line(l)
		if f.In() != want[i] {
			t.Errorf("after line %d %q: in=%v, want %v", i+1, l, f.In(), want[i])
		}
	}
}

// Kural: dil tanımında ürün adını (PyTorch, NumPy, Go ...) ya da "sürüm"ü aynı cümlede izleyen, var olmayan madde
// numarası bir ürün sürümüdür, atıf değildir; üç parçalı numara (2.14.0) hiç sayılmaz; var olan madde aynı bağlamda da
// anmadır (TCB sağlık taraması P2: "PyTorch 2.14" beş "no item 2.14" notu veriyordu).
func TestProductVersionsAreNotItemReferences(t *testing.T) {
	spec := specFile(15, 1, 2, 3)
	spec.Text += "PyTorch 15.14 ve NumPy 15.1.1 ile; PyTorch (`CUDA_LAUNCH_BLOCKING`; 15.14 belgesi) ve PyTorch 15.3 gibi; sürüm 15.19\n" +
		"ayrıca (15.9) bkz.\n"
	_, ms := build(spec)
	var weak []string
	for _, m := range ms {
		if m.Kind == "weak" {
			weak = append(weak, m.Tokens[0].ID)
		}
	}
	if strings.Join(weak, ",") != "15.3,15.9" {
		t.Errorf("weak mentions = %v, want the existing 15.3 and the plain missing 15.9", weak)
	}
}

// Kural: dış adrese (http, https) giden bağlantının yazısındaki numara, ürün adı versionWords'te olsun olmasın, madde
// atfı değildir; madde var olsa da (Apply onu yeniden numaralamaz). Aynı satırdaki bağlantı dışı anmalar ve iç
// bağlantılar sayılmaya devam eder (gaps.md: "[Kubernetes 1.37]", "[CPython 3.15 lexer]" not veriyordu).
func TestExternalLinkTextIsNotAnItemReference(t *testing.T) {
	spec := specFile(15, 1, 2, 3)
	spec.Text += "[CPython 15.14 lexer](https://github.com/python/cpython/blob/15.14/x.h) ve [Mojo 15.2](http://mojo.example) ile, (15.9) ve (15.3) bkz.\n"
	_, ms := build(spec)
	var weak []string
	for _, m := range ms {
		if m.Kind == "weak" {
			weak = append(weak, m.Tokens[0].ID)
		}
	}
	if strings.Join(weak, ",") != "15.9,15.3" {
		t.Errorf("weak mentions = %v, want only the plain 15.9 and 15.3 outside the external links", weak)
	}
}

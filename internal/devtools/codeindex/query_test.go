package codeindex

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func fixtureReader(t *testing.T) *Reader {
	t.Helper()
	return NewReader(buildFixture(t))
}

func TestDefinition(t *testing.T) {
	res := fixtureReader(t).Definition("Kare", QueryOptions{})
	if res.Status != StatusOK || res.Target == nil || res.Target.ID != "struct:lib/geo.tr#Kare" {
		t.Fatalf("%+v", res)
	}
	if len(res.Members) != 3 {
		t.Errorf("members: %+v", res.Members)
	}
}

func TestResolveTiers(t *testing.T) {
	r := fixtureReader(t)
	cases := map[string]string{
		"struct:lib/geo.tr#Kare": "struct:lib/geo.tr#Kare",
		"Kare":                   "struct:lib/geo.tr#Kare",
		"geo.Kare":               "struct:lib/geo.tr#Kare",
		"geo.Kare.alan":          "method:lib/geo.tr#Kare.alan",
		"Kare.alan":              "method:lib/geo.tr#Kare.alan",
		"kare.ALAN":              "method:lib/geo.tr#Kare.alan", // büyük harf var, tam eşleşme yok: harf yok sayılır
		"kare.alan":              "method:lib/geo.tr#Kare.alan",
		"lib/geo.tr":             "module:lib/geo.tr",
		"geo":                    "module:lib/geo.tr",
		"tr/geo":                 "module:lib/geo.tr",
		"lib/geo_test.tr:5:6":    "function:lib/geo.tr#yaz_alan", // imlecin altındaki ad
		"lib/geo.tr:15:12":       "function:lib/geo.tr#çevre_hesapla",
		"çevre_hesapla":          "function:lib/geo.tr#çevre_hesapla",
		"ÇEVRE_HESAPLA":          "function:lib/geo.tr#çevre_hesapla",
		"ÇEVRE_HESAPLAMA":        "",
		"geo.çevre_hesapla":      "function:lib/geo.tr#çevre_hesapla",
		"yazdır":                 "builtin:TR_fn_print", // TR adı
		"print":                  "builtin:TR_fn_print", // İngilizce adı
		"TR_fn_print":            "builtin:TR_fn_print",
	}
	for q, want := range cases {
		got := r.Resolve(q, "")
		switch {
		case want == "" && len(got) != 0:
			t.Errorf("%q should not resolve, got %v", q, got[0].ID)
		case want != "" && (len(got) != 1 || got[0].ID != want):
			ids := []string{}
			for _, n := range got {
				ids = append(ids, n.ID)
			}
			t.Errorf("%q -> %v, want %s", q, ids, want)
		}
	}
}

func buildSource(t *testing.T, src string) *Reader {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.tr"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return NewReader(idx)
}

func TestTurkishCaseFolding(t *testing.T) {
	r := buildSource(t, "fonksiyon Işık() {\n    sonuç 1\n}\n")
	if got := r.Resolve("ışık", ""); len(got) != 1 {
		t.Errorf("ışık (dotless ı) must match Işık: %v", got)
	}
	if got := r.Resolve("isik", ""); len(got) != 0 {
		t.Errorf("isik is another word: %v", got)
	}
}

// Kural: harf kuralı NameKeys'tir (iki aracın tek kuralı): küçük harfli sorgu harfe bakmaz, büyük harfli sorgu önce tam
// eşleşir, tam eşleşen yoksa harfe bakılmaz; o geçişte ASCII küçültmesi de denenir; dilin bir sözcüğüyle birebir aynı
// sorguda geçiş yoktur (bağımsız denetim grup 6 ve 7, R1).
func TestCaseRuleExactFirstThenFolded(t *testing.T) {
	r := buildSource(t, `yapı Kare {
    a
}

fonksiyon kare() {
    sonuç 1
}

fonksiyon gizli_işlev() {
    sonuç metin(1)
}
`)
	for q, want := range map[string]string{
		"Kare":        "struct:m.tr#Kare",
		"kare":        "function:m.tr#kare struct:m.tr#Kare",
		"KARE":        "function:m.tr#kare struct:m.tr#Kare",
		"GİZLİ_İŞLEV": "function:m.tr#gizli_işlev", // Türkçe büyük harf
		"GIZLI_IŞLEV": "function:m.tr#gizli_işlev", // İngilizce klavye: I, Türkçede ı olurdu
		"Metin":       "",                          // dilin tip adı: metin() yerleşiğine sessizce düşülmez
		"metin":       "builtin:TR_fn_str",
	} {
		var ids []string
		for _, n := range r.Resolve(q, "") {
			ids = append(ids, n.ID)
		}
		sort.Strings(ids)
		if strings.Join(ids, " ") != want {
			t.Errorf("%q -> %v, want %s", q, ids, want)
		}
	}
}

// Kural: çıplak adın sınıf sırası PickByName'dir: modül adı bildirimlerden önce, yerleşik ad uyan ilk sınıfa katılır
// (`metin` hem modül hem yerleşik), test açıklaması başka aday yoksa (`ortalama` yerleşiktir, aynı açıklamalı test
// değil) (bağımsız denetim grup 6, R1).
func TestBareNameClasses(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"metin.tr": "değişken metin_boyu = uzunluk(metin(12))\n",
		"m_test.tr": `test "ortalama" {
    doğrula(ortalama([1, 3]) == 2)
}

test "yalnız bir test" {
    doğrula(doğru)
}
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	for q, want := range map[string]string{
		"metin":           "builtin:TR_fn_str module:metin.tr",
		"ortalama":        "builtin:TR_fn_mean",
		"yalnız bir test": "test_block:m_test.tr#5",
	} {
		var ids []string
		for _, n := range r.Resolve(q, "") {
			ids = append(ids, n.ID)
		}
		sort.Strings(ids)
		if strings.Join(ids, " ") != want {
			t.Errorf("%q -> %v, want %s", q, ids, want)
		}
	}
}

func TestAmbiguousAndNotFound(t *testing.T) {
	r := buildSource(t, "yapı A {\n    fonksiyon ileri() {\n        sonuç 1\n    }\n}\nyapı B {\n    fonksiyon ileri() {\n        sonuç 2\n    }\n}\n")
	res := r.Definition("ileri", QueryOptions{})
	if res.Status != StatusAmbiguous || len(res.Candidates) != 2 || ExitCode(res.Status) != ExitAmbiguous {
		t.Fatalf("%+v", res)
	}
	miss := r.Definition("ilerı", QueryOptions{})
	if miss.Status != StatusNotFound || len(miss.DidYouMean) == 0 {
		t.Fatalf("a near miss gets suggestions: %+v", miss)
	}
	if !strings.Contains(Text(miss), messages.Text(messages.H8783)) || !strings.Contains(Text(res), messages.Text(messages.H8780, "ileri", 2)) {
		t.Error("text output for the two non-ok statuses")
	}
}

func TestReferences(t *testing.T) {
	r := fixtureReader(t)
	res := r.References("Kare", QueryOptions{})
	if res.Status != StatusOK || res.Counts.Total < 3 {
		t.Fatalf("%+v", res)
	}
	roles := map[Role]int{}
	for _, u := range res.Users {
		roles[u.Role]++
		if u.Pos.Line == 0 || u.Pos.Col == 0 || u.In == nil {
			t.Errorf("a use carries its position and its enclosing declaration: %+v", u)
		}
	}
	if roles[RoleCall] == 0 || roles[RoleTest] == 0 || roles[RoleDefinition] != 0 {
		t.Errorf("roles: %v (calls and tests, no definition unless asked)", roles)
	}
	with := r.References("Kare", QueryOptions{IncludeDefinition: true})
	if with.Counts.ByRole[RoleDefinition] != 1 {
		t.Errorf("includeDefinition: %v", with.Counts.ByRole)
	}
	if limited := r.References("Kare", QueryOptions{Limit: 1}); len(limited.Users) != 1 || !limited.Truncated || limited.Counts.Total != res.Counts.Total {
		t.Errorf("limit keeps the total and marks truncation: %+v", limited)
	}
}

func TestBuiltinReferencesCarryDisplayNames(t *testing.T) {
	res := fixtureReader(t).References("print", QueryOptions{})
	if res.Status != StatusOK || res.Target.ID != "builtin:TR_fn_print" || res.Target.Display != "yazdır" {
		t.Fatalf("a built-in is found by its English name and shown in the reader's language: %+v", res.Target)
	}
	if res.Counts.Total < 3 {
		t.Errorf("uses of yazdır: %d", res.Counts.Total)
	}
}

func TestTestedByDirectBeforeIndirect(t *testing.T) {
	r := fixtureReader(t)
	res := r.TestedBy("çevre_hesapla", QueryOptions{})
	if len(res.Tests) != 2 {
		t.Fatalf("%+v", res.Tests)
	}
	first, second := res.Tests[0], res.Tests[1]
	if first.Steps != 0 || first.In.ID != "test_block:lib/geo_test.tr#8" || first.Confidence != Extracted {
		t.Errorf("direct first: %+v", first)
	}
	if second.Steps != 2 || second.In.ID != "test_block:lib/geo_test.tr#3" || second.Confidence != Inferred {
		t.Errorf("indirect through yaz_alan: %+v", second)
	}
	if got := r.TestedBy("sayaç", QueryOptions{}); len(got.Tests) != 0 {
		t.Errorf("nobody tests sayaç: %+v", got.Tests)
	}
}

func TestImpactOfAModule(t *testing.T) {
	res := fixtureReader(t).Impact("lib/geo.tr", QueryOptions{})
	if res.Status != StatusOK || res.Affected == nil {
		t.Fatalf("%+v", res)
	}
	mods := map[string]bool{}
	for _, m := range res.Affected.Modules {
		mods[m.Name] = true
	}
	if !mods["examples/ana.tr"] || !mods["lib/geo_test.tr"] || len(mods) != 2 {
		t.Errorf("importing modules: %v", mods)
	}
	for _, u := range res.Affected.Users {
		if u.Pos.File == "lib/geo.tr" {
			t.Errorf("uses inside the module are not impact: %+v", u)
		}
	}
	if len(res.Affected.Tests) != 2 {
		t.Errorf("tests that reach the module's declarations: %+v", res.Affected.Tests)
	}
}

// Komut çıktıları altın dosyalarla birebirdir (-update ile yeniden yazılır).
func TestCommandOutputGolden(t *testing.T) {
	r := fixtureReader(t)
	cases := map[string]*Result{
		"definition_kare.json":     r.Definition("Kare", QueryOptions{}),
		"references_kare.json":     r.References("Kare", QueryOptions{}),
		"tested_by_cevre.json":     r.TestedBy("çevre_hesapla", QueryOptions{}),
		"impact_geo.json":          r.Impact("lib/geo.tr", QueryOptions{}),
		"references_print.json":    r.References("print", QueryOptions{}),
		"definition_notfound.json": r.Definition("yaz_alam", QueryOptions{}),
	}
	for name, res := range cases {
		got, err := EncodeJSON(res)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "golden", name)
		if *update {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.ReplaceAll(string(want), "\r\n", "\n") != string(got) {
			t.Errorf("%s changed (go test -update to rewrite):\n%s", name, got)
		}
	}
}

func TestTextOutput(t *testing.T) {
	text := Text(fixtureReader(t).References("alan", QueryOptions{}))
	for _, want := range []string{KindLabel(KindMethod) + " geo.Kare.alan", RoleLabel(RoleCall), "[INFERRED]", "geo.yaz_alan", "lib/geo.tr:"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	// Başlıklar ileti kodlarından gelir (okuyanın dilinde).
	if !strings.Contains(text, strings.SplitN(messages.Text(messages.H8785, 1, ""), "1", 2)[0]) {
		t.Errorf("the users heading comes from H8785:\n%s", text)
	}
}

// Kural: bir test bloğu bir işlev yerleşiğini çağırınca, argüman olarak verince (`indirge(maks)`) ya da atadığı
// değişkeni çağırınca (`g = karekök` ... `g(4)`) sınar; yalnızca değer olarak anması (`f = zaman_ms`, f çağrılmaz) onu
// sınamaz. Sabit yerleşik (pi) okunarak kullanılır, okunması sınamaktır (bağımsız denetim S3: anılan yerleşik testli
// görünüyor, untested_names.txt'teki istisnayı bayat yapıyordu; DENETİM doğrulaması: argüman ve atama da sınamaktır).
// Go testindeki TR parçası da aynı kuralla (Snippet).
func TestATestBlockTestsAFunctionBuiltinOnlyByCallingIt(t *testing.T) {
	dir := t.TempDir()
	src := "test \"değer\" {\n    değişken f = zaman_ms\n    doğrula(doğru)\n}\n\ntest \"çağrı\" {\n    doğrula(sinüs(pi) < 1)\n}\n" +
		"test \"argüman\" {\n    doğrula([3, 1, 2].indirge(maks) == 3)\n}\n" +
		"test \"atama\" {\n    değişken g = karekök\n    doğrula(g(4) == 2)\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "m_test.tr"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(idx)
	for name, want := range map[string]int{"zaman_ms": 0, "sinüs": 1, "pi": 1, "doğrula": 4, "maks": 1, "karekök": 1} {
		if got := len(r.TestedBy(name, QueryOptions{}).Tests); got != want {
			t.Errorf("TestedBy(%s): %d tests, want %d", name, got, want)
		}
	}
	codes := map[string]bool{}
	for _, u := range Snippet("değişken f = zaman_ms\nyazdır(sinüs(pi))\n") {
		codes[u.Code] = true
	}
	if codes["TR_fn_time_ms"] || !codes["TR_const_pi"] || len(codes) != 3 {
		t.Errorf("Snippet: zaman_ms only named, sinüs and yazdır called, pi read: %v", codes)
	}
}

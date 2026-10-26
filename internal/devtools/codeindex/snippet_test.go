package codeindex

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/lang/source"
)

const builtinUseSource = `değişken kn = kontrol_noktası(1, {}, her_dakika: 3)
kn.güncelle(1)

test "kontrol noktası adımı bildirir" {
    değişken k = kontrol_noktası(1, {}, her_dakika: 1)
    k.güncelle(2)
}
`

func builtinUseReader(t *testing.T) *Reader {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests", "a_test.tr"), []byte(builtinUseSource), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return NewReader(idx)
}

// Kural: üye (mem) ve adlı argüman (arg) kodları da yerleşik düğümdür; test bloğunun kullandığı her yerleşik, üye ve
// adlı argüman o ad koduna `tests` bağıdır (`sınayanlar` boş dönmez). Ad koduyla da sorulabilir (`TR_arg_every_minutes`).
func TestMembersAndNamedArgumentsAreBuiltinsTestedByTestBlocks(t *testing.T) {
	r := builtinUseReader(t)
	for _, q := range []string{"TR_arg_every_minutes", "her_dakika", "TR_mem_update", "güncelle"} {
		res := r.TestedBy(q, QueryOptions{})
		if res.Status != StatusOK || len(res.Tests) != 1 || !strings.Contains(res.Tests[0].In.Name, "kontrol noktası adımı bildirir") {
			t.Errorf("sınayanlar %s: %+v", q, res)
		}
	}
	users := r.References("TR_arg_every_minutes", QueryOptions{})
	if users.Status != StatusOK || users.Counts == nil || users.Counts.Total < 2 {
		t.Errorf("kullananlar TR_arg_every_minutes sees the program and the test block: %+v", users)
	}
}

// Kural: aynı ad hem bildirilmiş bir adı (komut satırı seçeneği) hem yerleşik bir adı gösteriyorsa ikisi de aday olur.
func TestADeclaredNameAndABuiltinNameAreBothCandidates(t *testing.T) {
	dir := t.TempDir()
	src := "komut_satırı(her_dakika: 5)\nkontrol_noktası(1, {}, her_dakika: 3)\n"
	if err := os.WriteFile(filepath.Join(dir, "a.tr"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, n := range NewReader(idx).Resolve("her_dakika", "") {
		kinds = append(kinds, string(n.Kind))
	}
	if len(kinds) < 1 {
		t.Fatal("her_dakika resolves to nothing")
	}
	hasBuiltin := false
	for _, k := range kinds {
		hasBuiltin = hasBuiltin || k == string(KindBuiltin)
	}
	if !hasBuiltin {
		t.Errorf("the builtin named argument must be among the candidates: %v", kinds)
	}
}

// Kural: Snippet bir metindeki TR kodunun kullandığı yerleşik, üye ve adlı argüman kodlarını verir (Go test dizeleri
// için); ayrıştırılamayan metin nil verir.
func TestSnippetListsTheCodesAProgramTextUses(t *testing.T) {
	got, _ := Snippet(builtinUseSource, nil)
	var codes []string
	for _, u := range got {
		codes = append(codes, u.Code)
	}
	if !strings.Contains(strings.Join(codes, ","), "TR_fn_checkpoint") || !strings.Contains(strings.Join(codes, ","), "TR_arg_every_minutes") ||
		!strings.Contains(strings.Join(codes, ","), "TR_mem_update") {
		t.Errorf("codes = %v", codes)
	}
	if uses, imports := Snippet("bu bir ((( program değil", nil); uses != nil || imports != nil {
		t.Error("an unparsable text has no uses")
	}
}

// Kural: kitaplık verilince Snippet metnin `aktar "tr/ad"` ile aldığı kitaplık modüllerini dosyadaki kuralla çözer ve
// kitaplıktaki yollarını verir (kök boşken de): üst düzeyde ve iç blokta, takma adlı ya da adsız, düz ve kodlu yazımda.
// Kitaplıkta olmayan modül ve göreli aktarma yol vermez; kitaplıksız çağrı (yalnız ad kodları) aktarma vermez.
func TestSnippetListsTheLibraryModulesItImports(t *testing.T) {
	library := fstest.MapFS{
		"modelx.tr":  {Data: []byte("aktar \"katmanx\"\nfonksiyon modelx_kur() { sonuç 1 }\n")},
		"katmanx.tr": {Data: []byte("fonksiyon GPT() { sonuç 1 }\n")},
	}
	src := "aktar \"tr/modelx\"\naktar \"yerel\"\naktar \"tr/olmayan\"\ntest \"t\" {\n    aktar \"tr/katmanx\" olarak k\n    yazdır(modelx_kur())\n}\naktar \"tr/modelx\" olarak h\n"
	if _, imports := Snippet(src, library); !slices.Equal(imports, []string{"lib/modelx.tr", "lib/katmanx.tr"}) {
		t.Errorf("imports = %v, want lib/modelx.tr and lib/katmanx.tr once each, in source order", imports)
	}
	coded, err := source.Encode("aktar \"tr/katmanx\"\nyazdır(GPT())\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(coded, source.Header) {
		t.Fatalf("the coded text has no header: %q", coded)
	}
	if uses, imports := Snippet(coded, library); !slices.Equal(imports, []string{"lib/katmanx.tr"}) || len(uses) == 0 {
		t.Errorf("coded text: imports %v, uses %v", imports, uses)
	}
	if _, imports := Snippet(src, nil); imports != nil {
		t.Errorf("without a library no import resolves: %v", imports)
	}
}

// Kural: kitaplık verilince Snippet metnin sınadığı kitaplık adlarını kodlarıyla verir (yerleşikle aynı kural: işlev ve
// metot çağrılınca, yapı okununca); yalnız anılan işlev sayılmaz, kitaplığın çekirdekle ortak kodlu parametresi
// (adım: TR_arg_step) de sayılmaz. Go testi bir TR betiğini dosyaya yazıp tr ikilisiyle koşunca codegraph testi bu
// kodlara bağlar (TestReadyModelMatchesTransformers → TR_fn_pretrained_model).
func TestSnippetListsTheLibraryCodesItExercises(t *testing.T) {
	library := fstest.MapFS{"hazirx.tr": {Data: []byte("yapı GPT {\n    fonksiyon ileri(x) {\n        sonuç x\n    }\n}\n\n" +
		"fonksiyon hazır_model(klasör, adım = 0) {\n    sonuç GPT()\n}\n\nfonksiyon öğrenme_oranı_planı(adım) {\n    sonuç adım\n}\n")}}
	got := func(src string) string {
		uses, _ := Snippet(src, library)
		var out []string
		for _, u := range uses {
			out = append(out, u.Code)
		}
		return strings.Join(out, " ")
	}
	if s := got("aktar \"tr/hazirx\"\ndeğişken m = hazır_model(\"k\", adım: 1)\nyazdır(GPT)\n"); !strings.Contains(s, "TR_fn_pretrained_model") ||
		!strings.Contains(s, "TR_type_GPT") || strings.Contains(s, "TR_arg_step") {
		t.Errorf("uses = %s, want the called library function and the read struct, not the shared parameter code", s)
	}
	if s := got("aktar \"tr/hazirx\"\ndeğişken f = öğrenme_oranı_planı\n"); strings.Contains(s, "TR_fn_lr_schedule") {
		t.Errorf("uses = %s: a library function only named, not called, is not exercised", s)
	}
	if s := got("değişken m = hazır_model(\"k\")\n"); strings.Contains(s, "TR_fn_pretrained_model") {
		t.Errorf("uses = %s: without its import the library name is not the library's", s)
	}
}

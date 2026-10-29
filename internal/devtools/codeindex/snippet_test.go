package codeindex

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const builtinUseSource = `TR_var kn = TR_Checkpoint(1, {}, TR_every_minutes: 3)
kn.TR_update(1)

TR_test "kontrol noktası adımı bildirir" {
    TR_var k = TR_Checkpoint(1, {}, TR_every_minutes: 1)
    k.TR_update(2)
}
`

func builtinUseReader(t *testing.T) *Reader {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests", "a_test.tr"), trText(filepath.Join(dir, "tests", "a_test.tr"), builtinUseSource), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := Build(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return NewReader(idx)
}

// Kural: üye (mem) ve adlı argüman (arg) kodları da yerleşik düğümdür; test bloğunun kullandığı her yerleşik, üye ve
// adlı argüman o ad koduna `tests` bağıdır (`tested_by` boş dönmez). Ad koduyla da sorulabilir (`TR_arg_every_minutes`).
func TestMembersAndNamedArgumentsAreBuiltinsTestedByTestBlocks(t *testing.T) {
	r := builtinUseReader(t)
	for _, q := range []string{"TR_every_minutes", "her_dakika", "TR_update", "güncelle"} {
		res := r.TestedBy(q, QueryOptions{})
		if res.Status != StatusOK || len(res.Tests) != 1 || !strings.Contains(res.Tests[0].In.Name, "kontrol noktası adımı bildirir") {
			t.Errorf("sınayanlar %s: %+v", q, res)
		}
	}
	users := r.References("TR_every_minutes", QueryOptions{})
	if users.Status != StatusOK || users.Counts == nil || users.Counts.Total < 2 {
		t.Errorf("kullananlar TR_every_minutes sees the program and the test block: %+v", users)
	}
}

// Kural: aynı ad hem bildirilmiş bir adı (komut satırı seçeneği) hem yerleşik bir adı gösteriyorsa ikisi de aday olur.
func TestADeclaredNameAndABuiltinNameAreBothCandidates(t *testing.T) {
	dir := t.TempDir()
	src := "TR_command_line(TR_every_minutes: 5)\nTR_Checkpoint(1, {}, TR_every_minutes: 3)\n"
	if err := os.WriteFile(filepath.Join(dir, "a.tr"), trText(filepath.Join(dir, "a.tr"), src), 0o644); err != nil {
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
	if !strings.Contains(strings.Join(codes, ","), "TR_type_Checkpoint") || !strings.Contains(strings.Join(codes, ","), "TR_arg_every_minutes") ||
		!strings.Contains(strings.Join(codes, ","), "TR_mem_update") {
		t.Errorf("codes = %v", codes)
	}
	if uses, imports := Snippet("bu bir ((( program değil", nil); uses != nil || imports != nil {
		t.Error("an unparsable text has no uses")
	}
}

// Kural: kitaplık verilince Snippet metnin `TR_import "tr/ad"` ile aldığı kitaplık modüllerini dosyadaki kuralla çözer ve
// kitaplıktaki yollarını verir (kök boşken de): aktarmalar dosyanın başındadır (dil tanımı 2.1), takma adlı ya da adsız;
// metin başlıksız bir gövde (Go testinin dizesi) ya da başlıklı bir dosya olabilir. Kitaplıkta olmayan modül ve göreli
// aktarma yol vermez; kitaplıksız çağrı (yalnız ad kodları) aktarma vermez.
func TestSnippetListsTheLibraryModulesItImports(t *testing.T) {
	library := fstest.MapFS{
		"modelx.tr":  {Data: trText("modelx.tr", "TR_import \"katmanx\"\nTR_func modelx_kur() { TR_return 1 }\n")},
		"katmanx.tr": {Data: trText("katmanx.tr", "TR_func gpt_kur() { TR_return 1 }\n")},
	}
	src := "TR_import \"tr/modelx\"\nTR_import \"yerel\"\nTR_import \"tr/olmayan\"\nTR_import \"tr/katmanx\" TR_as k\nTR_import \"tr/modelx\" TR_as h\n" +
		"TR_test \"t\" {\n    TR_print(modelx_kur(), k.gpt_kur())\n}\n"
	if _, imports := Snippet(src, library); !slices.Equal(imports, []string{"lib/modelx.tr", "lib/katmanx.tr"}) {
		t.Errorf("imports = %v, want lib/modelx.tr and lib/katmanx.tr once each, in source order", imports)
	}
	file := string(trText("x.tr", "TR_import \"tr/katmanx\"\nTR_print(gpt_kur())\n"))
	if uses, imports := Snippet(file, library); !slices.Equal(imports, []string{"lib/katmanx.tr"}) || len(uses) == 0 {
		t.Errorf("a file with the header: imports %v, uses %v", imports, uses)
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
	library := fstest.MapFS{"hazirx.tr": {Data: trText("hazirx.tr", "TR_struct TR_GPT {\n    TR_func TR_forward(TR_x) {\n        TR_return TR_x\n    }\n}\n\n"+
		"TR_func TR_pretrained_model(TR_folder, TR_step = 0) {\n    TR_return TR_GPT()\n}\n\nTR_func TR_lr_schedule(TR_step) {\n    TR_return TR_step\n}\n")}}
	got := func(src string) string {
		uses, _ := Snippet(src, library)
		var out []string
		for _, u := range uses {
			out = append(out, u.Code)
		}
		return strings.Join(out, " ")
	}
	if s := got("TR_import \"tr/hazirx\"\nTR_var m = TR_pretrained_model(\"k\", TR_step: 1)\nTR_print(TR_GPT)\n"); !strings.Contains(s, "TR_fn_pretrained_model") ||
		!strings.Contains(s, "TR_type_GPT") || strings.Contains(s, "TR_arg_step") {
		t.Errorf("uses = %s, want the called library function and the read struct, not the shared parameter code", s)
	}
	if s := got("TR_import \"tr/hazirx\"\nTR_var f = TR_lr_schedule\n"); strings.Contains(s, "TR_fn_lr_schedule") {
		t.Errorf("uses = %s: a library function only named, not called, is not exercised", s)
	}
	if s := got("TR_var m = TR_pretrained_model(\"k\")\n"); strings.Contains(s, "TR_fn_pretrained_model") {
		t.Errorf("uses = %s: without its import the library name is not the library's", s)
	}
}

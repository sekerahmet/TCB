package checker_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// moduleDefs modül testlerinin ortak tanımlarıdır: sabit, değişken, fonksiyon, kurucusuz ve kuruculu
// yapı, durumlu tip.
const moduleDefs = `TR_const N = 2
TR_var v = 1
TR_func f() -> TR_Int {
    TR_return N
}
TR_struct Nokta {
    x: TR_Int = 0
}
TR_struct Kurulu {
    x: TR_Int = 0
    TR_func TR_init(x: TR_Int) {
        TR_self.x = x
    }
}
TR_enum Durum {
    Bekliyor
    Çalışıyor(adım: TR_Int)
}
`

// moduleCodes modül olarak denetlenen metnin 1.1 kodlarıdır (H1107, H1108).
func moduleCodes(t *testing.T, text string, module bool) []string {
	t.Helper()
	r := source.Prepare(text, source.Options{TypeCheck: true, Module: module, Body: true})
	if r.Stage == source.StageSyntax {
		t.Fatalf("sözdizimi: %v\n%s", r.Errors, text)
	}
	var out []string
	for _, err := range r.Errors {
		for _, code := range []messages.Code{messages.H1107, messages.H1108} {
			if messages.Has(err, code) {
				out = append(out, code.String())
			}
		}
	}
	return out
}

// Kural (1.1): modülün en dışındaki TR_const ve TR_var'ın ilk değeri sabit ifadedir; sabit olmayan
// ilk değer H1108'dir. Aynı metin programda hatasızdır.
func TestModuleFirstValuesAreConstant(t *testing.T) {
	constant := []string{`1`, `-2.5`, `"a \(N)"`, `TR_true`, `TR_none`, `N * 2 + 1`, `[1, N]`, `{"a": N}`,
		`(1, "a")`, `0..N`, `Nokta(x: N)`, `Durum.Bekliyor`, `Durum.Çalışıyor(N)`, `TR_Bytes([1, 2])`, `f`,
		`(x: TR_Int) => x + N`, `TR_sqrt(64.0)`, `TR_max(N, 3)`, `[1, 2][0]`, `TR_fuse(f)`}
	for _, value := range constant {
		if got := moduleCodes(t, moduleDefs+"TR_const S = "+value+"\n", true); len(got) != 0 {
			t.Errorf("%s sabittir: %v", value, got)
		}
	}
	notConstant := []string{`f()`, `v`, `v + 1`, `[f()]`, `{"a": v}`, `Kurulu(1)`, `(x: TR_Int) => x + v`,
		`TR_temp_dir()`, `"\(v)"`, `Durum.Çalışıyor(f())`}
	for _, value := range notConstant {
		text := moduleDefs + "TR_const S = " + value + "\n"
		if got := moduleCodes(t, text, true); strings.Join(got, " ") != "H1108" {
			t.Errorf("%s sabit değildir (H1108): %v", value, got)
		}
		if got := moduleCodes(t, text, false); len(got) != 0 {
			t.Errorf("programda %s serbesttir: %v", value, got)
		}
	}
}

// Kural (1.1, 12.1): modülün ve test dosyasının (TR_test içeren dosya) en dışında yalnız tanım
// bulunur; başka deyim H1107'dir. Modül de test bloğu tutabilir; test bloğu olmayan program serbesttir.
func TestModuleTopLevelHoldsOnlyDefinitions(t *testing.T) {
	for _, stmt := range []string{`TR_print(N)`, `v = 2`, `f()`, "TR_for i TR_in 0..2 {\n    v += i\n}", "TR_if v > 0 {\n    v = 0\n}"} {
		text := moduleDefs + stmt + "\n"
		if got := moduleCodes(t, text, true); strings.Join(got, " ") != "H1107" {
			t.Errorf("modülde %q H1107 vermeli: %v", stmt, got)
		}
		if got := moduleCodes(t, text, false); len(got) != 0 {
			t.Errorf("programda %q serbesttir: %v", stmt, got)
		}
		if got := moduleCodes(t, text+"TR_test \"t\" {\n    TR_assert(TR_true)\n}\n", false); strings.Join(got, " ") != "H1107" {
			t.Errorf("test dosyasında %q H1107 vermeli: %v", stmt, got)
		}
	}
	if got := moduleCodes(t, "TR_import \"tr/layers\"\n"+moduleDefs+"TR_test \"t\" {\n    TR_assert(v == 1)\n}\n", true); len(got) != 0 {
		t.Errorf("modülde test bloğu serbesttir: %v", got)
	}
}

// Kural (iletinin önerisi): H1107 deyimi bir fonksiyona alır, H1108 değeri tipiyle döndüren bir
// fonksiyon verir; iki ileti de dosyanın türünü söyler. Önerinin çalıştığı: interp
// TestModuleRuleSuggestionRuns.
func TestModuleRuleMessages(t *testing.T) {
	r := source.Prepare("TR_var v = 1\nTR_print(v)\nTR_const K = \"\\(v) adım\"\n", source.Options{TypeCheck: true, Module: true, Body: true})
	if len(r.Errors) != 2 {
		t.Fatalf("iki hata bekleniyordu: %v", r.Errors)
	}
	testkitContains(t, r.Errors[0].Error(), "modülün en dışında", "TR_func kur() {\n    TR_print(v)\n}")
	testkitContains(t, r.Errors[1].Error(), "\"K\"", "\"v\" sabit değil", "TR_func k() -> TR_String {\n    TR_return \"\\(v) adım\"\n}")
	r = source.Prepare("TR_print(1)\nTR_test \"t\" {\n    TR_assert(TR_true)\n}\n", source.Options{TypeCheck: true, Body: true})
	if len(r.Errors) != 1 || !strings.Contains(r.Errors[0].Error(), "test dosyasının en dışında") {
		t.Errorf("test dosyasının iletisi bekleniyordu: %v", r.Errors)
	}
}

func testkitContains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Errorf("iletide %q yok:\n%s", p, text)
		}
	}
}

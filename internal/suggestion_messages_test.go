package internal_test

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
	_ "github.com/sekerahmet/TCB/internal/stdlib"
	"github.com/sekerahmet/TCB/lib"
)

// suggestionCase bir iletinin önerdiği yazımın hem iletide göründüğünü hem de hatalı programa
// uygulandığında programın denetimden ve çalıştırmadan hatasız geçtiğini sınar.
type suggestionCase struct {
	code       messages.Code
	wrong      string
	replace    string
	suggestion string
}

var suggestionCases = []suggestionCase{
	{messages.H0026, "TR_pritn(1)", "TR_pritn", "TR_print"},
	{messages.H0027, "TR_var x: TR_List[TR_Int] = [1, 2]\nTR_var y: TR_Int = TR_length", "TR_length", "x.TR_length"},
	{messages.H0029, "TR_var x: TR_Int = 1\nTR_var y: TR_Bool = x.TR_true", "x.TR_true", "TR_true"},
	{messages.H0037, "TR_var x: TR_type_Int = 1", "TR_type_Int", "TR_Int"},
	{messages.H0037, "TR_var x: TR_List[TR_Int] = [1, 2]\nTR_var y: TR_Int = x.TR_mem_length", ".TR_mem_length", ".TR_length"},
	{messages.H0037, "TR_kw_var x: TR_Int = 1", "TR_kw_var", "TR_var"},
	{messages.H0037, "TR_var t: TR_Tensor = TR_zeros([1], TR_arg_requires_grad: TR_true)", "TR_arg_requires_grad:", "TR_requires_grad:"},
	{messages.H1007, "TR_var a: ondalik = 1.5", "ondalik", "TR_Float"},
	{messages.H1012, "TR_var öğrenciSayısı: TR_Int = 1", "öğrenciSayısı", "öğrenci_sayısı"},
	{messages.H1013, "TR_const en_büyük = 10", "en_büyük", "EN_BÜYÜK"},
	{messages.H1014, "TR_struct önbellek_durumu { x: TR_Int = 0 }", "önbellek_durumu", "ÖnbellekDurumu"},
	{messages.H1015, "TR_var ADIM: TR_Int = 300", "ADIM", "adım"},
	{messages.H1026, "TR_var a, b, c = (1, 2)", "TR_var a, b, c = (1, 2)", "TR_var a, b = (1, 2)"},
	{messages.H1105, `TR_var t: TR_Bool = TR_Type(1) == "Int"`, `"Int"`, "TR_Int"},
	{messages.H1106, "TR_var sayac: TR_Int = 1\nTR_print(sayaç)", "sayaç", "sayac"},
	{messages.H2410, "TR_func f(isim: TR_Int) -> TR_Int { TR_return isim }\nTR_var x: TR_Int = f(isimm: 1)", "isimm", "isim"},
	{messages.H2418, "TR_struct Nokta { konum: TR_Int = 0 }\nTR_var n: Nokta = Nokta(konumm: 1)", "konumm", "konum"},
	{messages.H2437, "TR_var sayac: TR_Int = 1\nsayaç += 2", "sayaç", "sayac"},
	{messages.H2438, "TR_var sayac: TR_Int = 1\nsayaç = 2", "sayaç", "sayac"},
	{messages.H2478, `TR_import "tr/layer"`, "tr/layer", "tr/layers"},
	{messages.H2709, "TR_func g() -> (TR_Int, TR_String) { TR_return 1, \"x\" }\nTR_var f = g\nTR_var a, b, c = f()", "TR_var a, b, c = f()", "TR_var a, b = f()"},
	{messages.H2767, "TR_enum Sonuç { Tamam(değer: TR_Int), Boş }\nTR_var s: Sonuç = Sonuç.Tamam(deger: 1)", "deger", "değer"},
	{messages.H2778, "TR_var t = (10, \"x\")\nTR_var v: TR_Int = t[0]", "t[0]", "t.0"},
	{messages.H6811, `TR_var t: TR_Tensor = TR_zeros([1], TR_dtype: "float64")`, "float64", "float32"},
	{messages.H7229, `TR_var ayar = TR_command_line({"adım": 1}, TR_descriptions: {"adim": "x"})`, "adim", "adım"},
}

// maxUntestedSuggestionCodes izin listesindeki kayıt sayısının üst sınırıdır; liste yalnız küçülür.
const maxUntestedSuggestionCodes = 27

// untestedSuggestionCodes definitions.json'da suggestion/example yuvası taşıyan ama henüz bu tabloda
// sınanmayan kodlardır. Her yeni sınanan kod buradan silinir; liste büyüyemez.
var untestedSuggestionCodes = map[messages.Code]string{
	messages.H0028: "henüz sınanmadı",
	messages.H0143: "henüz sınanmadı",
	messages.H5529: "henüz sınanmadı",
	messages.H6001: "henüz sınanmadı",
	messages.H6002: "henüz sınanmadı",
	messages.H6104: "henüz sınanmadı",
	messages.H6402: "henüz sınanmadı",
	messages.H6405: "henüz sınanmadı",
	messages.H6430: "henüz sınanmadı",
	messages.H6431: "henüz sınanmadı",
	messages.H6513: "henüz sınanmadı",
	messages.H6514: "henüz sınanmadı",
	messages.H6520: "henüz sınanmadı",
	messages.H6530: "henüz sınanmadı",
	messages.H6532: "henüz sınanmadı",
	messages.H6700: "henüz sınanmadı",
	messages.H7203: "henüz sınanmadı",
	messages.H7209: "henüz sınanmadı",
	messages.H7319: "henüz sınanmadı",
	messages.H7435: "henüz sınanmadı",
	messages.H7453: "henüz sınanmadı",
	messages.H7900: "henüz sınanmadı",
	messages.H7903: "henüz sınanmadı",
	messages.H8664: "henüz sınanmadı",
	messages.H8768: "henüz sınanmadı",
	messages.H8820: "henüz sınanmadı",
	messages.H8864: "henüz sınanmadı",
}

func TestMessageSuggestionsRunCleanly(t *testing.T) {
	interp.SetLibrary(lib.Files)
	tested := map[messages.Code]bool{}
	for _, tc := range suggestionCases {
		tested[tc.code] = true
		verifySuggestionCase(t, tc)
	}
	checkSuggestionCoverage(t, tested)
}

func verifySuggestionCase(t *testing.T, tc suggestionCase) {
	t.Helper()
	diags := collectProgramDiagnostics(tc.wrong)
	var matched error
	for _, d := range diags {
		if messages.Has(d, tc.code) {
			matched = d
			break
		}
	}
	if matched == nil {
		t.Fatalf("%s (%q): beklenen kod üretilmedi; alınan: %v", tc.code, tc.wrong, diags)
	}
	for _, lang := range []string{"tr", "en"} {
		if text := messages.ErrorIn(matched, lang); !strings.Contains(text, tc.suggestion) {
			t.Errorf("%s (%s): ileti %q önerisini içermiyor: %q", tc.code, lang, tc.suggestion, text)
		}
	}
	fixed := strings.Replace(tc.wrong, tc.replace, tc.suggestion, 1)
	if fixed == tc.wrong {
		t.Fatalf("%s: %q içinde %q bulunamadı", tc.code, tc.wrong, tc.replace)
	}
	if after := collectProgramDiagnostics(fixed); len(after) > 0 {
		t.Errorf("%s: öneri uygulandıktan sonra (%q) hata/uyarı kaldı: %v", tc.code, fixed, after)
	}
}

func collectProgramDiagnostics(code string) []error {
	r := source.Prepare(code, source.Options{Body: true})
	var out []error
	out = append(out, r.Errors...)
	if r.Stage == source.StageSyntax || r.Program == nil {
		return out
	}
	chk := checker.New()
	chk.CheckProgram(r.Program)
	out = append(out, chk.Errs()...)
	out = append(out, chk.Warnings()...)
	vm := interp.New()
	var buf bytes.Buffer
	vm.Out, vm.Err = &buf, &buf
	if _, err := vm.Run(r.Program); err != nil {
		out = append(out, err)
	}
	vm.WaitAsync()
	return out
}

func checkSuggestionCoverage(t *testing.T, tested map[messages.Code]bool) {
	t.Helper()
	if len(untestedSuggestionCodes) > maxUntestedSuggestionCodes {
		t.Errorf("untestedSuggestionCodes: %d kayıt, en fazla %d; izin listesi yalnızca küçülür",
			len(untestedSuggestionCodes), maxUntestedSuggestionCodes)
	}
	defined := definitionSuggestionCodes(t)
	for _, c := range defined {
		inTable := tested[c]
		reason, inAllow := untestedSuggestionCodes[c]
		switch {
		case !inTable && !inAllow:
			t.Errorf("%s: suggestion/example yuvalı ileti ne suggestionCases tablosunda ne de untestedSuggestionCodes izin listesinde", c)
		case inTable && inAllow:
			t.Errorf("%s: hem suggestionCases tablosunda hem untestedSuggestionCodes izin listesinde; izin listesinden çıkarın", c)
		case inAllow && strings.TrimSpace(reason) == "":
			t.Errorf("%s: untestedSuggestionCodes kaydının gerekçesi boş", c)
		}
	}
	for c := range untestedSuggestionCodes {
		if !slices.Contains(defined, c) {
			t.Errorf("%s: untestedSuggestionCodes içinde ama definitions.json'da suggestion/example yuvası yok", c)
		}
	}
}

func definitionSuggestionCodes(t *testing.T) []messages.Code {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("internal", "lang", "messages", "definitions.json"))
	if err != nil {
		t.Fatal(err)
	}
	defs, err := messages.ParseDefinitions(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []messages.Code
	for _, c := range defs.Codes {
		for _, p := range defs.Params[c] {
			lp := strings.ToLower(p)
			if strings.Contains(lp, "suggestion") || strings.Contains(lp, "example") {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

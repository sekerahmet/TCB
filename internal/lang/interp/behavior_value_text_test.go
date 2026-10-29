package interp

import (
	"bytes"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Değerin metni dilden bağımsızdır (3.14 ilk yanı; dosyanın dili yoktur, 2.1): Metin(), yazdır,
// metin yerleştirmesi ve Tip() adları kimliğin adıyla çıkar (true, none, Int) — programı hangi dilde
// okursa okusun, kitaplıkta da, her makinede aynı. Yakalanan hatanın metni sabit İngilizce katalogdan
// kurulur (caughtErrorLanguage). Testlerin programları dosya gibi kimliklerle yazılır.

// runCoded kimliklerle yazılmış main gövdesini (başlıksız) programın kendisi gibi çalıştırır; files
// çalışma klasörüne olduğu gibi yazılır.
func runCoded(t *testing.T, main string, files map[string]string) string {
	t.Helper()
	out, err := runCodedErr(t, main, files)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return out
}

// runCodedErr runCoded gibidir; çalışma hatasını testi düşürmeden döndürür.
func runCodedErr(t *testing.T, main string, files map[string]string) (string, error) {
	t.Helper()
	coded := source.Header + "\n" + main
	root := t.TempDir()
	runInDir(t, root)
	for name, content := range files {
		testkit.WritePath(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	r := source.Prepare(coded, source.Options{})
	if r.Stage != "" {
		t.Fatalf("hazırlık: %v", r.Errors)
	}
	var out bytes.Buffer
	m := New()
	m.Out, m.Err, m.Stdin = &out, &out, strings.NewReader("\n")
	m.SourceFile, m.SourceCode = filepath.Join(root, "main.tr"), r.Text
	_, err := m.Run(r.Program)
	return out.String(), err
}

// Değerin metni kimliğin adıdır (true, none, Int); programın çalıştığı süreç Türkçedir (TR_LANG).
func TestValueTextIsLanguageIndependent(t *testing.T) {
	want := "true none Int [true, none, {\"k\": false}]\n" +
		"true none Bool [List]\n" +
		"true true true\n"
	if out := runCoded(t, `TR_var b = TR_true
TR_print(b, TR_none, TR_Type(1), [TR_true, TR_none, {"k": TR_false}])
TR_print("\(b) \(TR_none) \(TR_Type(b)) \([TR_Type([])])")
TR_print(TR_String(b) == "true", TR_String(TR_none) == "none", TR_String(TR_Type(1.5)) == "Float")
`, nil); out != want {
		t.Errorf("program:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Kitaplığın ve yerel modülün fonksiyonu da aynı metni verir; geri çağırma da.
func TestValueTextIsTheSameInEveryModule(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\nTR_func anlat(x) { TR_return \"\\(x) \" + TR_String(TR_Type(x)) }\nTR_func uygula(f) { TR_print(f(TR_true)) }\n" +
		"" +
		"")}})
	out := runCoded(t, `TR_import "tr/kitap" TR_as k
TR_import "yerel" TR_as y
TR_print(k.anlat(TR_true), y.anlat(TR_none))
k.uygula((x) => "\(x) " + TR_String(x))
y.uygula((x) => TR_String([x]))
`, map[string]string{"yerel.tr": source.Header + "\nTR_func anlat(x) { TR_return TR_String(x) }\nTR_func uygula(f) { TR_print(f(TR_none)) }\n"})
	want := "true Bool none\ntrue true\n[none]\n"
	if out != want {
		t.Errorf("metin her modülde aynı olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Fırlatılan değerin ve doğrula'nın mesaj değerinin metni de dilden bağımsızdır.
func TestThrownValueTextIsLanguageIndependent(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\nTR_func at() { TR_throw [TR_true, TR_none] }\n")}})
	out := runCoded(t, `TR_import "tr/kitap" TR_as k
TR_try { TR_throw TR_true } TR_catch e { TR_print(e, e == "true") }
TR_try { TR_throw [TR_none, TR_Int] } TR_catch e { TR_print(e) }
TR_try { k.at() } TR_catch e { TR_print(e) }
TR_try { TR_assert(TR_false, TR_none) } TR_catch e { TR_print(e.TR_ends_with(": none")) }
`, nil)
	want := "true true\n[none, Int]\n[true, none]\ntrue\n"
	if out != want {
		t.Errorf("yakalanan değerin metni dilden bağımsız olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Ondalık() sonsuz ve tanımsız sayının dilden bağımsız adını okur ("inf", "nan"); bir dildeki ad
// ("sonsuz") okunmaz; Metin(Ondalık(...)) aynı adı verir, gidiş dönüş tutar (3.2).
func TestFloatValueNamesRoundTrip(t *testing.T) {
	out := runCoded(t, `TR_print(TR_Float("inf"), -TR_Float("inf"), TR_Float("nan"), TR_String(TR_Float("-inf")) == "-inf", TR_Float(TR_String(TR_Float("inf"))))
TR_try { TR_Float("sonsuz") } TR_catch e { TR_print("not read") }
`, nil)
	if want := "inf -inf nan true inf\nnot read\n"; out != want {
		t.Errorf("değer adları:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Kural (3.14, 3.2): her değerin tek metni vardır. Yerleşiğin, fonksiyonun ve yapının metni sabit
// İngilizce ve kimlik öneksizdir; ondalık 3.2'nin eşiğiyle yazılır (bilimsel yalnız üs < -4 ya da
// >= 16); TR_inf ve TR_nan programda yazılır. Inspect, TR_String ve yazdır aynı metni verir; testin
// süreç dili (TR_LANG) Türkçedir.
func TestOneValueTextForEveryValue(t *testing.T) {
	out := runCoded(t, `TR_func f() {}
TR_struct P { a }
TR_print([TR_print, TR_Int, f, P, P(1), TR_inf, -TR_inf, TR_nan, 1e6, 1234567.0, 1e15, 1e16, 0.00001, -0.0, (x) => x])
TR_print(TR_String(TR_print), "\(f)", TR_String(1e15), TR_String(TR_inf) == "inf")
`, nil)
	want := "[<builtin print>, Int, <function f>, P, P(a: 1), inf, -inf, nan, 1000000.0, 1234567.0, " +
		"1000000000000000.0, 1e+16, 1e-05, -0.0, <lambda>]\n<builtin print> <function f> 1000000000000000.0 true\n"
	if out != want {
		t.Errorf("değerin metni:\n%s\nbeklenen:\n%s", out, want)
	}
	m := New()
	for _, v := range []Value{True, Null, &FloatVal{Val: 1e6}, &FloatVal{Val: math.NaN()}, TypeOfValue(Null),
		m.Builtin(codes.FnPrint), &ListVal{Elements: []Value{False, &StringVal{Val: "a"}}}} {
		if got, want := v.Inspect(), m.textOf(v); got != want {
			t.Errorf("Inspect %q, program metni %q: aynı olmalı", got, want)
		}
	}
}

// Yakalanan çalışma anı hatası programın Metin değeridir: ileti sabit İngilizce katalogdan kurulur,
// içindeki tip ve değerlerle; süreç Türkçe olsa (TR_LANG) ve hata Türkçe adlı bir kitaplık fonksiyonunda
// oluşsa da aynıdır. Satır dosyanın satırıdır (başlık 1. satır).
func TestCaughtErrorTextIsEnglish(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\nTR_func taşır() { TR_return [1][5] }\n")}})
	out := runCoded(t, "TR_import \"tr/kitap\" TR_as k\nTR_try { [1].TR_join(TR_true) } TR_catch e { TR_print(e) }\nTR_try { k.taşır() } TR_catch e { TR_print(e) }\n", nil)
	for _, want := range []string{"Line 3:", "list.join() expects String as argument 1, Bool (true) given", "List index out of range: 5 (length 1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q yok:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Satır") || strings.Contains(out, "Mantık") {
		t.Errorf("yakalanan hata Türkçe:\n%s", out)
	}
}

// Kodlu ileti olmayan hata türleri de (dosya hatası, ertele hatası; messages.Localized) yakalanınca
// İngilizce yazılır. Her hata türünün Localized olduğunu TestEveryErrorTypeIsLocalized (messages) korur.
func TestCaughtErrorTypesAreEnglish(t *testing.T) {
	out := runCoded(t, `TR_try { TR_read_file("missing.txt") } TR_catch e { TR_print(e) }
TR_func f() {
	TR_defer { TR_var a = [1][7] }
	TR_var b = [1][5]
}
TR_try { f() } TR_catch e { TR_print(e) }
`, nil)
	for _, want := range []string{`read_file(): "missing.txt" could not be read: no such file or folder`, "error in defer as well:"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q yok:\n%s", want, out)
		}
	}
	for _, turkish := range []string{"okunamadı", "dosya_oku", "ertele", "Satır", "bakılan"} {
		if strings.Contains(out, turkish) {
			t.Errorf("yakalanan hatada Türkçe %q:\n%s", turkish, out)
		}
	}
}

// Kural (3.7): sözlük anahtarı bir metindir ve çevrilmez: {"step": 1} her gösterimde aynı anahtardır,
// yazdır onu tırnakla yazar. Anahtar yalnız köşeli parantezle okunur; noktayla okuma (bir üye adıyla
// da) hatadır ve iletisi köşeli parantezli yazımı verir (sabit İngilizce, yakalanan hata).
func TestDictKeysAreTextNotMembers(t *testing.T) {
	out := runCoded(t, "TR_var cfg = {\"step\": 1}\ncfg[\"step\"] += 1\nTR_print(cfg, cfg[\"step\"], cfg.TR_keys())\nTR_try { TR_print(cfg.TR_step) } TR_catch e { TR_print(e) }\n", nil)
	if want := "{\"step\": 2} 2 [\"step\"]\n"; !strings.HasPrefix(out, want) || !strings.Contains(out, `write ["step"]`) {
		t.Errorf("sözlük anahtarı metin olmalı, noktayla okuma hata vermeli:\n%s", out)
	}
}

// Programın değerden metin yaptığı her yer aynı kuralı izler: yerleştirme (metne başka tip '+'
// ile eklenmez, 4.10), birleştir, dosya_yaz'ın içeriği, girdi'nin istemi; kitaplığın içinde de.
func TestEveryTextConversionIsLanguageIndependent(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\n" +
		"TR_func ekle(x) { TR_return \"a\\(x),\" + [x, TR_none].TR_join(\"/\") }\n")}})
	out := runCoded(t, `TR_import "tr/kitap" TR_as k
TR_print("x\(TR_true)", "\(TR_none)!", [TR_true, TR_none, TR_Int].TR_join(","), k.ekle(TR_false))
TR_write_file("out.txt", [TR_false])
TR_print(TR_read_file("out.txt"))
TR_input(TR_true)
`, nil)
	want := "xtrue none! true,none,Int afalse,false/none\n[false]\ntrue"
	if out != want {
		t.Errorf("değerin metni dilden bağımsız olmalı:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Kural: sınır dışı indeksin ipucu öğeyi okuyan metodu çağrı olarak yazar: nokta, metodun adı
// iletinin dilinde ve indeks (.varsa_al(5), .get(5)) (4D denetimi, bulgu 5: nokta şablondadır,
// silinince bu test kalır).
func TestIndexErrorHintWritesTheGetCall(t *testing.T) {
	_, err := New().Run(mustParseProgram(t, "[1][5]\n"))
	if tr, en := messages.ErrorIn(err, "tr"), messages.ErrorIn(err, "en"); !strings.Contains(tr, ".varsa_al(5) ?? ") || !strings.Contains(en, ".get(5) ?? ") {
		t.Errorf("tr %q, en %q", tr, en)
	}
}

// Kural (B1): kurucusu olan yapının oluşturulmasındaki argüman hatası yapının adını, metodun
// argüman hatası Yapı.metot'u söyler (Python: Point.__init__(), Point.go()).
func TestStructCallErrorsNameTheStruct(t *testing.T) {
	out := runCoded(t, `TR_struct Point {
	x
	TR_func TR_init(a, b = 2) { TR_self.x = a + b }
	TR_func go(n) { TR_self.x = n }
}
TR_try { Point(1, 2, 3) } TR_catch e { TR_print(e) }
TR_try { Point(1, z: 2) } TR_catch e { TR_print(e) }
TR_try { Point(1).go(1, 2) } TR_catch e { TR_print(e) }
TR_try { Point(1).go() } TR_catch e { TR_print(e) }
`, nil)
	for _, want := range []string{"the struct 'Point' takes at most 2 arguments, 3 given",
		"the struct 'Point' takes no parameter named 'z'", "the function 'Point.go' takes at most 1 argument, 2 given",
		"the function 'Point.go'"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q yok:\n%s", want, out)
		}
	}
	if strings.Contains(out, "'init'") || strings.Contains(out, "kurucu") {
		t.Errorf("kurucunun adı geçmemeli:\n%s", out)
	}
}

// Kural (B11, 2.3a): tanımsız ada öneri dilin adlarını her yazım dilindeki yazımıyla karşılaştırır ve
// kimliğin yazımıyla önerir (prnt → 'TR_print': dosyaya olduğu gibi yazılır); kullanıcının adı olduğu gibi.
func TestUndefinedNameSuggestionIsTheIdentity(t *testing.T) {
	out := runCoded(t, `TR_var counter = 1
TR_try { prnt(1) } TR_catch e { TR_print(e) }
TR_try { TR_print(countr) } TR_catch e { TR_print(e) }
`, nil)
	for _, want := range []string{"Did you mean 'TR_print'?", "Did you mean 'counter'?"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q yok:\n%s", want, out)
		}
	}
}

package interp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Değerin metni dilden bağımsızdır (3.14 ilk yanı; dosyanın dili yoktur, 2.1): metin(), yazdır,
// metin yerleştirmesi ve tip() adları kimliğin adıyla çıkar (true, none, Int) — programı hangi dilde
// okursa okusun, kitaplıkta da, her makinede aynı. Yakalanan hatanın metni sabit İngilizce katalogdan
// kurulur (caughtErrorLanguage). Testlerin programları bir dildeki gösterimde yazılır ve kodlanarak
// çalıştırılır (dosya kimliklerle yazılır).

// runEnglish İngilizce gösterimde yazılmış main metnini kodlayıp programın kendisi gibi çalıştırır;
// files çalışma klasörüne yazılan modüllerdir.
func runEnglish(t *testing.T, main string, files map[string]string) string {
	t.Helper()
	return runCoded(t, main, "en", files)
}

// runCoded lang gösteriminde yazılmış main metnini kodlayıp programın kendisi gibi çalıştırır; files
// çalışma klasörüne olduğu gibi yazılır.
func runCoded(t *testing.T, main, lang string, files map[string]string) string {
	t.Helper()
	out, err := runCodedErr(t, main, lang, files)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return out
}

// runCodedErr runCoded gibidir; çalışma hatasını testi düşürmeden döndürür.
func runCodedErr(t *testing.T, main, lang string, files map[string]string) (string, error) {
	t.Helper()
	l, err := codes.LoadLanguage(lang)
	if err != nil {
		t.Fatal(err)
	}
	coded, err := source.EncodeIn(main, l)
	if err != nil {
		t.Fatalf("%s metni kodlanamadı: %v", lang, err)
	}
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
	_, err = m.Run(r.Program)
	return out.String(), err
}

// Aynı program İngilizce ve Türkçe gösterimde yazılsa da (dosya aynı kimlikleri taşır) aynı metni verir.
func TestValueTextIsLanguageIndependent(t *testing.T) {
	want := "true none Int [true, none, {\"k\": false}]\n" +
		"true none Bool [List]\n" +
		"true true true\n"
	for lang, main := range map[string]string{
		"en": `var b = true
print(b, none, type(1), [true, none, {k: false}])
print("\(b) \(none) \(type(b)) \([type([])])")
print(str(b) == "true", str(none) == "none", str(type(1.5)) == "Float")
`,
		"tr": `değişken b = doğru
yazdır(b, yok, tip(1), [doğru, yok, {k: yanlış}])
yazdır("\(b) \(yok) \(tip(b)) \([tip([])])")
yazdır(metin(b) == "true", metin(yok) == "none", metin(tip(1.5)) == "Float")
`,
	} {
		if out := runCoded(t, main, lang, nil); out != want {
			t.Errorf("%s gösteriminde yazılan program:\n%s\nbeklenen:\n%s", lang, out, want)
		}
	}
}

// Kitaplığın ve yerel modülün fonksiyonu da aynı metni verir; geri çağırma da.
func TestValueTextIsTheSameInEveryModule(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\n" +
		"TR_func anlat(x) { TR_return \"\\(x) \" + TR_str(TR_type(x)) }\n" +
		"TR_func uygula(f) { TR_print(f(TR_true)) }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
import "yerel" as y
print(k.anlat(true), y.anlat(none))
k.uygula((x) => "\(x) " + str(x))
y.uygula((x) => str([x]))
`, map[string]string{"yerel.tr": source.Header + "\nTR_func anlat(x) { TR_return TR_str(x) }\nTR_func uygula(f) { TR_print(f(TR_none)) }\n"})
	want := "true Bool none\ntrue true\n[none]\n"
	if out != want {
		t.Errorf("metin her modülde aynı olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Fırlatılan değerin ve doğrula'nın mesaj değerinin metni de dilden bağımsızdır.
func TestThrownValueTextIsLanguageIndependent(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\nTR_func at() { TR_throw [TR_true, TR_none] }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
try { throw true } catch e { print(e, e == "true") }
try { throw [none, Int] } catch e { print(e) }
try { k.at() } catch e { print(e) }
try { assert(false, none) } catch e { print(e.ends_with(": none")) }
`, nil)
	want := "true true\n[none, Int]\n[true, none]\ntrue\n"
	if out != want {
		t.Errorf("yakalanan değerin metni dilden bağımsız olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// ondalık() sonsuz ve tanımsız sayının dilden bağımsız adını okur ("inf", "nan"); bir dildeki ad
// ("sonsuz") okunmaz; metin(ondalık(...)) aynı adı verir, gidiş dönüş tutar (3.2).
func TestFloatValueNamesRoundTrip(t *testing.T) {
	out := runEnglish(t, `print(float("inf"), -float("inf"), float("nan"), str(float("-inf")) == "-inf", float(str(float("inf"))))
try { float("sonsuz") } catch e { print("not read") }
`, nil)
	if want := "inf -inf nan true inf\nnot read\n"; out != want {
		t.Errorf("değer adları:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Yakalanan çalışma anı hatası programın Metin değeridir: ileti sabit İngilizce katalogdan kurulur,
// içindeki tip ve değerlerle; program hangi gösterimde yazılırsa yazılsın ve hata Türkçe adlı bir
// kitaplık fonksiyonunda oluşsa da aynıdır. Satır dosyanın satırıdır (başlık 1. satır).
func TestCaughtErrorTextIsEnglish(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\nTR_func taşır() { TR_return [1][5] }\n")}})
	for lang, main := range map[string]string{
		"en": "import \"tr/kitap\" as k\ntry { [1].join(true) } catch e { print(e) }\ntry { k.taşır() } catch e { print(e) }\n",
		"tr": "aktar \"tr/kitap\" olarak k\ndene { [1].birleştir(doğru) } yakala e { yazdır(e) }\ndene { k.taşır() } yakala e { yazdır(e) }\n",
	} {
		out := runCoded(t, main, lang, nil)
		for _, want := range []string{"Line 3:", "list.join() expects String as argument 1, Bool (true) given", "List index out of range: 5 (length 1)"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: %q yok:\n%s", lang, want, out)
			}
		}
		if strings.Contains(out, "Satır") || strings.Contains(out, "Mantık") {
			t.Errorf("%s: yakalanan hata Türkçe:\n%s", lang, out)
		}
	}
}

// Kodlu ileti olmayan hata türleri de (dosya hatası, ertele hatası; messages.Localized) yakalanınca
// İngilizce yazılır. Her hata türünün Localized olduğunu TestEveryErrorTypeIsLocalized (messages) korur.
func TestCaughtErrorTypesAreEnglish(t *testing.T) {
	out := runEnglish(t, `try { read_file("missing.txt") } catch e { print(e) }
func f() {
	defer { var a = [1][7] }
	var b = [1][5]
}
try { f() } catch e { print(e) }
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

// Kimlik taşıyan çıplak sözlük anahtarı ({TR_step: 1}) içeride kimliğin dilden bağımsız adıyla
// saklanır ("step"; 3.7, 17.3): anahtarlar() ve metin anahtar (d["step"]) aynı anahtarı görür; yazdır
// kodlu anahtarı tırnaksız, metin anahtarı tırnaklı yazar. d.TR_step okuması ve yazması bu anahtarı
// arar; bir dildeki ad ("adım") ayrı bir metin anahtardır, çevrilmez. Yeni anahtar kodla eklenir.
func TestDictCodedKeysAreLanguageIndependent(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\n" +
		"TR_func artır(ayar) {\n    ayar.TR_step += 1\n    ayar.TR_lr = 0.1\n    TR_return ayar.TR_step\n}\n" +
		"TR_func yeni() { TR_return {TR_step: 7, renk: 1} }\n" +
		"TR_func göster(d) { TR_print(d, d.TR_keys()) }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
var cfg = {step: 1, renk: "mavi"}
print(k.artır(cfg), cfg.keys(), cfg.lr, cfg["adım"], cfg["step"])
cfg.lr = 0.2
var t = k.yeni()
t.step = 8
print(cfg, t, t.step)
k.göster(t)
var both = {step: 1}
both["adım"] = 2
both.step += 10
print(both)
var d = {step: 1, foo: 2}
print(d.keys(), d["step"], d.step)
`, nil)
	want := "2 [\"step\", \"renk\", \"lr\"] 0.1 none 2\n" +
		"{step: 2, \"renk\": \"mavi\", lr: 0.2} {step: 8, \"renk\": 1} 8\n" +
		"{step: 8, \"renk\": 1} [\"step\", \"renk\"]\n" +
		"{step: 11, \"adım\": 2}\n" + // metin anahtar "adım" ayrı bir anahtardır, tırnaklı
		"[\"step\", \"foo\"] 1 1\n"
	if out != want {
		t.Errorf("kodlu anahtar dilden bağımsız saklanmalı ve yazdırılmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Programın değerden metin yaptığı her yer aynı kuralı izler: yerleştirme (metne başka tip '+'
// ile eklenmez, 4.10), birleştir, dosya_yaz'ın içeriği, girdi'nin istemi; kitaplığın içinde de.
func TestEveryTextConversionIsLanguageIndependent(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(source.Header + "\n" +
		"TR_func ekle(x) { TR_return \"a\\(x),\" + [x, TR_none].TR_join(\"/\") }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
print("x\(true)", "\(none)!", [true, none, Int].join(","), k.ekle(false))
write_file("out.txt", [false])
print(read_file("out.txt"))
input(true)
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
	out := runEnglish(t, `struct Point {
	x
	func init(a, b = 2) { self.x = a + b }
	func go(n) { self.x = n }
}
try { Point(1, 2, 3) } catch e { print(e) }
try { Point(1, z: 2) } catch e { print(e) }
try { Point(1).go(1, 2) } catch e { print(e) }
try { Point(1).go() } catch e { print(e) }
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
	out := runEnglish(t, `var counter = 1
try { prnt(1) } catch e { print(e) }
try { print(countr) } catch e { print(e) }
`, nil)
	for _, want := range []string{"Did you mean 'TR_print'?", "Did you mean 'counter'?"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q yok:\n%s", want, out)
		}
	}
}

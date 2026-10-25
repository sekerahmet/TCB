package interp

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Değerin metni işlemin yazıldığı dosyanın dilindedir (3, 17.1; sözcüksel kural): metin(),
// yazdır, metin yerleştirmesi ve tip() adları o dosyanın dilinde çıkar. Kitaplık (Türkçe)
// fonksiyonu kendi dilinde, geri çağırma kendi dosyasının dilinde çevirir. Inspect kanonik
// kalır (JSON yazıcısı ve günlük onu kullanır).

// runEnglish İngilizce yazılmış main metnini kodlayıp (dil=en) programın kendisi gibi çalıştırır;
// files çalışma klasörüne yazılan (Türkçe) modüllerdir.
func runEnglish(t *testing.T, main string, files map[string]string) string {
	t.Helper()
	en, err := codes.LoadLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	coded, err := source.EncodeIn(main, en)
	if err != nil {
		t.Fatalf("İngilizce metin kodlanamadı: %v", err)
	}
	root := t.TempDir()
	runInDir(t, root)
	for name, content := range files {
		testkit.WritePath(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	r := source.Prepare(coded, source.Options{})
	if r.Stage != "" || r.Language != "en" {
		t.Fatalf("hazırlık: dil %q, %v", r.Language, r.Errors)
	}
	var out bytes.Buffer
	m := New()
	m.Out, m.Err, m.Stdin = &out, &out, strings.NewReader("\n")
	m.SourceFile, m.SourceCode, m.SourceLanguage = filepath.Join(root, "main.tr"), r.Text, r.Language
	if _, err := m.Run(r.Program); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return out.String()
}

func TestValueTextFollowsTheFilesLanguage(t *testing.T) {
	out := runEnglish(t, `var b = true
print(b, none, type(1), [true, none, {k: false}], {false: 1})
print("{b} {none} {type(b)} {[type([])]}")
print(str(b) == "true", str(none) == "none", str(type(1.5)) == "Float")
`, nil)
	want := "true none Int [true, none, {\"k\": false}] {false: 1}\n" +
		"true none Bool [List]\n" +
		"true true true\n"
	if out != want {
		t.Errorf("İngilizce dosyada değerler İngilizce yazılmalı:\n%s\nbeklenen:\n%s", out, want)
	}
	// Anlam kanonik kalır: Inspect (veri biçimleri) Türkçedir.
	if True.Inspect() != "doğru" || Null.Inspect() != "yok" || TypeOfValue(NewInt(1)).Inspect() != "Tam" {
		t.Errorf("Inspect kanonik kalmalı: %s %s %s", True.Inspect(), Null.Inspect(), TypeOfValue(NewInt(1)).Inspect())
	}
}

// Türkçe modülün fonksiyonu İngilizce programdan çağrılınca metni Türkçedir (modülün dili);
// modüle verilen İngilizce geri çağırma, modülden çağrılınca İngilizce çevirir (kendi dosyası).
func TestValueTextIsLexical(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(
		"fonksiyon anlat(x) { sonuç \"{x} \" + metin(tip(x)) }\n" +
			"fonksiyon uygula(f) { yazdır(f(doğru)) }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
import "yerel" as y
print(k.anlat(true), y.anlat(none))
k.uygula((x) => "{x} " + str(x))
y.uygula((x) => str([x]))
`, map[string]string{"yerel.tr": "fonksiyon anlat(x) { sonuç metin(x) }\nfonksiyon uygula(f) { yazdır(f(yok)) }\n"})
	want := "doğru Mantık yok\ntrue true\n[none]\n"
	if out != want {
		t.Errorf("metin işlemin yazıldığı dosyanın dilinde olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Programa geri dönen hata metni de program metnidir: fırlatılan değer ve doğrula'nın mesaj
// değeri fırlatıldığı dosyanın dilinde çevrilir (iletinin çerçevesi görüntüdür, TR_DIL).
func TestThrownValueTextFollowsTheFilesLanguage(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte("fonksiyon at() { fırlat [doğru, yok] }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
try { throw true } catch e { print(e, e == "true") }
try { throw [none, Int] } catch e { print(e) }
try { k.at() } catch e { print(e) }
try { assert(false, none) } catch e { print(e.ends_with(": none")) }
`, nil)
	want := "true true\n[none, Int]\n[doğru, yok]\ntrue\n"
	if out != want {
		t.Errorf("yakalanan metin fırlatıldığı dosyanın dilinde olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// ondalık() sonsuz ve tanımsız sayının dosyanın dilindeki adını okur ("inf", "nan" her dilde
// okunur); metin(ondalık(...)) aynı adı verir, gidiş dönüş tutar (3.2).
func TestFloatValueNamesRoundTrip(t *testing.T) {
	out := runEnglish(t, `print(float("inf"), -float("inf"), float("nan"), str(float("-inf")) == "-inf", float(str(float("inf"))))
try { float("sonsuz") } catch e { print("not read") }
`, nil)
	if want := "inf -inf nan true inf\nnot read\n"; out != want {
		t.Errorf("İngilizce dosyada değer adları:\n%s\nbeklenen:\n%s", out, want)
	}
	_, val, err := runTROut(t, `sonuç [ondalık("sonsuz"), ondalık("-sonsuz"), ondalık("+sonsuz"), ondalık("inf"), metin(ondalık("NaN")), ondalık(metin(ondalık("-inf")))]`)
	if err != nil || val.Inspect() != `[sonsuz, -sonsuz, sonsuz, sonsuz, "NaN", -sonsuz]` {
		t.Errorf("Türkçe dosyada değer adları: %v, %v", val, err)
	}
}

// Yakalanan çalışma anı hatası programın Metin değeridir: ileti onu yakalayan dosyanın dilinde
// kurulur, içindeki tip ve değerlerle (DENETİM 3. adım, karar 4). Sınama süreci Türkçedir
// (TR_DIL=tr); İngilizce dosyanın yakaladığı hata yine İngilizcedir, Türkçe kitaplıkta oluşsa da.
func TestCaughtErrorTextFollowsTheCatchingFile(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte("fonksiyon taşır() { sonuç [1][5] }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
try { [1].join(true) } catch e { print(e) }
try { k.taşır() } catch e { print(e) }
`, nil)
	for _, want := range []string{"Line 2:", "expects String as argument 1, Bool (true) given", "List index out of range: 5 (length 1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q yok:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Satır") || strings.Contains(out, "Mantık") {
		t.Errorf("İngilizce dosyanın yakaladığı hata Türkçe:\n%s", out)
	}
}

// Kod taşıyan çıplak sözlük anahtarı ({step: 1}, Y500) işlemin yazıldığı dosyanın dilindeki
// adıdır; d.ad okuması ve yazması anahtarı kodun her yazım dilindeki adıyla arar (önce dosyanın
// dilindeki), yoksa dosyanın dilindeki adla ekler: aynı kodun iki anahtarı oluşmaz (3.7, 17.3).
// Metin anahtar (d["adım"]) tam metindir; kodsuz çıplak anahtar olduğu gibi kalır.
func TestDictMemberKeysFollowTheFilesLanguage(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(
		"fonksiyon artır(ayar) {\n    ayar.adım += 1\n    ayar.öğrenme_oranı = 0.1\n    sonuç ayar.adım\n}\n" +
			"fonksiyon yeni() { sonuç {adım: 7, renk: 1} }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
var cfg = {step: 1, renk: "mavi"}
print(k.artır(cfg), cfg.keys(), cfg.lr, cfg["adım"], cfg["step"])
cfg.lr = 0.2
var t = k.yeni()
t.step = 8
print(cfg, t, t.step)
var both = {step: 1}
both["adım"] = 2
both.step += 10
print(both)
var d = {step: 1, foo: 2}
print(d.keys(), d["step"], d.step)
`, nil)
	want := "2 [\"step\", \"renk\", \"öğrenme_oranı\"] 0.1 none 2\n" +
		"{\"step\": 2, \"renk\": \"mavi\", \"öğrenme_oranı\": 0.2} {\"adım\": 8, \"renk\": 1} 8\n" +
		"{\"step\": 11, \"adım\": 2}\n" + // iki dildeki ad birden varsa dosyanın dilindeki seçilir
		"[\"step\", \"foo\"] 1 1\n" // DENETİM 5: metin anahtar ve keys() dosyanın dilindeki adı görür
	if out != want {
		t.Errorf("sözlük anahtarları dosyanın dilinde, üye erişimi koda göre olmalı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Programın değerden metin yaptığı her yer aynı kuralı izler: Metin ile '+', birleştir,
// dosya_yaz'ın içeriği, girdi'nin istemi. Kitaplığın içindeki birleştirme Türkçe kalır.
func TestEveryTextConversionFollowsTheFilesLanguage(t *testing.T) {
	withLibrary(t, fstest.MapFS{"kitap.tr": {Data: []byte(
		"fonksiyon ekle(x) { sonuç \"a\" + x + \",\" + [x, yok].birleştir(\"/\") }\n")}})
	out := runEnglish(t, `import "tr/kitap" as k
print("x" + true, none + "!", [true, none, Int].join(","), k.ekle(false))
write_file("out.txt", [false])
print(read_file("out.txt"))
input(true)
`, nil)
	want := "xtrue none! true,none,Int ayanlış,yanlış/yok\n[false]\ntrue"
	if out != want {
		t.Errorf("değerin metni işlemin yazıldığı dosyanın dilinde olmalı:\n%q\nbeklenen:\n%q", out, want)
	}
}

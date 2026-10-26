package formats

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Testlerdeki VM'ler bu paketin yerleşiklerini de içersin (github.com/sekerahmet/TCB/internal/stdlib gibi).
func init() {
	interp.AddEngine(Register)
}

// run programı çalıştırır; yazdır() çıktısını ve son değeri döndürür.
func run(t *testing.T, code string) (string, interp.Value, error) {
	t.Helper()
	o := trrun.Exec(t, code)
	return o.Out, o.Value, o.Err
}

// json_çöz sözlük anahtarlarının sırasını korumuyordu (Go map'i sırasızdır).
func TestJSONDecodePreservesKeyOrder(t *testing.T) {
	_, val, err := run(t, `
		değişken d = json_çöz("\{\"zeta\": 1, \"alfa\": 2.5, \"orta\": [true, null, \"x\"], \"iç\": \{\"b\": 1, \"a\": 2\}\}")
		sonuç [d.anahtarlar(), d["iç"].anahtarlar(), d["orta"], d["alfa"], json_metin(d)]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[["zeta", "alfa", "orta", "iç"], ["b", "a"], [doğru, yok, "x"], 2.5, `
	if !strings.HasPrefix(val.Inspect(), want) {
		t.Fatalf("%s… bekleniyordu, %s alındı", want, val.Inspect())
	}
}

// d[1] ile d["1"] eskiden Inspect() ile aynı anahtara düşüyordu.
func TestDictKeysAreTypeAware(t *testing.T) {
	_, val, err := run(t, `
		değişken d = {1: "tam", "1": "metin"}
		d[doğru] = "mantık"
		d["doğru"] = "metin2"
		sonuç [d[1], d["1"], d[doğru], d["doğru"], d.uzunluk, d.içerir(1), d.içerir("2")]
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `["tam", "metin", "mantık", "metin2", 4, doğru, yanlış]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}

	// Döngü ve anahtarlar() özgün anahtar tiplerini geri vermeli.
	_, val, err = run(t, `
		değişken d = {1: "a", "b": 2}
		değişken tipler = []
		döngü k, v içinde d { tipler = tipler + [tip(k)] }
		sonuç [tipler, d.anahtarlar(), [tip(k) döngü k içinde d]]
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `[[Tam, Metin], [1, "b"], [Tam, Metin]]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}

	// Yazdırma ve JSON dönüşümü metin anahtarlar için değişmemeli.
	out, _, err := run(t, `
		değişken d = {"ad": "Ali", 1: "bir"}
		yazdır(d)
		yazdır(json_metin({"ad": "Ali"}))
		değişken j = json_çöz("\{\"x\": 1\}")
		yazdır(j["x"])
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := "{\"ad\": \"Ali\", 1: \"bir\"}\n{\"ad\":\"Ali\"}\n1\n"
	if out != want {
		t.Fatalf("çıktı %q bekleniyordu, %q alındı", want, out)
	}

	// Metin olmayan anahtarlar Go/JSON tarafına görünen hâlleriyle aktarılır.
	d := interp.NewDictVal()
	d.SetValue(&interp.IntVal{Val: 7}, &interp.StringVal{Val: "yedi"})
	if b, err := jsonwrite.Write(d); err != nil || string(b) != `{"7":"yedi"}` {
		t.Fatalf("JSON anahtarı \"7\" olmalı, alınan: %s %v", b, err)
	}
}

// csv_oku yalnızca dosya okur; CSV metni csv_çöz ile ayrıştırılır (json_çöz gibi). Eskiden
// csv_oku metni de alıyordu ve bir metnin var olan dosyanın adı olup olmadığı belirsizdi.
func TestCSVReadFileAndParseText(t *testing.T) {
	dir := t.TempDir()
	file := filepath.ToSlash(filepath.Join(dir, "veri.csv"))
	if err := os.WriteFile(file, []byte("ad, yaş\nAli, 30\nAyşe, 27.5\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, val, err := run(t, fmt.Sprintf(`sonuç [csv_oku(%q), csv_çöz("x,1\ny,2"), csv_çöz("tek"), csv_çöz("tarih,1/2/2024"), csv_çöz("")]`, file))
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := `[[["ad", "yaş"], ["Ali", 30], ["Ayşe", 27.5]], [["x", 1], ["y", 2]], [["tek"]], [["tarih", "1/2/2024"]], []]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}
	// csv_çöz dosya okumaz: var olan dosyanın yolu da tek hücreli bir tablodur.
	_, val, err = run(t, fmt.Sprintf(`sonuç csv_çöz(%q)`, file))
	if err != nil || val.Inspect() != fmt.Sprintf(`[[%q]]`, file) {
		t.Fatalf("csv_çöz yolu metin saymalı: %v %v", val, err)
	}
}

// csv_oku'ya verilen CSV metni sessizce ayrıştırılmaz: satır sonlu metinde ve virgüllü ama
// var olmayan "yolda" hata csv_çöz'ü önerir; öteki yoksa "dosya yok" hatasıdır (kod 0).
func TestCSVReadRejectsText(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []struct {
		arg  string
		want messages.Code
	}{
		{"x,1\ny,2", messages.H7006},
		{"a\r\nb", messages.H7006},
		{"tarih,1/2/2024", messages.H7007},
		{"yok.csv", 0},
		{"VERI.CSV", 0},
		{"tek", 0},
		{filepath.ToSlash(filepath.Join(dir, "yok", "veri.txt")), 0},
		{"a\tb.csv", messages.H2489},
	} {
		_, val, err := run(t, fmt.Sprintf(`sonuç csv_oku(%q)`, d.arg))
		ok := messages.Has(err, d.want)
		if d.want == 0 {
			ok = errors.Is(err, fs.ErrNotExist) && !messages.Has(err, messages.H7007)
		}
		if !ok {
			t.Errorf("csv_oku(%q): %s bekleniyordu, alınan: %v, %v", d.arg, d.want, val, err)
		}
	}
	// Klasör yolu da sessizce metin sayılmamalı.
	_, _, err := run(t, fmt.Sprintf(`sonuç csv_oku(%q)`, filepath.ToSlash(dir)))
	if !messages.Has(err, messages.H7003) {
		t.Fatalf("klasör hatası (H7003) bekleniyordu, alınan: %v", err)
	}
}

// Göreli yol çalışma klasöründe yoksa programın klasörüne göre çözülür.
func TestCSVResolvesRelativeToProgram(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "yerel_tablo.csv"), []byte("a,b\n1,2"), 0644); err != nil {
		t.Fatal(err)
	}
	p := parser.New(source.TextLexer(`sonuç csv_oku("yerel_tablo.csv")`))
	prog := p.ParseProgram()
	machine := interp.New()
	machine.SourceFile = filepath.Join(dir, "program.tr")
	val, err := machine.Run(prog)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `[["a", "b"], [1, 2]]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}
}

// Hakem bulgusu 15: CSV'de BOM soyulmuyor, "NaN"/"Inf" sayıya çevriliyor, baştaki sıfırlar
// ve Tam aralığını aşan sayılar sessizce bozuluyordu. Artık yalnızca katı sayı biçimi sayıdır;
// öteki hücreler olduğu gibi metin kalır.
func TestCSVStrictNumbersAndBOM(t *testing.T) {
	dir := t.TempDir()
	file := filepath.ToSlash(filepath.Join(dir, "bom.csv"))
	if err := os.WriteFile(file, []byte("\ufeffad,deger\nx,1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, val, err := run(t, fmt.Sprintf(`sonuç [csv_oku(%q)[0][0] == "ad", csv_çöz("%sa,b")[0][0]]`, file, "\ufeff"))
	if err != nil || val.Inspect() != `[doğru, "a"]` {
		t.Fatalf("BOM soyulmalı: %v %v", val, err)
	}
	_, val, err = run(t, `
		değişken s = csv_çöz("a,b,c,d,e,f,g,h,i,j,k,l\nNaN,Inf,-Inf,infinity,007,12345678901234567890,0x10,1_000,1e400,+5,00.5,.5\n-0,0,1.0,2.5e3,-7,9223372036854775807,1E-3,0.0,,  42 ,1.,1e")
		sonuç [s[1], [tip(h) döngü h içinde s[1]], s[2], [tip(h) döngü h içinde s[2]]]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[["NaN", "Inf", "-Inf", "infinity", "007", "12345678901234567890", "0x10", "1_000", "1e400", "+5", "00.5", ".5"], ` +
		`[Metin, Metin, Metin, Metin, Metin, Metin, Metin, Metin, Metin, Metin, Metin, Metin], ` +
		`[0, 0, 1.0, 2500.0, -7, 9223372036854775807, 0.001, 0.0, "", 42, "1.", "1e"], ` +
		`[Tam, Tam, Ondalık, Ondalık, Tam, Tam, Ondalık, Ondalık, Metin, Tam, Metin, Metin]]`
	if val.Inspect() != want {
		t.Fatalf("\n%s\nbeklenen\n%s", val.Inspect(), want)
	}
}

// Hakem bulgusu 15 ve 5: json_metin anahtar sırasını kaybediyordu, NaN'da İngilizce hata
// veriyordu, döngüsel listede süreç yığın taşmasıyla çöküyordu (dene yakalayamıyordu).
func TestJSONTextOrderNaNAndCycle(t *testing.T) {
	_, val, err := run(t, `sonuç json_metin(json_çöz("\{\"b\": 1, \"a\": \{\"z\": 2, \"y\": [3]\}\}"))`)
	if err != nil || val.Inspect() != `{"b":1,"a":{"z":2,"y":[3]}}` {
		t.Fatalf("sıra korunmalı: %v %v", val, err)
	}
	_, val, err = run(t, `
		değişken l = [1]
		l.ekle(l)
		değişken d = {"a": 1}
		d["ben"] = [d]
		değişken mesajlar = []
		dene { json_metin(l) } yakala h { mesajlar.ekle("{h}") }
		dene { json_metin(d) } yakala h { mesajlar.ekle("{h}") }
		dene { json_metin({"x": karekök(-1.0)}) } yakala h { mesajlar.ekle("{h}") }
		dene { json_metin([1e308 * 10.0]) } yakala h { mesajlar.ekle("{h}") }
		sonuç mesajlar`)
	if err != nil {
		t.Fatal(err)
	}
	m := val.Inspect()
	if strings.Count(m, "kendini içeriyor") != 2 || strings.Count(m, "JSON'a yazılamaz") != 4 || strings.Contains(m, "unsupported") {
		t.Fatalf("Türkçe döngü ve NaN hataları bekleniyordu: %s", m)
	}
	// BOM'lu JSON metni de çözülür
	_, val, err = run(t, fmt.Sprintf(`sonuç json_çöz("%s\{\"a\": 1\}")["a"]`, "\ufeff"))
	if err != nil || val.Inspect() != "1" {
		t.Fatalf("BOM'lu JSON: %v %v", val, err)
	}
}

// json_çöz, csv_çöz ve csv_oku metin olmayan argümanı görünüşüyle ayrıştırmaz, açık hata verir
// (json_çöz(5) eskiden 5'i, json_çöz([1]) listenin TR yazımını çözüyordu).
func TestDecodersRequireText(t *testing.T) {
	for _, d := range []struct {
		code string
		want messages.Code
	}{{`json_çöz(5)`, messages.H7000}, {`json_çöz([1, 2])`, messages.H7000}, {`csv_oku(5)`, messages.H2423}, {`csv_oku([1])`, messages.H2423}, {`csv_çöz(5)`, messages.H2423}} {
		if _, _, err := run(t, d.code); !messages.Has(err, d.want) {
			t.Fatalf("%s: %s beklenirdi: %v", d.code, d.want, err)
		}
	}
	out, _, err := run(t, `yazdır(json_çöz("[1, 2]"), csv_çöz("a,b\n1,2"))`)
	if err != nil || out != "[1, 2] [[\"a\", \"b\"], [1, 2]]\n" {
		t.Fatalf("metin argümanı: %q, %v", out, err)
	}
}

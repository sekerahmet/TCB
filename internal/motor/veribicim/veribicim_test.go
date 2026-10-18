package veribicim

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tr_dili/internal/dil/lexer"
	"tr_dili/internal/dil/parser"
	"tr_dili/internal/dil/vm"
)

// Testlerdeki VM'ler bu paketin yerleşiklerini de içersin (tr_dili/internal/motor gibi).
func init() {
	vm.MotorEkle(Kaydet)
}

// calistir programı çalıştırır; yazdır() çıktısını ve son değeri döndürür.
func calistir(t *testing.T, code string) (string, vm.Value, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	var out bytes.Buffer
	machine := vm.New()
	machine.Out = &out
	val, err := machine.Run(prog)
	machine.WaitAsync()
	return out.String(), val, err
}

// json_çöz sözlük anahtarlarının sırasını korumuyordu (Go map'i sırasızdır).
func TestJSONDecodePreservesKeyOrder(t *testing.T) {
	_, val, err := calistir(t, `
		değişken d = json_çöz("{\"zeta\": 1, \"alfa\": 2.5, \"orta\": [true, null, \"x\"], \"iç\": {\"b\": 1, \"a\": 2}}")
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
	_, val, err := calistir(t, `
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
	_, val, err = calistir(t, `
		değişken d = {1: "a", "b": 2}
		değişken tipler = []
		döngü k, v içinde d { tipler = tipler + [tip(k)] }
		sonuç [tipler, d.anahtarlar(), [tip(k) döngü k içinde d]]
	`)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `[["Tam", "Metin"], [1, "b"], ["Tam", "Metin"]]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}

	// Yazdırma ve JSON dönüşümü metin anahtarlar için değişmemeli.
	out, _, err := calistir(t, `
		değişken d = {"ad": "Ali", 1: "bir"}
		yazdır(d)
		yazdır(json_metin({"ad": "Ali"}))
		değişken j = json_çöz("{\"x\": 1}")
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
	d := vm.NewDictVal()
	d.SetValue(&vm.IntVal{Val: 7}, &vm.StringVal{Val: "yedi"})
	m := vm.TRToGoValue(d).(map[string]interface{})
	if m["7"] != "yedi" {
		t.Fatalf("TRToGoValue anahtarı \"7\" olmalı, alınan: %v", m)
	}
}

// csv_oku var olan dosyayı okur, metin içeriğini ayrıştırır; yol gibi görünen ama var
// olmayan bir argümanda eskiden [["yok.csv"]] döndürürken artık açık hata verir.
func TestCSVReadFileTextAndMissingPath(t *testing.T) {
	dir := t.TempDir()
	dosya := filepath.ToSlash(filepath.Join(dir, "veri.csv"))
	if err := os.WriteFile(dosya, []byte("ad, yaş\nAli, 30\nAyşe, 27.5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, val, err := calistir(t, fmt.Sprintf(`sonuç [csv_oku(%q), csv_oku("x,1\ny,2"), csv_oku("tek"), csv_oku("tarih,1/2/2024")]`, dosya))
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	want := `[[["ad", "yaş"], ["Ali", 30], ["Ayşe", 27.5]], [["x", 1], ["y", 2]], [["tek"]], [["tarih", "1/2/2024"]]]`
	if val.Inspect() != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, val.Inspect())
	}

	for _, arg := range []string{
		"yok.csv",
		"VERI.CSV",
		filepath.ToSlash(filepath.Join(dir, "yok", "veri.txt")),
		`klasor\yok`,
	} {
		_, val, err := calistir(t, fmt.Sprintf(`sonuç csv_oku(%q)`, arg))
		if err == nil || !strings.Contains(err.Error(), "okunamadı: ") || !strings.Contains(err.Error(), " yok") {
			t.Errorf("csv_oku(%q): 'dosya yok' hatası bekleniyordu, alınan: %v, %v", arg, val, err)
		}
	}

	// Klasör yolu da sessizce metin sayılmamalı.
	_, _, err = calistir(t, fmt.Sprintf(`sonuç csv_oku(%q)`, filepath.ToSlash(dir)))
	if err == nil || !strings.Contains(err.Error(), "klasör") {
		t.Fatalf("klasör hatası bekleniyordu, alınan: %v", err)
	}
}

// Göreli yol çalışma klasöründe yoksa programın klasörüne göre çözülür.
func TestCSVResolvesRelativeToProgram(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "yerel_tablo.csv"), []byte("a,b\n1,2"), 0644); err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(`sonuç csv_oku("yerel_tablo.csv")`))
	prog := p.ParseProgram()
	machine := vm.New()
	machine.SourceFile = filepath.Join(dir, "program.tr")
	val, err := machine.Run(prog)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if val.Inspect() != `[["a", "b"], [1, 2]]` {
		t.Fatalf("beklenmeyen sonuç: %s", val.Inspect())
	}
}

func TestYolGibi(t *testing.T) {
	for s, want := range map[string]bool{
		"yok.csv":          true,
		"a/b":              true,
		`a\b`:              true,
		"a,b":              false,
		"tarih,1/2/2024":   false,
		"a,b\nc,d.csv":     false,
		"":                 false,
		"sadece_bir_metin": false,
	} {
		if got := yolGibi(s); got != want {
			t.Errorf("yolGibi(%q) = %v, %v bekleniyordu", s, got, want)
		}
	}
}

// Hakem bulgusu 15: CSV'de BOM soyulmuyor, "NaN"/"Inf" sayıya çevriliyor, baştaki sıfırlar
// ve Tam aralığını aşan sayılar sessizce bozuluyordu. Artık yalnızca katı sayı biçimi sayıdır;
// öteki hücreler olduğu gibi metin kalır.
func TestCSVKatiSayiVeBOM(t *testing.T) {
	dir := t.TempDir()
	dosya := filepath.ToSlash(filepath.Join(dir, "bom.csv"))
	if err := os.WriteFile(dosya, []byte("\ufeffad,deger\nx,1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, val, err := calistir(t, fmt.Sprintf(`sonuç [csv_oku(%q)[0][0] == "ad", csv_oku("%sa,b")[0][0]]`, dosya, "\ufeff"))
	if err != nil || val.Inspect() != `[doğru, "a"]` {
		t.Fatalf("BOM soyulmalı: %v %v", val, err)
	}
	_, val, err = calistir(t, `
		değişken s = csv_oku("a,b,c,d,e,f,g,h,i,j,k,l\nNaN,Inf,-Inf,infinity,007,12345678901234567890,0x10,1_000,1e400,+5,00.5,.5\n-0,0,1.0,2.5e3,-7,9223372036854775807,1E-3,0.0,,  42 ,1.,1e")
		sonuç [s[1], [tip(h) döngü h içinde s[1]], s[2], [tip(h) döngü h içinde s[2]]]`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[["NaN", "Inf", "-Inf", "infinity", "007", "12345678901234567890", "0x10", "1_000", "1e400", "+5", "00.5", ".5"], ` +
		`["Metin", "Metin", "Metin", "Metin", "Metin", "Metin", "Metin", "Metin", "Metin", "Metin", "Metin", "Metin"], ` +
		`[0, 0, 1.0, 2500.0, -7, 9223372036854775807, 0.001, 0.0, "", 42, "1.", "1e"], ` +
		`["Tam", "Tam", "Ondalık", "Ondalık", "Tam", "Tam", "Ondalık", "Ondalık", "Metin", "Tam", "Metin", "Metin"]]`
	if val.Inspect() != want {
		t.Fatalf("\n%s\nbeklenen\n%s", val.Inspect(), want)
	}
}

// Hakem bulgusu 15 ve 5: json_metin anahtar sırasını kaybediyordu, NaN'da İngilizce hata
// veriyordu, döngüsel listede süreç yığın taşmasıyla çöküyordu (dene yakalayamıyordu).
func TestJSONMetinSiraNaNVeDongu(t *testing.T) {
	_, val, err := calistir(t, `sonuç json_metin(json_çöz("{\"b\": 1, \"a\": {\"z\": 2, \"y\": [3]}}"))`)
	if err != nil || val.Inspect() != `{"b":1,"a":{"z":2,"y":[3]}}` {
		t.Fatalf("sıra korunmalı: %v %v", val, err)
	}
	_, val, err = calistir(t, `
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
	_, val, err = calistir(t, fmt.Sprintf(`sonuç json_çöz("%s{\"a\": 1}")["a"]`, "\ufeff"))
	if err != nil || val.Inspect() != "1" {
		t.Fatalf("BOM'lu JSON: %v %v", val, err)
	}
}

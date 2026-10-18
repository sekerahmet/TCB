package veritabani

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"tr_dili/pkg/vm"
)

// Veritabanı dosyası da TR'nin tek yol kuralını izler; dosya hataları Türkçedir.
func TestVeritabaniYolKuraliVeTurkceHata(t *testing.T) {
	_, err := runTR(t, "değişken vt = veritabanı_aç(\"olmayan_klasor_vt/a.trdb\")\nvt.kaydet()")
	if err == nil || !strings.HasSuffix(err.Error(), `veritabanı: "olmayan_klasor_vt/a.trdb" kaydedilemedi: "olmayan_klasor_vt" klasörü yok`) {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	bozuk := filepath.Join(t.TempDir(), "bozuk.trdb")
	if err := os.WriteFile(bozuk, []byte("{bozuk"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = runTR(t, fmt.Sprintf("veritabanı_aç(%q)", filepath.ToSlash(bozuk)))
	if err == nil || !strings.Contains(err.Error(), "geçerli bir TR veritabanı (JSON) değil") {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
}

func TestDatabaseFileContainingNullLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bos.trdb")
	if err := os.WriteFile(path, []byte("null"), 0644); err != nil {
		t.Fatal(err)
	}
	_, val, err := runTRWithin(t, 5*time.Second, fmt.Sprintf(`
		değişken vt = veritabanı_aç(%q)
		vt.ekle("t", {"ad": "x"})
		sonuç vt.say("t")`, filepath.ToSlash(path)))
	if err != nil {
		t.Fatalf("'null' içerikli veritabanı açılamadı: %v", err)
	}
	if val.Inspect() != "1" {
		t.Fatalf("1 bekleniyordu, %s alındı", val.Inspect())
	}
}

// ---------------------------------------------------------------------------
// NEREDE koşulları artık TR kodu olarak çalıştırılmamalı
// ---------------------------------------------------------------------------

func testRows() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": int64(1), "ad": "Ayşe", "departman": "Yapay Zeka", "maas": int64(92000)},
		{"id": int64(2), "ad": "Ali", "departman": "Satış", "maas": int64(60000)},
		{"id": int64(3), "ad": "İrem", "departman": "Yapay Zeka", "maas": 75000.5},
	}
}

func knownCols() map[string]bool {
	return map[string]bool{"id": true, "ad": true, "departman": true, "maas": true}
}

func TestRowConditionBasics(t *testing.T) {
	cases := []struct {
		cond string
		want int
	}{
		{"maas >= 80000", 1},
		{"departman = 'Yapay Zeka' VE maas > 70000", 2},
		{"departman == \"Satış\" veya id == 1", 2},
		{"değil (id <> 2)", 1},
		{"DEĞİL id = 2 && maas < 100000", 2},
		{"(id > 1) == DOĞRU veya ad == yok", 2},
		{"maas * 2 > 150000", 2},
		{"ad == 'O''Brien'", 0},
		{"(id == 1 || id == 3) ve maas >= 75000.5", 2},
	}
	for _, c := range cases {
		matched, _, err := filterRows(testRows(), c.cond, knownCols(), nil)
		if err != nil {
			t.Fatalf("%q: beklenmeyen hata: %v", c.cond, err)
		}
		if len(matched) != c.want {
			t.Errorf("%q: %d satır bekleniyordu, %d alındı", c.cond, c.want, len(matched))
		}
	}
}

// Koşul sözcükleri yalnızca Türkçedir: İngilizce ve ASCII yazımlar hata verir, Türkçe
// karşılığı önerilir; o adda bir sütun varsa sütundur.
func TestRowConditionYalnizcaTurkce(t *testing.T) {
	for cond, want := range map[string]string{
		"maas > 1 AND id = 2":    "'AND' TR'de yok; 'VE' yazmak mı istediniz?",
		"maas > 1 or id = 2":     "'or' TR'de yok; 'VEYA' yazmak mı istediniz?",
		"NOT id = 2":             "'NOT' TR'de yok; 'DEĞİL' yazmak mı istediniz?",
		"degil id = 2":           "'degil' TR'de yok; 'DEĞİL' yazmak mı istediniz?",
		"ad = TRUE":              "'TRUE' TR'de yok; 'DOĞRU' yazmak mı istediniz?",
		"ad = dogru":             "'dogru' TR'de yok; 'DOĞRU' yazmak mı istediniz?",
		"ad = YANLIS":            "'YANLIS' TR'de yok; 'YANLIŞ' yazmak mı istediniz?",
		"ad = null":              "'null' TR'de yok; 'YOK' yazmak mı istediniz?",
		"maas > 1 ORDER BY maas": "'ORDER' TR'de yok; 'SIRALA' yazmak mı istediniz?",
	} {
		if _, _, err := filterRows(testRows(), cond, knownCols(), nil); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", cond, want, err)
		}
	}
	bilinen := map[string]bool{"not": true, "id": true}
	satirlar := []map[string]interface{}{{"id": int64(1), "not": int64(90)}, {"id": int64(2), "not": int64(40)}}
	if m, _, err := filterRows(satirlar, "not > 50", bilinen, nil); err != nil || len(m) != 1 {
		t.Errorf("'not' adlı sütun kullanılabilmeli: %v %d", err, len(m))
	}
}

func TestRowConditionRejectsCodeExecution(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pwned.txt")
	// Eski enterpolasyon saldırısı: metin içindeki süslü parantezler artık düz metindir
	interp := fmt.Sprintf(`ad == '{dosya_yaz(%q, "RCE")}'`, marker)
	if matched, _, err := filterRows(testRows(), interp, knownCols(), nil); err != nil || len(matched) != 0 {
		t.Errorf("%q: 0 eşleşme ve hatasız sonuç bekleniyordu (err=%v, n=%d)", interp, err, len(matched))
	}
	// Fonksiyon çağrısı ve üye erişimi tamamen reddedilmeli
	for _, cond := range []string{
		fmt.Sprintf(`dosya_yaz(%q, "RCE")`, marker),
		`web_gönder("http://127.0.0.1:1/", "veri")`,
		`ad.uzunluk > 0`,
	} {
		if _, _, err := filterRows(testRows(), cond, knownCols(), nil); err == nil {
			t.Errorf("%q: güvenli olmayan koşul reddedilmeliydi", cond)
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("koşul içinden dosya_yaz çalıştı: kod yürütme açığı hâlâ var")
	}
}

func TestRowConditionParamsPreventInjection(t *testing.T) {
	evil := &vm.StringVal{Val: "x' veya doğru veya 'y"}
	matched, _, err := filterRows(testRows(), "departman == ?", knownCols(), []vm.Value{evil})
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 0 {
		t.Fatalf("parametre enjeksiyonu %d satır döndürdü, 0 bekleniyordu", len(matched))
	}
	matched, _, err = filterRows(testRows(), "departman == ? ve maas > ?", knownCols(),
		[]vm.Value{&vm.StringVal{Val: "Yapay Zeka"}, &vm.IntVal{Val: 80000}})
	if err != nil || len(matched) != 1 {
		t.Fatalf("parametreli sorgu 1 satır döndürmeliydi (err=%v, n=%d)", err, len(matched))
	}
	if _, _, err := filterRows(testRows(), "id == ? ve id == ?", knownCols(), []vm.Value{&vm.IntVal{Val: 1}}); err == nil {
		t.Fatal("eksik parametre hata vermeliydi")
	}
}

func TestRowConditionErrorsAreReported(t *testing.T) {
	for _, cond := range []string{"maaas > 10", "maas >", "ad == 'kapanmamış", "ad > 5"} {
		if _, _, err := filterRows(testRows(), cond, knownCols(), nil); err == nil {
			t.Errorf("%q: hata bekleniyordu", cond)
		}
	}
}

func TestSQLParamsFromTR(t *testing.T) {
	val, err := runTR(t, `
		değişken vt = veritabanı_aç(":bellek:")
		vt.ekle("personel", [
			{"id": 1, "ad": "Ayşe", "departman": "Yapay Zeka", "maas": 92000},
			{"id": 2, "ad": "Ali", "departman": "Satış", "maas": 60000}
		])
		değişken kotu = "x' veya doğru veya 'y"
		değişken a = vt.sorgula("SEÇ ad TABLO personel NEREDE departman == ?", [kotu])
		değişken b = vt.sorgula("SEÇ ad TABLO personel NEREDE departman == ? SIRALA maas AZALAN", ["Satış"])
		değişken c = vt.say("personel", "maas > ?", [70000])
		değişken d = vt.güncelle("personel", {"maas": 61000}, "id == ?", [2])
		değişken e = tablo_sorgula(vt.hepsini_al("personel"), "maas < ?", [70000])
		// Koşul içinde sayma: eskiden kilitlenme (deadlock) yapıyordu
		değişken f = vt.sil("personel", "id == ?", [vt.say("personel")])
		sonuç [a.uzunluk, b[0]["ad"], c, d, e[0]["maas"], f, vt.say("personel")]
	`)
	if err != nil {
		t.Fatalf("SQL hatası: %v", err)
	}
	if got, want := val.Inspect(), `[0, "Ali", 1, 1, 61000, 1, 1]`; got != want {
		t.Fatalf("%s bekleniyordu, %s alındı", want, got)
	}
}

// Sorgu sözcükleri yalnızca Türkçedir: İngilizce ve ASCII yazımlar Türkçe karşılığı öneren
// bir hata verir (sessizce tablo adının ya da koşulun parçası sayılmaz).
func TestSQLYalnizcaTurkce(t *testing.T) {
	kur := `değişken vt = veritabanı_aç(":bellek:")
vt.ekle("p", [{"ad": "a", "maas": 1}])
`
	for sorgu, want := range map[string]string{
		"SELECT * FROM p":                "'SELECT' TR'de yok; 'SEÇ' yazmak mı istediniz?",
		"SEC * TABLO p":                  "'SEC' TR'de yok; 'SEÇ' yazmak mı istediniz?",
		"DELETE TABLO p":                 "'DELETE' TR'de yok; 'SİL' yazmak mı istediniz?",
		"SIL TABLO p":                    "'SIL' TR'de yok; 'SİL' yazmak mı istediniz?",
		"SEÇ * FROM p":                   "'FROM' TR'de yok; 'TABLO' yazmak mı istediniz?",
		"SEÇ * DAN p":                    "'DAN' TR'de yok; 'TABLO' yazmak mı istediniz?",
		"SEÇ * TABLO p WHERE maas > 0":   "'WHERE' TR'de yok; 'NEREDE' yazmak mı istediniz?",
		"SEÇ * TABLO p SIRALA maas DESC": "'DESC' TR'de yok; 'AZALAN' yazmak mı istediniz?",
		"SEÇ * TABLO p LIMIT 1":          "'LIMIT' TR'de yok; 'LİMİT' yazmak mı istediniz?",
		"SEÇ * TABLO p ORDER BY maas":    "'ORDER' TR'de yok; 'SIRALA' yazmak mı istediniz?",
		"SİL TABLO p WHERE maas > 0":     "'WHERE' TR'de yok; 'NEREDE' yazmak mı istediniz?",
	} {
		_, err := runTR(t, kur+"vt.sorgula(\""+sorgu+"\")")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %q içeren hata bekleniyordu: %v", sorgu, want, err)
		}
	}
	for kod, want := range map[string]string{
		`veritabanı_aç(":memory:")`:                   "':memory:' TR'de yok; ':bellek:' yazmak mı istediniz?",
		`tablo_sorgula([{"a": 1}], sırala: "a:desc")`: "'desc' TR'de yok; 'AZALAN' yazmak mı istediniz?",
	} {
		if _, err := runTR(t, kod); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %q içeren hata bekleniyordu: %v", kod, want, err)
		}
	}
}

func TestSQLTurkishCaseAndUTF8Safety(t *testing.T) {
	code := `
		değişken vt = veritabanı_aç(":bellek:")
		vt.ekle("çalışanlar", [
			{"id": 1, "ad": "Işık", "şehir": "İstanbul", "maaş": 90000},
			{"id": 2, "ad": "Ali",  "şehir": "Ankara",   "maaş": 60000},
			{"id": 3, "ad": "İrem", "şehir": "İstanbul", "maaş": 95000}
		])
		// Küçük harfli Türkçe SQL ('ı', 'İ', 'ş')
		değişken s1 = vt.sorgula("seç ad, maaş tablo çalışanlar nerede şehir = 'İstanbul' ve maaş >= 90000 sırala maaş azalan limit 2")
		değişken silinen = vt.sorgula("sil tablo çalışanlar nerede id <> 1 ve id <> 3")
		sonuç [s1.uzunluk, s1[0]["ad"], silinen, vt.say("çalışanlar")]
	`
	val, err := runTR(t, code)
	if err != nil {
		t.Fatalf("SQL hatası: %v", err)
	}
	if val.Inspect() != `[2, "İrem", 1, 2]` {
		t.Fatalf(`[2, "İrem", 1, 2] bekleniyordu, %s alındı`, val.Inspect())
	}
}

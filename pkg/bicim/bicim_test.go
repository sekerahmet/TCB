package bicim

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bicimleT(t *testing.T, kaynak string) string {
	t.Helper()
	cikti, err := Bicimle(kaynak)
	if err != nil {
		t.Fatalf("Bicimle hata verdi: %v\nkaynak:\n%s", err, kaynak)
	}
	return cikti
}

// Gidiş-dönüş: beklenen standart biçim üretilmeli ve ikinci kez biçimlemek bir şey değiştirmemeli.
func TestStandartBicim(t *testing.T) {
	durumlar := []struct{ ad, girdi, beklenen string }{
		{"operatör boşlukları", "değişken   x=1+2*3\n", "değişken x = 1 + 2 * 3\n"},
		{"virgül ve parantez", "yazdır( 1 ,2,  3 )\n", "yazdır(1, 2, 3)\n"},
		{"tekli eksi", "x = -1\ny = a-b\nz = f(-x) * -2\nsonuç -1\n", "x = -1\ny = a - b\nz = f(-x) * -2\nsonuç -1\n"},
		{"değil ve ünlem", "a = !b\nc = değil (d)\n", "a = !b\nc = değil (d)\n"},
		{"girinti 4 boşluk", "eğer x {\n  yazdır(1)\n\t\teğer y {\nyazdır(2)\n}\n}\n",
			"eğer x {\n    yazdır(1)\n    eğer y {\n        yazdır(2)\n    }\n}\n"},
		{"süslü aynı satırda", "fonksiyon f(a)\n{\n    sonuç a\n}\n", "fonksiyon f(a) {\n    sonuç a\n}\n"},
		{"değilse önceki satıra", "eğer x {\n    a()\n}\ndeğilse\n{\n    b()\n}\n", "eğer x {\n    a()\n} değilse {\n    b()\n}\n"},
		{"yakala önceki satıra", "dene {\n    a()\n}\nyakala h {\n    b(h)\n}\n", "dene {\n    a()\n} yakala h {\n    b(h)\n}\n"},
		{"boş satırlar", "\n\n\na = 1\n\n\n\nb = 2\n\n\n", "a = 1\n\nb = 2\n"},
		{"blok başı/sonu boş satır", "eğer x {\n\n    a()\n\n}\n", "eğer x {\n    a()\n}\n"},
		{"satır sonu boşlukları", "a = 1   \nb = 2\t\n", "a = 1\nb = 2\n"},
		{"son satır sonu eklenir", "a = 1", "a = 1\n"},
		{"sözlük", "s = { \"a\" :1,b: {\"c\":2} }\n", "s = {\"a\": 1, b: {\"c\": 2}}\n"},
		{"sondaki blok", "y = l.filtrele{o>1}\n", "y = l.filtrele { o > 1 }\n"},
		{"aralık", "döngü i içinde 1 ..= 10 { yazdır(i) }\n", "döngü i içinde 1..=10 { yazdır(i) }\n"},
		{"dönüş tipi oku", "fonksiyon f(a:Tam)->Tam { sonuç a }\n", "fonksiyon f(a: Tam) -> Tam { sonuç a }\n"},
		{"lambda", "f = (a,b)=>a+b\ng = x=>x*2\n", "f = (a, b) => a + b\ng = x => x * 2\n"},
		{"devam satırı", "t = 1 +\n2\nl\n|> yazdır\n", "t = 1 +\n    2\nl\n    |> yazdır\n"},
		{"çok satırlı liste", "l = [\n1,\n  2\n    ]\n", "l = [\n    1,\n    2\n]\n"},
		{"kapanışla başlayan satır", "f(fonksiyon() {\nyazdır(1)\n}, 2)\n", "f(fonksiyon() {\n    yazdır(1)\n}, 2)\n"},
		{"CRLF", "a = 1\r\neğer a {\r\n  b()\r\n}\r\n", "a = 1\neğer a {\n    b()\n}\n"},
		{"test bloğu", "test \"ad\"{doğrula(1==1)}\n", "test \"ad\" { doğrula(1 == 1) }\n"},
		{"boş kaynak", "", ""},
	}
	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			cikti := bicimleT(t, d.girdi)
			if cikti != d.beklenen {
				t.Fatalf("beklenen:\n%q\nalınan:\n%q", d.beklenen, cikti)
			}
			if ikinci := bicimleT(t, cikti); ikinci != cikti {
				t.Fatalf("idempotent değil:\n%q\n->\n%q", cikti, ikinci)
			}
		})
	}
}

func TestYorumlarKorunur(t *testing.T) {
	girdi := `// dosya başı yorumu
// ikinci yorum
değişken x = 1   // sondaki yorum
/* çok
   satırlı yorum */
eğer x {
        // blok içi yorum
    yazdır(x) /* satır içi */ ; yazdır(2)
    // kapanıştan önce
}
// dosya sonu
`
	cikti := bicimleT(t, girdi)
	for _, y := range []string{"// dosya başı yorumu", "// ikinci yorum", "// sondaki yorum",
		"/* çok\n   satırlı yorum */", "// blok içi yorum", "/* satır içi */", "// kapanıştan önce", "// dosya sonu"} {
		if !strings.Contains(cikti, y) {
			t.Errorf("yorum kayboldu: %q\nçıktı:\n%s", y, cikti)
		}
	}
	if !strings.Contains(cikti, "\n    // blok içi yorum\n") || !strings.Contains(cikti, "\n    // kapanıştan önce\n}") {
		t.Errorf("blok içi yorumlar girintilenmedi:\n%s", cikti)
	}
	if !strings.Contains(cikti, "değişken x = 1 // sondaki yorum\n") {
		t.Errorf("sondaki yorum satırında kalmadı:\n%s", cikti)
	}
	if ikinci := bicimleT(t, cikti); ikinci != cikti {
		t.Errorf("yorumlu kodda idempotent değil:\n%s\n->\n%s", cikti, ikinci)
	}
}

// Yorum, '{' ile önceki satır arasındaysa '{' taşınmaz (yorum koddan sonra kalmasın diye).
func TestYorumluSusluTasinmaz(t *testing.T) {
	girdi := "eğer x // neden\n{\n    a()\n}\n"
	if cikti := bicimleT(t, girdi); cikti != girdi {
		t.Fatalf("beklenmeyen çıktı:\n%q", cikti)
	}
}

func TestMetinlerBaytBaytKorunur(t *testing.T) {
	metinler := []string{
		`"  iki  boşluk  "`, `'tek tırnak'`, `"kaçış \" \n \t \\ \{ \}"`, `"ara {x+1} değer"`,
		`"{p['ad']}"`, "\"çok\nsatırlı   \n  metin\"",
	}
	for _, m := range metinler {
		girdi := "değişken x = 1\ndeğişken p = {}\ndeğişken s = " + m + "\n"
		cikti := bicimleT(t, girdi)
		if !strings.Contains(cikti, m) {
			t.Errorf("metin değişti: %s\nçıktı:\n%s", m, cikti)
		}
	}
}

// Satıra duyarlı yapılar: yeni satırdaki '(', '[' ve sözlük '{' önceki ifadeye bağlanmamalı.
func TestSatirYapisiKorunur(t *testing.T) {
	girdiler := []string{
		"değişken f = 5\n(1 + 2) |> yazdır\n",
		"değişken l = [1]\n[2, 3] |> yazdır\n",
		"değişken s = l.filtrele\n{\"a\": 1} |> yazdır\n",
	}
	for _, g := range girdiler {
		if cikti := bicimleT(t, g); cikti != g {
			t.Errorf("satır yapısı değişti:\n%q\n->\n%q", g, cikti)
		}
	}
}

// Anlamı değiştiren bir dönüşüm reddedilmeli.
func TestAnlamDegisirseReddedilir(t *testing.T) {
	eski := donustur
	defer func() { donustur = eski }()
	var anlam *AnlamDegistiHatasi

	// Bozuk dönüşüm: satırları birleştirir; yeni satırdaki '(' önceki ifadeye bağlanır (5(1 + 2)).
	donustur = func(kaynak string, sus map[konum]int) string {
		return strings.ReplaceAll(kaynak, "\n", " ") + "\n"
	}
	if _, err := Bicimle("değişken f = 5\n(1 + 2) |> yazdır\n"); !errors.As(err, &anlam) {
		t.Fatalf("anlam değişikliği reddedilmedi, hata: %v", err)
	}

	// Sözcük değiştiren dönüşüm de reddedilmeli
	donustur = func(kaynak string, sus map[konum]int) string {
		return strings.ReplaceAll(kaynak, "1", "2")
	}
	if _, err := Bicimle("a = 1\n"); !errors.As(err, &anlam) {
		t.Fatalf("sözcük değişikliği reddedilmedi, hata: %v", err)
	}
}

func TestAnlamAyniMi(t *testing.T) {
	if err := AnlamAyniMi("a=1+2\n", "a = 1 + 2\n"); err != nil {
		t.Fatalf("aynı anlamlı kaynaklar farklı sayıldı: %v", err)
	}
	if err := AnlamAyniMi("değişken f = 5\n(1)\n", "değişken f = 5 (1)\n"); err == nil {
		t.Fatal("farklı anlamlı kaynaklar aynı sayıldı")
	}
}

func TestSozdizimiHatasiBicimlenmez(t *testing.T) {
	_, err := Bicimle("değişken = \n")
	var sh *SozdizimiHatasi
	if !errors.As(err, &sh) {
		t.Fatalf("sözdizimi hatası bildirilmedi: %v", err)
	}
}

// Depodaki tüm .tr dosyaları biçimlenebilmeli ve sonuç idempotent olmalı.
func TestDepodakiDosyalar(t *testing.T) {
	var dosyalar []string
	for _, desen := range []string{"../../ornekler/*.tr", "../../calisma/*.tr", "../../testler/*.tr"} {
		d, _ := filepath.Glob(desen)
		dosyalar = append(dosyalar, d...)
	}
	if len(dosyalar) == 0 {
		t.Skip("örnek dosya bulunamadı")
	}
	for _, yol := range dosyalar {
		veri, err := os.ReadFile(yol)
		if err != nil {
			t.Fatal(err)
		}
		cikti, err := Bicimle(string(veri))
		if err != nil {
			t.Errorf("%s: %v", yol, err)
			continue
		}
		if ikinci, err := Bicimle(cikti); err != nil || ikinci != cikti {
			t.Errorf("%s: idempotent değil (hata: %v)", yol, err)
		}
	}
}

func TestBOMKorunur(t *testing.T) {
	cikti := bicimleT(t, "\xef\xbb\xbfa=1\n")
	if cikti != "\xef\xbb\xbfa = 1\n" {
		t.Fatalf("BOM korunmadı: %q", cikti)
	}
}

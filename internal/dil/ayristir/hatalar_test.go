package ayristir

import (
	"regexp"
	"strings"
	"testing"
)

// İç token adları (DOSYA_SONU, TANIMLAYICI, SONUÇ...) kullanıcıya sızmamalı.
var icAdRe = regexp.MustCompile(`DOSYA_SONU|TANIMLAYICI|TAM_SAYI|ONDALIK_SAYI|METİN|SONUÇ|DEĞİŞKEN|DEĞİLSE|EĞER|FONKSİYON|GEÇERSİZ`)

func TestHataMesajlariTurkceAnlatimKullanir(t *testing.T) {
	durumlar := map[string]string{
		"yazdır(1":                    "')' bekleniyordu, ancak dosya sonu bulundu",
		"değişken = 4":                "bir ad bekleniyordu, ancak '=' bulundu",
		"eğer x = 5 { }":              "'{' bekleniyordu, ancak '=' bulundu (karşılaştırma için '==' yazın",
		"değişken d = [1, 2\nsonuç 3": "']' bekleniyordu, ancak 'sonuç' anahtar kelimesi bulundu",
		"fonksiyon f(a b) { }":        "')' bekleniyordu, ancak 'b' adı bulundu",
		"eşle 1 { 1 \"bir\" }":        "'=>' bekleniyordu, ancak \"bir\" metni bulundu",
		")":                           "burada bir değer ya da ifade bekleniyordu, ancak ')' bulundu",
		"değişken a, b 5":             "çoklu değişken tanımında '=' bekleniyordu, ancak 5 sayısı bulundu",
		"döngü i içinde [1] değilse":  "'{' bekleniyordu, ancak 'değilse' anahtar kelimesi bulundu",
		"x = 99999999999999999999":    "99999999999999999999 tam sayı olarak çok büyük",
	}
	for kod, ister := range durumlar {
		hatalar := parseErrors(kod)
		if len(hatalar) == 0 || !strings.Contains(hatalar[0], ister) {
			t.Errorf("%q: %q içeren hata bekleniyordu, alınan: %v", kod, ister, hatalar)
		}
		for _, h := range hatalar {
			if icAdRe.MatchString(h) {
				t.Errorf("%q: iç token adı sızdı: %s", kod, h)
			}
		}
	}
}

// Tek bir yanlış, ardından gelen geçerli satırlarda art arda hata üretmemeli.
func TestTekHataArtArdaHataUretmez(t *testing.T) {
	durumlar := map[string]int{
		// '=' hatası; gövde ve 'değilse' kolu hatalı deyimin parçasıdır
		"değişken x = 5\neğer x = 5 {\n    yazdır(\"a\")\n    yazdır(\"b\")\n} değilse {\n    yazdır(\"c\")\n}\nyazdır(x)": 1,
		// Parametre listesindeki eksik virgül; gövde atlanır
		"fonksiyon f(a b) {\n    sonuç a\n}\nyazdır(1)": 1,
		// Bloğu kapatan '}' hatalı deyimde yutulmaz: "'{' kapatılmadı" hatası çıkmaz
		"fonksiyon g() {\n    eğer doğru { yazdır( }\n    sonuç 3\n}\nyazdır(g())": 1,
		// Eşle bloğundaki hata kalan durumları da bozmaz
		"eşle 3 {\n    1 \"bir\"\n    2 => \"iki\"\n    _ => \"çok\"\n}\nyazdır(1)": 1,
		// Ayrı satırlardaki bağımsız hatalar ayrı ayrı bildirilir
		"değişken = 1\nyazdır(2)\ndeğişken = 3": 2,
	}
	for kod, ister := range durumlar {
		if hatalar := parseErrors(kod); len(hatalar) != ister {
			t.Errorf("%q: %d hata bekleniyordu, %d alındı:\n%s", kod, ister, len(hatalar), strings.Join(hatalar, "\n"))
		}
	}
}

// Kaldırılmış yazımların ipucu (sozcuk.KaldirilanYazimIpucu) hatalı satırda korunur.
func TestKaldirilanYazimIpucuKorunur(t *testing.T) {
	hatalar := parseErrors("işlev topla(a, b) -> Tam { döndür a + b }")
	birlesik := strings.Join(hatalar, "\n")
	for _, ister := range []string{"'işlev' TR'de anahtar kelime değil; 'fonksiyon'", "'döndür' TR'de anahtar kelime değil; 'sonuç'"} {
		if !strings.Contains(birlesik, ister) {
			t.Errorf("%q ipucu bekleniyordu, alınan:\n%s", ister, birlesik)
		}
	}
}

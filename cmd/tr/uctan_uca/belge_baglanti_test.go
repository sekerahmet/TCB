package uctanuca

import (
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"tr_dili/internal/depo"
)

// Belgelerdeki göreli bağlantılar (`[yazı](yol)`, `[yazı](yol#başlık)`, `[yazı](#başlık)`) var
// olan bir dosyaya ya da klasöre, başlık verilmişse o dosyadaki bir başlığa gitmelidir. Bir
// belge taşındığında ya da bölündüğünde kırılan bağlantıları bu test yakalar.

var (
	baglantiRe = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	baslikRe   = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
)

// kodsuzSatirlar belgenin satırlarıdır; ``` ile çevrili kod bloklarının satırları boş gelir
// (satır numaraları korunur).
func kodsuzSatirlar(metin string) []string {
	var satirlar []string
	kod := false
	for _, s := range strings.Split(metin, "\n") {
		if strings.HasPrefix(strings.TrimSpace(s), "```") {
			kod = !kod
			s = ""
		}
		if kod {
			s = ""
		}
		satirlar = append(satirlar, strings.TrimRight(s, "\r"))
	}
	return satirlar
}

// baslikCapasi bir başlıktan barındırma sitesinin ürettiği çapayı üretir: küçük harf, harf,
// rakam, boşluk, '-' ve '_' dışındaki karakterler atılır, boşluklar '-' olur.
func baslikCapasi(baslik string) string {
	var b strings.Builder
	for _, r := range baslik {
		switch {
		case r == 'İ':
			b.WriteString("i̇") // büyük İ küçültülünce noktalı i + birleşen nokta olur
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || unicode.Is(unicode.Mn, r):
			b.WriteRune(unicode.ToLower(r))
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// capalar belgedeki başlıkların çapalarıdır (aynı başlık tekrar ederse -1, -2 ... eklenir).
func capalar(metin string) map[string]bool {
	sonuc := map[string]bool{}
	sayac := map[string]int{}
	for _, s := range kodsuzSatirlar(metin) {
		m := baslikRe.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		c := baslikCapasi(m[1])
		if n := sayac[c]; n > 0 {
			sonuc[c+"-"+strconv.Itoa(n)] = true
		} else {
			sonuc[c] = true
		}
		sayac[c]++
	}
	return sonuc
}

func TestBelgeBaglantilari(t *testing.T) {
	okunan := map[string]string{}
	oku := func(yol string) (string, bool) {
		if m, ok := okunan[yol]; ok {
			return m, true
		}
		veri, err := os.ReadFile(depo.Yol(yol))
		if err != nil {
			return "", false
		}
		okunan[yol] = string(veri)
		return okunan[yol], true
	}
	toplam := 0
	for _, belge := range belgeListesi(t) {
		metin, ok := oku(belge)
		if !ok {
			t.Fatalf("%s okunamadı", belge)
		}
		for i, satir := range kodsuzSatirlar(metin) {
			for _, m := range baglantiRe.FindAllStringSubmatch(satir, -1) {
				hedef := m[1]
				if strings.Contains(hedef, "://") || strings.HasPrefix(hedef, "mailto:") {
					continue
				}
				toplam++
				yol, capa, _ := strings.Cut(hedef, "#")
				hedefDosya := belge
				if yol != "" {
					hedefDosya = path.Clean(path.Join(path.Dir(belge), yol))
				}
				if strings.HasPrefix(hedefDosya, "../") {
					t.Errorf("%s:%d: %q depo dışına gidiyor", belge, i+1, hedef)
					continue
				}
				bilgi, err := os.Stat(depo.Yol(hedefDosya))
				if err != nil {
					t.Errorf("%s:%d: %q bağlantısının hedefi yok: %s", belge, i+1, hedef, hedefDosya)
					continue
				}
				if capa == "" {
					continue
				}
				if bilgi.IsDir() || !strings.HasSuffix(hedefDosya, ".md") {
					t.Errorf("%s:%d: %q yalnızca bir .md dosyasının başlığına çapa verebilir", belge, i+1, hedef)
					continue
				}
				hedefMetin, _ := oku(hedefDosya)
				if !capalar(hedefMetin)[capa] {
					t.Errorf("%s:%d: %q bağlantısındaki #%s başlığı %s içinde yok", belge, i+1, hedef, capa, hedefDosya)
				}
			}
		}
	}
	if toplam < 30 {
		t.Fatalf("yalnızca %d göreli bağlantı bulundu; düzenli ifade bozulmuş olabilir", toplam)
	}
}

func TestBaslikCapasi(t *testing.T) {
	for baslik, beklenen := range map[string]string{
		"C. Tensörler ve Otomatik Türev":          "c-tensörler-ve-otomatik-türev",
		"7. Test Yazma ve Çalıştırma (`tr test`)": "7-test-yazma-ve-çalıştırma-tr-test",
		"Örnekler":      "örnekler",
		"B. İstatistik": "b-i̇statistik",
	} {
		if c := baslikCapasi(baslik); c != beklenen {
			t.Errorf("baslikCapasi(%q) = %q, beklenen %q", baslik, c, beklenen)
		}
	}
}

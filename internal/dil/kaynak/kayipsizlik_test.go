package kaynak

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/sozcuk"
)

// Kayıpsızlık sınaması (17.4): depodaki bütün .tr dosyaları ve belgelerdeki ```tr kod
// blokları için TR → kod → TR bayt bayt aynıdır, kod → TR → kod da bayt bayt aynıdır. Kodlu
// dosyanın Türkçesiyle aynı çıktıyı verdiği (tr test ve örnekler) internal/arac/testkosucu'da
// sınanır.

// atlananKlasorler depoya ait olmayan ya da üretilmiş klasörlerdir.
var atlananKlasorler = map[string]bool{".git": true, ".claude": true, "bin": true, "calisma": true, "node_modules": true}

// depoDosyalari depodaki uzantısı verilen bütün dosyalardır (göreli yollarıyla).
func depoDosyalari(t *testing.T, uzanti string) []string {
	t.Helper()
	var dosyalar []string
	kok := depo.Kok()
	err := filepath.WalkDir(kok, func(yol string, g fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if g.IsDir() && yol != kok && atlananKlasorler[g.Name()] {
			return filepath.SkipDir
		}
		if !g.IsDir() && strings.HasSuffix(g.Name(), uzanti) {
			goreli, _ := filepath.Rel(kok, yol)
			dosyalar = append(dosyalar, filepath.ToSlash(goreli))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dosyalar
}

// gidisDonus bir kaynağın iki yönlü kimliğini sınar. Kaynak ayrıştırılamıyorsa (bilerek
// hatalı test dosyaları, belgelerdeki hata örnekleri) false döner.
func gidisDonus(t *testing.T, ad, metin string) bool {
	t.Helper()
	b, err := Ayir(metin)
	if err != nil {
		return false // tanınmayan başlık (bilerek hatalı fikstür)
	}
	if b.Kodlu && oluKodVar(b) {
		// Ölü kodlar okunur ama çözülüp yeniden kodlanınca adla kalır (17.6).
		return false
	}
	if b.Kodlu {
		tr, err := Coz(metin, kod.Etkin())
		var sh *SozdizimiHatasi
		if errors.As(err, &sh) {
			return false
		}
		if err != nil {
			t.Errorf("%s: kodlu dosya çözülemedi: %v", ad, err)
			return false
		}
		if geri, err := Kodla(tr); err != nil || geri != metin {
			t.Errorf("%s: kod → TR → kod aynı değil (%v)\n--- özgün\n%s\n--- geri\n%s", ad, err, metin, geri)
		}
		return true
	}
	kodlu, err := Kodla(metin)
	var sh *SozdizimiHatasi
	if errors.As(err, &sh) {
		return false
	}
	if err != nil {
		t.Errorf("%s: kodlanamadı: %v", ad, err)
		return false
	}
	if !strings.Contains(kodlu, Baslik) {
		t.Errorf("%s: kodlu metinde başlık yok", ad)
	}
	geri, err := Coz(kodlu, kod.Etkin())
	if err != nil || geri != metin {
		t.Errorf("%s: TR → kod → TR aynı değil (%v)\n%s", ad, err, ilkFark(metin, geri))
		return false
	}
	if yine, err := Kodla(geri); err != nil || yine != kodlu {
		t.Errorf("%s: kod → TR → kod aynı değil (%v)\n%s", ad, err, ilkFark(kodlu, yine))
	}
	return true
}

// oluKodVar kodlu gövdede ölü (taşınmış ya da kaldırılmış) bir kod var mı?
func oluKodVar(b Bolumler) bool {
	l := b.Sozcukler()
	for tok := l.NextToken(); tok.Type != sozcuk.TOKEN_EOF; tok = l.NextToken() {
		if g, ok := kod.Etkin().Girdisi(tok.Kod); ok && g.Olu() {
			return true
		}
	}
	return false
}

// ilkFark iki metnin ayrıldığı ilk satırı gösterir.
func ilkFark(a, b string) string {
	as, bs := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y string
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		if x != y {
			return "satır " + itoa(i+1) + ":\n  özgün: " + x + "\n  geri:  " + y
		}
	}
	return "(fark yok)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestDepodakiTRDosyalariKayipsizKodlanir(t *testing.T) {
	dosyalar := depoDosyalari(t, ".tr")
	if len(dosyalar) < 40 {
		t.Fatalf("depoda yalnızca %d .tr dosyası bulundu; tarama bozulmuş olabilir", len(dosyalar))
	}
	sinanan, atlanan := 0, []string{}
	for _, d := range dosyalar {
		veri, err := os.ReadFile(depo.Yol(d))
		if err != nil {
			t.Fatal(err)
		}
		if gidisDonus(t, d, string(veri)) {
			sinanan++
		} else {
			atlanan = append(atlanan, d)
		}
	}
	t.Logf("%d .tr dosyası sınandı; ayrıştırılamadığı için atlanan %d: %v", sinanan, len(atlanan), atlanan)
	for _, d := range atlanan {
		if !strings.HasPrefix(d, "testler/olumsuz/") {
			t.Errorf("%s ayrıştırılamadı; yalnızca bilerek hatalı olumsuz test dosyaları atlanabilir", d)
		}
	}
}

// trBlogu belgelerdeki ```tr ... ``` kod bloklarıdır.
var trBlogu = regexp.MustCompile("(?s)```tr\r?\n(.*?)```")

func TestBelgelerdekiKodBloklariKayipsizKodlanir(t *testing.T) {
	sinanan, atlanan := 0, 0
	for _, d := range depoDosyalari(t, ".md") {
		veri, err := os.ReadFile(depo.Yol(d))
		if err != nil {
			t.Fatal(err)
		}
		for i, m := range trBlogu.FindAllStringSubmatch(string(veri), -1) {
			if gidisDonus(t, d+" #"+itoa(i+1), m[1]) {
				sinanan++
			} else {
				atlanan++
			}
		}
	}
	t.Logf("belgelerde %d tr kod bloğu sınandı, %d blok ayrıştırılamadığı için atlandı", sinanan, atlanan)
	if sinanan < 50 {
		t.Errorf("belgelerde yalnızca %d kod bloğu sınandı; tarama bozulmuş olabilir", sinanan)
	}
}

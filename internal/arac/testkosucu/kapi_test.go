package testkosucu

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"tr_dili/internal/depo"
	"tr_dili/internal/dil/yurut"
	"tr_dili/kitaplik"
)

// Faz 3 kapısı: "her kural bir testte". Dil tanımında (docs/dil/NN_*.md) numarası geçen her
// kuralın (**4.2** gibi) testler/ klasöründe adı o numarayla başlayan en az bir testi olmalı;
// tersine, her test numarası tanımda bir kurala karşılık gelmeli. Ardından tüm testler geçmelidir.

// Kural numaraları harfli olabilir (**2.9a**, **6.3a**); testin numarası da harfiyle
// birlikte okunur ve tanımdaki bir kurala tam olarak karşılık gelmelidir (test "6.3a ...",
// kural **6.3a**). Harf yalnızca harfli kurallar içindir; aynı kuralın birden çok testi aynı
// numarayı taşır.
var (
	kuralRe = regexp.MustCompile(`\*\*(\d+\.\d+[a-z]?)\*\*`)
	testRe  = regexp.MustCompile(`(?m)^test "(\d+\.\d+[a-z]?)\s`)
	// Kuralın tanımı satır başında yazılır (**4.2** ...); metnin içindeki **13.1** gibi
	// geçişler başka bir bölümdeki kurala başvurudur.
	kuralTanimiRe = regexp.MustCompile(`(?m)^\*\*(\d+)\.\d+[a-z]?\*\*`)
	bolumDosyaRe  = regexp.MustCompile(`^(\d\d)_[a-z0-9_]+\.md$`)
)

// dilTanimi docs/dil/ altındaki bölüm dosyalarını (NN_ad.md) sırayla okur.
func dilTanimi(t *testing.T) map[string]string {
	t.Helper()
	dosyalar, err := filepath.Glob(depo.Yol("docs/dil/*.md"))
	if err != nil {
		t.Fatal(err)
	}
	bolumler := map[string]string{}
	for _, d := range dosyalar {
		if filepath.Base(d) == "README.md" {
			continue
		}
		if !bolumDosyaRe.MatchString(filepath.Base(d)) {
			t.Errorf("docs/dil/%s: bölüm dosyalarının adı NN_ad.md biçiminde olmalı", filepath.Base(d))
			continue
		}
		veri, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		bolumler[filepath.Base(d)] = string(veri)
	}
	if len(bolumler) == 0 {
		t.Fatal("docs/dil/ altında bölüm dosyası bulunamadı")
	}
	return bolumler
}

// Her kural kendi bölümünün dosyasında tanımlanır: **4.2**, docs/dil/04_*.md'dedir. Böylece
// kural numarası dosyayı da söyler.
func TestKuralKendiBolumunde(t *testing.T) {
	for ad, metin := range dilTanimi(t) {
		no := bolumDosyaRe.FindStringSubmatch(ad)[1]
		for _, m := range kuralTanimiRe.FindAllStringSubmatch(metin, -1) {
			n, _ := strconv.Atoi(m[1])
			if fmt.Sprintf("%02d", n) != no {
				t.Errorf("docs/dil/%s: §%d kuralı tanımlanıyor; kural docs/dil/%02d_*.md dosyasına taşınmalı", ad, n, n)
			}
		}
	}
}

func TestHerKuralBirTestte(t *testing.T) {
	kurallar := map[string]bool{}
	for _, metin := range dilTanimi(t) {
		for _, m := range kuralRe.FindAllStringSubmatch(metin, -1) {
			kurallar[m[1]] = true
		}
	}
	if len(kurallar) < 50 {
		t.Fatalf("tanımda yalnızca %d kural bulundu; biçim değişmiş olabilir", len(kurallar))
	}

	dosyalar, _ := filepath.Glob(depo.Yol("testler/*_test.tr"))
	testli := map[string]bool{}
	for _, d := range dosyalar {
		veri, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range testRe.FindAllStringSubmatch(string(veri), -1) {
			testli[m[1]] = true
			if !kurallar[m[1]] {
				t.Errorf("%s: %s numaralı test tanımda bir kurala karşılık gelmiyor", filepath.Base(d), m[1])
			}
		}
	}

	var eksik []string
	for k := range kurallar {
		if !testli[k] {
			eksik = append(eksik, k)
		}
	}
	sort.Strings(eksik)
	if len(eksik) > 0 {
		t.Errorf("testi olmayan kurallar: %v", eksik)
	}
	// Harfli kurallar kapıdan kaçmamalı (eski düzenli ifade onları hiç saymıyordu).
	for _, k := range []string{"2.9a", "6.3a"} {
		if !kurallar[k] {
			t.Errorf("harfli kural %s tanımda bulunamadı; kapının düzenli ifadesi harfli kuralları kapsamalı", k)
		}
	}
}

// testler/olumsuz/ altındaki bilerek hatalı modüller (yükleme hatası beklenen olumsuz
// testler) bir teste bağlı olmalı: kullanılmayan bir fikstür hiçbir şeyi sınamaz.
func TestOlumsuzFiksturlerKullanilir(t *testing.T) {
	fiksturler, _ := filepath.Glob(depo.Yol("testler/olumsuz/*.tr"))
	if len(fiksturler) == 0 {
		t.Fatal("testler/olumsuz/ altında fikstür bulunamadı")
	}
	var tum []byte
	dosyalar, _ := filepath.Glob(depo.Yol("testler/*_test.tr"))
	for _, d := range dosyalar {
		veri, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		tum = append(tum, veri...)
	}
	for _, f := range fiksturler {
		ad := strings.TrimSuffix(filepath.Base(f), ".tr")
		if !bytes.Contains(tum, []byte(`aktar "olumsuz/`+ad+`"`)) {
			t.Errorf("%s hiçbir testte aktarılmıyor", filepath.Base(f))
		}
	}
}

// Dil tanımının bütün testleri (testler/) yorumlayıcının içinden koşar. Tam denetim
// (tools/tam_denetim.sh) Go testlerini -short ile koşar ve aynı takımı bir kez
// `tr test testler` ile koşar; takım iki derleme ayarında üç kez koşmasın diye burada atlanır.
func TestDilTanimiTestleriGecer(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: dil tanımı testleri tam denetimde `tr test testler` ile koşar")
	}
	// Komut satırındaki gibi dilin kitaplığı bağlanır (11.6: aktar "tr/...").
	yurut.KitaplikBagla(kitaplik.Dosyalar)
	defer yurut.KitaplikBagla(nil)
	var cikti bytes.Buffer
	ozet, err := Calistir(depo.Yol("testler"), &cikti)
	if err != nil {
		t.Fatal(err)
	}
	if !ozet.Basarili() {
		t.Fatalf("dil tanımı testleri kaldı: %s\n%s", ozet, cikti.String())
	}
}

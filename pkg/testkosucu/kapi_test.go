package testkosucu

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Faz 3 kapısı: "her kural bir testte". docs/DIL_TANIMI.md'de numarası geçen her kuralın
// (**4.2** gibi) testler/ klasöründe adı o numarayla başlayan en az bir testi olmalı; tersine,
// her test numarası tanımda bir kurala karşılık gelmeli. Ardından tüm testler geçmelidir.

// Kural numaraları harfli olabilir (**2.9a**, **6.3a**); testin numarası da harfiyle
// birlikte okunur ve tanımdaki bir kurala tam olarak karşılık gelmelidir (test "6.3a ...",
// kural **6.3a**). Harf yalnızca harfli kurallar içindir; aynı kuralın birden çok testi aynı
// numarayı taşır.
var (
	kuralRe = regexp.MustCompile(`\*\*(\d+\.\d+[a-z]?)\*\*`)
	testRe  = regexp.MustCompile(`(?m)^test "(\d+\.\d+[a-z]?)\s`)
)

func TestHerKuralBirTestte(t *testing.T) {
	tanim, err := os.ReadFile("../../docs/DIL_TANIMI.md")
	if err != nil {
		t.Fatalf("dil tanımı okunamadı: %v", err)
	}
	kurallar := map[string]bool{}
	for _, m := range kuralRe.FindAllStringSubmatch(string(tanim), -1) {
		kurallar[m[1]] = true
	}
	if len(kurallar) < 50 {
		t.Fatalf("tanımda yalnızca %d kural bulundu; biçim değişmiş olabilir", len(kurallar))
	}

	dosyalar, _ := filepath.Glob("../../testler/*_test.tr")
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
	fiksturler, _ := filepath.Glob("../../testler/olumsuz/*.tr")
	if len(fiksturler) == 0 {
		t.Fatal("testler/olumsuz/ altında fikstür bulunamadı")
	}
	var tum []byte
	dosyalar, _ := filepath.Glob("../../testler/*_test.tr")
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

func TestDilTanimiTestleriGecer(t *testing.T) {
	var cikti bytes.Buffer
	ozet, err := Calistir("../../testler", &cikti)
	if err != nil {
		t.Fatal(err)
	}
	if !ozet.Basarili() {
		t.Fatalf("dil tanımı testleri kaldı: %s\n%s", ozet, cikti.String())
	}
}

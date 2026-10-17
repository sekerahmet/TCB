package testkosucu

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// Faz 3 kapısı: "her kural bir testte". docs/DIL_TANIMI.md'de numarası geçen her kuralın
// (**4.2** gibi) testler/ klasöründe adı o numarayla başlayan en az bir testi olmalı; tersine,
// her test numarası tanımda bir kurala karşılık gelmeli. Ardından tüm testler geçmelidir.

var (
	kuralRe = regexp.MustCompile(`\*\*(\d+\.\d+)\*\*`)
	testRe  = regexp.MustCompile(`(?m)^test "(\d+\.\d+)[a-z]?\s`)
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

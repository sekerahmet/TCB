package internal_test

import (
	"errors"
	"go/build"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"tr_dili/internal/depo"
)

// Katman kuralı: kim kimi içe aktarabilir. Okun yönü "içe aktarabilir" demektir:
//
//	araçlar (internal/arac, cmd/tr) ─▶ motorlar (internal/motor) ─▶ dil çekirdeği (internal/dil)
//	                                        └──────▶ hesap çekirdekleri (internal/hesap)
//
// Hesap çekirdekleri dili bilmez (tek başına sınanır ve ölçülür), dil çekirdeği motorları ve
// araçları bilmez, motorlar araçları bilmez. Kural yalnızca paketlerin kendi (test dışı)
// içe aktarmalarına uygulanır: testler, sınamak için daha üst katmanları da içe aktarabilir
// (ör. motor testleri hata ayıklayıcıyla çalışır). Her paket tablodaki bir satıra düşmelidir;
// yeni bir klasör açılınca tablo da güncellenir. Ayrıntı: docs/mimari.md.
var katmanlar = []struct {
	onek   string   // paket yolu bu önekle başlar (modül yoluna göre)
	izinli []string // içe aktarabileceği iç paketlerin önekleri
}{
	{"internal/hesap/", []string{"internal/hesap/"}},
	{"internal/dil/", []string{"internal/dil/"}},
	{"internal/motor/", []string{"internal/dil/", "internal/hesap/", "internal/motor/"}},
	{"internal/arac/", []string{"internal/dil/", "internal/hesap/", "internal/motor/", "internal/arac/"}},
	{"internal/depo", nil}, // testlerin depo kökü yardımcısı; yalnızca testlerden ve bakim/'dan içe aktarılır
	{"kitaplik", nil},      // TR ile yazılı kitaplık: Go tarafında yalnızca gömme dosyası
	{"cmd/", []string{"internal/dil/", "internal/hesap/", "internal/motor/", "internal/arac/", "kitaplik"}},
	{"bakim/", []string{"internal/", "kitaplik"}},
}

const modul = "tr_dili"

// atlanan klasörler: Go paketi olmayan ya da depoya ait olmayan yerler.
var atlanan = map[string]bool{".git": true, ".claude": true, "ikili": true, "calisma": true, "testdata": true, "node_modules": true}

func katmanBul(yol string) (izinli []string, ok bool) {
	for _, k := range katmanlar {
		if yol == strings.TrimSuffix(k.onek, "/") || strings.HasPrefix(yol, k.onek) {
			return k.izinli, true
		}
	}
	return nil, false
}

func TestKatmanKurali(t *testing.T) {
	kok := depo.Kok()
	paketSayisi := 0
	err := filepath.WalkDir(kok, func(yol string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if yol != kok && (atlanan[d.Name()] || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
			return filepath.SkipDir
		}
		p, err := build.ImportDir(yol, 0)
		if err != nil {
			var yok *build.NoGoError
			if errors.As(err, &yok) {
				return nil
			}
			t.Errorf("%s: %v", yol, err)
			return nil
		}
		goreli, _ := filepath.Rel(kok, yol)
		goreli = filepath.ToSlash(goreli)
		if len(p.GoFiles) == 0 {
			return nil // yalnızca test dosyası olan klasör (bu dosyanın kendisi gibi)
		}
		paketSayisi++
		izinli, ok := katmanBul(goreli)
		if !ok {
			t.Errorf("%s: paket hiçbir katmana düşmüyor; internal/katman_test.go tablosuna ekleyin", goreli)
			return nil
		}
		ice := append([]string(nil), p.Imports...)
		sort.Strings(ice)
		for _, imp := range ice {
			if !strings.HasPrefix(imp, modul+"/") {
				continue
			}
			hedef := strings.TrimPrefix(imp, modul+"/")
			izin := false
			for _, o := range izinli {
				if hedef == strings.TrimSuffix(o, "/") || strings.HasPrefix(hedef, o) {
					izin = true
					break
				}
			}
			if !izin {
				t.Errorf("%s → %s: katman kuralına aykırı (izinli: %v)", goreli, hedef, izinli)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if paketSayisi < 20 {
		t.Fatalf("yalnızca %d paket bulundu; depo yapısı değişmiş olabilir", paketSayisi)
	}
}

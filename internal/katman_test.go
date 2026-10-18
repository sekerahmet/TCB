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
	{"internal/depo", nil}, // testlerin depo kökü yardımcısı; yalnızca testlerden ve tools/'dan içe aktarılır
	{"kitaplik", nil},      // TR ile yazılı kitaplık: Go tarafında yalnızca gömme dosyası
	{"cmd/", []string{"internal/dil/", "internal/hesap/", "internal/motor/", "internal/arac/", "kitaplik"}},
	{"tools/", []string{"internal/", "kitaplik"}},
}

// dilSirasi dil çekirdeğinin iç sırasıdır: bir dil paketi yalnızca bu listede kendisinden
// önce gelen dil paketlerini içe aktarabilir (sözcük çözümleyici yorumlayıcıyı, denetleyici
// yorumlayıcıyı bilmez). Döngüye en yakın katman burasıdır; yeni bir dil paketi listeye
// yerine göre eklenir. Ayrıntı: docs/mimari.md.
var dilSirasi = []string{
	"internal/dil/turkce", // Türkçe ek, harf ve hata çerçevesi yardımcıları
	"internal/dil/kod",    // kod listesi (adlar ve tanımlar)
	"internal/dil/ipucu",  // ad ipuçları (kaldırılan, taşınan, ASCII yazılan adlar)
	"internal/dil/sozcuk", // sözcük çözümleyici
	"internal/dil/agac",   // sözdizimi ağacı
	"internal/dil/ayristir",
	"internal/dil/denetci",
	"internal/dil/yurut",
}

const modul = "tr_dili"

// atlanan klasörler: Go paketi olmayan ya da depoya ait olmayan yerler.
var atlanan = map[string]bool{".git": true, ".claude": true, "bin": true, "calisma": true, "testdata": true, "node_modules": true}

// onekEslesir paket yolu önekin kendisi ya da onun altındaki bir paket mi? Önek sonunda "/"
// olsa da olmasa da klasör sınırında eşlenir: "internal/depo" ve "kitaplik" önekleri
// "internal/depolama" ya da "kitaplikx" paketini kapsamaz.
func onekEslesir(yol, onek string) bool {
	o := strings.TrimSuffix(onek, "/")
	return yol == o || strings.HasPrefix(yol, o+"/")
}

func TestOnekEslesmesi(t *testing.T) {
	for _, d := range []struct {
		yol, onek string
		esler     bool
	}{
		{"internal/depo", "internal/depo", true},
		{"internal/depo/alt", "internal/depo", true},
		{"internal/depolama", "internal/depo", false},
		{"kitaplik", "kitaplik", true},
		{"kitaplikx", "kitaplik", false},
		{"internal/motor", "internal/motor", true},
		{"internal/motorlar", "internal/motor", false},
		{"internal/hesap", "internal/hesap/", true},
		{"internal/hesap/tensor", "internal/hesap/", true},
		{"internal/hesaplama", "internal/hesap/", false},
		{"cmd/tr", "cmd/", true},
	} {
		if g := onekEslesir(d.yol, d.onek); g != d.esler {
			t.Errorf("onekEslesir(%q, %q) = %v; %v olmalı", d.yol, d.onek, g, d.esler)
		}
	}
}

func katmanBul(yol string) (izinli []string, ok bool) {
	for _, k := range katmanlar {
		if onekEslesir(yol, k.onek) {
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
				if onekEslesir(hedef, o) {
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

// disBagimliliklar modül dışı (standart kitaplık olmayan) içe aktarmalar ve onları içe
// aktarabilen tek paket. Yeni bir dış bağımlılık baş geliştiricinin kararıdır; tek dosya
// dağıtım ve cgo'suz derleme korunur. Dinamik yükleyici: GPU arka ucunun Linux'ta NVIDIA sürücü
// kitaplığını cgo'suz yüklemesi (internal/hesap/gpu/cuda_linux.go).
var disBagimliliklar = map[string]string{
	"github.com/ebitengine/purego": "internal/hesap/gpu",
}

// TestDisBagimliliklar her işletim sistemi için (yapı kısıtları farklı dosyaları seçer)
// modül dışı içe aktarmaları denetler.
func TestDisBagimliliklar(t *testing.T) {
	kok := depo.Kok()
	for _, goos := range []string{"linux", "windows", "darwin"} {
		ctx := build.Default
		ctx.GOOS, ctx.CgoEnabled = goos, false
		err := filepath.WalkDir(kok, func(yol string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			if yol != kok && (atlanan[d.Name()] || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
				return filepath.SkipDir
			}
			p, err := ctx.ImportDir(yol, 0)
			if err != nil {
				return nil
			}
			goreli, _ := filepath.Rel(kok, yol)
			goreli = filepath.ToSlash(goreli)
			for _, imp := range p.Imports {
				ilk, _, _ := strings.Cut(imp, "/")
				if !strings.Contains(ilk, ".") {
					continue // standart kitaplık ya da modülün kendisi
				}
				if izinli, ok := disBagimliliklar[imp]; !ok || izinli != goreli {
					t.Errorf("%s (%s): dış bağımlılık %s izinli değil (izinliler: %v)", goreli, goos, imp, disBagimliliklar)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Dil çekirdeğinin iç sırası (dilSirasi): her dil paketi listede olmalı ve yalnızca
// solundakileri içe aktarmalı.
func TestDilIcSirasi(t *testing.T) {
	kok := depo.Kok()
	sira := map[string]int{}
	for i, p := range dilSirasi {
		sira[p] = i
	}
	girdiler, err := filepath.Glob(filepath.Join(kok, "internal", "dil", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range girdiler {
		p, err := build.ImportDir(g, 0)
		if err != nil || len(p.GoFiles) == 0 {
			continue // Go paketi değil ya da yalnızca test
		}
		goreli, _ := filepath.Rel(kok, g)
		goreli = filepath.ToSlash(goreli)
		yer, ok := sira[goreli]
		if !ok {
			t.Errorf("%s: dil paketi internal/katman_test.go'daki dilSirasi listesinde yok", goreli)
			continue
		}
		for _, imp := range p.Imports {
			hedef := strings.TrimPrefix(imp, modul+"/")
			if hedef == imp || !strings.HasPrefix(hedef, "internal/dil/") {
				continue
			}
			if h, ok := sira[hedef]; !ok || h >= yer {
				t.Errorf("%s → %s: dil içi sıraya aykırı (bir dil paketi yalnızca dilSirasi'nda solundakileri içe aktarabilir)", goreli, hedef)
			}
		}
	}
}

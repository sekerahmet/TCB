package internal_test

import (
	"errors"
	"go/build"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Katman kuralı: kim kimi içe aktarabilir. Okun yönü "içe aktarabilir" demektir:
//
//	araçlar (internal/devtools, cmd/tr) ─▶ motorlar (internal/stdlib) ─▶ dil çekirdeği (internal/lang)
//	                                        └──────▶ hesap çekirdekleri (internal/compute)
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
	{"internal/compute/", []string{"internal/compute/"}},
	{"internal/lang/", []string{"internal/lang/"}},
	{"internal/stdlib/", []string{"internal/lang/", "internal/compute/", "internal/stdlib/"}},
	{"internal/devtools/", []string{"internal/lang/", "internal/compute/", "internal/stdlib/", "internal/devtools/"}},
	{"internal/reporoot", nil}, // testlerin depo kökü yardımcısı; yalnızca testlerden ve tools/'dan içe aktarılır
	{"lib", nil},               // TR ile yazılı kitaplık: Go tarafında yalnızca gömme dosyası
	{"cmd/", []string{"internal/lang/", "internal/compute/", "internal/stdlib/", "internal/devtools/", "lib"}},
	{"tools/", []string{"internal/", "lib"}},
}

// dilSirasi dil çekirdeğinin iç sırasıdır: bir dil paketi yalnızca bu listede kendisinden
// önce gelen dil paketlerini içe aktarabilir (sözcük çözümleyici yorumlayıcıyı, denetleyici
// yorumlayıcıyı bilmez). Döngüye en yakın katman burasıdır; yeni bir dil paketi listeye
// yerine göre eklenir. Ayrıntı: docs/mimari.md.
var dilSirasi = []string{
	"internal/lang/turkish", // Türkçe ek, harf ve hata çerçevesi yardımcıları
	"internal/lang/codes",   // kod listesi (adlar ve tanımlar)
	"internal/lang/hint",    // ad ipuçları (kaldırılan, taşınan, ASCII yazılan adlar)
	"internal/lang/lexer",   // sözcük çözümleyici
	"internal/lang/ast",     // sözdizimi ağacı
	"internal/lang/parser",
	"internal/lang/checker",
	"internal/lang/source", // kaynaktan programa giriş noktası, kodlu biçim ve çevirici
	"internal/lang/interp",
}

const modul = "github.com/sekerahmet/TCB"

// atlanan klasörler: Go paketi olmayan ya da depoya ait olmayan yerler.
var atlanan = map[string]bool{".git": true, ".claude": true, "bin": true, "scratch": true, "testdata": true, "node_modules": true}

// onekEslesir paket yolu önekin kendisi ya da onun altındaki bir paket mi? Önek sonunda "/"
// olsa da olmasa da klasör sınırında eşlenir: "internal/reporoot" ve "lib" önekleri
// "internal/reporootx" ya da "libx" paketini kapsamaz.
func onekEslesir(yol, onek string) bool {
	o := strings.TrimSuffix(onek, "/")
	return yol == o || strings.HasPrefix(yol, o+"/")
}

func TestOnekEslesmesi(t *testing.T) {
	for _, d := range []struct {
		yol, onek string
		esler     bool
	}{
		{"internal/reporoot", "internal/reporoot", true},
		{"internal/reporoot/alt", "internal/reporoot", true},
		{"internal/reporootx", "internal/reporoot", false},
		{"lib", "lib", true},
		{"libx", "lib", false},
		{"internal/stdlib", "internal/stdlib", true},
		{"internal/stdlibx", "internal/stdlib", false},
		{"internal/compute", "internal/compute/", true},
		{"internal/compute/tensor", "internal/compute/", true},
		{"internal/computex", "internal/compute/", false},
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
	kok := reporoot.Kok()
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
// kitaplığını cgo'suz yüklemesi (internal/compute/gpu/cuda_linux.go).
var disBagimliliklar = map[string]string{
	"github.com/ebitengine/purego": "internal/compute/gpu",
}

// TestDisBagimliliklar her işletim sistemi için (yapı kısıtları farklı dosyaları seçer)
// modül dışı içe aktarmaları denetler.
func TestDisBagimliliklar(t *testing.T) {
	kok := reporoot.Kok()
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
				if !strings.Contains(ilk, ".") || onekEslesir(imp, modul) {
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
	kok := reporoot.Kok()
	sira := map[string]int{}
	for i, p := range dilSirasi {
		sira[p] = i
	}
	girdiler, err := filepath.Glob(filepath.Join(kok, "internal", "lang", "*"))
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
			if hedef == imp || !strings.HasPrefix(hedef, "internal/lang/") {
				continue
			}
			if h, ok := sira[hedef]; !ok || h >= yer {
				t.Errorf("%s → %s: dil içi sıraya aykırı (bir dil paketi yalnızca dilSirasi'nda solundakileri içe aktarabilir)", goreli, hedef)
			}
		}
	}
}

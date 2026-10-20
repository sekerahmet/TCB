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
var layers = []struct {
	prefix  string   // paket yolu bu önekle başlar (modül yoluna göre)
	allowed []string // içe aktarabileceği iç paketlerin önekleri
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

// langOrder dil çekirdeğinin iç sırasıdır: bir dil paketi yalnızca bu listede kendisinden
// önce gelen dil paketlerini içe aktarabilir (sözcük çözümleyici yorumlayıcıyı, denetleyici
// yorumlayıcıyı bilmez). Döngüye en yakın katman burasıdır; yeni bir dil paketi listeye
// yerine göre eklenir. Ayrıntı: docs/mimari.md.
var langOrder = []string{
	"internal/lang/messages", // ileti kodları ve metinleri (en alt: yalnızca standart kitaplık)
	"internal/lang/turkish",  // Türkçe ek, harf ve hata çerçevesi yardımcıları
	"internal/lang/codes",    // kod listesi (adlar ve tanımlar)
	"internal/lang/hint",     // ad ipuçları (kaldırılan, taşınan, ASCII yazılan adlar)
	"internal/lang/lexer",    // sözcük çözümleyici
	"internal/lang/ast",      // sözdizimi ağacı
	"internal/lang/parser",
	"internal/lang/checker",
	"internal/lang/source", // kaynaktan programa giriş noktası, kodlu biçim ve çevirici
	"internal/lang/interp",
}

const module = "github.com/sekerahmet/TCB"

// skipped klasörler: Go paketi olmayan ya da depoya ait olmayan yerler.
var skipped = map[string]bool{".git": true, ".claude": true, "bin": true, "scratch": true, "testdata": true, "node_modules": true}

// prefixMatches paket yolu önekin kendisi ya da onun altındaki bir paket mi? Önek sonunda "/"
// olsa da olmasa da klasör sınırında eşlenir: "internal/reporoot" ve "lib" önekleri
// "internal/reporootx" ya da "libx" paketini kapsamaz.
func prefixMatches(path, prefix string) bool {
	o := strings.TrimSuffix(prefix, "/")
	return path == o || strings.HasPrefix(path, o+"/")
}

func TestPrefixMatch(t *testing.T) {
	for _, d := range []struct {
		path, prefix string
		matches      bool
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
		if g := prefixMatches(d.path, d.prefix); g != d.matches {
			t.Errorf("prefixMatches(%q, %q) = %v; %v olmalı", d.path, d.prefix, g, d.matches)
		}
	}
}

func findLayer(path string) (allowed []string, ok bool) {
	for _, k := range layers {
		if prefixMatches(path, k.prefix) {
			return k.allowed, true
		}
	}
	return nil, false
}

func TestLayerRule(t *testing.T) {
	root := reporoot.Root()
	pkgCount := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (skipped[d.Name()] || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
			return filepath.SkipDir
		}
		p, err := build.ImportDir(path, 0)
		if err != nil {
			var missing *build.NoGoError
			if errors.As(err, &missing) {
				return nil
			}
			t.Errorf("%s: %v", path, err)
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if len(p.GoFiles) == 0 {
			return nil // yalnızca test dosyası olan klasör (bu dosyanın kendisi gibi)
		}
		pkgCount++
		allowed, ok := findLayer(rel)
		if !ok {
			t.Errorf("%s: paket hiçbir katmana düşmüyor; internal/layers_test.go tablosuna ekleyin", rel)
			return nil
		}
		imports := append([]string(nil), p.Imports...)
		sort.Strings(imports)
		for _, imp := range imports {
			if !strings.HasPrefix(imp, module+"/") {
				continue
			}
			target := strings.TrimPrefix(imp, module+"/")
			allow := false
			for _, o := range allowed {
				if prefixMatches(target, o) {
					allow = true
					break
				}
			}
			if !allow {
				t.Errorf("%s → %s: katman kuralına aykırı (izinli: %v)", rel, target, allowed)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pkgCount < 20 {
		t.Fatalf("yalnızca %d paket bulundu; depo yapısı değişmiş olabilir", pkgCount)
	}
}

// externalDeps modül dışı (standart kitaplık olmayan) içe aktarmalar ve onları içe
// aktarabilen tek paket. Yeni bir dış bağımlılık baş geliştiricinin kararıdır; tek dosya
// dağıtım ve cgo'suz derleme korunur. Dinamik yükleyici: GPU arka ucunun Linux'ta NVIDIA sürücü
// kitaplığını cgo'suz yüklemesi (internal/compute/gpu/cuda_linux.go).
var externalDeps = map[string]string{
	"github.com/ebitengine/purego": "internal/compute/gpu",
}

// TestExternalDependencies her işletim sistemi için (yapı kısıtları farklı dosyaları seçer)
// modül dışı içe aktarmaları denetler.
func TestExternalDependencies(t *testing.T) {
	root := reporoot.Root()
	for _, goos := range []string{"linux", "windows", "darwin"} {
		ctx := build.Default
		ctx.GOOS, ctx.CgoEnabled = goos, false
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			if path != root && (skipped[d.Name()] || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
				return filepath.SkipDir
			}
			p, err := ctx.ImportDir(path, 0)
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			for _, imp := range p.Imports {
				first, _, _ := strings.Cut(imp, "/")
				if !strings.Contains(first, ".") || prefixMatches(imp, module) {
					continue // standart kitaplık ya da modülün kendisi
				}
				if allowed, ok := externalDeps[imp]; !ok || allowed != rel {
					t.Errorf("%s (%s): dış bağımlılık %s izinli değil (izinliler: %v)", rel, goos, imp, externalDeps)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Dil çekirdeğinin iç sırası (langOrder): her dil paketi listede olmalı ve yalnızca
// solundakileri içe aktarmalı.
func TestLangOrder(t *testing.T) {
	root := reporoot.Root()
	order := map[string]int{}
	for i, p := range langOrder {
		order[p] = i
	}
	entries, err := filepath.Glob(filepath.Join(root, "internal", "lang", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range entries {
		p, err := build.ImportDir(g, 0)
		if err != nil || len(p.GoFiles) == 0 {
			continue // Go paketi değil ya da yalnızca test
		}
		rel, _ := filepath.Rel(root, g)
		rel = filepath.ToSlash(rel)
		place, ok := order[rel]
		if !ok {
			t.Errorf("%s: dil paketi internal/layers_test.go'daki langOrder listesinde yok", rel)
			continue
		}
		for _, imp := range p.Imports {
			target := strings.TrimPrefix(imp, module+"/")
			if target == imp || !strings.HasPrefix(target, "internal/lang/") {
				continue
			}
			if h, ok := order[target]; !ok || h >= place {
				t.Errorf("%s → %s: dil içi sıraya aykırı (bir dil paketi yalnızca langOrder'da solundakileri içe aktarabilir)", rel, target)
			}
		}
	}
}

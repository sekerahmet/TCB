package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// indexProject küçük bir TR projesi yazar: bir kitaplık modülü, onu aktaran ve sınayan dosyalar.
func indexProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, dir, "lib/geo.tr", "// Kare bir kenar uzunluğu taşır.\nyapı Kare {\n    kenar = 1\n    fonksiyon alan() {\n        sonuç kendi.kenar * kendi.kenar\n    }\n}\n\nfonksiyon çevre(k) {\n    yazdır(k.kenar)\n    sonuç 4 * k.kenar\n}\n")
	writeFile(t, dir, "lib/geo_test.tr", "aktar \"geo\"\n\ntest \"çevre hesaplanır\" {\n    çevre(Kare())\n}\n")
	return dir
}

// 'tr dizin' dizini kurar; alt komutlar tanımı, kullananları, testleri ve etkiyi verir (17.8).
func TestIndexCommandAndSubcommands(t *testing.T) {
	dir := indexProject(t)
	wait(t, tr(t, dir, "dizin"), 0, []string{"2 dosya"}, nil)
	wait(t, tr(t, dir, "dizin", "tanım", "Kare"), 0, []string{"lib/geo.tr:2:1", "Kare bir kenar uzunluğu taşır."}, nil)
	wait(t, tr(t, dir, "dizin", "kullananlar", "çevre"), 0, []string{"lib/geo_test.tr:4:5"}, nil)
	wait(t, tr(t, dir, "dizin", "sınayanlar", "çevre"), 0, []string{"lib/geo_test.tr:4:5"}, nil)
	wait(t, tr(t, dir, "dizin", "etki", "lib/geo.tr"), 0, []string{"lib/geo_test.tr"}, nil)
}

// Alt komutlar her gömülü dilde kabul edilir; üst düzeyde komut değildir.
func TestIndexSubcommandsInEveryLanguage(t *testing.T) {
	dir := indexProject(t)
	wait(t, tr(t, dir, "index", "references", "çevre", "--limit", "5"), 0, []string{"lib/geo_test.tr:4:5"}, nil)
	wait(t, run(t, trBinary, dir, "", []string{"TR_DIL=en"}, "dizin", "kullananlar", "çevre"), 0, []string{"lib/geo_test.tr:4:5"}, nil)
	for _, name := range []string{"tanım", "definition", "kullananlar", "etki"} {
		s := tr(t, dir, name, "Kare")
		if s.code == 0 {
			t.Errorf("tr %s must not be a command: %+v", name, s)
		}
	}
}

// Çıkış kodu sonucun durumudur: belirsiz ad 3, bulunamayan ad 4; --json kararlı şemadır.
func TestIndexExitCodesAndJSON(t *testing.T) {
	dir := indexProject(t)
	writeFile(t, dir, "m.tr", "yapı A {\n    fonksiyon ileri() {\n        sonuç 1\n    }\n}\nyapı B {\n    fonksiyon ileri() {\n        sonuç 2\n    }\n}\n")
	wait(t, tr(t, dir, "dizin", "tanım", "ileri"), 3, []string{"A.ileri", "B.ileri"}, nil)
	wait(t, tr(t, dir, "dizin", "tanım", "yok_boyle_bir_ad"), 4, nil, nil)
	s := tr(t, dir, "dizin", "kullananlar", "çevre", "--json")
	wait(t, s, 0, []string{`"schema": "tr-codeindex/1"`}, nil)
	var res struct {
		Schema, Command, Status string
		Counts                  struct{ Total int }
	}
	if err := json.Unmarshal([]byte(s.stdout), &res); err != nil {
		t.Fatalf("--json is JSON: %v\n%s", err, s.stdout)
	}
	if res.Command != "references" || res.Status != "ok" || res.Counts.Total != 1 {
		t.Errorf("result: %+v", res)
	}
}

// Yanlış kullanım açık iletiyle 1 koduyla biter.
func TestIndexUsageErrors(t *testing.T) {
	dir := indexProject(t)
	wait(t, tr(t, dir, "dizin", "bilinmeyen_alt"), 1, nil, []string{"bilinmeyen_alt"})
	wait(t, tr(t, dir, "dizin", "tanım"), 1, nil, []string{"tanım"})
	wait(t, tr(t, dir, "dizin", "tanım", "Kare", "--sınır", "x"), 1, nil, []string{"--sınır"})
	wait(t, tr(t, dir, "dizin", "tanım", "Kare", "--durum"), 1, nil, []string{"--durum"})
}

// Proje kökü: --kök, yoksa .git bulunan üst klasör (alt klasörden de sorulabilir).
func TestIndexRootDiscovery(t *testing.T) {
	dir := indexProject(t)
	writeFile(t, dir, "derin/ic/boş.txt", "x")
	wait(t, tr(t, dir+"/derin/ic", "dizin", "tanım", "Kare"), 0, []string{"lib/geo.tr:2:1"}, nil)
	wait(t, tr(t, t.TempDir(), "dizin", "tanım", "Kare", "--kök", dir), 0, []string{"lib/geo.tr:2:1"}, nil)
}

// Önbellek: ilk çalıştırma kurar, ikincisi önbellekten okur, --durum değişeni söyler, --dizin-yok
// değişikliği görmez, --temizle önbelleği siler.
func TestIndexCache(t *testing.T) {
	dir := indexProject(t)
	wait(t, tr(t, dir, "dizin", "--durum"), 0, []string{"önbellek yok"}, nil)
	wait(t, tr(t, dir, "dizin"), 0, []string{"2/2 dosya yeniden ayrıştırıldı"}, nil)
	wait(t, tr(t, dir, "dizin"), 0, []string{"önbellekten okundu"}, nil)
	writeFile(t, dir, "lib/geo_test.tr", "aktar \"geo\"\n\ntest \"çevre hesaplanır\" {\n    çevre(Kare())\n    çevre(Kare())\n}\n")
	wait(t, tr(t, dir, "dizin", "--durum"), 0, []string{"1 değişmiş", "lib/geo_test.tr"}, nil)
	wait(t, tr(t, dir, "dizin", "kullananlar", "çevre", "--dizin-yok", "--json"), 0, []string{`"total": 1`}, nil)
	wait(t, tr(t, dir, "dizin", "kullananlar", "çevre", "--json"), 0, []string{`"total": 2`}, nil)
	wait(t, tr(t, dir, "dizin", "--temizle"), 0, []string{"önbellek silindi"}, nil)
	wait(t, tr(t, dir, "dizin", "--temizle"), 0, []string{"silinecek önbellek yok"}, nil)
}

// 'tr dizin yeniden_adlandır' değişecek yerleri kesin ve olası diye listeler, hiçbir dosyayı değiştirmez.
func TestIndexRename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, dir, "m.tr", "yapı Dikkat {\n    baş\n    fonksiyon say() {\n        sonuç kendi.baş\n    }\n}\n\nfonksiyon yap(baş) {\n    değişken d = Dikkat()\n    sonuç d.baş + baş\n}\n\ndeğişken e = yap(baş: 4)\nyazdır(e)\n")
	writeFile(t, dir, "doc.md", "Dikkatin başı ve başka şey.\n")
	r := tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş", "bakış_sayısı", "--metin-yok")
	wait(t, r, 0, []string{"kesin (", "olası (", "m.tr:2:5", "m.tr:4:21", "hiçbir dosya değiştirilmedi"}, nil)
	if strings.Contains(r.stdout, "doc.md") {
		t.Error("--metin-yok: no text scan")
	}
	wait(t, tr(t, dir, "rename", "baş", "bakış_sayısı"), 1, nil, nil) // alt komut yalnızca 'dizin' altında
	wait(t, tr(t, dir, "index", "rename", "Dikkat.baş", "bakış_sayısı", "--json"), 0, []string{`"command": "rename"`, `"doc.md"`, `"suffixed"`}, nil)
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş", "yazdır"), 2, nil, nil)
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "yazdır", "yaz"), 2, nil, nil)
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş"), 1, nil, []string{"eski ve yeni ad"})
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.yok_boyle", "x"), 4, nil, nil)
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş", "bakış", "--sınır", "1"), 0, []string{"--tümü"}, nil)
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "baş", "bakış_sayısı"), 2, []string{"birden çok", "field:m.tr#Dikkat.baş", "parameter:m.tr#yap.baş"}, nil)
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "başka_yok", "x"), 0, []string{"terim kipi"}, nil)
	wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "baş", "bakış", "--terim"), 0, []string{"terim kipi", "doc.md:1:10"}, nil)
}

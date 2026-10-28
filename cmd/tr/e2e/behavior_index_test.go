package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/testkit"
)

// indexProject küçük bir TR projesi yazar: bir kitaplık modülü, onu aktaran ve sınayan dosyalar.
func indexProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testkit.WriteFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	writeTR(t, dir, "lib/geo.tr", "# Kare bir kenar uzunluğu taşır.\nTR_struct Kare {\n    kenar = 1\n    TR_func alan() {\n        TR_return TR_self.kenar * TR_self.kenar\n    }\n}\n\nTR_func çevre(k) {\n    TR_print(k.kenar)\n    TR_return 4 * k.kenar\n}\n")
	writeTR(t, dir, "lib/geo_test.tr", "TR_import \"geo\"\n\nTR_test \"çevre hesaplanır\" {\n    çevre(Kare())\n}\n")
	return dir
}

// 'tr dizin' dizini kurar; alt komutlar tanımı, kullananları, testleri ve etkiyi verir (17.8).
func TestIndexCommandAndSubcommands(t *testing.T) {
	dir := indexProject(t)
	testkit.Wait(t, tr(t, dir, "dizin"), 0, []string{"2 dosya"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "tanım", "Kare"), 0, []string{"lib/geo.tr:3:1", "Kare bir kenar uzunluğu taşır."}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "kullananlar", "çevre"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "sınayanlar", "çevre"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "etki", "lib/geo.tr"), 0, []string{"lib/geo_test.tr"}, nil)
}

// Alt komutlar her gömülü dilde kabul edilir; üst düzeyde komut değildir.
func TestIndexSubcommandsInEveryLanguage(t *testing.T) {
	dir := indexProject(t)
	testkit.Wait(t, tr(t, dir, "index", "references", "çevre", "--limit", "5"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_DIL=en"}, "dizin", "kullananlar", "çevre"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	for _, name := range []string{"tanım", "definition", "kullananlar", "etki"} {
		s := tr(t, dir, name, "Kare")
		if s.Code == 0 {
			t.Errorf("tr %s must not be a command: %+v", name, s)
		}
	}
}

// Çıkış kodu sonucun durumudur: belirsiz ad 3, bulunamayan ad 4; --json kararlı şemadır.
func TestIndexExitCodesAndJSON(t *testing.T) {
	dir := indexProject(t)
	writeTR(t, dir, "m.tr", "TR_struct A {\n    TR_func TR_forward() {\n        TR_return 1\n    }\n}\nTR_struct B {\n    TR_func TR_forward() {\n        TR_return 2\n    }\n}\n")
	testkit.Wait(t, tr(t, dir, "dizin", "tanım", "ileri"), 3, []string{"A.ileri", "B.ileri"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "tanım", "yok_boyle_bir_ad"), 4, nil, nil)
	s := tr(t, dir, "dizin", "kullananlar", "çevre", "--json")
	testkit.Wait(t, s, 0, []string{`"schema": "tr-codeindex/2"`}, nil)
	var res struct {
		Schema, Command, Status string
		Counts                  struct{ Total int }
	}
	if err := json.Unmarshal([]byte(s.Stdout), &res); err != nil {
		t.Fatalf("--json is JSON: %v\n%s", err, s.Stdout)
	}
	if res.Command != "references" || res.Status != "ok" || res.Counts.Total != 1 {
		t.Errorf("result: %+v", res)
	}
}

// Yanlış kullanım açık iletiyle kullanım hatası koduyla (2) biter (1.9).
func TestIndexUsageErrors(t *testing.T) {
	dir := indexProject(t)
	testkit.Wait(t, tr(t, dir, "dizin", "bilinmeyen_alt"), 2, nil, []string{"bilinmeyen_alt"})
	testkit.Wait(t, tr(t, dir, "dizin", "tanım"), 2, nil, []string{"tanım"})
	testkit.Wait(t, tr(t, dir, "dizin", "tanım", "Kare", "--sınır", "x"), 2, nil, []string{"--sınır"})
	testkit.Wait(t, tr(t, dir, "dizin", "tanım", "Kare", "--durum"), 2, nil, []string{"--durum"})
}

// Proje kökü: --kök, yoksa .git bulunan üst klasör (alt klasörden de sorulabilir).
func TestIndexRootDiscovery(t *testing.T) {
	dir := indexProject(t)
	testkit.WriteFile(t, dir, "derin/ic/boş.txt", "x")
	testkit.Wait(t, tr(t, dir+"/derin/ic", "dizin", "tanım", "Kare"), 0, []string{"lib/geo.tr:3:1"}, nil)
	testkit.Wait(t, tr(t, t.TempDir(), "dizin", "tanım", "Kare", "--kök", dir), 0, []string{"lib/geo.tr:3:1"}, nil)
}

// Önbellek: ilk çalıştırma kurar, ikincisi önbellekten okur, --durum değişeni söyler, --dizin-yok
// değişikliği görmez, --temizle önbelleği siler.
func TestIndexCache(t *testing.T) {
	dir := indexProject(t)
	testkit.Wait(t, tr(t, dir, "dizin", "--durum"), 0, []string{"önbellek yok"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin"), 0, []string{"2/2 dosya yeniden ayrıştırıldı"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin"), 0, []string{"önbellekten okundu"}, nil)
	writeTR(t, dir, "lib/geo_test.tr", "TR_import \"geo\"\n\nTR_test \"çevre hesaplanır\" {\n    çevre(Kare())\n    çevre(Kare())\n}\n")
	testkit.Wait(t, tr(t, dir, "dizin", "--durum"), 0, []string{"1 değişmiş", "lib/geo_test.tr"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "kullananlar", "çevre", "--dizin-yok", "--json"), 0, []string{`"total": 1`}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "kullananlar", "çevre", "--json"), 0, []string{`"total": 2`}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "--temizle"), 0, []string{"önbellek silindi"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "--temizle"), 0, []string{"silinecek önbellek yok"}, nil)
}

// 'tr dizin yeniden_adlandır' değişecek yerleri kesin ve olası diye listeler, hiçbir dosyayı değiştirmez.
func TestIndexRename(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	writeTR(t, dir, "m.tr", "TR_struct Dikkat {\n    baş\n    TR_func say() {\n        TR_return TR_self.baş\n    }\n}\n\nTR_func yap(baş) {\n    TR_var d = Dikkat()\n    TR_return d.baş + baş\n}\n\nTR_var e = yap(baş: 4)\nTR_print(e)\n")
	testkit.WriteFile(t, dir, "doc.md", "Dikkatin başı ve başka şey.\n")
	r := tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş", "bakış_sayısı", "--metin-yok")
	testkit.Wait(t, r, 0, []string{"kesin (", "olası (", "m.tr:3:5", "m.tr:5:27", "hiçbir dosya değiştirilmedi"}, nil)
	if strings.Contains(r.Stdout, "doc.md") {
		t.Error("--metin-yok: no text scan")
	}
	testkit.Wait(t, tr(t, dir, "rename", "baş", "bakış_sayısı"), 1, nil, nil) // alt komut yalnızca 'dizin' altında
	testkit.Wait(t, tr(t, dir, "index", "rename", "Dikkat.baş", "bakış_sayısı", "--json"), 0, []string{`"command": "rename"`, `"doc.md"`, `"suffixed"`}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş", "yazdır"), 2, nil, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "yazdır", "yaz"), 2, nil, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş"), 2, nil, []string{"eski ve yeni ad"})
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.yok_boyle", "x"), 4, nil, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "Dikkat.baş", "bakış", "--sınır", "1"), 0, []string{"--tümü"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "baş", "bakış_sayısı"), 2, []string{"birden çok", "field:m.tr#Dikkat.baş", "parameter:m.tr#yap.baş"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "başka_yok", "x"), 0, []string{"terim kipi"}, nil)
	testkit.Wait(t, tr(t, dir, "dizin", "yeniden_adlandır", "baş", "bakış", "--terim"), 0, []string{"terim kipi", "doc.md:1:10"}, nil)
}

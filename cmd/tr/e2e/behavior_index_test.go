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

// 'tr index' dizini kurar; alt komutlar tanımı, kullananları, testleri ve etkiyi verir (17.8).
func TestIndexCommandAndSubcommands(t *testing.T) {
	dir := indexProject(t)
	testkit.Wait(t, tr(t, dir, "index"), 0, []string{"2 dosya"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "definition", "Kare"), 0, []string{"lib/geo.tr:3:1", "Kare bir kenar uzunluğu taşır."}, nil)
	testkit.Wait(t, tr(t, dir, "index", "references", "çevre"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "tested_by", "çevre"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "impact", "lib/geo.tr"), 0, []string{"lib/geo_test.tr"}, nil)
}

// Alt komutlar her gömülü dilde kabul edilir; üst düzeyde komut değildir.
func TestIndexSubcommandsInEveryLanguage(t *testing.T) {
	dir := indexProject(t)
	testkit.Wait(t, tr(t, dir, "index", "references", "çevre", "--limit", "5"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	testkit.Wait(t, testkit.RunBinary(t, trBinary, dir, "", []string{"TR_LANG=en"}, "index", "references", "çevre"), 0, []string{"lib/geo_test.tr:5:5"}, nil)
	for _, name := range []string{"definition", "definition", "references", "impact"} {
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
	testkit.Wait(t, tr(t, dir, "index", "definition", "ileri"), 3, []string{"A.ileri", "B.ileri"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "definition", "yok_boyle_bir_ad"), 4, nil, nil)
	s := tr(t, dir, "index", "references", "çevre", "--json")
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
	testkit.Wait(t, tr(t, dir, "index", "bilinmeyen_alt"), 2, nil, []string{"bilinmeyen_alt"})
	testkit.Wait(t, tr(t, dir, "index", "definition"), 2, nil, []string{"definition"})
	testkit.Wait(t, tr(t, dir, "index", "definition", "Kare", "--limit", "x"), 2, nil, []string{"--limit"})
	testkit.Wait(t, tr(t, dir, "index", "definition", "Kare", "--status"), 2, nil, []string{"--status"})
}

// Proje kökü: --root, yoksa .git bulunan üst klasör (alt klasörden de sorulabilir).
func TestIndexRootDiscovery(t *testing.T) {
	dir := indexProject(t)
	testkit.WriteFile(t, dir, "derin/ic/boş.txt", "x")
	testkit.Wait(t, tr(t, dir+"/derin/ic", "index", "definition", "Kare"), 0, []string{"lib/geo.tr:3:1"}, nil)
	testkit.Wait(t, tr(t, t.TempDir(), "index", "definition", "Kare", "--root", dir), 0, []string{"lib/geo.tr:3:1"}, nil)
}

// Önbellek: ilk çalıştırma kurar, ikincisi önbellekten okur, --status değişeni söyler, --no-index
// değişikliği görmez, --clear önbelleği siler.
func TestIndexCache(t *testing.T) {
	dir := indexProject(t)
	testkit.Wait(t, tr(t, dir, "index", "--status"), 0, []string{"önbellek yok"}, nil)
	testkit.Wait(t, tr(t, dir, "index"), 0, []string{"2/2 dosya yeniden ayrıştırıldı"}, nil)
	testkit.Wait(t, tr(t, dir, "index"), 0, []string{"önbellekten okundu"}, nil)
	writeTR(t, dir, "lib/geo_test.tr", "TR_import \"geo\"\n\nTR_test \"çevre hesaplanır\" {\n    çevre(Kare())\n    çevre(Kare())\n}\n")
	testkit.Wait(t, tr(t, dir, "index", "--status"), 0, []string{"1 değişmiş", "lib/geo_test.tr"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "references", "çevre", "--no-index", "--json"), 0, []string{`"total": 1`}, nil)
	testkit.Wait(t, tr(t, dir, "index", "references", "çevre", "--json"), 0, []string{`"total": 2`}, nil)
	testkit.Wait(t, tr(t, dir, "index", "--clear"), 0, []string{"önbellek silindi"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "--clear"), 0, []string{"silinecek önbellek yok"}, nil)
}

// 'tr index rename' değişecek yerleri kesin ve olası diye listeler, hiçbir dosyayı değiştirmez.
func TestIndexRename(t *testing.T) {
	dir := t.TempDir()
	testkit.WriteFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	writeTR(t, dir, "m.tr", "TR_struct Dikkat {\n    baş\n    TR_func say() {\n        TR_return TR_self.baş\n    }\n}\n\nTR_func yap(baş) {\n    TR_var d = Dikkat()\n    TR_return d.baş + baş\n}\n\nTR_var e = yap(baş: 4)\nTR_print(e)\n")
	testkit.WriteFile(t, dir, "doc.md", "Dikkatin başı ve başka şey.\n")
	r := tr(t, dir, "index", "rename", "Dikkat.baş", "bakış_sayısı", "--no-text")
	testkit.Wait(t, r, 0, []string{"kesin (", "olası (", "m.tr:3:5", "m.tr:5:27", "hiçbir dosya değiştirilmedi"}, nil)
	if strings.Contains(r.Stdout, "doc.md") {
		t.Error("--no-text: no text scan")
	}
	testkit.Wait(t, tr(t, dir, "rename", "baş", "bakış_sayısı"), 1, nil, nil) // alt komut yalnızca 'tr index' altında
	testkit.Wait(t, tr(t, dir, "index", "rename", "Dikkat.baş", "bakış_sayısı", "--json"), 0, []string{`"command": "rename"`, `"doc.md"`, `"suffixed"`}, nil)
	testkit.Wait(t, tr(t, dir, "index", "rename", "Dikkat.baş", "yazdır"), 2, nil, nil)
	testkit.Wait(t, tr(t, dir, "index", "rename", "yazdır", "yaz"), 2, nil, nil)
	testkit.Wait(t, tr(t, dir, "index", "rename", "Dikkat.baş"), 2, nil, []string{"eski ve yeni ad"})
	testkit.Wait(t, tr(t, dir, "index", "rename", "Dikkat.yok_boyle", "x"), 4, nil, nil)
	testkit.Wait(t, tr(t, dir, "index", "rename", "Dikkat.baş", "bakış", "--limit", "1"), 0, []string{"--all"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "rename", "baş", "bakış_sayısı"), 2, []string{"birden çok", "field:m.tr#Dikkat.baş", "parameter:m.tr#yap.baş"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "rename", "başka_yok", "x"), 0, []string{"terim kipi"}, nil)
	testkit.Wait(t, tr(t, dir, "index", "rename", "baş", "bakış", "--term"), 0, []string{"terim kipi", "doc.md:1:10"}, nil)
}

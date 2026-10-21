package e2e

import (
	"encoding/json"
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

package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Tam denetimin go-esleri kipi (AVX2 kapalı, assembly'siz Go eşleri) Go testlerini yalnızca
// esleri_paketleri dizisindeki paketlerde koşar (tools/full_check.sh). Assembly (.s) taşıyan her
// paket bu dizide olmalıdır; yoksa o paketin Go eşleri hiç sınanmaz.
func TestGoEquivalentPackagesListed(t *testing.T) {
	root := reporoot.Path(".")
	script, err := os.ReadFile(filepath.Join(root, "tools", "full_check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^esleri_paketleri=\(([^)]*)\)`).FindSubmatch(script)
	if m == nil {
		t.Fatal("tools/full_check.sh: esleri_paketleri dizisi bulunamadı")
	}
	listed := map[string]bool{}
	for _, p := range strings.Fields(string(m[1])) {
		listed[strings.TrimPrefix(p, "./")] = true
	}
	asm := map[string]bool{}
	for _, dir := range []string{"internal", "cmd"} {
		reporoot.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".s") {
				rel, _ := filepath.Rel(root, filepath.Dir(path))
				asm[filepath.ToSlash(rel)] = true
			}
			return nil
		})
	}
	if len(asm) == 0 {
		t.Fatal("assembly dosyası bulunamadı (internal/compute/tensor'da olmalı)")
	}
	var missing []string
	for p := range asm {
		if !listed[p] {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("assembly taşıyan paketler esleri_paketleri'nde yok (tools/full_check.sh): %v", missing)
	}
	// TR_CPU_AVX2 paket başlatılırken okunur, Go test önbelleğinin anahtarına girmez: go-esleri
	// turu önbelleksiz koşmazsa dagitim turunun sonucunu yeniden kullanır.
	if !regexp.MustCompile(`(?m)^\s*go_tests go-esleri .*-count=1`).Match(script) {
		t.Error("tools/full_check.sh: go-esleri turu -count=1 ile koşmalı (önbellek dagitim'in sonucunu verir)")
	}
}

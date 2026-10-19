package files

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func content(t *testing.T, fpath string) string {
	t.Helper()
	b, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCopyFile(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "a.txt"), "bir")
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(filepath.Join(k, "a.txt"), old, old)
	os.Mkdir(filepath.Join(k, "yedek"), 0o755)
	if got := mustRunTR(t, `kopyala("a.txt", "yedek")`); got != "yedek/a.txt" {
		t.Fatalf("klasörün içine: %s", got)
	}
	if b, _ := os.Stat(filepath.Join(k, "yedek", "a.txt")); !b.ModTime().Equal(old) {
		t.Errorf("değiştirilme zamanı korunmadı: %v", b.ModTime())
	}
	errContains(t, `kopyala("a.txt", "yedek/")`, `hedef "yedek/a.txt" zaten var`)
	mustRunTR(t, `kopyala(yol("a.txt"), yol("b.txt"))`)
	if content(t, filepath.Join(k, "b.txt")) != "bir" {
		t.Fatal("yeni adla kopya")
	}
	write(t, filepath.Join(k, "a.txt"), "iki")
	errContains(t, `kopyala("a.txt", "b.txt")`, `kopyala(): hedef "b.txt" zaten var; yerine koymak için üzerine_yaz: doğru verin`)
	mustRunTR(t, `kopyala("a.txt", "b.txt", üzerine_yaz: doğru)`)
	if content(t, filepath.Join(k, "b.txt")) != "iki" {
		t.Fatal("üzerine yazılmadı")
	}
	errContains(t, `kopyala("a.txt", "a.txt", üzerine_yaz: doğru)`, "kaynak ile hedef aynı")
	errContains(t, `kopyala("a.txt", "olmayan/")`, `hedef klasör "olmayan/" yok; önce klasör_oluştur("olmayan")`)
	errContains(t, `kopyala("a.txt", "olmayan/b.txt")`, `"olmayan" klasörü yok`)
	errContains(t, `kopyala("yok.txt", "c.txt")`, `"yok.txt" bulunamadı`)
	errContains(t, `kopyala("a.txt", "b.txt", overwrite: doğru)`, "Bunu mu demek istediniz: 'üzerine_yaz'?")
	// Dosya klasörün, klasör dosyanın yerine konmaz.
	os.MkdirAll(filepath.Join(k, "h", "a.txt"), 0o755)
	errContains(t, `kopyala("a.txt", "h", üzerine_yaz: doğru)`, "bir klasör, kaynak bir dosya")
	if entries, _ := os.ReadDir(k); len(entries) != 4 {
		t.Errorf("geçici dosya kaldı: %v", entries)
	}
}

func TestCopyDir(t *testing.T) {
	k := tmpWork(t)
	buildTree(t, k)
	write(t, filepath.Join(k, "dis", "x.txt"), "x")
	dirLink(t, filepath.Join(k, "dis"), filepath.Join(k, "v", "bag"))
	s := runTR(t, "", `kopyala("v", "v2")`)
	if s.failure != nil || s.value.Inspect() != "v2" {
		t.Fatalf("%v %v", s.value, s.failure)
	}
	if !strings.Contains(s.warning, "1 bağlantı ya da özel dosya kopyalanmadı") {
		t.Errorf("uyarı: %q", s.warning)
	}
	if content(t, filepath.Join(k, "v2", "alt", "derin", "d.npy")) != "x" || !exists(filepath.Join(k, "v2", ".gizli", "e.npy")) || exists(filepath.Join(k, "v2", "bag")) {
		t.Fatal("ağaç eksik kopyalandı ya da bağlantı kopyalandı")
	}
	errContains(t, `kopyala("v", "v/alt")`, "kendi içine")
	errContains(t, `kopyala("v/bag", "x")`, "bir bağlantı; TR bağlantıları izlemez ve kopyalamaz")
	// Var olan klasörün yerine konma: eski içerik gider, yenisi gelir.
	write(t, filepath.Join(k, "hedef", "v", "eski.txt"), "e")
	mustRunTR(t, `kopyala("v", "hedef", üzerine_yaz: doğru)`)
	if exists(filepath.Join(k, "hedef", "v", "eski.txt")) || !exists(filepath.Join(k, "hedef", "v", "a.npy")) {
		t.Fatal("klasör yerine konmadı")
	}
	if g, _ := filepath.Glob(filepath.Join(k, "hedef", ".*")); len(g) != 0 {
		t.Errorf("geçici klasör kaldı: %v", g)
	}
	// Çalışma klasörünün yerine bir şey konmaz.
	name := filepath.Base(k)
	write(t, filepath.Join(k, "alt", name, "x.txt"), "x")
	errContains(t, `kopyala("alt/`+name+`", "`+forwardPath(filepath.Dir(k))+`", üzerine_yaz: doğru)`, "kaynağı içeriyor")
}

func TestMove(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "a.txt"), "x")
	if got := mustRunTR(t, `taşı("a.txt", "b.txt")`); got != "b.txt" || exists(filepath.Join(k, "a.txt")) {
		t.Fatalf("yeniden adlandırma: %s", got)
	}
	buildTree(t, k)
	if got := mustRunTR(t, `taşı("v", "w")`); got != "w" || exists(filepath.Join(k, "v")) || !exists(filepath.Join(k, "w", "alt", "c.npy")) {
		t.Fatalf("klasör: %s", got)
	}
	write(t, filepath.Join(k, "c.txt"), "c")
	errContains(t, `taşı("c.txt", "b.txt")`, "zaten var")
	mustRunTR(t, `taşı("c.txt", "b.txt", üzerine_yaz: doğru)`)
	if content(t, filepath.Join(k, "b.txt")) != "c" {
		t.Fatal("üzerine taşınmadı")
	}
	errContains(t, `taşı(".", "x")`, "çalışma klasörü", "yerinden kaldırmaz")
}

// Başka bir diske taşıma: yeniden adlandırma "farklı aygıt" hatası verince kopyalanır,
// doğrulanır, kaynak silinir.
func TestMoveToOtherDisk(t *testing.T) {
	k := tmpWork(t)
	old := rename
	t.Cleanup(func() { rename = old })
	no := syscall.EXDEV
	if runtime.GOOS == "windows" {
		no = syscall.Errno(17)
	}
	rename = func(a, b string) error {
		if strings.Contains(a, ".kopyalaniyor") || strings.Contains(a, ".eski") || strings.Contains(b, ".eski") {
			return old(a, b)
		}
		return &os.LinkError{Op: "rename", Old: a, New: b, Err: no}
	}
	buildTree(t, k)
	if got := mustRunTR(t, `taşı("v", "w")`); got != "w" || exists(filepath.Join(k, "v")) || content(t, filepath.Join(k, "w", "alt", "derin", "d.npy")) != "x" {
		t.Fatalf("başka diske klasör: %s", got)
	}
	write(t, filepath.Join(k, "a.txt"), "dosya")
	mustRunTR(t, `taşı("a.txt", "w")`)
	if exists(filepath.Join(k, "a.txt")) || content(t, filepath.Join(k, "w", "a.txt")) != "dosya" {
		t.Fatal("başka diske dosya")
	}
	// Kaynakta bağlantı varsa başka diske taşınmaz, kaynağa dokunulmaz.
	write(t, filepath.Join(k, "dis", "x.txt"), "x")
	write(t, filepath.Join(k, "b", "y.txt"), "y")
	dirLink(t, filepath.Join(k, "dis"), filepath.Join(k, "b", "bag"))
	errContains(t, `taşı("b", "c")`, "başka bir diske taşınamaz")
	if !exists(filepath.Join(k, "b", "y.txt")) || exists(filepath.Join(k, "c")) {
		t.Fatal("başarısız taşıma kaynağı ya da hedefi değiştirdi")
	}
}

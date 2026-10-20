package files

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/messages"
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
	errHas(t, `kopyala("a.txt", "a.txt", üzerine_yaz: doğru)`, messages.H7113)
	errContains(t, `kopyala("a.txt", "olmayan/")`, `hedef klasör "olmayan/" yok; önce klasör_oluştur("olmayan")`)
	errHas(t, `kopyala("a.txt", "olmayan/b.txt")`, messages.H2131)
	errContains(t, `kopyala("yok.txt", "c.txt")`, `"yok.txt" bulunamadı`)
	errHas(t, `kopyala("a.txt", "b.txt", overwrite: doğru)`, messages.H7900)
	// Dosya klasörün, klasör dosyanın yerine konmaz.
	os.MkdirAll(filepath.Join(k, "h", "a.txt"), 0o755)
	errHas(t, `kopyala("a.txt", "h", üzerine_yaz: doğru)`, messages.H7117)
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
	errHas(t, `kopyala("v", "v/alt")`, messages.H7114)
	errHas(t, `kopyala("v/bag", "x")`, messages.H7107)
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
	errHas(t, `kopyala("alt/`+name+`", "`+forwardPath(filepath.Dir(k))+`", üzerine_yaz: doğru)`, messages.H7119)
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
	errHas(t, `taşı("c.txt", "b.txt")`, messages.H7115)
	mustRunTR(t, `taşı("c.txt", "b.txt", üzerine_yaz: doğru)`)
	if content(t, filepath.Join(k, "b.txt")) != "c" {
		t.Fatal("üzerine taşınmadı")
	}
	errHas(t, `taşı(".", "x")`, messages.H7109, messages.H7151)
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
	errHas(t, `taşı("b", "c")`, messages.H7101)
	if !exists(filepath.Join(k, "b", "y.txt")) || exists(filepath.Join(k, "c")) {
		t.Fatal("başarısız taşıma kaynağı ya da hedefi değiştirdi")
	}
	// Kaynakta salt okunur dosya varsa (kaynak silinemezdi) hedefe dokunmadan durulur; iki
	// kopya kalmaz.
	write(t, filepath.Join(k, "ro", "kilitli.txt"), "r")
	locked := filepath.Join(k, "ro", "kilitli.txt")
	os.Chmod(locked, 0o444)
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	errHas(t, `taşı("ro", "ro2")`, messages.H7104)
	if !exists(locked) || exists(filepath.Join(k, "ro2")) {
		t.Fatal("silinemeyecek kaynak başka diske kopyalandı")
	}
}

// Yerine konacak klasörde salt okunur dosya varsa üzerine yazma başlamadan durur: eskiden
// yeni klasör konuyor, eskisi silinemeyip gizli bir .eski klasörü olarak kalıyordu.
func TestOverwriteDirWithReadOnlyFile(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "yeni", "a.txt"), "yeni")
	write(t, filepath.Join(k, "hedef", "yeni", "kilitli.txt"), "eski") // hedef/yeni yerine konacak
	locked := filepath.Join(k, "hedef", "yeni", "kilitli.txt")
	os.Chmod(locked, 0o444)
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	errHas(t, `kopyala("yeni", "hedef", üzerine_yaz: doğru)`, messages.H7104)
	if content(t, locked) != "eski" || exists(filepath.Join(k, "hedef", "yeni", "a.txt")) {
		t.Fatal("hedef değişti")
	}
	entries, _ := os.ReadDir(filepath.Join(k, "hedef"))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".eski") || strings.Contains(e.Name(), ".kopyalaniyor") {
			t.Fatalf("artık klasör kaldı: %s", e.Name())
		}
	}
}

// Bu programda bir satır okuyucusuyla açık dosya taşınmaz: işlem başlamadan, ne yapılacağını
// söyleyen bir hata verilir ve dosya yerinde kalır (Windows'ta taşıma yarıda kalırdı).
func TestMoveOpenReader(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "k", "s.txt"), "a\nb\nc\n")
	s := runTR(t, "", `değişken o = dosya_satırları("k/s.txt"); o.sonraki(); taşı("k", "w")`)
	if !messages.Has(s.failure, messages.H7154) {
		t.Fatalf("açık dosya: %v", s.failure)
	}
	CloseAll()
	if !exists(filepath.Join(k, "k", "s.txt")) || exists(filepath.Join(k, "w")) {
		t.Fatal("açık dosyalı klasör taşındı")
	}
	// Okuyucu kapanınca aynı taşıma çalışır.
	if got := mustRunTR(t, `taşı("k", "w")`); got != "w" || !exists(filepath.Join(k, "w", "s.txt")) {
		t.Fatalf("okuyucu kapandıktan sonra taşıma: %s", got)
	}
}

package dosya

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func icerik(t *testing.T, yol string) string {
	t.Helper()
	b, err := os.ReadFile(yol)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestKopyalaDosya(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "a.txt"), "bir")
	eski := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(filepath.Join(k, "a.txt"), eski, eski)
	os.Mkdir(filepath.Join(k, "yedek"), 0o755)
	if got := tamam(t, `kopyala("a.txt", "yedek")`); got != "yedek/a.txt" {
		t.Fatalf("klasörün içine: %s", got)
	}
	if b, _ := os.Stat(filepath.Join(k, "yedek", "a.txt")); !b.ModTime().Equal(eski) {
		t.Errorf("değiştirilme zamanı korunmadı: %v", b.ModTime())
	}
	hataIcerir(t, `kopyala("a.txt", "yedek/")`, `hedef "yedek/a.txt" zaten var`)
	tamam(t, `kopyala(yol("a.txt"), yol("b.txt"))`)
	if icerik(t, filepath.Join(k, "b.txt")) != "bir" {
		t.Fatal("yeni adla kopya")
	}
	yaz(t, filepath.Join(k, "a.txt"), "iki")
	hataIcerir(t, `kopyala("a.txt", "b.txt")`, `kopyala(): hedef "b.txt" zaten var; yerine koymak için üzerine_yaz: doğru verin`)
	tamam(t, `kopyala("a.txt", "b.txt", üzerine_yaz: doğru)`)
	if icerik(t, filepath.Join(k, "b.txt")) != "iki" {
		t.Fatal("üzerine yazılmadı")
	}
	hataIcerir(t, `kopyala("a.txt", "a.txt", üzerine_yaz: doğru)`, "kaynak ile hedef aynı")
	hataIcerir(t, `kopyala("a.txt", "olmayan/")`, `hedef klasör "olmayan/" yok; önce klasör_oluştur("olmayan")`)
	hataIcerir(t, `kopyala("a.txt", "olmayan/b.txt")`, `"olmayan" klasörü yok`)
	hataIcerir(t, `kopyala("yok.txt", "c.txt")`, `"yok.txt" bulunamadı`)
	hataIcerir(t, `kopyala("a.txt", "b.txt", overwrite: doğru)`, "Bunu mu demek istediniz: 'üzerine_yaz'?")
	// Dosya klasörün, klasör dosyanın yerine konmaz.
	os.MkdirAll(filepath.Join(k, "h", "a.txt"), 0o755)
	hataIcerir(t, `kopyala("a.txt", "h", üzerine_yaz: doğru)`, "bir klasör, kaynak bir dosya")
	if entries, _ := os.ReadDir(k); len(entries) != 4 {
		t.Errorf("geçici dosya kaldı: %v", entries)
	}
}

func TestKopyalaKlasor(t *testing.T) {
	k := geciciCalisma(t)
	agacKur(t, k)
	yaz(t, filepath.Join(k, "dis", "x.txt"), "x")
	klasorBaglantisi(t, filepath.Join(k, "dis"), filepath.Join(k, "v", "bag"))
	s := trCalistir(t, "", `kopyala("v", "v2")`)
	if s.hata != nil || s.deger.Inspect() != "v2" {
		t.Fatalf("%v %v", s.deger, s.hata)
	}
	if !strings.Contains(s.uyari, "1 bağlantı ya da özel dosya kopyalanmadı") {
		t.Errorf("uyarı: %q", s.uyari)
	}
	if icerik(t, filepath.Join(k, "v2", "alt", "derin", "d.npy")) != "x" || !varMi(filepath.Join(k, "v2", ".gizli", "e.npy")) || varMi(filepath.Join(k, "v2", "bag")) {
		t.Fatal("ağaç eksik kopyalandı ya da bağlantı kopyalandı")
	}
	hataIcerir(t, `kopyala("v", "v/alt")`, "kendi içine")
	hataIcerir(t, `kopyala("v/bag", "x")`, "bir bağlantı; TR bağlantıları izlemez ve kopyalamaz")
	// Var olan klasörün yerine konma: eski içerik gider, yenisi gelir.
	yaz(t, filepath.Join(k, "hedef", "v", "eski.txt"), "e")
	tamam(t, `kopyala("v", "hedef", üzerine_yaz: doğru)`)
	if varMi(filepath.Join(k, "hedef", "v", "eski.txt")) || !varMi(filepath.Join(k, "hedef", "v", "a.npy")) {
		t.Fatal("klasör yerine konmadı")
	}
	if g, _ := filepath.Glob(filepath.Join(k, "hedef", ".*")); len(g) != 0 {
		t.Errorf("geçici klasör kaldı: %v", g)
	}
	// Çalışma klasörünün yerine bir şey konmaz.
	ad := filepath.Base(k)
	yaz(t, filepath.Join(k, "alt", ad, "x.txt"), "x")
	hataIcerir(t, `kopyala("alt/`+ad+`", "`+ileriYol(filepath.Dir(k))+`", üzerine_yaz: doğru)`, "kaynağı içeriyor")
}

func TestTasi(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "a.txt"), "x")
	if got := tamam(t, `taşı("a.txt", "b.txt")`); got != "b.txt" || varMi(filepath.Join(k, "a.txt")) {
		t.Fatalf("yeniden adlandırma: %s", got)
	}
	agacKur(t, k)
	if got := tamam(t, `taşı("v", "w")`); got != "w" || varMi(filepath.Join(k, "v")) || !varMi(filepath.Join(k, "w", "alt", "c.npy")) {
		t.Fatalf("klasör: %s", got)
	}
	yaz(t, filepath.Join(k, "c.txt"), "c")
	hataIcerir(t, `taşı("c.txt", "b.txt")`, "zaten var")
	tamam(t, `taşı("c.txt", "b.txt", üzerine_yaz: doğru)`)
	if icerik(t, filepath.Join(k, "b.txt")) != "c" {
		t.Fatal("üzerine taşınmadı")
	}
	hataIcerir(t, `taşı(".", "x")`, "çalışma klasörü", "yerinden kaldırmaz")
}

// Başka bir diske taşıma: yeniden adlandırma "farklı aygıt" hatası verince kopyalanır,
// doğrulanır, kaynak silinir.
func TestTasiBaskaDiske(t *testing.T) {
	k := geciciCalisma(t)
	eski := yenidenAdlandir
	t.Cleanup(func() { yenidenAdlandir = eski })
	no := syscall.EXDEV
	if runtime.GOOS == "windows" {
		no = syscall.Errno(17)
	}
	yenidenAdlandir = func(a, b string) error {
		if strings.Contains(a, ".kopyalaniyor") || strings.Contains(a, ".eski") || strings.Contains(b, ".eski") {
			return eski(a, b)
		}
		return &os.LinkError{Op: "rename", Old: a, New: b, Err: no}
	}
	agacKur(t, k)
	if got := tamam(t, `taşı("v", "w")`); got != "w" || varMi(filepath.Join(k, "v")) || icerik(t, filepath.Join(k, "w", "alt", "derin", "d.npy")) != "x" {
		t.Fatalf("başka diske klasör: %s", got)
	}
	yaz(t, filepath.Join(k, "a.txt"), "dosya")
	tamam(t, `taşı("a.txt", "w")`)
	if varMi(filepath.Join(k, "a.txt")) || icerik(t, filepath.Join(k, "w", "a.txt")) != "dosya" {
		t.Fatal("başka diske dosya")
	}
	// Kaynakta bağlantı varsa başka diske taşınmaz, kaynağa dokunulmaz.
	yaz(t, filepath.Join(k, "dis", "x.txt"), "x")
	yaz(t, filepath.Join(k, "b", "y.txt"), "y")
	klasorBaglantisi(t, filepath.Join(k, "dis"), filepath.Join(k, "b", "bag"))
	hataIcerir(t, `taşı("b", "c")`, "başka bir diske taşınamaz")
	if !varMi(filepath.Join(k, "b", "y.txt")) || varMi(filepath.Join(k, "c")) {
		t.Fatal("başarısız taşıma kaynağı ya da hedefi değiştirdi")
	}
}

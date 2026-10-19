package files

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDeleteBasic(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "a.txt"), "x")
	if got := mustRunTR(t, `sil("a.txt")`); got != "1" || exists(filepath.Join(k, "a.txt")) {
		t.Fatalf("dosya: %s", got)
	}
	os.Mkdir(filepath.Join(k, "bos"), 0o755)
	if got := mustRunTR(t, `sil(yol("bos"))`); got != "0" || exists(filepath.Join(k, "bos")) {
		t.Fatalf("boş klasör: %s", got)
	}
	buildTree(t, k)
	errContains(t, `sil("v")`, `sil(): "v" klasörü boş değil; içindekilerle birlikte silmek için içindekilerle: doğru verin`)
	if got := mustRunTR(t, `sil("v", içindekilerle: doğru)`); got != "7" || exists(filepath.Join(k, "v")) {
		t.Fatalf("dolu klasör: %s", got)
	}
	errContains(t, `sil("yok.txt")`, `sil(): "yok.txt" silinemedi: böyle bir dosya ya da klasör yok`, "bakılan yer")
	errContains(t, `sil("*.txt")`, "silinemedi")
	errContains(t, `sil("a", içindekilerle: 1)`, "doğru ya da yanlış olmalıdır")
	errContains(t, `sil("a", recursive: doğru)`, "Bunu mu demek istediniz: 'içindekilerle'?")
}

// Salt okunur dosya zorla silinmez; klasörde böyle bir dosya varsa hiçbir şey silinmez.
func TestDeleteReadOnly(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "k", "a.txt"), "x")
	write(t, filepath.Join(k, "k", "z", "kilitli.txt"), "x")
	locked := filepath.Join(k, "k", "z", "kilitli.txt")
	os.Chmod(locked, 0o444)
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	errContains(t, `sil("k/z/kilitli.txt")`, "salt okunur bir dosya; TR salt okunur dosyaları zorla silmez")
	errContains(t, `sil("k", içindekilerle: doğru)`, `içinde salt okunur bir dosya var ("z/kilitli.txt")`, "hiçbir şey silinmedi")
	if !exists(filepath.Join(k, "k", "a.txt")) || !exists(locked) {
		t.Fatal("salt okunur dosya varken bir şey silindi")
	}
}

// Bağlantılar izlenmez: klasörün içindeki ya da silinen yolun kendisi olan bir bağlantının
// gösterdiği yere dokunulmaz.
func TestDeleteDoesNotFollowLinks(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "dis", "degerli.txt"), "x")
	write(t, filepath.Join(k, "k", "a.txt"), "x")
	dirLink(t, filepath.Join(k, "dis"), filepath.Join(k, "k", "bag"))
	if got := mustRunTR(t, `sil("k", içindekilerle: doğru)`); got != "2" {
		t.Errorf("sayı: %s", got)
	}
	if !exists(filepath.Join(k, "dis", "degerli.txt")) {
		t.Fatal("bağlantının gösterdiği klasör silindi")
	}
	dirLink(t, filepath.Join(k, "dis"), filepath.Join(k, "bag2"))
	if got := mustRunTR(t, `sil("bag2", içindekilerle: doğru)`); got != "1" || exists(filepath.Join(k, "bag2")) || !exists(filepath.Join(k, "dis", "degerli.txt")) {
		t.Fatalf("bağlantının kendisi silinmeli, gösterdiği yer kalmalı: %s", got)
	}
}

// Windows dizin bağlantıları (junction) da bağlantı sayılır (sembolik bağlantı kurma yetkisi
// olsa da olmasa da ayrıca sınanır).
func TestDeleteWindowsJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("yalnızca Windows")
	}
	k := tmpWork(t)
	write(t, filepath.Join(k, "dis", "degerli.txt"), "x")
	write(t, filepath.Join(k, "k", "a.txt"), "x")
	for _, name := range []string{filepath.Join(k, "k", "bag"), filepath.Join(k, "bag2")} {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", name, filepath.Join(k, "dis")).CombinedOutput(); err != nil {
			t.Skipf("dizin bağlantısı kurulamadı: %v %s", err, out)
		}
	}
	if b, _ := os.Lstat(filepath.Join(k, "bag2")); !isLink(b.Mode()) {
		t.Fatalf("dizin bağlantısı bağlantı sayılmadı: %v", b.Mode())
	}
	mustRunTR(t, `sil("k", içindekilerle: doğru)`)
	mustRunTR(t, `sil("bag2")`)
	if !exists(filepath.Join(k, "dis", "degerli.txt")) || exists(filepath.Join(k, "bag2")) {
		t.Fatal("dizin bağlantısının gösterdiği klasör silindi ya da bağlantı kaldı")
	}
}

// Korumalar: hiçbir durumda içindekilerle verilmez; koruma bozulsa bile işlem "boş değil"
// hatasıyla durur, hiçbir şey silinmez.
func TestDeleteProtections(t *testing.T) {
	k := tmpWork(t)
	program := filepath.Join(k, "proje", "betikler")
	os.MkdirAll(program, 0o755)
	write(t, filepath.Join(program, "x.txt"), "x")
	home, _ := os.UserHomeDir()
	cases := map[string]string{
		`sil(".")`:                            "çalışma klasörü ya da onun bir üst klasörü",
		`sil("..")`:                           "çalışma klasörü ya da onun bir üst klasörü",
		`sil("` + forwardPath(k) + `")`:       "çalışma klasörü",
		`sil("` + forwardPath(program) + `")`: "programın (ya da modülün) klasörü",
		`sil("` + forwardPath(filepath.Dir(program)) + `")`:                             "programın (ya da modülün) klasörü ya da onun bir üst klasörü",
		`sil("` + forwardPath(filepath.VolumeName(k)+string(filepath.Separator)) + `")`: "bir kök klasör",
	}
	if home != "" {
		cases[`sil("`+forwardPath(home)+`")`] = "ev klasörünüzün kendisi"
		cases[`sil("`+forwardPath(filepath.Dir(home))+`")`] = "ev klasörünüzün bir üst klasörü"
	}
	for code, wants := range cases {
		s := runTR(t, program, code)
		if s.failure == nil || !strings.Contains(s.failure.Error(), wants) || !strings.Contains(s.failure.Error(), "TR bunu silmez") {
			t.Errorf("%s: %q bekleniyordu: %v", code, wants, s.failure)
		}
	}
	if !exists(filepath.Join(program, "x.txt")) {
		t.Fatal("korunan klasörden bir şey silindi")
	}
}

// Sistem klasörleri ve üstleri: koruma işlevi doğrudan sınanır (silme denenmez).
func TestSystemDirProtection(t *testing.T) {
	dirs := systemDirs()
	if len(dirs) == 0 {
		t.Skip("sistem klasörü bilinmiyor")
	}
	for _, s := range dirs {
		g := resolvedDir(s)
		if n := deleteProtection(nil, g); n == "" {
			t.Errorf("%s korunmuyor", g)
		}
		if parent := filepath.Dir(g); parent != g {
			if n := deleteProtection(nil, parent); n == "" {
				t.Errorf("%s (üst) korunmuyor", parent)
			}
		}
	}
	if n := deleteProtection(nil, resolvedDir(t.TempDir())); n != "" {
		t.Errorf("geçici klasör korunmamalı: %s", n)
	}
}

// Bu programda bir satır okuyucusuyla açık dosya silinmez (Windows'ta silme yarıda kalırdı).
func TestDeleteOpenReader(t *testing.T) {
	k := tmpWork(t)
	write(t, filepath.Join(k, "k", "s.txt"), "a\nb\nc\n")
	s := runTR(t, "", `değişken o = dosya_satırları("k/s.txt"); o.sonraki(); sil("k", içindekilerle: doğru)`)
	if s.failure == nil || !strings.Contains(s.failure.Error(), "SatırOkuyucu ile okunuyor") {
		t.Fatalf("açık dosya: %v", s.failure)
	}
	CloseAll()
	if !exists(filepath.Join(k, "k", "s.txt")) {
		t.Fatal("açık dosyalı klasör silindi")
	}
}

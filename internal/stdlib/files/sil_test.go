package files

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSilTemel(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "a.txt"), "x")
	if got := tamam(t, `sil("a.txt")`); got != "1" || varMi(filepath.Join(k, "a.txt")) {
		t.Fatalf("dosya: %s", got)
	}
	os.Mkdir(filepath.Join(k, "bos"), 0o755)
	if got := tamam(t, `sil(yol("bos"))`); got != "0" || varMi(filepath.Join(k, "bos")) {
		t.Fatalf("boş klasör: %s", got)
	}
	agacKur(t, k)
	hataIcerir(t, `sil("v")`, `sil(): "v" klasörü boş değil; içindekilerle birlikte silmek için içindekilerle: doğru verin`)
	if got := tamam(t, `sil("v", içindekilerle: doğru)`); got != "7" || varMi(filepath.Join(k, "v")) {
		t.Fatalf("dolu klasör: %s", got)
	}
	hataIcerir(t, `sil("yok.txt")`, `sil(): "yok.txt" silinemedi: böyle bir dosya ya da klasör yok`, "bakılan yer")
	hataIcerir(t, `sil("*.txt")`, "silinemedi")
	hataIcerir(t, `sil("a", içindekilerle: 1)`, "doğru ya da yanlış olmalıdır")
	hataIcerir(t, `sil("a", recursive: doğru)`, "Bunu mu demek istediniz: 'içindekilerle'?")
}

// Salt okunur dosya zorla silinmez; klasörde böyle bir dosya varsa hiçbir şey silinmez.
func TestSilSaltOkunur(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "k", "a.txt"), "x")
	yaz(t, filepath.Join(k, "k", "z", "kilitli.txt"), "x")
	kilitli := filepath.Join(k, "k", "z", "kilitli.txt")
	os.Chmod(kilitli, 0o444)
	t.Cleanup(func() { os.Chmod(kilitli, 0o644) })
	hataIcerir(t, `sil("k/z/kilitli.txt")`, "salt okunur bir dosya; TR salt okunur dosyaları zorla silmez")
	hataIcerir(t, `sil("k", içindekilerle: doğru)`, `içinde salt okunur bir dosya var ("z/kilitli.txt")`, "hiçbir şey silinmedi")
	if !varMi(filepath.Join(k, "k", "a.txt")) || !varMi(kilitli) {
		t.Fatal("salt okunur dosya varken bir şey silindi")
	}
}

// Bağlantılar izlenmez: klasörün içindeki ya da silinen yolun kendisi olan bir bağlantının
// gösterdiği yere dokunulmaz.
func TestSilBaglantiIzlemez(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "dis", "degerli.txt"), "x")
	yaz(t, filepath.Join(k, "k", "a.txt"), "x")
	klasorBaglantisi(t, filepath.Join(k, "dis"), filepath.Join(k, "k", "bag"))
	if got := tamam(t, `sil("k", içindekilerle: doğru)`); got != "2" {
		t.Errorf("sayı: %s", got)
	}
	if !varMi(filepath.Join(k, "dis", "degerli.txt")) {
		t.Fatal("bağlantının gösterdiği klasör silindi")
	}
	klasorBaglantisi(t, filepath.Join(k, "dis"), filepath.Join(k, "bag2"))
	if got := tamam(t, `sil("bag2", içindekilerle: doğru)`); got != "1" || varMi(filepath.Join(k, "bag2")) || !varMi(filepath.Join(k, "dis", "degerli.txt")) {
		t.Fatalf("bağlantının kendisi silinmeli, gösterdiği yer kalmalı: %s", got)
	}
}

// Windows dizin bağlantıları (junction) da bağlantı sayılır (sembolik bağlantı kurma yetkisi
// olsa da olmasa da ayrıca sınanır).
func TestSilWindowsDizinBaglantisi(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("yalnızca Windows")
	}
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "dis", "degerli.txt"), "x")
	yaz(t, filepath.Join(k, "k", "a.txt"), "x")
	for _, ad := range []string{filepath.Join(k, "k", "bag"), filepath.Join(k, "bag2")} {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", ad, filepath.Join(k, "dis")).CombinedOutput(); err != nil {
			t.Skipf("dizin bağlantısı kurulamadı: %v %s", err, out)
		}
	}
	if b, _ := os.Lstat(filepath.Join(k, "bag2")); !baglantiMi(b.Mode()) {
		t.Fatalf("dizin bağlantısı bağlantı sayılmadı: %v", b.Mode())
	}
	tamam(t, `sil("k", içindekilerle: doğru)`)
	tamam(t, `sil("bag2")`)
	if !varMi(filepath.Join(k, "dis", "degerli.txt")) || varMi(filepath.Join(k, "bag2")) {
		t.Fatal("dizin bağlantısının gösterdiği klasör silindi ya da bağlantı kaldı")
	}
}

// Korumalar: hiçbir durumda içindekilerle verilmez; koruma bozulsa bile işlem "boş değil"
// hatasıyla durur, hiçbir şey silinmez.
func TestSilKorumalari(t *testing.T) {
	k := geciciCalisma(t)
	program := filepath.Join(k, "proje", "betikler")
	os.MkdirAll(program, 0o755)
	yaz(t, filepath.Join(program, "x.txt"), "x")
	ev, _ := os.UserHomeDir()
	durumlar := map[string]string{
		`sil(".")`:                         "çalışma klasörü ya da onun bir üst klasörü",
		`sil("..")`:                        "çalışma klasörü ya da onun bir üst klasörü",
		`sil("` + ileriYol(k) + `")`:       "çalışma klasörü",
		`sil("` + ileriYol(program) + `")`: "programın (ya da modülün) klasörü",
		`sil("` + ileriYol(filepath.Dir(program)) + `")`:                             "programın (ya da modülün) klasörü ya da onun bir üst klasörü",
		`sil("` + ileriYol(filepath.VolumeName(k)+string(filepath.Separator)) + `")`: "bir kök klasör",
	}
	if ev != "" {
		durumlar[`sil("`+ileriYol(ev)+`")`] = "ev klasörünüzün kendisi"
		durumlar[`sil("`+ileriYol(filepath.Dir(ev))+`")`] = "ev klasörünüzün bir üst klasörü"
	}
	for kod, ister := range durumlar {
		s := trCalistir(t, program, kod)
		if s.hata == nil || !strings.Contains(s.hata.Error(), ister) || !strings.Contains(s.hata.Error(), "TR bunu silmez") {
			t.Errorf("%s: %q bekleniyordu: %v", kod, ister, s.hata)
		}
	}
	if !varMi(filepath.Join(program, "x.txt")) {
		t.Fatal("korunan klasörden bir şey silindi")
	}
}

// Sistem klasörleri ve üstleri: koruma işlevi doğrudan sınanır (silme denenmez).
func TestSistemKlasoruKorumasi(t *testing.T) {
	klasorler := sistemKlasorleri()
	if len(klasorler) == 0 {
		t.Skip("sistem klasörü bilinmiyor")
	}
	for _, s := range klasorler {
		g := cozulmusKlasor(s)
		if n := silmeKorumasi(nil, g); n == "" {
			t.Errorf("%s korunmuyor", g)
		}
		if ust := filepath.Dir(g); ust != g {
			if n := silmeKorumasi(nil, ust); n == "" {
				t.Errorf("%s (üst) korunmuyor", ust)
			}
		}
	}
	if n := silmeKorumasi(nil, cozulmusKlasor(t.TempDir())); n != "" {
		t.Errorf("geçici klasör korunmamalı: %s", n)
	}
}

// Bu programda bir satır okuyucusuyla açık dosya silinmez (Windows'ta silme yarıda kalırdı).
func TestSilAcikOkuyucu(t *testing.T) {
	k := geciciCalisma(t)
	yaz(t, filepath.Join(k, "k", "s.txt"), "a\nb\nc\n")
	s := trCalistir(t, "", `değişken o = dosya_satırları("k/s.txt"); o.sonraki(); sil("k", içindekilerle: doğru)`)
	if s.hata == nil || !strings.Contains(s.hata.Error(), "SatırOkuyucu ile okunuyor") {
		t.Fatalf("açık dosya: %v", s.hata)
	}
	TumunuKapat()
	if !varMi(filepath.Join(k, "k", "s.txt")) {
		t.Fatal("açık dosyalı klasör silindi")
	}
}

package files

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// geciciTaban işletim sisteminin geçici klasörünü test bitene kadar yeni, boş bir klasör yapar
// ve onu verir (geçici_klasör()'ün oluşturduklarını saymak için).
func geciciTaban(t *testing.T) string {
	t.Helper()
	taban := t.TempDir()
	for _, ad := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(ad, taban)
	}
	if cozulmusKlasor(os.TempDir()) != cozulmusKlasor(taban) {
		t.Skipf("geçici klasör ortam değişkeniyle değiştirilemedi: %s", os.TempDir())
	}
	return taban
}

func tabanIcerigi(t *testing.T, taban string) []string {
	t.Helper()
	girdiler, err := os.ReadDir(taban)
	if err != nil {
		t.Fatal(err)
	}
	var adlar []string
	for _, g := range girdiler {
		adlar = append(adlar, g.Name())
	}
	return adlar
}

// 18.14: geçici_klasör() geçici klasörün altında yeni, boş, yalnızca kullanıcıya açık bir
// klasör verir; her çağrı ayrı bir klasördür ve program bitince içindekilerle silinir.
func TestGeciciKlasor(t *testing.T) {
	geciciCalisma(t)
	taban := geciciTaban(t)
	got := tamam(t, `değişken k = geçici_klasör()
		değişken bos = klasörü_listele(k).uzunluk
		dosya_yaz(yol(k, "a.txt"), "x")
		klasör_oluştur(yol(k, "alt/derin"))
		dosya_yaz(yol(k, "alt/derin/b.txt"), "y")
		değişken o = dosya_satırları(yol(k, "alt/derin/b.txt"))
		[tip(k) == Yol, k.klasör_mü, k.mutlak == k, bos, k != geçici_klasör()]`)
	if got != `[doğru, doğru, doğru, 0, doğru]` {
		t.Fatal(got)
	}
	adlar := tabanIcerigi(t, taban)
	if len(adlar) != 2 {
		t.Fatalf("iki geçici klasör bekleniyordu: %v", adlar)
	}
	onek := "tr_" + strconv.Itoa(os.Getpid()) + "_"
	for _, ad := range adlar {
		if !strings.HasPrefix(ad, onek) {
			t.Errorf("%q adı %q ile başlamalı", ad, onek)
		}
		if b, err := os.Stat(filepath.Join(taban, ad)); err != nil || (runtime.GOOS != "windows" && b.Mode().Perm() != 0o700) {
			t.Errorf("%s yalnızca kullanıcıya açık olmalı: %v %v", ad, b.Mode(), err)
		}
	}
	// Açık bir satır okuyucusu da olsa (önce kapatılır) klasörler silinir.
	TumunuKapat()
	if kalan := tabanIcerigi(t, taban); len(kalan) != 0 {
		t.Fatalf("program bitince geçici klasör kalmamalı: %v", kalan)
	}
	TumunuKapat() // ikinci kez çağırmak sorun değildir
	hataIcerir(t, `geçici_klasör(1)`, "geçici_klasör() 0 argüman bekler, 1 verildi")
}

// Program geçici klasörü kendisi silmiş ya da taşımışsa program sonunda hata yoktur.
func TestGeciciKlasorProgramSilmis(t *testing.T) {
	geciciCalisma(t)
	taban := geciciTaban(t)
	tamam(t, `değişken k = geçici_klasör(); sil(k); taşı(geçici_klasör(), "tasindi")`)
	if hatalar := geciciKlasorleriSil(); len(hatalar) != 0 {
		t.Fatal(hatalar)
	}
	if kalan := tabanIcerigi(t, taban); len(kalan) != 0 {
		t.Fatalf("taban boş olmalı: %v", kalan)
	}
}

// Geçici klasör silinirken bağlantılar izlenmez: bağlantının gösterdiği klasöre dokunulmaz.
func TestGeciciKlasorBaglantiIzlemez(t *testing.T) {
	k := geciciCalisma(t)
	geciciTaban(t)
	dis := filepath.Join(k, "dis")
	yaz(t, filepath.Join(dis, "korunan.txt"), "x")
	s := trCalistir(t, "", `geçici_klasör()`)
	if s.hata != nil {
		t.Fatal(s.hata)
	}
	gecici := filepath.FromSlash(s.deger.Inspect())
	klasorBaglantisi(t, dis, filepath.Join(gecici, "baglanti"))
	if hatalar := geciciKlasorleriSil(); len(hatalar) != 0 {
		t.Fatal(hatalar)
	}
	if varMi(gecici) {
		t.Error("geçici klasör silinmeli")
	}
	if !varMi(filepath.Join(dis, "korunan.txt")) {
		t.Fatal("bağlantının gösterdiği klasördeki dosya silindi")
	}
}

// Salt okunur bir dosya bulunan geçici klasör zorla silinmez (sil'in kuralı); uyarı döner.
func TestGeciciKlasorSaltOkunur(t *testing.T) {
	geciciCalisma(t)
	geciciTaban(t)
	s := trCalistir(t, "", `değişken k = geçici_klasör(); dosya_yaz(yol(k, "s.txt"), "x"); k`)
	if s.hata != nil {
		t.Fatal(s.hata)
	}
	dosya := filepath.Join(filepath.FromSlash(s.deger.Inspect()), "s.txt")
	if err := os.Chmod(dosya, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dosya, 0o644) })
	hatalar := geciciKlasorleriSil()
	if len(hatalar) != 1 || !strings.Contains(hatalar[0].Error(), "geçici_klasör(): ") ||
		!strings.Contains(hatalar[0].Error(), "salt okunur") {
		t.Fatalf("salt okunur dosya uyarısı bekleniyordu: %v", hatalar)
	}
}

package interp

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// kitaplikla f kitaplığını test boyunca bağlar.
func kitaplikla(t *testing.T, f fstest.MapFS) {
	t.Helper()
	onceki := kitaplikDosyalari
	SetLibrary(f)
	t.Cleanup(func() { SetLibrary(onceki) })
}

// Güvenlik bulgusu O2: göreli 'aktar' çalışma klasöründe aynı adlı bir dosya varken
// başka klasördeki programın (ya da modülün) yanındaki dosyayı değil onu yüklüyordu. Kural
// 11.1: önce aktaran dosyanın klasörü, sonra programın klasörü; çalışma klasörüne bakılmaz.
func TestAktarCalismaKlasorunuKullanmaz(t *testing.T) {
	kok := t.TempDir()
	calisma := filepath.Join(kok, "scratch")
	dosyaYaz(t, filepath.Join(calisma, "yardimci.tr"), "değişken kimden = \"çalışma klasörü\"\n")
	dosyaYaz(t, filepath.Join(calisma, "ozel.tr"), "değişken ozel = \"çalışma klasörü\"\n")
	klasordeCalis(t, calisma)
	prog := filepath.Join(kok, "program")
	out, err := modulluCalistir(t, prog, map[string]string{
		"yardimci.tr":     "değişken kimden = \"programın yanı\"\n",
		"alt/modul.tr":    "aktar \"yardimci\"\ndeğişken alt_kimden = kimden\n",
		"alt/yardimci.tr": "değişken kimden = \"alt modülün yanı\"\n",
		"ana.tr": "aktar \"yardimci\"\nyazdır(kimden)\naktar \"alt/modul\" olarak m\nyazdır(m.alt_kimden)\n" +
			"dene { aktar \"ozel\" } yakala h { yazdır(h) }\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	satirlar := strings.Split(strings.TrimSpace(out), "\n")
	if len(satirlar) != 3 || satirlar[0] != "programın yanı" || satirlar[1] != "alt modülün yanı" {
		t.Fatalf("modüller aktaranın klasöründen yüklenmeli:\n%s", out)
	}
	// Yalnızca çalışma klasöründe olan modül bulunmaz; hata bakılan yerleri söyler.
	if !strings.Contains(satirlar[2], "bulunamadı") && !strings.Contains(satirlar[2], "yok") || strings.Contains(satirlar[2], calisma) {
		t.Fatalf("çalışma klasöründeki modül yüklenmemeli, hata yalnızca programın klasörünü göstermeli:\n%s", satirlar[2])
	}
}

// Kural 11.6: "tr/" önekli yol dilin gömülü kitaplığındandır; diskteki tr/ klasörü onu
// gölgeleyemez, "./tr/" ise diskteki klasördür. Kitaplık modülünün içindeki göreli aktar
// kitaplıkta çözülür; olmayan modülün hatası kitaplıktakileri ve en yakın adı söyler.
func TestKitaplikOneki(t *testing.T) {
	kitaplikla(t, fstest.MapFS{
		"katman.tr":       {Data: []byte("aktar \"ortak\"\ndeğişken kaynak = \"kitaplık\" + ek\n")},
		"ortak.tr":        {Data: []byte("değişken ek = \"!\"\n")},
		"iyilestirici.tr": {Data: []byte("değişken iy = 1\n")},
		"katman_test.tr":  {Data: []byte("test \"x\" { }\n")},
	})
	kok := t.TempDir()
	klasordeCalis(t, kok)
	out, err := modulluCalistir(t, kok, map[string]string{
		"tr/katman.tr": "değişken kaynak = \"disk\"\n",
		"ana.tr": "aktar \"tr/katman\" olarak k\nyazdır(k.kaynak)\naktar \"./tr/katman\" olarak d\nyazdır(d.kaynak)\n" +
			"dene { aktar \"tr/iyileştirici\" } yakala h { yazdır(h) }\n" +
			"dene { aktar \"tr/katman_test\" } yakala h { yazdır(h) }\n" +
			"dene { aktar \"tr/../ana\" } yakala h { yazdır(h) }\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	satirlar := strings.Split(strings.TrimSpace(out), "\n")
	if len(satirlar) != 5 || satirlar[0] != "kitaplık!" || satirlar[1] != "disk" {
		t.Fatalf("tr/ kitaplıktan, ./tr/ diskten yüklenmeli:\n%s", out)
	}
	if !strings.Contains(satirlar[2], `"tr/iyileştirici" dilin kitaplığında yok`) || !strings.Contains(satirlar[2], `Bunu mu demek istediniz: "tr/iyilestirici"?`) {
		t.Fatalf("en yakın kitaplık modülü önerilmeli:\n%s", satirlar[2])
	}
	for _, s := range satirlar[3:] {
		if !strings.Contains(s, "dilin kitaplığında yok; kitaplıktaki modüller: tr/iyilestirici, tr/katman, tr/ortak") {
			t.Fatalf("test dosyası ve kitaplık dışı yol modül değildir:\n%s", s)
		}
	}
}

// Kitaplık modülünün hatası "tr/..." adıyla ve kendi satırıyla yazılır; göreli dosya yolları
// kitaplığın (diskte olmayan) klasörüne göre aranmaz.
func TestKitaplikModuluHatasi(t *testing.T) {
	kitaplikla(t, fstest.MapFS{"m.tr": {Data: []byte("fonksiyon böl(a) {\n    sonuç a / 0\n}\n")}})
	kok := t.TempDir()
	klasordeCalis(t, kok)
	_, err := modulluCalistir(t, kok, map[string]string{"ana.tr": "aktar \"tr/m\"\nböl(1)\n"})
	if err == nil || !strings.Contains(err.Error(), "tr/m.tr, Satır 2: Sıfıra bölme hatası") {
		t.Fatalf("hata kitaplık modülünün adı ve satırıyla gelmeli: %v", err)
	}
}

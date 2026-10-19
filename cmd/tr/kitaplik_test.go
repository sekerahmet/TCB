package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Kural 11.1 (güvenlik bulgusu O2): başka klasörden çalıştırılan programın göreli 'aktar'ı
// çalışma klasöründeki aynı adlı dosyayı değil, programın yanındakini yükler.
func TestAktarCalismaKlasorundekiDosyayiYuklemez(t *testing.T) {
	proje := t.TempDir()
	dosyaYaz(t, proje, "ana.tr", "aktar \"yardimci\"\nyazdır(kaynak)\n")
	dosyaYaz(t, proje, "yardimci.tr", "değişken kaynak = \"programın yanı\"\n")
	calisma := t.TempDir()
	dosyaYaz(t, calisma, "yardimci.tr", "değişken kaynak = \"SAHTE\"\n")
	s := tr(t, calisma, filepath.Join(proje, "ana.tr"))
	bekle(t, s, 0, []string{"programın yanı"}, nil)
	if strings.Contains(s.cikti, "SAHTE") {
		t.Error("çalışma klasöründeki dosya programın modülünün yerine geçti")
	}
	// tr kontrol ve tr derle aynı kuralla çözer.
	bekle(t, tr(t, calisma, "kontrol", filepath.Join(proje, "ana.tr")), 0, nil, nil)
}

// Kural 11.6: tr/ önekli modül gömülü kitaplıktandır; tr kontrol olmayanı bildirir, tr derle
// kitaplığı pakete gömmez (uygulama yorumlayıcının kopyasıdır) ve uygulama başka klasörde,
// diskte tr/katman.tr varken de kitaplıktakini yükler.
func TestKitaplikModuluKontrolVeDerle(t *testing.T) {
	proje := t.TempDir()
	dosyaYaz(t, proje, "ana.tr", strings.Join([]string{
		`aktar "tr/katman" olarak k`,
		`aktar "yerel"`,
		`değişken d = k.Doğrusal(3, 2, tohum: 1)`,
		`yazdır("şekil {d.ileri(birler([1, 3])).şekil} {YEREL}")`,
	}, "\n")+"\n")
	dosyaYaz(t, proje, "yerel.tr", "değişken YEREL = \"yerel modül\"\n")
	dosyaYaz(t, proje, "tr/katman.tr", "değişken Doğrusal = yok\n")
	beklenen := "şekil [1, 2] yerel modül"
	bekle(t, tr(t, proje, "ana.tr"), 0, []string{beklenen}, nil)

	cikis := filepath.Join(t.TempDir(), "uygulama.exe")
	s := tr(t, proje, "derle", "ana.tr", cikis)
	bekle(t, s, 0, []string{"Başarıyla derlendi", "Gömülen modüller (1)", "yerel.tr"}, nil)
	if strings.Contains(s.cikti, "katman") {
		t.Errorf("kitaplık modülü pakete gömülmemeli:\n%s", s.cikti)
	}
	baska := t.TempDir()
	dosyaYaz(t, baska, "tr/katman.tr", "değişken Doğrusal = yok\n")
	bekle(t, calistir(t, cikis, baska, "", nil), 0, []string{beklenen}, nil)

	dosyaYaz(t, proje, "hatali.tr", "aktar \"tr/olmayan\"\n")
	bekle(t, tr(t, proje, "kontrol", "hatali.tr"), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok; kitaplıktaki modüller: tr/iyilestirici, tr/katman`})
	bekle(t, tr(t, proje, "derle", "hatali.tr", cikis), 1, nil, []string{`"tr/olmayan" dilin kitaplığında yok`})
}

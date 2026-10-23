package ml

import (
	"testing"
)

// Kural (dil tanımı 15.44, 15.36): karşılaştırma ve sonlu_mu kaynaştır içinde de türev izlemez;
// çıktısı türev isteyen girdiye türevlenebilir bir yoldan bağlı olmayan kaynaşmış fonksiyonun
// sonucu kaynaştır dışındaki gibi türevsizdir. (Gemini gerileme denetimi, Bulgu 7.)
func TestFusedComparisonDoesNotTrackGrad(t *testing.T) {
	out := mustRun(t, `
değişken x = tensör([1.0, 2.0], türev_izle: doğru)
değişken dış_hata = yanlış
dene {
    geri_yayılım(toplam(sonlu_mu(x)))
} yakala h {
    dış_hata = doğru
}
yazdır("DIS_HATA:", dış_hata, "DIS_GRAD:", x.gradyan)

değişken f = kaynaştır(fonksiyon(t) { sonuç sonlu_mu(t) })
değişken iç_hata = yanlış
dene {
    geri_yayılım(toplam(f(x)))
} yakala h {
    iç_hata = doğru
}
yazdır("IC_HATA:", iç_hata, "IC_GRAD:", x.gradyan)
`)
	want := "DIS_HATA: doğru DIS_GRAD: yok\nIC_HATA: doğru IC_GRAD: yok\n"
	if out != want {
		t.Fatalf("sonlu_mu kaynaştır içinde de türev izlememeliydi (15.36):\nalınan:\n%s\nbeklenen:\n%s", out, want)
	}
}

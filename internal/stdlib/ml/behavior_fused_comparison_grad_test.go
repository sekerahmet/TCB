package ml

import (
	"testing"
)

// Kural (dil tanımı 15.44, 15.36): karşılaştırma ve sonlu_mu kaynaştır içinde de türev izlemez;
// çıktısı türev isteyen girdiye türevlenebilir bir yoldan bağlı olmayan kaynaşmış fonksiyonun
// sonucu kaynaştır dışındaki gibi türevsizdir. (Gemini gerileme denetimi, Bulgu 7.)
func TestFusedComparisonDoesNotTrackGrad(t *testing.T) {
	out := mustRun(t, `
TR_var x = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var dış_hata = TR_false
TR_try {
    TR_backward(TR_sum(TR_is_finite(x)))
} TR_catch h {
    dış_hata = TR_true
}
TR_print("DIS_HATA:", dış_hata, "DIS_GRAD:", x.TR_grad)

TR_var f = TR_fuse(TR_func(t) { TR_return TR_is_finite(t) })
TR_var iç_hata = TR_false
TR_try {
    TR_backward(TR_sum(f(x)))
} TR_catch h {
    iç_hata = TR_true
}
TR_print("IC_HATA:", iç_hata, "IC_GRAD:", x.TR_grad)
`)
	want := "DIS_HATA: doğru DIS_GRAD: yok\nIC_HATA: doğru IC_GRAD: yok\n"
	if out != want {
		t.Fatalf("sonlu_mu kaynaştır içinde de türev izlememeliydi (15.36):\nalınan:\n%s\nbeklenen:\n%s", out, want)
	}
}

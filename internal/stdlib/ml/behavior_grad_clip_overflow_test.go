package ml

import (
	"testing"
)

// Kural (dil tanımı 15.36): float32'de taşan norm ([3e38, 3e38]) sonsuzdur; ölçek 0 olur, sonlu
// gradyanlar 0'dır (PyTorch 2.14 clip_grad_norm_ ile aynı). (Gemini gerileme denetimi, Bulgu 2.)
func TestGradClipOverflowingNormZeroesGrads(t *testing.T) {
	out := mustRun(t, `
değişken w = tensör([1.0, 1.0], türev_izle: doğru)
geri_yayılım(toplam(w * tensör([3e38, 3e38])))
değişken norm = gradyan_kırp([w], en_fazla: 1.0)
yazdır("NORM:", norm.değer)
yazdır("GRAD:", w.gradyan.değer)
`)
	want := "NORM: sonsuz\nGRAD: [0.0, 0.0]\n"
	if out != want {
		t.Fatalf("float32'de taşan normda ([3e38, 3e38]) ölçek 0 ve sonlu gradyanlar [0.0, 0.0] olmalıydı:\nalınan:\n%s\nbeklenen:\n%s", out, want)
	}
}

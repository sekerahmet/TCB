package ml

import (
	"testing"
)

// Kural (dil tanımı 15.36): float32'de taşan norm ([3e38, 3e38]) sonsuzdur; ölçek 0 olur, sonlu
// gradyanlar 0'dır (PyTorch 2.14 clip_grad_norm_ ile aynı). (Gemini gerileme denetimi, Bulgu 2.)
func TestGradClipOverflowingNormZeroesGrads(t *testing.T) {
	out := mustRun(t, `
TR_var w = TR_tensor([1.0, 1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(w * TR_tensor([3e38, 3e38])))
TR_var norm = TR_clip_grad([w], TR_max: 1.0)
TR_print("NORM:", norm.TR_tolist)
TR_print("GRAD:", w.TR_grad.TR_tolist)
`)
	want := "NORM: inf\nGRAD: [0.0, 0.0]\n"
	if out != want {
		t.Fatalf("float32'de taşan normda ([3e38, 3e38]) ölçek 0 ve sonlu gradyanlar [0.0, 0.0] olmalıydı:\nalınan:\n%s\nbeklenen:\n%s", out, want)
	}
}

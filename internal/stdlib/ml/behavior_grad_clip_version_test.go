package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (dil tanımı 15.36): sonlu_değilse_hata: doğru sonlu olmayan normda gradyanlara dokunmadan
// hata verir; gradyanın sürümü de artmaz, CPU ve GPU'da aynı. (Gemini gerileme denetimi, Bulgu 8.)
func TestGradClipStrictFailureLeavesGradsUntouched(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	out := mustRun(t, `
TR_func sına(cihaz_adı) {
    TR_default_device(cihaz_adı)
    TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
    TR_backward(TR_sum(w * TR_Float("nan")))
    TR_var v = TR_Tensor([1.0, 1.0], TR_requires_grad: TR_true)
    TR_var y = v * w.TR_grad
    TR_try {
        TR_clip_grad([w], TR_max: 1.0, TR_error_if_nonfinite: TR_true)
    } TR_catch h {}
    TR_backward(TR_sum(y))
    TR_return TR_is_finite(v.TR_grad).TR_tolist
}
TR_print("CPU:", sına("cpu"))
TR_print("GPU:", sına("gpu"))
`)
	want := "CPU: [0.0, 0.0]\nGPU: [0.0, 0.0]\n"
	if out != want {
		t.Fatalf("GPU'da sonlu_değilse_hata: doğru hatasında gradyanlara dokunulmamalıydı:\nalınan:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Kural (dil tanımı 15.36): kırpma gradyanları yerinde yazar sayılır; norm sınırın altında olsa da
// gradyanın sürümü artar (PyTorch clip_grad_norm_ ölçek 1'de de yerinde çarpar). Kırpmadan önce
// gradyanı kaydetmiş bir hesabın geri yayılımı CPU'da da GPU'da da aynı hatayı verir.
func TestGradClipBumpsVersionOnBothDevices(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	out := mustRun(t, `
TR_func sına(cihaz_adı) {
    TR_default_device(cihaz_adı)
    TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
    TR_backward(TR_sum(w * 0.1))
    TR_var v = TR_Tensor([1.0, 1.0], TR_requires_grad: TR_true)
    TR_var y = v * w.TR_grad
    TR_clip_grad([w], TR_max: 100.0)
    TR_var mesaj = ""
    TR_try { TR_backward(TR_sum(y)) } TR_catch h { mesaj = h }
    TR_return mesaj.TR_contains("modified in place")
}
TR_print("CPU:", sına("cpu"))
TR_print("GPU:", sına("gpu"))
`)
	if want := "CPU: true\nGPU: true\n"; out != want {
		t.Fatalf("kırpma iki cihazda da gradyan sürümünü artırmalı:\nalınan:\n%s\nbeklenen:\n%s", out, want)
	}
}

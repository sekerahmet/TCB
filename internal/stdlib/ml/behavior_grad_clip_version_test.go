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
fonksiyon sına(cihaz_adı) {
    varsayılan_cihaz(cihaz_adı)
    değişken w = tensör([1.0, 2.0], türev_izle: doğru)
    geri_yayılım(toplam(w * ondalık("nan")))
    değişken v = tensör([1.0, 1.0], türev_izle: doğru)
    değişken y = v * w.gradyan
    dene {
        gradyan_kırp([w], en_fazla: 1.0, sonlu_değilse_hata: doğru)
    } yakala h {}
    geri_yayılım(toplam(y))
    sonuç sonlu_mu(v.gradyan).değer
}
yazdır("CPU:", sına("cpu"))
yazdır("GPU:", sına("gpu"))
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
fonksiyon sına(cihaz_adı) {
    varsayılan_cihaz(cihaz_adı)
    değişken w = tensör([1.0, 2.0], türev_izle: doğru)
    geri_yayılım(toplam(w * 0.1))
    değişken v = tensör([1.0, 1.0], türev_izle: doğru)
    değişken y = v * w.gradyan
    gradyan_kırp([w], en_fazla: 100.0)
    değişken mesaj = ""
    dene { geri_yayılım(toplam(y)) } yakala h { mesaj = h }
    sonuç mesaj.içerir("yerinde değiştirildi")
}
yazdır("CPU:", sına("cpu"))
yazdır("GPU:", sına("gpu"))
`)
	if want := "CPU: doğru\nGPU: doğru\n"; out != want {
		t.Fatalf("kırpma iki cihazda da gradyan sürümünü artırmalı:\nalınan:\n%s\nbeklenen:\n%s", out, want)
	}
}

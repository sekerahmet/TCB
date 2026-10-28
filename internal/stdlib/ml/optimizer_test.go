package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestGradClip(t *testing.T) {
	out := mustRun(t, `TR_var w = TR_Tensor([1.0, 1.0], TR_requires_grad: TR_true)
TR_var b = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(w * TR_Tensor([3.0, 0.0])) + TR_sum(b * 4))
TR_print(TR_clip_grad([w, b], TR_max: 1.0).TR_tolist)
TR_print(w.TR_grad.TR_tolist, b.TR_grad.TR_tolist)
TR_print(TR_clip_grad([w, b]))`)
	// Kırpma öncesi norm 0 boyutlu tensördür; sayısı .değer ile okunur.
	want := "5.0\n[0.6, 0.0] [0.8]\nTensor(1.0000)\n"
	if out != want {
		t.Fatalf("gradyan_kırp:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"TR_clip_grad([TR_Tensor([1.0], TR_requires_grad: TR_true)], TR_max: 0)", messages.H6429},
		{"TR_clip_grad([TR_Tensor([1.0], TR_requires_grad: TR_true)], max_norm: 1)", messages.H7901},
		{"TR_clip_grad([TR_zeros([1])])", messages.H6513},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// AdamW'nin epsilon'u pozitif olmalı: 0 (ya da float32'de 0'a düşen değer) sıfır gradyanlı
// ilk adımda 0/(√0+0) = NaN üretirdi.
func TestAdamWEpsilonPositive(t *testing.T) {
	for _, eps := range []string{"0", "1e-50"} {
		_, err := run(t, "TR_var w = TR_Tensor([0.0], TR_requires_grad: TR_true)\nTR_AdamW([w], TR_eps: "+eps+")")
		if err == nil || !messages.Has(err, messages.H6527) {
			t.Fatalf("epsilon %s: hata beklenirdi: %v", eps, err)
		}
	}
}

// Kural (15.36): gradyan_kırp(..., sonlu_değilse_hata: doğru) toplam norm NaN ya da sonsuzsa
// H6534 verir (PyTorch error_if_nonfinite); varsayılan (yanlış) ve sonlu normda davranış
// değişmez. Mantık olmayan değer açık hatadır.
// Neden: uzun eğitim bozuk bir gradyanla sessizce sürmesin; isteyen erken durur.
func TestGradClipErrorIfNonfinite(t *testing.T) {
	out := mustRun(t, `TR_var w = TR_Tensor([3.0, 4.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(w * w))
TR_print(TR_clip_grad([w], TR_error_if_nonfinite: TR_true).TR_tolist)
TR_var v = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(v * TR_Float("nan")))
TR_print(TR_is_finite(TR_clip_grad([v]).TR_tolist))`)
	if out != "10.0\nfalse\n" {
		t.Fatalf("çıktı: %q", out)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`TR_var v = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(v * TR_Float("nan")))
TR_clip_grad([v], TR_error_if_nonfinite: TR_true)`, messages.H6534},
		{`TR_var v = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(v * TR_Float("inf")))
TR_clip_grad([v], TR_max: 2.0, TR_error_if_nonfinite: TR_true)`, messages.H6534},
		{`TR_var v = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(v))
TR_clip_grad([v], TR_error_if_nonfinite: 1)`, messages.H7905},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// Kural (15.36): sonlu_değilse_hata: doğru normu kırpmadan önce denetler; float32'de taşan norm
// ([3e38, 3e38]) sonsuzdur, hata verilir ve gradyanlar değişmez. Hata istenmezse sonlu olmayan
// normda kırpma PyTorch 2.14 clip_grad_norm_ ile aynıdır: sonsuz eleman NaN, sonlu eleman 0.
// Neden: eskiden gradyanlar önce kırpılıp sonra hata veriliyordu (Gemini kararlılık denetimi,
// bulgu 7); PyTorch error_if_nonfinite'te gradyanlara dokunmaz.
func TestGradClipChecksBeforeScaling(t *testing.T) {
	out := mustRun(t, `TR_var w = TR_Tensor([1.0, 1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(w * TR_Tensor([3e38, 3e38])))
TR_var hata_verdi = TR_false
TR_try { TR_clip_grad([w], TR_max: 1.0, TR_error_if_nonfinite: TR_true) } TR_catch h { hata_verdi = TR_true }
TR_print(hata_verdi, w.TR_grad.TR_tolist[0] > 1e38)
TR_var v = TR_Tensor([1.0, 1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(v * TR_Tensor([TR_Float("inf"), 2.0])))
TR_clip_grad([v], TR_max: 1.0)
TR_print(TR_is_finite(v.TR_grad.TR_tolist[0]), v.TR_grad.TR_tolist[1])`)
	if out != "true true\nfalse 0.0\n" {
		t.Fatalf("çıktı: %q", out)
	}
}

package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Genişletilmiş matematik yerleşikleri: sayıda ve tensörde dönüş tipleri, tam sayılı işaretin
// korunması, adlı argümanlar, yayınlama, türev ve yol gösteren hatalar.

func TestWideMathReturnTypes(t *testing.T) {
	out := mustRun(t, `TR_print(TR_type(TR_silu(1)), TR_type(TR_tan(1)), TR_type(TR_asin(0.5)), TR_type(TR_acos(0.5)), TR_type(TR_atan(2)))
TR_print(TR_type(TR_sinh(1)), TR_type(TR_cosh(1)), TR_type(TR_log1p(1)), TR_type(TR_log2(4)), TR_type(TR_log10(10)), TR_type(TR_expm1(1)))
TR_print(TR_type(TR_silu(TR_tensor([1.0]))), TR_type(TR_atan2(1, 2)), TR_type(TR_atan2(TR_tensor([1.0]), 2)))
TR_print(TR_floor(3.7), TR_floor(-3), TR_ceil(-3.7), TR_ceil(2), TR_sign(-7), TR_sign(2.5), TR_sign(0))
TR_print(TR_clamp(15, TR_max: 10), TR_clamp(-3, TR_min: 0), TR_clamp(5, TR_min: 0.5), TR_clamp(2.5, TR_min: 0, TR_max: 1))
TR_print(TR_log2(8), TR_log10(1000), TR_atan2(0, -1) == TR_pi, TR_pi > 3.14159 && TR_pi < 3.1416)`)
	want := "Ondalık Ondalık Ondalık Ondalık Ondalık\n" +
		"Ondalık Ondalık Ondalık Ondalık Ondalık Ondalık\n" +
		"Tensör Ondalık Tensör\n" +
		"3.0 -3 -3.0 2 -1 1.0 0\n" +
		"10 0 5.0 1.0\n" +
		"3.0 3.0 doğru doğru\n"
	if out != want {
		t.Fatalf("dönüş tipleri:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestWideMathTensor(t *testing.T) {
	out := mustRun(t, `TR_var i = TR_argmax(TR_tensor([[1, 5], [7, 2]]))
TR_print(TR_floor(i).TR_tolist, TR_sign(i - 1).TR_tolist, TR_clamp(i, TR_min: 0, TR_max: 0).TR_tolist, TR_clamp(i, TR_max: 0.5).TR_tolist)
TR_var x = TR_tensor([-2.0, 0.5, 3.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(TR_clamp(x, TR_min: -1, TR_max: 1) + TR_floor(x) * 5))
TR_print(x.TR_grad.TR_tolist)
TR_var s = TR_topk(TR_tensor([[3, 1, 3], [0, 2, 1]]), 2)
TR_print(s[0].TR_tolist, s[1].TR_tolist, TR_type(s[1].TR_tolist[0][0]))
TR_print(TR_argsort(TR_tensor([2, 1, 2, 0]), TR_dim: 0).TR_tolist, TR_cumsum(i).TR_tolist)
TR_var m = TR_tensor([[1.0], [0.0]])
TR_print(TR_where(m, TR_tensor([1.0, 2.0]), -1).TR_tolist, TR_where(TR_false, TR_tensor([1.0]), 7).TR_tolist)
TR_print((TR_tensor([1, 2, 3]) >= TR_tensor([[2], [1]])).TR_tolist, (2 < TR_tensor([1, 2, 3])).TR_tolist)`)
	want := "[1, 0] [0.0, -1.0] [0, 0] [0.5, 0.0]\n" + // i - 1 tam sayılı değil (15.19)
		"[0.0, 1.0, 0.0]\n" +
		"[[3.0, 3.0], [2.0, 1.0]] [[0, 2], [1, 2]] Tam\n" +
		"[3, 1, 0, 2] [1, 1]\n" +
		"[[1.0, 2.0], [-1.0, -1.0]] [7.0]\n" +
		"[[0.0, 1.0, 1.0], [1.0, 1.0, 1.0]] [0.0, 0.0, 1.0]\n"
	if out != want {
		t.Fatalf("tensör yolları:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestWideMathErrors(t *testing.T) {
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"TR_silu([1, 2])", messages.H6104},
		{"TR_floor([1.5])", messages.H6104},
		{"TR_atan2([1], 2)", messages.H6104},
		{"TR_atan2(1)", messages.H6116},
		{"TR_atan2(TR_tensor([1, 2]), TR_tensor([1, 2, 3]))", messages.H6100},
		{"TR_clamp(3)", messages.H6112},
		{"TR_clamp(3, TR_min: 2, TR_max: 1)", messages.H6113},
		{"TR_clamp(3, min: 1)", messages.H7900},
		{"TR_clamp(3, TR_min: \"a\")", messages.H6107},
		{"TR_clamp([1], TR_min: 0)", messages.H6104},
		{"TR_where(1, 2)", messages.H6201},
		{"TR_where(\"a\", 1, 2)", messages.H6207},
		{"TR_where(TR_tensor([1, 0]), TR_tensor([1, 2, 3]), 0)", messages.H6214},
		{"TR_where([1], 1, 2)", messages.H6104},
		{"TR_cumsum([1, 2])", messages.H6104},
		{"TR_cumsum(TR_tensor([1, 2]), 0)", messages.H6200},
		{"TR_cumsum(TR_tensor(3.0))", messages.H6015},
		{"TR_topk(TR_tensor([1, 2]), 3)", messages.H6210},
		{"TR_topk(TR_tensor([1, 2]), 1.5)", messages.H6208},
		{"TR_argsort(TR_tensor([[1, 2]]), TR_dim: 2)", messages.H6016},
		{"TR_tensor([1, 2]) < TR_tensor([1, 2, 3])", messages.H6100},
		{"TR_max(TR_tensor([1, 2]), 0)", messages.H6205},
		{"TR_min(TR_tensor([1, 2]), 0)", messages.H6205},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// sınırla Tam sınırları float64'e çevirmeden uygular: 2⁵³'ten büyük değerlerde yuvarlama
// kırpmayı bozmaz, en_az: 2⁶³-1 taşıp en küçük Tam'a dönmez.
func TestClampIntLimits(t *testing.T) {
	out := mustRun(t, `TR_print(TR_clamp(9223372036854775807, TR_max: 9223372036854775806))
TR_print(TR_clamp(5, TR_min: 9223372036854775807))
TR_print(TR_clamp(-9223372036854775807, TR_min: -9223372036854775806, TR_max: 0))
TR_print(TR_clamp(7, TR_min: 1, TR_max: 3), TR_clamp(7, TR_min: 1.5))`)
	want := "9223372036854775806\n9223372036854775807\n-9223372036854775806\n3 7.0\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
	if _, err := run(t, "TR_clamp(1, TR_min: 9223372036854775807, TR_max: 9223372036854775806)"); err == nil ||
		!strings.Contains(err.Error(), "9223372036854775807") {
		t.Fatalf("ters sınır hatası tam değeri göstermeli: %v", err)
	}
}

// Kural (13.7, 15.36): sonlu_mu sayıda Mantık verir (NaN ve ±sonsuz yanlış, her Tam doğru);
// tensörde eleman bazlı, sonlu yerde 1 öbür yerde 0 olan float32 tensör verir, 16 bitte de
// aynıdır ve türev izlemez. Listede, argümansız ve adlı argümanla açık hatadır.
// Neden: eğitim döngüsü gradyan normunu ve kaybı NaN için böyle denetler (torch.isfinite).
func TestIsFiniteNumbersAndTensors(t *testing.T) {
	out := mustRun(t, `TR_var nan = TR_float("nan")
TR_var sonsuz = TR_float("inf")
TR_print(TR_is_finite(3), TR_is_finite(-2.5), TR_is_finite(nan), TR_is_finite(sonsuz), TR_is_finite(-sonsuz), TR_type(TR_is_finite(0)))
TR_var x = TR_tensor([1.0, nan, sonsuz, -sonsuz, -0.0], TR_requires_grad: TR_true)
TR_var s = TR_is_finite(x)
TR_print(s.TR_tolist, s.TR_dtype)
TR_print(TR_is_finite(x.TR_to_dtype("bf16")).TR_tolist, TR_is_finite(TR_tensor([[1, 2]])).TR_tolist)`)
	want := "doğru doğru yanlış yanlış yanlış Mantık\n" +
		"[1.0, 0.0, 0.0, 0.0, 1.0] float32\n" +
		"[1.0, 0.0, 0.0, 0.0, 1.0] [[1.0, 1.0]]\n"
	if out != want {
		t.Fatalf("sonlu_mu:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"TR_is_finite([1.0])", messages.H6104},
		{"TR_is_finite()", messages.H2011},
		{"TR_is_finite(1, 2)", messages.H2011},
		{"TR_is_finite(\"a\")", messages.H2423},
		{"TR_is_finite(1.0, TR_dim: 0)", messages.H7902},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// Kural (15.36): sonlu_mu'nun sonucu karşılaştırmalar gibi her zaman Ondalık 0/1'dir; tam sayılı
// işaretli girdide (argmax, sıralama_indeksi) de, kaynaştır içindekiyle aynı.
// Neden: eskiden tam sayı işareti korunup [1, 1] veriyordu (Gemini kararlılık denetimi, bulgu 5).
func TestIsFiniteIsAFloatMaskOnIntTensors(t *testing.T) {
	out := mustRun(t, `TR_var idx = TR_argsort(TR_tensor([20.0, 10.0]))
TR_var f = TR_fuse(TR_func(x) { TR_return TR_is_finite(x) })
TR_print(TR_is_finite(idx).TR_tolist, f(idx).TR_tolist)`)
	if out != "[1.0, 1.0] [1.0, 1.0]\n" {
		t.Fatalf("çıktı: %q", out)
	}
}

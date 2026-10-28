package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Görünüm metotları (şekillendir, devrik, eksenleri_sırala, dilim) ve bitiştir tam sayılı
// tensörün (token kimlikleri, argmax) tam sayılı olduğunu korur: .değer Tam verir.
func TestViewsKeepInts(t *testing.T) {
	out := mustRun(t, `TR_var i = TR_argmax(TR_Tensor([[1.0, 5.0, 0.0], [7.0, 2.0, 9.0]]), TR_dim: 0)
TR_var m = i.TR_reshape([3, 1])
TR_print(m.TR_tolist, m.TR_transpose().TR_tolist, m.TR_permute([1, 0]).TR_tolist, i.TR_slice(0, 0, 2).TR_tolist)
TR_print(TR_concat([i, i]).TR_tolist, TR_concat([i, TR_Tensor([1.0, 2.0, 3.0])]).TR_tolist)`)
	want := "[[1], [0], [1]] [[1, 0, 1]] [[1, 0, 1]] [1, 0]\n[1, 0, 1, 1, 0, 1] [1.0, 0.0, 1.0, 1.0, 2.0, 3.0]\n"
	if out != want {
		t.Fatalf("çıktı:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Sıfır uzunluklu eksen eleman sınırını atlatmaz: [0, 10⁹, 10⁹] ve [10⁹, 0] gibi şekiller
// (veri yok ama 10⁹ boş liste ya da [4,0]@[0,N] ile dev çıktı) reddedilir.
func TestShapeLimitZeroAxis(t *testing.T) {
	for _, code := range []string{
		"TR_zeros([0, 1000000000, 1000000000])",
		"TR_ones([1000000000000, 0])",
		"TR_zeros([0]).TR_reshape([1000000000000, 0])",
	} {
		_, err := run(t, code)
		if err == nil || !messages.Has(err, messages.H6028) {
			t.Fatalf("%s: sınır hatası beklenirdi: %v", code, err)
		}
	}
	if out := mustRun(t, "TR_print(TR_zeros([0, 3]).TR_shape, TR_zeros([2, 0]).TR_tolist)"); out != "[0, 3] [[], []]\n" {
		t.Fatalf("küçük sıfır eksenli şekiller: %q", out)
	}
}

// Kural (B14): devrik() argüman almaz; argümanla çağrılınca ileti eksenlerin sırasını veren metodu
// (eksenleri_sırala) söyler (PyTorch: t.T ile t.permute(...)).
func TestTransposeWithArgumentsPointsToPermute(t *testing.T) {
	_, err := run(t, "TR_Tensor([[1.0, 2.0]]).TR_transpose(1, 0)")
	if !messages.Has(err, messages.H6722) || !strings.Contains(err.Error(), "eksenleri_sırala(") {
		t.Fatalf("devrik argümanla: %v", err)
	}
}

package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (25.9, 15.41): TR_logsumexp kartta hata verir, CPU'ya sessizce düşmez; ileti taşımanın
// yazımını söyler. TR_conv1d kartta çalışır (TestConv1dRunsOnTheCard).
func TestLogSumExpDoesNotFallBackFromTheCard(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, src := range []string{
		`TR_logsumexp(TR_ones([2]).TR_to_device("gpu"))`,
	} {
		_, err := run(t, src)
		if err == nil || !strings.Contains(err.Error(), ".TR_to_device(") {
			t.Errorf("%s: kartta taşıma önerilmeliydi: %v", src, err)
		}
	}
}

// Kural (25.9, 15.41, 15.23c): TR_conv1d kartta da çalışır: sonuç ve türevler kartta kalır, işlemciyle
// aynıdır (küçük tam değerler: 15.41'in basamaklarında fark çıkmaz); tensörler farklı cihazdaysa
// H6102 taşımanın yazımını söyler.
func TestConv1dRunsOnTheCard(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_func evrişim(cihaz: TR_String) -> TR_List[TR_String] {
    TR_var x = TR_Tensor(TR_Tensor([[[1.0, 2.0, 3.0, 4.0], [0.5, -1.0, 2.0, 0.0]]]).TR_to_device(cihaz), TR_requires_grad: TR_true)
    TR_var w = TR_Tensor(TR_Tensor([[[0.5, -1.0], [2.0, 1.0]]]).TR_to_device(cihaz), TR_requires_grad: TR_true)
    TR_var b = TR_Tensor(TR_Tensor([0.25]).TR_to_device(cihaz), TR_requires_grad: TR_true)
    TR_var y = TR_conv1d(x, w, b, TR_padding: 1, TR_stride: 2)
    TR_backward(TR_sum(y * y))
    TR_return [y.TR_device, TR_String(y.TR_to_device("cpu").TR_tolist), TR_String(x.TR_grad.TR_to_device("cpu").TR_tolist),
        TR_String(w.TR_grad.TR_to_device("cpu").TR_tolist), TR_String(b.TR_grad.TR_to_device("cpu").TR_tolist), x.TR_grad.TR_device]
}
TR_var c = evrişim("cpu")
TR_var g = evrişim("gpu")
TR_print(g[0], g[5], c[1] == g[1], c[2] == g[2], c[3] == g[3], c[4] == g[4])`)
	if want := "gpu:0 gpu:0 true true true true" + "\n"; out != want {
		t.Fatalf("kartta evrişim: %q, beklenen %q", out, want)
	}
	expectCodes(t, []codeCase{
		{`TR_conv1d(TR_ones([1, 1, 4]).TR_to_device("gpu"), TR_ones([1, 1, 2]), TR_none)`, messages.H6102},
		{`TR_conv1d(TR_ones([1, 1, 4]), TR_ones([1, 1, 2]).TR_to_device("gpu"), TR_none)`, messages.H6102},
		{`TR_conv1d(TR_ones([1, 1, 4]), TR_ones([1, 1, 2]), TR_ones([1]).TR_to_device("gpu"))`, messages.H6102},
	})
	// H6102'nin önerdiği yazım (`.TR_to_device(...)`) koşar ve evrişimin sonucunu verir.
	if out := mustRun(t, `TR_var x = TR_ones([1, 1, 4]).TR_to_device("gpu")
TR_print(TR_conv1d(x, TR_ones([1, 1, 2]).TR_to_device(x.TR_device), TR_none).TR_to_device("cpu").TR_tolist)`); out != "[[[2.0, 2.0, 2.0]]]"+"\n" {
		t.Errorf("H6102'nin önerisi: %q", out)
	}
}

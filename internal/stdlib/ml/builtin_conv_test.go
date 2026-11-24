package ml

import (
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (25.9, 15.41, 15.18): TR_logsumexp kartta da çalışır: sonuç ve türev kartta kalır,
// işlemciyle aynıdır (küçük tam değerler; Whisper'ın zaman damgası kuralının biçimi,
// `lib/whisper.tr:118`). Kartta float64 yoktur: ileti taşımanın yazımını söyler, CPU'ya sessizce
// düşmez; önerisi koşar.
func TestLogSumExpRunsOnTheCard(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := mustRun(t, `TR_func lse(cihaz: TR_String) -> TR_List[TR_Tensor] {
    TR_var x = TR_Tensor(TR_Tensor([[0.0, 1.0, 2.0, 3.0], [1.0, 1.0, 1.0, 1.0]]).TR_to_device(cihaz), TR_requires_grad: TR_true)
    TR_var lp = TR_log_softmax(x)
    TR_var y = TR_logsumexp(lp.TR_slice(1, 1, 4), TR_dim: 1)
    TR_backward(TR_sum(TR_logsumexp(x, TR_dim: [0, 1])))
    TR_return [y, x.TR_grad]
}
TR_var c = lse("cpu")
TR_var g = lse("gpu")
TR_print(g[0].TR_device, g[1].TR_device, TR_all_close(c[0], g[0].TR_to_device("cpu")), TR_all_close(c[1], g[1].TR_to_device("cpu")))`)
	if want := "gpu:0 gpu:0 true true\n"; out != want {
		t.Fatalf("kartta TR_logsumexp: %q, beklenen %q", out, want)
	}
	src := `TR_logsumexp(TR_Tensor([1.0, 2.0], TR_dtype: "float64").TR_to_device("gpu"))`
	if _, err := run(t, src); !messages.Has(err, messages.H7081) || !strings.Contains(err.Error(), ".TR_to_device(") {
		t.Errorf("%s: kartta float64 taşıma önermeliydi: %v", src, err)
	}
	if out := mustRun(t, `TR_var x = TR_Tensor([1.0, 2.0], TR_dtype: "float64").TR_to_device("gpu")
TR_print(TR_logsumexp(x.TR_to_device("cpu")).TR_device)`); out != "cpu\n" {
		t.Errorf("H7081'in önerisi: %q", out)
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
	// 15.23c: 16 bitlik girdi ve ağırlıkla float32 sapma kartta da 15.6c'yle birleşir; sonuç float32
	// ve işlemcininkiyle aynı (sapma bf16'ya inseydi 0.001 kaybolurdu).
	if out := mustRun(t, `TR_func sapmalı(cihaz: TR_String) -> TR_List[TR_String] {
    TR_var x = TR_ones([1, 2, 5]).TR_to_dtype("bf16").TR_to_device(cihaz)
    TR_var w = TR_ones([3, 2, 3]).TR_to_dtype("bf16").TR_to_device(cihaz)
    TR_var y = TR_conv1d(x, w, TR_Tensor([0.001, 0.002, 0.003]).TR_to_device(cihaz), TR_padding: 1)
    TR_return [y.TR_dtype, y.TR_device, TR_String(y.TR_to_device("cpu").TR_tolist)]
}
TR_var c = sapmalı("cpu")
TR_var g = sapmalı("gpu")
TR_print(g[0], g[1], c[2] == g[2], c[2])`); out != "float32 gpu:0 true [[[4.001, 6.001, 6.001, 6.001, 4.001], [4.002, 6.002, 6.002, 6.002, 4.002], [4.003, 6.003, 6.003, 6.003, 4.003]]]"+"\n" {
		t.Errorf("kartta sapmalı 16 bitlik evrişim: %q", out)
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

// convOperand n öğeli, sıfırdan farklı belirlenimli değerlerle [shape] tensörüdür.
func convOperand(shape ...int) *tensor.Tensor {
	n := 1
	for _, d := range shape {
		n *= d
	}
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(math.Sin(float64(i)*0.37)) * 0.5
	}
	return tensor.New(v, shape...)
}

// Kural (DENETİM 849 N2): TR_conv1d kapsamdaki çağrıyı işlemcinin türevsiz çekirdeğine
// (tensor.Conv1dKernel) gönderir; türevli, karışık hassasiyetli, 16 bitlik, kartta ve eski yolun gemv'e
// gittiği çağrı (B · çıkış = 1, N · K ≥ 4096) tensor.Conv1d'den gider. İki yol aynı bitleri verdiği
// için seçim çıktıdan görünmez; bu test seçimin kendisini sınar.
func TestConv1dDirectChoosesTheKernel(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	o := tensor.Conv1dOptions{Stride: 1, Padding: 1, Dilation: 1, Groups: 1}
	p0 := tensor.Conv1dOptions{Stride: 1, Dilation: 1, Groups: 1}
	x, w, b := convOperand(1, 4, 50), convOperand(8, 4, 3), convOperand(8)
	f32 := tensor.Float32
	for _, c := range []struct {
		name    string
		mixed   tensor.DType
		x, w, b *tensor.Tensor
		o       tensor.Conv1dOptions
		want    bool
	}{
		{"kapsamda", f32, x, w, b, o, true},
		{"sapmasız", f32, x, w, nil, o, true},
		{"bitişik olmayan girdi", f32, convOperand(1, 50, 4).Transpose(1, 2), w, b, o, true},
		{"çıkış boyu 1, küçük iç boyut", f32, convOperand(1, 2, 3), convOperand(3, 2, 3), nil, p0, true},
		{"çıkış boyu 1, toplu 2", f32, convOperand(2, 64, 5), convOperand(64, 64, 5), nil, p0, true},
		{"çıkış boyu 1, gemv", f32, convOperand(1, 64, 5), convOperand(64, 64, 5), nil, p0, false},
		{"türevli girdi", f32, convOperand(1, 4, 50).SetRequiresGrad(true), w, b, o, false},
		{"türevli ağırlık", f32, x, convOperand(8, 4, 3).SetRequiresGrad(true), b, o, false},
		{"türevli sapma", f32, x, w, convOperand(8).SetRequiresGrad(true), o, false},
		{"karışık hassasiyet", tensor.BFloat16, x, w, b, o, false},
		{"16 bit", f32, x.Cast(tensor.BFloat16), w.Cast(tensor.BFloat16), nil, o, false},
		{"kartta", f32, x.To(tensor.GPU(0)), w.To(tensor.GPU(0)), nil, o, false},
	} {
		if got := conv1dDirect(c.mixed, c.x, c.w, c.b, c.o); got != c.want {
			t.Errorf("%s: çekirdeğe gider %v, beklenen %v", c.name, got, c.want)
		}
	}
}

// Kural (15.22a, 15.23c): TR_conv1d'in sonucu iki yolda eski yolla (tensor.Conv1d) bit bit aynıdır:
// çekirdeğe giden bitişik olmayan girdi (çağıran bitişik yapar) ve eski yola kalan gemv çağrısı
// (çıkış boyu 1, Cin 64 × K 5 = 320 > kcBlock).
func TestConv1dBuiltinMatchesOldPathBits(t *testing.T) {
	o := tensor.Conv1dOptions{Stride: 1, Dilation: 1, Groups: 1}
	for _, c := range []struct {
		name    string
		x, w, b *tensor.Tensor
	}{
		{"bitişik olmayan girdi", convOperand(1, 50, 4).Transpose(1, 2), convOperand(8, 4, 3), convOperand(8)},
		{"çıkış boyu 1, gemv", convOperand(1, 64, 5), convOperand(64, 64, 5), convOperand(64)},
	} {
		r, err := conv1dBuiltin(nil, []interp.Value{New(c.x), New(c.w), New(c.b)}, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got, want := r.(*TensorValue).t.Data(), tensor.Conv1d(c.x, c.w, c.b, o, tensor.MatMul).Data()
		if len(got) != len(want) {
			t.Fatalf("%s: %d öğe, eski yolda %d", c.name, len(got), len(want))
		}
		for i := range got {
			if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
				t.Fatalf("%s: %d. öğe %v, eski yolda %v", c.name, i, got[i], want[i])
			}
		}
	}
}

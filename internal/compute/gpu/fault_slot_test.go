package gpu

import (
	"math"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (15.45, 15.6; T1 dilim 2a): TR_replay kaydında hatası öğeye bağlı çekirdek beklemez,
// hatayı kendi yuvasına yazar; yuva her oynatmada boşalır. Oynatmadan sonra FirstFault kayıt
// sırasındaki ilk dolu yuvayı ve o işlemin ilk hatalı öğesini (işlemcinin iletisiyle aynı
// ayrıntı, girdinin işlem anındaki değeri) verir; hatasız oynatmada -1. Kayıtta konağa okuma
// yoktur (kayıt H4065 vermez).
func TestFaultSlotInsideRecording(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	a := tensor.NewOn(gpuDevice, []float32{1, 2, 3}, 3)
	b := tensor.NewOn(gpuDevice, []float32{4, 5, 6}, 3)
	zero := tensor.NewOn(gpuDevice, []float32{0, 0, 0}, 3)
	f := func() (*tensor.Tensor, *tensor.Tensor, []*tensor.ElemFault) {
		ra, fa := tensor.Convert(a, tensor.Int8)
		rb, fb := tensor.Convert(b, tensor.Int8)
		// İkinci dönüşümün girdisi kayıtta sonradan değişir: ayrıntı işlem anındaki değerdir.
		b.CopyFrom(zero)
		return ra, rb, []*tensor.ElemFault{fa, fb}
	}
	f() // ısınma
	b.CopyFrom(tensor.NewOn(gpuDevice, []float32{4, 5, 6}, 3))
	g.BeginCapture()
	ra, rb, faults := f()
	gr := g.EndCapture()
	defer gr.Release()
	for i, fl := range faults {
		if fl == nil || fl.Kind != tensor.FaultRecorded || fl.Index != i {
			t.Fatalf("kayıtta %d. dönüşüm: %+v, beklenen FaultRecorded, yuva %d", i, fl, i)
		}
	}
	fr := tensor.Recording(gr).(tensor.FaultRecording)
	replay := func(av, bv []float32) (int, *tensor.ElemFault) {
		a.CopyFrom(tensor.NewOn(gpuDevice, av, 3))
		b.CopyFrom(tensor.NewOn(gpuDevice, bv, 3))
		gr.Replay()
		return fr.FirstFault()
	}
	if site, fl := replay([]float32{1, -2, 3}, []float32{7, 8, 9}); site != -1 || fl != nil {
		t.Fatalf("hatasız oynatma: yuva %d, %+v", site, fl)
	}
	if got := ra.Int64s(); !slices.Equal(got, []int64{1, -2, 3}) {
		t.Fatalf("oynatmanın sonucu %v", got)
	}
	if got := rb.Int64s(); !slices.Equal(got, []int64{7, 8, 9}) {
		t.Fatalf("oynatmanın ikinci sonucu %v", got)
	}
	site, fl := replay([]float32{1, 2, 3}, []float32{9, 300, float32(math.NaN())})
	want, _ := tensor.Convert(tensor.NewOn(tensor.CPU, []float32{9, 300, 0}, 3), tensor.Int8)
	_, cf := tensor.Convert(tensor.NewOn(tensor.CPU, []float32{9, 300, float32(math.NaN())}, 3), tensor.Int8)
	if want != nil || site != 1 || !sameFault(fl, cf) {
		t.Fatalf("ikinci işlemde hata: yuva %d, %+v; beklenen yuva 1, %+v", site, fl, cf)
	}
	site, fl = replay([]float32{1e9, 2, 3}, []float32{9, 300, 1})
	if site != 0 || fl.Kind != tensor.FaultNotFit || fl.Index != 0 || fl.Float != 1e9 {
		t.Fatalf("iki işlemde hata: yuva %d, %+v; beklenen yuva 0, konum 0", site, fl)
	}
	if site, fl := replay([]float32{1, 2, 3}, []float32{4, 5, 6}); site != -1 || fl != nil {
		t.Fatalf("yuvalar oynatmada boşalmadı: yuva %d, %+v", site, fl)
	}
}

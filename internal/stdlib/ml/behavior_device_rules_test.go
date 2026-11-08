package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (dil tanımı 15.40): farklı cihazlardaki tensörlerle indeksle atama sessizce kopyalamaz;
// hata .cihaza(...) önerir.
// Neden: a[0] = b başka cihazdaki b'yi sessizce kopyalıyordu.
func TestIndexAssignAcrossDevicesIsAnError(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	_, err := run(t, `TR_var a = TR_zeros([2, 1]).TR_to_device("cpu")
TR_var b = TR_ones([1]).TR_to_device("gpu")
a[0] = b`)
	if err == nil || !strings.Contains(err.Error(), ".cihaza(") {
		t.Errorf("a[0] = b (farklı cihaz) .cihaza(...) öneren bir hata vermeliydi: %v", err)
	}
}

// Kural (dil tanımı 15.45): tekrar_oynat kaydında indeksle atamanın sayı işleneni cihazda doldurulur.
// Neden: SetIndex sayıyı konakta kurup yüklüyordu; kaydın ikinci çağrısı "GPU kaydı sürerken konaktan
// yüklenemez" hatasıyla düşüyordu.
func TestReplayIndexAssignScalarIsFilledOnDevice(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	out := mustRun(t, `TR_default_device("gpu")
TR_func sıfırla_ilk(x: TR_Tensor) -> TR_Tensor {
    x[0] = 0.0
    TR_return x * 2
}
TR_var f = TR_replay(sıfırla_ilk)
TR_no_grad {
    TR_var a = TR_Tensor([5.0, 3.0])
    TR_var b = TR_Tensor([7.0, 4.0])
    TR_var c = TR_Tensor([9.0, 6.0])
    TR_print(f(a).TR_tolist, a.TR_tolist)
    TR_print(f(b).TR_tolist, b.TR_tolist)
    TR_print(f(c).TR_tolist, c.TR_tolist)
}`)
	want := "[0.0, 6.0] [0.0, 3.0]\n[0.0, 8.0] [0.0, 4.0]\n[0.0, 12.0] [0.0, 6.0]\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
}

// Kural (dil tanımı 15.6d): tensörle indekse yazma kartta da H7042'dir; hedef kopya kalır, t değişmez.
// Önerilen döngü kartta yazar.
func TestTensorIndexWriteOnDevice(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	const prelude = `TR_var t = TR_Tensor([1.0, 2.0, 3.0, 4.0]).TR_to_device("gpu")
TR_var idx = TR_Tensor([1, 3])
TR_var d = TR_Tensor([5.0, 6.0]).TR_to_device("gpu")
`
	_, err := run(t, prelude+"t[idx] = d")
	if err == nil || !messages.Has(err, messages.H7042) {
		t.Fatalf("kartta t[idx] = d: %v, H7042 bekleniyordu", err)
	}
	if got := mustRun(t, prelude+suggestedSpelling(t, err.Error())+"\nTR_print(t.TR_tolist, t.TR_device)"); got != "[1.0, 5.0, 3.0, 6.0] gpu:0\n" {
		t.Errorf("kartta önerilen döngü: %q", got)
	}
}

package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (dil tanımı 15.40): farklı cihazlardaki tensörlerle indeksle atama sessizce kopyalamaz;
// hata .cihaza(...) önerir.
// Neden: a[0] = b başka cihazdaki b'yi sessizce kopyalıyordu.
func TestIndexAssignAcrossDevicesIsAnError(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	_, err := run(t, `değişken a = sıfırlar([2, 1]).cihaza("cpu")
değişken b = birler([1]).cihaza("gpu")
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

	out := mustRun(t, `varsayılan_cihaz("gpu")
fonksiyon sıfırla_ilk(x) {
    x[0] = 0.0
    sonuç x * 2
}
değişken f = tekrar_oynat(sıfırla_ilk)
türevsiz {
    değişken a = tensör([5.0, 3.0])
    değişken b = tensör([7.0, 4.0])
    değişken c = tensör([9.0, 6.0])
    yazdır(f(a).değer, a.değer)
    yazdır(f(b).değer, b.değer)
    yazdır(f(c).değer, c.değer)
}`)
	want := "[0.0, 6.0] [0.0, 3.0]\n[0.0, 8.0] [0.0, 4.0]\n[0.0, 12.0] [0.0, 6.0]\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
}

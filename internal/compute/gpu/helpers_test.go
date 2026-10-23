package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Sınamalar gerçek bir GPU varsa onda, yoksa PTX benzetiminde koşar; her işlemin
// sonucu ve gradyanları CPU arka ucuyla karşılaştırılır.

var gpuDevice = tensor.GPU(0)

func TestMain(m *testing.M) {
	if RegisterSimulator() {
		fmt.Println("GPU sınamaları PTX benzetiminde koşuyor (gerçek GPU yok: " + Status() + ")")
	} else {
		fmt.Println("GPU sınamaları gerçek GPU'da koşuyor: " + Status())
	}
	kernelBuilt = recordPTX
	code := m.Run()
	if code == 0 {
		code = checkPTXDigest()
	}
	os.Exit(code)
}

// gpuBackend sınamaların GPU arka ucu.
func gpuBackend() *backend { return tensor.BackendFor(gpuDevice).(*backend) }

// inSim sınamaların benzetimde koşup koşmadığı.
func inSim() bool {
	_, ok := gpuBackend().sr.(*simDriver)
	return ok
}

func random(rng *rand.Rand, n int, scaleF float64) []float32 {
	d := make([]float32, n)
	for i := range d {
		d[i] = float32(rng.NormFloat64() * scaleF)
	}
	return d
}

// twin aynı veriyle CPU ve GPU'da birer tensör kurar.
func twin(data []float32, grad bool, shape ...int) [2]*tensor.Tensor {
	return [2]*tensor.Tensor{
		tensor.NewOn(tensor.CPU, data, shape...).SetRequiresGrad(grad),
		tensor.NewOn(gpuDevice, data, shape...).SetRequiresGrad(grad),
	}
}

// checkDiff iki dizinin göreli farkını denetler: |a−b| ≤ tol·(1 + |b|).
func checkDiff(t *testing.T, ad string, gpu, cpu []float32, tol float64) {
	t.Helper()
	if len(gpu) != len(cpu) {
		t.Fatalf("%s: uzunluk farklı: GPU %d, CPU %d", ad, len(gpu), len(cpu))
	}
	worst, place := 0.0, -1
	for i := range gpu {
		a, b := float64(gpu[i]), float64(cpu[i])
		if math.IsNaN(a) || math.IsNaN(b) {
			if math.IsNaN(a) != math.IsNaN(b) {
				t.Fatalf("%s[%d]: NaN uyuşmazlığı: GPU %v, CPU %v", ad, i, a, b)
			}
			continue
		}
		if math.IsInf(b, 0) || math.IsInf(a, 0) {
			if a != b {
				t.Fatalf("%s[%d]: sonsuz uyuşmazlığı: GPU %v, CPU %v", ad, i, a, b)
			}
			continue
		}
		if f := math.Abs(a-b) / (1 + math.Abs(b)); f > worst {
			worst, place = f, i
		}
	}
	if worst > tol {
		t.Fatalf("%s: en büyük göreli fark %.3g (izin %.3g) %d. elemanda: GPU %v, CPU %v", ad, worst, tol, place, gpu[place], cpu[place])
	}
}

// compare kur'un iki cihazda ürettiği çıktıyı ve (geri yayılımla) yaprakların
// gradyanlarını karşılaştırır. kur aynı veriden tensörleri cihaz sırasıyla (0 CPU, 1 GPU)
// kurmalı ve çıktıyla yaprakları vermelidir. Geri yayılımın tohumu, çıktıyla aynı
// şekilli sabit rastgele ağırlıklardır.
func compare(t *testing.T, ad string, tol float64, setupFn func(c int) (*tensor.Tensor, []*tensor.Tensor)) {
	t.Helper()
	var output [2][]float32
	var grads [2][][]float32
	for c := 0; c < 2; c++ {
		out, leaves := setupFn(c)
		if out.Device() != []tensor.Device{tensor.CPU, gpuDevice}[c] {
			t.Fatalf("%s: çıktı yanlış cihazda: %s", ad, out.Device())
		}
		output[c] = out.Data()
		if len(leaves) == 0 {
			continue
		}
		w := random(rand.New(rand.NewSource(99)), out.Numel(), 1)
		wt := tensor.NewOn(out.Device(), w, out.Shape()...)
		tensor.Mul(out, wt).SumAll().Backward()
		for _, y := range leaves {
			if y.Grad() == nil {
				grads[c] = append(grads[c], nil)
				continue
			}
			grads[c] = append(grads[c], y.Grad().Data())
		}
	}
	checkDiff(t, ad+" ileri", output[1], output[0], tol)
	for i := range grads[0] {
		if (grads[0][i] == nil) != (grads[1][i] == nil) {
			t.Fatalf("%s: %d. yaprağın gradyanı bir cihazda yok", ad, i)
		}
		checkDiff(t, fmt.Sprintf("%s gradyan %d", ad, i), grads[1][i], grads[0][i], tol)
	}
}

// panicText fn'in panik mesajını verir (panik yoksa boş).
func panicText(fn func()) (m string) {
	defer func() {
		if r := recover(); r != nil {
			m = fmt.Sprint(r)
		}
	}()
	fn()
	return ""
}

// internalDetail hata zincirindeki ilk iç hatanın (H9000) İngilizce ayrıntısını verir (yoksa boş).
func internalDetail(err error) string {
	if err == nil {
		return ""
	}
	if m, ok := err.(*messages.Error); ok {
		if m.Code == messages.H9000 {
			return fmt.Sprint(m.Args[0])
		}
		for _, a := range m.Args {
			if e, ok := a.(error); ok {
				if d := internalDetail(e); d != "" {
					return d
				}
			}
		}
	}
	return ""
}

// skipUnderRace ağır sayısal çekirdek testlerini yarış dedektörü altında atlar: benzetimde
// çekirdek hesabının doğruluğunu sınarlar (eşzamanlılık değil) ve yarış dedektörüyle 10–20
// kat yavaşlayıp otomatik sınamanın yarış işini 8 dakikaya çıkarıyorlardı. Aynı testler öbür
// işlerde (dagitim, go-esleri) koşar.
func skipUnderRace(t *testing.T) {
	t.Helper()
	if raceEnabled {
		t.Skip("ağır sayısal çekirdek testi; yarış dedektörü altında atlanır (öbür kiplerde koşar)")
	}
}

// flatView depolamanın ilk n elemanına 1 boyutlu bitişik görünümdür.
func flatView(s tensor.Storage, n int) tensor.View { return tensor.DenseView(s, []int{n}) }

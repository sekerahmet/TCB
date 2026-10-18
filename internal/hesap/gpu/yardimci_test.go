package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// Sınamalar gerçek bir GPU varsa onda, yoksa PTX benzetiminde koşar; her işlemin
// sonucu ve gradyanları CPU arka ucuyla karşılaştırılır.

var gpuCihaz = tensor.GPU(0)

func TestMain(m *testing.M) {
	if BenzetimiKaydet() {
		fmt.Println("GPU sınamaları PTX benzetiminde koşuyor (gerçek GPU yok: " + Durum() + ")")
	} else {
		fmt.Println("GPU sınamaları gerçek GPU'da koşuyor: " + Durum())
	}
	os.Exit(m.Run())
}

// gpuArkaUc sınamaların GPU arka ucu.
func gpuArkaUc() *arkaUc { return tensor.BackendFor(gpuCihaz).(*arkaUc) }

// benzetimde sınamaların benzetimde koşup koşmadığı.
func benzetimde() bool {
	_, ok := gpuArkaUc().sr.(*benzetimSurucu)
	return ok
}

func rastgele(rng *rand.Rand, n int, olcek float64) []float32 {
	d := make([]float32, n)
	for i := range d {
		d[i] = float32(rng.NormFloat64() * olcek)
	}
	return d
}

// ikiz aynı veriyle CPU ve GPU'da birer tensör kurar.
func ikiz(veri []float32, grad bool, shape ...int) [2]*tensor.Tensor {
	return [2]*tensor.Tensor{
		tensor.NewOn(tensor.CPU, veri, shape...).SetRequiresGrad(grad),
		tensor.NewOn(gpuCihaz, veri, shape...).SetRequiresGrad(grad),
	}
}

// farkDenetle iki dizinin göreli farkını denetler: |a−b| ≤ tol·(1 + |b|).
func farkDenetle(t *testing.T, ad string, gpu, cpu []float32, tol float64) {
	t.Helper()
	if len(gpu) != len(cpu) {
		t.Fatalf("%s: uzunluk farklı: GPU %d, CPU %d", ad, len(gpu), len(cpu))
	}
	enKotu, yer := 0.0, -1
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
		if f := math.Abs(a-b) / (1 + math.Abs(b)); f > enKotu {
			enKotu, yer = f, i
		}
	}
	if enKotu > tol {
		t.Fatalf("%s: en büyük göreli fark %.3g (izin %.3g) %d. elemanda: GPU %v, CPU %v", ad, enKotu, tol, yer, gpu[yer], cpu[yer])
	}
}

// karsilastir kur'un iki cihazda ürettiği çıktıyı ve (geri yayılımla) yaprakların
// gradyanlarını karşılaştırır. kur aynı veriden tensörleri cihaz sırasıyla (0 CPU, 1 GPU)
// kurmalı ve çıktıyla yaprakları vermelidir. Geri yayılımın tohumu, çıktıyla aynı
// şekilli sabit rastgele ağırlıklardır.
func karsilastir(t *testing.T, ad string, tol float64, kur func(c int) (*tensor.Tensor, []*tensor.Tensor)) {
	t.Helper()
	var cikti [2][]float32
	var gradlar [2][][]float32
	for c := 0; c < 2; c++ {
		out, yapraklar := kur(c)
		if out.Device() != []tensor.Device{tensor.CPU, gpuCihaz}[c] {
			t.Fatalf("%s: çıktı yanlış cihazda: %s", ad, out.Device())
		}
		cikti[c] = out.Data()
		if len(yapraklar) == 0 {
			continue
		}
		w := rastgele(rand.New(rand.NewSource(99)), out.Numel(), 1)
		wt := tensor.NewOn(out.Device(), w, out.Shape()...)
		tensor.Mul(out, wt).SumAll().Backward()
		for _, y := range yapraklar {
			if y.Grad() == nil {
				gradlar[c] = append(gradlar[c], nil)
				continue
			}
			gradlar[c] = append(gradlar[c], y.Grad().Data())
		}
	}
	farkDenetle(t, ad+" ileri", cikti[1], cikti[0], tol)
	for i := range gradlar[0] {
		if (gradlar[0][i] == nil) != (gradlar[1][i] == nil) {
			t.Fatalf("%s: %d. yaprağın gradyanı bir cihazda yok", ad, i)
		}
		farkDenetle(t, fmt.Sprintf("%s gradyan %d", ad, i), gradlar[1][i], gradlar[0][i], tol)
	}
}

// panikMetni fn'in panik mesajını verir (panik yoksa boş).
func panikMetni(fn func()) (m string) {
	defer func() {
		if r := recover(); r != nil {
			m = fmt.Sprint(r)
		}
	}()
	fn()
	return ""
}

package gpu

import (
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// normReference katman ya da RMS normunun float64'te elle hesaplanan ileri sonucu ve
// gradyanları; kayıp Σ çıktı·w'dir (yukarıdan gelen gradyan w).
type normReference struct{ out, dx, dGamma, dBeta []float64 }

func referenceNorm(layer, affine bool, x, gamma, beta, w []float32, R, D int, eps float64) normReference {
	r := normReference{out: make([]float64, R*D), dx: make([]float64, R*D), dGamma: make([]float64, D), dBeta: make([]float64, D)}
	scale := func(j int) float64 {
		if !affine {
			return 1
		}
		return float64(gamma[j])
	}
	for i := 0; i < R; i++ {
		row := x[i*D : (i+1)*D]
		mean := 0.0
		if layer {
			for _, v := range row {
				mean += float64(v)
			}
			mean /= float64(D)
		}
		ss := 0.0
		for _, v := range row {
			ss += (float64(v) - mean) * (float64(v) - mean)
		}
		rstd := 1 / math.Sqrt(ss/float64(D)+eps)
		sumA, sumAX := 0.0, 0.0
		for j := 0; j < D; j++ {
			xh := (float64(row[j]) - mean) * rstd
			wj := float64(w[i*D+j])
			r.out[i*D+j] = xh * scale(j)
			if affine && layer {
				r.out[i*D+j] += float64(beta[j])
			}
			r.dGamma[j] += wj * xh
			r.dBeta[j] += wj
			a := wj * scale(j)
			sumA += a
			sumAX += a * xh
		}
		for j := 0; j < D; j++ {
			xh := (float64(row[j]) - mean) * rstd
			a := float64(w[i*D+j]) * scale(j)
			if layer {
				r.dx[i*D+j] = rstd / float64(D) * (float64(D)*a - sumA - xh*sumAX)
			} else {
				r.dx[i*D+j] = rstd * (a - xh*sumAX/float64(D))
			}
		}
	}
	return r
}

// Kural: normların ileri sonucu ve her gradyanı, CPU'da da GPU'da da float64 referansa satır
// sayısından bağımsız bir hatayla yaklaşır (|a − ref| ≤ izin·(1 + |ref|)). γ ve β'nın
// gradyanları satırlar boyunca toplamdır: GPU'nun son toplamı telafilidir (kernel_reduce.go);
// düz float32 toplamda 2000 satırda β gradyanının hatası 1.7e-5'e çıkıyordu ve gerçek kartta
// TestNorms'un CPU karşılaştırmasını aşıyordu. TestNorms iki cihazı birbirine karşı sınar; bu
// sınama hangisinin doğru olduğunu söyler.
func TestNormsMatchFloat64Reference(t *testing.T) {
	// Ölçülen en büyük fark (RTX PRO 6000 ve benzetici): GPU γ gradyanı 6e-6, CPU 2.4e-6; düz
	// toplamla GPU'da β gradyanı 1.7e-5 (benzetici), γ gradyanı 2.6e-5 (gerçek kart) idi.
	const tol = 1e-5
	rng := rand.New(rand.NewSource(10))
	sizes := [][2]int{{300, 129}, {70, 600}, {300, 32}, {21, 512}}
	if !testing.Short() {
		sizes = append(sizes, [][2]int{{1000, 64}, {2000, 512}, {777, 256}, {257, 512}}...)
	}
	for _, d := range sizes {
		R, D := d[0], d[1]
		xv, gv, bv := random(rng, R*D, 2), random(rng, D, 1), random(rng, D, 1)
		w := random(rng, R*D, 1)
		for _, c := range []struct {
			ad                   string
			layer, affine, xGrad bool
		}{{"rms normu", false, true, true}, {"katman normu", true, true, true},
			{"rms normu, yalnız γ gradyanı", false, true, false}, {"ölçeksiz katman normu", true, false, true}} {
			ref := referenceNorm(c.layer, c.affine, xv, gv, bv, w, R, D, 1e-5)
			for _, dev := range []tensor.Device{tensor.CPU, gpuDevice} {
				x := tensor.NewOn(dev, xv, R, D).SetRequiresGrad(c.xGrad)
				var gamma, beta, out *tensor.Tensor
				if c.affine {
					gamma = tensor.NewOn(dev, gv, D).SetRequiresGrad(true)
				}
				switch {
				case c.layer && c.affine:
					beta = tensor.NewOn(dev, bv, D).SetRequiresGrad(true)
					out = tensor.LayerNorm(x, gamma, beta, 1e-5)
				case c.layer:
					out = tensor.LayerNorm(x, nil, nil, 1e-5)
				default:
					out = tensor.RMSNorm(x, gamma, 1e-5)
				}
				outData := out.Data()
				tensor.Mul(out, tensor.NewOn(dev, w, R, D)).SumAll().Backward()
				check := func(part string, got []float32, want []float64) {
					t.Helper()
					worst, at := 0.0, 0
					for i := range got {
						if e := math.Abs(float64(got[i])-want[i]) / (1 + math.Abs(want[i])); e > worst {
							worst, at = e, i
						}
					}
					if worst > tol {
						t.Errorf("%s %d×%d %s (%s): float64 referansa göre fark %.3g (izin %g) %d. elemanda: %v, referans %v",
							c.ad, R, D, part, dev, worst, tol, at, got[at], want[at])
					}
				}
				check("ileri", outData, ref.out)
				if c.xGrad {
					check("x gradyanı", x.Grad().Data(), ref.dx)
				}
				if gamma != nil {
					check("γ gradyanı", gamma.Grad().Data(), ref.dGamma)
				}
				if beta != nil {
					check("β gradyanı", beta.Grad().Data(), ref.dBeta)
				}
			}
		}
	}
}

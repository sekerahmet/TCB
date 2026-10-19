package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestCokTensorluIyilestirici birden çok parametreyi (biri bir bloğun parçasından uzun, biri
// bfloat16 ana kopyalı, biri gradyansız) tek başlatmada güncelleyen yolu ve gradyan kırpmanın
// toplu ölçeklemesini CPU ile karşılaştırır. Öğrenme oranı adımlar arasında değiştirilir: yeni
// oran sonraki adımda kullanılmalıdır (öğrenme_oranı ataması, Y504).
func TestMultiTensorOptimizer(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	sizes := []int{7, multiPart + 300, 33, 5}
	pv := make([][]float32, len(sizes))
	hv := make([][]float32, len(sizes))
	for i, n := range sizes {
		pv[i], hv[i] = random(rng, n, 1), random(rng, n, 1)
	}
	for _, ad := range []string{"adamw", "sgd momentumlu", "sgd"} {
		var result [2][][]float32
		var norms [2]float64
		for c := 0; c < 2; c++ {
			var ps []*tensor.Tensor
			for i := range sizes {
				p := twin(pv[i], true, sizes[i])[c]
				if i == 2 {
					p = inLeafDType(p, tensor.BFloat16)
				}
				ps = append(ps, p)
			}
			var step func()
			var ratio *float32
			switch ad {
			case "adamw":
				o := tensor.NewAdamW(ps, 0.01, 0.1)
				step, ratio = o.Step, &o.LR
			case "sgd":
				o := tensor.NewSGD(ps, 0.1, 0)
				step, ratio = o.Step, &o.LR
			default:
				o := tensor.NewSGD(ps, 0.1, 0.9)
				step, ratio = o.Step, &o.LR
			}
			base2 := *ratio
			for k := 0; k < 3; k++ {
				*ratio = base2 * []float32{1, 0.3, 2}[k]
				tensor.ZeroGrads(ps)
				var loss2 *tensor.Tensor
				for i, p := range ps[:3] { // son parametre gradyan almaz
					h := tensor.NewOn(p.Device(), hv[i], sizes[i])
					y := tensor.Mul(p.Cast(tensor.Float32), h).Tanh().SumAll()
					if loss2 == nil {
						loss2 = y
					} else {
						loss2 = tensor.Add(loss2, y)
					}
				}
				loss2.Backward()
				norms[c] = tensor.ClipGradNorm(ps, 0.5)
				step()
			}
			for _, p := range ps {
				result[c] = append(result[c], p.Cast(tensor.Float32).Data())
			}
		}
		for i := range result[0] {
			checkDiff(t, fmt.Sprintf("%s parametre %d", ad, i), result[1][i], result[0][i], 1e-5)
		}
		if d := norms[1] - norms[0]; d > 1e-5*norms[0] || d < -1e-5*norms[0] {
			t.Fatalf("%s: gradyan normu GPU %v, CPU %v", ad, norms[1], norms[0])
		}
	}
}

// TestKirpmaCihazda gradyan kırpmanın cihazdaki yolunu (kareler toplamı, kirp_norm,
// cok_olcek_bellek; konak beklemez) CPU ile karşılaştırır: norm 0 boyutlu GPU tensörüdür,
// kırpılan ve kırpılmayan (ölçek 1: gradyanlar bit bit aynı kalır) durumlar, birden çok
// bloklu ve gradyansız parametre.
func TestClipOnDevice(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	sizes := []int{3, 40000, 17, 5}
	hv := make([][]float32, len(sizes))
	for i, n := range sizes {
		hv[i] = random(rng, n, 1)
	}
	for _, most := range []float64{0.5, 1e9} {
		var norms [2]float32
		var grads [2][][]float32
		for c, device := range []tensor.Device{tensor.CPU, gpuDevice} {
			var ps []*tensor.Tensor
			var loss2 *tensor.Tensor
			for i, n := range sizes {
				p := tensor.NewOn(device, make([]float32, n), n).SetRequiresGrad(true)
				ps = append(ps, p)
				if i == len(sizes)-1 {
					continue // son parametre gradyan almaz
				}
				y := tensor.Mul(p, tensor.NewOn(device, hv[i], n)).SumAll()
				if loss2 == nil {
					loss2 = y
				} else {
					loss2 = tensor.Add(loss2, y)
				}
			}
			loss2.Backward()
			norm := tensor.ClipGradNormTensor(ps, most)
			if len(norm.Shape()) != 0 {
				t.Fatalf("norm şekli %v, 0 boyutlu bekleniyordu", norm.Shape())
			}
			if c == 1 && norm.Device() != gpuDevice {
				t.Fatalf("GPU'da norm %s cihazında, cihazda kalmalıydı", norm.Device())
			}
			norms[c] = norm.Data()[0]
			for _, p := range ps[:len(ps)-1] {
				grads[c] = append(grads[c], p.Grad().Data())
			}
		}
		if d := float64(norms[1] - norms[0]); d > 1e-6*float64(norms[0]) || d < -1e-6*float64(norms[0]) {
			t.Fatalf("en_fazla %g: norm GPU %v, CPU %v", most, norms[1], norms[0])
		}
		for i := range grads[0] {
			tol := 1e-6
			if most > 1e8 {
				tol = 0 // kırpılmaz: gradyanlar aynen kalır
			}
			checkDiff(t, fmt.Sprintf("en_fazla %g gradyan %d", most, i), grads[1][i], grads[0][i], tol)
		}
	}
	// Gradyanı olmayan liste: norm 0.
	p := tensor.NewOn(gpuDevice, []float32{1}, 1).SetRequiresGrad(true)
	if v := tensor.ClipGradNormTensor([]*tensor.Tensor{p}, 1).Data()[0]; v != 0 {
		t.Fatalf("gradyansız norm %v", v)
	}
}

// TestIyilestiriciTekrarliParametre aynı parametre listede iki kez: CPU'daki gibi iki ardışık
// güncelleme (tek başlatmada yarış olmaz); gradyan kırpma da aynı gradyanı iki kez ölçekler.
func TestOptimizerRepeatedParam(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	pv, hv := random(rng, 50, 1), random(rng, 50, 1)
	for _, ad := range []string{"adamw", "sgd"} {
		var result [2][]float32
		for c := 0; c < 2; c++ {
			p := twin(pv, true, 50)[c]
			ps := []*tensor.Tensor{p, p}
			step := tensor.NewSGD(ps, 0.1, 0.9).Step
			if ad == "adamw" {
				step = tensor.NewAdamW(ps, 0.01, 0.1).Step
			}
			for k := 0; k < 3; k++ {
				p.ZeroGrad()
				tensor.Mul(p, tensor.NewOn(p.Device(), hv, 50)).Tanh().SumAll().Backward()
				tensor.ClipGradNorm(ps, 0.5)
				step()
			}
			result[c] = p.Data()
		}
		checkDiff(t, ad+" tekrarlı parametre", result[1], result[0], 1e-5)
	}
}

// TestCokTensorluParcali geçici kopya sınırı aşılınca çok tensörlü işin parçalara bölündüğü
// yolu (16 bitlik gradyanlar açılır) CPU ile karşılaştırır.
func TestMultiTensorChunked(t *testing.T) {
	oldLimit, oldType := multiTempLimit, tensor.GradDType()
	multiTempLimit = 64 // her tensörde bir parça
	tensor.SetGradDType(tensor.BFloat16)
	defer func() { multiTempLimit = oldLimit; tensor.SetGradDType(oldType) }()
	rng := rand.New(rand.NewSource(10))
	sizes := []int{30, 70, 9}
	var result [2][][]float32
	for c := 0; c < 2; c++ {
		r := rand.New(rand.NewSource(11))
		var ps []*tensor.Tensor
		for _, n := range sizes {
			ps = append(ps, twin(random(r, n, 1), true, n)[c])
		}
		o := tensor.NewAdamW(ps, 0.01, 0.1)
		for k := 0; k < 2; k++ {
			tensor.ZeroGrads(ps)
			var loss2 *tensor.Tensor
			for i, p := range ps {
				y := tensor.Mul(p, tensor.NewOn(p.Device(), random(rand.New(rand.NewSource(int64(i))), sizes[i], 1), sizes[i])).SumAll()
				if loss2 == nil {
					loss2 = y
				} else {
					loss2 = tensor.Add(loss2, y)
				}
			}
			loss2.Backward()
			tensor.ClipGradNorm(ps, 0.1)
			o.Step()
		}
		for _, p := range ps {
			result[c] = append(result[c], p.Data())
		}
	}
	_ = rng
	for i := range sizes {
		checkDiff(t, fmt.Sprintf("parçalı parametre %d", i), result[1][i], result[0][i], 2e-3)
	}
}

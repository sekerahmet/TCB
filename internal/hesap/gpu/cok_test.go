package gpu

import (
	"fmt"
	"math/rand"
	"testing"

	"tr_dili/internal/hesap/tensor"
)

// TestCokTensorluIyilestirici birden çok parametreyi (biri bir bloğun parçasından uzun, biri
// bfloat16 ana kopyalı, biri gradyansız) tek başlatmada güncelleyen yolu ve gradyan kırpmanın
// toplu ölçeklemesini CPU ile karşılaştırır.
func TestCokTensorluIyilestirici(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	boylar := []int{7, cokParca + 300, 33, 5}
	pv := make([][]float32, len(boylar))
	hv := make([][]float32, len(boylar))
	for i, n := range boylar {
		pv[i], hv[i] = rastgele(rng, n, 1), rastgele(rng, n, 1)
	}
	for _, ad := range []string{"adamw", "sgd momentumlu", "sgd"} {
		var sonuc [2][][]float32
		var normlar [2]float64
		for c := 0; c < 2; c++ {
			var ps []*tensor.Tensor
			for i := range boylar {
				p := ikiz(pv[i], true, boylar[i])[c]
				if i == 2 {
					p = yaprakTurde(p, tensor.BFloat16)
				}
				ps = append(ps, p)
			}
			var adim func()
			switch ad {
			case "adamw":
				adim = tensor.NewAdamW(ps, 0.01, 0.1).Step
			case "sgd":
				adim = tensor.NewSGD(ps, 0.1, 0).Step
			default:
				adim = tensor.NewSGD(ps, 0.1, 0.9).Step
			}
			for k := 0; k < 3; k++ {
				tensor.ZeroGrads(ps)
				var kayip *tensor.Tensor
				for i, p := range ps[:3] { // son parametre gradyan almaz
					h := tensor.NewOn(p.Device(), hv[i], boylar[i])
					y := tensor.Mul(p.Cast(tensor.Float32), h).Tanh().SumAll()
					if kayip == nil {
						kayip = y
					} else {
						kayip = tensor.Add(kayip, y)
					}
				}
				kayip.Backward()
				normlar[c] = tensor.ClipGradNorm(ps, 0.5)
				adim()
			}
			for _, p := range ps {
				sonuc[c] = append(sonuc[c], p.Cast(tensor.Float32).Data())
			}
		}
		for i := range sonuc[0] {
			farkDenetle(t, fmt.Sprintf("%s parametre %d", ad, i), sonuc[1][i], sonuc[0][i], 1e-5)
		}
		if d := normlar[1] - normlar[0]; d > 1e-5*normlar[0] || d < -1e-5*normlar[0] {
			t.Fatalf("%s: gradyan normu GPU %v, CPU %v", ad, normlar[1], normlar[0])
		}
	}
}

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
			var oran *float32
			switch ad {
			case "adamw":
				o := tensor.NewAdamW(ps, 0.01, 0.1)
				adim, oran = o.Step, &o.LR
			case "sgd":
				o := tensor.NewSGD(ps, 0.1, 0)
				adim, oran = o.Step, &o.LR
			default:
				o := tensor.NewSGD(ps, 0.1, 0.9)
				adim, oran = o.Step, &o.LR
			}
			taban := *oran
			for k := 0; k < 3; k++ {
				*oran = taban * []float32{1, 0.3, 2}[k]
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

// TestKirpmaCihazda gradyan kırpmanın cihazdaki yolunu (kareler toplamı, kirp_norm,
// cok_olcek_bellek; konak beklemez) CPU ile karşılaştırır: norm 0 boyutlu GPU tensörüdür,
// kırpılan ve kırpılmayan (ölçek 1: gradyanlar bit bit aynı kalır) durumlar, birden çok
// bloklu ve gradyansız parametre.
func TestKirpmaCihazda(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	boylar := []int{3, 40000, 17, 5}
	hv := make([][]float32, len(boylar))
	for i, n := range boylar {
		hv[i] = rastgele(rng, n, 1)
	}
	for _, enFazla := range []float64{0.5, 1e9} {
		var normlar [2]float32
		var gradlar [2][][]float32
		for c, cihaz := range []tensor.Device{tensor.CPU, gpuCihaz} {
			var ps []*tensor.Tensor
			var kayip *tensor.Tensor
			for i, n := range boylar {
				p := tensor.NewOn(cihaz, make([]float32, n), n).SetRequiresGrad(true)
				ps = append(ps, p)
				if i == len(boylar)-1 {
					continue // son parametre gradyan almaz
				}
				y := tensor.Mul(p, tensor.NewOn(cihaz, hv[i], n)).SumAll()
				if kayip == nil {
					kayip = y
				} else {
					kayip = tensor.Add(kayip, y)
				}
			}
			kayip.Backward()
			norm := tensor.ClipGradNormTensor(ps, enFazla)
			if len(norm.Shape()) != 0 {
				t.Fatalf("norm şekli %v, 0 boyutlu bekleniyordu", norm.Shape())
			}
			if c == 1 && norm.Device() != gpuCihaz {
				t.Fatalf("GPU'da norm %s cihazında, cihazda kalmalıydı", norm.Device())
			}
			normlar[c] = norm.Data()[0]
			for _, p := range ps[:len(ps)-1] {
				gradlar[c] = append(gradlar[c], p.Grad().Data())
			}
		}
		if d := float64(normlar[1] - normlar[0]); d > 1e-6*float64(normlar[0]) || d < -1e-6*float64(normlar[0]) {
			t.Fatalf("en_fazla %g: norm GPU %v, CPU %v", enFazla, normlar[1], normlar[0])
		}
		for i := range gradlar[0] {
			tol := 1e-6
			if enFazla > 1e8 {
				tol = 0 // kırpılmaz: gradyanlar aynen kalır
			}
			farkDenetle(t, fmt.Sprintf("en_fazla %g gradyan %d", enFazla, i), gradlar[1][i], gradlar[0][i], tol)
		}
	}
	// Gradyanı olmayan liste: norm 0.
	p := tensor.NewOn(gpuCihaz, []float32{1}, 1).SetRequiresGrad(true)
	if v := tensor.ClipGradNormTensor([]*tensor.Tensor{p}, 1).Data()[0]; v != 0 {
		t.Fatalf("gradyansız norm %v", v)
	}
}

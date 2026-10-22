package gpu

import (
	"fmt"
	"math/rand"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// microNorm hedef modelin (tools/gpu/rgn512.tr) RMS normlarıdır, bf16, ileri ve geri: blok
// normu [T, 512]; q_norm [T, 8, 64] (bitişik); k_norm [T, 4, 64] (K ile V tek çarpımdan
// gelir ve bölünür: satırlar bitişik, token adımı 512). T bir mikro toplu iştir (32 × 1024).
// Karşılığı tools/gpu/micro_compare.py normlar(). Bant: okunan ve yazılan bayt / süre.
func microNorm(g *backend, band float64, divide int) {
	rng := rand.New(rand.NewSource(7))
	T := 32768 / divide
	type normCase struct {
		ad             string
		shape, strides []int
		n              int // girdinin depolamasının eleman sayısı
	}
	cases := []normCase{
		{fmt.Sprintf("norm blok %d×512", T), []int{T, 512}, []int{512, 1}, T * 512},
		{fmt.Sprintf("norm q %d×8×64", T), []int{T, 8, 64}, []int{512, 64, 1}, T * 512},
		{fmt.Sprintf("norm k adımlı %d×4×64", T), []int{T, 4, 64}, []int{512, 64, 1}, T * 512},
	}
	for _, c := range cases {
		D := c.shape[len(c.shape)-1]
		rows := numel(c.shape) / D
		x := g.FromHost(random(rng, c.n, 1), tensor.BFloat16)
		gamma := g.FromHost(random(rng, D, 1), tensor.Float32)
		gs := g.FromHost(random(rng, rows*D, 1), tensor.BFloat16)
		out := g.AllocUninit(rows*D, tensor.BFloat16)
		gx := g.AllocUninit(rows*D, tensor.BFloat16)
		ggamma := g.AllocUninit(D, tensor.Float32)
		xv := tensor.View{S: x, Shape: c.shape, Strides: c.strides}
		gv := tensor.View{S: gamma, Shape: []int{D}, Strides: []int{1}}
		var rinv tensor.Storage
		e := rows * D
		ms := microMeasure(g, 10, func() {
			if rinv != nil {
				g.Release(rinv)
			}
			rinv = g.RMSNorm(out, xv, &gv, D, 1e-6)
		})
		microWrite(c.ad+" ileri", "kendi", ms, float64(4*e)/1e9/(ms/1e3), fmt.Sprintf("GB/s (%%%.0f)", 100*float64(4*e)/1e9/(ms/1e3)/band))
		ms = microMeasure(g, 10, func() { g.RMSNormGrad(gx, ggamma, gs, xv, &gv, rinv, D) })
		microWrite(c.ad+" geri", "kendi", ms, float64(6*e)/1e9/(ms/1e3), fmt.Sprintf("GB/s (%%%.0f)", 100*float64(6*e)/1e9/(ms/1e3)/band))
		for _, s := range []tensor.Storage{x, gamma, gs, out, gx, ggamma, rinv} {
			g.Release(s)
		}
	}
}

package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (15.22a): sonuç girdinin bellek hizasına ve kartın SM sayısına bağlı olmaz. Aynı değerler
// kendi tensöründen ve önüne 1, 2, 3 öğe konmuş düz bir tensörün görünümünden verilince bitler
// her çekirdek ailesinde aynıdır, kartta (benzetici) da işlemcide de; işlemcide işçi sayısı da
// bitleri değiştirmez (GOMAXPROCS=1 ve öntanımlı).

// shiftedOn aynı değerleri off öğe kayık bir düz tensörün görünümü olarak kurar (off = 0: kendi
// tensörü). 16 bitlik türde kayma 2·off bayttır.
func shiftedOn(dev tensor.Device, data []float32, dt tensor.DType, off int, shape ...int) *tensor.Tensor {
	pad := make([]float32, off+len(data))
	copy(pad[off:], data)
	flat := tensor.NewOn(dev, pad, len(pad))
	if dt != tensor.Float32 {
		flat = flat.Cast(dt)
	}
	return flat.Slice(0, off, off+len(data)).Reshape(shape...)
}

// bitsOf tensörün float32 değerlerinin bitleridir.
func bitsOf(t *tensor.Tensor) []uint32 {
	d := t.Cast(tensor.Float32).Data()
	b := make([]uint32, len(d))
	for i, v := range d {
		b[i] = math.Float32bits(v)
	}
	return b
}

// sameBits iki bit dizisini karşılaştırır, ilk farkı söyler.
func sameBitsAt(t *testing.T, name string, got, want []uint32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d öğe, %d bekleniyordu", name, len(got), len(want))
	}
	diff := 0
	first := -1
	for i := range got {
		if got[i] != want[i] {
			if first < 0 {
				first = i
			}
			diff++
		}
	}
	if diff > 0 {
		t.Errorf("%s: %d öğenin %d'i farklı (ilk %d: %g ile %g)", name, len(got), diff, first,
			math.Float32frombits(got[first]), math.Float32frombits(want[first]))
	}
}

// bitFamily bir çekirdek ailesidir: off kaymalı girdilerle sonucu kurar.
type bitFamily struct {
	name string
	cpu  bool // yalnız işlemcide (kartta bu işlem yok)
	run  func(dev tensor.Device, off int) *tensor.Tensor
}

func bitFamilies(t *testing.T) []bitFamily {
	rng := rand.New(rand.NewSource(71))
	const M, K, N = 5, 64, 36
	x1, xm := random(rng, K, 1), random(rng, M*K, 1)
	w, wt := random(rng, K*N, 1), random(rng, N*K, 1)
	q, kv := random(rng, 2*6*8, 1), random(rng, 2*7*8, 1)
	row := random(rng, 3*130, 3)
	mm := func(name string, a []float32, rows int, b []float32, trans bool, dt tensor.DType) bitFamily {
		return bitFamily{name: name, run: func(dev tensor.Device, off int) *tensor.Tensor {
			x := shiftedOn(dev, a, dt, off, rows, K)
			if trans {
				return tensor.MatMul(x, shiftedOn(dev, b, dt, off, N, K).T())
			}
			return tensor.MatMul(x, shiftedOn(dev, b, dt, off, K, N))
		}}
	}
	fams := []bitFamily{
		mm("@ M = 1", x1, 1, w, false, tensor.Float32),
		mm("@ M = 1, B devrik", x1, 1, wt, true, tensor.Float32),
		mm("@ M > 1", xm, M, w, false, tensor.Float32),
		mm("@ M > 1, B devrik", xm, M, wt, true, tensor.Float32),
		mm("16 bitlik @ M > 1", xm, M, w, false, tensor.BFloat16),
		mm("16 bitlik @ M = 1", x1, 1, w, false, tensor.BFloat16),
		{name: "TR_attention", run: func(dev tensor.Device, off int) *tensor.Tensor {
			return tensor.Attention(shiftedOn(dev, q, tensor.Float32, off, 6, 2, 8),
				shiftedOn(dev, kv, tensor.Float32, off, 7, 2, 8), shiftedOn(dev, kv, tensor.Float32, off, 7, 2, 8),
				tensor.AttentionOptions{Causal: true})
		}},
		{name: "TR_sum (eksen)", run: func(dev tensor.Device, off int) *tensor.Tensor {
			return shiftedOn(dev, row, tensor.Float32, off, 3, 130).Sum(1, false)
		}},
		{name: "TR_sum (hepsi)", run: func(dev tensor.Device, off int) *tensor.Tensor {
			return shiftedOn(dev, row, tensor.Float32, off, 3, 130).SumAll()
		}},
		{name: "TR_max", run: func(dev tensor.Device, off int) *tensor.Tensor {
			return shiftedOn(dev, row, tensor.Float32, off, 3, 130).Max(1, false)
		}},
		{name: "TR_logsumexp", cpu: true, run: func(dev tensor.Device, off int) *tensor.Tensor {
			return tensor.LogSumExp(shiftedOn(dev, row, tensor.Float32, off, 3, 130), []int{1}, false)
		}},
		{name: "üstel (öğe öğe)", run: func(dev tensor.Device, off int) *tensor.Tensor {
			return shiftedOn(dev, row, tensor.Float32, off, 3, 130).Exp()
		}},
	}
	for _, dt := range quantTypes {
		elems, _ := dt.QuantBlock()
		KQ := max(elems, 2*K)
		host, card := quantWeights(t, rng, dt, N, KQ)
		xq1, xqm := random(rng, KQ, 1), random(rng, M*KQ, 1)
		for _, c := range []struct {
			rows int
			x    []float32
		}{{1, xq1}, {M, xqm}} {
			c := c
			fams = append(fams, bitFamily{name: fmt.Sprintf("nicem %s M = %d", dt.Name(), c.rows),
				run: func(dev tensor.Device, off int) *tensor.Tensor {
					wq := host
					if dev != tensor.CPU {
						wq = card
					}
					return tensor.MatMul(shiftedOn(dev, c.x, tensor.Float32, off, c.rows, KQ), wq.T())
				}})
		}
	}
	return fams
}

func TestMisalignedViewsMatchBits(t *testing.T) {
	fams := bitFamilies(t)
	for _, dev := range []tensor.Device{tensor.CPU, gpuDevice} {
		for _, f := range fams {
			if f.cpu && dev != tensor.CPU {
				continue
			}
			want := bitsOf(f.run(dev, 0))
			for off := 1; off <= 3; off++ {
				sameBitsAt(t, fmt.Sprintf("%s %s, %d öğe kayık", dev, f.name, off), bitsOf(f.run(dev, off)), want)
			}
			if dev == tensor.CPU {
				prev := runtime.GOMAXPROCS(1)
				one := bitsOf(f.run(dev, 1))
				runtime.GOMAXPROCS(prev)
				sameBitsAt(t, fmt.Sprintf("%s %s, GOMAXPROCS=1", dev, f.name), one, want)
			}
		}
	}
}

// TestSmCountDoesNotChangeBits: kendi çekirdeklerin ayarı yalnız boydan seçilir; SM 1 ve 128'lik
// benzeticide tek satırlık çarpım (düz ve devrik B), norm geri yönü (LayerNorm ve RMSNorm, γ'nın
// türevi) ve döşemeli çarpım bit bit aynıdır (15.22a).
func TestSmCountDoesNotChangeBits(t *testing.T) {
	g := gpuBackend()
	sd, ok := g.sr.(*simDriver)
	if !ok {
		t.Skip("SM sayısı yalnız benzeticide değiştirilebilir")
	}
	rng := rand.New(rand.NewSource(73))
	const K, N, R, D = 96, 40, 300, 48
	x1, w, wt := random(rng, K, 1), random(rng, K*N, 1), random(rng, N*K, 1)
	xm := random(rng, 70*K, 1)
	xs, gm := random(rng, R*D, 1), random(rng, D, 1)
	run := func() [][]uint32 {
		var out [][]uint32
		x := tensor.NewOn(gpuDevice, x1, 1, K)
		out = append(out, bitsOf(tensor.MatMul(x, tensor.NewOn(gpuDevice, w, K, N))))
		out = append(out, bitsOf(tensor.MatMul(x, tensor.NewOn(gpuDevice, wt, N, K).T())))
		out = append(out, bitsOf(tensor.MatMul(tensor.NewOn(gpuDevice, xm, 70, K), tensor.NewOn(gpuDevice, w, K, N))))
		for _, layer := range []bool{true, false} {
			xv := tensor.NewOn(gpuDevice, xs, R, D).SetRequiresGrad(true)
			gv := tensor.NewOn(gpuDevice, gm, D).SetRequiresGrad(true)
			var y *tensor.Tensor
			if layer {
				y = tensor.LayerNorm(xv, gv, nil, 1e-5)
			} else {
				y = tensor.RMSNorm(xv, gv, 1e-5)
			}
			tensor.Mul(y, y).SumAll().Backward()
			out = append(out, bitsOf(gv.Grad()), bitsOf(xv.Grad()))
		}
		return out
	}
	old := sd.smCount
	defer func() { sd.smCount = old }()
	sd.smCount = 1
	one := run()
	sd.smCount = 128
	many := run()
	names := []string{"gemv", "gemv devrik", "döşemeli çarpım", "LayerNorm γ'", "LayerNorm x'", "RMSNorm γ'", "RMSNorm x'"}
	for i := range one {
		sameBitsAt(t, names[i]+": SM 128 ile SM 1", many[i], one[i])
	}
}

// TestGemmPaddingKeepsNegativeZero: K dışındaki dolgu adımı −0'ı +0'a çevirmez. K = 24 (8'in katı,
// 16'nın değil): BK = 16'lık döşeme 8 dolgu adımı atar, BK = 8'lik hiç atmaz. Bütün çarpımları
// sıfıra yuvarlanan küçük eksi sayılar (−1e-30 · 1e-30) olan öğe −0'da biter; iki ayarda da −0'dır
// (+0'lık dolgu fma(0, 0, −0) = +0 yapardı).
func TestGemmPaddingKeepsNegativeZero(t *testing.T) {
	g := gpuBackend()
	const M, K, N = 3, 24, 5
	a := make([]float32, M*K)
	b := make([]float32, K*N)
	for i := range a {
		a[i] = -1e-30
	}
	for i := range b {
		b[i] = 1e-30
	}
	defer g.gemmTile(nil)
	var got [][]uint32
	for _, cfg := range []gemmConfig{defaultGemm, {64, 64, 8, 4, 4}} {
		g.gemmTile(&cfg)
		got = append(got, bitsOf(tensor.MatMul(tensor.NewOn(gpuDevice, a, M, K), tensor.NewOn(gpuDevice, b, K, N))))
	}
	sameBitsAt(t, "BK 8 ile BK 16", got[1], got[0])
	for i, v := range got[0] {
		if v != math.Float32bits(float32(math.Copysign(0, -1))) {
			t.Fatalf("öğe %d: %g, −0 bekleniyordu", i, math.Float32frombits(v))
		}
	}
}

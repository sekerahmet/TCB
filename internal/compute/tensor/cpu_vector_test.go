package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// Vektör çekirdekleri (GOAMD64=v3'te AVX2, aksi halde Go eşleri) taşınabilir Go
// eşleriyle bit düzeyinde aynı sonucu vermeli: v1 ve v3 derlemeleri aynı
// sonucu üretir. Kuyruklu (D mod 8 ≠ 0) ve seçeneksiz (γ/β/gradyan yok) yollar
// dahil.

func equalBits32(t *testing.T, ad string, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: uzunluk %d, beklenen %d", ad, len(got), len(want))
	}
	for i := range got {
		if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
			t.Fatalf("%s [%d]: %v (%#x), beklenen %v (%#x)", ad, i, got[i], math.Float32bits(got[i]), want[i], math.Float32bits(want[i]))
		}
	}
}

func equalBits64(t *testing.T, ad string, got, want []float64) {
	t.Helper()
	for i := range got {
		if math.Float64bits(got[i]) != math.Float64bits(want[i]) {
			t.Fatalf("%s [%d]: %v, beklenen %v", ad, i, got[i], want[i])
		}
	}
}

func TestNormVectorMatchesGo(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	slice := func(n int, shift float64) []float32 {
		d := make([]float32, n)
		for i := range d {
			d[i] = float32(rng.NormFloat64()*1.5 + shift)
		}
		return d
	}
	t.Logf("çekirdek: %s", microKernelName)
	for _, D := range []int{1, 3, 7, 8, 9, 15, 16, 17, 31, 64, 67, 131, 512} {
		for _, R := range []int{1, 2, 5} {
			for choice := 0; choice < 8; choice++ {
				rms := choice&4 != 0
				x := slice(R*D, 0.7)
				var g, b []float32
				if choice&1 != 0 {
					g = slice(D, 1)
				}
				if choice&2 != 0 && !rms {
					b = slice(D, 0)
				}
				y1, y2 := make([]float32, R*D), make([]float32, R*D)
				rs1, rs2 := make([]float32, R), make([]float32, R)
				var m1, m2 []float32
				if !rms {
					m1, m2 = make([]float32, R), make([]float32, R)
				}
				lnForwardBlock(y1, x, g, b, m1, rs1, D, 1e-5)
				lnForwardBlockGo(y2, x, g, b, m2, rs2, D, 1e-5)
				equalBits32(t, "ileri y", y1, y2)
				equalBits32(t, "ileri rstd", rs1, rs2)
				equalBits32(t, "ileri mean", m1, m2)

				gr := slice(R*D, 0)
				for hasXG := 0; hasXG < 2; hasXG++ {
					var xg1, xg2 []float32
					if hasXG == 1 {
						xg1, xg2 = slice(R*D, 0), make([]float32, R*D)
						copy(xg2, xg1)
					}
					pg1, pg2 := make([]float64, D), make([]float64, D)
					pb1, pb2 := make([]float64, D), make([]float64, D)
					for i := range pg1 {
						pg1[i] = rng.Float64()
						pg2[i], pb1[i], pb2[i] = pg1[i], -pg1[i], -pg1[i]
					}
					if choice&2 != 0 {
						pb1, pb2 = nil, nil
					}
					lnBackwardBlock(xg1, pg1, pb1, x, gr, g, m1, rs1, D)
					lnBackwardBlockGo(xg2, pg2, pb2, x, gr, g, m2, rs2, D)
					equalBits32(t, "geri xg", xg1, xg2)
					equalBits64(t, "geri pg", pg1, pg2)
					equalBits64(t, "geri pb", pb1, pb2)
				}
			}
		}
	}
}

// normSummary sabit girdilerle katman/RMS normu ileri ve geri sonuçlarının bit özeti.
func normSummary() uint64 {
	rng := rand.New(rand.NewSource(5))
	h := uint64(14695981039346656037)
	add := func(b uint64) { h = (h ^ b) * 1099511628211 }
	for _, D := range []int{5, 64, 67} {
		R := 9
		x, gr, g, b := make([]float32, R*D), make([]float32, R*D), make([]float32, D), make([]float32, D)
		for _, s := range [][]float32{x, gr, g, b} {
			for i := range s {
				s[i] = float32(rng.NormFloat64())
			}
		}
		for _, rms := range []bool{false, true} {
			y, rs, xg := make([]float32, R*D), make([]float32, R), make([]float32, R*D)
			var mean []float32
			if !rms {
				mean = make([]float32, R)
			}
			pg, pb := make([]float64, D), make([]float64, D)
			lnForwardBlock(y, x, g, b, mean, rs, D, 1e-5)
			lnBackwardBlock(xg, pg, pb, x, gr, g, mean, rs, D)
			for _, s := range [][]float32{y, rs, mean, xg} {
				for _, v := range s {
					add(uint64(math.Float32bits(v)))
				}
			}
			for _, s := range [][]float64{pg, pb} {
				for _, v := range s {
					add(math.Float64bits(v))
				}
			}
		}
	}
	return h
}

// GOAMD64=v1 (Go eşleri) ve v3 (AVX2) derlemeleri aynı bitleri üretmeli; özet iki
// derlemede de ölçülüp sabitlenmiştir.
func TestNormSameAcrossBuilds(t *testing.T) {
	const expected = 0xfe0296651a382150
	if h := normSummary(); h != expected {
		t.Fatalf("norm çekirdeklerinin bit özeti %#x, beklenen %#x (%s)", h, uint64(expected), microKernelName)
	}
}

// Vektör çekirdeği satır dışına (komşu bellek) yazmamalı: kuyruk maskeli saklanır.
func TestNormVectorNoOverflow(t *testing.T) {
	for _, D := range []int{1, 5, 13, 64, 67} {
		R := 3
		buffer := make([]float32, R*D+16)
		for i := range buffer {
			buffer[i] = 42
		}
		x := make([]float32, R*D)
		for i := range x {
			x[i] = float32(i%7) - 3
		}
		y := buffer[:R*D]
		lnForwardBlock(y, x, nil, nil, make([]float32, R), make([]float32, R), D, 1e-5)
		for i := R * D; i < len(buffer); i++ {
			if buffer[i] != 42 {
				t.Fatalf("D=%d: ileri yön satır dışına yazdı ([%d] = %v)", D, i, buffer[i])
			}
		}
		pg := make([]float64, D+8)
		for i := range pg {
			pg[i] = 7
		}
		xg := make([]float32, R*D+16)
		lnBackwardBlock(xg[:R*D], pg[:D], nil, x, x, nil, make([]float32, R), make([]float32, R), D)
		for i := D; i < len(pg); i++ {
			if pg[i] != 7 {
				t.Fatalf("D=%d: geri yön ölçek gradyanını taşırdı ([%d] = %v)", D, i, pg[i])
			}
		}
		for i := R * D; i < len(xg); i++ {
			if xg[i] != 0 {
				t.Fatalf("D=%d: geri yön girdi gradyanını taşırdı ([%d] = %v)", D, i, xg[i])
			}
		}
	}
}

// GELU vektör çekirdekleri Go eşiyle aynı işlem sırasını izler; yalnızca üstel
// 1-2 ulp farklı olabildiğinden sonuçlar birkaç ulp içinde olmalı. Ayrıca float64
// başvurusuna yakın olmalı ve özel değerler (NaN, ±sonsuz, çok büyük |x|) ile
// kuyruk uzunlukları doğru işlenmeli; kuyruk dışına yazılmamalı.
func TestGeluVectorMatchesGo(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	for _, n := range []int{1, 7, 8, 9, 31, 256, 1000} {
		x := make([]float32, n)
		for i := range x {
			x[i] = float32(rng.NormFloat64() * 3)
		}
		if n >= 9 {
			x[0], x[1], x[2], x[3] = float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)), 0
			x[4], x[5], x[6], x[7] = 2e19, -2e19, 12, -12
		}
		y1, y2 := make([]float32, n+8), make([]float32, n)
		for i := range y1 {
			y1[i] = 42
		}
		geluForwardVec(y1[:n], x)
		geluForwardGo(y2, x)
		for i := n; i < len(y1); i++ {
			if y1[i] != 42 {
				t.Fatalf("n=%d: ileri yön dizinin dışına yazdı", n)
			}
		}
		g := make([]float32, n)
		for i := range g {
			g[i] = float32(rng.NormFloat64())
		}
		gx1, gx2 := make([]float32, n), make([]float32, n)
		for i := range gx1 {
			gx1[i], gx2[i] = 0.25, 0.25
		}
		geluBackwardVec(gx1, g, x)
		geluBackwardGo(gx2, g, x)
		for i, v := range x {
			for _, c := range []struct {
				ad   string
				a, b float32
				ref  float64
			}{
				{"ileri", y1[i], y2[i], geluRef(float64(v))},
				{"geri", gx1[i], gx2[i], 0.25 + float64(g[i])*geluDerivRef(float64(v))},
			} {
				if c.a != c.a || c.b != c.b {
					if (c.a != c.a) != (c.b != c.b) {
						t.Fatalf("%s n=%d x=%v: vektör %v, Go eşi %v", c.ad, n, v, c.a, c.b)
					}
					continue
				}
				if c.a != c.b && (math.IsInf(float64(c.a), 0) || math.IsInf(float64(c.b), 0) || ulpDiff(c.a, c.b) > 4) {
					t.Fatalf("%s n=%d x=%v: vektör %v, Go eşi %v", c.ad, n, v, c.a, c.b)
				}
				if !math.IsNaN(c.ref) && !math.IsInf(c.ref, 0) && math.Abs(float64(c.a)-c.ref) > 1e-6+3e-6*math.Abs(c.ref) {
					t.Fatalf("%s n=%d x=%v: %v, başvuru %v", c.ad, n, v, c.a, c.ref)
				}
			}
		}
	}
}

// Çapraz entropinin satır çekirdekleri Go eşleriyle aynı işlem sırasını izler:
// en büyük tam olarak aynı, toplam ve gradyan birkaç ulp içinde olmalı; NaN
// girdiler en büyükte yok sayılır, kuyruk dışına yazılmaz.
func TestCrossEntropyVectorMatchesGo(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	for _, n := range []int{1, 3, 8, 9, 16, 131, 1000} {
		x := make([]float32, n)
		for i := range x {
			x[i] = float32(rng.NormFloat64() * 4)
		}
		if n >= 9 {
			x[n-1] = float32(math.NaN())
		}
		m1, s1 := ceRow(x)
		m2, s2 := ceRowGo(x)
		if math.Float32bits(m1) != math.Float32bits(m2) {
			t.Fatalf("n=%d: en büyük %v, Go eşi %v", n, m1, m2)
		}
		if n >= 9 {
			if s1 == s1 || s2 == s2 {
				t.Fatalf("n=%d: NaN girdili toplam NaN olmalı: %v, %v", n, s1, s2)
			}
			x[n-1] = 0
			m1, s1 = ceRow(x)
			m2, s2 = ceRowGo(x)
		}
		var ref float64
		for _, v := range x {
			ref += math.Exp(float64(v) - float64(m2))
		}
		if math.Abs(s1-s2) > 4e-7*s2 || math.Abs(s1-ref) > 4e-7*ref {
			t.Fatalf("n=%d: toplam %v, Go eşi %v, başvuru %v", n, s1, s2, ref)
		}
		l := float32(float64(m1) + math.Log(s1))
		h1, h2 := make([]float32, n+8), make([]float32, n)
		for i := range h1 {
			h1[i] = 42
			if i < n {
				h2[i] = 42
			}
		}
		ceBackward(h1[:n], x, l, 0.375)
		ceBackwardGo(h2, x, l, 0.375)
		for i := range h1 {
			if i >= n {
				if h1[i] != 42 {
					t.Fatalf("n=%d: geri yön dizinin dışına yazdı", n)
				}
				continue
			}
			if ulpDiff(h1[i], h2[i]) > 2 {
				t.Fatalf("n=%d [%d]: gradyan %v, Go eşi %v", n, i, h1[i], h2[i])
			}
		}
	}
}

func geluRef(x float64) float64 {
	return 0.5 * x * (1 + math.Tanh(geluC*(x+geluA*x*x*x)))
}

func geluDerivRef(x float64) float64 {
	if math.Abs(x) > 20 {
		if x > 0 {
			return 1
		}
		return 0
	}
	th := math.Tanh(geluC * (x + geluA*x*x*x))
	return 0.5*(1+th) + 0.5*x*(1-th*th)*geluC*(1+3*geluA*x*x)
}

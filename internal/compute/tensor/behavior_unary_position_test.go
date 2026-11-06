package tensor

import (
	"math"
	"math/rand"
	"testing"
)

// sameBitsOrNaN iki değer de NaN ise eşit sayar; NaN yükü konuma bağlı değildir.
func sameBitsOrNaN(a, b float32) bool {
	if a != a && b != b {
		return true
	}
	return math.Float32bits(a) == math.Float32bits(b)
}

// TestUnaryOpsIgnorePosition: RunFused satırı iki parçaya bölebilir (fuse_cpu.go); bitler buna
// dayanır: her tekli işlem, ileri ve geri, bütün dilimle rastgele kesilmiş dilimlerde aynı biti verir.
func TestUnaryOpsIgnorePosition(t *testing.T) {
	r := rand.New(rand.NewSource(212))
	n := 4099
	x, g := make([]float32, n), make([]float32, n)
	sp := []float32{0, float32(math.Copysign(0, -1)), 1e-45, -1e-45, 1e-30, 88.7, -103, 1e30, -1e30, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN()), 5.5, -5.5, 0.5}
	for i := range x {
		g[i] = float32(r.NormFloat64())
		switch r.Intn(4) {
		case 0:
			x[i] = sp[r.Intn(len(sp))]
		case 1:
			x[i] = float32(r.NormFloat64() * 30)
		default:
			x[i] = float32(r.NormFloat64())
		}
	}
	bad := 0
	for op := UnOp(0); op < numUnOps; op++ {
		for _, p := range []float32{0.5, 1, 2, 1.7} {
			whole := make([]float32, n)
			unaryLoop(op, p, whole, x)
			gw := make([]float32, n)
			unaryGradLoop(op, p, gw, g, x, whole)
			for rep := 0; rep < 30; rep++ {
				y, gy := make([]float32, n), make([]float32, n)
				for lo := 0; lo < n; {
					hi := min(n, lo+1+r.Intn(700))
					unaryLoop(op, p, y[lo:hi], x[lo:hi])
					unaryGradLoop(op, p, gy[lo:hi], g[lo:hi], x[lo:hi], whole[lo:hi])
					lo = hi
				}
				for i := range y {
					if !sameBitsOrNaN(y[i], whole[i]) {
						t.Errorf("op %d p %v ileri [%d] x=%v: %v / %v", op, p, i, x[i], y[i], whole[i])
						bad++
						break
					}
					if !sameBitsOrNaN(gy[i], gw[i]) {
						t.Errorf("op %d p %v geri [%d] x=%v: %v / %v", op, p, i, x[i], gy[i], gw[i])
						bad++
						break
					}
				}
				if bad > 20 {
					t.FailNow()
				}
			}
		}
	}
}

package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// TestMmaFragmentLayout mma.sync.m16n8k16 parça düzenini (sim_warp.go) ve pack2'yi sınar: tek
// warp, A (16×16) ve B (16×8) float32'den 16 bitlik çiftlere paketlenir, D = A·B + C yazılır ve
// 16 bite yuvarlanmış girdilerle float64'te hesaplanan başvuruyla karşılaştırılır. Gerçek GPU'da
// (sm_80 ve üstü) da koşar: benzetimin düzeni donanımla aynı olmalıdır.
func TestMmaFragmentLayout(t *testing.T) {
	g := gpuBackend()
	if !inSim() {
		if _, sm := g.sr.capability(); sm < 80 {
			t.Skip("tensör çekirdeği sm_80 ister")
		}
	}
	rng := rand.New(rand.NewSource(41))
	for _, f16 := range []bool{false, true} {
		dt := tensor.BFloat16
		if f16 {
			dt = tensor.Float16
		}
		A, B, C := random(rng, 16*16, 1), random(rng, 16*8, 1), random(rng, 16*8, 1)
		ad := fmt.Sprintf("mma_duzen_%v", f16)
		ga, gb, gc := g.FromHost(A, tensor.Float32), g.FromHost(B, tensor.Float32), g.FromHost(C, tensor.Float32)
		g.enter()
		gd := g.allocate(16*8, tensor.Float32, true)
		ks := g.takeKernel(ad, func() *emitter {
			k := newEmitter(ad)
			k.target = 80
			pa, pb, pc, pd := k.pointer("a"), k.pointer("b"), k.pointer("c"), k.written("d")
			lane := k.special("%tid.x")
			gr, q := k.r(), k.r()
			k.e("shr.u32", gr, lane, 2)
			k.e("and.b32", q, lane, 3)
			at := func(p reg, row, col reg, rowOfs, colOfs, width int) reg {
				i := k.r()
				k.e("add.s32", i, row, rowOfs)
				c := k.r()
				k.e("add.s32", c, col, colOfs)
				k.e("mad.lo.s32", i, i, width, c)
				v := k.f()
				k.e("ld.global.f32", v, mem(k.address(p, i, 4), 0))
				return v
			}
			q2 := k.r()
			k.e("shl.b32", q2, q, 1)
			var m mmaParts
			for i, o := range [4][2]int{{0, 0}, {8, 0}, {0, 8}, {8, 8}} {
				m.a[i] = k.pack2(at(pa, gr, q2, o[0], o[1], 16), at(pa, gr, q2, o[0], o[1]+1, 16), f16)
			}
			for i := range 2 {
				// B(k, n): k = 2q (+8), n = g; B satır öncelikli 16×8.
				m.b[i] = k.pack2(at(pb, q2, gr, 8*i, 0, 8), at(pb, q2, gr, 8*i+1, 0, 8), f16)
			}
			for i := range 4 {
				m.c[i] = at(pc, gr, q2, 8*(i/2), i%2, 8)
			}
			k.mma(m.c, m.c, m.a, m.b, f16)
			for i := range 4 {
				r, c := k.r(), k.r()
				k.e("add.s32", r, gr, 8*(i/2))
				k.e("add.s32", c, q2, i%2)
				k.e("mad.lo.s32", r, r, 8, c)
				k.store(tensor.Float32, k.address(pd, r, 4), m.c[i])
			}
			return k
		})
		g.launch(ks, [3]uint32{1, 1, 1}, [3]uint32{32, 1, 1}, g.param().read(g.storageOf(ga), 0).read(g.storageOf(gb), 0).read(g.storageOf(gc), 0).write(gd, 0))
		g.exit()
		got := g.ToHost(tensor.View{S: gd, Shape: []int{128}, Strides: []int{1}})
		round := func(v float32) float64 { return float64(tensor.FromBits16(dt, tensor.Bits16(dt, v))) }
		for r := range 16 {
			for c := range 8 {
				want := float64(C[r*8+c])
				for kk := range 16 {
					want += round(A[r*16+kk]) * round(B[kk*8+c])
				}
				if d := math.Abs(float64(got[r*8+c]) - want); d > 1e-4*(1+math.Abs(want)) {
					t.Fatalf("%v: D[%d][%d] = %v, beklenen %v", dt, r, c, got[r*8+c], want)
				}
			}
		}
	}
}

// TestShflXor shfl.sync.bfly'yi sınar: şerit l, l ^ 5 şeridinin değerini alır.
func TestShflXor(t *testing.T) {
	g := gpuBackend()
	g.enter()
	d := g.allocate(32, tensor.Float32, true)
	ks := g.takeKernel("shfl_xor", func() *emitter {
		k := newEmitter("shfl_xor")
		pd := k.written("d")
		lane := k.special("%tid.x")
		v := k.f()
		k.e("cvt.rn.f32.u32", v, lane)
		k.store(tensor.Float32, k.address(pd, lane, 4), k.shflXor(v, 5))
		return k
	})
	g.launch(ks, [3]uint32{1, 1, 1}, [3]uint32{32, 1, 1}, g.param().write(d, 0))
	g.exit()
	got := g.ToHost(tensor.View{S: d, Shape: []int{32}, Strides: []int{1}})
	for l := range 32 {
		if got[l] != float32(l^5) {
			t.Fatalf("şerit %d: %v, beklenen %d", l, got[l], l^5)
		}
	}
}

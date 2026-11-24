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
		if cc, _ := g.sr.capability(); cc < 80 {
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

// int8Codes [-128, 127] aralığında rastgele tam sayılardır (float32 olarak; uçlar dahil).
func int8Codes(rng *rand.Rand, n int) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(rng.Intn(256) - 128)
	}
	v[0], v[n-1] = -128, 127
	return v
}

// packS8 p[base + i·step] (i = 0..3) float32 tam sayılarını bir sözcüğün dört işaretli baytına paketler.
func packS8(k *emitter, p, base reg, step int) reg {
	w := k.whole(0)
	for i := range 4 {
		at := k.r()
		k.e("add.s32", at, base, i*step)
		f, c := k.f(), k.r()
		k.e("ld.global.f32", f, mem(k.address(p, at, 4), 0))
		k.e("cvt.rzi.s32.f32", c, f)
		k.e("and.b32", c, c, 0xFF)
		k.e("shl.b32", c, c, 8*i)
		k.e("or.b32", w, w, c)
	}
	return w
}

// TestMmaS8FragmentLayout mma.sync'in int8 biçimlerini (m16n8k32 ve m16n8k16 .s32.s8.s8.s32;
// sim_warp.go) sınar: tek warp, A (16×kk) ve B (kk×8) işaretli baytlara paketlenir, D = A·B + C
// tamsayıda kesin başvuruyla karşılaştırılır. Parça düzeni: A a0 (g, 4t..), a1 (g+8, 4t..), k32'de
// a2 (g, 16+4t..), a3 (g+8, 16+4t..); B b0 (k 4t.., n g), k32'de b1 (k 16+4t.., n g); C c0 (g, 2t),
// c1 (g, 2t+1), c2 (g+8, 2t), c3 (g+8, 2t+1). Gerçek GPU'da (sm_80 ve üstü) da koşar.
func TestMmaS8FragmentLayout(t *testing.T) {
	g := gpuBackend()
	if !inSim() {
		if cc, _ := g.sr.capability(); cc < 80 {
			t.Skip("int8 mma.sync sm_80 ister")
		}
	}
	rng := rand.New(rand.NewSource(44))
	for _, kk := range []int{32, 16} {
		A, B := int8Codes(rng, 16*kk), int8Codes(rng, kk*8)
		C := make([]float32, 16*8)
		for i := range C {
			C[i] = float32(rng.Intn(2001) - 1000)
		}
		ad := fmt.Sprintf("mma_s8_duzen_%d", kk)
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
			q4 := k.r()
			k.e("shl.b32", q4, q, 2)
			var a, b []reg
			for i := range kk / 8 {
				// A(row, col): row = g (+8), col = 4t (+16); satır öncelikli 16×kk.
				at := k.r()
				k.e("add.s32", at, gr, 8*(i%2))
				k.e("mad.lo.s32", at, at, kk, q4)
				k.e("add.s32", at, at, 16*(i/2))
				a = append(a, packS8(k, pa, at, 1))
			}
			for i := range kk / 16 {
				// B(k, n): k = 4t (+16), n = g; satır öncelikli kk×8, öğeler k boyunca (adım 8).
				at := k.r()
				k.e("add.s32", at, q4, 16*i)
				k.e("mad.lo.s32", at, at, 8, gr)
				b = append(b, packS8(k, pb, at, 8))
			}
			var c [4]reg
			idx := func(i int) reg {
				r, col := k.r(), k.r()
				k.e("add.s32", r, gr, 8*(i/2))
				k.e("shl.b32", col, q, 1)
				k.e("add.s32", col, col, i%2)
				k.e("mad.lo.s32", r, r, 8, col)
				return r
			}
			for i := range c {
				f := k.f()
				k.e("ld.global.f32", f, mem(k.address(pc, idx(i), 4), 0))
				c[i] = k.r()
				k.e("cvt.rzi.s32.f32", c[i], f)
			}
			shape := "m16n8k32"
			if kk == 16 {
				shape = "m16n8k16"
			}
			k.e("mma.sync.aligned."+shape+".row.col.s32.s8.s8.s32", vlist(c[:]), vlist(a), vlist(b), vlist(c[:]))
			for i := range c {
				k.store(tensor.Float32, k.address(pd, idx(i), 4), k.toFloat(c[i]))
			}
			return k
		})
		g.launch(ks, [3]uint32{1, 1, 1}, [3]uint32{32, 1, 1}, g.param().read(g.storageOf(ga), 0).read(g.storageOf(gb), 0).read(g.storageOf(gc), 0).write(gd, 0))
		g.exit()
		got := g.ToHost(tensor.View{S: gd, Shape: []int{128}, Strides: []int{1}})
		for r := range 16 {
			for c := range 8 {
				want := int(C[r*8+c])
				for x := range kk {
					want += int(A[r*kk+x]) * int(B[x*8+c])
				}
				if int(got[r*8+c]) != want {
					t.Fatalf("k%d: D[%d][%d] = %v, beklenen %d", kk, r, c, got[r*8+c], want)
				}
			}
		}
	}
}

// TestDp4a dp4a.s32.s32'yi sınar: d = c + Σ a'nın ve b'nin aynı sıradaki işaretli baytlarının çarpımı;
// uçlar (−128·−128, 127·−128) dahil. Gerçek GPU'da (sm_61 ve üstü) da koşar.
func TestDp4a(t *testing.T) {
	g := gpuBackend()
	if !inSim() {
		if cc, _ := g.sr.capability(); cc < 61 {
			t.Skip("dp4a sm_61 ister")
		}
	}
	rng := rand.New(rand.NewSource(45))
	A, B := int8Codes(rng, 32*4), int8Codes(rng, 32*4)
	A[4], B[4], A[5], B[5] = -128, -128, 127, -128
	C := make([]float32, 32)
	for i := range C {
		C[i] = float32(rng.Intn(200001) - 100000)
	}
	ga, gb, gc := g.FromHost(A, tensor.Float32), g.FromHost(B, tensor.Float32), g.FromHost(C, tensor.Float32)
	g.enter()
	gd := g.allocate(32, tensor.Float32, true)
	ks := g.takeKernel("dp4a_sinama", func() *emitter {
		k := newEmitter("dp4a_sinama")
		k.target = 61
		pa, pb, pc, pd := k.pointer("a"), k.pointer("b"), k.pointer("c"), k.written("d")
		lane := k.special("%tid.x")
		at := k.r()
		k.e("shl.b32", at, lane, 2)
		a, b := packS8(k, pa, at, 1), packS8(k, pb, at, 1)
		f, c := k.f(), k.r()
		k.e("ld.global.f32", f, mem(k.address(pc, lane, 4), 0))
		k.e("cvt.rzi.s32.f32", c, f)
		k.e("dp4a.s32.s32", c, a, b, c)
		k.store(tensor.Float32, k.address(pd, lane, 4), k.toFloat(c))
		return k
	})
	g.launch(ks, [3]uint32{1, 1, 1}, [3]uint32{32, 1, 1}, g.param().read(g.storageOf(ga), 0).read(g.storageOf(gb), 0).read(g.storageOf(gc), 0).write(gd, 0))
	g.exit()
	got := g.ToHost(tensor.View{S: gd, Shape: []int{32}, Strides: []int{1}})
	for l := range 32 {
		want := int(C[l])
		for i := range 4 {
			want += int(A[4*l+i]) * int(B[4*l+i])
		}
		if int(got[l]) != want {
			t.Fatalf("şerit %d: %v, beklenen %d", l, got[l], want)
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

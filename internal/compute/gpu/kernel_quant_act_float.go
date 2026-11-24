package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// quantActNonFiniteKernel sonlu olmayan değer taşıyan satırların (fin[m] = 1) float yoludur: CPU'da quantGemmQ8 ve
// quantGemvQ8 böyle satırı quantGemv'e verir; burada her çıktı AVX2'siz işlemcideki Go eşinin (dotQuantGo:
// dequantBlocks + dot32) sırasıyla toplanır (defter 6): sekiz toplayıcı, öğe j toplayıcı j%8'e (çarpma ve toplama
// ayrı yuvarlanır), sonda ((s0+s1)+(s2+s3))+((s4+s5)+(s6+s7)). Ağırlığın öğesi kesin açılır (quantElem). Söz NaN
// olmayı ve sonsuzun işaretini kapsar, bitleri kapsamaz (15.53a). AVX2'li işlemci FMA'lı 32 şeritle toplar
// (sdot32): taşan ara toplamlı satırda o işlemciyle ayrılabilir. Izgara (N/128, M); sonlu satırda blok hemen biter.
func quantActNonFiniteKernel(dt tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("nicem_etk_sonsuz_%s", dt.Name()))
		pa, pw, pfin := k.pointer("a"), k.pointer("w"), k.pointer("fin")
		pc := k.written("c")
		N, K := k.u32p("n"), k.u32p("k")
		row := k.special("%ctaid.y")
		end := k.label()
		f, p := k.r(), k.p()
		k.e("ld.global.u32", f, mem(k.address(pfin, row, 4), 0))
		k.e("setp.eq.u32", p, f, 0)
		k.jumpIf(p, end)
		n := k.globalIndex()
		k.e("setp.ge.u32", p, n, N)
		k.jumpIf(p, end)
		_, size := dt.QuantBlock()
		elems, _ := dt.QuantBlock()
		rowBytes, wrow := k.r(), k.rd()
		k.e("shr.u32", rowBytes, K, log2(elems))
		k.e("mul.lo.s32", rowBytes, rowBytes, size)
		k.e("mul.wide.u32", wrow, n, rowBytes)
		k.e("add.s64", wrow, wrow, pw)
		xr := k.r()
		k.e("mul.lo.s32", xr, row, K)
		x0 := k.address(pa, xr, 4)
		var s [8]reg
		for j := range s {
			s[j] = k.constant(0)
		}
		e := k.whole(0)
		k.whileLess("u32", e, K, func() {
			xa := k.address(x0, e, 4)
			for j := range s {
				x, ej, v := k.f(), k.r(), k.f()
				k.e("ld.global.f32", x, mem(xa, 4*j))
				k.e("add.s32", ej, e, j)
				k.e("mul.rn.f32", v, x, k.quantElem(dt, wrow, ej))
				k.e("add.rn.f32", s[j], s[j], v)
			}
			k.e("add.s32", e, e, 8)
		})
		a, b, c, d := k.f(), k.f(), k.f(), k.f()
		k.e("add.rn.f32", a, s[0], s[1])
		k.e("add.rn.f32", b, s[2], s[3])
		k.e("add.rn.f32", c, s[4], s[5])
		k.e("add.rn.f32", d, s[6], s[7])
		k.e("add.rn.f32", a, a, b)
		k.e("add.rn.f32", c, c, d)
		k.e("add.rn.f32", a, a, c)
		idx := k.r()
		k.e("mad.lo.s32", idx, row, N, n)
		ad := k.address(pc, idx, 4)
		old := k.f()
		k.e("ld.global.f32", old, mem(ad, 0))
		k.e("add.rn.f32", old, old, a)
		k.e("st.global.f32", mem(ad, 0), old)
		k.put(end)
		return k
	}
}

package gpu

import "fmt"

// ============================================================================
// Etkinleştirmesi nicemlenen çarpım (15.53a'nın TR_quantized_activations bloğu), GPU
// ============================================================================
//
// CPU'nun yoluyla (cpu_quant_q8.go) aynı bitler: etkinleştirme ağırlığın bloğuyla (q4 32, q4_k ve q6_k 256)
// int8'e nicemlenir (quantizeQ8Go'nun formülü), nokta çarpımı tamsayıdadır (mma.sync s8, dp4a ya da dört tamsayı
// çarpma-toplama: tamsayıda kesin, sırası önemsiz), blok başına float32 formülü CPU'daki sırayla tek iş
// parçacığında döner (çarpma ve toplama ayrı yuvarlanır, .rn). Her çıktı öğesinin blokları sırayla toplanır;
// K bölünmez. Yol yalnız kartın yeteneğinden, M'den ve K'dan seçilir, bitleri değiştirmez (backend_quant_act.go).
// Tek satırın çekirdeği satırı kendi nicemler (kernel_quant_act_gemv.go). Sonlu olmayan değer taşıyan satır CPU'daki
// gibi float yoluyla çarpılır (kernel_quant_act_float.go).

const (
	actBlk     = 256 // etkinleştirmenin bloğu (Q4_K, Q6_K)
	actTile    = 64  // çok satırlı çarpımın blok karosu (satır ve sütun)
	actWarpsMM = 4   // karodaki warp: 2 × 2, her biri 32 × 32
	actWarpsMV = 8   // tek satırlı çarpımda blok başına çıktı
)

// quantActQuantizeKernel (M > 1) a'nın satırlarını blk'lık (32: q4, 256: q4_k, q6_k) bloklarla nicemler: q int8 kodlar
// [M, K], d ölçekler [M, K/blk], bs 16'lık alt blokların kod toplamları (int32) [M, K/16]; fin[m] satırda sonlu
// olmayan değer varsa 1 olur (önceden sıfır; çarpım o satırı float yoluyla yapar, quantGemmQ8 gibi). Izgara
// (⌈K/256⌉, M), blok bir warp; şerit 8 öğe alır, blk/8 şeritlik öbek bir bloktur.
func quantActQuantizeKernel(blk int) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("nicem_etk_q8_%d", blk))
		pa := k.pointer("a")
		pq, pd, pbs, pfin := k.written("q"), k.written("d"), k.written("bs"), k.written("fin")
		K := k.u32p("k")
		lane, chunk, row := k.special("%tid.x"), k.special("%ctaid.x"), k.special("%ctaid.y")
		e := k.r() // satırdaki ilk öğe: chunk·256 + lane·8
		k.e("mul.lo.s32", e, chunk, actBlk)
		k.e("mad.lo.s32", e, lane, 8, e)
		valid := k.p()
		k.e("setp.lt.u32", valid, e, K)
		at := k.r()
		k.e("mad.lo.s32", at, row, K, e)
		x := make([]reg, 8)
		for i := range x {
			x[i] = k.constant(0)
		}
		amax := k.constant(0)
		nonFinite := k.p()
		k.e("setp.ne.u32", nonFinite, lane, lane) // yanlış
		skip := k.label()
		k.jumpUnless(valid, skip)
		src := k.address(pa, at, 4)
		for i := range x {
			k.e("ld.global.f32", x[i], mem(src, 4*i))
			a, t, p := k.f(), k.f(), k.p()
			k.e("abs.f32", a, x[i])
			k.e("max.f32", amax, amax, a)
			k.e("sub.rn.f32", t, x[i], x[i])
			k.e("setp.neu.f32", p, t, float32(0))
			k.e("or.pred", nonFinite, nonFinite, p)
		}
		k.put(skip)
		flag := k.label()
		k.jumpUnless(nonFinite, flag)
		k.e("st.global.u32", mem(k.address(pfin, row, 4), 0), k.whole(1))
		k.put(flag)
		for m := blk / 16; m > 0; m /= 2 {
			o := k.shflXor(amax, m)
			k.e("max.f32", amax, amax, o)
		}
		id, d := k.actScale(amax)
		words, sum := k.actCodes(x, id)
		pair := k.shflXor(sum, 1)
		k.e("add.s32", sum, sum, pair)
		done := k.label()
		k.jumpUnless(valid, done)
		k.e("st.global.v2.u32", mem(k.address(pq, at, 1), 0), vlist(words[:]))
		p := k.p()
		odd := k.r()
		k.e("and.b32", odd, lane, 1)
		k.e("setp.ne.u32", p, odd, 0)
		k.jumpIf(p, done)
		bi := k.r() // (row·K + e) / 16
		k.e("shr.u32", bi, at, 4)
		k.e("st.global.u32", mem(k.address(pbs, bi, 4), 0), sum)
		first := k.r()
		k.e("and.b32", first, lane, blk/8-1)
		k.e("setp.ne.u32", p, first, 0)
		k.jumpIf(p, done)
		di := k.r() // (row·K + e) / blk
		k.e("shr.u32", di, at, log2(blk))
		k.e("st.global.f32", mem(k.address(pd, di, 4), 0), d)
		k.put(done)
		return k
	}
}

// actScale bloğun en büyüğünden ölçekler: id = 127 / amax, d = amax / 127; amax = 0 ya da id sonlu değilse ikisi 0.
func (k *emitter) actScale(amax reg) (id, d reg) {
	id, d = k.f(), k.f()
	k.e("div.rn.f32", id, float32(127), amax)
	k.e("div.rn.f32", d, amax, float32(127))
	zero, inf, t := k.p(), k.p(), k.f()
	k.e("setp.le.f32", zero, amax, float32(0))
	k.e("sub.rn.f32", t, id, id)
	k.e("setp.neu.f32", inf, t, float32(0))
	k.e("or.pred", zero, zero, inf)
	k.e("selp.f32", id, float32(0), id, zero)
	k.e("selp.f32", d, float32(0), d, zero)
	return id, d
}

// actCodes sekiz öğenin kodlarıdır: x·id'nin float32 sonucu çifte yuvarlanır, ±127'ye kırpılır; iki sözcük ve toplam.
func (k *emitter) actCodes(x []reg, id reg) (words [2]reg, sum reg) {
	words = [2]reg{k.whole(0), k.whole(0)}
	sum = k.whole(0)
	for i := range x {
		y, c := k.f(), k.r()
		k.e("mul.rn.f32", y, x[i], id)
		k.e("cvt.rni.s32.f32", c, y)
		k.e("min.s32", c, c, 127)
		k.e("max.s32", c, c, -127)
		k.e("add.s32", sum, sum, c)
		b := k.r()
		k.e("and.b32", b, c, 0xFF)
		k.e("shl.b32", b, b, 8*(i%4))
		k.e("or.b32", words[i/4], words[i/4], b)
	}
	return words, sum
}

// dot4 acc += Σ a'nın ve b'nin aynı sıradaki işaretli baytlarının çarpımıdır: sm_61+ dp4a, altında dört
// tamsayı çarpma-toplama (tamsayıda aynı sonuç).
func (k *emitter) dot4(acc, a, b reg) {
	if k.target >= 61 {
		k.e("dp4a.s32.s32", acc, a, b, acc)
		return
	}
	for j := range 4 {
		k.e("mad.lo.s32", acc, k.byteS(a, j), k.byteS(b, j), acc)
	}
}

// words16 2 bayta hizalı adresten 32 bitlik sözcük okur (iki 16 bitlik okuma).
func (k *emitter) words16(addr reg, offset int) reg {
	lo, hi := k.r(), k.r()
	k.e("ld.global.u16", lo, mem(addr, offset))
	k.e("ld.global.u16", hi, mem(addr, offset+2))
	k.e("shl.b32", hi, hi, 16)
	k.e("or.b32", lo, lo, hi)
	return lo
}

// byteU sözcüğün j. baytıdır (işaretsiz).
func (k *emitter) byteU(w reg, j int) reg {
	r := k.r()
	if j == 0 {
		k.e("and.b32", r, w, 0xFF)
		return r
	}
	k.e("shr.u32", r, w, 8*j)
	if j < 3 {
		k.e("and.b32", r, r, 0xFF)
	}
	return r
}

// byteS sözcüğün j. baytıdır (işaretli).
func (k *emitter) byteS(w reg, j int) reg {
	r := k.r()
	k.e("shl.b32", r, w, 24-8*j)
	k.e("shr.s32", r, r, 24)
	return r
}

package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Tek satırlık matris çarpımı (M = 1: çıkarımda bir parça), float32
// ============================================================================
//
// c[z, n] (+)= Σ_k a[z, k]·op(B[z])[k, n]. Döşemeli çarpım (kendi çekirdeğimiz ya da
// kitaplık) M = 1'de döşemenin satırlarının çoğunu boş harcar ve ağırlığı az blokla okur;
// iş ağırlığı bir kez okumaktır (bellek bandıyla sınırlı). Bu çekirdekler ağırlığı birleşik
// (coalesced) erişimle, çok blokla ve iş parçacığı başına birkaç bağımsız yüklemeyle okur.
//
// gemvKernel (B [K, N]; doğrusal katman x @ W): blok BN = L·vw sütun hesaplar (vw = 4:
// iş parçacığı başına dört bitişik sütun, tek ld.global.v4; vw = 1: bir sütun). Bir warp'ın
// 32 şeridi 32/L dilimdir, bloğun T iş parçacığı S = T/L dilim: dilim s, k = s, s+S, ...
// satırlarını dört ayrı toplayıcıyla (dörtlü açılım) biriktirir.
//
// gemvTKernel (B [N, K]; devrik: paylaşımlı gömmenin devriği x @ Wᵀ ve geri yayılımda
// dX = dY @ Wᵀ): her warp bir çıktı sütununu hesaplar; şeritler bitişik K satırını vw'lik
// parçalarla gezer.
//
// B (ağırlık) float32 ya da 16 bitlik (bf16, fp16) olabilir: 16 bitlik ağırlık açılmadan okunur
// (bayt yarıya iner), a, toplayıcılar ve c float32'dir (ağırlığı 16 bit, hesabı 32 bit; dil
// tanımı 15.12'de farklı türlerin çarpımı float32'dir).
//
// Toplama sırası sabittir: toplayıcılar sırayla, warp içi kelebek karıştırmayla (shfl.bfly;
// iki eş aynı toplamı bulur), warp'lar paylaşımlı bellekte warp sırasıyla. Sonuç aynı ayarla
// her koşuda aynıdır (belirlenimci); ayar (L, T) yalnızca boydan ve SM sayısından seçilir.

// gemvConfig devrik olmayan tek satırlık çarpımın ayarı: L bir satırdaki şerit sayısı, T
// bloktaki iş parçacığı sayısı (ikisi de ikinin kuvveti, L ≤ 32 ≤ T ≤ 1024).
type gemvConfig struct{ L, T int }

// gemvUnroll iş parçacığının aynı anda okuduğu satır sayısı (bağımsız toplayıcılar).
const gemvUnroll = 4

func gemvKernel(a gemvConfig, vw int, over bool, bt tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("gemv_%d_%d_%d_%v_%s", a.L, a.T, vw, over, bt))
		pa, pb := k.pointer("a"), k.pointer("b")
		pc := k.written("c")
		N, K := k.u32p("n"), k.u32p("k")
		W, S, BN := a.T/32, a.T/a.L, a.L*vw
		sm := k.shared("sm", 4*W*BN)
		tid := k.special("%tid.x")
		lane, warp, cg, s := k.r(), k.r(), k.r(), k.r()
		k.e("and.b32", lane, tid, 31)
		k.e("shr.u32", warp, tid, 5)
		k.e("and.b32", cg, lane, a.L-1)
		k.e("shr.u32", s, lane, log2(a.L))
		k.e("mad.lo.s32", s, warp, 32/a.L, s)
		n0, n := k.r(), k.r()
		k.e("mul.lo.s32", n0, k.special("%ctaid.x"), BN)
		k.e("mad.lo.s32", n, cg, vw, n0)
		k.batchOffsets(pa, pb, pc, N, K, dtypeSize(bt))
		// Son bloğun dışarda kalan sütunları geçerli bir sütunu okur (sonuçları yazılmaz): yükleme
		// koşulsuzdur. vw = 4'te N 4'ün katıdır.
		nr := k.r()
		k.e("sub.s32", nr, N, vw)
		k.e("min.u32", nr, n, nr)
		acc := make([][]reg, gemvUnroll)
		for u := range acc {
			acc[u] = make([]reg, vw)
			for j := range acc[u] {
				acc[u][j] = k.constant(0)
			}
		}
		row := func(kk reg, acc []reg) {
			xv := k.f()
			k.e("ld.global.f32", xv, mem(k.address(pa, kk, 4), 0))
			idx := k.r()
			k.e("mad.lo.s32", idx, kk, N, nr)
			bv := k.gemvLoad(bt, k.address(pb, idx, dtypeSize(bt)), vw)
			for j := range acc {
				k.e("fma.rn.f32", acc[j], xv, bv[j], acc[j])
			}
		}
		kk := k.r()
		k.e("mov.u32", kk, s)
		k.unrolledRows(kk, K, S, func(u int, r reg) { row(r, acc[u]) })
		// Toplayıcılar sabit sırayla, sonra warp içindeki dilimler kelebekle.
		t := make([]reg, vw)
		for j := range t {
			t[j] = k.sum(k.sum(acc[0][j], acc[1][j]), k.sum(acc[2][j], acc[3][j]))
			for m := a.L; m < 32; m *= 2 {
				t[j] = k.sum(t[j], k.shflXor(t[j], m))
			}
		}
		k.gemvWarpSum(a, vw, t, sm, tid, lane, warp, cg, n0, N, pc, over)
		return k
	}
}

// gemvWarpSum warp toplamlarını (t, her warp'ın ilk dilimindeki şeritlerde) paylaşımlı belleğe
// yazar; ilk BN iş parçacığı bir sütunun warp toplamlarını warp sırasıyla toplayıp c'ye yazar.
func (k *emitter) gemvWarpSum(a gemvConfig, vw int, t []reg, sm, tid, lane, warp, cg, n0, N, pc reg, over bool) {
	W, BN := a.T/32, a.L*vw
	// Her warp'ın ilk dilimi (lane < L) warp toplamını paylaşımlı belleğe yazar.
	first, skip := k.p(), k.label()
	k.e("setp.lt.u32", first, lane, a.L)
	k.jumpUnless(first, skip)
	sa := k.r()
	k.e("mad.lo.s32", sa, warp, BN, cg)
	k.e("mad.lo.s32", sa, sa, 4, sm)
	for j := range t {
		k.e("st.shared.f32", mem(sa, 4*(j*a.L)), t[j])
	}
	k.put(skip)
	k.barrier()
	// İlk BN iş parçacığı bir sütunun warp toplamlarını warp sırasıyla toplar. Paylaşımlı
	// düzen [warp][j·L + cg]: sütun n0 + cg·vw + j.
	end := k.label()
	p := k.p()
	k.e("setp.ge.u32", p, tid, BN)
	k.jumpIf(p, end)
	jj, cc := k.r(), k.r()
	k.e("shr.u32", jj, tid, log2(a.L))
	k.e("and.b32", cc, tid, a.L-1)
	col := k.r()
	k.e("mad.lo.s32", col, cc, vw, n0)
	k.e("add.s32", col, col, jj)
	k.e("setp.ge.u32", p, col, N)
	k.jumpIf(p, end)
	ta := k.r()
	k.e("mad.lo.s32", ta, tid, 4, sm)
	total := k.f()
	k.e("ld.shared.f32", total, mem(ta, 0))
	for w := 1; w < W; w++ {
		v := k.f()
		k.e("ld.shared.f32", v, mem(ta, 4*w*BN))
		total = k.sum(total, v)
	}
	k.gemvWrite(pc, col, total, over)
	k.put(end)
}

// gemvTWarps devrik tek satırlık çarpımın bloğundaki warp sayısı (blok başına çıktı).
const gemvTWarps = 8

func gemvTKernel(vw int, over bool, bt tensor.DType) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("gemv_t_%d_%v_%s", vw, over, bt))
		pa, pb := k.pointer("a"), k.pointer("b")
		pc := k.written("c")
		N, K := k.u32p("n"), k.u32p("k")
		tid := k.special("%tid.x")
		lane, n := k.r(), k.r()
		k.e("and.b32", lane, tid, 31)
		k.e("shr.u32", n, tid, 5)
		k.e("mad.lo.s32", n, k.special("%ctaid.x"), gemvTWarps, n)
		k.batchOffsets(pa, pb, pc, N, K, dtypeSize(bt))
		end := k.label()
		p := k.p()
		k.e("setp.ge.u32", p, n, N) // bütün warp aynı sütunda: warp birlikte çıkar
		k.jumpIf(p, end)
		base := k.r()
		k.e("mul.lo.s32", base, n, K)
		acc := make([][]reg, gemvUnroll)
		for u := range acc {
			acc[u] = make([]reg, vw)
			for j := range acc[u] {
				acc[u][j] = k.constant(0)
			}
		}
		row := func(kk reg, acc []reg) {
			idx := k.r()
			k.e("add.s32", idx, base, kk)
			xv := k.gemvLoad(tensor.Float32, k.address(pa, kk, 4), vw)
			bv := k.gemvLoad(bt, k.address(pb, idx, dtypeSize(bt)), vw)
			for j := range acc {
				k.e("fma.rn.f32", acc[j], xv[j], bv[j], acc[j])
			}
		}
		kk := k.r()
		k.e("mul.lo.s32", kk, lane, vw)
		k.unrolledRows(kk, K, 32*vw, func(u int, r reg) { row(r, acc[u]) })
		var total reg
		for j := 0; j < vw; j++ {
			t := k.sum(k.sum(acc[0][j], acc[1][j]), k.sum(acc[2][j], acc[3][j]))
			if j == 0 {
				total = t
			} else {
				total = k.sum(total, t)
			}
		}
		for m := 16; m > 0; m /= 2 {
			total = k.sum(total, k.shflXor(total, m))
		}
		k.e("setp.ne.u32", p, lane, 0)
		k.jumpIf(p, end)
		k.gemvWrite(pc, n, total, over)
		k.put(end)
		return k
	}
}

// batchOffsets toplu işin (ctaid.y) kaydırmalarını göstericilere ekler: a += z·K,
// b += z·K·N, c += z·N (64 bit; b'nin elemanı bb bayt, a ile c float32).
func (k *emitter) batchOffsets(pa, pb, pc, N, K reg, bb int) {
	z := k.special("%ctaid.y")
	kn := k.r()
	k.e("mul.lo.s32", kn, K, N)
	for _, s := range []struct {
		p, size reg
		shift   int
	}{{pa, K, 2}, {pb, kn, log2(bb)}, {pc, N, 2}} {
		off := k.rd()
		k.e("mul.wide.u32", off, z, s.size)
		k.e("shl.b64", off, off, s.shift)
		k.e("add.s64", s.p, s.p, off)
	}
}

// gemvLoad adresteki n (1 ya da 4) bitişik dt elemanını float32 olarak okur: float32 dörtlüsü
// tek ld.v4, bf16 dörtlüsü iki 32 bitlik sözcük (ceValues), fp16 dörtlüsü dört 16 bitlik okuma.
func (k *emitter) gemvLoad(dt tensor.DType, addr reg, n int) []reg {
	switch {
	case n == 1:
		return []reg{k.load(dt, addr)}
	case dt == tensor.Float32:
		return k.loadV("global", "", addr, 0, 4)
	case dt == tensor.BFloat16:
		return k.ceValues(dt, addr, 4)
	}
	out := make([]reg, n)
	for j := range out {
		a := addr
		if j > 0 {
			a = k.rd()
			k.e("add.s64", a, addr, 2*j)
		}
		out[j] = k.load(dt, a)
	}
	return out
}

// unrolledRows kk, kk+step, ... (< K) satırlarını gezer: önce gemvUnroll'lu gruplar (u.
// satır u. toplayıcıya), kalan satırlar tek tek ilk toplayıcıya. kk ilerletilir.
func (k *emitter) unrolledRows(kk, K reg, step int, body func(u int, r reg)) {
	loop, tail, done := k.label(), k.label(), k.label()
	p := k.p()
	k.put(loop)
	last := k.r()
	k.e("add.s32", last, kk, (gemvUnroll-1)*step)
	k.e("setp.ge.u32", p, last, K)
	k.jumpIf(p, tail)
	for u := 0; u < gemvUnroll; u++ {
		r := kk
		if u > 0 {
			r = k.r()
			k.e("add.s32", r, kk, u*step)
		}
		body(u, r)
	}
	k.e("add.s32", kk, kk, gemvUnroll*step)
	k.jump(loop)
	k.put(tail)
	k.e("setp.ge.u32", p, kk, K)
	k.jumpIf(p, done)
	body(0, kk)
	k.e("add.s32", kk, kk, step)
	k.jump(tail)
	k.put(done)
}

// gemvWrite c[i]'ye yazar (uzerine değilse eski değere ekler).
func (k *emitter) gemvWrite(pc, i, v reg, over bool) {
	ha := k.address(pc, i, 4)
	if !over {
		old := k.f()
		k.e("ld.global.f32", old, mem(ha, 0))
		v = k.sum(old, v)
	}
	k.e("st.global.f32", mem(ha, 0), v)
}

package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Toplu matris çarpımı (paylaşımlı bellek döşemeli, float32)
// ============================================================================
//
// C[z] (+)= op(A[z])·op(B[z]); her blok C'nin BM×BN'lik bir döşemesini hesaplar. K
// ekseni BK'lık dilimlerle gezilir: A ve B döşemeleri paylaşımlı belleğe yüklenir (sınır
// dışı elemanlar 0), engelden sonra her iş parçacığı TM×TN'lik kayıt bloğunu fma ile
// biriktirir. İş parçacığının satırları w = min(TM, 4)'lü bitişik gruplardır:
// g·(BM·w/TM) + ty·w + i (g < TM/w, i < w), sütunları aynı biçimde tx'ten; böylece
// paylaşımlı bellekten her grup tek okumayla (w = 4: ld.shared.v4) gelir (A satırları çeyrek warp'ta yayınla, B
// sütunları çakışmasız). Paylaşımlı dizilerin satırları 4 elemanla dolgulanır (yükleme
// sırasında bank çakışmalarını azaltır, 16 bayt hizayı korur). Döşeme boyları arka ucun
// ayarıdır (carpimAyari), kullanıcıya görünmez. Makinede üreticinin kitaplığı varsa çarpım
// onunla yapılır (carpim_kitaplik.go); bu çekirdek yedektir.

// carpimAyari matris çarpımı döşeme boyları (hepsi ikinin kuvveti).
type gemmConfig struct {
	BM, BN, BK, TM, TN int
}

var (
	// varsayilanCarpim küçük ve orta matrisler için.
	defaultGemm = gemmConfig{64, 64, 16, 4, 4}
	// buyukCarpim büyük matrisler için (iş parçacığı başına 8×8).
	largeGemm = gemmConfig{128, 128, 8, 8, 8}
)

func (a gemmConfig) is() int { return (a.BM / a.TM) * (a.BN / a.TN) }

// gecerli ayarın yükleme bölüşümünün tam olduğunu denetler.
func (a gemmConfig) valid() bool {
	t := a.is()
	twoK := func(n int) bool { return n > 0 && n&(n-1) == 0 }
	return twoK(a.BM) && twoK(a.BN) && twoK(a.BK) && twoK(a.TM) && twoK(a.TN) && t <= 1024 &&
		(a.BM*a.BK)%t == 0 && (a.BN*a.BK)%t == 0 && 4*(a.BK*(a.BM+4)+a.BK*(a.BN+4)) <= 48<<10
}

func log2(n int) int {
	r := 0
	for n > 1 {
		n >>= 1
		r++
	}
	return r
}

// donemeYukle bir döşemenin q. yükleme payının satır/sütun indekslerini verir: e = tid +
// T·q; iç eksen (genel bellekte bitişik olan) e'nin alt bitleridir.
func (k *emitter) epochIndex(tid reg, q, T, ic int) (outerI, innerI reg) {
	e := k.r()
	k.e("add.s32", e, tid, q*T)
	outerI, innerI = k.r(), k.r()
	k.e("and.b32", innerI, e, ic-1)
	k.e("shr.u32", outerI, e, log2(ic))
	return outerI, innerI
}

// carpimTasarimi döşeme yüklemesinin bir payı: genel bellekteki satır/sütun (döşeme
// içi) ve paylaşımlı bellek ofseti.
type gemmShare struct {
	outer, ic reg
	smOfs     reg
}

func gemmKernel(a gemmConfig, transA, transB, over bool) func() *emitter {
	return func() *emitter {
		k := newEmitter(fmt.Sprintf("carpim_%d_%d_%d_%d_%d_%v_%v_%v", a.BM, a.BN, a.BK, a.TM, a.TN, transA, transB, over))
		pa, pb := k.pointer("a"), k.pointer("b")
		pc := k.written("c")
		M, N, K := k.u32p("m"), k.u32p("n"), k.u32p("k")
		T := a.is()
		sa := k.shared("sa", 4*a.BK*(a.BM+4))
		sb := k.shared("sb", 4*a.BK*(a.BN+4))
		tid := k.special("%tid.x")
		tx, ty := k.r(), k.r()
		k.e("and.b32", tx, tid, a.BN/a.TN-1)
		k.e("shr.u32", ty, tid, log2(a.BN/a.TN))
		m0, n0, z := k.r(), k.r(), k.special("%ctaid.z")
		k.e("mul.lo.s32", m0, k.special("%ctaid.y"), a.BM)
		k.e("mul.lo.s32", n0, k.special("%ctaid.x"), a.BN)
		// Toplu iş kaydırmaları (64 bit).
		for _, s := range []struct {
			p    reg
			x, y reg
		}{{pa, M, K}, {pb, K, N}, {pc, M, N}} {
			mk, off := k.r(), k.rd()
			k.e("mul.lo.s32", mk, s.x, s.y)
			k.e("mul.wide.u32", off, z, mk)
			k.e("shl.b64", off, off, 2)
			k.e("add.s64", s.p, s.p, off)
		}
		// Yükleme payları (döngü dışında hesaplanır).
		aShare := k.gemmShares(tid, a.BM, a.BK, a.BM+4, T, transA)
		bShare := k.gemmShares(tid, a.BN, a.BK, a.BN+4, T, !transB)
		acc := make([]reg, a.TM*a.TN)
		for i := range acc {
			acc[i] = k.constant(0)
		}
		k0 := k.whole(0)
		loop, done := k.label(), k.label()
		k.put(loop)
		p := k.p()
		k.e("setp.ge.u32", p, k0, K)
		k.jumpIf(p, done)
		k.loadGemm(aShare, pa, sa, m0, k0, M, K, transA)
		k.loadGemm(bShare, pb, sb, n0, k0, N, K, !transB)
		k.barrier()
		k.computeGemm(a, acc, sa, sb, tx, ty)
		k.barrier()
		k.e("add.s32", k0, k0, a.BK)
		k.jump(loop)
		k.put(done)
		k.writeGemm(a, acc, pc, m0, n0, tx, ty, M, N, over)
		return k
	}
}

// carpimPaylari bir döşemenin (dış boy D, K boyu BK) yükleme paylarını kurar. kBitisik
// doğruysa genel bellekte K ekseni bitişiktir (A: transA değil; B: transB), değilse dış
// eksen bitişiktir. Paylaşımlı bellekte düzen [k][dış] (satır adımı sat).
func (k *emitter) gemmShares(tid reg, D, BK, row2, T int, outerContiguous bool) []gemmShare {
	var shares []gemmShare
	for q := 0; q < D*BK/T; q++ {
		var outer, kk reg
		if outerContiguous {
			kk, outer = k.epochIndex(tid, q, T, D)
		} else {
			outer, kk = k.epochIndex(tid, q, T, BK)
		}
		so := k.r()
		k.e("mad.lo.s32", so, kk, row2, outer)
		k.e("shl.b32", so, so, 2)
		shares = append(shares, gemmShare{outer: outer, ic: kk, smOfs: so})
	}
	return shares
}

// carpimYukle payları genel bellekten (sınır dışı 0) paylaşımlı belleğe taşır. Genel
// bellekteki eleman: disBitisik ise (k0+kk)·Dboy + (d0+dis), değilse (d0+dis)·K + k0+kk.
func (k *emitter) loadGemm(shares []gemmShare, ptr, sm, d0, k0, DSize, K reg, outerContiguous bool) {
	for _, py := range shares {
		gd, gk := k.r(), k.r()
		k.e("add.s32", gd, d0, py.outer)
		k.e("add.s32", gk, k0, py.ic)
		p1, p2 := k.p(), k.p()
		k.e("setp.lt.u32", p1, gd, DSize)
		k.e("setp.lt.u32", p2, gk, K)
		k.e("and.pred", p1, p1, p2)
		idx := k.r()
		if outerContiguous {
			k.e("mad.lo.s32", idx, gk, DSize, gd)
		} else {
			k.e("mad.lo.s32", idx, gd, K, gk)
		}
		v := k.constant(0)
		k.ifCase(p1, "ld.global.f32", v, mem(k.address(ptr, idx, 4), 0))
		ad := k.r()
		k.e("add.s32", ad, sm, py.smOfs)
		k.e("st.shared.f32", mem(ad, 0), v)
	}
}

// grupGenisligi iş parçacığının bitişik satır (sütun) grubunun boyu.
func groupWidth(T int) int { return min(T, 4) }

// grupOfseti iş parçacığının i. satırının (ya da sütununun) döşeme içindeki ofsetinin
// iş parçacığından bağımsız kısmı: g·(D·w/T) + i%w (g = i/w).
func groupOffset(i, D, T int) int {
	w := groupWidth(T)
	return i/w*(D*w/T) + i%w
}

// grupYukle paylaşımlı bellekten bir grubu (w = 4, 2, 1 eleman) yükler.
func (k *emitter) loadGroup(addr reg, offset int, r []reg) {
	if len(r) == 1 {
		k.e("ld.shared.f32", r[0], mem(addr, offset))
		return
	}
	k.loadVInto("shared", "", addr, offset, r)
}

// carpimHesapla döşemenin BK adımını açılmış olarak biriktirir.
func (k *emitter) computeGemm(a gemmConfig, acc []reg, sa, sb, tx, ty reg) {
	wm, wn := groupWidth(a.TM), groupWidth(a.TN)
	at, bt := k.r(), k.r()
	k.e("mad.lo.s32", at, ty, 4*wm, sa)
	k.e("mad.lo.s32", bt, tx, 4*wn, sb)
	av, bv := make([]reg, a.TM), make([]reg, a.TN)
	for i := range av {
		av[i] = k.f()
	}
	for j := range bv {
		bv[j] = k.f()
	}
	for kk := 0; kk < a.BK; kk++ {
		for i := 0; i < a.TM; i += wm {
			k.loadGroup(at, 4*(kk*(a.BM+4)+groupOffset(i, a.BM, a.TM)), av[i:i+wm])
		}
		for j := 0; j < a.TN; j += wn {
			k.loadGroup(bt, 4*(kk*(a.BN+4)+groupOffset(j, a.BN, a.TN)), bv[j:j+wn])
		}
		for i := range av {
			for j := range bv {
				c := acc[i*a.TN+j]
				k.e("fma.rn.f32", c, av[i], bv[j], c)
			}
		}
	}
}

// carpimYaz biriktirilen bloğu C'ye yazar (uzerine değilse ekler).
func (k *emitter) writeGemm(a gemmConfig, acc []reg, pc, m0, n0, tx, ty, M, N reg, over bool) {
	for i := 0; i < a.TM; i++ {
		row := k.r()
		k.e("mad.lo.s32", row, ty, groupWidth(a.TM), m0)
		k.e("add.s32", row, row, groupOffset(i, a.BM, a.TM))
		for j := 0; j < a.TN; j++ {
			col := k.r()
			k.e("mad.lo.s32", col, tx, groupWidth(a.TN), n0)
			k.e("add.s32", col, col, groupOffset(j, a.BN, a.TN))
			skip := k.label()
			p1, p2 := k.p(), k.p()
			k.e("setp.ge.u32", p1, row, M)
			k.e("setp.ge.u32", p2, col, N)
			k.e("or.pred", p1, p1, p2)
			k.jumpIf(p1, skip)
			idx := k.r()
			k.e("mad.lo.s32", idx, row, N, col)
			ha := k.address(pc, idx, 4)
			v := acc[i*a.TN+j]
			if !over {
				old := k.load(tensor.Float32, ha)
				k.e("add.rn.f32", v, old, v)
			}
			k.store(tensor.Float32, ha, v)
			k.put(skip)
		}
	}
}

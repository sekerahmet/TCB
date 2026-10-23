package tensor

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// CPU arka ucu: döşemeli, çevrimiçi softmax'lı dikkat
// ============================================================================
//
// İleri geçiş: iş birimi bir (toplu eleman, bakış, sorgu satır bloğu) üçlüsüdür;
// birimler işçi havuzuna dağıtılır. Her birim, bakabildiği anahtar aralığını
// (nedensellik ve pencereye göre; aralık dışındaki bloklar hiç hesaplanmaz)
// anahtar blokları hâlinde gezer:
//
//	S = Q_i·K_jᵀ·ölçek (+ maske)          blok × blok, önbellekte
//	m' = max(m, satır en büyüğü); P = e^{S-m'}; l = l·e^{m-m'} + ΣP
//	O = O·e^{m-m'} + P·V_j
//
// Sonda O/l yazılır ve satır başına logsumexp (m + log l) saklanır. İç
// çarpımlar matris çarpımının paketleme ve mikro çekirdekleriyle (AVX2/FMA
// derlemesinde 6×16 çekirdek) tek iş parçacığında yapılır; paralellik birimler
// arasındadır.
//
// Geri geçiş puanları yeniden hesaplar (P = e^{S - lse}); ek bellek yalnızca
// satır başına D = Σ dO·O'dur. k/v bakışını paylaşan bütün sorgu bakışları aynı
// birimde gezildiği için dK ve dV yarışsız biriktirilir. Toplu × k/v bakışı
// sayısı işçi sayısına yetiyorsa birim bir k/v bakışıdır ve dQ da aynı geçişte
// biriktirilir (tek geçiş); yetmiyorsa iki geçiş yapılır: (k/v bakışı, anahtar
// bloğu) birimleriyle dK/dV, ardından (bakış, sorgu bloğu) birimleriyle dQ. İki
// kip aynı blok ızgarasını ve aynı toplama sırasını kullanır: sonuç işçi
// sayısından bağımsız olarak bit düzeyinde aynıdır.

// Varsayılan blok boyutları (BenchmarkDikkatBlok ile ölçülerek seçildi; i7-13700H,
// dh 64, H 8, 2 ve 20 iş parçacığı, GOAMD64=v3). Anahtar bloğu 64: 16 ve 32
// paketleme ve satır işlemleri yüzünden, 128 ise küçük T'de yavaştı. Sorgu
// bloğu 64; uzun dizilerde (Tq ≥ 1024) 128 (T=2048'de %10-20 daha hızlı: K/V
// panelleri daha çok satırla paylaşılır). 128×64'lük S bloğu (32 KB), Q ve O
// blokları ve paketlenmiş K/V ile birlikte (~130 KB) L2'de kalır. Seçim
// yalnızca şekle bağlıdır (işçi sayısına değil); böylece sonuç her işçi
// sayısında aynıdır.
const (
	defaultAttnBlockQ     = 64
	defaultAttnBlockQLong = 128
	longSeqThreshold      = 1024
	defaultAttnBlockK     = 64
)

var cpuAttnBlockQ, cpuAttnBlockK atomic.Int32 // 0: varsayılan

// AttentionBlocks (AttentionTiler) ayarlanmış sorgu ve anahtar blok boyutları;
// 0 varsayılanın (şekle göre seçilen) kullanıldığını gösterir.
func (cpuBackend) AttentionBlocks() (q, k int) {
	return int(cpuAttnBlockQ.Load()), int(cpuAttnBlockK.Load())
}

// attnBlocks p için kullanılacak blok boyutları.
func attnBlocks(p *AttentionParams) (bq, bk int) {
	bq, bk = int(cpuAttnBlockQ.Load()), int(cpuAttnBlockK.Load())
	if bq <= 0 {
		bq = defaultAttnBlockQ
		if p.Tq >= longSeqThreshold {
			bq = defaultAttnBlockQLong
		}
	}
	if bk <= 0 {
		bk = defaultAttnBlockK
	}
	return bq, bk
}

// SetAttentionBlocks (AttentionTiler) blok boyutlarını ayarlar; 0 varsayılana döner. Süreç genelidir.
func (cpuBackend) SetAttentionBlocks(q, k int) {
	if q < 0 || k < 0 {
		panic(messages.New(messages.H3011, q, k))
	}
	cpuAttnBlockQ.Store(int32(q))
	cpuAttnBlockK.Store(int32(k))
}

// attnBuf bir katılımcının geçici tamponları (çağrılar arasında havuzda).
type attnBuf struct {
	bp, ap     []float32 // matris çarpımı paketleme
	s, dp      []float32 // puan / olasılık ve dP blokları
	o, ka, va  []float32 // O biriktirici, dK ve dV biriktiricileri
	qz         []float32 // NaN puanlı satırları sıfırlanmış sorgu bloğu kopyası (dK için)
	enb, denom []float32 // satır başına koşan en büyük ve payda
}

var attnBufPool sync.Pool

func takeAttnBuf() *attnBuf {
	if t, ok := attnBufPool.Get().(*attnBuf); ok {
		return t
	}
	return &attnBuf{}
}

// sized b'yi n uzunluğa getirir (gerekirse yeniden ayırır; içerik belirsiz).
func sized(b []float32, n int) []float32 {
	if cap(b) < n {
		return make([]float32, n)
	}
	return b[:n]
}

// gemmSingle tek iş parçacıklı C += op(A)·op(B): op(A)(i, k) = a[i*ais + k*aks],
// op(B)(k, j) = b[k*bks + j*bjs], C(i, j) = c[i*ldc + j]. Matris çarpımının
// paketleme ve mikro çekirdeklerini kullanır (bkz. cpu_carpim.go).
func gemmSingle(M, N, K int, a []float32, ais, axisN int, b []float32, bks, bjs int, c []float32, ldc int, tb *attnBuf) {
	if M == 0 || N == 0 || K == 0 {
		return
	}
	np := (N + nr - 1) / nr
	tb.bp = sized(tb.bp, np*K*nr)
	bp := tb.bp
	for jp := 0; jp < np; jp++ {
		packBPanel(b, bks, bjs, K, N, jp, bp[jp*K*nr:(jp+1)*K*nr])
	}
	direct := directA && np < 12
	kcMax := min(K, kcBlock)
	if direct {
		tb.ap = sized(tb.ap, mr*kcMax)
	} else {
		tb.ap = sized(tb.ap, (M+mr-1)/mr*mr*kcMax)
	}
	ap := tb.ap
	for p0 := 0; p0 < K; p0 += kcBlock {
		kb := min(kcBlock, K-p0)
		whole := 0
		if direct {
			whole = M / mr
			if M > whole*mr {
				packA(a, ais, axisN, whole*mr, M-whole*mr, p0, kb, ap)
			}
		} else {
			packA(a, ais, axisN, 0, M, p0, kb, ap)
		}
		for jp := 0; jp < np; jp++ {
			j0 := jp * nr
			nn := min(nr, N-j0)
			bpan := bp[jp*K*nr+p0*nr : jp*K*nr+(p0+kb)*nr]
			for ip := 0; ip*mr < M; ip++ {
				mm := min(mr, M-ip*mr)
				cBlock := c[(ip*mr)*ldc+j0:]
				switch {
				case ip < whole:
					microA(kb, a[(ip*mr)*ais+p0*axisN:], ais, axisN, bpan, cBlock, ldc, mm, nn)
				case direct:
					microKernel(kb, ap[:kb*mr], bpan, cBlock, ldc, mm, nn)
				default:
					microKernel(kb, ap[ip*kb*mr:(ip+1)*kb*mr], bpan, cBlock, ldc, mm, nn)
				}
			}
		}
	}
}

var negInf = float32(math.Inf(-1))

// attnWorkerCount toplam iş (yaklaşık çarpma-toplama) için katılımcı sayısı.
func attnWorkerCount(is int) int {
	if is < singleCoreThreshold {
		return 1
	}
	return workerCount(is, minPerWorker, gemmBigJob)
}

// attnJob ileri ya da geri geçişin yaklaşık çarpma-toplama sayısı.
func attnJob(p *AttentionParams) int {
	is := p.Batch * p.H * p.Tq * p.Tk * (p.Dh + p.Dv)
	if p.Causal && p.Tq > 1 {
		is /= 2
	}
	if p.Window > 0 {
		is = min(is, p.Batch*p.H*p.Tq*p.Window*(p.Dh+p.Dv))
	}
	return max(is, 1)
}

// masked sorgu satırı i'nin anahtar jg'ye bakamadığını söyler (ek maske).
func masked(md []float32, floorV, i, jg int, p *AttentionParams) bool {
	return md[floorV+i*p.MaskStrideQ+jg*p.MaskStrideK] == 0
}

// blockRange sorgu satırı i'nin [j0, j0+n) anahtar bloğunda nedensellik ve
// pencereye göre bakabildiği sütunları [a, c) olarak verir (blok içi indeks).
func (p *AttentionParams) blockRange(i, j0, n int) (a, c int) {
	lo, hi := p.Limits(i)
	a = min(max(lo-j0, 0), n)
	c = min(max(hi-j0, 0), n)
	return a, max(a, c)
}

// expMasked seg = e^seg; ek maskenin kapattığı yerler (seg'de -sonsuz) 0
// olur. Kapalı yerler üstele -sonsuz olarak verilmez: AVX2 çekirdeğinde normal
// altı ara sonuçlar mikro kod yardımıyla çok yavaştır (karakter modelinde
// üstel, adımın en pahalı işiydi).
func (p *AttentionParams) expMasked(seg, md []float32, floorV, i, jStart int) {
	if md == nil {
		expVec(seg, seg)
		return
	}
	for jj, x := range seg {
		if x == negInf {
			seg[jj] = 0
		}
	}
	expVec(seg, seg)
	for jj := range seg {
		if masked(md, floorV, i, jStart+jj, p) {
			seg[jj] = 0
		}
	}
}

func (cpuBackend) f32Attention(out Storage, q, k, v View, mask Storage, p AttentionParams) Storage {
	qd, kd, vd := cd(q.S)[q.Offset:], cd(k.S)[k.Offset:], cd(v.S)[v.Offset:]
	od := cd(out)
	md := cd(mask)
	lse := cpuBackend{}.allocUninit(p.Batch*p.H*p.Tq, Float32)
	ld := cd(lse)
	bq, bk := attnBlocks(&p)
	nqb := (p.Tq + bq - 1) / bq
	unit := p.Batch * p.H * nqb
	if unit == 0 {
		return lse
	}
	group := p.H / p.Hkv
	W := attnWorkerCount(attnJob(&p))
	buffers := make([]*attnBuf, max(W, 1))
	scale := p.Scale
	distribute(W, unit, func(factor, u int) {
		tb := buffers[factor]
		if tb == nil {
			tb = takeAttnBuf()
			buffers[factor] = tb
		}
		// Pahalı birimler önce: nedensel dikkatte son sorgu blokları en çok
		// anahtara bakar. Sona küçük birimler kalır, yavaş çekirdekler kuyruğu
		// uzatmaz.
		bh, ib := u%(p.Batch*p.H), nqb-1-u/(p.Batch*p.H)
		b, h := bh/p.H, bh%p.H
		kvh := b*p.Hkv + h/group
		i0 := ib * bq
		m := min(bq, p.Tq-i0)
		qb := qd[(bh*p.Tq+i0)*p.Dh : (bh*p.Tq+i0+m)*p.Dh]
		kb := kd[kvh*p.Tk*p.Dh : (kvh+1)*p.Tk*p.Dh]
		vb := vd[kvh*p.Tk*p.Dv : (kvh+1)*p.Tk*p.Dv]
		ob := od[(bh*p.Tq+i0)*p.Dv : (bh*p.Tq+i0+m)*p.Dv]
		lsb := ld[bh*p.Tq+i0 : bh*p.Tq+i0+m]
		floorV := 0
		if md != nil {
			floorV = p.MaskBase + p.MaskOffsets[bh]
		}
		jlo, wholeHi := p.Limits(i0)
		wholeLo, jhi := p.Limits(i0 + m - 1)

		tb.o = sized(tb.o, m*p.Dv)
		O := tb.o
		clear(O)
		tb.enb = sized(tb.enb, m)
		tb.denom = sized(tb.denom, m)
		enb, denom := tb.enb, tb.denom
		for r := range enb {
			enb[r] = negInf
			denom[r] = 0
		}
		for j0 := jlo; j0 < jhi; j0 += bk {
			n := min(bk, jhi-j0)
			tb.s = sized(tb.s, m*n)
			S := tb.s
			clear(S)
			gemmSingle(m, n, p.Dh, qb, p.Dh, 1, kb[j0*p.Dh:], 1, p.Dh, S, n, tb)
			partial := j0 < wholeLo || j0+n > wholeHi || md != nil
			empty := true
			for r := 0; r < m; r++ {
				row := S[r*n : (r+1)*n]
				// Üstel yalnızca bakılabilen [a, c) sütunlarında hesaplanır; dışı 0.
				a, c := 0, n
				if partial {
					a, c = p.blockRange(i0+r, j0, n)
				}
				seg := row[a:c]
				// en NaN'ı korur: NaN puanlı satır NaN verir, maskeli satırla karışmaz.
				en := negInf
				if md != nil {
					for jj, x := range seg {
						if masked(md, floorV, i0+r, j0+a+jj, &p) {
							seg[jj] = negInf
							continue
						}
						x *= scale
						seg[jj] = x
						if x > en || x != x {
							en = x
						}
					}
				} else {
					for jj, x := range seg {
						x *= scale
						seg[jj] = x
						if x > en || x != x {
							en = x
						}
					}
				}
				if en == negInf {
					clear(row) // bu blokta bakılacak anahtar yok
					continue
				}
				empty = false
				old := enb[r]
				freshT := max(old, en)
				for jj, x := range seg {
					seg[jj] = x - freshT
				}
				p.expMasked(seg, md, floorV, i0+r, j0+a)
				clear(row[:a])
				clear(row[c:])
				var totalSum float32
				for _, x := range seg {
					totalSum += x
				}
				if old != freshT {
					// İlk katkıda (eski -sonsuz) O satırı ve payda zaten 0'dır.
					if old != negInf {
						alpha := expf32(old - freshT)
						denom[r] *= alpha
						or := O[r*p.Dv : (r+1)*p.Dv]
						for d := range or {
							or[d] *= alpha
						}
					}
					enb[r] = freshT
				}
				denom[r] += totalSum
			}
			if !empty {
				gemmSingle(m, p.Dv, n, S, n, 1, vb[j0*p.Dv:], p.Dv, 1, O, p.Dv, tb)
			}
		}
		for r := 0; r < m; r++ {
			or := ob[r*p.Dv : (r+1)*p.Dv]
			if denom[r] == 0 {
				clear(or)
				lsb[r] = negInf
				continue
			}
			inverse := 1 / denom[r]
			for d, x := range O[r*p.Dv : (r+1)*p.Dv] {
				or[d] = x * inverse
			}
			lsb[r] = enb[r] + log32(denom[r])
		}
	})
	for _, tb := range buffers {
		if tb != nil {
			attnBufPool.Put(tb)
		}
	}
	return lse
}

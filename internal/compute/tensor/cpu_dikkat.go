package tensor

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// ============================================================================
// CPU arka ucu: döşemeli, çevrimiçi softmax'lı dikkat
// ============================================================================
//
// İleri geçiş: iş birimi bir (toplu eleman, baş, sorgu satır bloğu) üçlüsüdür;
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
// satır başına D = Σ dO·O'dur. k/v başını paylaşan bütün sorgu başları aynı
// birimde gezildiği için dK ve dV yarışsız biriktirilir. Toplu × k/v başı
// sayısı işçi sayısına yetiyorsa birim bir k/v başıdır ve dQ da aynı geçişte
// biriktirilir (tek geçiş); yetmiyorsa iki geçiş yapılır: (k/v başı, anahtar
// bloğu) birimleriyle dK/dV, ardından (baş, sorgu bloğu) birimleriyle dQ. İki
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
	varsayilanDikkatBlokQ     = 64
	varsayilanDikkatBlokQUzun = 128
	uzunDiziEsigi             = 1024
	varsayilanDikkatBlokK     = 64
)

var cpuDikkatBlokQ, cpuDikkatBlokK atomic.Int32 // 0: varsayılan

// AttentionBlocks (AttentionTiler) ayarlanmış sorgu ve anahtar blok boyutları;
// 0 varsayılanın (şekle göre seçilen) kullanıldığını gösterir.
func (cpuBackend) AttentionBlocks() (q, k int) {
	return int(cpuDikkatBlokQ.Load()), int(cpuDikkatBlokK.Load())
}

// dikkatBloklari p için kullanılacak blok boyutları.
func dikkatBloklari(p *AttentionParams) (bq, bk int) {
	bq, bk = int(cpuDikkatBlokQ.Load()), int(cpuDikkatBlokK.Load())
	if bq <= 0 {
		bq = varsayilanDikkatBlokQ
		if p.Tq >= uzunDiziEsigi {
			bq = varsayilanDikkatBlokQUzun
		}
	}
	if bk <= 0 {
		bk = varsayilanDikkatBlokK
	}
	return bq, bk
}

// SetAttentionBlocks (AttentionTiler) blok boyutlarını ayarlar; 0 varsayılana
// döner. Süreç genelidir.
func (cpuBackend) SetAttentionBlocks(q, k int) {
	if q < 0 || k < 0 {
		panic(fmt.Sprintf("tensor: dikkat blok boyutları negatif olamaz (%d, %d)", q, k))
	}
	cpuDikkatBlokQ.Store(int32(q))
	cpuDikkatBlokK.Store(int32(k))
}

// dikkatTampon bir katılımcının geçici tamponları (çağrılar arasında havuzda).
type dikkatTampon struct {
	bp, ap     []float32 // matris çarpımı paketleme
	s, dp      []float32 // puan / olasılık ve dP blokları
	o, ka, va  []float32 // O biriktirici, dK ve dV biriktiricileri
	enb, payda []float32 // satır başına koşan en büyük ve payda
}

var dikkatTamponHavuzu sync.Pool

func dikkatTamponAl() *dikkatTampon {
	if t, ok := dikkatTamponHavuzu.Get().(*dikkatTampon); ok {
		return t
	}
	return &dikkatTampon{}
}

// boyutla b'yi n uzunluğa getirir (gerekirse yeniden ayırır; içerik belirsiz).
func boyutla(b []float32, n int) []float32 {
	if cap(b) < n {
		return make([]float32, n)
	}
	return b[:n]
}

// gemmTek tek iş parçacıklı C += op(A)·op(B): op(A)(i, k) = a[i*ais + k*aks],
// op(B)(k, j) = b[k*bks + j*bjs], C(i, j) = c[i*ldc + j]. Matris çarpımının
// paketleme ve mikro çekirdeklerini kullanır (bkz. cpu_carpim.go).
func gemmTek(M, N, K int, a []float32, ais, aks int, b []float32, bks, bjs int, c []float32, ldc int, tb *dikkatTampon) {
	if M == 0 || N == 0 || K == 0 {
		return
	}
	np := (N + nr - 1) / nr
	tb.bp = boyutla(tb.bp, np*K*nr)
	bp := tb.bp
	for jp := 0; jp < np; jp++ {
		packBPanel(b, bks, bjs, K, N, jp, bp[jp*K*nr:(jp+1)*K*nr])
	}
	dogrudan := dogrudanA && np < 12
	kcMax := min(K, kcBlok)
	if dogrudan {
		tb.ap = boyutla(tb.ap, mr*kcMax)
	} else {
		tb.ap = boyutla(tb.ap, (M+mr-1)/mr*mr*kcMax)
	}
	ap := tb.ap
	for p0 := 0; p0 < K; p0 += kcBlok {
		kb := min(kcBlok, K-p0)
		tam := 0
		if dogrudan {
			tam = M / mr
			if M > tam*mr {
				packA(a, ais, aks, tam*mr, M-tam*mr, p0, kb, ap)
			}
		} else {
			packA(a, ais, aks, 0, M, p0, kb, ap)
		}
		for jp := 0; jp < np; jp++ {
			j0 := jp * nr
			nn := min(nr, N-j0)
			bpan := bp[jp*K*nr+p0*nr : jp*K*nr+(p0+kb)*nr]
			for ip := 0; ip*mr < M; ip++ {
				mm := min(mr, M-ip*mr)
				cBlok := c[(ip*mr)*ldc+j0:]
				switch {
				case ip < tam:
					mikroA(kb, a[(ip*mr)*ais+p0*aks:], ais, aks, bpan, cBlok, ldc, mm, nn)
				case dogrudan:
					microKernel(kb, ap[:kb*mr], bpan, cBlok, ldc, mm, nn)
				default:
					microKernel(kb, ap[ip*kb*mr:(ip+1)*kb*mr], bpan, cBlok, ldc, mm, nn)
				}
			}
		}
	}
}

var eksiSonsuz = float32(math.Inf(-1))

// dikkatIsciSayisi toplam iş (yaklaşık çarpma-toplama) için katılımcı sayısı.
func dikkatIsciSayisi(is int) int {
	if is < tekCekirdekEsik {
		return 1
	}
	return isciSayisi(is, isciBasinaEnAz, gemmBuyukIs)
}

// dikkatIs ileri ya da geri geçişin yaklaşık çarpma-toplama sayısı.
func dikkatIs(p *AttentionParams) int {
	is := p.Batch * p.H * p.Tq * p.Tk * (p.Dh + p.Dv)
	if p.Causal && p.Tq > 1 {
		is /= 2
	}
	if p.Window > 0 {
		is = min(is, p.Batch*p.H*p.Tq*p.Window*(p.Dh+p.Dv))
	}
	return max(is, 1)
}

// maskeli sorgu satırı i'nin anahtar jg'ye bakamadığını söyler (ek maske).
func maskeli(md []float32, taban, i, jg int, p *AttentionParams) bool {
	return md[taban+i*p.MaskStrideQ+jg*p.MaskStrideK] == 0
}

// blokAraligi sorgu satırı i'nin [j0, j0+n) anahtar bloğunda nedensellik ve
// pencereye göre bakabildiği sütunları [a, c) olarak verir (blok içi indeks).
func (p *AttentionParams) blokAraligi(i, j0, n int) (a, c int) {
	lo, hi := p.Limits(i)
	a = min(max(lo-j0, 0), n)
	c = min(max(hi-j0, 0), n)
	return a, max(a, c)
}

// ustelMaskeli seg = e^seg; ek maskenin kapattığı yerler (seg'de -sonsuz) 0
// olur. Kapalı yerler üstele -sonsuz olarak verilmez: AVX2 çekirdeğinde normal
// altı ara sonuçlar mikro kod yardımıyla çok yavaştır (karakter modelinde
// üstel, adımın en pahalı işiydi).
func (p *AttentionParams) ustelMaskeli(seg, md []float32, taban, i, jBas int) {
	if md == nil {
		expVek(seg, seg)
		return
	}
	for jj, x := range seg {
		if x == eksiSonsuz {
			seg[jj] = 0
		}
	}
	expVek(seg, seg)
	for jj := range seg {
		if maskeli(md, taban, i, jBas+jj, p) {
			seg[jj] = 0
		}
	}
}

func (cpuBackend) f32Attention(out Storage, q, k, v View, mask Storage, p AttentionParams) Storage {
	qd, kd, vd := cd(q.S)[q.Offset:], cd(k.S)[k.Offset:], cd(v.S)[v.Offset:]
	od := cd(out)
	md := cd(mask)
	lse := cpuBackend{}.allocBos(p.Batch*p.H*p.Tq, Float32)
	ld := cd(lse)
	bq, bk := dikkatBloklari(&p)
	nqb := (p.Tq + bq - 1) / bq
	birim := p.Batch * p.H * nqb
	if birim == 0 {
		return lse
	}
	grup := p.H / p.Hkv
	W := dikkatIsciSayisi(dikkatIs(&p))
	tamponlar := make([]*dikkatTampon, max(W, 1))
	scale := p.Scale
	dagit(W, birim, func(kat, u int) {
		tb := tamponlar[kat]
		if tb == nil {
			tb = dikkatTamponAl()
			tamponlar[kat] = tb
		}
		// Pahalı birimler önce: nedensel dikkatte son sorgu blokları en çok
		// anahtara bakar. Sona küçük birimler kalır, yavaş çekirdekler kuyruğu
		// uzatmaz.
		bh, ib := u%(p.Batch*p.H), nqb-1-u/(p.Batch*p.H)
		b, h := bh/p.H, bh%p.H
		kvh := b*p.Hkv + h/grup
		i0 := ib * bq
		m := min(bq, p.Tq-i0)
		qb := qd[(bh*p.Tq+i0)*p.Dh : (bh*p.Tq+i0+m)*p.Dh]
		kb := kd[kvh*p.Tk*p.Dh : (kvh+1)*p.Tk*p.Dh]
		vb := vd[kvh*p.Tk*p.Dv : (kvh+1)*p.Tk*p.Dv]
		ob := od[(bh*p.Tq+i0)*p.Dv : (bh*p.Tq+i0+m)*p.Dv]
		lsb := ld[bh*p.Tq+i0 : bh*p.Tq+i0+m]
		taban := 0
		if md != nil {
			taban = p.MaskOffsets[bh]
		}
		jlo, tamHi := p.Limits(i0)
		tamLo, jhi := p.Limits(i0 + m - 1)

		tb.o = boyutla(tb.o, m*p.Dv)
		O := tb.o
		clear(O)
		tb.enb = boyutla(tb.enb, m)
		tb.payda = boyutla(tb.payda, m)
		enb, payda := tb.enb, tb.payda
		for r := range enb {
			enb[r] = eksiSonsuz
			payda[r] = 0
		}
		for j0 := jlo; j0 < jhi; j0 += bk {
			n := min(bk, jhi-j0)
			tb.s = boyutla(tb.s, m*n)
			S := tb.s
			clear(S)
			gemmTek(m, n, p.Dh, qb, p.Dh, 1, kb[j0*p.Dh:], 1, p.Dh, S, n, tb)
			kismi := j0 < tamLo || j0+n > tamHi || md != nil
			bos := true
			for r := 0; r < m; r++ {
				row := S[r*n : (r+1)*n]
				// Üstel yalnızca bakılabilen [a, c) sütunlarında hesaplanır; dışı 0.
				a, c := 0, n
				if kismi {
					a, c = p.blokAraligi(i0+r, j0, n)
				}
				seg := row[a:c]
				en := eksiSonsuz
				if md != nil {
					for jj, x := range seg {
						if maskeli(md, taban, i0+r, j0+a+jj, &p) {
							seg[jj] = eksiSonsuz
							continue
						}
						x *= scale
						seg[jj] = x
						if x > en {
							en = x
						}
					}
				} else {
					for jj, x := range seg {
						x *= scale
						seg[jj] = x
						if x > en {
							en = x
						}
					}
				}
				if en == eksiSonsuz {
					clear(row) // bu blokta bakılacak anahtar yok
					continue
				}
				bos = false
				eski := enb[r]
				yeni := max(eski, en)
				for jj, x := range seg {
					seg[jj] = x - yeni
				}
				p.ustelMaskeli(seg, md, taban, i0+r, j0+a)
				clear(row[:a])
				clear(row[c:])
				var top float32
				for _, x := range seg {
					top += x
				}
				if eski != yeni {
					// İlk katkıda (eski -sonsuz) O satırı ve payda zaten 0'dır.
					if eski != eksiSonsuz {
						alfa := expf32(eski - yeni)
						payda[r] *= alfa
						or := O[r*p.Dv : (r+1)*p.Dv]
						for d := range or {
							or[d] *= alfa
						}
					}
					enb[r] = yeni
				}
				payda[r] += top
			}
			if !bos {
				gemmTek(m, p.Dv, n, S, n, 1, vb[j0*p.Dv:], p.Dv, 1, O, p.Dv, tb)
			}
		}
		for r := 0; r < m; r++ {
			or := ob[r*p.Dv : (r+1)*p.Dv]
			if payda[r] == 0 {
				clear(or)
				lsb[r] = eksiSonsuz
				continue
			}
			ters := 1 / payda[r]
			for d, x := range O[r*p.Dv : (r+1)*p.Dv] {
				or[d] = x * ters
			}
			lsb[r] = enb[r] + log32(payda[r])
		}
	})
	for _, tb := range tamponlar {
		if tb != nil {
			dikkatTamponHavuzu.Put(tb)
		}
	}
	return lse
}

// dikkatGeri geri geçişin paylaşılan (salt okunur) durumu.
type dikkatGeri struct {
	p               *AttentionParams
	qd, kd, vd      []float32
	gd, ld, md, D   []float32
	gq, gk, gv      []float32 // nil olanlar hesaplanmaz
	bq, bk, grup    int
	scale           float32
	tamponlar       []*dikkatTampon
	nqb, nkb, bhSay int
}

func (s *dikkatGeri) tampon(kat int) *dikkatTampon {
	tb := s.tamponlar[kat]
	if tb == nil {
		tb = dikkatTamponAl()
		s.tamponlar[kat] = tb
	}
	return tb
}

// ikili sorgu bloğu [i0, i0+m) (baş bh) ile anahtar bloğu [j0, j0+n) (k/v başı
// kvh) için P'yi tb.s'ye yeniden hesaplar; dS isteniyorsa dS = P∘(dP-D)·ölçek'i
// tb.dp'ye yazar. dVa verilmişse dVa += Pᵀ·dO eklenir.
func (s *dikkatGeri) ikili(tb *dikkatTampon, bh, kvh, i0, m, j0, n int, dVa []float32, dS bool) {
	p := s.p
	qi := s.qd[(bh*p.Tq+i0)*p.Dh:]
	doi := s.gd[(bh*p.Tq+i0)*p.Dv:]
	kj := s.kd[(kvh*p.Tk+j0)*p.Dh:]
	vj := s.vd[(kvh*p.Tk+j0)*p.Dv:]
	tb.s = boyutla(tb.s, m*n)
	S := tb.s
	clear(S)
	gemmTek(m, n, p.Dh, qi, p.Dh, 1, kj, 1, p.Dh, S, n, tb)
	_, tamHi := p.Limits(i0)
	tamLo, _ := p.Limits(i0 + m - 1)
	kismi := j0 < tamLo || j0+n > tamHi || s.md != nil
	mtaban := 0
	if s.md != nil {
		mtaban = p.MaskOffsets[bh]
	}
	for r := 0; r < m; r++ {
		row := S[r*n : (r+1)*n]
		L := s.ld[bh*p.Tq+i0+r]
		if L == eksiSonsuz {
			clear(row)
			continue
		}
		a, c := 0, n
		if kismi {
			a, c = p.blokAraligi(i0+r, j0, n)
		}
		seg := row[a:c]
		for jj, x := range seg {
			seg[jj] = x*s.scale - L
		}
		if s.md != nil {
			for jj := range seg {
				if maskeli(s.md, mtaban, i0+r, j0+a+jj, p) {
					seg[jj] = eksiSonsuz
				}
			}
		}
		p.ustelMaskeli(seg, s.md, mtaban, i0+r, j0+a)
		clear(row[:a])
		clear(row[c:])
	}
	if dVa != nil {
		gemmTek(n, p.Dv, m, S, 1, n, doi, p.Dv, 1, dVa, p.Dv, tb)
	}
	if !dS {
		return
	}
	tb.dp = boyutla(tb.dp, m*n)
	dP := tb.dp
	clear(dP)
	gemmTek(m, n, p.Dv, doi, p.Dv, 1, vj, 1, p.Dv, dP, n, tb)
	for r := 0; r < m; r++ {
		Dr := s.D[bh*p.Tq+i0+r]
		ps, ds := S[r*n:(r+1)*n], dP[r*n:(r+1)*n]
		for jj, x := range ps {
			ds[jj] = x * (ds[jj] - Dr) * s.scale
		}
	}
}

// sorguAraligi anahtar bloğu [j0, j1)'e bakan sorgu satırlarını kapsayan,
// sorgu blok ızgarasına hizalı aralık [iBas, iSon).
func (s *dikkatGeri) sorguAraligi(j0, j1 int) (iBas, iSon int) {
	p := s.p
	iMin := sort.Search(p.Tq, func(i int) bool { _, hi := p.Limits(i); return hi > j0 })
	iMax := sort.Search(p.Tq, func(i int) bool { lo, _ := p.Limits(i); return lo >= j1 })
	if iMin >= iMax {
		return 0, 0
	}
	return iMin / s.bq * s.bq, iMax
}

// anahtarBlogu k/v başı kvh'nin jb. anahtar bloğu için dK ve dV'yi (bu k/v
// başını kullanan bütün sorgu başları ve bloğa bakan bütün sorgu blokları
// üzerinden) hesaplayıp gk/gv'ye ekler; dq doğruysa dQ katkılarını da gq'ya
// ekler (yalnızca tek geçiş kipinde: o zaman bir sorgu başının bütün anahtar
// blokları aynı birimde, artan sırayla gezilir).
func (s *dikkatGeri) anahtarBlogu(tb *dikkatTampon, kvh, jb int, dq bool) {
	p := s.p
	j0 := jb * s.bk
	n := min(s.bk, p.Tk-j0)
	iBas, iSon := s.sorguAraligi(j0, j0+n)
	if iBas >= iSon {
		return
	}
	var dKa, dVa []float32
	if s.gk != nil {
		tb.ka = boyutla(tb.ka, n*p.Dh)
		dKa = tb.ka
		clear(dKa)
	}
	if s.gv != nil {
		tb.va = boyutla(tb.va, n*p.Dv)
		dVa = tb.va
		clear(dVa)
	}
	dS := dKa != nil || dq
	b, g := kvh/p.Hkv, kvh%p.Hkv
	kj := s.kd[(kvh*p.Tk+j0)*p.Dh:]
	for hh := 0; hh < s.grup; hh++ {
		bh := b*p.H + g*s.grup + hh
		for i0 := iBas; i0 < iSon; i0 += s.bq {
			m := min(s.bq, p.Tq-i0)
			s.ikili(tb, bh, kvh, i0, m, j0, n, dVa, dS)
			if dq {
				gemmTek(m, p.Dh, n, tb.dp, n, 1, kj, p.Dh, 1, s.gq[(bh*p.Tq+i0)*p.Dh:], p.Dh, tb)
			}
			if dKa != nil {
				qi := s.qd[(bh*p.Tq+i0)*p.Dh:]
				gemmTek(n, p.Dh, m, tb.dp, 1, n, qi, p.Dh, 1, dKa, p.Dh, tb)
			}
		}
	}
	if dKa != nil {
		h := s.gk[(kvh*p.Tk+j0)*p.Dh : (kvh*p.Tk+j0+n)*p.Dh]
		for i, x := range dKa {
			h[i] += x
		}
	}
	if dVa != nil {
		h := s.gv[(kvh*p.Tk+j0)*p.Dv : (kvh*p.Tk+j0+n)*p.Dv]
		for i, x := range dVa {
			h[i] += x
		}
	}
}

// sorguBlogu baş bh'nin ib. sorgu bloğunun dQ'sunu, bloğun baktığı anahtar
// bloklarını artan sırayla gezerek gq'ya ekler (iki geçiş kipinin ikinci
// geçişi). Toplama sırası ve blok ızgarası tek geçiş kipiyle aynıdır; sonuç
// bit düzeyinde aynıdır.
func (s *dikkatGeri) sorguBlogu(tb *dikkatTampon, bh, ib int) {
	p := s.p
	i0 := ib * s.bq
	m := min(s.bq, p.Tq-i0)
	b, h := bh/p.H, bh%p.H
	kvh := b*p.Hkv + h/s.grup
	for jb := 0; jb < s.nkb; jb++ {
		j0 := jb * s.bk
		n := min(s.bk, p.Tk-j0)
		if iBas, iSon := s.sorguAraligi(j0, j0+n); i0 < iBas || i0 >= iSon {
			continue
		}
		s.ikili(tb, bh, kvh, i0, m, j0, n, nil, true)
		gemmTek(m, p.Dh, n, tb.dp, n, 1, s.kd[(kvh*p.Tk+j0)*p.Dh:], p.Dh, 1, s.gq[(bh*p.Tq+i0)*p.Dh:], p.Dh, tb)
	}
}

func (cpuBackend) f32AttentionGrad(gq, gk, gv, g Storage, q, k, v View, mask Storage, o, lse Storage, p AttentionParams) {
	if gq == nil && gk == nil && gv == nil {
		return
	}
	satir := p.Batch * p.H * p.Tq
	if satir == 0 || p.Tk == 0 {
		return
	}
	bq, bk := dikkatBloklari(&p)
	s := &dikkatGeri{p: &p,
		qd: cd(q.S)[q.Offset:], kd: cd(k.S)[k.Offset:], vd: cd(v.S)[v.Offset:],
		gd: cd(g), ld: cd(lse), md: cd(mask), gq: cd(gq), gk: cd(gk), gv: cd(gv),
		bq: bq, bk: bk, grup: p.H / p.Hkv, scale: p.Scale,
		nqb: (p.Tq + bq - 1) / bq, nkb: (p.Tk + bk - 1) / bk, bhSay: p.Batch * p.H}
	// Satır başına D = Σ dO·O.
	od := cd(o)
	s.D = make([]float32, satir)
	parallelFor(satir, p.Dv, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			gs, os := s.gd[i*p.Dv:(i+1)*p.Dv], od[i*p.Dv:(i+1)*p.Dv]
			var t float32
			for d, x := range gs {
				t += x * os[d]
			}
			s.D[i] = t
		}
	})
	W := dikkatIsciSayisi(2 * dikkatIs(&p))
	s.tamponlar = make([]*dikkatTampon, max(W, 1))
	kvSay := p.Batch * p.Hkv
	if s.gq == nil || kvSay >= 2*W {
		// Tek geçiş: birim bir k/v başıdır; dK, dV ve dQ birlikte biriktirilir.
		dagit(W, kvSay, func(kat, u int) {
			tb := s.tampon(kat)
			for jb := 0; jb < s.nkb; jb++ {
				s.anahtarBlogu(tb, u, jb, s.gq != nil)
			}
		})
	} else {
		// İki geçiş (az baş, çok işçi): önce (k/v başı, anahtar bloğu)
		// birimleriyle dK ve dV, sonra (baş, sorgu bloğu) birimleriyle dQ.
		// Puanlar iki kez hesaplanır ama ek bellek gerekmez ve birimler
		// küçüktür (yavaş ve hızlı çekirdekler arasında yük dengesi).
		if s.gk != nil || s.gv != nil {
			dagit(W, kvSay*s.nkb, func(kat, u int) {
				s.anahtarBlogu(s.tampon(kat), u%kvSay, u/kvSay, false)
			})
		}
		dagit(W, s.bhSay*s.nqb, func(kat, u int) {
			s.sorguBlogu(s.tampon(kat), u%s.bhSay, s.nqb-1-u/s.bhSay)
		})
	}
	for _, tb := range s.tamponlar {
		if tb != nil {
			dikkatTamponHavuzu.Put(tb)
		}
	}
}

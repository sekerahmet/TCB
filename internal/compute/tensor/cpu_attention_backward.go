package tensor

import (
	"sort"
)

// attnBackward geri geçişin paylaşılan (salt okunur) durumu.
type attnBackward struct {
	p                 *AttentionParams
	qd, kd, vd        []float32
	gd, ld, md, D     []float32
	gq, gk, gv        []float32 // nil olanlar hesaplanmaz
	bq, bk, group     int
	scale             float32
	buffers           []*attnBuf
	nqb, nkb, bhCount int
}

func (s *attnBackward) buffer(factor int) *attnBuf {
	tb := s.buffers[factor]
	if tb == nil {
		tb = takeAttnBuf()
		s.buffers[factor] = tb
	}
	return tb
}

// binaryArg sorgu bloğu [i0, i0+m) (bakış bh) ile anahtar bloğu [j0, j0+n) (k/v bakışı
// kvh) için P'yi tb.s'ye yeniden hesaplar; dS isteniyorsa dS = P∘(dP-D)·ölçek'i
// tb.dp'ye yazar. dVa verilmişse dVa += Pᵀ·dO eklenir.
func (s *attnBackward) binaryArg(tb *attnBuf, bh, kvh, i0, m, j0, n int, dVa []float32, dS bool) {
	p := s.p
	qi := s.qd[(bh*p.Tq+i0)*p.Dh:]
	doi := s.gd[(bh*p.Tq+i0)*p.Dv:]
	kj := s.kd[(kvh*p.Tk+j0)*p.Dh:]
	vj := s.vd[(kvh*p.Tk+j0)*p.Dv:]
	tb.s = sized(tb.s, m*n)
	S := tb.s
	clear(S)
	gemmSingle(m, n, p.Dh, qi, p.Dh, 1, kj, 1, p.Dh, S, n, tb)
	_, wholeHi := p.Limits(i0)
	wholeLo, _ := p.Limits(i0 + m - 1)
	partial := j0 < wholeLo || j0+n > wholeHi || s.md != nil
	mBase := 0
	if s.md != nil {
		mBase = p.MaskBase + p.MaskOffsets[bh]
	}
	for r := 0; r < m; r++ {
		row := S[r*n : (r+1)*n]
		L := s.ld[bh*p.Tq+i0+r]
		if L == negInf {
			clear(row)
			continue
		}
		a, c := 0, n
		if partial {
			a, c = p.blockRange(i0+r, j0, n)
		}
		seg := row[a:c]
		for jj, x := range seg {
			seg[jj] = x*s.scale - L
		}
		if s.md != nil {
			for jj := range seg {
				if masked(s.md, mBase, i0+r, j0+a+jj, p) {
					seg[jj] = negInf
				}
			}
		}
		p.expMasked(seg, s.md, mBase, i0+r, j0+a)
		clear(row[:a])
		clear(row[c:])
	}
	if dVa != nil {
		gemmSingle(n, p.Dv, m, S, 1, n, doi, p.Dv, 1, dVa, p.Dv, tb)
	}
	if !dS {
		return
	}
	tb.dp = sized(tb.dp, m*n)
	dP := tb.dp
	clear(dP)
	gemmSingle(m, n, p.Dv, doi, p.Dv, 1, vj, 1, p.Dv, dP, n, tb)
	for r := 0; r < m; r++ {
		Dr := s.D[bh*p.Tq+i0+r]
		ps, ds := S[r*n:(r+1)*n], dP[r*n:(r+1)*n]
		if s.ld[bh*p.Tq+i0+r] == negInf {
			clear(ds) // hiçbir anahtara bakmayan satır: gradyan 0
			continue
		}
		a, c := 0, n
		if partial {
			a, c = p.blockRange(i0+r, j0, n)
		}
		// Bakılmayan (pencere, nedensellik ya da maske dışı) hücrenin gradyanı 0'dır; çarpımla
		// bulunmaz: satırın Dr'ı NaN ise 0·NaN bakmadığı anahtarlara NaN sızdırırdı (15.24).
		clear(ds[:a])
		clear(ds[c:])
		for jj := a; jj < c; jj++ {
			if s.md != nil && masked(s.md, mBase, i0+r, j0+jj, p) {
				ds[jj] = 0
				continue
			}
			ds[jj] = ps[jj] * (ds[jj] - Dr) * s.scale
		}
	}
}

// nanRows bakış bh'nin [i0, i0+m) sorgu satırlarından logsumexp'i NaN olanları (puanı NaN satır)
// işaretler; hiç yoksa nil verir (yaygın yol ayırma yapmaz).
func (s *attnBackward) nanRows(bh, i0, m int) []bool {
	var rows []bool
	for r := 0; r < m; r++ {
		if l := s.ld[bh*s.p.Tq+i0+r]; l != l {
			if rows == nil {
				rows = make([]bool, m)
			}
			rows[r] = true
		}
	}
	return rows
}

// queryRange anahtar bloğu [j0, j1)'e bakan sorgu satırlarını kapsayan,
// sorgu blok ızgarasına hizalı aralık [iBas, iSon).
func (s *attnBackward) queryRange(j0, j1 int) (iStart, iEnd int) {
	p := s.p
	iMin := sort.Search(p.Tq, func(i int) bool { _, hi := p.Limits(i); return hi > j0 })
	iMax := sort.Search(p.Tq, func(i int) bool { lo, _ := p.Limits(i); return lo >= j1 })
	if iMin >= iMax {
		return 0, 0
	}
	return iMin / s.bq * s.bq, iMax
}

// keyBlock k/v bakışı kvh'nin jb. anahtar bloğu için dK ve dV'yi (bu k/v
// bakışını kullanan bütün sorgu bakışları ve bloğa bakan bütün sorgu blokları
// üzerinden) hesaplayıp gk/gv'ye ekler; dq doğruysa dQ katkılarını da gq'ya
// ekler (yalnızca tek geçiş kipinde: o zaman bir sorgu bakışının bütün anahtar
// blokları aynı birimde, artan sırayla gezilir).
func (s *attnBackward) keyBlock(tb *attnBuf, kvh, jb int, dq bool) {
	p := s.p
	j0 := jb * s.bk
	n := min(s.bk, p.Tk-j0)
	iStart, iEnd := s.queryRange(j0, j0+n)
	if iStart >= iEnd {
		return
	}
	var dKa, dVa []float32
	if s.gk != nil {
		tb.ka = sized(tb.ka, n*p.Dh)
		dKa = tb.ka
		clear(dKa)
	}
	if s.gv != nil {
		tb.va = sized(tb.va, n*p.Dv)
		dVa = tb.va
		clear(dVa)
	}
	dS := dKa != nil || dq
	b, g := kvh/p.Hkv, kvh%p.Hkv
	kj := s.kd[(kvh*p.Tk+j0)*p.Dh:]
	for hh := 0; hh < s.group; hh++ {
		bh := b*p.H + g*s.group + hh
		for i0 := iStart; i0 < iEnd; i0 += s.bq {
			m := min(s.bq, p.Tq-i0)
			s.binaryArg(tb, bh, kvh, i0, m, j0, n, dVa, dS)
			if dq {
				gemmSingle(m, p.Dh, n, tb.dp, n, 1, kj, p.Dh, 1, s.gq[(bh*p.Tq+i0)*p.Dh:], p.Dh, tb)
			}
			if dKa != nil {
				qi := s.qd[(bh*p.Tq+i0)*p.Dh:]
				// NaN puanlı bir satırın sorgusu dK çarpımına girerken sıfırlanır: bakmadığı anahtarın
				// dS'si 0'dır ama 0·NaN NaN verirdi; bakılan anahtarlar dS'den zaten NaN alır (15.24).
				if rows := s.nanRows(bh, i0, m); rows != nil {
					tb.qz = sized(tb.qz, m*p.Dh)
					for r := 0; r < m; r++ {
						if rows[r] {
							clear(tb.qz[r*p.Dh : (r+1)*p.Dh])
						} else {
							copy(tb.qz[r*p.Dh:(r+1)*p.Dh], qi[r*p.Dh:(r+1)*p.Dh])
						}
					}
					qi = tb.qz
				}
				gemmSingle(n, p.Dh, m, tb.dp, 1, n, qi, p.Dh, 1, dKa, p.Dh, tb)
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

// queryBlock bakış bh'nin ib. sorgu bloğunun dQ'sunu, bloğun baktığı anahtar
// bloklarını artan sırayla gezerek gq'ya ekler (iki geçiş kipinin ikinci
// geçişi). Toplama sırası ve blok ızgarası tek geçiş kipiyle aynıdır; sonuç
// bit düzeyinde aynıdır.
func (s *attnBackward) queryBlock(tb *attnBuf, bh, ib int) {
	p := s.p
	i0 := ib * s.bq
	m := min(s.bq, p.Tq-i0)
	b, h := bh/p.H, bh%p.H
	kvh := b*p.Hkv + h/s.group
	for jb := 0; jb < s.nkb; jb++ {
		j0 := jb * s.bk
		n := min(s.bk, p.Tk-j0)
		if iStart, iEnd := s.queryRange(j0, j0+n); i0 < iStart || i0 >= iEnd {
			continue
		}
		s.binaryArg(tb, bh, kvh, i0, m, j0, n, nil, true)
		gemmSingle(m, p.Dh, n, tb.dp, n, 1, s.kd[(kvh*p.Tk+j0)*p.Dh:], p.Dh, 1, s.gq[(bh*p.Tq+i0)*p.Dh:], p.Dh, tb)
	}
}

func (cpuBackend) f32AttentionGrad(gq, gk, gv, g Storage, q, k, v View, mask Storage, o, lse Storage, p AttentionParams) {
	if gq == nil && gk == nil && gv == nil {
		return
	}
	row := p.Batch * p.H * p.Tq
	if row == 0 || p.Tk == 0 {
		return
	}
	bq, bk := attnBlocks(&p)
	s := &attnBackward{p: &p,
		qd: cd(q.S)[q.Offset:], kd: cd(k.S)[k.Offset:], vd: cd(v.S)[v.Offset:],
		gd: cd(g), ld: cd(lse), md: cd(mask), gq: cd(gq), gk: cd(gk), gv: cd(gv),
		bq: bq, bk: bk, group: p.H / p.Hkv, scale: p.Scale,
		nqb: (p.Tq + bq - 1) / bq, nkb: (p.Tk + bk - 1) / bk, bhCount: p.Batch * p.H}
	// Satır başına D = Σ dO·O.
	od := cd(o)
	s.D = make([]float32, row)
	parallelFor(row, p.Dv, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			gs, os := s.gd[i*p.Dv:(i+1)*p.Dv], od[i*p.Dv:(i+1)*p.Dv]
			var t float32
			for d, x := range gs {
				t += x * os[d]
			}
			s.D[i] = t
		}
	})
	W := attnWorkerCount(2 * attnJob(&p))
	s.buffers = make([]*attnBuf, max(W, 1))
	kvCount := p.Batch * p.Hkv
	if s.gq == nil || kvCount >= 2*W {
		// Tek geçiş: birim bir k/v bakışıdır; dK, dV ve dQ birlikte biriktirilir.
		distribute(W, kvCount, func(factor, u int) {
			tb := s.buffer(factor)
			for jb := 0; jb < s.nkb; jb++ {
				s.keyBlock(tb, u, jb, s.gq != nil)
			}
		})
	} else {
		// İki geçiş (az bakış, çok işçi): önce (k/v bakışı, anahtar bloğu)
		// birimleriyle dK ve dV, sonra (bakış, sorgu bloğu) birimleriyle dQ.
		// Puanlar iki kez hesaplanır ama ek bellek gerekmez ve birimler
		// küçüktür (yavaş ve hızlı çekirdekler arasında yük dengesi).
		if s.gk != nil || s.gv != nil {
			distribute(W, kvCount*s.nkb, func(factor, u int) {
				s.keyBlock(s.buffer(factor), u%kvCount, u/kvCount, false)
			})
		}
		distribute(W, s.bhCount*s.nqb, func(factor, u int) {
			s.queryBlock(s.buffer(factor), u%s.bhCount, s.nqb-1-u/s.bhCount)
		})
	}
	for _, tb := range s.buffers {
		if tb != nil {
			attnBufPool.Put(tb)
		}
	}
}

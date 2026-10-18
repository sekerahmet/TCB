package gpu

import (
	"fmt"
	"runtime"
	"unsafe"
)

// ============================================================================
// Matris çarpımı kitaplığı: şekil planları ve çağrı
// ============================================================================
//
// Kitaplık sütun öncelikli çalışır; satır öncelikli C = op(A)·op(B), sütun öncelikli
// Cᵀ = op(B)ᵀ·op(A)ᵀ olarak verilir (A ile B'nin yeri değişir, devrik bayrakları aynı kalır).
// Toplu çarpım adımlı toplu düzendir. Her (şekil, devrikler, işaretçi hizaları) için bir plan
// kurulur: tanım, üç düzen, hizaların bildirildiği tercih ve sezgisel algoritma. Hiza bildirimi
// gereklidir: kitaplık bildirilmezse 256 bayt varsayar, ofsetli görünümde (ör. bir dilimin
// son satırı) hizasız erişim olurdu. Planlar en çok ltEnCokPlan tanedir (en eskisi yok edilir).
// Veri türü (ltR32F) 16 bitlik türler (2b: tensör çekirdekleri) için parametredir.

// Kitaplık sabitleri (cublasLt.h, library_types.h).
const (
	ltHesap32F       = 68 // CUBLAS_COMPUTE_32F
	ltR32F           = 0  // CUDA_R_32F
	ltTanimDevrikA   = 3  // CUBLASLT_MATMUL_DESC_TRANSA
	ltTanimDevrikB   = 4  // CUBLASLT_MATMUL_DESC_TRANSB
	ltDuzenToplu     = 5  // CUBLASLT_MATRIX_LAYOUT_BATCH_COUNT
	ltDuzenTopluAdim = 6  // CUBLASLT_MATRIX_LAYOUT_STRIDED_BATCH_OFFSET
	ltTercihAlan     = 1  // CUBLASLT_MATMUL_PREF_MAX_WORKSPACE_BYTES
	// CUBLASLT_MATMUL_PREF_MIN_ALIGNMENT_{A,B,C,D}_BYTES (uint32): 5, 6, 7, 8.
	ltTercihHizaA  = 5
	ltCalismaAlani = 32 << 20
	ltEnCokPlan    = 256
)

// ltAnahtar bir planın önbellek anahtarı (hizalar kitaplığın A, B, C işaretçileri içindir).
type ltAnahtar struct {
	batch, M, N, K   int
	ta, tb           bool
	hizA, hizB, hizC uint32
}

// ltPlan bir şeklin tanımları ve seçilen algoritması.
type ltPlan struct {
	tanim, tercih uintptr
	duzen         [3]uintptr // kitaplığın A'sı (bizim B), B'si (bizim A), C/D
	algo          [8]uint64
	alan          uint64
	gecerli       bool
}

// ltCikti kitaplığın yazdığı ve okuduğu değerler (yığında, taşınmaz).
type ltCikti struct {
	tutamac    uintptr
	sezgi      [12]uint64 // cublasLtMatmulHeuristicResult_t (96 bayt)
	sayi       int32
	alfa, beta float32
	i32        int32
	u32        uint32
	i64        int64
	u64        uint64
}

// ltCarpim bir GPU'nun kitaplık bağlantısı.
type ltCarpim struct {
	kit   *ltKitaplik
	h     uintptr
	alan  *depo
	o     *ltCikti
	plan  map[ltAnahtar]*ltPlan
	sira  []ltAnahtar // planların kuruluş sırası (en eskisi önce)
	adlar [4]string
	dusus int64 // kendi çekirdeğe düşülen çağrı sayısı
}

// hiza adresi bölen en büyük ikinin kuvveti (en çok 256 bayt).
func hiza(adr uint64) uint32 {
	if adr%256 == 0 {
		return 256
	}
	return uint32(adr & -adr)
}

// planYoket planın kitaplık nesnelerini yok eder.
func (l *ltCarpim) planYoket(p *ltPlan) {
	if p.tanim != 0 {
		l.cagir(ltTanimYoket, p.tanim)
	}
	for _, d := range p.duzen {
		if d != 0 {
			l.cagir(ltDuzenYoket, d)
		}
	}
	if p.tercih != 0 {
		l.cagir(ltTercihYoket, p.tercih)
	}
}

// planAl şeklin planını kurar (bir kez); kitaplık algoritma vermezse geçersiz plan.
func (l *ltCarpim) planAl(a ltAnahtar) *ltPlan {
	if p, ok := l.plan[a]; ok {
		return p
	}
	if len(l.sira) >= ltEnCokPlan {
		eski := l.sira[0]
		l.sira = l.sira[1:]
		l.planYoket(l.plan[eski])
		delete(l.plan, eski)
	}
	p := &ltPlan{}
	l.plan[a] = p
	l.sira = append(l.sira, a)
	if !l.tanimKur(p, a) || !l.tercihKur(p, a) {
		return p
	}
	o := l.o
	o.sayi = 0
	s := l.cagir(ltSezgisel, l.h, p.tanim, p.duzen[0], p.duzen[1], p.duzen[2], p.duzen[2], p.tercih, 1,
		uintptr(unsafe.Pointer(&o.sezgi[0])), uintptr(unsafe.Pointer(&o.sayi)))
	if s != 0 || o.sayi < 1 || int32(uint32(o.sezgi[9])) != 0 {
		return p
	}
	copy(p.algo[:], o.sezgi[:8])
	p.alan = o.sezgi[8]
	p.gecerli = p.alan <= ltCalismaAlani
	return p
}

// tanimKur işlem tanımını ve düzenleri kurar.
func (l *ltCarpim) tanimKur(p *ltPlan, a ltAnahtar) bool {
	o := l.o
	if l.cagir(ltTanimOlustur, uintptr(unsafe.Pointer(&o.tutamac)), ltHesap32F, ltR32F) != 0 {
		return false
	}
	p.tanim = o.tutamac
	// Kitaplığın A'sı bizim B'miz, B'si bizim A'mız (bkz. dosya başı).
	for i, devrik := range []bool{a.tb, a.ta} {
		o.i32 = 0
		if devrik {
			o.i32 = 1
		}
		if l.cagir(ltTanimAyarla, p.tanim, uintptr(ltTanimDevrikA+i), uintptr(unsafe.Pointer(&o.i32)), 4) != 0 {
			return false
		}
	}
	// Sütun öncelikli boylar: bizim B (satırca K×N ya da N×K), A (M×K ya da K×M), C (M×N).
	type boy struct{ sat, sut, adim int }
	duz := []boy{{a.N, a.K, a.K * a.N}, {a.K, a.M, a.M * a.K}, {a.N, a.M, a.M * a.N}}
	if a.tb {
		duz[0] = boy{a.K, a.N, a.K * a.N}
	}
	if a.ta {
		duz[1] = boy{a.M, a.K, a.M * a.K}
	}
	for i, b := range duz {
		if l.cagir(ltDuzenOlustur, uintptr(unsafe.Pointer(&o.tutamac)), ltR32F, uintptr(b.sat), uintptr(b.sut), uintptr(b.sat)) != 0 {
			return false
		}
		p.duzen[i] = o.tutamac
		if a.batch > 1 {
			o.i32, o.i64 = int32(a.batch), int64(b.adim)
			if l.cagir(ltDuzenAyarla, p.duzen[i], ltDuzenToplu, uintptr(unsafe.Pointer(&o.i32)), 4) != 0 ||
				l.cagir(ltDuzenAyarla, p.duzen[i], ltDuzenTopluAdim, uintptr(unsafe.Pointer(&o.i64)), 8) != 0 {
				return false
			}
		}
	}
	return true
}

// tercihKur çalışma alanı sınırı ve işaretçi hizalarıyla tercih nesnesini kurar.
func (l *ltCarpim) tercihKur(p *ltPlan, a ltAnahtar) bool {
	o := l.o
	if l.cagir(ltTercihOlustur, uintptr(unsafe.Pointer(&o.tutamac))) != 0 {
		return false
	}
	p.tercih = o.tutamac
	o.u64 = ltCalismaAlani
	if l.cagir(ltTercihAyarla, p.tercih, ltTercihAlan, uintptr(unsafe.Pointer(&o.u64)), 8) != 0 {
		return false
	}
	for i, h := range []uint32{a.hizA, a.hizB, a.hizC, a.hizC} { // A, B, C, D
		o.u32 = h
		if l.cagir(ltTercihAyarla, p.tercih, uintptr(ltTercihHizaA+i), uintptr(unsafe.Pointer(&o.u32)), 4) != 0 {
			return false
		}
	}
	return true
}

// ltCarp C (+)= op(A)·op(B) kitaplıkla; kitaplık bu çağrıyı yapamazsa (algoritma yok ya da
// çağrı hata döndü) yanlış döner ve kendi çekirdeğimiz kullanılır (düşüş sayılır). Adresler
// cihaz adresleridir (float32 bitişik).
func (g *arkaUc) ltCarp(batch, M, N, K int, a uint64, ta bool, b uint64, tb bool, c uint64, uzerine bool) bool {
	l := g.lt
	if l == nil || K == 0 {
		return false
	}
	p := l.planAl(ltAnahtar{batch: batch, M: M, N: N, K: K, ta: ta, tb: tb, hizA: hiza(b), hizB: hiza(a), hizC: hiza(c)})
	if !p.gecerli {
		l.dusus++
		return false
	}
	o := l.o
	o.alfa, o.beta = 1, 1
	if uzerine {
		o.beta = 0
	}
	ad := 0
	if ta {
		ad += 2
	}
	if tb {
		ad++
	}
	ev := g.olcBasla()
	s := l.cagir(ltCarp, l.h, p.tanim, uintptr(unsafe.Pointer(&o.alfa)), uintptr(b), p.duzen[0], uintptr(a), p.duzen[1],
		uintptr(unsafe.Pointer(&o.beta)), uintptr(c), p.duzen[2], uintptr(c), p.duzen[2],
		uintptr(unsafe.Pointer(&p.algo[0])), uintptr(l.alan.adr), uintptr(p.alan), 0)
	runtime.KeepAlive(p)
	g.olcBitir(l.adlar[ad], ev)
	if s != 0 {
		// Durum koduyla reddedilen çağrı hiçbir şey yazmamıştır: plan bırakılır, kendi
		// çekirdeğimiz yapar.
		p.gecerli = false
		l.dusus++
		g.ltSonHata = fmt.Sprintf("cuBLASLt durum %d (%d×%d×%d, toplu %d)", s, M, N, K, batch)
		return false
	}
	g.sonCekirdek = l.adlar[ad]
	if g.eszamanli {
		g.esitle()
	}
	return true
}

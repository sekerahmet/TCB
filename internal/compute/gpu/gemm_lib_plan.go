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
// A ve B'nin veri türü plan anahtarındadır: float32 ya da 16 bitlik girdiler (bf16, f16;
// tensör çekirdekleri). C her zaman float32'dir, birikim float32'de yapılır (CUBLAS_COMPUTE_32F).

// Kitaplık sabitleri (cublasLt.h, library_types.h).
const (
	ltCompute32F        = 68 // CUBLAS_COMPUTE_32F
	ltR32F              = 0  // CUDA_R_32F
	ltR16F              = 2  // CUDA_R_16F
	ltR16BF             = 14 // CUDA_R_16BF
	ltDescTransA        = 3  // CUBLASLT_MATMUL_DESC_TRANSA
	ltDescTransB        = 4  // CUBLASLT_MATMUL_DESC_TRANSB
	ltLayoutBatch       = 5  // CUBLASLT_MATRIX_LAYOUT_BATCH_COUNT
	ltLayoutBatchStride = 6  // CUBLASLT_MATRIX_LAYOUT_STRIDED_BATCH_OFFSET
	ltPrefSpace         = 1  // CUBLASLT_MATMUL_PREF_MAX_WORKSPACE_BYTES
	// CUBLASLT_MATMUL_PREF_MIN_ALIGNMENT_{A,B,C,D}_BYTES (uint32): 5, 6, 7, 8.
	ltPrefAlignA = 5
	ltWorkspace  = 32 << 20
	ltMaxPlans   = 256
)

// ltKey bir planın önbellek anahtarı (hizalar kitaplığın A, B, C işaretçileri içindir; ab A
// ile B'nin, ct C'nin veri türüdür).
type ltKey struct {
	batch, M, N, K         int
	ta, tb                 bool
	alignA, alignB, alignC uint32
	ab, ct                 uintptr
}

// ltPlanFor bir şeklin tanımları ve seçilen algoritması.
type ltPlanFor struct {
	def, preference uintptr
	layout          [3]uintptr // kitaplığın A'sı (bizim B), B'si (bizim A), C/D
	algo            [8]uint64
	space           uint64
	valid           bool
}

// ltOutput kitaplığın yazdığı ve okuduğu değerler (yığında, taşınmaz).
type ltOutput struct {
	handle      uintptr
	heuristic   [12]uint64 // cublasLtMatmulHeuristicResult_t (96 bayt)
	num         int32
	alpha, beta float32
	i32         int32
	u32         uint32
	i64         int64
	u64         uint64
}

// ltGemm bir GPU'nun kitaplık bağlantısı.
type ltGemm struct {
	kitLib          *ltLibrary
	h               uintptr
	space           *storage
	o               *ltOutput
	plan            map[ltKey]*ltPlanFor
	order           []ltKey // planların kuruluş sırası (en eskisi önce)
	names           [4]string
	fallthroughCase int64 // kendi çekirdeğe düşülen çağrı sayısı
}

// align adresi bölen en büyük ikinin kuvveti (en çok 256 bayt).
func align(addr uint64) uint32 {
	if addr%256 == 0 {
		return 256
	}
	return uint32(addr & -addr)
}

// planDestroy planın kitaplık nesnelerini yok eder.
func (l *ltGemm) planDestroy(p *ltPlanFor) {
	if p.def != 0 {
		l.call(ltDescDestroy, p.def)
	}
	for _, d := range p.layout {
		if d != 0 {
			l.call(ltLayoutDestroy, d)
		}
	}
	if p.preference != 0 {
		l.call(ltPrefDestroy, p.preference)
	}
}

// takePlan şeklin planını kurar (bir kez); kitaplık algoritma vermezse geçersiz plan.
func (l *ltGemm) takePlan(a ltKey) *ltPlanFor {
	if p, ok := l.plan[a]; ok {
		return p
	}
	if len(l.order) >= ltMaxPlans {
		old := l.order[0]
		l.order = l.order[1:]
		l.planDestroy(l.plan[old])
		delete(l.plan, old)
	}
	p := &ltPlanFor{}
	l.plan[a] = p
	l.order = append(l.order, a)
	if !l.setupDef(p, a) || !l.setupPreference(p, a) {
		return p
	}
	o := l.o
	o.num = 0
	s := l.call(ltHeuristic, l.h, p.def, p.layout[0], p.layout[1], p.layout[2], p.layout[2], p.preference, 1,
		uintptr(unsafe.Pointer(&o.heuristic[0])), uintptr(unsafe.Pointer(&o.num)))
	if s != 0 || o.num < 1 || int32(uint32(o.heuristic[9])) != 0 {
		return p
	}
	copy(p.algo[:], o.heuristic[:8])
	p.space = o.heuristic[8]
	p.valid = p.space <= ltWorkspace
	return p
}

// setupDef işlem tanımını ve düzenleri kurar.
func (l *ltGemm) setupDef(p *ltPlanFor, a ltKey) bool {
	o := l.o
	if l.call(ltDescCreate, uintptr(unsafe.Pointer(&o.handle)), ltCompute32F, ltR32F) != 0 {
		return false
	}
	p.def = o.handle
	// Kitaplığın A'sı bizim B'miz, B'si bizim A'mız (bkz. dosya başı).
	for i, transposed := range []bool{a.tb, a.ta} {
		o.i32 = 0
		if transposed {
			o.i32 = 1
		}
		if l.call(ltDescSet, p.def, uintptr(ltDescTransA+i), uintptr(unsafe.Pointer(&o.i32)), 4) != 0 {
			return false
		}
	}
	// Sütun öncelikli boylar: bizim B (satırca K×N ya da N×K), A (M×K ya da K×M), C (M×N).
	type size struct{ row2, column, step int }
	plain := []size{{a.N, a.K, a.K * a.N}, {a.K, a.M, a.M * a.K}, {a.N, a.M, a.M * a.N}}
	if a.tb {
		plain[0] = size{a.K, a.N, a.K * a.N}
	}
	if a.ta {
		plain[1] = size{a.M, a.K, a.M * a.K}
	}
	for i, b := range plain {
		dt := a.ct
		if i < 2 {
			dt = a.ab
		}
		if l.call(ltLayoutCreate, uintptr(unsafe.Pointer(&o.handle)), dt, uintptr(b.row2), uintptr(b.column), uintptr(b.row2)) != 0 {
			return false
		}
		p.layout[i] = o.handle
		if a.batch > 1 {
			o.i32, o.i64 = int32(a.batch), int64(b.step)
			if l.call(ltLayoutSet, p.layout[i], ltLayoutBatch, uintptr(unsafe.Pointer(&o.i32)), 4) != 0 ||
				l.call(ltLayoutSet, p.layout[i], ltLayoutBatchStride, uintptr(unsafe.Pointer(&o.i64)), 8) != 0 {
				return false
			}
		}
	}
	return true
}

// setupPreference çalışma alanı sınırı ve işaretçi hizalarıyla tercih nesnesini kurar.
func (l *ltGemm) setupPreference(p *ltPlanFor, a ltKey) bool {
	o := l.o
	if l.call(ltPrefCreate, uintptr(unsafe.Pointer(&o.handle))) != 0 {
		return false
	}
	p.preference = o.handle
	o.u64 = ltWorkspace
	if l.call(ltPrefSet, p.preference, ltPrefSpace, uintptr(unsafe.Pointer(&o.u64)), 8) != 0 {
		return false
	}
	for i, h := range []uint32{a.alignA, a.alignB, a.alignC, a.alignC} { // A, B, C, D
		o.u32 = h
		if l.call(ltPrefSet, p.preference, uintptr(ltPrefAlignA+i), uintptr(unsafe.Pointer(&o.u32)), 4) != 0 {
			return false
		}
	}
	return true
}

// ltMul C (+)= op(A)·op(B) kitaplıkla; kitaplık bu çağrıyı yapamazsa (algoritma yok ya da
// çağrı hata döndü) yanlış döner ve kendi çekirdeğimiz kullanılır (düşüş sayılır). Adresler
// cihaz adresleridir (bitişik); A ile B ab türünde (ltR32F, ltR16BF, ltR16F), C ct türünde
// (float32 ya da A ile aynı 16 bitlik tür). Biriktirme her zaman float32'dedir; 16 bitlik C
// eski içeriğiyle birlikte (β = 1) float32'de toplanıp bir kez yuvarlanır.
func (g *backend) ltMul(batch, M, N, K int, a uint64, ta bool, b uint64, tb bool, c uint64, over bool, ab, ct uintptr) bool {
	l := g.lt
	if l == nil || K == 0 {
		return false
	}
	p := l.takePlan(ltKey{batch: batch, M: M, N: N, K: K, ta: ta, tb: tb, alignA: align(b), alignB: align(a), alignC: align(c), ab: ab, ct: ct})
	if !p.valid {
		l.fallthroughCase++
		return false
	}
	o := l.o
	o.alpha, o.beta = 1, 1
	if over {
		o.beta = 0
	}
	ad := 0
	if ta {
		ad += 2
	}
	if tb {
		ad++
	}
	ev := g.measureStart()
	s := l.call(ltMul, l.h, p.def, uintptr(unsafe.Pointer(&o.alpha)), uintptr(b), p.layout[0], uintptr(a), p.layout[1],
		uintptr(unsafe.Pointer(&o.beta)), uintptr(c), p.layout[2], uintptr(c), p.layout[2],
		uintptr(unsafe.Pointer(&p.algo[0])), uintptr(l.space.addr), uintptr(p.space), 0)
	runtime.KeepAlive(p)
	g.measureEnd(l.names[ad], ev)
	if s != 0 {
		// Durum koduyla reddedilen çağrı hiçbir şey yazmamıştır: plan bırakılır, kendi
		// çekirdeğimiz yapar.
		p.valid = false
		l.fallthroughCase++
		g.ltLastError = fmt.Sprintf("cuBLASLt durum %d (%d×%d×%d, toplu %d)", s, M, N, K, batch)
		return false
	}
	g.lastKernel = l.names[ad]
	if g.concurrent {
		g.equalize()
	}
	return true
}

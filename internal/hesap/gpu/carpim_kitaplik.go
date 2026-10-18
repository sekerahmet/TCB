package gpu

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Matris çarpımı kitaplığı (NVIDIA cuBLASLt; makinede varsa)
// ============================================================================
//
// float32 matris çarpımı makinede kuruluysa üreticinin kitaplığıyla yapılır: kitaplık
// sürücü gibi çalışma anında yüklenir (dağıtılmaz; arama yolu carpimKitapligiAc'ta).
// Bulunamazsa, sürümü uyumsuzsa ya da bir şekil için algoritma vermezse aynı cihazda kendi
// çekirdeğimiz (cekirdek_carpim.go) kullanılır: sonuç aynıdır, yalnızca hız farklıdır.
// TR_GPU_KENDI_CARPIM=1 kendi çekirdeğimizi zorlar (sınama ve karşılaştırma).
//
// Hesap float32'dir (TF32 kapalı: CUBLAS_COMPUTE_32F). Kitaplık sütun öncelikli çalışır;
// satır öncelikli C = op(A)·op(B), sütun öncelikli Cᵀ = op(B)ᵀ·op(A)ᵀ olarak verilir (A ile
// B'nin yeri değişir, devrik bayrakları aynı kalır). Toplu çarpım adımlı toplu düzendir.
// Veri türü parametresi (ltTur) 16 bitlik türler (2b: tensör çekirdekleri) için hazırdır.

// ltIslevAdlari yüklenen kitaplık işlevleri.
var ltIslevAdlari = [...]string{
	"cublasLtGetVersion", "cublasLtCreate", "cublasLtMatmulDescCreate", "cublasLtMatmulDescSetAttribute",
	"cublasLtMatrixLayoutCreate", "cublasLtMatrixLayoutSetAttribute", "cublasLtMatmulPreferenceCreate",
	"cublasLtMatmulPreferenceSetAttribute", "cublasLtMatmulAlgoGetHeuristic", "cublasLtMatmul",
}

const (
	ltSurum = iota
	ltOlustur
	ltTanimOlustur
	ltTanimAyarla
	ltDuzenOlustur
	ltDuzenAyarla
	ltTercihOlustur
	ltTercihAyarla
	ltSezgisel
	ltCarp
	ltSayisi
)

// Kitaplık sabitleri (cublasLt.h, library_types.h).
const (
	ltHesap32F       = 68 // CUBLAS_COMPUTE_32F
	ltR32F           = 0  // CUDA_R_32F
	ltTanimDevrikA   = 3  // CUBLASLT_MATMUL_DESC_TRANSA
	ltTanimDevrikB   = 4  // CUBLASLT_MATMUL_DESC_TRANSB
	ltDuzenToplu     = 5  // CUBLASLT_MATRIX_LAYOUT_BATCH_COUNT
	ltDuzenTopluAdim = 6  // CUBLASLT_MATRIX_LAYOUT_STRIDED_BATCH_OFFSET
	ltTercihAlan     = 1  // CUBLASLT_MATMUL_PREF_MAX_WORKSPACE_BYTES
	ltCalismaAlani   = 32 << 20
	ltEnAzSurum      = 110000 // 11.0
)

// ltKitaplik süreç genelinde bir kez yüklenen kitaplık.
type ltKitaplik struct {
	k     kutuphane
	fn    [ltSayisi]uintptr
	yol   string
	surum int
	hata  error
}

var (
	ltBirKez sync.Once
	ltKit    *ltKitaplik
)

func ltYukle() *ltKitaplik {
	ltBirKez.Do(func() {
		l := &ltKitaplik{}
		ltKit = l
		k, yol, err := carpimKitapligiAc()
		if err != nil {
			l.hata = fmt.Errorf("cuBLASLt bulunamadı (%v)", err)
			return
		}
		l.k, l.yol = k, yol
		for i, ad := range ltIslevAdlari {
			if l.fn[i], err = k.bul(ad); err != nil {
				l.hata = fmt.Errorf("%s içinde %s yok", yol, ad)
				return
			}
		}
		l.surum = int(k.cagir(l.fn[ltSurum]))
		if l.surum < ltEnAzSurum {
			l.hata = fmt.Errorf("cuBLASLt sürümü %d çok eski", l.surum)
		}
	})
	return ltKit
}

// ltAnahtar bir çarpım şeklinin önbellek anahtarı.
type ltAnahtar struct {
	batch, M, N, K int
	ta, tb, uz     bool
}

// ltPlan bir şeklin tanımları ve seçilen algoritması.
type ltPlan struct {
	tanim   uintptr
	duzen   [3]uintptr // kitaplığın A'sı (bizim B), B'si (bizim A), C/D
	algo    [8]uint64
	alan    uint64
	gecerli bool
}

// ltCikti kitaplığın yazdığı ve okuduğu değerler (yığında, taşınmaz).
type ltCikti struct {
	tutamac    uintptr
	sezgi      [12]uint64 // cublasLtMatmulHeuristicResult_t (96 bayt)
	sayi       int32
	alfa, beta float32
	i32        int32
	i64        int64
	u64        uint64
}

// ltCarpim bir GPU'nun kitaplık bağlantısı.
type ltCarpim struct {
	kit    *ltKitaplik
	h      uintptr
	tercih uintptr
	alan   *depo
	o      *ltCikti
	plan   map[ltAnahtar]*ltPlan
	adlar  [4]string
}

func (l *ltCarpim) cagir(f int, args ...uintptr) int32 {
	r := l.kit.k.cagir(l.kit.fn[f], args...)
	runtime.KeepAlive(l.o)
	return int32(uint32(r))
}

// ltKur kitaplığı yükler ve bu GPU için bağlantı kurar (kurulumda, bir kez). Kurulamazsa
// g.lt nil kalır ve kendi çekirdeğimiz kullanılır; nedeni carpimYolu'nda görünür.
func (g *arkaUc) ltKur() {
	if os.Getenv("TR_GPU_KENDI_CARPIM") == "1" {
		g.ltNeden = "kendi çekirdeği (TR_GPU_KENDI_CARPIM=1)"
		return
	}
	kit := ltYukle()
	if kit.hata != nil {
		g.ltNeden = "kendi çekirdeği (" + kit.hata.Error() + ")"
		return
	}
	g.gir()
	defer g.cik()
	l := &ltCarpim{kit: kit, o: &ltCikti{}, plan: map[ltAnahtar]*ltPlan{}}
	if s := l.cagir(ltOlustur, uintptr(unsafe.Pointer(&l.o.tutamac))); s != 0 {
		g.ltNeden = fmt.Sprintf("kendi çekirdeği (cublasLtCreate durum %d)", s)
		return
	}
	l.h = l.o.tutamac
	if s := l.cagir(ltTercihOlustur, uintptr(unsafe.Pointer(&l.o.tutamac))); s != 0 {
		g.ltNeden = fmt.Sprintf("kendi çekirdeği (tercih durum %d)", s)
		return
	}
	l.tercih = l.o.tutamac
	l.o.u64 = ltCalismaAlani
	l.cagir(ltTercihAyarla, l.tercih, ltTercihAlan, uintptr(unsafe.Pointer(&l.o.u64)), 8)
	l.alan = g.ayir(ltCalismaAlani/4, tensor.Float32, false)
	for i, ad := range []string{"nn", "nt", "tn", "tt"} {
		l.adlar[i] = "cublaslt_carpim_" + ad
	}
	g.lt = l
}

// carpimYolu matris çarpımının hangi yoldan yapıldığı (teşhis ve profil).
func (g *arkaUc) carpimYolu() string {
	if g.lt != nil {
		return fmt.Sprintf("cuBLASLt %d.%d (%s)", g.lt.kit.surum/10000, g.lt.kit.surum/100%100, g.lt.kit.yol)
	}
	if g.ltNeden != "" {
		return g.ltNeden
	}
	return "kendi çekirdeği"
}

// planAl şeklin planını kurar (bir kez); kitaplık algoritma vermezse geçersiz plan.
func (l *ltCarpim) planAl(a ltAnahtar) *ltPlan {
	if p, ok := l.plan[a]; ok {
		return p
	}
	p := &ltPlan{}
	l.plan[a] = p
	o := l.o
	if l.cagir(ltTanimOlustur, uintptr(unsafe.Pointer(&o.tutamac)), ltHesap32F, ltR32F) != 0 {
		return p
	}
	p.tanim = o.tutamac
	// Kitaplığın A'sı bizim B'miz, B'si bizim A'mız (bkz. dosya başı).
	for i, devrik := range []bool{a.tb, a.ta} {
		o.i32 = 0
		if devrik {
			o.i32 = 1
		}
		l.cagir(ltTanimAyarla, p.tanim, uintptr(ltTanimDevrikA+i), uintptr(unsafe.Pointer(&o.i32)), 4)
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
			return p
		}
		p.duzen[i] = o.tutamac
		if a.batch > 1 {
			o.i32, o.i64 = int32(a.batch), int64(b.adim)
			l.cagir(ltDuzenAyarla, p.duzen[i], ltDuzenToplu, uintptr(unsafe.Pointer(&o.i32)), 4)
			l.cagir(ltDuzenAyarla, p.duzen[i], ltDuzenTopluAdim, uintptr(unsafe.Pointer(&o.i64)), 8)
		}
	}
	o.sayi = 0
	s := l.cagir(ltSezgisel, l.h, p.tanim, p.duzen[0], p.duzen[1], p.duzen[2], p.duzen[2], l.tercih, 1,
		uintptr(unsafe.Pointer(&o.sezgi[0])), uintptr(unsafe.Pointer(&o.sayi)))
	if s != 0 || o.sayi < 1 || int32(uint32(o.sezgi[9])) != 0 {
		return p
	}
	copy(p.algo[:], o.sezgi[:8])
	p.alan = o.sezgi[8]
	p.gecerli = p.alan <= ltCalismaAlani
	return p
}

// carp C (+)= op(A)·op(B) kitaplıkla; şekil desteklenmiyorsa yanlış döner (kendi
// çekirdeğimiz kullanılır). Adresler cihaz adresleridir (float32 bitişik).
func (g *arkaUc) ltCarp(batch, M, N, K int, a uint64, ta bool, b uint64, tb bool, c uint64, uzerine bool) bool {
	l := g.lt
	if l == nil || K == 0 {
		return false
	}
	p := l.planAl(ltAnahtar{batch, M, N, K, ta, tb, uzerine})
	if !p.gecerli {
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
		panic(fmt.Sprintf("tensor: GPU: cuBLASLt matris çarpımı başarısız (durum %d; %d×%d×%d, toplu %d)", s, M, N, K, batch))
	}
	g.sonCekirdek = l.adlar[ad]
	if g.eszamanli {
		g.esitle()
	}
	return true
}

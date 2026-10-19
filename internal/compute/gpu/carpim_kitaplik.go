package gpu

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Matris çarpımı kitaplığı (NVIDIA cuBLASLt; makinede varsa): yükleme ve kurulum
// ============================================================================
//
// float32 matris çarpımı makinede kuruluysa üreticinin kitaplığıyla yapılır: kitaplık
// sürücü gibi çalışma anında yüklenir (dağıtılmaz; arama yerleri carpim_kitaplik_yol.go'da).
// Bulunamazsa, sürümü kartın mimarisi için eskiyse, kurulamazsa, bir şekil için algoritma
// vermezse ya da bir çağrı hata dönerse aynı cihazda kendi çekirdeğimiz (cekirdek_carpim.go)
// kullanılır: sonuç aynıdır, yalnızca hız farklıdır; düşüşler sayılır ve durumda görünür.
// TR_GPU_KENDI_CARPIM=1 kendi çekirdeğimizi zorlar (sınama ve karşılaştırma).
//
// Aday seçimi: açıkça verilen yol (TR_GPU_CUBLASLT) önce; öbür adayların hepsi açılır ve
// sürümleri (cublasLtGetVersion, sayısal) büyükten küçüğe denenir; kartın mimarisinin istediği
// en düşük sürümden eskiler atlanır; cublasLtCreate başarılı olan ilk aday kullanılır (ör.
// .so.13 sürücüyle kurulamazsa .so.12 denenir).
//
// Hesap float32'dir (TF32 kapalı: CUBLAS_COMPUTE_32F). Çağrılar ve plan önbelleği
// carpim_kitaplik_plan.go'da.

// ltIslevAdlari yüklenen kitaplık işlevleri.
var ltIslevAdlari = [...]string{
	"cublasLtGetVersion", "cublasLtCreate", "cublasLtMatmulDescCreate", "cublasLtMatmulDescSetAttribute",
	"cublasLtMatrixLayoutCreate", "cublasLtMatrixLayoutSetAttribute", "cublasLtMatmulPreferenceCreate",
	"cublasLtMatmulPreferenceSetAttribute", "cublasLtMatmulAlgoGetHeuristic", "cublasLtMatmul",
	"cublasLtMatmulDescDestroy", "cublasLtMatrixLayoutDestroy", "cublasLtMatmulPreferenceDestroy", "cublasLtDestroy",
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
	ltTanimYoket
	ltDuzenYoket
	ltTercihYoket
	ltYoket
	ltSayisi
)

// ltKitaplik açılmış bir kitaplık adayı.
type ltKitaplik struct {
	k     kutuphane
	fn    [ltSayisi]uintptr
	yol   string
	surum int
}

// ltAdayListesi süreç genelinde bir kez açılan adaylar (sıralı) ve açılamayanların notları.
var ltAdayListesi struct {
	once    sync.Once
	adaylar []*ltKitaplik
	notlar  []string
}

// ltAc bir adayı açar ve işlevlerini bulur.
func ltAc(yol string) (*ltKitaplik, error) {
	k, err := kitaplikAc(yol)
	if err != nil {
		return nil, err
	}
	l := &ltKitaplik{k: k, yol: yol}
	for i, ad := range ltIslevAdlari {
		if l.fn[i], err = k.bul(ad); err != nil {
			return nil, fmt.Errorf("%s yok", ad)
		}
	}
	l.surum = int(k.cagir(l.fn[ltSurum]))
	return l, nil
}

// ltAdaylariAc adayları açar: açıkça verilen önce, öbürleri sürüme göre büyükten küçüğe.
func ltAdaylariAc() ([]*ltKitaplik, []string) {
	a := &ltAdayListesi
	a.once.Do(func() {
		acik, yollar, err := carpimKitaplikAdaylari()
		if err != nil {
			a.notlar = append(a.notlar, err.Error())
		}
		if acik != "" {
			if l, err := ltAc(acik); err == nil {
				a.adaylar = append(a.adaylar, l)
			} else {
				a.notlar = append(a.notlar, fmt.Sprintf("TR_GPU_CUBLASLT (%s) açılamadı: %v", acik, err))
			}
		}
		var digerleri []*ltKitaplik
		for _, y := range yollar {
			if l, err := ltAc(y); err == nil {
				digerleri = append(digerleri, l)
			}
		}
		sort.SliceStable(digerleri, func(i, j int) bool { return digerleri[i].surum > digerleri[j].surum })
		a.adaylar = append(a.adaylar, digerleri...)
		if len(a.adaylar) == 0 {
			a.notlar = append(a.notlar, "cuBLASLt bulunamadı")
		}
	})
	return a.adaylar, a.notlar
}

// ltEnAzSurumu hesaplama yeteneği için gereken en düşük cuBLAS sürümü (major·10000 +
// minor·100). Kaynak: CUDA araç takımı sürüm notları — sm_100/sm_120 (Blackwell) desteği
// CUDA 12.8'de, sm_90 (Hopper) ve sm_89 (Ada) desteği CUDA 11.8'de, sm_86 CUDA 11.1'de
// geldi; daha eskiler için 11.0.
func ltEnAzSurumu(cc int) int {
	switch {
	case cc >= 100:
		return 120800
	case cc >= 89:
		return 110800
	case cc >= 86:
		return 110100
	}
	return 110000
}

func surumYazi(v int) string { return fmt.Sprintf("%d.%d.%d", v/10000, v/100%100, v%100) }

// ltKur bu GPU için kitaplık bağlantısını kurar (kurulumda, bir kez). Kurulamazsa g.lt nil
// kalır ve kendi çekirdeğimiz kullanılır; nedeni carpimYolu'nda görünür.
func (g *arkaUc) ltKur() {
	if os.Getenv("TR_GPU_KENDI_CARPIM") == "1" {
		g.ltNeden = "kendi çekirdeği (TR_GPU_KENDI_CARPIM=1)"
		return
	}
	adaylar, ortak := ltAdaylariAc()
	notlar := append([]string(nil), ortak...) // süreç genelindeki listeyi değiştirmeden
	cc, _ := g.sr.yetenek()
	enAz := ltEnAzSurumu(cc)
	g.gir()
	defer g.cik()
	for _, kit := range adaylar {
		if kit.surum < enAz {
			notlar = append(notlar, fmt.Sprintf("%s sürüm %s bu kart (sm_%d) için eski (en az %s)", kit.yol,
				surumYazi(kit.surum), cc, surumYazi(enAz)))
			continue
		}
		l := &ltCarpim{kit: kit, o: &ltCikti{}, plan: map[ltAnahtar]*ltPlan{}}
		if s := l.cagir(ltOlustur, uintptr(unsafe.Pointer(&l.o.tutamac))); s != 0 {
			notlar = append(notlar, fmt.Sprintf("%s: cublasLtCreate durum %d", kit.yol, s))
			continue
		}
		l.h = l.o.tutamac
		l.alan = g.ayir(ltCalismaAlani/4, tensor.Float32, false)
		for i, ad := range []string{"nn", "nt", "tn", "tt"} {
			l.adlar[i] = "cublaslt_carpim_" + ad
		}
		g.lt, g.ltNotlar = l, notlar
		return
	}
	g.ltNeden = "kendi çekirdeği (" + strings.Join(notlar, "; ") + ")"
}

// carpimYolu matris çarpımının hangi yoldan yapıldığı (teşhis ve profil).
func (g *arkaUc) carpimYolu() string {
	if g.lt != nil {
		y := fmt.Sprintf("cuBLASLt %s (%s)", surumYazi(g.lt.kit.surum), g.lt.kit.yol)
		if n := g.lt.dusus; n > 0 {
			y += fmt.Sprintf("; kendi çekirdeğe düşen çağrı %d", n)
			if g.ltSonHata != "" {
				y += " (son: " + g.ltSonHata + ")"
			}
		}
		if len(g.ltNotlar) > 0 {
			y += "; atlanan: " + strings.Join(g.ltNotlar, "; ")
		}
		return y
	}
	if g.ltNeden != "" {
		return g.ltNeden
	}
	return "kendi çekirdeği"
}

// ltKapat plan tanımlarını ve kitaplık tutamacını yok eder (program sonu).
func (g *arkaUc) ltKapat() {
	l := g.lt
	if l == nil {
		return
	}
	for a, p := range l.plan {
		l.planYoket(p)
		delete(l.plan, a)
	}
	l.sira = nil
	l.cagir(ltYoket, l.h)
	g.lt = nil
	g.ltNeden = "kapatıldı"
}

func (l *ltCarpim) cagir(f int, args ...uintptr) int32 {
	r := l.kit.k.cagir(l.kit.fn[f], args...)
	runtime.KeepAlive(l.o)
	return int32(uint32(r))
}

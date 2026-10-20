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
	"github.com/sekerahmet/TCB/internal/lang/messages"
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

// ltFuncNames yüklenen kitaplık işlevleri.
var ltFuncNames = [...]string{
	"cublasLtGetVersion", "cublasLtCreate", "cublasLtMatmulDescCreate", "cublasLtMatmulDescSetAttribute",
	"cublasLtMatrixLayoutCreate", "cublasLtMatrixLayoutSetAttribute", "cublasLtMatmulPreferenceCreate",
	"cublasLtMatmulPreferenceSetAttribute", "cublasLtMatmulAlgoGetHeuristic", "cublasLtMatmul",
	"cublasLtMatmulDescDestroy", "cublasLtMatrixLayoutDestroy", "cublasLtMatmulPreferenceDestroy", "cublasLtDestroy",
}

const (
	ltVersion = iota
	ltCreate
	ltDescCreate
	ltDescSet
	ltLayoutCreate
	ltLayoutSet
	ltPrefCreate
	ltPrefSet
	ltHeuristic
	ltMul
	ltDescDestroy
	ltLayoutDestroy
	ltPrefDestroy
	ltDestroy
	ltCount
)

// ltLibrary açılmış bir kitaplık adayı.
type ltLibrary struct {
	k       library
	fn      [ltCount]uintptr
	path    string
	version int
}

// ltCandidateList süreç genelinde bir kez açılan adaylar (sıralı) ve açılamayanların notları.
var ltCandidateList struct {
	before     sync.Once
	candidates []*ltLibrary
	notes      []string
}

// ltOpen bir adayı açar ve işlevlerini bulur.
func ltOpen(path string) (*ltLibrary, error) {
	k, err := openLibrary(path)
	if err != nil {
		return nil, err
	}
	l := &ltLibrary{k: k, path: path}
	for i, ad := range ltFuncNames {
		if l.fn[i], err = k.find(ad); err != nil {
			return nil, messages.New(messages.H4031, ad)
		}
	}
	l.version = int(k.call(l.fn[ltVersion]))
	return l, nil
}

// ltOpenCandidates adayları açar: açıkça verilen önce, öbürleri sürüme göre büyükten küçüğe.
func ltOpenCandidates() ([]*ltLibrary, []string) {
	a := &ltCandidateList
	a.before.Do(func() {
		open, paths, err := gemmLibCandidates()
		if err != nil {
			a.notes = append(a.notes, err.Error())
		}
		if open != "" {
			if l, err := ltOpen(open); err == nil {
				a.candidates = append(a.candidates, l)
			} else {
				a.notes = append(a.notes, messages.Text(messages.H4027, open, err))
			}
		}
		var others []*ltLibrary
		for _, y := range paths {
			if l, err := ltOpen(y); err == nil {
				others = append(others, l)
			}
		}
		sort.SliceStable(others, func(i, j int) bool { return others[i].version > others[j].version })
		a.candidates = append(a.candidates, others...)
		if len(a.candidates) == 0 {
			a.notes = append(a.notes, messages.Text(messages.H4028))
		}
	})
	return a.candidates, a.notes
}

// ltMinVersion hesaplama yeteneği için gereken en düşük cuBLAS sürümü (major·10000 +
// minor·100). Kaynak: CUDA araç takımı sürüm notları — sm_100/sm_120 (Blackwell) desteği
// CUDA 12.8'de, sm_90 (Hopper) ve sm_89 (Ada) desteği CUDA 11.8'de, sm_86 CUDA 11.1'de
// geldi; daha eskiler için 11.0.
func ltMinVersion(cc int) int {
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

func versionText(v int) string { return fmt.Sprintf("%d.%d.%d", v/10000, v/100%100, v%100) }

// ltSetup bu GPU için kitaplık bağlantısını kurar (kurulumda, bir kez). Kurulamazsa g.lt nil
// kalır ve kendi çekirdeğimiz kullanılır; nedeni carpimYolu'nda görünür.
func (g *backend) ltSetup() {
	if os.Getenv("TR_GPU_KENDI_CARPIM") == "1" {
		g.ltReason = messages.Text(messages.H4037, "TR_GPU_KENDI_CARPIM=1")
		return
	}
	candidates, shared := ltOpenCandidates()
	notes := append([]string(nil), shared...) // süreç genelindeki listeyi değiştirmeden
	cc, _ := g.sr.capability()
	minN := ltMinVersion(cc)
	g.enter()
	defer g.exit()
	for _, kitLib := range candidates {
		if kitLib.version < minN {
			notes = append(notes, messages.Text(messages.H4029, kitLib.path,
				versionText(kitLib.version), cc, versionText(minN)))
			continue
		}
		l := &ltGemm{kitLib: kitLib, o: &ltOutput{}, plan: map[ltKey]*ltPlanFor{}}
		if s := l.call(ltCreate, uintptr(unsafe.Pointer(&l.o.handle))); s != 0 {
			notes = append(notes, messages.Text(messages.H4030, kitLib.path, s))
			continue
		}
		l.h = l.o.handle
		l.space = g.allocate(ltWorkspace/4, tensor.Float32, false)
		for i, ad := range []string{"nn", "nt", "tn", "tt"} {
			l.names[i] = "cublaslt_carpim_" + ad
		}
		g.lt, g.ltNotes = l, notes
		return
	}
	g.ltReason = messages.Text(messages.H4037, strings.Join(notes, "; "))
}

// gemmPath matris çarpımının hangi yoldan yapıldığı (teşhis ve profil).
func (g *backend) gemmPath() string {
	if g.lt != nil {
		// Her madde kendi başına tam bir iletidir; başka iletiye parça olmaz, yalnızca liste olarak
		// yan yana durur.
		items := []string{messages.Text(messages.H4032, versionText(g.lt.kitLib.version), g.lt.kitLib.path)}
		if n := g.lt.fallthroughCase; n > 0 {
			items = append(items, messages.Text(messages.H4033, n))
			if g.ltLastError != "" {
				items = append(items, messages.Text(messages.H4034, g.ltLastError))
			}
		}
		if len(g.ltNotes) > 0 {
			items = append(items, messages.Text(messages.H4035, strings.Join(g.ltNotes, "; ")))
		}
		return strings.Join(items, "; ")
	}
	if g.ltReason != "" {
		return g.ltReason
	}
	return messages.Text(messages.H4036)
}

// ltClose plan tanımlarını ve kitaplık tutamacını yok eder (program sonu).
func (g *backend) ltClose() {
	l := g.lt
	if l == nil {
		return
	}
	for a, p := range l.plan {
		l.planDestroy(p)
		delete(l.plan, a)
	}
	l.order = nil
	l.call(ltDestroy, l.h)
	g.lt = nil
	g.ltReason = messages.Text(messages.H4063)
}

func (l *ltGemm) call(f int, args ...uintptr) int32 {
	r := l.kitLib.k.call(l.kitLib.fn[f], args...)
	runtime.KeepAlive(l.o)
	return int32(uint32(r))
}

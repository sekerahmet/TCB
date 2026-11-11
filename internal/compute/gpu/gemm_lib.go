package gpu

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
// Öntanımlı matris çarpımı TR'nin kendi çekirdeğidir (kernel_gemm.go, kernel_gemv.go); sonuç
// kurulu bir kitaplığa bağlı olmaz (15.22a). Üreticinin kitaplığı yalnız TR_vendor_matmul
// bloğundaki çarpımlarda kullanılır (15.53b): kitaplık sürücü gibi çalışma anında, bloğun
// kitaplığa gidecek ilk kart çarpımında yüklenir (ltEnsure; dağıtılmaz; arama yerleri
// gemm_lib_path.go'da). Bulunamazsa, sürümü kartın mimarisi için eskiyse ya da kurulamazsa
// H4130, bir şekil için algoritma vermezse ya da bir çağrı hata dönerse H4131 süreçte bir kez
// verilir ve aynı cihazda kendi çekirdeğimiz kullanılır; düşüşler sayılır ve durumda görünür.
// Kitaplığın sonucu kendi çekirdeğimizinkinden farklı bitler verebilir (ölçü: L4, 17 şeklin
// 13'ünde).
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

// ltEnsure kitaplık bağlantısını bu arka uçta bir kez kurar (kilit tutulurken; bloğun kitaplığa
// gidecek ilk çarpımında). Aynı anda gelen görevler kilitle sıralanır, kurulum bir kez yapılır.
// Kayıt sürerken (TR_replay) çağrılırsa çalışma alanı kaydın havuzundan değil ana havuzdan alınır:
// kitaplık kayıttan sonra da yaşar.
func (g *backend) ltEnsure() {
	g.ltOnce.Do(func() {
		g.ltSetups++
		priv := g.capturing
		g.capturing = nil // çalışma alanı kaydın havuzuna girmez
		defer func() { g.capturing = priv }()
		if g.ltSetupHook != nil {
			g.ltSetupHook()
			return
		}
		g.ltSetup()
	})
}

// ltSetup bu GPU için kitaplık bağlantısını kurar (kilit tutulurken; ltEnsure'dan bir kez).
// Kurulamazsa g.lt nil kalır, H4130 bir kez verilir ve kendi çekirdeğimiz kullanılır; nedeni
// gemmPath'te görünür. Benzeticide kitaplık hiç yüklenmez (benzeticinin adresleri kartın değildir).
func (g *backend) ltSetup() {
	var candidates []*ltLibrary
	notes := []string{messages.Text(messages.H4028)}
	reason := messages.New(messages.H4028) // H4130'un nedeni: son reddedilen adayınki (hepsi durum satırında)
	if _, sim := g.sr.(*simDriver); !sim {
		var shared []string
		candidates, shared = ltOpenCandidates()
		notes = append([]string(nil), shared...) // süreç genelindeki listeyi değiştirmeden
	}
	cc, _ := g.sr.capability()
	minN := ltMinVersion(cc)
	for _, kitLib := range candidates {
		if kitLib.version < minN {
			notes = append(notes, messages.Text(messages.H4029, kitLib.path,
				versionText(kitLib.version), cc, versionText(minN)))
			reason = messages.New(messages.H4029, kitLib.path, versionText(kitLib.version), cc, versionText(minN))
			continue
		}
		l := &ltGemm{kitLib: kitLib, o: &ltOutput{}, plan: map[ltKey]*ltPlanFor{}}
		if s := l.call(ltCreate, uintptr(unsafe.Pointer(&l.o.handle))); s != 0 {
			notes = append(notes, messages.Text(messages.H4030, kitLib.path, s))
			reason = messages.New(messages.H4030, kitLib.path, s)
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
	warnOnce(messages.H4130, reason, exampleFor(runtime.GOOS))
}

// vendorMul TR_vendor_matmul bloğundaki çarpımın kitaplık girişidir (15.53b): kitaplık gerekirse
// kurulur, çarpım ona verilir. Kitaplık yoksa ya da çağrıyı yapamazsa yanlış döner (çağıran
// kendi çekirdekle sürer); bir çağrıdaki ilk düşüş H4131'i süreçte bir kez verir.
func (g *backend) vendorMul(batch, M, N, K int, a uint64, ta bool, b uint64, tb bool, c uint64, over bool, ab, ct uintptr) bool {
	if K == 0 {
		return false
	}
	g.vendorCalls++
	if g.vendorHook == nil {
		g.ltEnsure()
		if g.lt == nil {
			return false
		}
		if g.ltMul(batch, M, N, K, a, ta, b, tb, c, over, ab, ct) {
			return true
		}
	} else if g.vendorHook(batch, M, N, K) {
		return true
	} else {
		g.ltLastErr = messages.New(messages.H4136)
	}
	shape := fmt.Sprintf("[%d, %d] @ [%d, %d]", M, K, K, N)
	if batch > 1 {
		shape = fmt.Sprintf("[%d, %d, %d] @ [%d, %d, %d]", batch, M, K, batch, K, N)
	}
	warnOnce(messages.H4131, shape, g.ltLastErr)
	return false
}

// exampleFor H4130'un önerisidir: kitaplığın tam yolunu veren komut, işletim sisteminin kabuğunda
// (Windows'ta PowerShell). Dilden bağımsızdır; yer tutucular "...".
func exampleFor(goos string) string {
	if goos == "windows" {
		return `$env:TR_GPU_CUBLASLT="C:\...\cublasLt64_12.dll"; trlang ...`
	}
	return "export TR_GPU_CUBLASLT=/.../libcublasLt.so.12; trlang ..."
}

// gemmLibPresent kitaplığı yüklemeden bir adayın diskte olup olmadığına bakar (blok dışı: H4132
// ve durum satırı). Yalnız mutlak adaylar ve TR_GPU_CUBLASLT'nin dosyası (os.Stat); Linux'ta
// standart arama yolunun dosya adıyla verilen adayları sayılmaz (onu ancak dlopen bilir).
func gemmLibPresent() (string, bool) {
	open, candidates, err := gemmLibCandidates()
	if err == nil && open != "" {
		if _, err := os.Stat(open); err == nil {
			return open, true
		}
	}
	for _, p := range candidates {
		if filepath.IsAbs(p) {
			return p, true
		}
	}
	return "", false
}

// warnedCodes süreçte verilmiş uyarı kodlarıdır (her kod bir kez); warnOut uyarıların yazıldığı
// yerdir (sınama kancası).
var (
	warnedCodes sync.Map
	warnOut     io.Writer = os.Stderr
)

// warnOnce kodlu uyarıyı süreçte bir kez yazar ("süreçte bir kez" her kod için ayrı sayılır).
func warnOnce(code messages.Code, args ...any) {
	if _, loaded := warnedCodes.LoadOrStore(code, true); loaded {
		return
	}
	fmt.Fprintln(warnOut, messages.Text(code, args...))
}

// warned kodun bu süreçte verilip verilmediğidir.
func warned(code messages.Code) bool {
	_, ok := warnedCodes.Load(code)
	return ok
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
	if path, ok := gemmLibPresent(); ok {
		return messages.Text(messages.H4133, path)
	}
	return messages.Text(messages.H4135)
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

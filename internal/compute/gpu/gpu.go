// Package gpu, TR tensör çekirdeğinin NVIDIA GPU arka ucudur: tensor.Backend arayüzünü
// sürücü API'si üzerinden uygular. Çekirdekler Go içinden PTX olarak üretilir
// (ptx*.go, cekirdek_*.go) ve sürücünün içindeki derleyiciyle ilk kullanımda karta
// derlenir; derlenen modüller süreç boyunca önbellekte tutulur. cgo ve CUDA araç takımı
// gerekmez: sürücü kitaplığı çalışma anında yüklenir (cuda*.go). Sürücü bulunamazsa
// paket hiçbir şey kaydetmez ve tensörler CPU'da çalışır.
//
// Donanımsız sınama için sürücünün yerine geçen bir PTX benzetimi vardır (benzetim*.go):
// arka ucun bütün çekirdekleri bu makinede de yürütülüp CPU arka ucuyla karşılaştırılır.
//
// Layer: compute core (internal/compute); yalnızca internal/compute paketlerini içe aktarır.
package gpu

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// backend bir GPU'nun tensor.Backend uygulaması.
type backend struct {
	dev tensor.Device
	sr  driver
	// mu bütün sürücü çağrılarını sıraya koyar (tek akış). Dışa açık yöntemler girişte
	// alır (gir); iç yardımcılar kilidin tutulduğunu varsayar.
	mu         sync.Mutex
	kernels    map[string]*kernel
	pool       pool
	setup      setup
	prm        paramWriter
	lastKernel string
	// concurrent her başlatmadan sonra eşitler (teşhis: hatalı çekirdeği tam yerinde
	// yakalar; TR_GPU_ESZAMANLI=1).
	concurrent bool
	gemm2      gemmConfig
	// gemmConst doluysa matris çarpımı döşemesi otomatik seçilmez (sınama).
	gemmConst *gemmConfig
	// gemvOff tek satırlık çarpım çekirdeğini kapatır (TR_GPU_GEMV=0; karşılaştırma): M = 1
	// çarpımları da kitaplığa ya da döşemeli çekirdeğe gider. gemvConst doluysa tek satırlık
	// çarpımın ayarı otomatik seçilmez (sınama ve ölçüm).
	gemvOff   bool
	gemvConst *gemvConfig
	// profiler çekirdek ölçümleri (bkz. profile.go).
	profiler profile
	// volume son çöp toplamadan bu yana verilen depolama baytı (havuzdan gelenler dahil).
	volume uint64
	// lt matris çarpımı kitaplığı (yüklenemediyse nil: kendi çekirdeğimiz).
	lt       *ltGemm
	ltReason string // lt nil ise nedeni (teşhis)
	// ltNotes kurulumda atlanan kitaplık adayları; ltSonHata kitaplığın son reddettiği çağrı.
	ltNotes     []string
	ltLastError string
	// attnPath TR_GPU_DIKKAT: boş (varsayılan: 16 bitlik girdilerde sm_80+ tensör çekirdekli,
	// öbürlerinde SIMT birleşik yol), "birlesik" (SIMT birleşik yolu zorlar), "carpim" (matris
	// çarpımı yolu) ya da "tensor" (tensör çekirdekli yolu sm denetimi olmadan zorlar; benzetimde
	// sınama için). Ölçüm ve karşılaştırma içindir.
	attnPath string
	// gcThreshold son çöp toplamadan bu yana bu kadar depolama verilince çöp toplama başlatılır
	// (bkz. copEsigiSec; TR_GPU_COP_ESIGI ile MB olarak değiştirilir).
	gcThreshold uint64
}

func init() {
	tensor.RegisterLazyBackend(tensor.KindGPU, tensor.LazyBackend{Count: deviceCount, New: setupFn})
}

// setupError son başarısız kurulumun nedeni (teşhis için; farklı gorutinlerden
// yazılıp okunabildiği için kilitli).
var setupError struct {
	mu  sync.Mutex
	err error
}

func writeSetupError(err error) {
	setupError.mu.Lock()
	setupError.err = err
	setupError.mu.Unlock()
}

func readSetupError() error {
	setupError.mu.Lock()
	defer setupError.mu.Unlock()
	return setupError.err
}

// deviceCount sürücünün gördüğü GPU sayısı (sürücü yoksa 0; geliştirme için
// TR_GPU_BENZETIM=1 ile benzetim tek GPU sayılır).
func deviceCount() int {
	c := loadCUDA()
	if c.failure != nil {
		writeSetupError(c.failure)
		if simRequested() {
			return 1
		}
		return 0
	}
	return c.num
}

// simRequested gerçek GPU yokken PTX benzetiminin "gpu" cihazı olarak kurulmasının
// istenip istenmediği (geliştirme ve sınama aracı: GPU kodunu donanımsız, dil düzeyinde
// uçtan uca çalıştırır; çok yavaştır).
func simRequested() bool { return os.Getenv("TR_GPU_BENZETIM") == "1" }

func setupFn(order int) tensor.Backend {
	if loadCUDA().failure != nil && simRequested() && order == 0 {
		return save(newBackend(tensor.GPU(0), newSim(1<<30)))
	}
	s, err := openCUDA(order)
	if err != nil {
		writeSetupError(err)
		return nil
	}
	g := save(newBackend(tensor.GPU(order), s))
	g.ltSetup()
	return g
}

// Status sürücünün yüklenme durumunu açıklar (teşhis: GPU neden görünmüyor).
func Status() string {
	n := deviceCount()
	if n == 0 {
		if err := readSetupError(); err != nil {
			return messages.Text(messages.H4021, err)
		}
		return messages.Text(messages.H4022)
	}
	if !tensor.HasDevice(tensor.GPU(0)) {
		return messages.Text(messages.H4023, n, readSetupError())
	}
	be := tensor.BackendFor(tensor.GPU(0)).(*backend)
	return messages.Text(messages.H4024, n, be.sr.cardName(), bytesText(be.sr.totalMemory()), be.gemmPath())
}

func newBackend(dev tensor.Device, sr driver) *backend {
	g := &backend{dev: dev, sr: sr, kernels: map[string]*kernel{}}
	g.pool.empty = map[uint64][]uint64{}
	g.concurrent = os.Getenv("TR_GPU_ESZAMANLI") == "1"
	g.attnPath = os.Getenv("TR_GPU_DIKKAT")
	g.gemm2 = defaultGemm
	g.gemvOff = os.Getenv("TR_GPU_GEMV") == "0"
	g.gcThreshold = pickGCThreshold(sr.totalMemory(), os.Getenv("TR_GPU_COP_ESIGI"))
	return g
}

// pickGCThreshold çöp toplama eşiği: kart belleğinin 1/64'ü, 256 MB ile 2 GB arasında (95 GB'ta
// ~1,5 GB, 24 GB'ta 384 MB). Ara tensörlerin cihaz belleği ancak Go çöp toplayıcısı
// çalışınca havuza döner; eşik tepe belleği bir adımın canlı tensörlerine yakın tutar.
// Ölçüm (RTX PRO 6000, gpt30m): eşiksiz 38,9 GB, 2 GB'ta 23,5 GB, 512 MB'ta 21,1 GB tepe;
// hız farkı %0,6. ortam (TR_GPU_COP_ESIGI, MB) verilirse o kullanılır; "0" eşiği kapatır.
func pickGCThreshold(total uint64, env string) uint64 {
	if mb, err := strconv.Atoi(env); err == nil && mb >= 0 {
		return uint64(mb) << 20
	}
	return min(max(total/64, 256<<20), 2<<30)
}

// RegisterSimulator, gerçek bir GPU yoksa PTX benzetimini "gpu" cihazı olarak kaydeder
// (yalnızca sınamalar için: GPU kodunun dil düzeyinde uçtan uca sınanması). Benzetim
// yavaştır; küçük şekiller kullanın. Kaydedildiyse doğru döner.
func RegisterSimulator() bool {
	if tensor.HasDevice(tensor.GPU(0)) {
		return false
	}
	tensor.RegisterBackend(save(newBackend(tensor.GPU(0), newSim(1<<30))))
	return true
}

func (g *backend) Device() tensor.Device { return g.dev }

// MemoryLimit (tensor.MemoryLimiter) tek ayırmanın sınırı: cihazın toplam belleği.
func (g *backend) MemoryLimit() int64 { return int64(g.sr.totalMemory()) }

// MemoryPlace hata mesajlarında anılan yerin anahtarıdır.
func (g *backend) MemoryPlace() string { return tensor.PlaceGPU }

// enter kilidi alır, gorutini iş parçacığına kilitler ve bağlamı bağlar.
func (g *backend) enter() {
	g.mu.Lock()
	if gpuClosed.Load() {
		g.mu.Unlock()
		panic(messages.New(messages.H4017))
	}
	g.prm.open = false // panikle yarıda kalmış bir önceki zincir
	runtime.LockOSThread()
	if s := g.sr.bind(); s != succeeded {
		err := g.driverError("cuCtxSetCurrent", s)
		g.exit()
		panic(err)
	}
	g.hostStart()
}

func (g *backend) exit() {
	g.hostEnd()
	runtime.UnlockOSThread()
	g.mu.Unlock()
}

// driverError sürücü hatasının açıklaması: kodun adı, son çekirdek, ayrıntı.
func (g *backend) driverError(op string, s result) error {
	name, code := g.sr.errName(s), int32(s)
	var err error
	switch {
	case g.lastKernel == "":
		err = messages.New(messages.H4007, op, name, code)
	case g.concurrent:
		err = messages.New(messages.H4008, op, name, code, g.lastKernel)
	default:
		err = messages.New(messages.H4009, op, name, code, g.lastKernel)
	}
	if d := g.sr.detail(); d != nil {
		err = messages.New(messages.H4010, err, d)
	}
	return err
}

// try_ sürücü dönüşünü denetler; hata paniğe çevrilir (dil 'dene/yakala' ile
// yakalayabilir).
func (g *backend) try_(op string, s result) {
	if s != succeeded {
		panic(g.driverError(op, s))
	}
}

// equalize sıradaki bütün işlerin bitmesini bekler.
func (g *backend) equalize() {
	g.equalBegin()
	g.try_("cuCtxSynchronize", g.sr.equalize())
	g.setup.offset = 0
	g.equalFinish()
}

// maxN tek bir çekirdeğin eleman sayısı sınırı (32 bitlik indeksler).
const maxN = 1<<31 - 1

// checkSize n elemanın tek çekirdekte işlenebildiğini denetler.
func checkSize(n int) {
	if n > maxN {
		panic(messages.New(messages.H4016, n))
	}
}

// none GPU'da henüz olmayan bir işlem için açık hata verir (sessizce CPU'ya düşülmez); kod
// işlemi anlatan H40xx iletisidir.
func none(code messages.Code, args ...any) {
	panic(messages.New(code, args...))
}

func bytesText(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	}
	return messages.Text(messages.H4020, b)
}

// Compile-time checks: the tensor package finds the optional fast paths of a backend by type
// assertion, so a renamed or changed method would otherwise make it fall back to the slow
// path silently.
var (
	_ tensor.Backend                = (*backend)(nil)
	_ tensor.Releaser               = (*backend)(nil)
	_ tensor.MemoryLimiter          = (*backend)(nil)
	_ tensor.UninitAllocator        = (*backend)(nil)
	_ tensor.SquareSummer           = (*backend)(nil)
	_ tensor.BackwardFinisher       = (*backend)(nil)
	_ tensor.MultiTensorOps         = (*backend)(nil)
	_ tensor.GemmSetter             = (*backend)(nil)
	_ tensor.AttentionGradSetter    = (*backend)(nil)
	_ tensor.CrossEntropyGradSetter = (*backend)(nil)
	_ tensor.UnaryGradSetter        = (*backend)(nil)
	_ tensor.GradClipper            = (*backend)(nil)
	_ tensor.Storage                = (*storage)(nil)
)

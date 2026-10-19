package tensor

import (
	"fmt"
	"sort"
	"sync"
)

// ============================================================================
// Arka uç kaydı, tembel kurulum ve isteğe bağlı arka uç arayüzleri
// ============================================================================

var (
	backendMu sync.RWMutex
	backends  = map[Device]Backend{}
	// defaultDevice oluşturucuların (Zeros, New, ...) kullandığı cihaz.
	defaultDevice = CPU

	// isLazy tembel kurulumları sıraya koyar (kurulum backendMu tutulmadan yapılır).
	isLazy sync.Mutex
	lazies = map[DeviceKind]LazyBackend{}
	// tried kurulumu denenmiş (başarılı ya da başarısız) cihazlar.
	tried = map[Device]bool{}
)

func init() {
	RegisterBackend(cpuBackend{})
}

// RegisterBackend bir cihazın arka ucunu kaydeder.
func RegisterBackend(b Backend) {
	backendMu.Lock()
	backends[b.Device()] = b
	backendMu.Unlock()
}

// LazyBackend bir cihaz türünün ilk gerektiğinde kurulan arka ucudur. Kurulum pahalı
// olabilir (ör. GPU sürücüsünün yüklenmesi) ve çoğu programda hiç gerekmez; bu yüzden
// cihaz ancak sorulduğunda (BackendFor, HasDevice, Devices) kurulur.
type LazyBackend struct {
	// Count kullanılabilir cihaz sayısı (sürücü yoksa 0).
	Count func() int
	// New verilen sıradaki cihazın arka ucunu kurar; kurulamazsa nil.
	New func(index int) Backend
}

// RegisterLazyBackend bir cihaz türünün tembel arka ucunu kaydeder (ör. GPU arka ucu
// paketinin init işlevinden). Aynı cihaz için daha önce yapılmış başarısız denemeler
// unutulur.
func RegisterLazyBackend(kind DeviceKind, l LazyBackend) {
	isLazy.Lock()
	lazies[kind] = l
	for d := range tried {
		if d.Kind == kind {
			delete(tried, d)
		}
	}
	isLazy.Unlock()
}

// findBackend cihazın arka ucunu verir; kayıtlı değilse tembel kurulumu bir kez dener.
func findBackend(d Device) (Backend, bool) {
	backendMu.RLock()
	b, ok := backends[d]
	backendMu.RUnlock()
	if ok {
		return b, true
	}
	isLazy.Lock()
	defer isLazy.Unlock()
	backendMu.RLock()
	b, ok = backends[d]
	backendMu.RUnlock()
	if ok || tried[d] {
		return b, ok
	}
	tried[d] = true
	l, registered := lazies[d.Kind]
	if !registered || d.Index < 0 || d.Index >= l.Count() {
		return nil, false
	}
	if b = l.New(d.Index); b == nil {
		return nil, false
	}
	RegisterBackend(b)
	return b, true
}

// BackendFor cihazın arka ucunu verir; kayıtlı değilse panik.
func BackendFor(d Device) Backend {
	b, ok := findBackend(d)
	if !ok {
		panic(fmt.Sprintf("tensor: %s cihazı için arka uç kayıtlı değil", d))
	}
	return b
}

// HasDevice cihazın arka ucu kayıtlı (ya da kurulabilir) mi.
func HasDevice(d Device) bool {
	_, ok := findBackend(d)
	return ok
}

// Devices kullanılabilir cihazları verir: önce CPU, sonra tür ve sıra düzeninde
// diğerleri (tembel arka uçlar bu çağrıda kurulur).
func Devices() []Device {
	isLazy.Lock()
	var candidates []Device
	for kind, l := range lazies {
		for i := 0; i < l.Count(); i++ {
			candidates = append(candidates, Device{Kind: kind, Index: i})
		}
	}
	isLazy.Unlock()
	for _, d := range candidates {
		findBackend(d)
	}
	backendMu.RLock()
	out := make([]Device, 0, len(backends))
	for d := range backends {
		out = append(out, d)
	}
	backendMu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Index < out[j].Index
	})
	return out
}

// SetDefaultDevice oluşturucuların kullanacağı cihazı ayarlar.
func SetDefaultDevice(d Device) {
	BackendFor(d)
	backendMu.Lock()
	defaultDevice = d
	backendMu.Unlock()
}

// DefaultDevice oluşturucuların kullandığı cihaz.
func DefaultDevice() Device {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return defaultDevice
}

// MemoryLimiter, belleği konaktan ayrı olan arka uçların (ör. GPU) isteğe bağlı olarak
// uyguladığı arayüzdür: tek bir ayırmanın en fazla baytı ve hata mesajında anılacak yer
// (ör. "bu GPU'da"). Cihazdan bağımsız katman, şekli bilinen her ayırmayı fiziksel
// belleğe ek olarak bu sınırla da denetler (bkz. bellek_siniri.go).
type MemoryLimiter interface {
	MemoryLimit() int64
	MemoryPlace() string
}

// UninitAllocator, içeriği belirsiz (sıfırlanmamış) depolama verebilen arka uçların
// isteğe bağlı arayüzüdür; yalnızca her elemanı yazılacak çıktılarda kullanılır (bkz.
// emptyOf). Alloc'un sıfırlama sözleşmesi değişmez.
type UninitAllocator interface {
	AllocUninit(n int, dt DType) Storage
}

// SquareSummer birden çok depolamanın kareler toplamını tek seferde veren arka uçların
// isteğe bağlı arayüzüdür (GPU'da her depolama için ayrı eşzamanlama yerine tek
// eşzamanlama). Sonuç, depolamaların kareler toplamlarının sırayla toplamıdır.
type SquareSummer interface {
	SumSquaresAll(s []Storage) float64
}

// BackwardFinisher bir geri yayılım bittiğinde haber almak isteyen arka uçların isteğe bağlı
// arayüzüdür: ileri yönün ara tensörleri o anda ulaşılamaz olur (GPU arka ucu çöp
// toplamayı başlatıp cihaz belleğini bir sonraki ileri yönden önce havuza döndürür).
type BackwardFinisher interface {
	BackwardDone()
}

// MultiTensorOps iyileştirici adımlarını ve ölçeklemeyi birden çok depolamaya tek seferde
// (GPU'da tek başlatmada) uygulayan arka uçların isteğe bağlı arayüzüdür. Sonuç her depolama
// için tekli yöntemlerle (AdamWStep, SGDStep, Scale) aynıdır; dilimler eşit uzunluktadır
// (SGDStepAll'da vel nil ya da nil elemanlı olabilir: momentum yok).
type MultiTensorOps interface {
	AdamWStepAll(p, g, m, v []Storage, lr, b1, b2, eps, wd float32, bc1, bc2 []float32)
	SGDStepAll(p, g, vel []Storage, lr, momentum, wd float32)
	ScaleAll(s []Storage, k float32)
}

// GemmSetter Gemm'in C'nin üzerine yazan biçimini (C = op(A)·op(B); C'nin eski içeriği
// okunmaz, K = 0 ise C sıfır olur) veren arka uçların isteğe bağlı arayüzüdür. Çıktı ve ilk
// gradyan sıfırlanmadan ayrılır: sıfırlama ve eski içeriğin okunması (iki tam bellek geçişi)
// kalkar.
type GemmSetter interface {
	GemmSet(batch, M, N, K int, a View, transA bool, b View, transB bool, c Storage)
}

// AttentionGradSetter AttentionGrad'ın taze (ilk) gradyanların üzerine yazan biçimini veren
// arka uçların isteğe bağlı arayüzüdür: taze[i] doğruysa gq, gk, gv'nin i.'si sıfırlanmadan
// ayrılmıştır ve üzerine yazılır, değilse eklenir.
type AttentionGradSetter interface {
	AttentionGradSet(gq, gk, gv, g Storage, q, k, v View, mask Storage, o, lse Storage, p AttentionParams, fresh [3]bool)
}

// CrossEntropyGradSetter CrossEntropyGrad'ın gx'in üzerine yazan biçimini (hedefi negatif
// satırlar 0 olur) veren arka uçların isteğe bağlı arayüzüdür.
type CrossEntropyGradSetter interface {
	CrossEntropyGradSet(gx, g Storage, x View, lse Storage, targets []int, C int)
}

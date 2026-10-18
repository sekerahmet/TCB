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
	// varsayilanCihaz oluşturucuların (Zeros, New, ...) kullandığı cihaz.
	varsayilanCihaz = CPU

	// tembelMu tembel kurulumları sıraya koyar (kurulum backendMu tutulmadan yapılır).
	tembelMu  sync.Mutex
	tembeller = map[DeviceKind]LazyBackend{}
	// denenen kurulumu denenmiş (başarılı ya da başarısız) cihazlar.
	denenen = map[Device]bool{}
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
	tembelMu.Lock()
	tembeller[kind] = l
	for d := range denenen {
		if d.Kind == kind {
			delete(denenen, d)
		}
	}
	tembelMu.Unlock()
}

// arkaUcBul cihazın arka ucunu verir; kayıtlı değilse tembel kurulumu bir kez dener.
func arkaUcBul(d Device) (Backend, bool) {
	backendMu.RLock()
	b, ok := backends[d]
	backendMu.RUnlock()
	if ok {
		return b, true
	}
	tembelMu.Lock()
	defer tembelMu.Unlock()
	backendMu.RLock()
	b, ok = backends[d]
	backendMu.RUnlock()
	if ok || denenen[d] {
		return b, ok
	}
	denenen[d] = true
	l, kayitli := tembeller[d.Kind]
	if !kayitli || d.Index < 0 || d.Index >= l.Count() {
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
	b, ok := arkaUcBul(d)
	if !ok {
		panic(fmt.Sprintf("tensor: %s cihazı için arka uç kayıtlı değil", d))
	}
	return b
}

// HasDevice cihazın arka ucu kayıtlı (ya da kurulabilir) mi.
func HasDevice(d Device) bool {
	_, ok := arkaUcBul(d)
	return ok
}

// Devices kullanılabilir cihazları verir: önce CPU, sonra tür ve sıra düzeninde
// diğerleri (tembel arka uçlar bu çağrıda kurulur).
func Devices() []Device {
	tembelMu.Lock()
	var adaylar []Device
	for kind, l := range tembeller {
		for i := 0; i < l.Count(); i++ {
			adaylar = append(adaylar, Device{Kind: kind, Index: i})
		}
	}
	tembelMu.Unlock()
	for _, d := range adaylar {
		arkaUcBul(d)
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
	varsayilanCihaz = d
	backendMu.Unlock()
}

// DefaultDevice oluşturucuların kullandığı cihaz.
func DefaultDevice() Device {
	backendMu.RLock()
	defer backendMu.RUnlock()
	return varsayilanCihaz
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

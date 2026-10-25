package gpu

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// GPU profili: dışa açık Go arayüzü ve özet
// ============================================================================
//
// İzleme araçları (canlı profil) şu üç işi kullanır: ProfilAc ile ölçümü açıp kapatır,
// CekirdekOlcumleri ile şimdiye kadarki çekirdek başına toplamları (anlık görüntü; arka ucu
// beklemez, saniyede birkaç kez okunabilir) ve BellekOlcumleri ile GPU belleğinin durumunu
// alır. TR_GPU_PROFIL=1 ile çalışan bir program sonunda (Kapat) özet standart hataya yazılır.

// KernelSample bir çekirdeğin (ya da sürücü bellek işinin) toplamları.
type KernelSample struct {
	Name    string // üretilen çekirdeğin adı (ör. "carpim_128_128_8_...")
	Family  string // adın ailesi (ör. "carpim")
	Calls   int64
	GPUTime time.Duration
}

// MemorySample bir GPU'nun bellek havuzunun durumu (bayt).
type MemorySample struct {
	Device       int
	Reserved     uint64 // sürücüden ayrılmış toplam
	Pooled       uint64 // ayrılmış ama boşta (yeniden kullanılmayı bekleyen)
	Live         uint64 // tensörlere verilmiş (çöp toplayıcının henüz bırakmadıkları dahil)
	PeakLive     uint64
	PeakReserved uint64
	GCRuns       int64 // bellek için başlatılan çöp toplama sayısı
	DriverAllocs int64 // sürücüden yeni blok ayırma sayısı
}

var registered struct {
	mu sync.Mutex
	uc []*backend
}

// save arka ucu profil ve bellek ölçümlerine ekler (cihaz olarak kaydedilenler).
func save(g *backend) *backend {
	registered.mu.Lock()
	registered.uc = append(registered.uc, g)
	registered.mu.Unlock()
	return g
}

func registeredAll() []*backend {
	registered.mu.Lock()
	defer registered.mu.Unlock()
	return append([]*backend(nil), registered.uc...)
}

// allocSite GPU belleği ayrılırken o anki konumu veren işlevdir (nil: izleme kapalı).
var allocSite atomic.Pointer[func() uint64]

// SetAllocSite tensör belleğini ayıran konumu bildiren işlevi kurar; nil izlemeyi kapatır.
// Konum, çağıranın anlamlandırdığı bir sayıdır (profil: fonksiyon<<32 | satır); 0 bilinmeyen
// konumdur. Kurulunca bütün GPU'ların önceki dağılımı silinir ve izleme o andan başlar.
// PyTorch'un bellek geçmişinin (torch.cuda.memory._record_memory_history) karşılığıdır:
// yığın yerine tek konum tutar.
func SetAllocSite(f func() uint64) {
	for _, g := range registeredAll() {
		g.pool.resetSites()
	}
	if f == nil {
		allocSite.Store(nil)
		return
	}
	allocSite.Store(&f)
}

// SitePeak tepe anında bir konumun bir veri türünde ayırdığı canlı bellektir.
type SitePeak struct {
	Site   uint64
	DType  tensor.DType
	Bytes  uint64 // ayırma sınıfına yuvarlanmış
	Blocks uint64 // canlı tensör deposu sayısı
}

// PeakLiveBySite izlenen GPU belleğinin tepe anındaki konum ve tür başına canlı belleğini ve o
// tepeyi verir (bütün GPU'ların toplamı; her GPU kendi tepesindeki dağılımıyla; sıra yoktur).
func PeakLiveBySite() (sites []SitePeak, peak uint64) {
	sum := map[siteKey]siteUse{}
	for _, g := range registeredAll() {
		g.pool.mu.Lock()
		for s, u := range g.pool.peakBy {
			t := sum[s]
			sum[s] = siteUse{t.bytes + u.bytes, t.blocks + u.blocks}
		}
		peak += g.pool.sitePeak
		g.pool.mu.Unlock()
	}
	for s, u := range sum {
		sites = append(sites, SitePeak{Site: s.site, DType: s.dt, Bytes: u.bytes, Blocks: u.blocks})
	}
	return sites, peak
}

// EnableProfile GPU çekirdek ölçümünü açar ya da kapatır (TR_GPU_PROFIL=1 ile başlangıçta
// açıktır). Kapalıyken maliyet başlatma başına tek bir atomik okumadır.
func EnableProfile(open bool) { profileOn.Store(open) }

// ProfileEnabled ölçümün açık olup olmadığı.
func ProfileEnabled() bool { return profileOn.Load() }

// KernelSamples bütün GPU'larda şimdiye kadar çözülmüş ölçümlerin çekirdek başına
// toplamlarını GPU süresine göre azalan sırada verir. Son ~250 ms'nin ölçümleri bir sonraki
// görüntüde görünür.
func KernelSamples() []KernelSample {
	sum := map[string]*KernelSample{}
	for _, g := range registeredAll() {
		for _, o := range g.profiler.sorted() {
			if t := sum[o.Name]; t != nil {
				t.Calls += o.Calls
				t.GPUTime += o.GPUTime
				continue
			}
			c := o
			sum[o.Name] = &c
		}
	}
	r := make([]KernelSample, 0, len(sum))
	for _, o := range sum {
		r = append(r, *o)
	}
	sort.Slice(r, func(i, j int) bool { return r[i].GPUTime > r[j].GPUTime })
	return r
}

// MemorySamples her GPU'nun bellek havuzunun durumunu verir.
func MemorySamples() []MemorySample {
	var r []MemorySample
	for _, g := range registeredAll() {
		b := g.pool.measurement()
		b.Device = g.dev.Index
		r = append(r, b)
	}
	return r
}

// gpuClosed dış profilleyici için C çıkış işleyicileri çalıştırıldıktan sonra doğrudur: o
// andan sonra sürücüye hiçbir çağrı yapılmaz (gir açık hata verir).
var gpuClosed atomic.Bool

var exitOnce sync.Once

// Close program sonunda çağrılır: ölçüm açıksa özeti standart hataya yazar, matris çarpımı
// kitaplığının tanımlarını yok eder ve bağlı dış profilleyicilere topladıkları veriyi
// yazdırır. Birden çok kez çağrılabilir; özet yalnızca son yazımdan sonra yeni ölçüm varsa
// yeniden yazılır.
//
// Dış profilleyici bağlıyken (CUDA_INJECTION64_PATH) bütün GPU'lar kapandıktan sonra, süreç
// genelinde bir kez C çıkış işleyicileri çalıştırılır (__cxa_finalize(NULL)): Go programı
// bunları çalıştırmadan biter, araçlar ise topladıklarını orada yazar. Bu işleyiciler
// sürücünün ve araçların C++ durumunu da yıkar; bu yüzden ardından GPU kullanılamaz
// (gpuKapandi) ve Kapat'ın sonraki çağrıları hiçbir şey yapmaz. Araç bağlı değilse hiç
// çalıştırılmaz.
func Close() {
	if gpuClosed.Load() {
		return
	}
	for _, g := range registeredAll() {
		g.closeFn()
	}
	if toolAttached() && len(registeredAll()) > 0 {
		exitOnce.Do(func() {
			for _, g := range registeredAll() {
				g.mu.Lock() // başka bir gorutin GPU'yu kullanıyorsa bitmesini bekle
			}
			gpuClosed.Store(true)
			runExitHandlers()
			for _, g := range registeredAll() {
				g.mu.Unlock()
			}
		})
	}
}

// closeFn bir GPU'nun program sonu işleri; sürücü hatası (ör. yapışkan bağlam hatası) programın
// çıkışını bozmasın diye yutulur (hata zaten bildirilmiştir).
func (g *backend) closeFn() {
	defer func() { _ = recover() }()
	g.enter()
	defer g.exit()
	if profileOn.Load() && g.profiler.support > 0 {
		g.equalize()
		g.markStepNow() // son adımın duvar süresi
		if g.profiler.callN != g.profiler.written {
			g.profiler.written = g.profiler.callN
			g.writeSummary(os.Stderr)
		}
	}
	g.ltClose()
	g.sr.flushTools()
}

// WriteProfileSummary bütün GPU'ların ölçüm özetini w'ye yazar (sürücü hatası yutulur).
func WriteProfileSummary(w io.Writer) {
	for _, g := range registeredAll() {
		func() {
			defer func() { _ = recover() }()
			g.enter()
			defer g.exit()
			g.equalize()
			g.writeSummary(w)
		}()
	}
}

// writeSummary bu GPU'nun özetini yazar (kilit tutulurken).
func (g *backend) writeSummary(w io.Writer) {
	p := &g.profiler
	fmt.Fprintln(w, messages.Text(messages.H4039, g.dev, g.sr.cardName(), g.gemmPath()))
	p.mu.Lock()
	var total2 stepStats
	num := 0
	// İlk iki adım (derleme ve ısınma) ve işaretten sonra yarım kalan son adım dışarıda.
	signed := len(p.steps)
	for i, a := range p.steps {
		if i < 2 || !a.done {
			continue
		}
		total2.wall += a.wall
		total2.host += a.host
		total2.equal += a.equal
		total2.gpuMS += a.gpuMS
		total2.idleMs += a.idleMs
		total2.launching += a.launching
		total2.gc += a.gc
		total2.gcTime += a.gcTime
		num++
	}
	p.mu.Unlock()
	if num > 0 {
		n := float64(num)
		ms := func(d time.Duration) float64 { return d.Seconds() * 1e3 / n }
		fmt.Fprintln(w, messages.Text(messages.H4040, num, fmt.Sprintf("%.2f", ms(total2.wall)), fmt.Sprintf("%.2f", total2.gpuMS/n),
			fmt.Sprintf("%.2f", total2.idleMs/n), fmt.Sprintf("%.0f", float64(total2.launching)/n), fmt.Sprintf("%.2f", ms(total2.host)),
			fmt.Sprintf("%.2f", ms(total2.equal)), fmt.Sprintf("%.2f", float64(total2.gc)/n), fmt.Sprintf("%.2f", ms(total2.gcTime))))
	} else {
		fmt.Fprintln(w, messages.Text(messages.H4041, signed))
	}
	measurements := p.sorted()
	var total time.Duration
	family := map[string]*KernelSample{}
	for _, o := range measurements {
		total += o.GPUTime
		a := family[o.Family]
		if a == nil {
			a = &KernelSample{Name: o.Family}
			family[o.Family] = a
		}
		a.Calls += o.Calls
		a.GPUTime += o.GPUTime
	}
	var families []KernelSample
	for _, a := range family {
		families = append(families, *a)
	}
	sort.Slice(families, func(i, j int) bool { return families[i].GPUTime > families[j].GPUTime })
	table := func(header string, r []KernelSample, maxN int) {
		fmt.Fprintf(w, "%-44s %9s %12s %10s %6s\n", header, messages.Text(messages.H8241), messages.Text(messages.H4043), messages.Text(messages.H4044), "%")
		for i, o := range r {
			if i >= maxN {
				break
			}
			percent := 0.0
			if total > 0 {
				percent = 100 * float64(o.GPUTime) / float64(total)
			}
			fmt.Fprintf(w, "%-44s %9d %12.2f %10.1f %6.1f\n", shorten(o.Name, 44), o.Calls, o.GPUTime.Seconds()*1e3,
				o.GPUTime.Seconds()*1e6/float64(max(o.Calls, 1)), percent)
		}
	}
	table(messages.Text(messages.H4045), families, 30)
	table(messages.Text(messages.H4046), measurements, 25)
	b := g.pool.measurement()
	bt := func(n uint64) tensor.ByteSize { return tensor.ByteSize(n) }
	fmt.Fprintln(w, messages.Text(messages.H4047, bt(b.Reserved), bt(b.Pooled), bt(b.Live), bt(b.PeakLive),
		bt(b.PeakReserved), b.DriverAllocs, b.GCRuns))
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

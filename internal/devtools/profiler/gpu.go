package profiler

import (
	"sort"
	"strings"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
)

// GPU bağlantısı: çekirdek süreleri ve GPU belleği GPU arka ucunun kendi profilinden okunur
// (internal/compute/gpu, profile_api.go). Tensör sayaçları GPU işlemlerinde yalnızca
// konakta geçen başlatma süresini görür; cihazda geçen süre buradan gelir.
//
// Çekirdek ölçümü (her başlatmanın iki zamanlama olayı arasına alınması) tr izle ve
// tr profil altında açılır, arka plan işinin profilinde kapalıdır (her zaman açık
// sayaçlar dışında yük eklenmez; GPU belleği yine görünür). Okumalar arka ucu beklemez.
// Program bitmeden (motorlar kapanmadan) BeforeClose çağrılırsa son ölçümler cihaz
// eşzamanlanarak çözülür ve arka ucun kendi raporu metin raporuna eklenir.

// gpuSnap bir anın GPU ölçüleridir.
type gpuSnap struct {
	kernel map[string]gpu.KernelSample
	memory []gpu.MemorySample
}

// takeGPUSnap ölçüleri okur (GPU kurulmamışsa ikisi de boş; sürücü yüklenmez).
func takeGPUSnap() gpuSnap {
	a := gpuSnap{memory: gpu.MemorySamples()}
	if len(a.memory) == 0 {
		return a
	}
	a.kernel = map[string]gpu.KernelSample{}
	for _, c := range gpu.KernelSamples() {
		a.kernel[c.Name] = c
	}
	return a
}

// enableGPUMeasure çekirdek ölçümünü açar; önceki durumu verir (Bitir geri yükler).
func enableGPUMeasure() (prev bool) {
	prev = gpu.ProfileEnabled()
	gpu.EnableProfile(true)
	return prev
}

// gpuDelta a ile once arasındaki çekirdek ölçümlerini (ad başına) ve aile toplamlarını
// süreye göre sıralı verir; toplam cihazda geçen süredir.
func gpuDelta(a, before map[string]gpu.KernelSample) (kernels, families []KernelInfo, total time.Duration) {
	family := map[string]*KernelInfo{}
	type raw struct {
		ad    string
		calls int64
		dur   time.Duration
	}
	var raws []raw
	for ad, c := range a {
		o := before[ad]
		n, d := c.Calls-o.Calls, c.GPUTime-o.GPUTime
		if n <= 0 && d <= 0 {
			continue
		}
		total += d
		raws = append(raws, raw{ad, n, d})
		ai := c.Family
		if ai == "" {
			ai = ad
		}
		x := family[ai]
		if x == nil {
			x = &KernelInfo{Name: ai}
			family[ai] = x
		}
		x.Calls += n
		x.Seconds += sn(d)
	}
	for _, h := range raws {
		kernels = append(kernels, KernelInfo{Name: h.ad, Calls: h.calls, Seconds: sn(h.dur), Share: ratio(sn(h.dur), sn(total))})
	}
	for _, x := range family {
		x.Share = ratio(x.Seconds, sn(total))
		families = append(families, *x)
	}
	sortBy := func(l []KernelInfo) {
		sort.Slice(l, func(i, j int) bool {
			if l[i].Seconds != l[j].Seconds {
				return l[i].Seconds > l[j].Seconds
			}
			return l[i].Name < l[j].Name
		})
	}
	sortBy(kernels)
	sortBy(families)
	return kernels, families, total
}

// gpuInfo profil başından (taban) a anına kadarki GPU ölçümleridir.
func (p *Profile) gpuInfo(a gpuSnap) GPUInfo {
	if len(a.memory) == 0 {
		return GPUInfo{Note: "GPU kullanılmadı"}
	}
	p.mu.Lock()
	measured, summary := p.gpuMeasured, p.gpuSummary
	p.mu.Unlock()
	g := GPUInfo{Used: true, Measured: measured || gpu.ProfileEnabled(), summary: summary}
	var total time.Duration
	g.Kernels, g.Families, total = gpuDelta(a.kernel, p.base.gpu.kernel)
	g.Seconds = sn(total)
	g.Kernels = g.Kernels[:min(len(g.Kernels), maxKernels)]
	switch {
	case !g.Measured:
		g.Note = "çekirdek süreleri ölçülmüyor (tr izle ve tr profil ölçer; ya da TR_GPU_PROFIL=1)"
	case len(g.Kernels) == 0:
		g.Note = "henüz çekirdek ölçümü yok"
	}
	for _, b := range a.memory {
		g.Memory = append(g.Memory, GPUMemory{
			Device: b.Device, Live: b.Live, Allocated: b.Reserved, Pooled: b.Pooled,
			PeakLive: b.PeakLive, PeakAllocated: b.PeakReserved, GCCollect: b.GCRuns,
		})
	}
	return g
}

// maxKernels dosyaya yazılan çekirdek sayısıdır (aileler hep tam).
const maxKernels = 30

// BeforeClose program bittiğinde, motorlar (GPU arka ucu) kapanmadan önce çağrılır.
// Çekirdek ölçümünü profil açtıysa: cihaz eşzamanlanır, bekleyen ölçümler çözülür, arka
// ucun kendi raporu son rapora eklenmek üzere alınır ve ölçüm eski durumuna döner (arka
// uç kapanırken raporu ikinci kez yazmasın). Birden çok kez çağrılabilir.
func (p *Profile) BeforeClose() {
	p.mu.Lock()
	ac := p.gpuOpened
	p.gpuOpened = false
	p.mu.Unlock()
	if !ac {
		return
	}
	if len(gpu.MemorySamples()) > 0 {
		var b strings.Builder
		gpu.WriteProfileSummary(&b)
		p.mu.Lock()
		p.gpuSummary = strings.TrimRight(b.String(), "\n")
		p.mu.Unlock()
	}
	gpu.EnableProfile(p.gpuPrev)
}

// gpuWindow pencerenin GPU ölçüleridir.
func gpuWindow(w *WindowInfo, a, before gpuSnap, dur time.Duration) {
	if len(a.memory) == 0 {
		return
	}
	_, families, total := gpuDelta(a.kernel, before.kernel)
	w.GPUSec, w.GPUShare, w.GPUFamilies = sn(total), ratio(sn(total), sn(dur)), families
}

// Package profiler TR'nin canlı profilidir: 'tr izle' ve 'tr profil' komutlarının ortak
// çekirdeği. Üç kaynaktan okur:
//
//   - Tensör sayaçları (internal/compute/tensor, counter.go): her zaman açık, işlem türüne,
//     yöne ve cihaza göre çağrı, süre ve bayt.
//
//   - Yorumlayıcı örneklemesi (sampler.go): yalnızca istenirse, hata ayıklayıcı
//     yuvasıyla; satır ve fonksiyon başına öz süre.
//
//   - Adım sınırı kancası (internal/stdlib/logging): ilerleme(...).güncelle(...) her çağrıda
//     bir adım sınırıdır; ms/adım ve adım başına işlem dağılımı buradan.
//
//   - GPU arka ucunun kendi profili (gpu.go): çekirdeklerin cihazda geçen süresi ve GPU
//     belleği.
//
// Profil saniyede bir anlık görüntü alır; isteyene (canlı tablo) verir ve istenmişse durum
// dosyasına (profil.json, arka plan işinde) atomik yazar. Program bitince son raporu kurar.
// Biçim: snapshot.go; mimari: docs/architecture.md, "Canlı profil".
//
// Layer: developer tools (internal/devtools); dil, motor, hesap ve araç katmanlarını içe aktarabilir.
package profiler

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Options bir profilin ayarlarıdır.
type Options struct {
	Program string // raporda görünen program adı (betik yolu)
	Source  string // ana programın kaynağı (satır metinleri)
	// SampleLines satır/fonksiyon örneklemesini açar (yorumlayıcının hata ayıklayıcı yuvası).
	SampleLines bool
	// GPUMeasure GPU çekirdeklerinin cihazdaki süresinin ölçümünü açar (gpu.go; tr izle ve
	// tr profil açar, arka plan işinin profili açmaz).
	GPUMeasure bool
	// StatusFile boş değilse canlı görüntü saniyede en çok bir kez bu dosyaya yazılır.
	StatusFile string
	// Warning durum dosyası yazılamazsa (bir kez) çağrılır; nil ise sessiz kalınır.
	Warning func(string)
}

// Profile çalışan bir programın profilidir.
type Profile struct {
	pick   Options
	start  time.Time
	smp    *sampler
	writer *statusWriter
	lines  []string
	// modLines 'aktar' ile yüklenen modüllerin satırlarıdır (moduleLines).
	isLines  sync.Mutex
	modLines map[*interp.Source][]string

	isStep sync.Mutex
	step   stepState

	mu       sync.Mutex
	base     snap   // profil başladığındaki sayaçlar (süreç geneli sayaçlardan çıkarılır)
	history  []snap // son windowTime'nin ham görüntüleri (pencere için)
	listener func(*Snapshot)

	// GPU çekirdek ölçümü (gpu.go; mu ile korunur): gpuOpened ölçümü bu profil açtı ve henüz
	// geri almadı, gpuPrev açmadan önceki durum, gpuMeasured ölçüm profil süresince
	// açıktı, gpuSummary arka ucun son raporu (BeforeClose).
	gpuOpened, gpuPrev, gpuMeasured bool
	gpuSummary                      string

	stop   chan struct{}
	done   sync.WaitGroup
	before sync.Once
}

// StatusFileName arka plan işinin klasöründeki canlı profil dosyasının adıdır.
const StatusFileName = "profil.json"

// JobStatusFile program bir arka plan işi olarak çalışıyorsa (TR_IS_KLASORU) iş klasöründeki
// profil.json'un yoludur; değilse boş (durum dosyası yazılmaz).
func JobStatusFile() string {
	if k := logging.JobDir(); k != "" {
		return filepath.Join(k, StatusFileName)
	}
	return ""
}

// windowTime canlı tablonun "son" ölçülerinin kapsadığı süredir.
const windowTime = 5 * time.Second

// stepState adım sınırı kancasının biriktirdiğidir.
type stepState struct {
	var_ bool
	// last ve total son güncellenen çubuğun adımı ve toplamıdır; all profil başından beri
	// bütün çubuklarda tamamlanan adımlardır (her epoch'ta yeni bir çubuk adımları baştan
	// sayar), allAtFirst ilk adım sınırındaki all'dır.
	last, total, all, allAtFirst int64
	firstTime, lastTime          time.Time
}

// Start m üzerinde çalışacak programın profilini başlatır; m.Run'dan önce çağrılır.
// Örnekleme istenmişse m.Debugger'a bağlanır (hata ayıklayıcıyla birlikte kullanılmaz).
func Start(m *interp.VM, s Options) *Profile {
	p := &Profile{pick: s, start: time.Now(), stop: make(chan struct{})}
	p.lines = strings.Split(s.Source, "\n")
	if s.StatusFile != "" {
		p.writer = &statusWriter{fpath: s.StatusFile, warning: s.Warning}
	}
	if s.SampleLines {
		p.smp = newSampler()
		m.Debugger = p.smp
		p.smp.start()
	}
	if s.GPUMeasure {
		// Ölçüm zaten açıksa (TR_GPU_PROFIL=1) dokunulmaz: arka uç raporunu kendisi yazar.
		p.gpuPrev = enableGPUMeasure()
		p.gpuOpened, p.gpuMeasured = !p.gpuPrev, true
	}
	logging.SetStepHook(p.stepBoundary)
	p.mu.Lock()
	p.base = p.takeSnap()
	p.history = []snap{p.base}
	p.mu.Unlock()
	p.done.Add(1)
	go p.loop()
	return p
}

// Listen her saniye alınan canlı görüntüyü f'ye verir (f goroutine'den çağrılır; kısa olmalı).
func (p *Profile) Listen(f func(*Snapshot)) {
	p.mu.Lock()
	p.listener = f
	p.mu.Unlock()
}

// stepBoundary ilerleme(...).güncelle(...)'nin her çağrısında çalışır (yorumlayıcı kilidi altında).
func (p *Profile) stepBoundary(a logging.StepBoundary) {
	p.isStep.Lock()
	d := &p.step
	if a.Step >= d.last {
		d.all += a.Step - d.last
	} else {
		d.all += a.Step // yeni çubuk (adım geriye düştü): adımları baştan sayıyor
	}
	if !d.var_ {
		d.var_, d.allAtFirst, d.firstTime = true, d.all, a.Time
	}
	d.last, d.lastTime, d.total = a.Step, a.Time, a.Total
	p.isStep.Unlock()
}

func (p *Profile) loop() {
	defer p.done.Done()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			p.tick()
		}
	}
}

// tick saniyelik görüntüyü alır, geçmişe ekler, dinleyiciye verir ve durum dosyasına yazar.
func (p *Profile) tick() {
	a := p.takeSnap()
	p.mu.Lock()
	p.history = append(p.history, a)
	for len(p.history) > 2 && a.time2.Sub(p.history[1].time2) >= windowTime {
		p.history = p.history[1:]
	}
	before := p.history[0]
	f := p.listener
	p.mu.Unlock()
	g := p.setup(a, &before, false)
	if f != nil {
		f(g)
	}
	if p.writer != nil {
		p.writer.write(g)
	}
}

// Finish profili durdurur ve son raporu verir; kod programın çıkış kodudur. Birden çok kez
// çağrılabilir (sonraki çağrılar yalnızca raporu yeniden kurar). BeforeClose
// çağrılmadıysa (ör. program başlamadan çıkıldı) GPU ölçümü burada eski durumuna döner.
func (p *Profile) Finish(code int) *Snapshot {
	p.before.Do(func() {
		close(p.stop)
		p.done.Wait()
		if p.smp != nil {
			p.smp.halt()
		}
		logging.SetStepHook(nil)
		p.mu.Lock()
		if p.gpuOpened {
			p.gpuOpened = false
			gpu.EnableProfile(p.gpuPrev)
		}
		p.mu.Unlock()
	})
	a := p.takeSnap()
	p.mu.Lock()
	before := p.history[0]
	p.mu.Unlock()
	g := p.setup(a, &before, true)
	g.ExitCode = &code
	if p.writer != nil {
		p.writer.write(g)
	}
	return g
}

// snap bir anın ham ölçüleridir (pencere farkları bunlardan alınır).
type snap struct {
	time2      time.Time
	counters   []tensor.CounterRecord
	step       stepState
	positions  []posSample
	sampleTime time.Duration
	sample     int64
	memory     runtime.MemStats
	gpu        gpuSnap
}

func (p *Profile) takeSnap() snap {
	a := snap{time2: time.Now(), counters: tensor.CounterSnapshot()}
	p.isStep.Lock()
	a.step = p.step
	p.isStep.Unlock()
	if p.smp != nil {
		a.positions, a.sampleTime, a.sample = p.smp.snapshot()
	}
	runtime.ReadMemStats(&a.memory)
	a.gpu = takeGPUSnap()
	return a
}

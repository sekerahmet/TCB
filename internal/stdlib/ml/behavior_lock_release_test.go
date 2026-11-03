package ml

import (
	"fmt"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Uzun tensör işi sürerken başka görevin gecikmesi (10.2; Faz 2 Kol B, S-kilit). Bir görev 60
// kez 1024×1024 `@` yapar; öbürü ya 5 ms'de bir uyanır (gözcü) ya da 100 Hz'lik döngüdür (tur).
// Süreler Go'da tekdüze saatle ölçülür (TR_time_ms duvar saatidir, 10-11 ms sıçrar):
//   - bekleme: gözcünün uykusu bitip kilidi geri alana dek geçen süre (uykunun kendi aşımı değil);
//   - kaçan tur: 100 Hz'de tur 10 ms'lik penceredir; turun başlangıcı (kilit geri alındığında)
//     kendi penceresinin sonundan sonraysa tur kaçmıştır. Pencereler sabit takvimdedir: geç kalan
//     döngü sonraki pencereleri atlamaz, onlara yetişmeye çalışır.
//
// Yalnız yükün başı ile sonu arasındaki uyanışlar ve turlar sayılır. Bu dosya bugün ölçer ve
// sayıları yazar (go test -v); bitiş ölçütü (en büyük bekleme ≤ 10 ms, kaçan tur 0) kilidi
// bırakan yerleşikler gelince sınır olur.

// lockLoad yükün programıdır: kez kere boyut×boyut `@`, cihaz "cpu" ya da "gpu". GPU'da son
// satır kuyruğun bitişini bekler.
const lockLoad = `TR_var bitti = TR_false
TR_var a = TR_randn([boyut, boyut], TR_seed: 1).TR_to_device(cihaz)
TR_var b = TR_randn([boyut, boyut], TR_seed: 2).TR_to_device(cihaz)
TR_var c = a @ b
TR_var son = TR_sum(c).TR_tolist
TR_async { TR_for TR_not bitti { bekleyen() } }
TR_sleep(0.1)
yük_başı()
TR_for t TR_in 0..kez { c = a @ b }
son = TR_sum(c).TR_tolist
yük_sonu()
bitti = TR_true
TR_wait_all()
`

// lockProbe yükün penceresini ve bekleyen görevin ölçümlerini toplar.
type lockProbe struct {
	mu       sync.Mutex
	from, to time.Time
	at       []time.Time     // ölçümün ait olduğu an: uyanış ya da pencerenin başı
	waits    []time.Duration // gözcü: kilidi geri alma süresi; tur: başlangıcın pencere başından gecikmesi
}

func (p *lockProbe) add(at time.Time, d time.Duration) {
	p.mu.Lock()
	p.at, p.waits = append(p.at, at), append(p.waits, d)
	p.mu.Unlock()
}

// inLoad yük penceresindeki ölçümlerdir.
func (p *lockProbe) inLoad() []time.Duration {
	var out []time.Duration
	for i, at := range p.at {
		if !at.Before(p.from) && !at.After(p.to) {
			out = append(out, p.waits[i])
		}
	}
	return out
}

// lockWork bir yüktür: count kez size×size `@`, device'ta.
type lockWork struct {
	device      string
	size, count int
}

func (w lockWork) String() string {
	return fmt.Sprintf("%s %d×%d `@` ×%d", w.device, w.size, w.size, w.count)
}

// runLockLoad yükü çalıştırır; bekleyen görev her çağrıda wait'i çalıştırır.
func runLockLoad(t *testing.T, w lockWork, wait func(m *interp.VM, p *lockProbe)) (*lockProbe, time.Duration) {
	t.Helper()
	p := &lockProbe{}
	m := interp.New()
	m.Out = io.Discard
	mark := func(at *time.Time) *interp.BuiltinVal {
		return &interp.BuiltinVal{Name: "yük", Fn: func(*interp.VM, []interp.Value, map[string]interp.Value) (interp.Value, error) {
			*at = time.Now()
			return interp.Null, nil
		}}
	}
	m.GlobalEnv.Define("cihaz", &interp.StringVal{Val: w.device}, true)
	m.GlobalEnv.Define("boyut", interp.NewInt(int64(w.size)), true)
	m.GlobalEnv.Define("kez", interp.NewInt(int64(w.count)), true)
	m.GlobalEnv.Define("yük_başı", mark(&p.from), true)
	m.GlobalEnv.Define("yük_sonu", mark(&p.to), true)
	m.GlobalEnv.Define("bekleyen", &interp.BuiltinVal{Name: "bekleyen", Fn: func(y *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		wait(y, p)
		return interp.Null, nil
	}}, true)
	_, err := m.Run(trrun.Parse(t, lockLoad))
	m.WaitAsync()
	if err != nil {
		t.Fatal(err)
	}
	return p, p.to.Sub(p.from)
}

// sleeper 5 ms uyur; uyanıştan kilidin geri alınmasına dek geçen süreyi yazar.
func sleeper(m *interp.VM, p *lockProbe) {
	var woke time.Time
	m.Blocking(func() {
		time.Sleep(5 * time.Millisecond)
		woke = time.Now()
	})
	p.add(woke, time.Since(woke))
}

// looper 100 Hz'lik döngüdür: sıradaki pencerenin başına dek uyur, başlangıcın pencere başından
// gecikmesini yazar.
func looper() func(m *interp.VM, p *lockProbe) {
	const period = 10 * time.Millisecond
	var start time.Time
	k := 0
	return func(m *interp.VM, p *lockProbe) {
		if start.IsZero() {
			start = time.Now()
		}
		due := start.Add(time.Duration(k) * period)
		k++
		m.Blocking(func() { time.Sleep(time.Until(due)) })
		p.add(due, time.Since(due))
	}
}

// waitStats ortanca, %99 (en yakın sıra: ⌈0,99·n⌉. öğe) ve en büyüktür.
func waitStats(ws []time.Duration) (median, p99, worst time.Duration) {
	s := slices.Clone(ws)
	slices.Sort(s)
	n := len(s)
	return s[(n-1)/2], s[(99*n+99)/100-1], s[n-1]
}

func ms(d time.Duration) string { return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond)) }

// Ölçüm (10.2, S-kilit adım 1): uzun tensör işi sürerken 5 ms'de bir uyanan görevin kilidi geri
// alma süresi ve 100 Hz döngünün kaçan turu. İki yük: bitiş ölçütünün yükü (60 kez 1024×1024 `@`;
// her çarpım bir deyimdir, kilit bugün çarpımlar arasında devredilir) ve tek çağrısı uzun süren
// yük (8 kez 2048×2048 `@`; çağrı sürerken devir yok). Bugün yalnız sayıları yazar.
func TestOtherTasksRunDuringLongTensorWork(t *testing.T) {
	devices := []string{"cpu"}
	if tensor.HasDevice(tensor.GPU(0)) && !gpu.Simulated() {
		devices = append(devices, "gpu") // gecikme yalnız gerçek kartta anlamlıdır (benzetimde süre eşzamanlamaya yazılır)
	}
	for _, device := range devices {
		for _, w := range []lockWork{{device, 1024, 60}, {device, 2048, 8}} {
			reportLockWaits(t, w)
		}
	}
}

// reportLockWaits yükü gözcüyle ve 100 Hz döngüyle birer kez çalıştırır, ölçümü yazar.
func reportLockWaits(t *testing.T, w lockWork) {
	t.Helper()
	p, load := runLockLoad(t, w, sleeper)
	waits := p.inLoad()
	if len(waits) == 0 {
		t.Fatalf("%s: yük %s sürdü, gözcü yük sırasında hiç uyanmadı", w, ms(load))
	}
	med, p99, worst := waitStats(waits)
	t.Logf("%s, gözcü: yük %s, %d uyanış; kilidi geri alma ortanca %s, %%99 %s, en büyük %s",
		w, ms(load), len(waits), ms(med), ms(p99), ms(worst))

	p, load = runLockLoad(t, w, looper())
	turns := p.inLoad()
	missed := 0
	for _, late := range turns {
		if late > 10*time.Millisecond {
			missed++
		}
	}
	_, _, worst = waitStats(append(turns, 0))
	t.Logf("%s, 100 Hz: yük %s, %d tur, kaçan %d; en geç başlangıç pencere başından %s sonra",
		w, ms(load), len(turns), missed, ms(worst))
}

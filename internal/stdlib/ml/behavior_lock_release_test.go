package ml

import (
	"fmt"
	"io"
	"slices"
	"sync"
	"sync/atomic"
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
	overs    []time.Duration // gözcü: uykunun aşımı (uyanan gorutinin boş işlemci beklemesi dahil)
	// gateWaits kilitli yolun çakışan kilitsiz işlemi bekleyişleridir (kilit tutularak bekler).
	gateWaits int64
}

func (p *lockProbe) add(at time.Time, d, over time.Duration) {
	p.mu.Lock()
	p.at, p.waits, p.overs = append(p.at, at), append(p.waits, d), append(p.overs, over)
	p.mu.Unlock()
}

// inLoad yük penceresindeki ölçümlerdir: bekleme ve uyku aşımı.
func (p *lockProbe) inLoad() (waits, overs []time.Duration) {
	for i, at := range p.at {
		if !at.Before(p.from) && !at.After(p.to) {
			waits, overs = append(waits, p.waits[i]), append(overs, p.overs[i])
		}
	}
	return waits, overs
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
	var gateWaits atomic.Int64
	tensor.GateWaited = func() { gateWaits.Add(1) }
	defer func() { tensor.GateWaited = nil }()
	defer func() { p.gateWaits = gateWaits.Load() }()
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

// sleeperNap gözcünün uykusudur.
const sleeperNap = 5 * time.Millisecond

// sleeper sleeperNap uyur; uyanıştan kilidin geri alınmasına dek geçen süreyi yazar.
func sleeper(m *interp.VM, p *lockProbe) {
	var start, woke time.Time
	m.Blocking(func() {
		start = time.Now()
		time.Sleep(sleeperNap)
		woke = time.Now()
	})
	p.add(woke, time.Since(woke), woke.Sub(start)-sleeperNap)
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
		p.add(due, time.Since(due), 0)
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

// Ölçüm (10.2, S-kilit): uzun tensör işi sürerken 5 ms'de bir uyanan görevin kilidi geri alma
// süresi ve 100 Hz döngünün kaçan turu. İki yük (bitiş ölçütü, DENETİM istek 64 Ş6): 60 ardışık
// 1024×1024 `@` ve tek çağrısı en az 30 ms süren `@` (8 çağrı; boyut makinede ölçülerek seçilir).
func TestOtherTasksRunDuringLongTensorWork(t *testing.T) {
	devices := []string{"cpu"}
	if tensor.HasDevice(tensor.GPU(0)) && !gpu.Simulated() {
		devices = append(devices, "gpu") // gecikme yalnız gerçek kartta anlamlıdır (benzetimde süre eşzamanlamaya yazılır)
	}
	for _, device := range devices {
		for _, w := range []lockWork{{device, 1024, 60}, {device, longCallSize(t, device), 8}} {
			reportLockWaits(t, w)
		}
	}
}

// longCall tek çağrısı uzun sayılan `@`'ın en kısa süresidir (bitiş ölçütü: "en az 30 ms").
const longCall = 30 * time.Millisecond

// longCallSize tek `@` çağrısı longCall'dan uzun sürene dek kare matrisin boyutunu 256'şar
// büyütür (iki çağrının kısası; GPU'da sonuç konağa indirilerek).
func longCallSize(t *testing.T, device string) int {
	t.Helper()
	dev := tensor.CPU
	if device == "gpu" {
		dev = tensor.GPU(0)
	}
	for n := 1024; n <= 16384; n += 256 {
		a, b := tensor.ZerosOn(dev, n, n), tensor.ZerosOn(dev, n, n)
		best := time.Hour
		for range 2 {
			start := time.Now()
			_ = tensor.MatMul(a, b).SumAll().Data()
			best = min(best, time.Since(start))
		}
		if best >= longCall {
			t.Logf("%s: tek `@` %d×%d'de %s sürdü (≥ %s)", device, n, n, ms(best), ms(longCall))
			return n
		}
	}
	t.Fatalf("%s: 16384×16384 `@` bile %s'den kısa", device, ms(longCall))
	return 0
}

// reportLockWaits yükü gözcüyle ve 100 Hz döngüyle birer kez çalıştırır, ölçümü yazar. Uzun
// yükte tek çağrı yük sırasında (yük / kez) da longCall'dan uzun sürmelidir: başka görev canlıyken
// işçi yarıya iner ve çağrı kalibrasyondakinden kısalabilir (DENETİM istek 69 Ş1). Kısa kalırsa
// boyut 256 büyür ve iki ölçüm yinelenir; yalnız koşulu sağlayan ölçüm yazılır.
func reportLockWaits(t *testing.T, w lockWork) {
	t.Helper()
	for ; w.size <= 16384; w.size += 256 {
		p, load := runLockLoad(t, w, sleeper)
		q, loopLoad := runLockLoad(t, w, looper())
		call, loopCall := load/time.Duration(w.count), loopLoad/time.Duration(w.count)
		if w.count != 60 && min(call, loopCall) < longCall {
			t.Logf("%s: yük sırasında tek çağrı %s ve %s (< %s), boyut büyüyor", w, ms(call), ms(loopCall), ms(longCall))
			continue
		}
		waits, overs := p.inLoad()
		if len(waits) == 0 {
			// Hızlı kartta 60 çağrı gözcünün bir uykusundan kısa sürebilir: bekleme yoktur, ölçülecek de yoktur.
			if load < 2*sleeperNap {
				t.Logf("%s: yük %s sürdü, gözcünün uykusundan kısa; ölçüm yok", w, ms(load))
				return
			}
			t.Fatalf("%s: yük %s sürdü, gözcü yük sırasında hiç uyanmadı", w, ms(load))
		}
		med, p99, worst := waitStats(waits)
		_, _, overWorst := waitStats(overs)
		t.Logf("%s, gözcü: yük %s, %d uyanış; kilidi geri alma ortanca %s, %%99 %s, en büyük %s; uyku aşımının en büyüğü %s; çakışma beklemesi %d; tek çağrı %s",
			w, ms(load), len(waits), ms(med), ms(p99), ms(worst), ms(overWorst), p.gateWaits, ms(call))

		turns, _ := q.inLoad()
		missed := 0
		for _, late := range turns {
			if late > 10*time.Millisecond {
				missed++
			}
		}
		_, _, worst = waitStats(append(turns, 0))
		t.Logf("%s, 100 Hz: yük %s, %d tur, kaçan %d; en geç başlangıç pencere başından %s sonra; çakışma beklemesi %d; tek çağrı %s",
			w, ms(loopLoad), len(turns), missed, ms(worst), q.gateWaits, ms(loopCall))
		return
	}
	t.Fatalf("%s: 16384×16384'te bile yük sırasında tek çağrı %s'den kısa", w.device, ms(longCall))
}

// Kural (10.2, S-kilit, izin): kilitsiz bir `@` sürerken başka görev girdisine yerinde yazarsa
// yazma çarpımın bitmesini bekler; çarpım yarım güncellenmiş girdi okumaz. Her turda gözcü görev
// çarpım başlarken uyanır ve girdiyi sıfırlar: sonuç ya eski girdinin çarpımıdır (yazma bekledi)
// ya da sıfırdır (yazma çarpımdan önce geldi), karışığı hiç değildir.
func TestInPlaceWriteWaitsForTheUnlockedMatMul(t *testing.T) {
	out := mustRun(t, `TR_var b = TR_randn([768, 768], TR_seed: 2)
TR_var karışık = 0
TR_var sıfır = 0
TR_for tur TR_in 0..20 {
    TR_var a = TR_randn([768, 768], TR_seed: 1)
    TR_var eski = a @ b
    TR_var başla = TR_false
    TR_async {
        TR_for TR_not başla { TR_sleep(0.0001) }
        a *= 0.0
    }
    başla = TR_true
    TR_var c = a @ b
    TR_wait_all()
    TR_var fark = TR_sum(TR_abs(c - eski)).TR_tolist
    TR_var büyüklük = TR_sum(TR_abs(c)).TR_tolist
    TR_if büyüklük == 0.0 { sıfır += 1 } TR_else TR_if fark != 0.0 { karışık += 1 }
}
TR_print(karışık)`)
	if out != "0\n" {
		t.Fatalf("kilitsiz çarpım yarım güncellenmiş girdi okudu (karışık sonuçlu tur sayısı): %s", out)
	}
}

// Kural (10.2, S-kilit, izin): kilitsiz öğe öğe işlem (2^23 öğeli `TR_sin`; eşik 2^23) için de aynısı:
// başka görevin girdiye yerinde yazması işlemin bitmesini bekler; sonuç ya eski girdinin
// sinüsüdür ya sıfırdır (sin 0 = 0), karışığı hiç değildir. İşlem birkaç ms sürer: yazan görevin
// uykusu (Windows'ta ~1 ms aşar) işlemin içine düşebilsin.
func TestInPlaceWriteWaitsForTheUnlockedElementwiseOp(t *testing.T) {
	out := mustRun(t, `TR_var karışık = 0
TR_for tur TR_in 0..20 {
    TR_var a = TR_randn([8388608], TR_seed: 1)
    TR_var eski = TR_sin(a)
    TR_var başla = TR_false
    TR_async {
        TR_for TR_not başla { TR_sleep(0.0001) }
        a *= 0.0
    }
    başla = TR_true
    TR_var c = TR_sin(a)
    TR_wait_all()
    TR_var fark = TR_sum(TR_abs(c - eski)).TR_tolist
    TR_var büyüklük = TR_sum(TR_abs(c)).TR_tolist
    TR_if büyüklük != 0.0 TR_and fark != 0.0 { karışık += 1 }
}
TR_print(karışık)`)
	if out != "0\n" {
		t.Fatalf("kilitsiz öğe öğe işlem yarım güncellenmiş girdi okudu (karışık sonuçlu tur sayısı): %s", out)
	}
}

// Kural (S-kilit, DENETİM istek 64 (4a) sapması): bileşik bir yerleşik (iyileştiricinin adımı,
// parametreleri tek tek günceller) kilitsiz bir işlemin okuduğu parametreye gelince kilidi tutarak
// bekler; öbür görevler yarım güncellenmiş parametre görmez ve bekleme kilitlenmeden biter. Bir
// görev w2'yi okuyan kilitsiz `@`'ı döngüde yapar, öbürü w1 ile w2'nin aynı adımda olduğunu
// (eşit değer) yoklar; ana görev w1, w2 sırasıyla güncelleyen adımı 30 kez atar.
func TestCompositeBuiltinShowsNoHalfUpdatedParameters(t *testing.T) {
	var waits atomic.Int64
	tensor.GateWaited = func() { waits.Add(1) }
	defer func() { tensor.GateWaited = nil }()
	out := mustRun(t, `TR_var w1 = TR_zeros([4], TR_requires_grad: TR_true)
TR_var w2 = TR_zeros([768, 768], TR_requires_grad: TR_true)
TR_var x = TR_randn([768, 768], TR_seed: 1)
TR_var op = TR_SGD([w1, w2], TR_lr: 1.0)
TR_var dur = TR_false
TR_var tutarsız = 0
TR_var yoklama = 0
TR_async { TR_for TR_not dur { TR_var c = x @ w2 } }
TR_async {
    TR_for TR_not dur {
        TR_if w1[0].TR_tolist != w2[0][0].TR_tolist { tutarsız += 1 }
        yoklama += 1
        TR_sleep(0.0002)
    }
}
TR_for i TR_in 0..30 {
    TR_backward(TR_sum(w1) + TR_sum(w2))
    op.TR_step()
}
dur = TR_true
TR_wait_all()
TR_print(tutarsız, yoklama > 0, w1[0].TR_tolist == w2[0][0].TR_tolist)`)
	if out != "0 true true\n" {
		t.Fatalf("bileşik yerleşik sürerken yarım güncellenmiş parametre görüldü (tutarsız, yoklandı, sonda eşit): %s", out)
	}
	if waits.Load() == 0 {
		t.Fatal("adım kilitsiz çarpımın okuduğu parametreyi hiç beklemedi: sınama çakışmayı kurmadı")
	}
	t.Logf("çakışma beklemesi %d", waits.Load())
}

// Kural (10.2, S-kilit): büyük adım (2^20'den çok öğe) kilit bırakılarak, tek çekirdek olarak
// çalışır; izinler birlikte alındığından öbür görev w1 ile w2'yi yine hep aynı adımda görür.
// Yoklayan görevin okuması adımın yazma iznini bekler (çakışma beklemesi > 0). Gradyan
// sıfırlanmadığından birikir: 60 adımda w = −(1 + … + 60) = −1830.
func TestUnlockedStepShowsNoHalfUpdatedParameters(t *testing.T) {
	var waits atomic.Int64
	tensor.GateWaited = func() { waits.Add(1) }
	defer func() { tensor.GateWaited = nil }()
	out := mustRun(t, `TR_var w1 = TR_zeros([4], TR_requires_grad: TR_true)
TR_var w2 = TR_zeros([1024, 1024], TR_requires_grad: TR_true)
TR_var op = TR_SGD([w1, w2], TR_lr: 1.0)
TR_var dur = TR_false
TR_var tutarsız = 0
TR_var yoklama = 0
TR_async {
    TR_for TR_not dur {
        TR_if w1[0].TR_tolist != w2[1023][1023].TR_tolist { tutarsız += 1 }
        yoklama += 1
        TR_sleep(0.0002)
    }
}
TR_for i TR_in 0..60 {
    TR_backward(TR_sum(w1) + TR_sum(w2))
    op.TR_step()
}
dur = TR_true
TR_wait_all()
TR_print(tutarsız, yoklama > 0, w1[0].TR_tolist == w2[1023][1023].TR_tolist, w1[0].TR_tolist)`)
	if out != "0 true true -1830.0\n" {
		t.Fatalf("kilitsiz adım sürerken yarım güncellenmiş parametre görüldü (tutarsız, yoklandı, sonda eşit, w1[0]): %s", out)
	}
	if waits.Load() == 0 {
		t.Fatal("yoklama kilitsiz adımı hiç beklemedi: adım kilit altında çalışmış olabilir")
	}
	t.Logf("çakışma beklemesi %d", waits.Load())
}

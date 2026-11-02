package interp

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// lockWaitSlack, (n + 1) × lockHold sınırının üstüne bir deyim, saatin adımı (Windows'ta ~0,5 ms)
// ve işletim sisteminin uyandırma gecikmesi için bırakılan paydır: 20 koşunun (iki ve üç meşgul
// görev, 40 ölçüm) en büyük aşımı 15,7 ms'ydi, iki katı alındı (yüklü makinede bir kez 21 ms
// görüldü; 2026-11-01, D1 ikinci dilim).
const lockWaitSlack = 32 * time.Millisecond

// lockWaits busy meşgul görevle (ve uyuyup uyanan ana görevle) programı çalıştırır; kilidi
// bekleyerek alan her görevin önündeki bekleyen sayısını, numarasını ve bekleme süresini döndürür.
func lockWaits(t *testing.T, busy int) (aheads []int, tickets []uint64, waits []time.Duration, turns string) {
	t.Helper()
	var code strings.Builder
	code.WriteString("TR_var dur = TR_false\nTR_var turlar = []\n")
	code.WriteString("TR_func meşgul(i: TR_Int) {\n    TR_for TR_not dur {\n        turlar[i] += 1\n    }\n}\n")
	for i := 0; i < busy; i++ {
		fmt.Fprintf(&code, "turlar.TR_append(0)\nTR_async { meşgul(%d) }\n", i)
	}
	code.WriteString("TR_sleep(0.3)\ndur = TR_true\nTR_wait_all()\nTR_var result = turlar\n")
	machine := New()
	machine.Out = io.Discard
	var mu sync.Mutex
	machine.gilWaitHook = func(ahead int, ticket uint64, waited time.Duration) {
		mu.Lock()
		aheads, tickets, waits = append(aheads, ahead), append(tickets, ticket), append(waits, waited)
		mu.Unlock()
	}
	val, err := machine.Run(mustParseProgram(t, code.String()))
	machine.WaitAsync()
	if err != nil {
		t.Fatal(err)
	}
	return aheads, tickets, waits, programResult(machine, val).Inspect()
}

// Kural (10.2): kilidi bekleyen görevler onu geldikleri sırayla alır; önünde n görev bekleyen bir
// görev kilidi en çok (n + 1) × 5 ms ve n + 1 deyim içinde alır. Süre kilit isteği ile alınması
// arasında, acquireGIL'in içinde tekdüze saatle ölçülür (TR_time_ms duvar saatidir, 10-11 ms
// sıçrar); iki ve üç meşgul görevle. Eskiden devir 256 turda bir ve sıra yoktu (sync.Mutex).
// Sıra her koşuda kesin sınanır. Süre işletim sisteminin zamanlamasına da bağlıdır: Windows'ta (saat
// adımı ~0,5 ms, yüklü makinede paralel testler) üç koşu yapılır ve biri sınıra sığmalıdır, kuralı
// bozan bir değişiklik (sayıyla ya da hiç devretmek) üçünü de aşar; Linux'ta tek koşu ve kesin
// sınır. Her koşunun ölçümü testin çıktısına yazılır (go test -v).
func TestLockIsTakenInArrivalOrderWithinTheDelay(t *testing.T) {
	runs := 3
	if runtime.GOOS == "linux" {
		runs = 1
	}
	for _, busy := range []int{2, 3} {
		within := false
		var last []string
		for run := 1; run <= runs; run++ {
			over := lockWaitsOverLimit(t, busy, run)
			if len(over) == 0 {
				within = true
			} else {
				last = over
			}
		}
		if !within {
			t.Errorf("%d meşgul görev: %d koşunun hiçbiri sınıra sığmadı; sonuncusu:\n%s", busy, runs, strings.Join(last, "\n"))
		}
	}
}

// lockWaitsOverLimit bir koşuda sırayı ve ilerlemeyi kesin sınar, ölçümü testin çıktısına yazar ve
// süre sınırını aşan beklemeleri verir.
func lockWaitsOverLimit(t *testing.T, busy, run int) []string {
	t.Helper()
	aheads, tickets, waits, turns := lockWaits(t, busy)
	if len(tickets) < 20 {
		t.Fatalf("%d meşgul görev: kilit yalnız %d kez beklenerek alındı", busy, len(tickets))
	}
	if strings.Contains(turns, "[0") || strings.Contains(turns, " 0") {
		t.Fatalf("%d meşgul görev: her görev ilerlemeliydi: %s", busy, turns)
	}
	var over []string
	var worst time.Duration
	for i, w := range waits {
		if i > 0 && tickets[i] <= tickets[i-1] {
			t.Fatalf("%d meşgul görev: kilit geliş sırasıyla verilmedi: %d. numara %d'den sonra", busy, tickets[i], tickets[i-1])
		}
		limit := time.Duration(aheads[i]+1)*lockHold + lockWaitSlack
		if w > limit {
			over = append(over, fmt.Sprintf("önünde %d bekleyenle %v beklendi, sınır %v", aheads[i], w, limit))
		}
		worst = max(worst, w-time.Duration(aheads[i]+1)*lockHold)
	}
	t.Logf("%s, %d meşgul görev, %d. koşu: %d bekleme, sınırı aşan %d; (n + 1) × 5 ms'nin en çok %v üstünde (eksi: altında), pay %v; turlar %s",
		runtime.GOOS, busy, run, len(waits), len(over), worst, lockWaitSlack, turns)
	return over
}

// Kuşak (10.2): kilidi tutanın süresini zamanlayıcı ölçer ve süresi dolan kuşağı yazar. Eski
// tutan için kurulmuş, geç kalan bir zamanlayıcı yeni tutanı devretmez; kendi kuşağının
// zamanlayıcısı devreder. Zamanlayıcı ve yedek saat kapalı, süre elle doldurulur (uykusuz).
func TestAnOldHoldersTimerDoesNotHandOffTheNewHolder(t *testing.T) {
	machine := New()
	machine.gil.timerOff, machine.gil.clockOff = true, true
	machine.acquireGIL()
	old := machine.gilGen
	machine.handOffLock()
	current := machine.gilGen
	if current == old {
		t.Fatalf("kilit el değiştirince kuşak artmalıydı: %d", current)
	}
	machine.pending.Add(1) // bekleyen var: deyim sınırı süreye bakar
	machine.gil.expire(old)
	machine.yieldPoint()
	if machine.gilGen != current {
		t.Fatalf("eski tutanın (%d. kuşak) süresi yeni tutanı (%d. kuşak) devretti", old, current)
	}
	machine.gil.expire(current)
	machine.yieldPoint()
	if machine.gilGen == current {
		t.Fatalf("tutanın kendi kuşağının (%d) süresi dolunca kilit devredilmeliydi", current)
	}
	machine.pending.Add(-1)
	machine.releaseGIL()
}

// Kuşak, zamanlayıcı açıkken (10.2): kuşak 1'de bekleyen gelince kurulan zamanlayıcı, kilit el
// değiştirdikten (kuşak 2) sonra ateşlenirse yeni tutan 5 ms dolmadan devretmez. Zamanlayıcının
// fonksiyonu arm'dan alınıp el değiştirdikten sonra elle çağrılır (geç ateşlenen, durdurulamamış
// eski zamanlayıcı). Zamanlayıcı kuşağı kurulurken değil ateşlenirken okusaydı yeni tutanın
// kuşağını yazardı.
func TestALateTimerOfTheOldHolderDoesNotHandOffTheNewHolder(t *testing.T) {
	machine := New()
	machine.gil.clockOff = true
	armed := make(chan func(), 1)
	machine.gil.afterFunc = func(_ time.Duration, f func()) *time.Timer {
		armed <- f
		return time.NewTimer(time.Hour) // durdurulur; ateşlenmesini test yapar
	}
	machine.acquireGIL()
	waiter := machine.NewTaskVM()
	fired := make(chan struct{})
	result := make(chan string, 1)
	go func() {
		waiter.acquireGIL() // kuşak 1'de kuyruğun başı olur: zamanlayıcı kurulur
		<-fired
		current, since := waiter.gilGen, monotonic()-waiter.gilSince
		waiter.pending.Add(1) // bekleyen var: deyim sınırı süreye bakar
		waiter.yieldPoint()
		switch {
		case waiter.gilGen != current:
			result <- fmt.Sprintf("eski tutanın zamanlayıcısı yeni tutanı (%d. kuşak) %v sonra devretti", current, since)
		case since >= lockHold:
			result <- fmt.Sprintf("sınama 5 ms içinde kalmadı (%v)", since)
		default:
			result <- ""
		}
		waiter.pending.Add(-1)
		waiter.releaseGIL()
	}()
	timer := <-armed
	old := machine.gilGen
	machine.releaseGIL() // kilit bekleyene geçer: kuşak 2
	timer()              // eski kuşağın zamanlayıcısı geç ateşlenir
	close(fired)
	if msg := <-result; msg != "" {
		t.Fatalf("%s (eski kuşak %d)", msg, old)
	}
}

// slowTurn yavaş() yerleşiğinin süresidir: kilidi tutarak yaklaşık bu kadar çalışır, içinde deyim
// sınırı yoktur. Süre saate bakılarak değil sabit bir hesapla tutulur (spinFor): Windows'ta
// tekdüze saat 0,5–2 ms adımla ilerler, saate bakan bir bekleme bir adımdan kısa sürmez.
const slowTurn = time.Millisecond

var spinSink uint64

// spin n adımlık sabit bir hesap yapar.
func spin(n int) {
	x := spinSink | 1
	for i := 0; i < n; i++ {
		x = x*6364136223846793005 + 1442695040888963407
	}
	spinSink = x
}

// spinFor spin'in d süren adım sayısını ölçer: uzun bir ölçümle (saatin adımı ölçüyü bozmasın).
func spinFor(d time.Duration) int {
	const probe = 1 << 25
	start := monotonic()
	spin(probe)
	return max(1, int(float64(probe)*float64(d)/float64(monotonic()-start)))
}

// slowPhase, slowPhaseWaits'in ölçümüdür: yavaş evre başladıktan sonra beklenerek alınan her
// kilidin önündeki bekleyen sayısı, bekleme süresi ve alındığı an; calls yavaş() çağrılarının
// başladığı anlar.
type slowPhase struct {
	aheads      []int
	waits, ends []time.Duration
	calls       []time.Duration
}

// slowPhaseWaits iki meşgul görev çalıştırır: biri hep hızlı bir döngüde; öbürü altı kez önce
// 300 000 hızlı tur (yedek saatin sayısı büyüsün), sonra yavaş() çağıran 200 tur. Yavaş turda tek
// deyim sınırı vardır (tur başı), yani 64 sınır 64 turdur. Her geçiş, öbürü kilidi beklerken bir
// tutuşun ortasında olur; geçişte saatin bir sonraki okumasına kalan sınır sayısı rastgeledir,
// altı geçiş onu tarar. setup zamanlayıcıyı ya da yedek saati kapatır.
func slowPhaseWaits(t *testing.T, setup func(*VM)) slowPhase {
	t.Helper()
	code := "TR_var dur = TR_false\nTR_var bitti = TR_false\n" +
		"TR_async {\n    TR_for evre TR_in 0..6 {\n        TR_var s = 0\n        TR_for s < 300000 {\n            s += 1\n        }\n" +
		"        TR_for i TR_in 0..200 {\n            yavaş()\n        }\n    }\n    bitti = TR_true\n}\n" +
		"TR_async {\n    TR_var b = 0\n    TR_for TR_not dur {\n        b += 1\n    }\n}\n" +
		"TR_for TR_not bitti {\n    TR_sleep(0.01)\n}\ndur = TR_true\nTR_wait_all()\n"
	machine := New()
	machine.Out = io.Discard
	setup(machine)
	steps := spinFor(slowTurn)
	var m slowPhase
	var slow atomic.Bool // yavaş evre başladı mı
	machine.RegisterBuiltin("yavaş", func(*VM, []Value, map[string]Value) (Value, error) {
		m.calls = append(m.calls, monotonic()) // yalnız yavaş görev yazar
		slow.Store(true)
		spin(steps)
		return Null, nil
	})
	var mu sync.Mutex
	machine.gilWaitHook = func(ahead int, _ uint64, waited time.Duration) {
		if slow.Load() {
			mu.Lock()
			m.aheads, m.waits, m.ends = append(m.aheads, ahead), append(m.waits, waited), append(m.ends, monotonic())
			mu.Unlock()
		}
	}
	_, err := machine.Run(mustParseProgram(t, code))
	machine.WaitAsync()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.waits) < 10 {
		t.Fatalf("yavaş evrelerde kilit yalnız %d kez beklenerek alındı", len(m.waits))
	}
	return m
}

// fallbackTurns yedek saatin sözüdür: saat en çok 64 deyim sınırında bir okunur, yani bir tutuş
// süresi dolduktan sonra en çok 64 yavaş tur sürer. Testte ayrı yazılır: gil.go'daki tavan
// (clockEveryMax) büyüyünce sınır da büyümesin, test düşsün.
const fallbackTurns = 64

// slowPhaseOverLimit bir koşunun yavaş evrelerinde sınırı aşan beklemeleri verir ve ölçümü testin
// çıktısına yazar. Zamanlayıcıyla sınır (n + 1) × 5 ms + paydır. Yalnız yedek saatle sınır turla
// sayılır: bir bekleme sürerken yavaş görev en çok bir kez tutar (kuyrukta bir kez bulunur); o
// tutuşta kilidi aldığı sınırdan sonra bir çağrı yapar, sonra saat en çok 64 sınırda bir okunur;
// bekleme içindeki yavaş() çağrıları 64 + 1'i geçmez (65 yarış altında ölçüldü, sınırın kendisi).
// Turla saymak makineden, saatin adımından ve yarış dedektöründen bağımsızdır; süreyle (64 × tur)
// sınamak, tutuştaki turlar ortalamadan uzun sürdüğü için sınırda kalıyordu.
func slowPhaseOverLimit(t *testing.T, name string, setup func(*VM), fallback bool, run int) []string {
	t.Helper()
	m := slowPhaseWaits(t, setup)
	var over []string
	var worst time.Duration
	most := 0
	for i, w := range m.waits {
		n := 0
		for _, c := range m.calls {
			if c > m.ends[i]-w && c < m.ends[i] {
				n++
			}
		}
		most = max(most, n)
		worst = max(worst, w)
		if fallback {
			if n > fallbackTurns+1 {
				over = append(over, fmt.Sprintf("önünde %d bekleyenle %v beklendi, bu sürede %d yavaş tur, sınır %d", m.aheads[i], w, n, fallbackTurns+1))
			}
		} else if limit := time.Duration(m.aheads[i]+1)*lockHold + lockWaitSlack; w > limit {
			over = append(over, fmt.Sprintf("önünde %d bekleyenle %v beklendi, sınır %v", m.aheads[i], w, limit))
		}
	}
	t.Logf("%s, %s, %d. koşu: %d bekleme, sınırı aşan %d, en uzun %v, bir beklemede en çok %d yavaş tur (%d çağrı)",
		runtime.GOOS, name, run, len(m.waits), len(over), worst, most, len(m.calls))
	return over
}

// Geçiş (10.2): hızlı bir döngüden sonra deyimler yavaşlayınca (her tur ~1 ms) bekleyen yine
// kilidi alır; devir, geçmiş deyimlerin hızından ayarlanan bir sayıya (k × yavaş tur) kalmaz.
// Zamanlayıcıyla süre sınanır: (n + 1) × 5 ms + pay. Yalnız yedek saatle (zamanlayıcı geç kalırsa
// sözü o tutar) tavan sınanır: bir beklemede en çok 64 + 1 yavaş tur. Tavansız k hızlı döngüde
// yüzlere çıkar ve altı geçişten birinde sınırı aşar (makine yavaşken sayı 64'ü geçmeyebilir; o
// koşuda tavan zaten bağlamaz). Süre için Windows'ta üç koşudan biri, Linux'ta tek koşu sınıra
// sığmalıdır; tur sayısı her yerde tek koşuda kesin sınanır.
func TestTheLockIsHandedOffInTimeAfterStatementsSlowDown(t *testing.T) {
	runs := 3
	if runtime.GOOS == "linux" {
		runs = 1
	}
	modes := []struct {
		name     string
		setup    func(*VM)
		fallback bool
	}{
		{"zamanlayıcıyla", func(*VM) {}, false},
		{"yalnız yedek saatle", func(m *VM) { m.gil.timerOff = true }, true},
	}
	for _, mode := range modes {
		modeRuns := runs
		if mode.fallback {
			modeRuns = 1 // tur sayısı zamanlamaya bağlı değil: her koşuda kesin
		}
		within := false
		var last []string
		for run := 1; run <= modeRuns && !within; run++ {
			last = slowPhaseOverLimit(t, mode.name, mode.setup, mode.fallback, run)
			within = len(last) == 0
		}
		if !within {
			t.Errorf("%s: %d koşunun hiçbiri sınıra sığmadı; sonuncusu:\n%s", mode.name, modeRuns, strings.Join(last, "\n"))
		}
	}
}

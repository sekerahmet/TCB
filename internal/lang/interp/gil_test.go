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

// slowTurn yavaş() yerleşiğinin süresidir: kilidi tutarak bu kadar çalışır, içinde deyim sınırı
// yoktur.
const slowTurn = 250 * time.Microsecond

// slowPhaseWaits iki meşgul görev çalıştırır: biri hep hızlı bir döngüde, öbürü 200 000 hızlı
// turdan sonra her turu yavaş bir kullanıcı fonksiyonu olan bir döngüde; ana görev geçişten sonra
// 0,3 s bekler. Geçiş, öbürü kilidi beklerken bir tutuşun ortasında olur. Yavaş evre başladıktan
// sonra alınan kilitlerin beklemelerini döndürür (geçişte süren bekleme dahil). setup
// zamanlayıcıyı ya da yedek saati kapatır.
func slowPhaseWaits(t *testing.T, setup func(*VM)) (aheads []int, waits []time.Duration) {
	t.Helper()
	code := "TR_var dur = TR_false\nTR_var sayaç = 0\n" +
		"TR_func yavaşla() {\n    yavaş()\n}\n" +
		"TR_async {\n    TR_for sayaç < 200000 {\n        sayaç += 1\n    }\n    TR_for TR_not dur {\n        yavaşla()\n    }\n}\n" +
		"TR_async {\n    TR_var b = 0\n    TR_for TR_not dur {\n        b += 1\n    }\n}\n" +
		"TR_for sayaç < 200000 {\n    TR_sleep(0.01)\n}\nTR_sleep(0.3)\ndur = TR_true\nTR_wait_all()\n"
	machine := New()
	machine.Out = io.Discard
	setup(machine)
	var slowSince atomic.Int64 // yavaş evrenin başladığı an (monotonic; 0: başlamadı)
	machine.RegisterBuiltin("yavaş", func(*VM, []Value, map[string]Value) (Value, error) {
		start := monotonic()
		slowSince.CompareAndSwap(0, int64(start))
		for monotonic()-start < slowTurn {
		}
		return Null, nil
	})
	var mu sync.Mutex
	machine.gilWaitHook = func(ahead int, _ uint64, waited time.Duration) {
		if slowSince.Load() != 0 {
			mu.Lock()
			aheads, waits = append(aheads, ahead), append(waits, waited)
			mu.Unlock()
		}
	}
	_, err := machine.Run(mustParseProgram(t, code))
	machine.WaitAsync()
	if err != nil {
		t.Fatal(err)
	}
	if len(waits) < 3 {
		t.Fatalf("yavaş evrede kilit yalnız %d kez beklenerek alındı", len(waits))
	}
	return aheads, waits
}

// slowPhaseOverLimit bir koşunun yavaş evresinde (n + 1) × 5 ms + payı aşan beklemeleri verir ve
// ölçümü testin çıktısına yazar.
func slowPhaseOverLimit(t *testing.T, name string, setup func(*VM), run int) []string {
	t.Helper()
	aheads, waits := slowPhaseWaits(t, setup)
	var over []string
	var worst time.Duration
	for i, w := range waits {
		if limit := time.Duration(aheads[i]+1)*lockHold + lockWaitSlack; w > limit {
			over = append(over, fmt.Sprintf("önünde %d bekleyenle %v beklendi, sınır %v", aheads[i], w, limit))
		}
		worst = max(worst, w)
	}
	t.Logf("%s, %s, %d. koşu: yavaş evrede %d bekleme, sınırı aşan %d, en uzun %v", runtime.GOOS, name, run, len(waits), len(over), worst)
	return over
}

// Geçiş (10.2): hızlı bir döngüden sonra deyimler yavaşlayınca (her tur ~0,25 ms) bekleyen yine
// (n + 1) × 5 ms + pay içinde alır; devir, geçmiş deyimlerin hızından ayarlanan bir sayıya
// (k × yavaş aralık) kalmaz. Zamanlayıcıyla ve yalnız yedek saatle (zamanlayıcı geç kalırsa sözü
// o tutar; k ≤ 64, en çok 64 × 0,25 ms = 16 ms; tavansız k hızlı döngüde yüzlere çıkar ve sınırı
// aşar). Windows'ta üç koşudan biri, Linux'ta tek koşu sınıra sığmalıdır.
func TestTheLockIsHandedOffInTimeAfterStatementsSlowDown(t *testing.T) {
	runs := 3
	if runtime.GOOS == "linux" {
		runs = 1
	}
	modes := []struct {
		name  string
		setup func(*VM)
	}{
		{"zamanlayıcıyla", func(*VM) {}},
		{"yalnız yedek saatle", func(m *VM) { m.gil.timerOff = true }},
	}
	for _, mode := range modes {
		within := false
		var last []string
		for run := 1; run <= runs && !within; run++ {
			last = slowPhaseOverLimit(t, mode.name, mode.setup, run)
			within = len(last) == 0
		}
		if !within {
			t.Errorf("%s: %d koşunun hiçbiri sınıra sığmadı; sonuncusu:\n%s", mode.name, runs, strings.Join(last, "\n"))
		}
	}
}

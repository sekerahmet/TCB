package interp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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
func TestLockIsTakenInArrivalOrderWithinTheDelay(t *testing.T) {
	for _, busy := range []int{2, 3} {
		aheads, tickets, waits, turns := lockWaits(t, busy)
		if len(tickets) < 20 {
			t.Fatalf("%d meşgul görev: kilit yalnız %d kez beklenerek alındı", busy, len(tickets))
		}
		var worst time.Duration
		for i, w := range waits {
			if i > 0 && tickets[i] <= tickets[i-1] {
				t.Fatalf("%d meşgul görev: kilit geliş sırasıyla verilmedi: %d. numara %d'den sonra", busy, tickets[i], tickets[i-1])
			}
			limit := time.Duration(aheads[i]+1)*lockHold + lockWaitSlack
			if w > limit {
				t.Errorf("%d meşgul görev: önünde %d bekleyenle %v beklendi, sınır %v", busy, aheads[i], w, limit)
			}
			worst = max(worst, w-time.Duration(aheads[i]+1)*lockHold)
		}
		if os.Getenv("TR_LOCK_WAIT_REPORT") != "" {
			t.Logf("%d meşgul görev: %d bekleme, sınırın en çok %v üstünde (eksi: altında); turlar %s", busy, len(waits), worst, turns)
		}
		if strings.Contains(turns, "[0") || strings.Contains(turns, " 0") {
			t.Errorf("%d meşgul görev: her görev ilerlemeliydi: %s", busy, turns)
		}
	}
}

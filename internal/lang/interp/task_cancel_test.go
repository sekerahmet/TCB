package interp

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Kural (10.9): g.TR_cancel() görevi bir sonraki güvenli noktada durdurur: süren bekleme
// bırakılır, döngü turunda durur. İptal görevin içinde TR_catch ile yakalanmaz, ertelenen işleri
// çalışır. Bekleyen görev durunca iptal hatasını (H2920) alır, g.TR_done() ancak o zaman doğrudur.
// Bitmiş görevi iptal etmenin ve ikinci iptalin etkisi yoktur; iptal bir görev hatası olarak
// yazılmaz (10.6).
func TestACancelledTaskStopsAtItsSafePoint(t *testing.T) {
	const source = `TR_var iz = []
TR_func uzun() -> TR_Int {
    TR_defer { iz.TR_append("ertele") }
    TR_try {
        TR_sleep(5)
    } TR_catch e {
        iz.TR_append("yakalandı")
    }
    TR_return 1
}
TR_var g = TR_async uzun()
TR_sleep(0.02)
g.TR_cancel()
g.TR_cancel()
TR_print(TR_error_code { g.TR_wait() }, g.TR_done(), iz)
TR_var d = TR_async {
    TR_var i = 0
    TR_for TR_true { i += 1 }
}
TR_sleep(0.02)
d.TR_cancel()
TR_print(TR_error_code { d.TR_wait() })
TR_var b = TR_async TR_sum([2, 3])
TR_print(b.TR_wait())
b.TR_cancel()
TR_print(b.TR_wait())
TR_var s = TR_async TR_sum([1])
s.TR_cancel()
TR_print(TR_error_code { s.TR_wait() })
`
	prog := mustParseProgram(t, source)
	var outBuf, failureBuf bytes.Buffer
	machine := New()
	machine.Out, machine.Err = &outBuf, &failureBuf
	_, err := machine.Run(prog)
	out, failure := outBuf.String(), failureBuf.String()
	const want = "H2920 true [\"ertele\"]\nH2920\n5\n5\nH2920\n"
	if err != nil || out != want || failure != "" {
		t.Fatalf("%q beklenirdi, gelen %q (%v; hata çıktısı %q)", want, out, err, failure)
	}
}

// Kural (10.9): iptal görevin başlattığı ve bitmemiş görevlere geçer; görev önce alt görevlerinin
// durmasını bekler, sonra ertelenen işleri çalışır. Hatasız biten görev de alt görevlerini bekler.
func TestCancelReachesTheTasksChildren(t *testing.T) {
	const source = `TR_var iz = []
TR_var alt: TR_List[TR_Task[TR_NoneType]] = []
TR_var ana = TR_async {
    alt.TR_append(TR_async { TR_sleep(5) })
    TR_defer { iz.TR_append("ana ertele \(alt[0].TR_done())") }
    TR_sleep(5)
}
TR_sleep(0.02)
ana.TR_cancel()
TR_print(TR_error_code { ana.TR_wait() }, TR_error_code { alt[0].TR_wait() }, iz)
TR_var bekleyen = TR_async {
    TR_async { TR_sleep(0.03); iz.TR_append("alt bitti") }
    TR_defer { iz.TR_append("üst ertele") }
}
bekleyen.TR_wait()
TR_print(iz)
`
	out, _, err := runTROut(t, source)
	const want = "H2920 H2920 [\"ana ertele true\"]\n[\"ana ertele true\", \"alt bitti\", \"üst ertele\"]\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Kural (9.4, 1.5, 10.9): yığın durdurma ya da iptalle çözülürken blokların ve fonksiyonların
// ertelenen işleri çalışır; verdikleri hata TR_catch ile yakalanmaz ama kaybolmaz: iptalde bekleyenin
// aldığı iptal hatasının TR_suppressed'ine, durdurmada programın hatalarına eklenir (çıkış kodu 1).
func TestDefersKeepTheirErrorsWhileUnwinding(t *testing.T) {
	const source = `TR_var iz = []
TR_func iç() {
    TR_defer { TR_throw "ertele patladı" }
    TR_if TR_true {
        TR_defer { iz.TR_append("blok") }
        TR_sleep(5)
    }
}
TR_var g = TR_async {
    TR_try { iç() } TR_catch e { iz.TR_append("yakalandı") }
}
TR_sleep(0.02)
g.TR_cancel()
TR_try { g.TR_wait() } TR_catch e { TR_print(e.TR_code, e.TR_suppressed, iz) }
`
	out, _, err := runTROut(t, source)
	const want = "H2920 [ertele patladı] [\"blok\"]\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}

	prog := mustParseProgram(t, `TR_var iz = []
TR_func iş() {
    TR_defer { TR_throw "ertele patladı" }
    TR_if TR_true {
        TR_defer { iz.TR_append("blok") }
        hazır()
        TR_for { }
    }
}
iş()
`)
	vmx := New()
	var failure bytes.Buffer
	vmx.Err = &failure
	ready := make(chan struct{})
	vmx.RegisterBuiltin("hazır", func(*VM, []Value, map[string]Value) (Value, error) {
		close(ready)
		return Null, nil
	})
	go func() {
		<-ready
		time.Sleep(5 * time.Millisecond)
		vmx.Interrupt()
	}()
	_, runErr := vmx.Run(prog)
	end := vmx.End(runErr)
	traces, _ := vmx.GlobalEnv.Get("iz")
	if end.Code != 1 || !strings.Contains(fmt.Sprint(end.Errors), "ertele patladı") || traces.Inspect() != `["blok"]` {
		t.Fatalf("durdurmada ertelenen işin hatası yazılmalı ve kod 1 olmalı: %+v, iz %s", end, traces.Inspect())
	}
}

// Kural (10.7): g.TR_wait(TR_timeout: s) en çok s saniye bekler. Süresinde biten görev sonucunu
// verir; süre dolarsa görev iptal edilir, durması beklenir (ertelenen işi çalışır) ve zaman aşımı
// hatası (H2921) fırlar; aynı görevi sonra bekleyen iptal hatasını alır. Süre uyu()'nunki gibi
// denetlenir (H2433).
func TestAWaitWithATimeout(t *testing.T) {
	const source = `TR_func kare(x: TR_Int) -> TR_Int {
    TR_sleep(0.01)
    TR_return x * x
}
TR_var g = TR_async kare(3)
TR_print(g.TR_wait(TR_timeout: 1))
TR_var iz = []
TR_var u = TR_async {
    TR_defer { iz.TR_append("ertele") }
    TR_sleep(10)
}
TR_print(TR_error_code { u.TR_wait(TR_timeout: 0.02) }, u.TR_done(), iz)
TR_print(TR_error_code { u.TR_wait(TR_timeout: 1) })
TR_print(TR_error_code { g.TR_wait(TR_timeout: -1) }, TR_error_code { g.TR_wait(TR_timeout: "bir") })
`
	out, _, err := runTROut(t, source)
	const want = "9\nH2921 true [\"ertele\"]\nH2920\nH2433 H2433\n"
	if err != nil || out != want {
		t.Fatalf("%q beklenirdi, gelen %q (%v)", want, out, err)
	}
}

// Kural (10.7, 10.9): ağır turlu döngüdeki görev de süre dolunca iptal edilir; iptal döngünün bir
// sonraki turunda fark edilir, bekleme süreyi az aşar. Süre tekdüze saatle (time.Since) ölçülür;
// Windows'ta üç koşudan biri sınıra sığmalıdır (gil_test.go'nun kuralı), Linux'ta tek koşu.
func TestATimeoutStopsABusyLoopSoon(t *testing.T) {
	const source = `TR_var d = TR_async {
    TR_var i = 0
    TR_for TR_true { i += 1 }
}
TR_print(TR_error_code { d.TR_wait(TR_timeout: 0.05) })
`
	runs := 3
	if runtime.GOOS == "linux" {
		runs = 1
	}
	const limit = 50*time.Millisecond + 250*time.Millisecond
	var took []time.Duration
	for run := 1; run <= runs; run++ {
		start := time.Now()
		out, _, err := runTROut(t, source)
		elapsed := time.Since(start)
		if err != nil || out != "H2921\n" {
			t.Fatalf("H2921 beklenirdi, gelen %q (%v)", out, err)
		}
		t.Logf("%d. koşu: %v", run, elapsed)
		if elapsed >= 50*time.Millisecond && elapsed <= limit {
			return
		}
		took = append(took, elapsed)
	}
	t.Fatalf("%d koşunun hiçbiri 50 ms ile %v arasında bitmedi: %v", runs, limit, took)
}

// Kural (10.9, 10.7): gövdesi bitmiş, alt görevini bekleyen görevin iptali de alt göreve geçer:
// bekleme alt görevin kendiliğinden bitmesini beklemez. Süre sınırlı bekleme sınır kadar, düz iptal
// hemen biter; alt görev 5 saniye uyur. Süre tekdüze saatle (time.Since) ölçülür.
func TestCancelReachesAFinishedBodysChildren(t *testing.T) {
	const timeout = `TR_var ana = TR_async {
    TR_async { TR_sleep(5) }
}
TR_sleep(0.02)
TR_print(TR_error_code { ana.TR_wait(TR_timeout: 0.1) })
`
	const cancel = `TR_var ana = TR_async {
    TR_async { TR_sleep(5) }
}
TR_sleep(0.02)
ana.TR_cancel()
TR_print(TR_error_code { ana.TR_wait() })
`
	for _, c := range []struct{ name, source, want string }{
		{"süre sınırı", timeout, "H2921\n"},
		{"iptal", cancel, "H2920\n"},
	} {
		start := time.Now()
		out, _, err := runTROut(t, c.source)
		elapsed := time.Since(start)
		if err != nil || out != c.want {
			t.Fatalf("%s: %q beklenirdi, gelen %q (%v)", c.name, c.want, out, err)
		}
		if elapsed > 2*time.Second {
			t.Fatalf("%s: bekleme alt görevin kendi bitişini bekledi: %v", c.name, elapsed)
		}
		t.Logf("%s: %v", c.name, elapsed)
	}
}

// Kural (10.9): uzun ömürlü görevin alt görev listesi bitmiş alt görevlerle büyümez: liste dolunca
// bitmişler budanır. Görev her turda biten bir alt görev başlatır; listede birkaç görev kalır.
func TestAFinishedChildIsNotKept(t *testing.T) {
	prog := mustParseProgram(t, `TR_var ana = TR_async {
    TR_for i TR_in 0..200 {
        TR_async { }
        TR_sleep(0.001)
    }
    TR_print(altlar() < 64)
}
ana.TR_wait()
`)
	machine := New()
	var out bytes.Buffer
	machine.Out = &out
	machine.RegisterBuiltin("altlar", func(y *VM, _ []Value, _ map[string]Value) (Value, error) {
		y.task.mu.Lock()
		defer y.task.mu.Unlock()
		return NewInt(int64(len(y.task.children))), nil
	})
	if _, err := machine.Run(prog); err != nil || out.String() != "true\n" {
		t.Fatalf("liste budanmalıydı: %q (%v)", out.String(), err)
	}
}

// Kural (10.6, 10.5): TR_wait_all(görevler) listedeki görevleri ancak bütün liste beklenince
// beklenmiş sayar. Bekleyen görev yarıda iptal edilirse listenin hatalı biten görevi beklenmemiş
// kalır: hatası program biterken yazılır, çıkış kodu 1.
func TestAnAbandonedWaitAllKeepsTheErrors(t *testing.T) {
	prog := mustParseProgram(t, `TR_func hata() -> TR_Int { TR_throw "a hatası" }
TR_var a = TR_async hata()
TR_var b = TR_async { TR_sleep(10) }
TR_var w = TR_async { TR_wait_all([a, b]) }
TR_sleep(0.05)
w.TR_cancel()
b.TR_cancel()
TR_print(TR_error_code { w.TR_wait() })
`)
	var outBuf, failure bytes.Buffer
	machine := New()
	machine.Out, machine.Err = &outBuf, &failure
	_, err := machine.Run(prog)
	end := machine.End(err)
	if outBuf.String() != "H2920\n" || end.Code != 1 || !strings.Contains(failure.String(), "a hatası") {
		t.Fatalf("a'nın hatası program sonunda yazılmalıydı: çıktı %q, kod %d, hata çıktısı %q", outBuf.String(), end.Code, failure.String())
	}
}

package interp

import (
	"bytes"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Kes programı bir sonraki güvenli noktada durdurur: Run ErrKesildi döner, dene/yakala
// kesilmeyi yakalamaz, ertelenen işler (fonksiyon, eşzamanlı görev, program) çalışır,
// eşzamanlı görevin kesilmesi hata olarak bildirilmez.
func TestInterrupt(t *testing.T) {
	codeVal := `
TR_var izler = []
TR_defer { izler.TR_append("program") }
TR_async {
    TR_defer { izler.TR_append("görev") }
    görev_hazır()
    TR_for { }
}
TR_func iş() {
    TR_defer { izler.TR_append("fonksiyon") }
    TR_try {
        hazır()
        TR_for { }
    } TR_catch h {
        izler.TR_append("yakalandı")
    }
}
iş()
izler.TR_append("sürdü")
`
	prog := mustParseProgram(t, codeVal)
	vmx := New()
	var failure bytes.Buffer
	vmx.Err = &failure
	// Kesme iki yer de ertelenen işini kaydettikten sonra gelir: görev TR_defer'e varmadan
	// kesilirse ertelenecek işi yoktur, testin sorusu bu değil (CI'da ve yerelde %1–5 düşüyordu).
	ready, taskReady := make(chan struct{}), make(chan struct{})
	vmx.RegisterBuiltin("hazır", func(*VM, []Value, map[string]Value) (Value, error) {
		close(ready)
		return Null, nil
	})
	vmx.RegisterBuiltin("görev_hazır", func(*VM, []Value, map[string]Value) (Value, error) {
		close(taskReady)
		return Null, nil
	})
	go func() {
		<-ready
		<-taskReady
		time.Sleep(5 * time.Millisecond)
		vmx.Interrupt()
	}()
	_, err := vmx.Run(prog)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("ErrKesildi bekleniyordu: %v", err)
	}
	vmx.WaitAsync()
	traces, _ := vmx.GlobalEnv.Get("izler")
	text := traces.Inspect()
	for _, iz := range []string{`"fonksiyon"`, `"program"`, `"görev"`} {
		if !strings.Contains(text, iz) {
			t.Errorf("%s ertelenen işi çalışmadı: %s", iz, text)
		}
	}
	for _, iz := range []string{"yakalandı", "sürdü"} {
		if strings.Contains(text, iz) {
			t.Errorf("kesilmeden sonra %q olmamalıydı: %s", iz, text)
		}
	}
	if failure.Len() > 0 {
		t.Errorf("görevin kesilmesi hata olarak bildirilmemeli: %s", failure.String())
	}
	if !vmx.Interrupted() || vmx.Interrupt() {
		t.Error("Kes yalnızca ilk çağrıda doğru dönmeli")
	}
}

// İlk kesme isteği çalışan bir yerleşiği yarıda bırakmaz (dil tanımı 1.8): istek hesabın
// ortasında gelse de yerleşik işini bitirip döner, program bir sonraki güvenli noktada
// (fonksiyon çağrısı) durur.
func TestInterruptWaitsForBuiltin(t *testing.T) {
	prog := mustParseProgram(t, `
TR_var izler = []
TR_func sür() { izler.TR_append("sürdü") }
izler.TR_append(uzun())
sür()
`)
	vmx := New()
	finished := false
	vmx.RegisterBuiltin("uzun", func(v *VM, _ []Value, _ map[string]Value) (Value, error) {
		v.Interrupt()   // istek hesabın ortasında gelir
		finished = true // hesabın geri kalanı
		return &StringVal{Val: "bitti"}, nil
	})
	if _, err := vmx.Run(prog); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("ErrKesildi bekleniyordu: %v", err)
	}
	if !finished {
		t.Error("yerleşik kesme isteğiyle yarıda kaldı")
	}
	traces, _ := vmx.GlobalEnv.Get("izler")
	if text := traces.Inspect(); strings.Contains(text, "sürdü") {
		t.Errorf("program fonksiyon çağrısında durmalıydı: %s", text)
	}
}

// Döngü turu güvenli noktadır (1.5): gövdesi yalnız yerleşik çağıran bir döngü de durdurmayı
// bir sonraki turda uygular; uzun süren turlarda gecikme tur sayısıyla büyümez.
func TestInterruptStopsALoopOfBuiltinsAtTheNextTurn(t *testing.T) {
	prog := mustParseProgram(t, `
TR_var tur = 0
TR_for i TR_in 1..1000 {
    tur = i
    adım(i)
}
`)
	vmx := New()
	vmx.RegisterBuiltin("adım", func(v *VM, args []Value, _ map[string]Value) (Value, error) {
		if n, _ := args[0].(*IntVal); n != nil && n.Val == 3 {
			v.Interrupt()
		}
		return Null, nil
	})
	if _, err := vmx.Run(prog); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("ErrKesildi bekleniyordu: %v", err)
	}
	if turn, _ := vmx.GlobalEnv.Get("tur"); turn.Inspect() != "3" {
		t.Errorf("döngü durdurmanın geldiği turdan sonraki turda durmalıydı; son tur %s", turn.Inspect())
	}
}

// RequestStop isteği dört yoldan karşılar (1.5, 1.6, 1.15) ve ilk sinyali korur (1.9).
func TestRequestStopDispositions(t *testing.T) {
	v := New()
	if d := v.RequestStop(2); d != StopApplied || !v.Interrupted() || v.StopSignal() != 2 {
		t.Fatalf("erteleyen ve üstlenen yokken istek hemen uygulanmalıydı: %v kesildi=%v sinyal=%d", d, v.Interrupted(), v.StopSignal())
	}
	if d := v.RequestStop(15); d != StopRepeated || v.StopSignal() != 2 {
		t.Errorf("ikinci istek ilk sinyali değiştirmemeli: %v sinyal=%d", d, v.StopSignal())
	}

	v = New()
	v.HandleStop()
	if d := v.RequestStop(15); d != StopHandled || v.Interrupted() || !v.StopRequested() {
		t.Errorf("üstlenen programda istek kaydedilmeli, program durmamalı: %v kesildi=%v", d, v.Interrupted())
	}

	v = New()
	v.DeferStops(func() (time.Duration, bool) { return time.Hour, true })
	if d := v.RequestStop(2); d != StopDeferred || v.Interrupted() || !v.StopRequested() {
		t.Errorf("kontrol noktası isteği ertelemeliydi: %v kesildi=%v", d, v.Interrupted())
	}

	v = New()
	v.DeferStops(func() (time.Duration, bool) { return 0, false })
	if d := v.RequestStop(2); d != StopApplied || !v.Interrupted() {
		t.Errorf("ikinci güncelleme gelmeden istek ertelenmemeli: %v", d)
	}

	v = New()
	v.HandleStop()
	v.DeferStops(func() (time.Duration, bool) { return time.Hour, true })
	if d := v.RequestStop(2); d != StopHandled || v.Interrupted() {
		t.Errorf("üstlenen programda kontrol noktası yalnız kaydeder, ertelemez: %v", d)
	}

	v = New()
	v.finishing.Store(true)
	if d := v.RequestStop(2); d != StopFinishing || v.Interrupted() || v.StopSignal() != 2 {
		t.Errorf("biterken gelen istek ertelenen işleri kesmemeli: %v kesildi=%v", d, v.Interrupted())
	}
}

// Erteleme süresi dolunca istek 1.5'teki gibi uygulanır; bekleyen bir görev o anda uyanır (1.6).
func TestDeferredStopExpires(t *testing.T) {
	v := New()
	v.DeferStops(func() (time.Duration, bool) { return 30 * time.Millisecond, true })
	if d := v.RequestStop(2); d != StopDeferred {
		t.Fatalf("istek ertelenmeliydi: %v", d)
	}
	select {
	case <-v.interruptChan:
	case <-time.After(10 * time.Second):
		t.Fatal("erteleme süresi dolunca istek uygulanmalıydı")
	}
	if !v.Interrupted() {
		t.Error("erteleme süresi dolunca program durdurulmalıydı")
	}
}

// stopAfter, program hazır yerleşiğine gelince kısa süre sonra signal numaralı isteği gönderir.
func stopAfter(v *VM, signal int) {
	ready := make(chan struct{})
	v.RegisterBuiltin("hazır", func(*VM, []Value, map[string]Value) (Value, error) {
		close(ready)
		return Null, nil
	})
	go func() {
		<-ready
		time.Sleep(10 * time.Millisecond)
		v.RequestStop(signal)
	}()
}

// Durdurmayı üstlenen programda istek geldikten sonra bekleyen yerleşik "durdurma istendi"
// hatasıyla döner; bu hatayı yakalayan yakala gövdesinde ve ertele bloğunda bekleme normaldir;
// program sona gelince süreç isteğin sinyaliyle biter (1.6, 1.9).
func TestHandledStopTurnsWaitsIntoErrors(t *testing.T) {
	v := New()
	stopAfter(v, 15)
	prog := mustParseProgram(t, `# TR 4
TR_handle_stop()
TR_var izler = []
TR_defer { TR_sleep(0.05); izler.TR_append("ertele bekledi") }
hazır()
TR_try { TR_sleep(60) } TR_catch e { TR_sleep(0.05); izler.TR_append("yakala bekledi") }
TR_var istendi = TR_stop_requested()
TR_try { TR_sleep(0.01) } TR_catch e { izler.TR_append("yeni bekleme hata verdi") }
`)
	start := time.Now()
	_, err := v.Run(prog)
	if err != nil {
		t.Fatalf("üstlenen program hatasız bitmeli: %v", err)
	}
	if time.Since(start) > 30*time.Second {
		t.Error("uyku istek gelince uyanmalıydı")
	}
	traces, _ := v.GlobalEnv.Get("izler")
	for _, s := range []string{"yakala bekledi", "yeni bekleme hata verdi", "ertele bekledi"} {
		if !strings.Contains(traces.Inspect(), s) {
			t.Errorf("%q bekleniyordu: %s", s, traces.Inspect())
		}
	}
	if asked, _ := v.GlobalEnv.Get("istendi"); asked != True {
		t.Errorf("durdurma_istendi() istek gelince doğru dönmeli: %v", asked)
	}
	if end := v.End(err); end.Signal != 15 || end.Code != 0 || len(end.Errors) != 0 {
		t.Errorf("sona gelen üstlenmiş program isteğin sinyaliyle bitmeli: %+v", end)
	}
}

// Durdurmayı üstlenmemiş programda istek bekleyen yerleşiği bırakır; ertele bloğundaki bekleme
// bırakılmaz, tamamlanır (1.5).
func TestUnhandledStopAbandonsWaitsButNotInDefer(t *testing.T) {
	v := New()
	stopAfter(v, 2)
	prog := mustParseProgram(t, `# TR 4
TR_var izler = []
TR_defer { TR_sleep(0.2); izler.TR_append("ertele bekledi") }
hazır()
TR_sleep(60)
izler.TR_append("sürdü")
`)
	start := time.Now()
	_, err := v.Run(prog)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("ErrInterrupted bekleniyordu: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond || elapsed > 30*time.Second {
		t.Errorf("uyku bırakılmalı (30 sn'den kısa), ertele bloğunun beklemesi tamamlanmalı (0,15 sn'den uzun): %v", elapsed)
	}
	traces, _ := v.GlobalEnv.Get("izler")
	if text := traces.Inspect(); !strings.Contains(text, "ertele bekledi") || strings.Contains(text, "sürdü") {
		t.Errorf("ertele beklemesi bitmeli, program sürmemeli: %s", text)
	}
	if end := v.End(err); end.Signal != 2 || len(end.Errors) != 0 {
		t.Errorf("durdurulan program isteğin sinyaliyle bitmeli, hata yazılmamalı: %+v", end)
	}
}

// Bırakılabilir bir bekleme (dosya yükleme gibi) uygulanan durdurmayla biter; işin sonucu atılır (1.5).
func TestAbandonableWaitEndsWithTheStop(t *testing.T) {
	v := New()
	release := make(chan struct{})
	defer close(release)
	v.RegisterBuiltin("yükle", func(m *VM, _ []Value, _ map[string]Value) (Value, error) {
		if err := m.Abandonable(func() { <-release }); err != nil {
			return nil, err
		}
		return Null, nil
	})
	stopAfter(v, 2)
	prog := mustParseProgram(t, "# TR 4\nhazır()\nyükle()\nTR_print(\"sürdü\")\n")
	var out bytes.Buffer
	v.Out = &out
	if _, err := v.Run(prog); !errors.Is(err, ErrInterrupted) || out.Len() != 0 {
		t.Fatalf("bekleme bırakılmalı, program sürmemeli: %v %q", err, out.String())
	}
}

// InterruptNow programı o noktada durdurur: ertelenen işler çalışır, sonrası çalışmaz.
func TestInterruptNowStopsAtThePoint(t *testing.T) {
	v := New()
	v.GlobalEnv.Define("dur", &BuiltinVal{Name: "dur", Fn: func(m *VM, _ []Value, _ map[string]Value) (Value, error) {
		m.InterruptNow()
		return Null, nil
	}}, true)
	var out bytes.Buffer
	v.Out = &out
	prog := mustParseProgram(t, "# TR 4\nTR_defer { TR_print(\"ertelenen\") }\ndur()\nTR_print(\"sürdü\")\n")
	if _, err := v.Run(prog); !errors.Is(err, ErrInterrupted) || out.String() != "ertelenen\n" {
		t.Fatalf("InterruptNow programı o noktada durdurmalıydı: %v %q", err, out.String())
	}
}

// Kontrol noktası bir isteği ertelemişken program durdurmayı üstlenirse erteleme zamanlayıcısı
// kalkar: süre dolunca istek uygulanmaz, üstlenilmiş istek olarak kalır ve yeni bekleyen yerleşik
// "durdurma istendi" hatası verir (1.6).
func TestHandleStopCancelsTheDeferralTimer(t *testing.T) {
	v := New()
	v.DeferStops(func() (time.Duration, bool) { return 30 * time.Millisecond, true })
	if d := v.RequestStop(2); d != StopDeferred {
		t.Fatalf("istek ertelenmeliydi: %v", d)
	}
	v.HandleStop()
	time.Sleep(300 * time.Millisecond) // erteleme süresinin çok üstü
	if v.Interrupted() {
		t.Fatal("üstlenen programda erteleme süresi dolunca istek uygulanmamalı")
	}
	if !v.StopRequested() || v.stopWaitError("sleep") == nil {
		t.Errorf("ertelenen istek üstlenilmiş istek olmalı: istendi=%v", v.StopRequested())
	}
	select {
	case <-v.stopChan:
	default:
		t.Error("bekleyen yerleşiği uyandıran kanal kapanmalıydı")
	}
}

// reuseLoop aynı VM'de sonraki programdır: döngü turu güvenli noktadır, kesilmiş VM'de H2461 alır.
const reuseLoop = "TR_var s = 0\nTR_for i TR_in [1, 2, 3] {\n    s += i\n}\n"

// runReuseLoop reuseLoop'u v'de çalıştırır ve hatasını verir; hatasızsa toplamı da denetler.
func runReuseLoop(t *testing.T, v *VM) error {
	t.Helper()
	_, err := v.Run(mustParseProgram(t, reuseLoop))
	if err == nil {
		if s, _ := v.GlobalEnv.Get("s"); s.Inspect() != "6" {
			t.Errorf("döngülü program 6 vermeli: %v", s.Inspect())
		}
	}
	return err
}

// Yakalanmayan hatayla ya da TR_exit ile biten programın kesmesi (1.9, 1.10) programın kendisinindir:
// görevler durup ertelenen işler bitince kalkar, aynı VM'deki sonraki program kesilmiş başlamaz. Ertelenen
// işte açılan görev beklenmez; canlıyken kesme kalır ve görev hiç çalışmaz.
func TestErrorDoesNotStopTheNextProgram(t *testing.T) {
	for name, first := range map[string]string{
		"uncaught_error":    "TR_var x = [1]\nx[5]\n",
		"error_with_a_task": "TR_async {\n    TR_for { }\n}\nTR_var x = [1]\nx[5]\n",
		"exit":              "TR_exit(4)\n",
	} {
		t.Run(name, func(t *testing.T) {
			v := New()
			if _, err := v.Run(mustParseProgram(t, first)); err == nil {
				t.Fatal("ilk program hatayla bitmeliydi")
			}
			if v.Interrupted() {
				t.Error("programın kendi kesmesi Run dönmeden kalkmalı")
			}
			if err := runReuseLoop(t, v); err != nil {
				t.Errorf("sonraki program kesilmemeli: %v", err)
			}
		})
	}
	t.Run("task_opened_in_defer", func(t *testing.T) {
		v := New()
		var ran atomic.Int32
		v.RegisterBuiltin("say", func(*VM, []Value, map[string]Value) (Value, error) {
			ran.Add(1)
			return Null, nil
		})
		code := "TR_defer {\n    TR_async {\n        say()\n        TR_for { }\n    }\n}\nTR_var x = [1]\nx[5]\n"
		if _, err := v.Run(mustParseProgram(t, code)); err == nil {
			t.Fatal("program hatayla bitmeliydi")
		}
		// Görev durdurulmadan çalışırsa sonsuz döngüdedir: süreli beklenir, sonra durdurulur.
		for deadline := time.Now().Add(5 * time.Second); v.liveTasks.Load() != 0 && time.Now().Before(deadline); {
			time.Sleep(time.Millisecond)
		}
		if v.liveTasks.Load() != 0 {
			v.Interrupt()
			v.WaitAsync()
			t.Fatalf("ertelenen işte açılan görev 5 sn'de durmadı (çalıştı=%d)", ran.Load())
		}
		if ran.Load() != 0 || !v.Interrupted() {
			t.Fatalf("ertelenen işte açılan görev durdurulmalı (çalıştı=%d), kesme kalmalı (%v)", ran.Load(), v.Interrupted())
		}
		if err := runReuseLoop(t, v); !errors.Is(err, ErrInterrupted) {
			t.Errorf("kesme kaldığı için sonraki program durmalı: %v", err)
		}
		// Kalan kesme ilk programındır (kaldırılmadı, ownStop durur); canlı görev kalmayınca ikinci
		// programın sonunda kalkar: VM bir program sonra düzelir.
		if v.Interrupted() {
			t.Error("ikinci programdan sonra kesme kalkmalı")
		}
		if err := runReuseLoop(t, v); err != nil {
			t.Errorf("üçüncü program kesilmemeli: %v", err)
		}
	})
}

// stopDuring v'de sonsuz döngülü bir program başlatır, döngüye girince RequestStop gönderir ve isteğin
// nasıl karşılandığını ve Run'ın hatasını verir. Program durmazsa test düşer.
func stopDuring(t *testing.T, v *VM) (StopDisposition, error) {
	t.Helper()
	ready := make(chan struct{})
	v.RegisterBuiltin("hazır", func(*VM, []Value, map[string]Value) (Value, error) {
		close(ready)
		return Null, nil
	})
	got := make(chan StopDisposition, 1)
	go func() {
		<-ready
		time.Sleep(5 * time.Millisecond)
		got <- v.RequestStop(2)
	}()
	done := make(chan error, 1)
	go func() {
		_, err := v.Run(mustParseProgram(t, "hazır()\nTR_for { }\n"))
		done <- err
	}()
	select {
	case err := <-done:
		return <-got, err
	case <-time.After(10 * time.Second):
		v.Interrupt() // takılan programı bırak
		t.Fatalf("istek programı durdurmadı: %v", <-got)
		return 0, nil
	}
}

// Aynı VM'de gerçek kesme sürer (1.5, 1.15): önceki programın bitişi sonrakine gelen isteği yutmaz;
// önceki program biterken ya da programlar arasında gelen istek, istekle duran programın kesmesi ve
// Go'dan doğrudan Interrupt kalıcıdır.
func TestReusedVMStillStops(t *testing.T) {
	t.Run("after_a_clean_program", func(t *testing.T) {
		v := New()
		if err := runReuseLoop(t, v); err != nil {
			t.Fatal(err)
		}
		if d, err := stopDuring(t, v); d != StopApplied || !errors.Is(err, ErrInterrupted) {
			t.Errorf("hatasız programdan sonra istek uygulanmalı: %v %v", d, err)
		}
	})
	t.Run("after_an_error_program", func(t *testing.T) {
		v := New()
		v.Run(mustParseProgram(t, "TR_var x = [1]\nx[5]\n"))
		if d, err := stopDuring(t, v); d != StopApplied || !errors.Is(err, ErrInterrupted) {
			t.Errorf("hatalı programdan sonra istek uygulanmalı: %v %v", d, err)
		}
	})
	t.Run("request_between_programs", func(t *testing.T) {
		for _, first := range []string{reuseLoop, "TR_var x = [1]\nx[5]\n"} {
			v := New()
			v.Run(mustParseProgram(t, first))
			if d := v.RequestStop(2); d != StopFinishing {
				t.Fatalf("programlar arasındaki istek biten programın isteğidir: %v", d)
			}
			if err := runReuseLoop(t, v); !errors.Is(err, ErrInterrupted) {
				t.Errorf("kaydedilmiş istek sonraki programı durdurmalı: %v\n%s", err, first)
			}
		}
	})
	t.Run("after_a_stopped_program", func(t *testing.T) {
		v := New()
		if _, err := stopDuring(t, v); !errors.Is(err, ErrInterrupted) {
			t.Fatalf("istek programı durdurmalı: %v", err)
		}
		if err := runReuseLoop(t, v); !errors.Is(err, ErrInterrupted) {
			t.Errorf("istekle duran programdan sonra da durmalı: %v", err)
		}
	})
	t.Run("direct_interrupt_is_kept", func(t *testing.T) {
		v := New()
		v.RegisterBuiltin("kes", func(y *VM, _ []Value, _ map[string]Value) (Value, error) {
			y.Interrupt() // programın kendi kesmesinden sonra gelir, false döner
			return Null, nil
		})
		v.Run(mustParseProgram(t, "TR_defer { kes() }\nTR_var x = [1]\nx[5]\n"))
		if !v.Interrupted() {
			t.Fatal("Go'dan doğrudan Interrupt kalmalı")
		}
		if err := runReuseLoop(t, v); !errors.Is(err, ErrInterrupted) {
			t.Errorf("doğrudan Interrupt'tan sonra program durmalı: %v", err)
		}
	})
}

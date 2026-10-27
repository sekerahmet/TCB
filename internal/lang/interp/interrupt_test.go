package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// Kes programı bir sonraki güvenli noktada durdurur: Run ErrKesildi döner, dene/yakala
// kesilmeyi yakalamaz, ertelenen işler (fonksiyon, eşzamanlı görev, program) çalışır,
// eşzamanlı görevin kesilmesi hata olarak bildirilmez.
func TestInterrupt(t *testing.T) {
	codeVal := `
değişken izler = []
ertele { izler.ekle("program") }
eşzamanlı {
    ertele { izler.ekle("görev") }
    döngü { }
}
fonksiyon iş() {
    ertele { izler.ekle("fonksiyon") }
    dene {
        hazır()
        döngü { }
    } yakala h {
        izler.ekle("yakalandı")
    }
}
iş()
izler.ekle("sürdü")
`
	prog := mustParseProgram(t, codeVal)
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
değişken izler = []
fonksiyon sür() { izler.ekle("sürdü") }
izler.ekle(uzun())
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

package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
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

// Kesmeyi erteleyen (kontrol noktası) yoksa KesmeIste programı Kes gibi durdurur; varsa
// yalnızca bayrak koyar, program SimdiKes çağrılınca o noktada durur.
func TestInterruptDeferred(t *testing.T) {
	v := New()
	if v.RequestInterrupt() || !v.Interrupted() {
		t.Fatal("erteleyen yokken kesme hemen olmalıydı")
	}
	v = New()
	v.DeferInterrupts()
	if !v.RequestInterrupt() || v.Interrupted() || !v.StopRequested() {
		t.Fatalf("kesme ertelenmeliydi: kesildi=%v istendi=%v", v.Interrupted(), v.StopRequested())
	}
	v.GlobalEnv.Define("dur", &BuiltinVal{Name: "dur", Fn: func(m *VM, _ []Value, _ map[string]Value) (Value, error) {
		m.InterruptNow()
		return Null, nil
	}}, true)
	var out bytes.Buffer
	v.Out = &out
	p := parser.New(source.TextLexer("TR_defer { TR_print(\"ertelenen\") }\ndur()\nTR_print(\"sürdü\")\n"))
	prog := p.ParseProgram()
	if _, err := v.Run(prog); !errors.Is(err, ErrInterrupted) || out.String() != "ertelenen\n" {
		t.Fatalf("SimdiKes programı o noktada durdurmalıydı: %v %q", err, out.String())
	}
}

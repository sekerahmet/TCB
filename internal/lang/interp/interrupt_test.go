package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
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
	p := parser.New(lexer.New(codeVal))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	vmx := New()
	var failure bytes.Buffer
	vmx.Err = &failure
	ready := make(chan struct{})
	vmx.RegisterBuiltin("hazır", func(*VM, []Value, map[string]Value) (Value, error) {
		close(ready)
		return NULL, nil
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
		return NULL, nil
	}}, true)
	var out bytes.Buffer
	v.Out = &out
	p := parser.New(lexer.New("ertele { yazdır(\"ertelenen\") }\ndur()\nyazdır(\"sürdü\")\n"))
	prog := p.ParseProgram()
	if _, err := v.Run(prog); !errors.Is(err, ErrInterrupted) || out.String() != "ertelenen\n" {
		t.Fatalf("SimdiKes programı o noktada durdurmalıydı: %v %q", err, out.String())
	}
}

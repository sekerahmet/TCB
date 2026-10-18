package ayikla

import (
	"io"
	"testing"
	"time"
	"tr_dili/internal/dil/ayristir"
	"tr_dili/internal/dil/sozcuk"
	"tr_dili/internal/dil/yurut"
)

// runTRWithin kodu bir süre sınırıyla çalıştırır; kilitlenme (deadlock) testi asmak yerine başarısız olur.
func runTRWithin(t *testing.T, limit time.Duration, code string) (*yurut.VM, yurut.Value, error) {
	t.Helper()
	p := ayristir.New(sozcuk.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	machine := yurut.New()
	machine.Out = io.Discard
	type result struct {
		val yurut.Value
		err error
	}
	done := make(chan result, 1)
	go func() {
		val, err := machine.Run(prog)
		machine.WaitAsync()
		done <- result{val, err}
	}()
	select {
	case r := <-done:
		return machine, r.val, r.err
	case <-time.After(limit):
		t.Fatalf("program %v içinde bitmedi (kilitlenme?)", limit)
		return nil, nil, nil
	}
}

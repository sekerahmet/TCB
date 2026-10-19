package debugger

import (
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"io"
	"testing"
	"time"
)

// runTRWithin kodu bir süre sınırıyla çalıştırır; kilitlenme (deadlock) testi asmak yerine başarısız olur.
func runTRWithin(t *testing.T, limit time.Duration, code string) (*interp.VM, interp.Value, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	machine := interp.New()
	machine.Out = io.Discard
	type result struct {
		val interp.Value
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

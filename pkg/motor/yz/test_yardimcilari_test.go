package yz

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/vm"
)

// Testlerdeki VM'ler bu paketin yerleşiklerini de içersin (tr_dili/pkg/motor gibi).
func init() {
	vm.MotorEkle(Kaydet)
}

// runTRWithin kodu bir süre sınırıyla çalıştırır; kilitlenme (deadlock) testi asmak yerine başarısız olur.
func runTRWithin(t *testing.T, limit time.Duration, code string) (*vm.VM, vm.Value, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	machine := vm.New()
	machine.Out = io.Discard
	type result struct {
		val vm.Value
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

func runTR(t *testing.T, code string) (vm.Value, error) {
	t.Helper()
	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	machine := vm.New()
	val, err := machine.Run(prog)
	machine.WaitAsync()
	return val, err
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// runTROut programı çalıştırır ve yazdır() çıktısını da döndürür.
func runTROut(t *testing.T, code string) (string, vm.Value, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Sözdizimi hatası: %v", p.Errors())
	}
	var out bytes.Buffer
	machine := vm.New()
	machine.Out = &out
	val, err := machine.Run(prog)
	machine.WaitAsync()
	return out.String(), val, err
}

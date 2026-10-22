// Package trrun, motor ve araç testlerinin TR programı çalıştırma yardımcılarını toplar:
// ayrıştırıp sözdizimi hatasında testi düşürme, yeni bir VM'de çalıştırıp çıktısını toplama,
// hatasız çalıştırma ve tip denetimi isteyen hatasız çalıştırma. Motorlar (ml, dataset ...) VM'e
// testin kendi içe aktarmalarıyla (init ve `_ ".../internal/stdlib"`) kayıtlanır; bu paket
// yalnızca dil çekirdeğini bilir. Yalnızca motor, araç ve komut testleri kullanır (dil
// çekirdeğinin kendi testleri kullanamaz: içe aktarma döngüsü olurdu).
//
// Layer: test helper (internal/testkit); dil çekirdeğini içe aktarır, yalnızca testler kullanır.
package trrun

import (
	"bytes"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/ast"
	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Parse kodu ayrıştırır; sözdizimi hatasında testi düşürür.
func Parse(t testing.TB, code string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("syntax error: %v\ncode:\n%s", p.Errors(), code)
	}
	return prog
}

// Outcome bir programın çalışmasının sonucudur.
type Outcome struct {
	Out   string       // yazdır() çıktısı
	VM    *interp.VM   // programın çalıştığı VM (genel değişkenleri okumak için)
	Value interp.Value // programın son değeri
	Err   error
}

// Exec kodu yeni bir VM'de çalıştırır, eşzamanlı görevleri bekler ve sonucu verir.
func Exec(t testing.TB, code string) Outcome {
	t.Helper()
	prog := Parse(t, code)
	m := interp.New()
	var out bytes.Buffer
	m.Out = &out
	val, err := m.Run(prog)
	m.WaitAsync()
	return Outcome{Out: out.String(), VM: m, Value: val, Err: err}
}

// MustExec Exec gibidir; program hata verirse testi düşürür.
func MustExec(t testing.TB, code string) Outcome {
	t.Helper()
	o := Exec(t, code)
	if o.Err != nil {
		t.Fatalf("unexpected error: %v\ncode:\n%s", o.Err, code)
	}
	return o
}

// MustExecChecked MustExec gibidir ama önce tr komutunun yaptığı gibi tip denetiminden geçirir:
// tanımsız bir ad yalnızca zamanlamaya bağlı olarak çalışan bir satırda olsa da test her makinede
// kalır.
func MustExecChecked(t testing.TB, code string) Outcome {
	t.Helper()
	p := parser.New(lexer.New(code))
	if errs := checker.New().CheckProgram(p.ParseProgram()); len(errs) > 0 {
		t.Fatalf("type check error: %v\ncode:\n%s", errs, code)
	}
	return MustExec(t, code)
}

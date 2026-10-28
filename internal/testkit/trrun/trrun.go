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
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Parse başlıksız bir program gövdesini (source.Options.Body) ayrıştırır; sözdizimi hatasında
// testi düşürür. Başlıklı metin de okunur.
func Parse(t testing.TB, code string) *ast.Program {
	t.Helper()
	r := source.Prepare(code, source.Options{Body: true})
	if r.Stage == source.StageSyntax {
		t.Fatalf("syntax error: %v\ncode:\n%s", r.Errors, code)
	}
	return r.Program
}

// Outcome bir programın çalışmasının sonucudur.
type Outcome struct {
	Out    string       // yazdır() çıktısı
	ErrOut string       // programın hata akışı (VM.Err): uyarılar ve görev hataları
	VM     *interp.VM   // programın çalıştığı VM (genel değişkenleri okumak için)
	Value  interp.Value // programın 'result' adlı genel değişkeni; yoksa son değeri
	Err    error
}

// Exec kodu yeni bir VM'de çalıştırır, eşzamanlı görevleri bekler ve sonucu verir. Programın
// sonucu 'result' adlı genel değişkendir: programın en dışında TR_return yazılmaz (1.10).
func Exec(t testing.TB, code string) Outcome {
	t.Helper()
	prog := Parse(t, code)
	m := interp.New()
	var out, errOut bytes.Buffer
	m.Out, m.Err = &out, &errOut
	val, err := m.Run(prog)
	if r, ok := m.GlobalEnv.Get("result"); ok {
		val = r
	}
	m.WaitAsync()
	return Outcome{Out: out.String(), ErrOut: errOut.String(), VM: m, Value: val, Err: err}
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
	if errs := checker.New().CheckProgram(Parse(t, code)); len(errs) > 0 {
		t.Fatalf("type check error: %v\ncode:\n%s", errs, code)
	}
	return MustExec(t, code)
}

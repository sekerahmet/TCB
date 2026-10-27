package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
)

// Ertelenen işin hatası yutulmaz: programın üst düzeyinde programın, görevde görevin hatası olur.
func TestDeferErrorInProgramAndTask(t *testing.T) {
	var out, failure bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &failure
	prog := parser.New(source.TextLexer("TR_defer { TR_throw \"üst düzey ertele\" }\nTR_async {\n    TR_defer { TR_throw \"görev ertelesi\" }\n}\nTR_print(\"gövde\")\n")).ParseProgram()
	_, err := m.Run(prog)
	if err == nil || err.Error() != "Satır 1: üst düzey ertele" {
		t.Fatalf("programın ertele hatası dönmeli: %v", err)
	}
	if !strings.Contains(failure.String(), "Eşzamanlı görev hatası") || !strings.Contains(failure.String(), "görev ertelesi") || !strings.Contains(out.String(), "gövde") {
		t.Fatalf("görevin ertele hatası raporlanmalı: %q / %q", failure.String(), out.String())
	}
}

// Kural (8.3): yakalanmayan 'fırlat' programı konumla (satır, çerçeve) durdurur, eşzamanlı görevin
// raporu da satırı söyler; yakalanan metin programın değeridir, öneksiz (8.2). Önceden yakalanmayan
// fırlat yalnız değeri yazıyordu ("• sıfır olamaz").
func TestUncaughtThrowStopsWithPosition(t *testing.T) {
	var out, failure bytes.Buffer
	m := New()
	m.Out, m.Err = &out, &failure
	prog := parser.New(source.TextLexer("TR_func f() {\n    TR_throw \"sıfır olamaz\"\n}\n" +
		"TR_try { f() } TR_catch e { TR_print(e) }\n" +
		"TR_async {\n    TR_throw \"kötü öğe\"\n}\nTR_wait_all()\nf()\n")).ParseProgram()
	_, err := m.Run(prog)
	if line, _, ok := messages.Pos(err); !ok || line != 2 || err.Error() != "Satır 2: sıfır olamaz" {
		t.Errorf("yakalanmayan fırlat satırıyla durmalı: %v (satır %d)", err, line)
	}
	if out.String() != "sıfır olamaz\n" {
		t.Errorf("yakalanan metin öneksiz olmalı: %q", out.String())
	}
	if !strings.Contains(failure.String(), "Satır 6: kötü öğe") {
		t.Errorf("görev raporu satırı söylemeli: %q", failure.String())
	}
}

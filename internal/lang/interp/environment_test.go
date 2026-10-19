package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Ertelenen işin hatası yutulmaz: programın üst düzeyinde programın, görevde görevin hatası olur.
func TestDeferErrorInProgramAndTask(t *testing.T) {
	var out, failure bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &failure
	prog := parser.New(lexer.New("ertele { fırlat \"üst düzey ertele\" }\neşzamanlı {\n    ertele { fırlat \"görev ertelesi\" }\n}\nyazdır(\"gövde\")\n")).ParseProgram()
	_, err := m.Run(prog)
	if err == nil || err.Error() != "üst düzey ertele" {
		t.Fatalf("programın ertele hatası dönmeli: %v", err)
	}
	if !strings.Contains(failure.String(), "Eşzamanlı görev hatası") || !strings.Contains(failure.String(), "görev ertelesi") || !strings.Contains(out.String(), "gövde") {
		t.Fatalf("görevin ertele hatası raporlanmalı: %q / %q", failure.String(), out.String())
	}
}

package vm

import (
	"bytes"
	"strings"
	"testing"
	"tr_dili/internal/dil/lexer"
	"tr_dili/internal/dil/parser"
)

// Eşzamanlı görev hataları sabit os.Stderr'e yazılıyor, VS Code oturumunda ve tr test
// çıktısında görünmüyordu. VM'in hata yazıcısına (Err) gider.
func TestGorevHatasiErrYazicisinaGider(t *testing.T) {
	var out, hata bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &hata
	prog := parser.New(lexer.New("eşzamanlı {\n    değişken x = 1 / 0\n}\nbekle_hepsi()\n")).ParseProgram()
	if _, err := m.Run(prog); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hata.String(), "Eşzamanlı görev hatası (satır 1'de başlatılan): Satır 2: Sıfıra bölme hatası") {
		t.Fatalf("görev hatası Err'e yazılmalı: %q (çıktı %q)", hata.String(), out.String())
	}
	if New().Err == nil {
		t.Fatal("varsayılan hata yazıcısı standart hata olmalı")
	}
}

// Ertelenen işin hatası yutulmaz: programın üst düzeyinde programın, görevde görevin hatası olur.
func TestErteleHatasiProgramVeGorevde(t *testing.T) {
	var out, hata bytes.Buffer
	m := New()
	m.Out = &out
	m.Err = &hata
	prog := parser.New(lexer.New("ertele { fırlat \"üst düzey ertele\" }\neşzamanlı {\n    ertele { fırlat \"görev ertelesi\" }\n}\nyazdır(\"gövde\")\n")).ParseProgram()
	_, err := m.Run(prog)
	if err == nil || err.Error() != "üst düzey ertele" {
		t.Fatalf("programın ertele hatası dönmeli: %v", err)
	}
	if !strings.Contains(hata.String(), "Eşzamanlı görev hatası") || !strings.Contains(hata.String(), "görev ertelesi") || !strings.Contains(out.String(), "gövde") {
		t.Fatalf("görevin ertele hatası raporlanmalı: %q / %q", hata.String(), out.String())
	}
}

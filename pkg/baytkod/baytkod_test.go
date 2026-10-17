package baytkod

import (
	"strings"
	"testing"
	"tr_dili/pkg/checker"
	"tr_dili/pkg/lexer"
	"tr_dili/pkg/parser"
)

func TestCheckerAndBytecode(t *testing.T) {
	code := `
		sabit MAX: Tam = 100
		değişken a, b = 1, 2
		değişken liste = [x * 2 döngü x içinde 1..=5 eğer x > 2]
		dene {
			eğer a > 0 {
				a += 1
			} değilse eğer a == 0 {
				a = 0
			} değilse {
				fırlat "hata"
			}
		} yakala h {
			yazdır(h)
		}
	`
	l := lexer.New(code)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parser hatası: %v", p.Errors())
	}
	chk := checker.New()
	if errs := chk.CheckProgram(prog); len(errs) > 0 {
		t.Fatalf("Checker hatası: %v", errs)
	}
	chunk := CompileToBytecode(prog)
	out := chunk.Disassemble()
	if !strings.Contains(out, "LİSTE_ÜRETECİ") || !strings.Contains(out, "DENE_BAŞLAT") {
		t.Fatalf("Bytecode çıktısında beklenen komutlar eksik:\n%s", out)
	}
}

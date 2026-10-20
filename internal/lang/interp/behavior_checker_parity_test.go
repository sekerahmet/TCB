package interp

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Denetçi çalışma anıyla aynı sözleri kullanır: aynı hata denetimde ve çalışırken aynı metinle
// anlatılır (ortak metin hint paketinden gelir). Her program önce denetçiden, sonra yorumlayıcıdan
// geçer; denetçinin hata ayrıntısı (konum öneki ve "Tanımsız ad — " başlığı dışında) çalışma
// anı iletisinin içinde aynen bulunmalıdır. Bilinen ayrışma: tanımsız bir adı değere atamak
// (`tanımsız = 3`) denetçide "Tanımsız ad" (H1010), çalışırken "tanımsız değişken" (H2448)
// olarak anlatılır; bu yüzden aşağıdaki programlar yalnızca ad okumalarıdır.
func TestCheckerAndRuntimeShareWording(t *testing.T) {
	programs := []string{
		"yazdir(1)",
		"değişken öğrenci_sayısı = 1\nyazdır(öğrenci_sayis)",
		"değişken l = [1]\nfiltrele(l, (x) => x > 0)",
		"matris([[1]])",
		"yazdır(zzz)",
	}
	const header = "Tanımsız ad — "
	for _, src := range programs {
		p := parser.New(lexer.New(src))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("%q: %v", src, p.Errors())
		}
		errs := checker.New().CheckProgram(prog)
		if len(errs) == 0 {
			t.Errorf("%q: denetçi hata vermeli", src)
			continue
		}
		_, _, runErr := runTROut(t, src)
		if runErr == nil {
			t.Errorf("%q: çalışma anında hata beklenirdi", src)
			continue
		}
		i := strings.Index(errs[0], header)
		if i < 0 {
			t.Errorf("%q: denetçi iletisi %q başlığını taşımıyor: %s", src, header, errs[0])
			continue
		}
		detail := errs[0][i+len(header):]
		if !strings.Contains(runErr.Error(), detail) {
			t.Errorf("%q: sözler ayrışıyor\n denetçi: %s\n çalışma: %s", src, detail, runErr.Error())
		}
	}
}

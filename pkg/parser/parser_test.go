package parser

import (
	"testing"
	"tr_dili/pkg/lexer"
)

// Hatalı yapı metodları eskiden "interface conversion: ast.Expression is nil" paniğine yol açıyordu.
func TestMalformedStructMethodReportsErrorInsteadOfPanic(t *testing.T) {
	for _, code := range []string{
		"yapı A { fonksiyon f { } }",
		"yapı A { fonksiyon f { yazdır(1) } }\nyazdır(2)",
		"yapı A { fonksiyon { } }",
		"yapı A { fonksiyon f(",
		"yapı A { fonksiyon f { { { }",
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%q ayrıştırılırken panik: %v", code, r)
				}
			}()
			p := New(lexer.New(code))
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Errorf("%q için sözdizimi hatası bekleniyordu", code)
			}
		}()
	}
}

func TestValidStructMethodStillParses(t *testing.T) {
	p := New(lexer.New("yapı A {\n  ad: Metin\n  fonksiyon selam() { sonuç kendi.ad }\n}"))
	p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("geçerli yapı hata vermemeli: %v", p.Errors())
	}
}

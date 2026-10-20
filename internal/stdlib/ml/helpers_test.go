package ml

import (
	"bytes"
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Testlerdeki VM'ler tensör motorunu içersin.
func init() {
	interp.AddEngine(Register)
}

// run TR kodunu çalıştırır; çıktıyı ve hatayı döndürür.
func run(t *testing.T, code string) (string, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	var out bytes.Buffer
	m.Out = &out
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

// mustRun kodu hatasız çalıştırır. Önce, tr komutunun yaptığı gibi tip denetiminden
// geçirir: tanımsız ya da kaldırılmış bir ad (ör. 'çık'ın eski adı) yalnızca zamanlamaya bağlı
// olarak çalışan bir satırda olsa da test her makinede kalır.
func mustRun(t *testing.T, code string) string {
	t.Helper()
	p := parser.New(lexer.New(code))
	if errs := checker.New().CheckProgram(p.ParseProgram()); len(errs) > 0 {
		t.Fatalf("tip denetimi hatası: %v\nkod:\n%s", errs, code)
	}
	out, err := run(t, code)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\nkod:\n%s", err, code)
	}
	return out
}

// value kodu çalıştırır ve son satırdaki 'cikti' değişkeninin değerini döndürür.
func value(t *testing.T, code string) interp.Value {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	m.Out = &bytes.Buffer{}
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	v, ok := m.GlobalEnv.Get("cikti")
	if !ok {
		t.Fatal("'cikti' tanımlı değil")
	}
	return v
}

func tensorData(t *testing.T, v interp.Value) ([]float32, []int) {
	t.Helper()
	d, ok := v.(*TensorValue)
	if !ok {
		t.Fatalf("tensör beklendi, %s (%s) geldi", v.Inspect(), v.Type())
	}
	return d.t.Data(), d.t.Shape()
}

func near(t *testing.T, ad string, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d eleman, %d bekleniyordu (%v)", ad, len(got), len(want), got)
	}
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > tol {
			t.Fatalf("%s: [%d] = %v, %v bekleniyordu (tümü %v)", ad, i, got[i], want[i], got)
		}
	}
}

func shapeEqual(t *testing.T, ad string, got, want []int) {
	t.Helper()
	if shapeText(got) != shapeText(want) {
		t.Fatalf("%s: şekil %v, %v bekleniyordu", ad, got, want)
	}
}

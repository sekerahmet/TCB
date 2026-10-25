package ml

import (
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"math"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Testlerdeki VM'ler tensör motorunu içersin.
func init() {
	interp.AddEngine(Register)
}

// run TR kodunu çalıştırır; çıktıyı ve hatayı döndürür.
func run(t *testing.T, code string) (string, error) {
	t.Helper()
	o := trrun.Exec(t, code)
	return o.Out, o.Err
}

// mustRun kodu hatasız çalıştırır. Önce, tr komutunun yaptığı gibi tip denetiminden
// geçirir: tanımsız bir ad yalnızca zamanlamaya bağlı
// olarak çalışan bir satırda olsa da test her makinede kalır.
func mustRun(t *testing.T, code string) string {
	t.Helper()
	return trrun.MustExecChecked(t, code).Out
}

// codeCase bir program ve onun durması beklenen kodlu hatadır.
type codeCase struct {
	code string
	want messages.Code
}

// expectCodes her programın beklenen ileti koduyla durduğunu denetler.
func expectCodes(t *testing.T, cases []codeCase) {
	t.Helper()
	for _, c := range cases {
		if _, err := run(t, c.code); err == nil || !messages.Has(err, c.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", c.code, err, c.want)
		}
	}
}

// value kodu çalıştırır ve son satırdaki 'cikti' değişkeninin değerini döndürür.
func value(t *testing.T, code string) interp.Value {
	t.Helper()
	v, ok := trrun.MustExec(t, code).VM.GlobalEnv.Get("cikti")
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
	if tensor.ShapeText(got) != tensor.ShapeText(want) {
		t.Fatalf("%s: şekil %v, %v bekleniyordu", ad, got, want)
	}
}

package debugger

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// sahteTensor, IzlemeDegeri arayüzünü uygulayan bir motor değerini taklit eder.
type sahteTensor struct{}

func (sahteTensor) Type() interp.ValueType { return interp.VAL_TENSOR }
func (sahteTensor) Inspect() string        { return "Tensör[2×2] [\n  [1, 2],\n  [3, 4]\n]" }
func (sahteTensor) IsTruthy() bool         { return true }
func (sahteTensor) IzlemeTuru() string     { return "tensor" }
func (sahteTensor) IzlemeEtiketi() string  { return "Tensör [2×2]" }
func (sahteTensor) TabloVerisi() ([]string, [][]string, error) {
	return []string{"Sütun 1", "Sütun 2"}, [][]string{{"1", "2"}, {"3", "4"}}, nil
}

func TestIzlemeDegeriArayuzu(t *testing.T) {
	var v interp.Value = sahteTensor{}
	if got := typeLabel(v); got != "Tensör [2×2]" {
		t.Fatalf("tip etiketi: %q", got)
	}
	if got := valueKind(v); got != "tensor" {
		t.Fatalf("tür: %q", got)
	}
	if got := display(v); got != "Tensör [2×2]" {
		t.Fatalf("gösterim: %q", got)
	}
	cols, rows, err := TableData(v)
	if err != nil || len(cols) != 2 || len(rows) != 2 || rows[1][1] != "4" {
		t.Fatalf("tablo: %v %v %v", cols, rows, err)
	}
	// Komut satırı ayıklayıcısının değişken kutusu çok satırlı biçimi tek satırda gösterir.
	if got := tekSatir(v.Inspect()); got != "Tensör[2×2] [[1, 2], [3, 4]]" {
		t.Fatalf("tek satır: %q", got)
	}
}

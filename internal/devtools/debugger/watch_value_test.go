package debugger

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/dataset"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/stdlib/tokenizer"
)

// Motor değerleri İzleme Masası arayüzlerini yapısal olarak uygular (motorlar bu paketi
// içe aktarmaz); bir ad değişikliği bağı sessizce koparmasın diye derleme anında denetlenir.
var (
	_ WatchValue      = (*ml.TensorValue)(nil)
	_ WatchSummarizer = (*ml.TensorValue)(nil)
	_ WatchValue      = (*dataset.Dataset)(nil)
	_ WatchValue      = (*dataset.Reader)(nil)
	_ WatchValue      = (*tokenizer.Object)(nil)
)

// fakeTensor, WatchValue arayüzünü uygulayan bir motor değerini taklit eder.
type fakeTensor struct{}

func (fakeTensor) Type() interp.ValueType { return interp.VAL_TENSOR }
func (fakeTensor) Inspect() string        { return "Tensör[2×2] [\n  [1, 2],\n  [3, 4]\n]" }
func (fakeTensor) IsTruthy() bool         { return true }
func (fakeTensor) WatchKind() string      { return "tensor" }
func (fakeTensor) WatchLabel() string     { return "Tensör [2×2]" }
func (fakeTensor) TableData() ([]string, [][]string, error) {
	return []string{"Sütun 1", "Sütun 2"}, [][]string{{"1", "2"}, {"3", "4"}}, nil
}

func TestWatchValueInterface(t *testing.T) {
	var v interp.Value = fakeTensor{}
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
	if got := singleLine(v.Inspect()); got != "Tensör[2×2] [[1, 2], [3, 4]]" {
		t.Fatalf("tek satır: %q", got)
	}
}

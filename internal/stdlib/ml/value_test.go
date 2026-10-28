package ml

import (
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.10): tensör koşul olamaz; doğruluk sınaması yoktur, değer açık karşılaştırılır.
func TestTensorIsNotACondition(t *testing.T) {
	out := mustRun(t, `TR_print(TR_error_code { TR_if TR_Tensor([0.0]) { } })
TR_print(TR_error_code { TR_if TR_sum(TR_Tensor([1, -1])) { } })
TR_print(TR_error_code { TR_if TR_zeros([0]) { } })
TR_if TR_sum(TR_Tensor([1, -1])).TR_tolist == 0 { TR_print("açık sınama") }`)
	want := "H2600\nH2600\nH2600\naçık sınama\n"
	if out != want {
		t.Fatalf("tensör koşulu:\n%q", out)
	}
}

func TestValueShortForm(t *testing.T) {
	out := mustRun(t, `TR_var a = TR_Tensor([0.1, 0.2, 1.5, 3])
TR_print(a.TR_tolist, a.TR_tolist[0] == 0.1, TR_sum(TR_Tensor([0.1])).TR_tolist)
TR_var m = TR_Tensor([[1, 2], [3, 4]]).TR_tolist
m[0].TR_append(9)
TR_print(m)`)
	want := "[0.1, 0.2, 1.5, 3.0] true 0.1\n[[1.0, 2.0, 9], [3.0, 4.0]]\n"
	if out != want {
		t.Fatalf("değer:\n%q\nbeklenen:\n%q", out, want)
	}
	// Yapı: 3 boyutlu tensör doğru iç içe listeye dönüşür.
	d := New(tensor.Arange(24).Reshape(2, 3, 4))
	v, _ := d.valueList()
	l := v.(*interp.ListVal)
	if got := l.Elements[1].(*interp.ListVal).Elements[2].(*interp.ListVal).Elements[3].(*interp.FloatVal).Val; got != 23 {
		t.Fatalf("[1][2][3] = %v", got)
	}
	if got := l.Inspect(); !strings.HasPrefix(got, "[[[0.0, 1.0, 2.0, 3.0], [4.0") {
		t.Fatalf("liste: %s", got)
	}
	// Boş eksenli tensör.
	if v, _ := New(tensor.Zeros(2, 0)).valueList(); v.Inspect() != "[[], []]" {
		got := v.Inspect()
		t.Fatalf("boş: %s", got)
	}
}

func TestValueAllocations(t *testing.T) {
	d := New(tensor.Zeros(100, 40))
	alloc := testing.AllocsPerRun(5, func() { _, _ = d.valueList() })
	if alloc > 10 {
		t.Fatalf(".değer %v ayırma yaptı; önceden ayrılmış dilimler bekleniyordu", alloc)
	}
}

func TestIntermediateTensorGrad(t *testing.T) {
	_, err := run(t, `TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(w * w))
w = w - w.TR_grad * 0.1
TR_print(w.TR_grad)`)
	if err == nil || !messages.Has(err, messages.H6700) {
		t.Fatalf("ara tensör gradyanı: %v", err)
	}
	// Geri yayılımdan sonra da ara tensör ara sayılır ve iyileştiriciye verilemez.
	_, err = run(t, `TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_var y = w * 2
TR_backward(TR_sum(y))
TR_AdamW([y])`)
	if err == nil || !messages.Has(err, messages.H6514) {
		t.Fatalf("ara tensör parametre: %v", err)
	}
	// Henüz geri yayılım yapılmamış yaprakta gradyan yoktur.
	out := mustRun(t, `TR_print(TR_Tensor([1.0], TR_requires_grad: TR_true).TR_grad, TR_zeros([2]).TR_grad)`)
	if out != "none none\n" {
		t.Fatalf("yaprak gradyanı: %q", out)
	}
}

// Kural: IntElements tensörün bütün elemanlarını tam sayı olarak verir; tam sayı olmayan ilk eleman
// bad ile bildirilir (ok yanlış).
func TestIntElements(t *testing.T) {
	ids, _, ok := New(tensor.NewOn(tensor.CPU, []float32{3, -1, 0}, 3)).IntElements()
	if !ok || !slices.Equal(ids, []int{3, -1, 0}) {
		t.Errorf("IntElements = %v, %v", ids, ok)
	}
	if _, bad, ok := New(tensor.NewOn(tensor.CPU, []float32{1, 2.5, 3.5}, 3)).IntElements(); ok || bad != 2.5 {
		t.Errorf("tam sayı olmayan eleman: bad = %v, ok = %v", bad, ok)
	}
	if ids, _, ok := New(tensor.NewOn(tensor.CPU, []float32{}, 0)).IntElements(); !ok || len(ids) != 0 {
		t.Errorf("boş tensör: %v, %v", ids, ok)
	}
}

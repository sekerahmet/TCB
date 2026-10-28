package ml

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

func TestPrintSpecialValues(t *testing.T) {
	out := mustRun(t, `TR_print(TR_Tensor([0.00001, 1]))
TR_print(TR_Tensor([1000000000.0, 2]))
TR_print(TR_log(TR_Tensor([-1, 0, 1])), TR_exp(TR_Tensor([100])))
TR_print(TR_sum(TR_Tensor([0.00002])))
TR_print(TR_min(TR_Tensor([3, -2, 5])), TR_min(TR_Tensor([3, -2], TR_requires_grad: TR_true)))`)
	want := "Tensor[2] [1.0000e-05, 1.0000e+00]\n" +
		"Tensor[2] [1.0000e+09, 2.0000e+00]\n" +
		"Tensor[3] [   nan,   -inf, 0.0000] Tensor[1] [inf]\n" +
		"Tensor(2.0000e-05)\n" +
		"Tensor(-2.0000) Tensor(-2.0000, requires grad)\n"
	if out != want {
		t.Fatalf("özel değerler:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestLargeTensorPrintCorrect(t *testing.T) {
	// Kısaltılmış yazdırma yalnızca gösterilen hücreleri okur; değerler doğru yerde olmalı.
	d := New(tensor.Arange(2000).Reshape(2, 10, 100))
	s := d.Inspect()
	for _, part := range []string{"[   0.0000,    1.0000,    2.0000, …,   97.0000,   98.0000,   99.0000]", "1999.0000", "1900.0000", "…"} {
		if !strings.Contains(s, part) {
			t.Fatalf("eksik %q:\n%s", part, s)
		}
	}
	if strings.Count(s, "\n") > 25 {
		t.Fatalf("kısaltılmadı:\n%s", s)
	}
	// 1000×1000 yazdırma hızlı ve az bellekli olmalı.
	b := New(tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000))
	if alloc := testing.AllocsPerRun(3, func() { _ = b.Inspect() }); alloc > 500 {
		t.Fatalf("büyük tensör yazdırması %v ayırma yaptı", alloc)
	}
}

// Yazdırma ve indirgeme hız ölçümleri: go test -bench . ./internal/stdlib/ml/

func BenchmarkPrintLarge(b *testing.B) {
	d := New(tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = d.Inspect()
	}
}

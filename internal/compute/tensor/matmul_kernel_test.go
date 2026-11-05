package tensor

import (
	"testing"
	"time"
)

// runKernelWithin çekirdeği kilitsiz yoldaki sırayla çalıştırır: izin, gövde, bitiş. Takılırsa
// asılmaz, düşer.
func runKernelWithin(t *testing.T, k *Kernel) {
	t.Helper()
	if !k.TryAcquire() {
		t.Fatal("çekirdek izin alamadı")
	}
	done := make(chan struct{})
	go func() {
		k.Run()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("çekirdek 10 saniyede bitmedi: gövde tuttuğu izni bekliyor olabilir")
	}
	k.End()
}

func ramp(shape ...int) *Tensor {
	data := make([]float32, NumelOf(shape))
	for i := range data {
		data[i] = float32(i%7) - 3
	}
	return New(data, shape...)
}

// Kural (10.2): bitişik kopya isteyen işlenenle kilitsiz çarpım kendi iznini beklemez, sonucu
// kilit altındakiyle aynıdır.
func TestMatMulKernelWithCopiedOperands(t *testing.T) {
	cases := []struct {
		name string
		a, b *Tensor
	}{
		{"sağ, 3 boyutlu devrik", ramp(2, 3, 4), ramp(2, 5, 4).Transpose(1, 2)},
		{"sol, 2 boyutlu devrik", ramp(4, 3).Transpose(0, 1), ramp(4, 5)},
		{"iki işlenen", ramp(2, 4, 3).Transpose(1, 2), ramp(2, 5, 4).Transpose(1, 2)},
		{"kopyasız", ramp(3, 4), ramp(4, 5)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, k := MatMulKernel(c.a, c.b, Float32)
			if k.end == nil {
				t.Fatal("türev izlenmiyorken kopya çekirdeğin sonunda iade edilmeli")
			}
			runKernelWithin(t, k)
			want := c.a.Contiguous().MatMul(c.b.Contiguous())
			got, exp := r.Data(), want.Data()
			if len(got) != len(exp) {
				t.Fatalf("boy %d, beklenen %d", len(got), len(exp))
			}
			for i := range got {
				if got[i] != exp[i] {
					t.Fatalf("öğe %d: %v, beklenen %v", i, got[i], exp[i])
				}
			}
		})
	}
}

// Türev izlenirken bitişik kopya geri yöne kalır: çekirdek bitince iade edilmez.
func TestMatMulKernelKeepsCopyForGradient(t *testing.T) {
	a := ramp(2, 3, 4)
	a.SetRequiresGrad(true)
	b := ramp(2, 5, 4)
	b.SetRequiresGrad(true)
	r, k := MatMulKernel(a, b.Transpose(1, 2), Float32)
	if k.end != nil {
		t.Fatal("türev izlenirken kopya çekirdeğin sonunda iade edilmemeli")
	}
	runKernelWithin(t, k)
	r.SumAll().Backward()
	if a.Grad() == nil || b.Grad() == nil {
		t.Fatal("gradyan yok")
	}
}

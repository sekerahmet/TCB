package tensor

import (
	"math/rand"
	"testing"
)

// Geri yayılımda iade edilen ara gradyanlar havuzdan yeniden kullanıldığında
// sonuçlar değişmemeli (yeniden kullanılan depolama sıfırlanmış olmalı).
func TestHavuzYenidenKullanim(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	x := Randn(rng, 64, 128)
	W := Randn(rng, 128, 96).SetRequiresGrad(true)
	b := Randn(rng, 96).SetRequiresGrad(true)
	adim := func() ([]float32, []float32) {
		W.grad, b.grad = nil, nil
		h := Add(MatMul(x, W), b).Gelu()
		h.Reshape(32, -1).Square().Mean(1, false).SumAll().Backward()
		return W.Grad().Data(), b.Grad().Data()
	}
	w0, b0 := adim()
	for i := 0; i < 5; i++ {
		w1, b1 := adim()
		yakin(t, "W gradyanı", w1, w0, 0)
		yakin(t, "b gradyanı", b1, b0, 0)
	}

	be := cpuBackend{}
	s := be.Alloc(havuzEnAz, Float32)
	be.Fill(s, 7)
	be.Release(s)
	panikBekle(t, "iki kez iade", func() { be.Release(s) })
	for i := 0; i < 3; i++ {
		y := be.Alloc(havuzEnAz, Float32)
		for _, v := range cd(y) {
			if v != 0 {
				t.Fatal("havuzdan gelen depolama sıfırlanmamış")
			}
		}
		be.Release(y)
	}
}

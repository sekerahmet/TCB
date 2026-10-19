package tensor

import (
	"math/rand"
	"testing"
)

// TestDikkatTakmaAdGradyani q, k, v aynı tensör olduğunda gradyanın, aynı veriden kurulmuş üç
// ayrı tensörün gradyanlarının toplamına eşit olduğunu denetler (16 bitlik gradyanda ayrı
// geçici tamponlar birbirinin sonucunu ezmemeli).
func TestDikkatTakmaAdGradyani(t *testing.T) {
	for _, dt := range []DType{Float32, BFloat16} {
		eski := GradDType()
		SetGradDType(dt)
		x0 := rastgele(rand.New(rand.NewSource(5)), -1, 1, 0, 1, 2, 9, 4)
		x := x0.Clone().SetRequiresGrad(true)
		Attention(x, x, x, AttentionOptions{Causal: true}).SumAll().Backward()
		a, b, c := x0.Clone().SetRequiresGrad(true), x0.Clone().SetRequiresGrad(true), x0.Clone().SetRequiresGrad(true)
		Attention(a, b, c, AttentionOptions{Causal: true}).SumAll().Backward()
		toplam := Add(Add(a.Grad().Cast(Float32), b.Grad().Cast(Float32)), c.Grad().Cast(Float32))
		tol := 1e-5
		if dt != Float32 {
			tol = 2e-2
		}
		yakin(t, "q=k=v gradyanı "+dt.String(), x.Grad().Cast(Float32).Data(), toplam.Data(), tol)
		SetGradDType(eski)
	}
}

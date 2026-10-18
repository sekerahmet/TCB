package tensor

import (
	"fmt"
	"math/rand"
	"testing"
)

// Birleşik dikkat ile bileşik sürümün (softmax(q·kᵀ·ölçek + maske)·v) hız ve
// bellek karşılaştırması: ileri + geri, toplu 1, H 8, dh 64.
//
//	go test ./pkg/tensor -run XXX -bench Dikkat -benchmem
func BenchmarkDikkat(b *testing.B) {
	for _, T := range []int{16, 128, 512, 2048} {
		for _, Hkv := range []int{8, 2} {
			for _, kip := range []struct {
				ad  string
				opt AttentionOptions
			}{{"nedensel", AttentionOptions{Causal: true}}, {"pencere128", AttentionOptions{Causal: true, Window: 128}}} {
				rng := rand.New(rand.NewSource(1))
				q := Randn(rng, 1, 8, T, 64).SetRequiresGrad(true)
				k := Randn(rng, 1, Hkv, T, 64).SetRequiresGrad(true)
				v := Randn(rng, 1, Hkv, T, 64).SetRequiresGrad(true)
				for _, yol := range []struct {
					ad string
					f  func(q, k, v *Tensor, opt AttentionOptions) *Tensor
				}{{"birleşik", Attention}, {"bileşik", bilesikDikkat}} {
					b.Run(fmt.Sprintf("T=%d/Hkv=%d/%s/%s", T, Hkv, kip.ad, yol.ad), func(b *testing.B) {
						b.ReportAllocs()
						for i := 0; i < b.N; i++ {
							q.grad, k.grad, v.grad = nil, nil, nil
							yol.f(q, k, v, kip.opt).SumAll().Backward()
						}
					})
				}
			}
		}
	}
}

// Yalnızca ileri geçiş (çıkarım ve önbellekli üretim: Tq = 1).
func BenchmarkDikkatIleri(b *testing.B) {
	for _, d := range []struct{ Tq, Tk int }{{512, 512}, {2048, 2048}, {1, 2048}} {
		rng := rand.New(rand.NewSource(1))
		q := Randn(rng, 1, 8, d.Tq, 64)
		k := Randn(rng, 1, 2, d.Tk, 64)
		v := Randn(rng, 1, 2, d.Tk, 64)
		opt := AttentionOptions{Causal: true}
		for _, yol := range []struct {
			ad string
			f  func(q, k, v *Tensor, opt AttentionOptions) *Tensor
		}{{"birleşik", Attention}, {"bileşik", bilesikDikkat}} {
			b.Run(fmt.Sprintf("Tq=%d/Tk=%d/%s", d.Tq, d.Tk, yol.ad), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					yol.f(q, k, v, opt)
				}
			})
		}
	}
}

// Blok boyutu taraması (CPU varsayılanını seçmek için).
func BenchmarkDikkatBlok(b *testing.B) {
	eq, ek := AttentionBlocks(CPU)
	defer SetAttentionBlocks(CPU, eq, ek)
	for _, T := range []int{512, 2048} {
		rng := rand.New(rand.NewSource(1))
		q := Randn(rng, 1, 8, T, 64).SetRequiresGrad(true)
		k := Randn(rng, 1, 8, T, 64).SetRequiresGrad(true)
		v := Randn(rng, 1, 8, T, 64).SetRequiresGrad(true)
		for _, bq := range []int{16, 32, 64, 128} {
			for _, bk := range []int{16, 32, 64, 128} {
				b.Run(fmt.Sprintf("T=%d/q=%d/k=%d", T, bq, bk), func(b *testing.B) {
					SetAttentionBlocks(CPU, bq, bk)
					for i := 0; i < b.N; i++ {
						q.grad, k.grad, v.grad = nil, nil, nil
						Attention(q, k, v, AttentionOptions{Causal: true}).SumAll().Backward()
					}
				})
			}
		}
	}
}

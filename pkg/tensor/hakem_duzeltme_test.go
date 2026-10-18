package tensor

import (
	"strings"
	"testing"
)

// Sert hakem denetiminin bulgularının regresyon testleri.

// panikIcerir fn'in, mesajında parca geçen bir panik verdiğini doğrular.
func panikIcerir(t *testing.T, ad, parca string, fn func()) {
	t.Helper()
	if m := panikMesaji(fn); !strings.Contains(m, parca) {
		t.Fatalf("%s: %q içeren panik bekleniyordu, alınan: %q", ad, parca, m)
	}
}

// Bulgu 2: Grad() gradyan deposunun canlı bir takma adıdır. Gradyanı değiştiren her işlem
// (geri yayılım birikimi, ZeroGrad, ClipGradNorm) deponun sürümünü artırır; saklanmış bir
// gradyanla kurulmuş çizge, gradyan değişince açık "yerinde değiştirildi" hatası verir.
func TestGradyanTakmaAdiSurumu(t *testing.T) {
	yeni := func() (*Tensor, *Tensor, *Tensor) {
		x := New([]float32{1, 2}, 2).SetRequiresGrad(true)
		w := New([]float32{1, 1}, 2).SetRequiresGrad(true)
		x.MulScalar(3).SumAll().Backward()
		g := x.Grad()
		return x, w, g.Mul(w).SumAll() // dk/dw = g = [3, 3]
	}
	// Değişiklik yoksa geri yayılım doğru.
	_, w, k := yeni()
	k.Backward()
	yakin(t, "değişmeyen gradyan", w.Grad().Data(), []float32{3, 3}, 0)

	x, _, k := yeni()
	x.MulScalar(5).SumAll().Backward() // x'in gradyanı yerinde birikir
	panikIcerir(t, "birikim", "yerinde değiştirildi", func() { k.Backward() })

	x, _, k = yeni()
	x.ZeroGrad()
	panikIcerir(t, "ZeroGrad", "yerinde değiştirildi", func() { k.Backward() })

	x, _, k = yeni()
	ClipGradNorm([]*Tensor{x}, 0.1)
	panikIcerir(t, "ClipGradNorm", "yerinde değiştirildi", func() { k.Backward() })

	// Aynı geri yayılımda hem gradyanı birikecek x hem de x'in saklanmış gradyanı
	// kullanılıyorsa da açık hata (sıraya bağlı sessiz yanlış yok).
	x, w, _ = yeni()
	g := x.Grad()
	k = Add(x.MulScalar(2).SumAll(), g.Mul(w).SumAll())
	panikIcerir(t, "aynı geri yayılım", "yerinde değiştirildi", func() { k.Backward() })
}

package tensor

import "testing"

// Konum kaynağı açıkken düğüm oluştuğu konumu taşır; geri hesabı çalışırken o konum etkin
// geri konumdur, geri yayılım bitince sıfırlanır. Kaynak yokken konum 0'dır.
func TestNodePositionDuringBackward(t *testing.T) {
	pos := uint64(42)
	SetPositionSource(func() uint64 { return pos })
	defer SetPositionSource(nil)
	w := New([]float32{1, 2, 3, 4}, 2, 2).SetRequiresGrad(true)
	var during uint64
	y := track(Zeros(2, 2), "sınama", []*Tensor{w}, func(o *node) { during = BackwardPosition() })
	if y.n.pos != 42 {
		t.Fatalf("düğümün konumu %d, beklenen 42", y.n.pos)
	}
	pos = 7
	loss := y.Reshape(-1).SumAll() // bu düğümler 7. konumda
	loss.Backward()
	if during != 42 {
		t.Errorf("geri hesap sırasında konum %d, beklenen 42 (düğümü üreten konum)", during)
	}
	if p := BackwardPosition(); p != 0 {
		t.Errorf("geri yayılımdan sonra konum %d, beklenen 0", p)
	}
	SetPositionSource(nil)
	if z := MatMul(w, w); z.n.pos != 0 {
		t.Errorf("kaynak yokken düğümün konumu %d, beklenen 0", z.n.pos)
	}
}

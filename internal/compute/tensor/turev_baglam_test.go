package tensor

import "testing"

// Türevsiz kip etkin bağlamın durumudur: bir bağlamın NoGrad'ı sürerken başka bir bağlam
// etkin yapılırsa (yorumlayıcıda kilidin başka göreve geçmesi) işlemler çizgeye kaydedilir;
// NoGrad girişte yakaladığı bağlamı çıkışta geri sayar.
func TestGradContextGorevBasina(t *testing.T) {
	onceki := SetGradContext(nil)
	defer SetGradContext(onceki)
	a, b := NewGradContext(), NewGradContext()
	w := New([]float32{1, 2}, 2).SetRequiresGrad(true)

	SetGradContext(a)
	NoGrad(func() {
		if GradEnabled() || w.MulScalar(2).RequiresGrad() {
			t.Fatal("a'nın NoGrad'ı içinde işlem izlendi")
		}
		// Kilit b'ye geçti: b türevli kipte.
		SetGradContext(b)
		if !GradEnabled() || !w.MulScalar(2).RequiresGrad() {
			t.Fatal("a'nın NoGrad'ı b'nin işlemlerini türevsiz yaptı")
		}
		NoGrad(func() {
			// b'nin NoGrad'ı sürerken kilit a'ya geri döndü.
			SetGradContext(a)
			if GradEnabled() {
				t.Fatal("a kendi NoGrad'ını görmedi")
			}
		})
		// b'nin NoGrad'ı b'yi geri saydı (etkin bağlam a olsa da).
		if !b.Enabled() || a.Enabled() {
			t.Fatalf("sayaçlar: a=%v b=%v", a.Enabled(), b.Enabled())
		}
	})
	if !a.Enabled() || !b.Enabled() {
		t.Fatalf("NoGrad sonrası: a=%v b=%v", a.Enabled(), b.Enabled())
	}
	// nil varsayılan (süreç geneli) bağlamı etkin yapar; önceki bağlam döner.
	if SetGradContext(nil) != a {
		t.Fatal("SetGradContext önceki bağlamı vermedi")
	}
	if !GradEnabled() {
		t.Fatal("varsayılan bağlam türevli değil")
	}
}

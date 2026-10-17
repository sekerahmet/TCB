package tensor

import "testing"

// Ortak bir ara sonuçtan iki ayrı kayıp için art arda geri yayılım yapmak, ilk
// geri yayılım ara sonuçları bıraktığı için yaprak gradyanlarını sessizce eksik
// bırakırdı; bunun yerine açık bir hata vermeli.
func TestIkinciGeriYayilimHataVerir(t *testing.T) {
	w := New([]float32{1, 2, 3, 4}, 2, 2).SetRequiresGrad(true)
	x := New([]float32{1, 1, 1, 1}, 2, 2)
	h := MatMul(x, w)
	k1 := h.SumAll()
	k2 := h.MulScalar(2).SumAll()
	k1.Backward()
	panikBekle(t, "ikinci geri yayılım", func() { k2.Backward() })
	panikBekle(t, "aynı kökten tekrar", func() { k1.Backward() })
}

// Doğru kullanım: kayıplar toplanıp tek seferde geri yayılır.
func TestToplanmisKayiplarTekGeriYayilim(t *testing.T) {
	w := New([]float32{1, 2, 3, 4}, 2, 2).SetRequiresGrad(true)
	x := New([]float32{1, 1, 1, 1}, 2, 2)
	h := MatMul(x, w)
	h.SumAll().Add(h.MulScalar(2).SumAll()).Backward()
	for i, v := range w.Grad().Data() {
		if v != 6 { // her eleman: 1·2 satır·(1+2)
			t.Fatalf("gradyan[%d] = %v, 6 bekleniyordu", i, v)
		}
	}
}

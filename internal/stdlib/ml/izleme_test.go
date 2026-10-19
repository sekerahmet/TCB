package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// İzleme Masası ve Değişkenler paneli skaler tensörün (ör. kayıp) değerini göstermeli;
// "Tensör (skaler)" etiketi tek başına yetmez. 1-8 elemanlı tensörler satır içinde, daha
// büyükleri yalnızca etiketle (tablo bağlantısıyla) görünür.
func TestIzlemeOzeti(t *testing.T) {
	durumlar := []struct {
		ad    string
		t     *tensor.Tensor
		bekle string
	}{
		{"skaler", tensor.Scalar(0.60738), "0.6074 (Tensör, skaler)"},
		{"skaler bf16", tensor.NewOf(tensor.BFloat16, []float32{2.5}), "2.5000 (Tensör, skaler, bf16)"},
		{"1×1", tensor.New([]float32{1.42981}, 1, 1), "[[1.4298]] (Tensör, 1×1)"},
		{"1 boyutlu", tensor.New([]float32{1, -2.5, 3}, 3), "[1.0000, -2.5000, 3.0000] (Tensör, 3)"},
		{"2×2", tensor.New([]float32{1, 2, 3, 4}, 2, 2), "[[1.0000, 2.0000], [3.0000, 4.0000]] (Tensör, 2×2)"},
		{"bilimsel", tensor.New([]float32{1e-5, 1}, 2), "[1.0000e-05, 1.0000e+00] (Tensör, 2)"},
		{"8 eleman", tensor.New(make([]float32, 8), 2, 2, 2), "[[[0.0000, 0.0000], [0.0000, 0.0000]], [[0.0000, 0.0000], [0.0000, 0.0000]]] (Tensör, 2×2×2)"},
		{"büyük", tensor.New(make([]float32, 9), 9), ""},
		{"boş", tensor.New(nil, 0), ""},
	}
	for _, d := range durumlar {
		if got := Yeni(d.t).IzlemeOzeti(); got != d.bekle {
			t.Errorf("%s: %q, beklenen %q", d.ad, got, d.bekle)
		}
	}
	// Tam sayılı tensör (maks_indeks sonucu) ondalıksız yazılır, yazdır() gibi.
	v := deger(t, "değişken cikti = maks_indeks(tensör([0.1, 0.9, 0.2]))")
	if got := v.(*Deger).IzlemeOzeti(); got != "1 (Tensör, skaler)" {
		t.Errorf("tam sayılı skaler: %q", got)
	}
}

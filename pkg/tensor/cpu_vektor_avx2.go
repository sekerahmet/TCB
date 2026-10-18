//go:build amd64.v3

package tensor

import "math"

// AVX2/FMA ile 8'li üstel (GOAMD64=v3). Algoritma expf32 ile aynıdır (aralık
// indirgemesi ve aynı polinom); ölçekleme iki çarpanla yapıldığı için normal
// altı sonuçlar ve taşma kütüphane işlevine düşmeden doğru çıkar.

// expSabit çekirdeğin sabit tablosu: her değer 8 kez (bir ymm) yinelenir.
var expSabit = func() (t [13 * 8]float32) {
	degerler := [...]float32{
		-104, 89, log2e32, ln2Yuksek, ln2Dusuk,
		1.9875691500e-4, 1.3981999507e-3, 8.3334519073e-3, 4.1665795894e-2, 1.6666665459e-1, 5.0000001201e-1,
		1, math.Float32frombits(127),
	}
	for i, v := range degerler {
		for j := 0; j < 8; j++ {
			t[i*8+j] = v
		}
	}
	return
}()

//go:noescape
func expVekCekirdek(dst, src *float32, n int, sabit *float32)

// expVek dst[i] = e^src[i] (dst ve src aynı dilim olabilir).
func expVek(dst, src []float32) {
	n := len(src)
	dst = dst[:n]
	m := n &^ 7
	if m > 0 {
		expVekCekirdek(&dst[0], &src[0], m, &expSabit[0])
	}
	for i := m; i < n; i++ {
		dst[i] = expf32(src[i])
	}
}

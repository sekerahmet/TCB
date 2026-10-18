//go:build !amd64.v3

package tensor

// expVek dst[i] = e^src[i] (dst ve src aynı dilim olabilir). Taşınabilir
// derlemede skaler expf32 döngüsüdür; GOAMD64=v3'te AVX2 çekirdeği kullanılır.
func expVek(dst, src []float32) {
	dst = dst[:len(src)]
	for i, v := range src {
		dst[i] = expf32(v)
	}
}

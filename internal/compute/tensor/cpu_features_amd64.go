package tensor

import "os"

// İşlemci yetenekleri çalışma anında okunur: aynı program dosyası AVX2/FMA'lı işlemcide
// assembly çekirdeklerini, öbürlerinde taşınabilir Go eşlerini kullanır (GOAMD64 düzeyine
// göre ayrı derleme gerekmez). TR_CPU_AVX2=0 çekirdekleri kapatır (sorun ayıklama ve Go
// eşlerinin sınanması için).

//go:noescape
func xgetbv0() (eax uint32)

// hasAVX2 AVX2 ve FMA çekirdekleri kullanılabilir mi. Testler Go eşlerini sınamak için
// geçici olarak değiştirebilir (useAVX2For).
var hasAVX2 = detectAVX2() && os.Getenv("TR_CPU_AVX2") != "0"

// detectAVX2 işlemcinin AVX2, FMA ve AVX'i desteklediğini ve işletim sisteminin ymm
// yazmaçlarını kaydettiğini (OSXSAVE, XCR0 bit 1 ve 2) denetler.
func detectAVX2() bool {
	maxLeaf, _, _, _ := cpuid(0, 0)
	if maxLeaf < 7 {
		return false
	}
	_, _, ecx1, _ := cpuid(1, 0)
	const fma, osxsave, avx = 1 << 12, 1 << 27, 1 << 28
	if ecx1&(fma|osxsave|avx) != fma|osxsave|avx || xgetbv0()&6 != 6 {
		return false
	}
	_, ebx7, _, _ := cpuid(7, 0)
	return ebx7&(1<<5) != 0
}

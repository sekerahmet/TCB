package tensor

import (
	"slices"
	"testing"
)

// checkReportAgainstKernels raporu çekirdeklerin kullandığı okumalarla karşılaştırır.
func checkReportAgainstKernels(t *testing.T, got []string) {
	t.Helper()
	if detectAVX2() {
		for _, f := range []string{"avx", "avx2", "fma"} {
			if !slices.Contains(got, f) {
				t.Fatalf("detectAVX2 doğru ama %q listede yok: %v", f, got)
			}
		}
	}
	if slices.Contains(got, "f16c") && !detectF16C() {
		t.Fatalf("f16c listede ama detectF16C yanlış: %v", got)
	}
	if slices.Contains(got, "avx512f") && !slices.Contains(got, "avx") {
		t.Fatalf("avx512f avx'siz olamaz: %v", got)
	}
}

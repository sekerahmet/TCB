package tensor

import (
	"runtime"
	"slices"
	"testing"
)

// TestCPUFeaturesReportTable: liste tablonun öğelerinden, tablonun sırasıyla ve tekrarsızdır;
// amd64'te çekirdeklerin kullandığı okumalarla (detectAVX2, detectF16C) tutarlıdır.
func TestCPUFeaturesReportTable(t *testing.T) {
	table := map[string][]string{
		"amd64": {"sse4_2", "avx", "avx2", "fma", "f16c", "avx512f"},
		"arm64": {"neon"},
	}[runtime.GOARCH]
	got := CPUFeatures()
	if got == nil {
		t.Fatalf("CPUFeatures nil döndü; boş liste bekleniyordu")
	}
	last := -1
	for _, f := range got {
		i := slices.Index(table, f)
		if i < 0 {
			t.Fatalf("%q tabloda yok (%s tablosu %v); liste %v", f, runtime.GOARCH, table, got)
		}
		if i <= last {
			t.Fatalf("liste tablonun sırasında ya da tekrarsız değil: %v", got)
		}
		last = i
	}
	switch runtime.GOARCH {
	case "arm64":
		if !slices.Equal(got, []string{"neon"}) {
			t.Fatalf("arm64'te [neon] bekleniyordu: %v", got)
		}
	}
	checkReportAgainstKernels(t, got)
	t.Logf("%s: %v", runtime.GOARCH, got)
}

//go:build !amd64

package tensor

import "testing"

// checkReportAgainstKernels amd64 dışında karşılaştırılacak çekirdek okuması yoktur.
func checkReportAgainstKernels(t *testing.T, got []string) { t.Helper() }

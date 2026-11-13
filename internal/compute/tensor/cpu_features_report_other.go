//go:build !amd64

package tensor

import "runtime"

// cpuFeatureReport amd64 dışında: arm64'te Advanced SIMD (NEON) AArch64'ün zorunlu
// parçasıdır, hep vardır; öbür mimarilerde tablo boştur.
func cpuFeatureReport() []string {
	if runtime.GOARCH == "arm64" {
		return []string{"neon"}
	}
	return []string{}
}

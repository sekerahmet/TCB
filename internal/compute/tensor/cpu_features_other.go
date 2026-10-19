//go:build !amd64

package tensor

// hasAVX2 yalnızca amd64'te anlamlıdır (bkz. cpu_features_amd64.go).
var hasAVX2 = false

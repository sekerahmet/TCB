//go:build !amd64 || amd64.v3

package detmath

// goUnfused Go'nun math paketinin bu derlemede birleşmediğini söyler (GOAMD64 v1 ya da v2).
const goUnfused = false

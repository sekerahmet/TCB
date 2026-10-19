//go:build !amd64

package tensor

// tscVar amd64 dışında yoktur: sayaçlar Go'nun monoton saatini kullanır.
const tscVar = false

// saat sayaçların tık birimindeki (nanosaniye) şimdiki zamandır.
func saat() int64 { return monotonNs() }

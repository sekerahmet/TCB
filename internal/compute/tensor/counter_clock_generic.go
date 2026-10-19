//go:build !amd64

package tensor

// tscVar amd64 dışında yoktur: sayaçlar Go'nun monoton saatini kullanır.
const hasTSC = false

// saat sayaçların tık birimindeki (nanosaniye) şimdiki zamandır.
func clock() int64 { return monotonicNs() }

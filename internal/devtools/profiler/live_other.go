//go:build !windows

package profiler

import "os"

// enableControlSeqs Unix terminallerinde denetim dizileri açıktır; yalnızca TERM=dumb
// (dizileri anlamayan terminal) düz satırlara düşer.
func enableControlSeqs(*os.File) (geri func(), ok bool) {
	return func() {}, os.Getenv("TERM") != "dumb"
}

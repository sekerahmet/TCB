//go:build !windows

package profiler

import "os"

// denetimDizileriniAc Unix terminallerinde denetim dizileri açıktır; yalnızca TERM=dumb
// (dizileri anlamayan terminal) düz satırlara düşer.
func denetimDizileriniAc(*os.File) (geri func(), ok bool) {
	return func() {}, os.Getenv("TERM") != "dumb"
}

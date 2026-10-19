//go:build !amd64

package tensor

// cpuPause dönerek beklemede çağrılır. Satır içine alınmayan ve başka bir
// işlevi çağıran (yaprak olmayan) bir işlev olduğundan girişindeki yığın
// denetimi çöp toplayıcı için işbirlikçi kesme noktasıdır (bkz. bekle_amd64.go).
//
//go:noinline
func cpuPause() { idleWait() }

//go:noinline
func idleWait() {}

//go:build amd64

package tensor

// tsc işlemcinin zaman damgası sayacını okur (RDTSC; bkz. sayac_saat_amd64.s).
//
//go:noescape
func tsc() int64

// cpuid CPUID komutunu verilen yaprak ve alt yaprakla çalıştırır.
//
//go:noescape
func cpuid(leaf, sub uint32) (eax, ebx, ecx, edx uint32)

// hasTSC işlemci değişmez (çekirdekler arasında eş, frekanstan bağımsız) zaman damgası
// sayacını bildiriyor mu: CPUID 0x80000007, EDX bit 8.
var hasTSC = func() bool {
	if largest, _, _, _ := cpuid(0x80000000, 0); largest < 0x80000007 {
		return false
	}
	_, _, _, edx := cpuid(0x80000007, 0)
	return edx&(1<<8) != 0
}()

// clock sayaçların tık birimindeki şimdiki zamandır.
func clock() int64 {
	if hasTSC {
		return tsc()
	}
	return monotonicNs()
}

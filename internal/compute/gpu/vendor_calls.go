package gpu

import "github.com/sekerahmet/TCB/internal/compute/tensor"

// VendorCalls "gpu" cihazında TR_vendor_matmul bloğunun kitaplık girişine (vendorMul) gelen
// çarpımların sayısıdır; cihaz yoksa 0 (yalnız sınama). Benzeticide kitaplık yoktur, çarpım
// kendi çekirdekle sürer; sayaç bloğun durumunun dil düzeyinden girişe ulaştığını gösterir
// (15.53b; yorumlayıcının bağladığı yerlerin testi ml paketinde).
func VendorCalls() int {
	if !tensor.HasDevice(tensor.GPU(0)) {
		return 0
	}
	g, ok := tensor.BackendFor(tensor.GPU(0)).(*backend)
	if !ok {
		return 0
	}
	g.enter()
	defer g.exit()
	return g.vendorCalls
}

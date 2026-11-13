package gpu

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// DeviceInfo açılmış bir GPU'nun adını, toplam belleğini (bayt) ve sürücü arayüzünün
// sürümünü ("<büyük>.<küçük>", CUDA'da cuDriverGetVersion: 12040 → "12.4") verir
// (15.37a, TR_device_info). Cihaz bu süreçte henüz açılmadıysa ok yanlıştır: çağıran önce
// cihazı açtırır (tensor.HasDevice). Benzetimde (TR_GPU_SIM=1) değerler sabittir: ad
// "TR_GPU_SIM", bellek benzetimin kapasitesi, sürüm "0.0". Bilinmeyen bir sürücüde ok
// yanlıştır.
func DeviceInfo(dev tensor.Device) (name string, memory uint64, driverVersion string, ok bool) {
	for _, be := range registeredAll() {
		if be.dev == dev {
			return deviceInfoOf(be)
		}
	}
	return "", 0, "", false
}

// deviceInfoOf arka ucun sürücüsünden okur. Okunan alanlar cihaz açılırken bir kez yazılır
// ve sonra değişmez; kilit gerekmez.
func deviceInfoOf(be *backend) (string, uint64, string, bool) {
	switch sr := be.sr.(type) {
	case *cudaDriver:
		v := sr.c.version
		return sr.ad, sr.memory, fmt.Sprintf("%d.%d", v/1000, v%1000/10), true
	case *simDriver:
		return "TR_GPU_SIM", sr.memory, "0.0", true
	}
	return "", 0, "", false
}

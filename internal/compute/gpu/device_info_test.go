package gpu

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

func TestDeviceInfoOfDrivers(t *testing.T) {
	cuda := &backend{dev: tensor.GPU(3), sr: &cudaDriver{c: &cudaLibrary{version: 12040}, ad: "Sınama Kartı", memory: 24 << 30}}
	name, mem, ver, ok := deviceInfoOf(cuda)
	if !ok || name != "Sınama Kartı" || mem != 24<<30 || ver != "12.4" {
		t.Fatalf("cuda: %q %d %q %v", name, mem, ver, ok)
	}
	cuda.sr.(*cudaDriver).c.version = 13010
	if _, _, ver, _ := deviceInfoOf(cuda); ver != "13.1" {
		t.Fatalf("13010 → %q, beklenen 13.1", ver)
	}
	sim := &backend{dev: tensor.GPU(4), sr: newSim(64 << 20)}
	name, mem, ver, ok = deviceInfoOf(sim)
	if !ok || name != "TR_GPU_SIM" || mem != 64<<20 || ver != "0.0" {
		t.Fatalf("benzetim: %q %d %q %v", name, mem, ver, ok)
	}
}

func TestDeviceInfoUnopenedDevice(t *testing.T) {
	if name, mem, ver, ok := DeviceInfo(tensor.GPU(97)); ok || name != "" || mem != 0 || ver != "" {
		t.Fatalf("açılmamış cihaz: %q %d %q %v", name, mem, ver, ok)
	}
}

package gpuprobe_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Kural (15.37a): cihaz_bilgisi("gpu") kartın adını, toplam belleğini ve sürücü arayüzünün
// sürümünü ("<büyük>.<küçük>") verir. Gerçek kartta (G4) ad boş değil, bellek pozitif; kartsız
// makinede PTX benzetiminin sabit değerleri.
const gpuDeviceInfoProgram = `
TR_var b = TR_device_info("gpu")
TR_print(b.TR_name)
TR_print(b.TR_memory_total_bytes)
TR_print(b.TR_driver_version)
TR_print(TR_Type(b) == TR_DeviceInfo, TR_device_info("gpu:0").TR_name == b.TR_name)
`

func TestGPUDeviceInfo(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	lines := strings.Split(strings.TrimSpace(trrun.MustExec(t, gpuDeviceInfoProgram).Out), "\n")
	if len(lines) != 4 {
		t.Fatalf("dört satır bekleniyordu: %q", lines)
	}
	name, driver := lines[0], lines[2]
	memory, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil || memory <= 0 {
		t.Fatalf("bellek %q", lines[1])
	}
	if !regexp.MustCompile(`^\d+\.\d+$`).MatchString(driver) {
		t.Fatalf("sürücü sürümü %q, beklenen <büyük>.<küçük>", driver)
	}
	if lines[3] != "true true" {
		t.Fatalf("tip ve kısa ad: %q", lines[3])
	}
	if gpu.Simulated() {
		if name != "TR_GPU_SIM" || memory != 1<<30 || driver != "0.0" {
			t.Fatalf("benzetimin sabit değerleri bekleniyordu: %q %d %q", name, memory, driver)
		}
		return
	}
	if name == "" || driver == "0.0" {
		t.Fatalf("gerçek kartta ad ve sürücü: %q %q", name, driver)
	}
	t.Logf("kart: %s, %d bayt, sürücü %s", name, memory, driver)
}

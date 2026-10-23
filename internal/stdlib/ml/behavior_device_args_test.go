package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (dil tanımı 15.38): bir tensörle işleme giren liste argümanı varsayılan cihazda değil,
// tensörün cihazında tensöre çevrilir (katman_normu ve dikkat'in maske listesi dahil).
// Neden: normParam ve maske listesi varsayılan cihazda çevriliyordu.
func TestListArgumentsFollowTheInputDevice(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	_, err := run(t, `varsayılan_cihaz("cpu")
değişken x = birler([1, 2]).cihaza("gpu")
değişken y = katman_normu(x, [1.0, 1.0], [0.0, 0.0])`)
	if err != nil {
		t.Errorf("katman_normu liste argümanlarını x'in cihazında çevirmeliydi: %v", err)
	}
}

// Kural (dil tanımı 15.40): farklı cihaz hatası kullanıcıya Go adını değil .cihaza(...) önerisini gösterir.
// Neden: çekirdek iletisi "önce To ile taşıyın" diyordu.
func TestDeviceMismatchMessageNamesTheTR(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)

	_, err := run(t, `değişken x = birler([1, 2]).cihaza("cpu")
değişken g = birler([2]).cihaza("gpu")
katman_normu(x, g, yok)`)
	if err == nil || !strings.Contains(err.Error(), ".cihaza(") || strings.Contains(err.Error(), "To ile") {
		t.Errorf("katman_normu farklı cihaz hatası .cihaza(...) önermeli, Go adını göstermemeli: %v", err)
	}
}

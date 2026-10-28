package checkpoint_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Durdurma isteğinin kontrol noktasındaki yolu (1.6): istek ikinci güncellemeden önce gelirse
// ertelenmez; ertelenen istek ilk güncelle ya da kaydet'te bir kez kaydedilir; program
// durdurmayı üstlendiyse nokta yalnız kaydedilir; erteleme süresi dolarsa istek uygulanır;
// bitişte ek kayıt yapılmaz.

// stopRun kontrol noktalı küçük bir programı çalıştırır; kesme_iste(), istekle ilgili sonucu
// (karşılanma yolunun adını) verir ve ilk argümanıyla sinyal numarasını alır. Saat her okunuşta tick
// kadar ilerler.
func stopRun(t *testing.T, tick time.Duration, body string) (output, runDir string, m *interp.VM, err error) {
	t.Helper()
	ticks := 0
	return stopRunClock(t, func() time.Time { ticks++; return time.Unix(0, 0).Add(time.Duration(ticks) * tick) }, nil, body)
}

// stopRunClock stopRun'ın saati dışarıdan verilen biçimidir; setup varsa VM'ye ek yerleşikler tanımlar.
func stopRunClock(t *testing.T, clock func() time.Time, setup func(*interp.VM), body string) (output, runDir string, m *interp.VM, err error) {
	t.Helper()
	runDir = t.TempDir()
	m = interp.New()
	w := &testkit.TeeWriter{}
	m.Out, m.Err = w, w
	defer checkpoint.FakeClock(clock)()
	m.GlobalEnv.Define("kesme_iste", &interp.BuiltinVal{Name: "kesme_iste", Fn: func(m *interp.VM, _ []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		return interp.NewInt(int64(m.RequestStop(2))), nil
	}}, true)
	if setup != nil {
		setup(m)
	}
	_, err = runVM(t, m, fmt.Sprintf(`# TR 4
TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)
TR_var kn = TR_checkpoint(%q, {w: w})
`, trPath(runDir))+body)
	return w.String(), runDir, m, err
}

func hasStep(runDir string, step int) bool {
	_, err := os.Stat(filepath.Join(runDir, "checkpoints", fmt.Sprintf("step_%010d", step)))
	return err == nil
}

// İkinci .güncelle'den önce gelen istek ertelenmez: 1.5'teki gibi uygulanır, nokta kaydedilmez.
func TestStopBeforeTheSecondUpdateIsNotDeferred(t *testing.T) {
	output, runDir, _, err := stopRun(t, time.Second, `
TR_for i TR_in 0..5 {
    TR_if i == 1 { kesme_iste() }
    kn.TR_update(i + 1)
    TR_print("adım bitti", i + 1)
}
`)
	if !errors.Is(err, interp.ErrInterrupted) {
		t.Fatalf("program durmalıydı: %v\n%s", err, output)
	}
	if strings.Contains(output, "Kontrol noktası kaydedildi") || hasStep(runDir, 2) {
		t.Errorf("ertelenmeyen istekte nokta kaydedilmemeli:\n%s", output)
	}
	if strings.Contains(output, "adım bitti 3") {
		t.Errorf("program bir sonraki güvenli noktada durmalıydı:\n%s", output)
	}
}

// Ertelenen istek .kaydet ile de karşılanır: kayıt istek için yeniden yapılmaz, nokta kaydedilir ve program
// durur (eskiden iki kez kaydediyordu).
func TestSaveHonoursTheDeferredRequestAndStops(t *testing.T) {
	output, runDir, _, err := stopRun(t, time.Second, `
TR_for i TR_in 0..3 { kn.TR_update(i + 1) }
kesme_iste()
kn.TR_save()
TR_print("sürdü (görünmemeli)")
`)
	if !errors.Is(err, interp.ErrInterrupted) {
		t.Fatalf("program durmalıydı: %v\n%s", err, output)
	}
	if n := strings.Count(output, "Kontrol noktası kaydedildi"); n != 1 {
		t.Errorf("nokta bir kez kaydedilmeli, %d kez kaydedildi:\n%s", n, output)
	}
	if strings.Contains(output, "sürdü") {
		t.Errorf("kayıttan sonra program durmalıydı:\n%s", output)
	}
	if !hasStep(runDir, 3) {
		t.Errorf("3. adımın noktası yok:\n%s", output)
	}
}

// Program durdurmayı üstlendiyse istek ertelenmez, kaydedilir: ilk güncelle noktayı kaydeder ve
// program durmaz; sonraki güncellemeler istek için yeniden kaydetmez (1.6).
func TestHandledStopCheckpointOnlySaves(t *testing.T) {
	output, runDir, m, err := stopRun(t, time.Second, `
TR_handle_stop()
TR_for i TR_in 0..6 {
    TR_if i == 2 { kesme_iste() }
    kn.TR_update(i + 1)
}
TR_print("döngü bitti")
`)
	if err != nil {
		t.Fatalf("üstlenen program kendi sınırında bitmeli: %v\n%s", err, output)
	}
	if n := strings.Count(output, "Kontrol noktası kaydedildi"); n != 1 || !strings.Contains(output, "döngü bitti") {
		t.Errorf("nokta bir kez kaydedilmeli, program sürmeli (%d):\n%s", n, output)
	}
	if !hasStep(runDir, 3) {
		t.Errorf("3. adımın noktası yok:\n%s", output)
	}
	if end := m.End(nil); end.Signal != 2 {
		t.Errorf("sona gelen üstlenmiş program isteğin sinyaliyle bitmeli: %+v", end)
	}
}

// Erteleme süresi (şimdiye dek iki güncelleme arasındaki en uzun sürenin iki katı) dolarsa istek
// uygulanır ve bekleyen yerleşik uyanır; nokta kaydedilmez. Saat her okunuşta bir milisaniye ilerler.
func TestDeferralExpiresWhileWaiting(t *testing.T) {
	start := time.Now()
	output, runDir, _, err := stopRun(t, time.Millisecond, `
TR_for i TR_in 0..5 {
    kn.TR_update(i + 1)
    TR_if i == 3 {
        kesme_iste()
        TR_sleep(60)
    }
}
`)
	if !errors.Is(err, interp.ErrInterrupted) {
		t.Fatalf("erteleme süresi dolunca istek uygulanmalıydı: %v\n%s", err, output)
	}
	if time.Since(start) > 30*time.Second {
		t.Error("uyku süre dolunca uyanmalıydı")
	}
	if strings.Contains(output, "Kontrol noktası kaydedildi") || hasStep(runDir, 4) {
		t.Errorf("kayda varılmadığı için istek için nokta yazılmamalı:\n%s", output)
	}
}

// Program kayda varmadan biterse ek kayıt yapılmaz; istek kaybolmaz, süreç isteğin sinyaliyle
// biter (eskiden çıkış 0'dı).
func TestNoExtraSaveAtTheEndAndTheRequestIsKept(t *testing.T) {
	output, runDir, m, err := stopRun(t, time.Second, `
kn.TR_update(1)
kn.TR_update(2)
kesme_iste()
TR_print("bitti")
`)
	if err != nil {
		t.Fatalf("program kayda varmadan sona gelmeli: %v\n%s", err, output)
	}
	if strings.Contains(output, "Kontrol noktası kaydedildi") || hasStep(runDir, 2) || hasStep(runDir, 3) {
		t.Errorf("bitişte ek kayıt yapılmamalı:\n%s", output)
	}
	if end := m.End(nil); end.Signal != 2 {
		t.Errorf("istek kaybolmamalı, süreç isteğin sinyaliyle bitmeli: %+v", end)
	}
}

// Erteleme süresi son aralığa değil şimdiye dek görülen en uzun aralığa bağlıdır (1.6): en uzun
// aralık 2 sn iken son aralık 10 ms olsa da süre 4 sn'dir; 300 ms'lik bir bekleme isteği uygulamaz,
// sıradaki .güncelle onu karşılar (kaydeder ve durur). Saat yalnız saat_ileri ile ilerler.
func TestDeferralWindowIsTwiceTheLongestInterval(t *testing.T) {
	var now time.Duration
	clock := func() time.Time { return time.Unix(0, 0).Add(now) }
	setup := func(m *interp.VM) {
		m.GlobalEnv.Define("saat_ileri", &interp.BuiltinVal{Name: "saat_ileri", Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			now += time.Duration(args[0].(*interp.IntVal).Val) * time.Millisecond
			return interp.NewInt(0), nil
		}}, true)
	}
	output, runDir, _, err := stopRunClock(t, clock, setup, `
kn.TR_update(1)
saat_ileri(2000)
kn.TR_update(2)
saat_ileri(10)
kn.TR_update(3)
kesme_iste()
TR_sleep(0.3)
TR_print("bekledi")
kn.TR_update(4)
TR_print("sürdü (görünmemeli)")
`)
	if !errors.Is(err, interp.ErrInterrupted) {
		t.Fatalf("sıradaki güncelleme isteği karşılayıp programı durdurmalıydı: %v\n%s", err, output)
	}
	if !strings.Contains(output, "bekledi") {
		t.Errorf("erteleme süresi en uzun aralığa göre uzun olmalı, bekleme kesilmemeliydi:\n%s", output)
	}
	if strings.Contains(output, "sürdü") || !hasStep(runDir, 4) {
		t.Errorf("4. adımın noktası kaydedilip program durmalıydı:\n%s", output)
	}
}

package checkpoint_test

import (
	"fmt"
	"strings"
	"testing"
)

// Kural (dil tanımı 20.2): düz değer yerinde yenilenirken aynı iç kap birden çok yerden bağlıysa
// ({a: l, b: l}) yalnızca ilk yerde (anahtar sırasıyla) yerinde yenilenir, öbür yerler kayıttaki
// kendi değerini alır; ikinci yenileme birinciyi ezmez, yükleme ve geri_dön belirlenimcidir.
// (Gemini gerileme denetimi, Bulgu 1.)
func TestRefreshKeepsSharedContainersApart(t *testing.T) {
	runDir := trPath(t.TempDir())

	// 1. koşu: kontrol noktasına izlenen = {a: [10], b: [20]} kaydedilir.
	saveCode := fmt.Sprintf(`
TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)
TR_var izlenen = {a: [10], b: [20]}
TR_var kn = TR_checkpoint(%q, {w: w, izlenen: izlenen})
kn.TR_update(1)
kn.TR_save()
`, runDir)
	if out, err := run(t, saveCode); err != nil {
		t.Fatalf("kayıt hatası: %v\n%s", err, out)
	}

	// 2. koşu: yeniden başlatmada izlenen = {a: l, b: l} ile yüklenir; ardından
	// geri_dön() içinde de aynı durum sınanır.
	loadCode := fmt.Sprintf(`
TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)
TR_var l = [0]
TR_var izlenen = {a: l, b: l}
TR_var kn = TR_checkpoint(%q, {w: w, izlenen: izlenen})
TR_print("YUKLE_A:", izlenen.a)
TR_print("YUKLE_B:", izlenen.b)

TR_var l2 = [99]
izlenen.a = l2
izlenen.b = l2
kn.TR_rollback()
TR_print("GERI_A:", izlenen.a)
TR_print("GERI_B:", izlenen.b)
`, runDir)
	out, err := run(t, loadCode)
	if err != nil {
		t.Fatalf("yükleme hatası: %v\n%s", err, out)
	}
	out = strings.ReplaceAll(out, runDir, "<KOŞU>")
	if !strings.Contains(out, "YUKLE_A: [10]") || !strings.Contains(out, "YUKLE_B: [20]") ||
		!strings.Contains(out, "GERI_A: [10]") || !strings.Contains(out, "GERI_B: [20]") {
		t.Fatalf("izlenen = {a: l, b: l} durumunda kayıttaki {a: [10], b: [20]} korunmalıydı, alınan çıktı:\n%s", out)
	}
}

package checkpoint_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// TestNoGradTensorsSaved: modelin içindeki türev izlemeyen tensörler (TR ile
// yazılmış iyileştiricinin momentumu, ara tensör olmayan her yaprak) kaydedilir ve yüklemede
// var olan tensöre yerinde yazılır; ara sonuçlar kaydedilmez.
func TestNoGradTensorsSaved(t *testing.T) {
	runDir := trPath(t.TempDir())
	if output, err := run(t, fmt.Sprintf(`
TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var model = {"w": w, "state": {"m": TR_Tensor([5.0, 6.0]), "step": TR_Tensor(3.0)}}
TR_var kn = TR_Checkpoint(%q, {"model": model, "tek": TR_Tensor([[1.0], [2.0]], TR_dtype: "bf16")})
kn.TR_update(4)
kn.TR_save()
`, runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	output, err := run(t, fmt.Sprintf(`
TR_var w = TR_Tensor([0.0, 0.0], TR_requires_grad: TR_true)
TR_var m = TR_Tensor([0.0, 0.0])
TR_var model = {"w": w, "state": {"m": m, "step": TR_Tensor(0.0)}}
TR_var tek = TR_Tensor([[0.0], [0.0]], TR_dtype: "bf16")
TR_var kn = TR_Checkpoint(%q, {"model": model, "tek": tek})
TR_print(kn.TR_resume_step, w.TR_tolist, m.TR_tolist, model["state"]["m"].TR_tolist, model["state"]["step"].TR_tolist, tek.TR_tolist, tek.TR_dtype)
`, runDir))
	if err != nil || !strings.Contains(output, "4 [1.0, 2.0] [5.0, 6.0] [5.0, 6.0] 3.0 [[1.0], [2.0]] bf16") {
		t.Fatalf("durum tensörleri yerinde yüklenmedi: %v\n%s", err, output)
	}
	// Kayıtta olan bir durum tensörü modelde yoksa ya da şekli farklıysa açık hata.
	for _, d := range []struct {
		model string
		want  messages.Code
	}{
		{`{"w": TR_Tensor([0.0, 0.0], TR_requires_grad: TR_true), "state": {"m": TR_Tensor([0.0, 0.0])}}`, messages.H7452}, // "state.step" yok
		{`{"w": TR_Tensor([0.0, 0.0], TR_requires_grad: TR_true), "state": {"m": TR_Tensor([0.0]), "step": TR_Tensor(0.0)}}`, messages.H7450},
		{`{"w": TR_Tensor([0.0, 0.0], TR_requires_grad: TR_true), "state": {"m": TR_Tensor([0.0, 0.0]), "step": TR_Tensor(0.0), "yeni": TR_Tensor([1.0])}}`, messages.H7449},
	} {
		output, err := run(t, fmt.Sprintf(`TR_Checkpoint(%q, {"model": %s, "tek": TR_Tensor([[0.0], [0.0]], TR_dtype: "bf16")})`, runDir, d.model))
		if !messages.Has(err, d.want) {
			t.Errorf("%s bekleniyordu: %v\n%s", d.want, err, output)
		}
	}
	// Ara sonuç (türev izleyen bir işlemin sonucu) tek başına verilemez; modelin içindeyse atlanır.
	output, err = run(t, fmt.Sprintf(`
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_Checkpoint(%q, {"ara": w * 2})
`, trPath(t.TempDir())))
	if !messages.Has(err, messages.H7437) {
		t.Errorf("ara tensör hatası bekleniyordu: %v\n%s", err, output)
	}
}

// Kayıttaki bir komut satırı ayarı yeni koşuda yoksa da fark olarak bildirilir.
func TestRemovedSettingReported(t *testing.T) {
	runDir := trPath(t.TempDir())
	program := `TR_var ayar = TR_command_line(%s)
TR_var w = TR_Tensor([1.0], TR_requires_grad: TR_true)
TR_var kn = TR_Checkpoint(%q, {"w": w})
kn.TR_save()`
	if output, err := run(t, fmt.Sprintf(program, `{"oran": 0.1, "katman": 2}`, runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	output, err := run(t, fmt.Sprintf(program, `{"oran": 0.1}`, runDir))
	if err != nil || !strings.Contains(output, "katman: 2 → yok") {
		t.Fatalf("kaldırılan ayar bildirilmedi: %v\n%s", err, output)
	}
}

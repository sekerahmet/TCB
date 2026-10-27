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
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken model = {w: w, durum: {m: tensör([5.0, 6.0]), adım: tensör(3.0)}}
değişken kn = kontrol_noktası(%q, {model: model, tek: tensör([[1.0], [2.0]], tür: "bf16")})
kn.güncelle(4)
kn.kaydet()
`, runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	output, err := run(t, fmt.Sprintf(`
değişken w = tensör([0.0, 0.0], türev_izle: doğru)
değişken m = tensör([0.0, 0.0])
değişken model = {w: w, durum: {m: m, adım: tensör(0.0)}}
değişken tek = tensör([[0.0], [0.0]], tür: "bf16")
değişken kn = kontrol_noktası(%q, {model: model, tek: tek})
yazdır(kn.kaldığı_adım, w.değer, m.değer, model.durum.m.değer, model.durum.adım.değer, tek.değer, tek.tür)
`, runDir))
	if err != nil || !strings.Contains(output, "4 [1.0, 2.0] [5.0, 6.0] [5.0, 6.0] 3.0 [[1.0], [2.0]] bf16") {
		t.Fatalf("durum tensörleri yerinde yüklenmedi: %v\n%s", err, output)
	}
	// Kayıtta olan bir durum tensörü modelde yoksa ya da şekli farklıysa açık hata.
	for _, d := range []struct {
		model string
		want  messages.Code
	}{
		{`{w: TR_tensor([0.0, 0.0], TR_requires_grad: TR_true), TR_state: {m: TR_tensor([0.0, 0.0])}}`, messages.H7452}, // "state.step" yok
		{`{w: TR_tensor([0.0, 0.0], TR_requires_grad: TR_true), TR_state: {m: TR_tensor([0.0]), TR_step: TR_tensor(0.0)}}`, messages.H7450},
		{`{w: TR_tensor([0.0, 0.0], TR_requires_grad: TR_true), TR_state: {m: TR_tensor([0.0, 0.0]), TR_step: TR_tensor(0.0), yeni: TR_tensor([1.0])}}`, messages.H7449},
	} {
		output, err := run(t, fmt.Sprintf(`kontrol_noktası(%q, {model: %s, tek: tensör([[0.0], [0.0]], tür: "bf16")})`, runDir, d.model))
		if !messages.Has(err, d.want) {
			t.Errorf("%s bekleniyordu: %v\n%s", d.want, err, output)
		}
	}
	// Ara sonuç (türev izleyen bir işlemin sonucu) tek başına verilemez; modelin içindeyse atlanır.
	output, err = run(t, fmt.Sprintf(`
değişken w = tensör([1.0], türev_izle: doğru)
kontrol_noktası(%q, {ara: w * 2})
`, trPath(t.TempDir())))
	if !messages.Has(err, messages.H7437) {
		t.Errorf("ara tensör hatası bekleniyordu: %v\n%s", err, output)
	}
}

// Kayıttaki bir komut satırı ayarı yeni koşuda yoksa da fark olarak bildirilir.
func TestRemovedSettingReported(t *testing.T) {
	runDir := trPath(t.TempDir())
	program := `değişken ayar = komut_satırı(%s)
değişken w = tensör([1.0], türev_izle: doğru)
değişken kn = kontrol_noktası(%q, {w: w})
kn.kaydet()`
	if output, err := run(t, fmt.Sprintf(program, "{oran: 0.1, katman: 2}", runDir)); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	output, err := run(t, fmt.Sprintf(program, "{oran: 0.1}", runDir))
	if err != nil || !strings.Contains(output, "katman: 2 → yok") {
		t.Fatalf("kaldırılan ayar bildirilmedi: %v\n%s", err, output)
	}
}

package checkpoint_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/stdlib/checkpoint"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// rollbackProgram trainingProgram'ın eğitimidir; tek farkı corruptAt adımında kaybın NaN'a
// dönmesi, iyileştiricinin bu bozuk gradyanla adım atması (parametreler bozulur) ve döngünün
// bunu görünce kn.geri_dön() ile son noktaya dönüp oradan sürmesidir. corruptAt < 0 bozmaz.
func rollbackProgram(runDir string, total, corruptAt int) string {
	return fmt.Sprintf(`
TR_var KOŞU = %q
TR_var V = 4096
TR_var D = 16
TR_var sayaç = 0
TR_func parametre(şekil, ölçek) {
    sayaç += 1
    TR_return TR_tensor(TR_no_grad(TR_randn(şekil, TR_seed: sayaç) * ölçek), TR_requires_grad: TR_true)
}
TR_var model = {E: parametre([V, D], 0.1), W: parametre([D, V], 0.1)}
TR_var ikinci = [parametre([D], 1.0)]
TR_var op = TR_AdamW(TR_parameters(model), TR_lr: 0.01)
TR_var op2 = TR_SGD(ikinci, TR_lr: 0.1, TR_momentum: 0.9)
TR_var küme = TR_dataset(%q)
TR_var ö = küme.TR_sampler(TR_batch_size: 4, TR_context: 8, TR_seed: 5)
TR_var s = küme.TR_sequential(TR_batch_size: 2, TR_context: 8)
TR_var g = TR_logger(TR_path(KOŞU, "metrics.jsonl"))
TR_var izlenen = {en_iyi: 99.0, geçmiş: [], son_sıralı: TR_none}
TR_var kn = TR_checkpoint(KOŞU, {model: model, op: op, ikinci: ikinci, op2: op2, ö: ö, s: s, g: g, izlenen: izlenen}, TR_every_minutes: 3)
TR_var ç = TR_progress(%d, TR_log: g)
TR_var bozuldu = TR_false
TR_var adım = kn.TR_resume_step
TR_for adım < %d {
    TR_var giriş, hedef = ö.TR_next()
    op.TR_zero_grad()
    op2.TR_zero_grad()
    TR_var kayıp = TR_cross_entropy(TR_embedding(model.E, giriş) @ model.W, hedef) + TR_sum(ikinci[0] * ikinci[0]) * 0.01
    TR_if adım == %d TR_and TR_not bozuldu {
        kayıp = kayıp * TR_float("nan")
        bozuldu = TR_true
    }
    TR_backward(kayıp)
    op.TR_step()
    op2.TR_step()
    TR_if TR_not TR_is_finite(kayıp.TR_tolist) {
        TR_print("bozuk:", TR_is_finite(TR_sum(model.W).TR_tolist))
        adım = kn.TR_rollback()
        TR_print("geri:", adım)
        TR_continue
    }
    TR_var d = s.TR_next()
    TR_if d != TR_none { izlenen.son_sıralı = d[0].TR_tolist[0][0] }
    TR_if kayıp.TR_tolist < izlenen.en_iyi { izlenen.en_iyi = kayıp.TR_tolist }
    izlenen.geçmiş.TR_append(adım)
    ç.TR_update({TR_loss: kayıp}, TR_step: adım + 1)
    adım += 1
    kn.TR_update(adım)
}
TR_save_tensors(TR_path(KOŞU, "son.trt"), TR_parameters([model, ikinci], TR_named: TR_true))
TR_save_tensors(TR_path(KOŞU, "op.trt"), op.TR_state)
TR_save_tensors(TR_path(KOŞU, "op2.trt"), op2.TR_state)
TR_print("izlenen:", izlenen)
TR_print("ö:", ö.TR_state)
`, trPath(runDir), trPath(filepath.Join(reporoot.Root(), "tests", "data", "text")), total, total, corruptAt)
}

// Kural (20.14): kn.geri_dön() en yeni geçerli noktayı aynı süreçte yerinde yükler —
// parametreler, iyileştirici durumları, okuyucular, düz değerler, günlüğe devam kaydı — ve
// noktanın adımını verir. NaN'la bozulan bir adımdan geri dönüp sürdürülen eğitim, hiç
// bozulmamış eğitimle bit düzeyinde aynı biter.
// Neden: uzun eğitimde kayıp sıçraması ya da NaN olunca son sağlam noktadan sürmek (PaLM §5.1).
func TestRollbackAfterNaNSameAsUninterrupted(t *testing.T) {
	var now time.Time
	t.Cleanup(checkpoint.FakeClock(func() time.Time { now = now.Add(time.Minute); return now }))
	a := t.TempDir()
	outputA, err := run(t, rollbackProgram(a, 20, -1))
	if err != nil {
		t.Fatalf("%v\n%s", err, outputA)
	}
	now = time.Time{}
	b := t.TempDir()
	outputB, err := run(t, rollbackProgram(b, 20, 11))
	if err != nil {
		t.Fatalf("%v\n%s", err, outputB)
	}
	// Her güncelle saati 1 dk ilerletir, her_dakika: 3 → 3, 6, 9. adımlarda kayıt; 11. adımda
	// bozulan eğitim 9'a döner.
	if !strings.Contains(outputB, "bozuk: yanlış") || !strings.Contains(outputB, "geri: 9") {
		t.Fatalf("bozulup 9. adıma dönmeliydi:\n%s", outputB)
	}
	sameResults(t, a, b, outputA, outputB)
}

// Kural (20.14): nokta yokken geri_dön açık hatadır (H7486); argüman almaz.
func TestRollbackWithoutCheckpointIsError(t *testing.T) {
	dir := trPath(t.TempDir())
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{fmt.Sprintf(`TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)
TR_var kn = TR_checkpoint(%q, {w: w})
kn.TR_rollback()`, dir), messages.H7486},
		{fmt.Sprintf(`TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)
TR_var kn = TR_checkpoint(%q, {w: w})
kn.TR_save()
kn.TR_rollback(1)`, dir), messages.H2012},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// Kural (20.7, 20.14): geri_dön (ve kurulumda yükleme) yazmaya başlamadan önce modelin
// tensörlerinin yazılabilir olduğunu denetler; yazılamayan bir tensör varsa hiçbir nesne
// (okuyucu, parametre, iyileştirici, düz değer) değişmez. Parametreler okuyuculardan önce
// yazılır; yazma yine de yarıda kalırsa (cihaz belleği) okuyucular değişmez.
// Neden: eskiden okuyucular önce sarılıyor, ikinci parametrede hata çıkınca ilk parametre ve
// okuyucu yeni, öbürleri eski kalıyordu (Gemini kararlılık denetimi, bulgu 1). Bırakılmış tensör
// TR kodundan oluşamaz; test onu bir Go yardımcısıyla kurar.
func TestRollbackStopsBeforeWritingWhenAParamIsUnusable(t *testing.T) {
	runDir := t.TempDir()
	dataDir := filepath.Join(reporoot.Root(), "tests", "data", "text")

	m := interp.New()
	w := &testkit.TeeWriter{}
	m.Out, m.Err = w, w
	m.GlobalEnv.Define("tensör_bırak", &interp.BuiltinVal{
		Name: "tensör_bırak",
		Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			args[0].(*ml.TensorValue).Tensor().Drop()
			return interp.Null, nil
		},
	}, true)

	code := fmt.Sprintf(`
TR_var KOŞU = %q
TR_var küme = TR_dataset(%q)
TR_var ö = küme.TR_sampler(TR_batch_size: 2, TR_context: 8, TR_seed: 1)
TR_var model = {
    a: TR_tensor([10.0, 20.0], TR_requires_grad: TR_true),
    b: TR_tensor([30.0, 40.0], TR_requires_grad: TR_true)
}
TR_var op = TR_AdamW([model.a, model.b], TR_lr: 0.01)
TR_var izlenen = {TR_step: 1}
TR_var kn = TR_checkpoint(KOŞU, {model: model, op: op, ö: ö, izlenen: izlenen})

# 1. adımda 1 toplu iş okuyup kontrol noktasını kaydet (ö.durum["counter"] == 1, model.a == [10, 20]).
ö.TR_next()
kn.TR_update(1)
kn.TR_save()

# Eğitim 4. adıma ilerlesin: ö 3 toplu iş daha okur (sayaç == 4), model.a = [1, 2], izlenen.adım = 4.
ö.TR_next()
ö.TR_next()
ö.TR_next()
TR_no_grad { model.a[0..2] = TR_tensor([1.0, 2.0]) }
izlenen.TR_step = 4

# İkinci parametre (model.b) writeParams sırasında hata üretsin.
tensör_bırak(model.b)

TR_var hata_var = TR_false
TR_try {
    kn.TR_rollback()
} TR_catch h {
    hata_var = TR_true
}
TR_print("HATA:", hata_var)
TR_print("OKU_SAYAC:", ö.TR_state["counter"])
TR_print("MODEL_A:", model.a.TR_tolist)
TR_print("IZLENEN_ADIM:", izlenen.TR_step)
`, trPath(runDir), trPath(dataDir))

	if _, err := runVM(t, m, code); err != nil {
		t.Fatalf("beklenmeyen çalışma hatası: %v\n%s", err, w.String())
	}
	output := w.String()
	if !strings.Contains(output, "HATA: doğru") {
		t.Fatalf("geri_dön hata vermeliydi:\n%s", output)
	}
	if !strings.Contains(output, "OKU_SAYAC: 4") || !strings.Contains(output, "MODEL_A: [1.0, 2.0]") || !strings.Contains(output, "IZLENEN_ADIM: 4") {
		t.Fatalf("geri_dön hata verdiğinde hiçbir nesne değişmemeliydi (20.7), ancak nesneler yarı yüklenmiş kaldı:\n%s", output)
	}
}

package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Norm epsilon'u float32'de pozitif ve sonlu olmalıdır: NaN ve sonsuz (f <= 0 denetimini
// geçerdi) ya da float32'de 0'a düşen değer çıktıyı NaN yapardı.
func TestNormEpsilonMustBeFinite(t *testing.T) {
	for _, eps := range []string{"nan", "1e300", "1e-50", "0", "-1"} {
		code := "TR_var nan = TR_log(TR_tensor([-1.0])).TR_tolist[0]\nTR_layer_norm(TR_tensor([[1.0, 2.0]]), TR_eps: " + eps + ")"
		if eps == "nan" {
			code = "TR_var nan = TR_log(TR_tensor([-1.0])).TR_tolist[0]\nTR_rms_norm(TR_tensor([[1.0, 2.0]]), TR_eps: nan)"
		}
		_, err := run(t, code)
		if err == nil || !messages.Has(err, messages.H6325) {
			t.Fatalf("epsilon %s: hata beklenirdi: %v", eps, err)
		}
	}
	mustRun(t, "TR_layer_norm(TR_tensor([[1.0, 2.0]]), TR_eps: 0.001)")
}

// türev_izle Mantık olmalıdır: dolu bir metin ("yanlış") doğru sayılıp türevi açmaz.
func TestRequiresGradMustBeBool(t *testing.T) {
	for _, code := range []string{
		`TR_tensor([1.0, 2.0], TR_requires_grad: "yanlış")`,
		`TR_tensor([1.0, 2.0], 1)`,
		`TR_zeros([2], TR_requires_grad: "doğru")`,
	} {
		_, err := run(t, code)
		if err == nil || !messages.Has(err, messages.H7905) {
			t.Fatalf("%s: hata beklenirdi: %v", code, err)
		}
	}
	mustRun(t, "TR_var q = TR_tensor([1.0], TR_requires_grad: TR_false)\nTR_print(q.TR_grad)")
}

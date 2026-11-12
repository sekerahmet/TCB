package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Norm epsilon'u float32'de pozitif ve sonlu olmalıdır: NaN ve sonsuz (f <= 0 denetimini
// geçerdi) ya da float32'de 0'a düşen değer çıktıyı NaN yapardı.
func TestNormEpsilonMustBeFinite(t *testing.T) {
	for _, eps := range []string{"nan", "1e300", "1e-50", "0", "-1"} {
		code := "TR_var nan = TR_log(TR_Tensor([-1.0])).TR_tolist[0]\nTR_layer_norm(TR_Tensor([[1.0, 2.0]]), TR_eps: " + eps + ")"
		if eps == "nan" {
			code = "TR_var nan = TR_log(TR_Tensor([-1.0])).TR_tolist[0]\nTR_rms_norm(TR_Tensor([[1.0, 2.0]]), TR_eps: nan)"
		}
		_, err := run(t, code)
		if err == nil || !messages.Has(err, messages.H6325) {
			t.Fatalf("epsilon %s: hata beklenirdi: %v", eps, err)
		}
	}
	mustRun(t, "TR_layer_norm(TR_Tensor([[1.0, 2.0]]), TR_eps: 0.001)")
}

// türev_izle Mantık olmalıdır: dolu bir metin ("yanlış") doğru sayılıp türevi açmaz.
func TestRequiresGradMustBeBool(t *testing.T) {
	for _, code := range []string{
		`TR_Tensor([1.0, 2.0], TR_requires_grad: "yanlış")`,
		`TR_Tensor([1.0, 2.0], 1)`,
		`TR_zeros([2], TR_requires_grad: "doğru")`,
	} {
		_, err := run(t, code)
		if err == nil || !messages.Has(err, messages.H7905) {
			t.Fatalf("%s: hata beklenirdi: %v", code, err)
		}
	}
	mustRun(t, "TR_var q = TR_Tensor([1.0], TR_requires_grad: TR_false)\nTR_print(q.TR_grad)")
}

// Kural (15.6, 15.23; T1 dilim 2c): gömmenin tensör indeksinde birden çok öğe hatalıysa ileti
// ilk hatalı öğeyi söyler (en küçük düz konum): konum 1'deki sınır dışı 9, konum 3'teki kesirli
// 2,5'ten önce gelir (eskiden önce bütün kesirliler taranırdı). İşlemcide ve kartta aynı.
func TestEmbeddingFirstFaultIsSmallestPosition(t *testing.T) {
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	for _, dev := range []string{"cpu", "gpu"} {
		_, err := run(t, `TR_default_device("`+dev+`")
TR_embedding_lookup(TR_ones([4, 2]), TR_Tensor([0.0, 9.0, 1.0, 2.5]))`)
		if !messages.Has(err, messages.H6305) || !strings.Contains(err.Error(), "9") {
			t.Errorf("%s: konum 1'deki 9 için H6305 bekleniyordu: %v", dev, err)
		}
		_, err = run(t, `TR_default_device("`+dev+`")
TR_embedding_lookup(TR_ones([4, 2]), TR_Tensor([0.0, 2.5, 1.0, 9.0]))`)
		if !messages.Has(err, messages.H6012) {
			t.Errorf("%s: konum 1'deki 2,5 için H6012 bekleniyordu: %v", dev, err)
		}
	}
}

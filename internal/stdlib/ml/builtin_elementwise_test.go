package ml

import (
	"testing"
)

func TestReturnTypeRule(t *testing.T) {
	// Sayıda Ondalık, tensörde tensör; liste kabul edilmez (karar 15).
	out := mustRun(t, `TR_print(TR_type(TR_relu(-1)), TR_type(TR_exp(0)), TR_type(TR_gelu(0.5)), TR_log(1))
TR_print(TR_type(TR_exp(TR_tensor([[0, 1]]))), TR_type(TR_log_softmax(TR_tensor([[1, 2]]))))
TR_print(TR_type(TR_exp(TR_tensor([0]))), TR_type(TR_log_softmax(TR_tensor([1.0, 2.0]))))
TR_var l = TR_log_softmax(TR_tensor([1, 2, 3])).TR_tolist
TR_print(TR_round(TR_exp(l[0]) + TR_exp(l[1]) + TR_exp(l[2]), 6))`)
	want := "Ondalık Ondalık Ondalık 0.0\nTensör Tensör\nTensör Tensör\n1.0\n"
	if out != want {
		t.Fatalf("dönüş tipleri:\n%q\nbeklenen:\n%q", out, want)
	}
}

// TestGeluApproximateArg gelu(x, yaklaşık: ...) sayıda, tensörde ve hatalarda: varsayılan
// ve yaklaşık: doğru tanh yaklaşımı, yaklaşık: yanlış tam (erf) GELU'dur.
func TestGeluApproximateArg(t *testing.T) {
	out := mustRun(t, `TR_print(TR_round(TR_gelu(1.0), 6), TR_round(TR_gelu(1.0, TR_approximate: TR_true), 6), TR_round(TR_gelu(1.0, TR_approximate: TR_false), 6))
TR_print(TR_type(TR_gelu(1, TR_approximate: TR_false)), TR_type(TR_gelu(TR_tensor([1.0]), TR_approximate: TR_false)))
TR_print(TR_round(TR_gelu(TR_tensor([-1.0]), TR_approximate: TR_false).TR_tolist[0], 6))`)
	want := "0.841192 0.841192 0.841345\nOndalık Tensör\n-0.158655\n"
	if out != want {
		t.Fatalf("gelu yaklaşık:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, code := range []string{
		`TR_gelu([1.0], TR_approximate: TR_false)`, // liste
		`TR_gelu(1.0, TR_approximate: 1)`,          // Mantık değil
		`TR_gelu(1.0, yaklasik: TR_false)`,         // bilinmeyen ad
	} {
		if _, err := run(t, code); err == nil {
			t.Errorf("%s hata vermedi", code)
		}
	}
}

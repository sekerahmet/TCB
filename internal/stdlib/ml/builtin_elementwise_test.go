package ml

import (
	"testing"
)

func TestReturnTypeRule(t *testing.T) {
	// Sayıda Ondalık, tensörde tensör; liste kabul edilmez (karar 15).
	out := mustRun(t, `yazdır(tip(relu(-1)), tip(üstel(0)), tip(gelu(0.5)), log(1))
yazdır(tip(üstel(tensör([[0, 1]]))), tip(log_softmax(tensör([[1, 2]]))))
yazdır(tip(üstel(tensör([0]))), tip(log_softmax(tensör([1.0, 2.0]))))
değişken l = log_softmax(tensör([1, 2, 3])).değer
yazdır(yuvarla(üstel(l[0]) + üstel(l[1]) + üstel(l[2]), 6))`)
	want := "Ondalık Ondalık Ondalık 0.0\nTensör Tensör\nTensör Tensör\n1.0\n"
	if out != want {
		t.Fatalf("dönüş tipleri:\n%q\nbeklenen:\n%q", out, want)
	}
}

// TestGeluApproximateArg gelu(x, yaklaşık: ...) sayıda, tensörde ve hatalarda: varsayılan
// ve yaklaşık: doğru tanh yaklaşımı, yaklaşık: yanlış tam (erf) GELU'dur.
func TestGeluApproximateArg(t *testing.T) {
	out := mustRun(t, `yazdır(yuvarla(gelu(1.0), 6), yuvarla(gelu(1.0, yaklaşık: doğru), 6), yuvarla(gelu(1.0, yaklaşık: yanlış), 6))
yazdır(tip(gelu(1, yaklaşık: yanlış)), tip(gelu(tensör([1.0]), yaklaşık: yanlış)))
yazdır(yuvarla(gelu(tensör([-1.0]), yaklaşık: yanlış).değer[0], 6))`)
	want := "0.841192 0.841192 0.841345\nOndalık Tensör\n-0.158655\n"
	if out != want {
		t.Fatalf("gelu yaklaşık:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, code := range []string{
		`gelu([1.0], yaklaşık: yanlış)`, // liste
		`gelu(1.0, yaklaşık: 1)`,        // Mantık değil
		`gelu(1.0, yaklasik: yanlış)`,   // bilinmeyen ad
	} {
		if _, err := run(t, code); err == nil {
			t.Errorf("%s hata vermedi", code)
		}
	}
}

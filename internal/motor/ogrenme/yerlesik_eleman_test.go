package ogrenme

import (
	"testing"
)

func TestDonusTipiKurali(t *testing.T) {
	// Sayıda Ondalık, tensörde tensör; liste kabul edilmez (karar 15).
	out := mustCalistir(t, `yazdır(tip(relu(-1)), tip(üstel(0)), tip(gelu(0.5)), log(1))
yazdır(tip(üstel(tensör([[0, 1]]))), tip(log_softmax(tensör([[1, 2]]))))
yazdır(tip(üstel(tensör([0]))), tip(log_softmax(tensör([1.0, 2.0]))))
değişken l = log_softmax(tensör([1, 2, 3])).değer
yazdır(yuvarla(üstel(l[0]) + üstel(l[1]) + üstel(l[2]), 6))`)
	want := "Ondalık Ondalık Ondalık 0.0\nTensör Tensör\nTensör Tensör\n1.0\n"
	if out != want {
		t.Fatalf("dönüş tipleri:\n%q\nbeklenen:\n%q", out, want)
	}
}

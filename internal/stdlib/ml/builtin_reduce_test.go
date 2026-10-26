package ml

import (
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestArgMax(t *testing.T) {
	out := mustRun(t, `değişken x = tensör([[1, 9, 3], [7, 2, 7]])
yazdır(argmax(x).değer, argmax(x, eksen: 0).değer, argmax(tensör([4, 8, 1])).değer)
yazdır(tam(argmax(tensör([4, 8, 1])).değer), argmax(x).şekil)
yazdır(gömme(tensör([[0, 0], [1, 1], [2, 2]]), argmax(tensör([[0, 0, 5]]))).değer)
değişken harfler = ["a", "b", "c"]
değişken i = argmax(tensör([[[0, 9, 1], [5, 0, 0]]]), eksen: -1)
yazdır(harfler[i.değer[0][0]] + harfler[i.değer[0][1]], tip(i.değer[0][1]), tip(argmax(tensör([1, 3])).değer))
yazdır(i, argmax(tensör([2, 1])))
yazdır(tip((i * 2).değer[0][0]), tip(tensör(i).değer[0][0]))`)
	want := "[1, 0] [1, 0, 1] 1\n1 [2]\n[[2.0, 2.0]]\n" +
		"ba Tam Tam\n" +
		"Tensör[1×2] [\n  [1, 0]\n] Tensör(0)\n" +
		"Ondalık Ondalık\n"
	if out != want {
		t.Fatalf("argmax:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"argmax([1, 2])", messages.H6217},
		{"argmax(tensör([1, 2]), 0)", messages.H6215},
		{"argmax(tensör([1, 2]), eksen: 1)", messages.H6016},
		{"argmax(sıfırlar([0]))", messages.H6209},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

func TestListReductions(t *testing.T) {
	// Listedeki tensörler çizgeyi koruyarak katlanır.
	out := mustRun(t, `değişken a = tensör([1.0, 2.0], türev_izle: doğru)
değişken b = tensör([3.0, 4.0])
yazdır(toplam([a, b]).değer, ortalama([1, 2, b]).şekil)
geri_yayılım(toplam(toplam([a, a, 1])))
yazdır(a.gradyan.değer)
yazdır(toplam([1, 2, 3]), max(4, 9), min([3, 1]), ortalama([1, 2, 3]))`)
	want := "[4.0, 6.0] [2]\n[2.0, 2.0]\n6 9 1 2.0\n"
	if out != want {
		t.Fatalf("liste indirgemesi:\n%q\nbeklenen:\n%q", out, want)
	}
	v := value(t, "değişken cikti = ortalama([1, 2, tensör([3.0, 6.0])])")
	data, _ := tensorData(t, v)
	near(t, "ortalama listesi", data, []float32{2, 3}, 1e-6)

	errs := []struct {
		code string
		want messages.Code
	}{
		{"toplam([[1, 2], [3, 4]])", messages.H6220},
		{"max([[1, 2], [3, 4]])", messages.H6220},
		{"ortalama([[1, 2]])", messages.H6220},
		{"max([tensör([1]), tensör([2])])", messages.H6224},
		{"toplam([1, 2], eksen: 0)", messages.H6219},
		{"max(tensör([1, 2]), 0)", messages.H6205},
		{"min(tensör([1, 2]), 0)", messages.H6205},
		{"sıfırlar(2, 3, türev_izle: doğru)", messages.H6020},
		{"rastgele_normal(2, 3)", messages.H6020},
		{"AdamW([tensör([1.0], türev_izle: doğru)], lr: 0.1)", messages.H7900},
		{"AdamW([tensör([1.0], türev_izle: doğru)], weight_decay: 0.1)", messages.H7900},
		{"toplam(tensör([1.0]), axis: 0)", messages.H7900},
		{"sıfırlar([2], requires_grad: doğru)", messages.H7900},
		{"rastgele_normal([2], seed: 1)", messages.H7900},
		{"katman_normu(birler([1, 2]), eps: 0.1)", messages.H7900},
	}
	for _, d := range errs {
		_, err := run(t, d.code)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.code, err, d.want)
		}
	}
	// toplam/ortalama için konumlu eksen kalır.
	v = value(t, "değişken cikti = toplam(tensör([[1, 2], [3, 4]]), 0)")
	data, _ = tensorData(t, v)
	near(t, "konumlu eksen", data, []float32{4, 6}, 0)
}

func BenchmarkMaxAll(b *testing.B) {
	t := tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = t.Reshape(-1).Max(0, false)
	}
}

func BenchmarkMinAll(b *testing.B) {
	t := tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = minAll(t)
	}
}

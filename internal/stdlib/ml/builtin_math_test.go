package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Genişletilmiş matematik yerleşikleri: sayıda ve tensörde dönüş tipleri, tam sayılı işaretin
// korunması, adlı argümanlar, yayınlama, türev ve yol gösteren hatalar.

func TestWideMathReturnTypes(t *testing.T) {
	out := mustRun(t, `yazdır(tip(silu(1)), tip(tan(1)), tip(asin(0.5)), tip(acos(0.5)), tip(atan(2)))
yazdır(tip(sinh(1)), tip(cosh(1)), tip(log1p(1)), tip(log2(4)), tip(log10(10)), tip(expm1(1)))
yazdır(tip(silu(tensör([1.0]))), tip(atan2(1, 2)), tip(atan2(tensör([1.0]), 2)))
yazdır(taban(3.7), taban(-3), tavan(-3.7), tavan(2), işaret(-7), işaret(2.5), işaret(0))
yazdır(sınırla(15, en_fazla: 10), sınırla(-3, en_az: 0), sınırla(5, en_az: 0.5), sınırla(2.5, en_az: 0, en_fazla: 1))
yazdır(log2(8), log10(1000), atan2(0, -1) == pi, pi > 3.14159 && pi < 3.1416)`)
	want := "Ondalık Ondalık Ondalık Ondalık Ondalık\n" +
		"Ondalık Ondalık Ondalık Ondalık Ondalık Ondalık\n" +
		"Tensör Ondalık Tensör\n" +
		"3.0 -3 -3.0 2 -1 1.0 0\n" +
		"10 0 5.0 1.0\n" +
		"3.0 3.0 doğru doğru\n"
	if out != want {
		t.Fatalf("dönüş tipleri:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestWideMathTensor(t *testing.T) {
	out := mustRun(t, `değişken i = argmax(tensör([[1, 5], [7, 2]]))
yazdır(taban(i).değer, işaret(i - 1).değer, sınırla(i, en_az: 0, en_fazla: 0).değer, sınırla(i, en_fazla: 0.5).değer)
değişken x = tensör([-2.0, 0.5, 3.0], türev_izle: doğru)
geri_yayılım(toplam(sınırla(x, en_az: -1, en_fazla: 1) + taban(x) * 5))
yazdır(x.gradyan.değer)
değişken s = en_büyük_k(tensör([[3, 1, 3], [0, 2, 1]]), 2)
yazdır(s[0].değer, s[1].değer, tip(s[1].değer[0][0]))
yazdır(sıralama_indeksi(tensör([2, 1, 2, 0]), eksen: 0).değer, birikimli_toplam(i).değer)
değişken m = tensör([[1.0], [0.0]])
yazdır(koşullu(m, tensör([1.0, 2.0]), -1).değer, koşullu(yanlış, tensör([1.0]), 7).değer)
yazdır((tensör([1, 2, 3]) >= tensör([[2], [1]])).değer, (2 < tensör([1, 2, 3])).değer)`)
	want := "[1, 0] [0.0, -1.0] [0, 0] [0.5, 0.0]\n" + // i - 1 tam sayılı değil (15.19)
		"[0.0, 1.0, 0.0]\n" +
		"[[3.0, 3.0], [2.0, 1.0]] [[0, 2], [1, 2]] Tam\n" +
		"[3, 1, 0, 2] [1, 1]\n" +
		"[[1.0, 2.0], [-1.0, -1.0]] [7.0]\n" +
		"[[0.0, 1.0, 1.0], [1.0, 1.0, 1.0]] [0.0, 0.0, 1.0]\n"
	if out != want {
		t.Fatalf("tensör yolları:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestWideMathErrors(t *testing.T) {
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"silu([1, 2])", messages.H6104},
		{"taban([1.5])", messages.H6104},
		{"atan2([1], 2)", messages.H6104},
		{"atan2(1)", messages.H6116},
		{"atan2(tensör([1, 2]), tensör([1, 2, 3]))", messages.H6100},
		{"sınırla(3)", messages.H6112},
		{"sınırla(3, en_az: 2, en_fazla: 1)", messages.H6113},
		{"sınırla(3, min: 1)", messages.H7900},
		{"sınırla(3, en_az: \"a\")", messages.H6107},
		{"sınırla([1], en_az: 0)", messages.H6104},
		{"koşullu(1, 2)", messages.H6201},
		{"koşullu(\"a\", 1, 2)", messages.H6207},
		{"koşullu(tensör([1, 0]), tensör([1, 2, 3]), 0)", messages.H6214},
		{"koşullu([1], 1, 2)", messages.H6104},
		{"birikimli_toplam([1, 2])", messages.H6104},
		{"birikimli_toplam(tensör([1, 2]), 0)", messages.H6200},
		{"birikimli_toplam(tensör(3.0))", messages.H6015},
		{"en_büyük_k(tensör([1, 2]), 3)", messages.H6210},
		{"en_büyük_k(tensör([1, 2]), 1.5)", messages.H6208},
		{"sıralama_indeksi(tensör([[1, 2]]), eksen: 2)", messages.H6016},
		{"tensör([1, 2]) < tensör([1, 2, 3])", messages.H6100},
		{"max(tensör([1, 2]), 0)", messages.H6205},
		{"min(tensör([1, 2]), 0)", messages.H6205},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// sınırla Tam sınırları float64'e çevirmeden uygular: 2⁵³'ten büyük değerlerde yuvarlama
// kırpmayı bozmaz, en_az: 2⁶³-1 taşıp en küçük Tam'a dönmez.
func TestClampIntLimits(t *testing.T) {
	out := mustRun(t, `yazdır(sınırla(9223372036854775807, en_fazla: 9223372036854775806))
yazdır(sınırla(5, en_az: 9223372036854775807))
yazdır(sınırla(-9223372036854775807, en_az: -9223372036854775806, en_fazla: 0))
yazdır(sınırla(7, en_az: 1, en_fazla: 3), sınırla(7, en_az: 1.5))`)
	want := "9223372036854775806\n9223372036854775807\n-9223372036854775806\n3 7.0\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
	if _, err := run(t, "sınırla(1, en_az: 9223372036854775807, en_fazla: 9223372036854775806)"); err == nil ||
		!strings.Contains(err.Error(), "9223372036854775807") {
		t.Fatalf("ters sınır hatası tam değeri göstermeli: %v", err)
	}
}

// Kural (13.7, 15.36): sonlu_mu sayıda Mantık verir (NaN ve ±sonsuz yanlış, her Tam doğru);
// tensörde eleman bazlı, sonlu yerde 1 öbür yerde 0 olan float32 tensör verir, 16 bitte de
// aynıdır ve türev izlemez. Listede, argümansız ve adlı argümanla açık hatadır.
// Neden: eğitim döngüsü gradyan normunu ve kaybı NaN için böyle denetler (torch.isfinite).
func TestIsFiniteNumbersAndTensors(t *testing.T) {
	out := mustRun(t, `değişken nan = ondalık("nan")
değişken sonsuz = ondalık("inf")
yazdır(sonlu_mu(3), sonlu_mu(-2.5), sonlu_mu(nan), sonlu_mu(sonsuz), sonlu_mu(-sonsuz), tip(sonlu_mu(0)))
değişken x = tensör([1.0, nan, sonsuz, -sonsuz, -0.0], türev_izle: doğru)
değişken s = sonlu_mu(x)
yazdır(s.değer, s.tür)
yazdır(sonlu_mu(x.türe("bf16")).değer, sonlu_mu(tensör([[1, 2]])).değer)`)
	want := "doğru doğru yanlış yanlış yanlış Mantık\n" +
		"[1.0, 0.0, 0.0, 0.0, 1.0] float32\n" +
		"[1.0, 0.0, 0.0, 0.0, 1.0] [[1.0, 1.0]]\n"
	if out != want {
		t.Fatalf("sonlu_mu:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"sonlu_mu([1.0])", messages.H6104},
		{"sonlu_mu()", messages.H2011},
		{"sonlu_mu(1, 2)", messages.H2011},
		{"sonlu_mu(\"a\")", messages.H2423},
		{"sonlu_mu(1.0, eksen: 0)", messages.H7902},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// Kural (15.36): sonlu_mu'nun sonucu karşılaştırmalar gibi her zaman Ondalık 0/1'dir; tam sayılı
// işaretli girdide (argmax, sıralama_indeksi) de, kaynaştır içindekiyle aynı.
// Neden: eskiden tam sayı işareti korunup [1, 1] veriyordu (Gemini kararlılık denetimi, bulgu 5).
func TestIsFiniteIsAFloatMaskOnIntTensors(t *testing.T) {
	out := mustRun(t, `değişken idx = sıralama_indeksi(tensör([20.0, 10.0]))
değişken f = kaynaştır(fonksiyon(x) { sonuç sonlu_mu(x) })
yazdır(sonlu_mu(idx).değer, f(idx).değer)`)
	if out != "[1.0, 1.0] [1.0, 1.0]\n" {
		t.Fatalf("çıktı: %q", out)
	}
}

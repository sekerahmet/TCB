package ml

import (
	"strings"
	"testing"
)

// Genişletilmiş matematik yerleşikleri: sayıda ve tensörde dönüş tipleri, tam sayılı işaretin
// korunması, adlı argümanlar, yayınlama, türev ve yol gösteren hatalar.

func TestWideMathReturnTypes(t *testing.T) {
	out := mustRun(t, `yazdır(tip(silu(1)), tip(tanjant(1)), tip(arksinüs(0.5)), tip(arkkosinüs(0.5)), tip(arktanjant(2)))
yazdır(tip(sinh(1)), tip(cosh(1)), tip(log1p(1)), tip(log2(4)), tip(log10(10)), tip(üstel_eksi_bir(1)))
yazdır(tip(silu(tensör([1.0]))), tip(arktanjant2(1, 2)), tip(arktanjant2(tensör([1.0]), 2)))
yazdır(taban(3.7), taban(-3), tavan(-3.7), tavan(2), işaret(-7), işaret(2.5), işaret(0))
yazdır(sınırla(15, en_fazla: 10), sınırla(-3, en_az: 0), sınırla(5, en_az: 0.5), sınırla(2.5, en_az: 0, en_fazla: 1))
yazdır(log2(8), log10(1000), arktanjant2(0, -1) == pi, pi > 3.14159 && pi < 3.1416)`)
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
	out := mustRun(t, `değişken i = maks_indeks(tensör([[1, 5], [7, 2]]))
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
	want := "[1, 0] [0.0, -1.0] [0, 0] [0.5, 0.0]\n" + // i - 1 tam sayılı değil (15.9)
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
	for _, d := range []struct{ code, expected string }{
		{"silu([1, 2])", "silu listede çalışmaz, yalnızca tensörde ve sayıda; listeyi tensör(...) ile çevirin"},
		{"taban([1.5])", "taban listede çalışmaz"},
		{"arktanjant2([1], 2)", "arktanjant2 listede çalışmaz"},
		{"arktanjant2(1)", "arktanjant2(y, x) 2 parametre bekler, 1 verildi"},
		{"arktanjant2(tensör([1, 2]), tensör([1, 2, 3]))", "[2] ile [3] şekilleri arktanjant2 için yayınlanamaz"},
		{"sınırla(3)", "sınırla: en_az ya da en_fazla verilmelidir"},
		{"sınırla(3, en_az: 2, en_fazla: 1)", "en_az (2) en_fazla'dan (1) büyük olamaz"},
		{"sınırla(3, min: 1)", "Bunu mu demek istediniz: 'en_az'?"},
		{"sınırla(3, en_az: \"a\")", "en_az bir sayı olmalıdır"},
		{"sınırla([1], en_az: 0)", "sınırla listede çalışmaz"},
		{"koşullu(1, 2)", "koşullu(koşul, a, b) 3 parametre bekler, 2 verildi"},
		{"koşullu(\"a\", 1, 2)", "koşul Mantık, sayı ya da tensör olmalıdır"},
		{"koşullu(tensör([1, 0]), tensör([1, 2, 3]), 0)", "koşullu: koşul [2], a [3] ve b [] şekilleri yayınlanamaz"},
		{"koşullu([1], 1, 2)", "koşullu listede çalışmaz"},
		{"birikimli_toplam([1, 2])", "birikimli_toplam listede çalışmaz, yalnızca tensörde"},
		{"birikimli_toplam(tensör([1, 2]), 0)", "eksen adlı verilir"},
		{"birikimli_toplam(tensör(3.0))", "skaler (0 boyutlu) tensörde eksen seçilemez"},
		{"en_büyük_k(tensör([1, 2]), 3)", "k = 3, ama [2] şeklinde 0. eksenin uzunluğu 2"},
		{"en_büyük_k(tensör([1, 2]), 1.5)", "k bir Tam olmalıdır"},
		{"sıralama_indeksi(tensör([[1, 2]]), eksen: 2)", "eksen 2, [1, 2] şeklindeki 2 boyutlu tensör için geçersiz"},
		{"tensör([1, 2]) < tensör([1, 2, 3])", "şekilleri tensör karşılaştırması (<) için yayınlanamaz"},
		{"maks(tensör([1, 2]), 0)", "eleman bazlı sınır için sınırla(x, en_az: 0) kullanın"},
		{"min(tensör([1, 2]), 0)", "eleman bazlı sınır için sınırla(x, en_fazla: 0) kullanın"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.expected)
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

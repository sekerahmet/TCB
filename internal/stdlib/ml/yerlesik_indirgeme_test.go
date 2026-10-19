package ml

import (
	"strings"
	"testing"
)

func TestMaksIndeks(t *testing.T) {
	out := mustCalistir(t, `değişken x = tensör([[1, 9, 3], [7, 2, 7]])
yazdır(maks_indeks(x).değer, maks_indeks(x, eksen: 0).değer, maks_indeks(tensör([4, 8, 1])).değer)
yazdır(tam(maks_indeks(tensör([4, 8, 1])).değer), maks_indeks(x).şekil)
yazdır(gömme(tensör([[0, 0], [1, 1], [2, 2]]), maks_indeks(tensör([[0, 0, 5]]))).değer)
değişken harfler = ["a", "b", "c"]
değişken i = maks_indeks(tensör([[[0, 9, 1], [5, 0, 0]]]), eksen: -1)
yazdır(harfler[i.değer[0][0]] + harfler[i.değer[0][1]], tip(i.değer[0][1]), tip(maks_indeks(tensör([1, 3])).değer))
yazdır(i, maks_indeks(tensör([2, 1])))
yazdır(tip((i * 2).değer[0][0]), tip(tensör(i).değer[0][0]))`)
	want := "[1, 0] [1, 0, 1] 1\n1 [2]\n[[2.0, 2.0]]\n" +
		"ba Tam Tam\n" +
		"Tensör[1×2] [\n  [1, 0]\n] Tensör(0)\n" +
		"Ondalık Ondalık\n"
	if out != want {
		t.Fatalf("maks_indeks:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct{ kod, beklenen string }{
		{"maks_indeks([1, 2])", "maks_indeks: bir tensör bekler, Liste verildi"},
		{"maks_indeks(tensör([1, 2]), 0)", "maks_indeks(x, eksen: 0): eksen adlı verilir"},
		{"maks_indeks(tensör([1, 2]), eksen: 1)", "eksen 1, [2] şeklindeki 1 boyutlu tensör için geçersiz"},
		{"maks_indeks(sıfırlar([0]))", "boş"},
	} {
		if _, err := calistir(t, d.kod); err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s: %v (içermeli: %s)", d.kod, err, d.beklenen)
		}
	}
}

func TestListeIndirgemeleri(t *testing.T) {
	// Listedeki tensörler çizgeyi koruyarak katlanır.
	out := mustCalistir(t, `değişken a = tensör([1.0, 2.0], türev_izle: doğru)
değişken b = tensör([3.0, 4.0])
yazdır(toplam([a, b]).değer, ortalama([1, 2, b]).şekil)
geri_yayılım(toplam(toplam([a, a, 1])))
yazdır(a.gradyan.değer)
yazdır(toplam([1, 2, 3]), maks(4, 9), min([3, 1]), ortalama([1, 2, 3]))`)
	want := "[4.0, 6.0] [2]\n[2.0, 2.0]\n6 9 1 2.0\n"
	if out != want {
		t.Fatalf("liste indirgemesi:\n%q\nbeklenen:\n%q", out, want)
	}
	v := deger(t, "değişken cikti = ortalama([1, 2, tensör([3.0, 6.0])])")
	veri, _ := tensorVeri(t, v)
	yakin(t, "ortalama listesi", veri, []float32{2, 3}, 1e-6)

	hatalar := []struct{ kod, beklenen string }{
		{"toplam([[1, 2], [3, 4]])", "toplam: iç içe liste indirgenemez"},
		{"maks([[1, 2], [3, 4]])", "maks: iç içe liste indirgenemez"},
		{"ortalama([[1, 2]])", "ortalama: iç içe liste indirgenemez"},
		{"maks([tensör([1]), tensör([2])])", "maks: tensör listesinin en büyük elemanı tanımsız"},
		{"toplam([1, 2], eksen: 0)", "toplam: eksen: yalnızca tensörlerde kullanılır, Liste verildi"},
		{"maks(tensör([1, 2]), 0)", "maks(x, 0): eksen için maks(x, eksen: 0) yazın; eleman bazlı sınır için sınırla(x, en_az: 0) kullanın"},
		{"min(tensör([1, 2]), 0)", "min(x, 0): eksen için min(x, eksen: 0) yazın"},
		{"sıfırlar(2, 3, türev_izle: doğru)", "sıfırlar(2, 3): şekil liste olarak verilir; sıfırlar([2, 3]) yazın"},
		{"rastgele_normal(2, 3)", "rastgele_normal(2, 3): şekil liste olarak verilir; rastgele_normal([2, 3]) yazın"},
		{"AdamW([tensör([1.0], türev_izle: doğru)], lr: 0.1)", "AdamW 'lr' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?"},
		{"AdamW([tensör([1.0], türev_izle: doğru)], weight_decay: 0.1)", "Bunu mu demek istediniz: 'ağırlık_çürümesi'?"},
		{"toplam(tensör([1.0]), axis: 0)", "Bunu mu demek istediniz: 'eksen'?"},
		{"sıfırlar([2], requires_grad: doğru)", "Bunu mu demek istediniz: 'türev_izle'?"},
		{"rastgele_normal([2], seed: 1)", "Bunu mu demek istediniz: 'tohum'?"},
		{"katman_normu(birler([1, 2]), eps: 0.1)", "Bunu mu demek istediniz: 'epsilon'?"},
	}
	for _, d := range hatalar {
		_, err := calistir(t, d.kod)
		if err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.kod, err, d.beklenen)
		}
	}
	// toplam/ortalama için konumlu eksen kalır.
	v = deger(t, "değişken cikti = toplam(tensör([[1, 2], [3, 4]]), 0)")
	veri, _ = tensorVeri(t, v)
	yakin(t, "konumlu eksen", veri, []float32{4, 6}, 0)
}

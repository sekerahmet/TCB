package tensor

import (
	"strings"
	"testing"
)

// Bağımsız denetimde bulunan sessiz yanlışların ve belirsiz çağrıların testleri.

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
		{"maks(tensör([1, 2]), 0)", "maks(x, 0): eksen için maks(x, eksen: 0) yazın; eleman bazlı sınır için relu(x - c) + c"},
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

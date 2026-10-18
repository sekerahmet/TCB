package tensor

import (
	"strings"
	"testing"
)

func TestDonusTipiKurali(t *testing.T) {
	// Tensör olmayan girdide sonuç girdinin biçimindedir; tensör yalnızca tensörden çıkar.
	out := mustCalistir(t, `yazdır(tip(relu([1, -2])), tip(üstel([0, 1])), tip(log([[1]])), tip(gelu(0.5)), tip(log_softmax([1, 2])))
yazdır(üstel([0, [0]]), log(1), tip(üstel(tensör([[0, 1]]))), tip(log_softmax(tensör([[1, 2]]))))
yazdır(tip(üstel(tensör([0]))), tip(log_softmax(tensör([1.0, 2.0]))))
değişken l = log_softmax([1, 2, 3])
yazdır(yuvarla(üstel(l[0]) + üstel(l[1]) + üstel(l[2]), 6))`)
	want := "Liste Liste Liste Ondalık Liste\n[1.0, [1.0]] 0.0 Tensör Tensör\nTensör Tensör\n1.0\n"
	if out != want {
		t.Fatalf("dönüş tipleri:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestMesajTutarliligi(t *testing.T) {
	durumlar := []struct{ kod, beklenen string }{
		{"AdamW([tensör(3.0)])", "1. parametre ([]) türev izlemiyor; tensör(..., türev_izle: doğru) ile oluşturun"},
		{"AdamW([3.0])", "1. parametre bir tensör olmalıdır, Ondalık verildi"},
		{"gömme(tensör(3.0), [0])", "tablo 2 boyutlu (satır sayısı × boyut) olmalıdır, [] verildi"},
		{"gömme(3.0, [0])", "tablo bir tensör olmalıdır, Ondalık verildi"},
		{"geri_yayılım(3.0)", "geri_yayılım() bir tensör bekler, Ondalık verildi"},
		{"geri_yayılım(tensör(3.0) * 2)", "kayıp türev izlenen hiçbir tensöre bağlı değil"},
		{"çapraz_entropi(sıfırlar([3, 4]), [[0, 1, 0, 0], [1, 0, 0, 0], [0, 0, 0, 1]])", "hedef sınıf indeksleri olmalı (ör. [2, 0, 1]); one-hot verdiniz gibi görünüyor"},
		{"log_softmax([1, 2], eksen: 0)", "log_softmax: eksen yalnızca tensörlerde kullanılır"},
		{"log_softmax([[1, 2]])", "log_softmax: liste düz sayı listesi olmalıdır"},
	}
	for _, d := range durumlar {
		_, err := calistir(t, d.kod)
		if err == nil || !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.kod, err, d.beklenen)
		}
		if err != nil && strings.Contains(err.Error(), "Tensör verildi") {
			t.Errorf("%s: çelişkili mesaj: %v", d.kod, err)
		}
	}
}

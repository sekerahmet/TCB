package ogrenme

import (
	"strings"
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

func TestMesajTutarliligi(t *testing.T) {
	durumlar := []struct{ kod, beklenen string }{
		{"AdamW([tensör(3.0)])", "1. parametre ([]) türev izlemiyor; tensör(..., türev_izle: doğru) ile oluşturun"},
		{"AdamW([3.0])", "1. parametre bir tensör olmalıdır, Ondalık verildi"},
		{"gömme(tensör(3.0), [0])", "tablo 2 boyutlu (satır sayısı × boyut) olmalıdır, [] verildi"},
		{"gömme(3.0, [0])", "tablo bir tensör olmalıdır, Ondalık verildi"},
		{"geri_yayılım(3.0)", "geri_yayılım() bir tensör bekler, Ondalık verildi"},
		{"geri_yayılım(tensör(3.0) * 2)", "kayıp türev izlenen hiçbir tensöre bağlı değil"},
		{"çapraz_entropi(sıfırlar([3, 4]), [[0, 1, 0, 0], [1, 0, 0, 0], [0, 0, 0, 1]])", "hedef sınıf indeksleri olmalı (ör. [2, 0, 1]); one-hot verdiniz gibi görünüyor"},
		{"log_softmax([1, 2], eksen: 0)", "log_softmax listede çalışmaz, yalnızca tensörde; listeyi tensör(...) ile çevirin"},
		{"relu([[1, 2]])", "relu listede çalışmaz, yalnızca tensörde ve sayıda; listeyi tensör(...) ile çevirin: relu(tensör(...))"},
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

package ogrenme

import (
	"strings"
	"testing"
)

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

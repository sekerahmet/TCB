package dataset_test

import (
	"strings"
	"testing"
)

// Sürdürme kesintisizle aynıdır: okuyucu durumu (ö.durum) düz değerli bir sözlüktür ve
// aynı ayarla kurulmuş yeni bir okuyucuya durumu_yükle ile verilince kesintisiz okuyucunun
// geri kalan toplu işlerini aynı sırayla verir. Durum sözlüğünün anahtarları kontrol noktası
// dosya biçiminin parçasıdır: sabittir, çevrilmez.
func TestReaderStateResumesSameBatches(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
değişken k = veri_kümesi("`+trPath(d)+`")
fonksiyon çek(ö, n) {
    değişken liste = []
    döngü i içinde 0..n {
        değişken b = ö.sonraki()
        eğer b == yok { liste.ekle(yok) } değilse { liste.ekle([b[0].değer, b[1].değer]) }
    }
    sonuç liste
}
fonksiyon dene_okuyucu(yap) {
    değişken kesintisiz = yap()
    çek(kesintisiz, 3)
    değişken durum = kesintisiz.durum
    değişken kalan = çek(kesintisiz, 4)
    değişken sürdürülen = yap()
    sürdürülen.durumu_yükle(durum)
    yazdır(kalan == çek(sürdürülen, 4))
    yazdır(durum.anahtarlar())
}
dene_okuyucu(fonksiyon() { sonuç k.örnekleyici(toplu: 2, bağlam: 8, tohum: 3) })
dene_okuyucu(fonksiyon() { sonuç k.sıralı(toplu: 2, bağlam: 8) })
`)
	want := "doğru\n" +
		`["tür", "toplu", "bağlam", "tohum", "küme", "dönem", "sıra", "pencere", "sayaç"]` + "\n" +
		"doğru\n" +
		`["tür", "toplu", "bağlam", "küme", "sıra", "pencere", "sayaç", "bitti"]` + "\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", strings.TrimSpace(out), strings.TrimSpace(want))
	}
}

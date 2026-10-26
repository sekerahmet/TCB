package dataset_test

import (
	"strings"
	"testing"
)

// Sürdürme kesintisizle aynıdır: okuyucu durumu (ö.durum) düz değerli bir sözlüktür ve
// aynı ayarla kurulmuş yeni bir okuyucuya durumu_yükle ile verilince kesintisiz okuyucunun
// geri kalan toplu işlerini aynı sırayla verir. Durum sözlüğünün anahtarları ve okuyucu türleri
// kontrol noktası dosya biçiminin parçasıdır: dilden bağımsız İngilizce tanımlayıcılardır, Türkçe
// programda da aynıdır (dil tanımı 17.3).
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
		`["kind", "batch", "context", "seed", "dataset", "epoch", "position", "window", "counter"]` + "\n" +
		"doğru\n" +
		`["kind", "batch", "context", "dataset", "position", "window", "counter", "done"]` + "\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", strings.TrimSpace(out), strings.TrimSpace(want))
	}
}

// Kural (20.15): ö.atla(n) sıradaki n toplu işi tensör kurmadan atlar ve atladığı sayıyı
// verir; sonraki .sonraki() n tane sonraki() çağrısından sonra gelecek toplu işin aynısıdır ve
// durumun sayacı n artar. Sıralı okuyucu biterse kalan kadar atlar, sonra yok verir. n = 0
// bir şey yapmaz; negatif, Tam olmayan ya da eksik/fazla argüman açık hatadır.
// Neden: kayıp sıçramasından sonra son noktaya dönen eğitim, sıçramayı tetikleyen toplu
// işleri atlar (PaLM §5.1: 200–500 toplu iş).
func TestReaderSkipSameAsNext(t *testing.T) {
	d := buildDataset(t)
	out, _ := mustRun(t, `
değişken k = veri_kümesi("`+trPath(d)+`")
fonksiyon dene_okuyucu(yap) {
    değişken a = yap()
    döngü i içinde 0..5 { a.sonraki() }
    değişken beklenen = a.sonraki()
    değişken b = yap()
    b.sonraki()
    yazdır(b.atla(0), b.atla(4))
    değişken x = b.sonraki()
    yazdır(x[0].değer == beklenen[0].değer, x[1].değer == beklenen[1].değer, b.durum["counter"] == a.durum["counter"])
}
dene_okuyucu(fonksiyon() { sonuç k.örnekleyici(toplu: 2, bağlam: 8, tohum: 3) })
dene_okuyucu(fonksiyon() { sonuç k.sıralı(toplu: 2, bağlam: 8) })
değişken s = k.sıralı(toplu: 8, bağlam: 32)
yazdır(s.atla(10), s.sonraki())
`)
	want := "0 4\ndoğru doğru doğru\n0 4\ndoğru doğru doğru\n3 yok\n"
	if out != want {
		t.Fatalf("çıktı:\n%s\nbeklenen:\n%s", out, want)
	}
	for _, code := range []string{"ö.atla(-1)", "ö.atla(1.5)", "ö.atla()", "ö.atla(1, 2)", "ö.atla(\"a\")"} {
		_, _, err := run(t, `değişken ö = veri_kümesi("`+trPath(d)+`").örnekleyici(toplu: 2, bağlam: 8, tohum: 3)
`+code)
		if err == nil {
			t.Errorf("%s hata vermeliydi", code)
		}
	}
}

package ogrenme

import (
	"testing"
)

// izliTR, bir tensörün hesap çizgesine kaydedilip kaydedilmediğini (geri yayılım
// yapılabiliyor mu) söyleyen TR yardımcısıdır; bekle_koşul koşul doğru olana dek (en çok
// 10 sn) kilidi bırakarak bekler. Testler süreye değil el sıkışmaya (bayraklara) dayanır.
const izliTR = `fonksiyon izli(t) {
    dene { geri_yayılım(toplam(t)) } yakala h { sonuç yanlış }
    sonuç doğru
}
fonksiyon bekle_koşul(k) {
    döngü i içinde 0..1000 {
        eğer k() { sonuç doğru }
        bekle(0.01)
    }
    sonuç yanlış
}
değişken d = {"başladı": yanlış, "devam": yanlış}
`

// türevsiz { } göreve bağlıdır: blok içindeki fonksiyon çağrıları ve döngü turları
// yorumlayıcı kilidini devrettiğinde, aynı anda çalışan başka görevin işlemleri çizgeye
// kaydedilmeye devam eder (hakem kanıtı: 3000 denemenin 2745'i "türev izlemiyor" hatası
// veriyordu).
func TestTurevsizGoreveBagli(t *testing.T) {
	out := mustCalistir(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
fonksiyon f(x) { sonuç x * 2 }
değişken bitti = yanlış
eşzamanlı {
    türevsiz {
        döngü i içinde 0..20000 { f(w) }
    }
    bitti = doğru
}
değişken izsiz = 0
değişken deneme = 0
döngü i içinde 0..3000 {
    değişken y = toplam(w * 3)
    deneme += 1
    dene { geri_yayılım(y) } yakala h { izsiz += 1 }
    eğer bitti { çık }
}
bekle_hepsi()
yazdır(deneme > 0, izsiz)`)
	if out != "doğru 0\n" {
		t.Fatalf("türevsiz başka görevi etkiledi:\n%s", out)
	}
}

// türevsiz bloğu kilidi bırakan bir çağrıdayken (bekle) başka görev türevli çalışır; blok
// beklemeden dönünce kendi türevsiz durumunu görür; yeni görev türevli başlar.
func TestTurevsizBekleyleGorevIzolasyonu(t *testing.T) {
	out := mustCalistir(t, izliTR+`değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken a = []
değişken b = []
eşzamanlı {
    türevsiz {
        a.ekle(izli(w * 2))
        d["başladı"] = doğru
        bekle_koşul(() => d["devam"])
        a.ekle(izli(w * 2))
        eşzamanlı { b.ekle(izli(w * 3)) }
    }
    a.ekle(izli(w * 2))
}
bekle_koşul(() => d["başladı"])
b.ekle(izli(w * 2))
d["devam"] = doğru
bekle_hepsi()
yazdır(a, b)`)
	if out != "[yanlış, yanlış, doğru] [doğru, doğru]\n" {
		t.Fatalf("türevsiz görev izolasyonu:\n%s", out)
	}
}

// özel_türev'in ileri ve geri hesapları da görev başına türevsiz çalışır: ileri ya da geri
// kilidi bıraktığında başka görevin hesabı çizgeye kaydedilir.
func TestOzelTurevIleriGeriGoreveBagli(t *testing.T) {
	out := mustCalistir(t, izliTR+`değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken v = tensör([1.0, 2.0], türev_izle: doğru)
değişken adım = {"ileri": yanlış, "ileri_devam": yanlış, "geri": yanlış, "geri_devam": yanlış}
değişken yavaş = özel_türev(
    ileri: fonksiyon(x) { adım["ileri"] = doğru; bekle_koşul(() => adım["ileri_devam"]); sonuç [x * 1, []] },
    geri: fonksiyon(s, g) { adım["geri"] = doğru; bekle_koşul(() => adım["geri_devam"]); sonuç [g] }
)
eşzamanlı {
    değişken y = yavaş(w)
    geri_yayılım(toplam(y))
}
bekle_koşul(() => adım["ileri"])
değişken i1 = izli(v * 2)
adım["ileri_devam"] = doğru
bekle_koşul(() => adım["geri"])
değişken i2 = izli(v * 2)
adım["geri_devam"] = doğru
bekle_hepsi()
yazdır(i1, i2, w.gradyan.değer)`)
	if out != "doğru doğru [1.0, 1.0]\n" {
		t.Fatalf("özel_türev görev izolasyonu:\n%s", out)
	}
}

// Başka bir görevin türevsiz bloğu sürerken, türev izlenen tensöre indeksle atama türevli
// görevde hâlâ reddedilir.
func TestTurevsizIndekseAtamaGoreveBagli(t *testing.T) {
	out := mustCalistir(t, izliTR+`değişken w = tensör([1.0, 2.0], türev_izle: doğru)
eşzamanlı {
    türevsiz { d["başladı"] = doğru; bekle_koşul(() => d["devam"]) }
}
bekle_koşul(() => d["başladı"])
değişken h = ""
dene { w[0] = 5 } yakala e { h = "{e}" }
d["devam"] = doğru
bekle_hepsi()
yazdır(w.değer, h.içerir("türevsiz"))`)
	if out != "[1.0, 2.0] doğru\n" {
		t.Fatalf("başka görevin türevsiz bloğu atamaya izin verdi:\n%s", out)
	}
}

// Bir görevin geri yayılımı özel_türev'in geri hesabında kilidi bırakırken, başka görev aynı
// çizgenin düğümlerinden geri yayılım başlatırsa açık hata alır (gradyanlar iki kez ya da
// eksik birikmez).
func TestEsZamanliGeriYayilimAyniCizge(t *testing.T) {
	out := mustCalistir(t, izliTR+`değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken yavaş = özel_türev(
    ileri: fonksiyon(x) { sonuç [x * 1, []] },
    geri: fonksiyon(s, g) { d["başladı"] = doğru; bekle_koşul(() => d["devam"]); sonuç [g] }
)
değişken o = yavaş(w)
değişken k1 = toplam(o * 3)
değişken k2 = toplam(o * 2)
eşzamanlı { geri_yayılım(k1) }
bekle_koşul(() => d["başladı"])
değişken h = ""
dene { geri_yayılım(k2) } yakala e { h = "{e}" }
d["devam"] = doğru
bekle_hepsi()
yazdır(h.içerir("başka bir geri yayılım"), w.gradyan.değer)`)
	if out != "doğru [3.0, 3.0]\n" {
		t.Fatalf("aynı çizgeden eşzamanlı geri yayılım:\n%s", out)
	}
}

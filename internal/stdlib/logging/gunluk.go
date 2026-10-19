// Package logging, model eğitimini izlemenin iki TR yerleşiğini içerir: JSON satırlarıyla
// eğitim günlüğü (günlük, G810; tip Günlük, A256) ve ilerleme çubuğu (ilerleme, G811; tip
// İlerleme, A255).
//
// Günlük dosyası biçimi (sürüm 1): her satır tek bir JSON nesnesidir ve tek bir yazma
// çağrısıyla dosyanın sonuna eklenir; süreç çökse de en çok son satır yarım kalır.
//
//	{"tür":"başlangıç","zaman":"2026-10-18T14:05:09.123+03:00","sürüm":1,"program":"egit.tr","toplam":5000,"ayarlar":{"adım":5000}}
//	{"zaman":"2026-10-18T14:05:10.004+03:00","adım":1,"kayıp":4.12,"hız":11.3}
//	{"zaman":"2026-10-18T14:19:12.870+03:00","doğrulama_kaybı":2.48}
//	{"tür":"devam","zaman":"...","adım":1500}
//	{"tür":"bitiş","zaman":"2026-10-18T15:06:12.411+03:00","adım":5000,"süre_sn":3663.3}
//
// "tür" taşımayan satırlar ölçü kayıtlarıdır. Anahtarlar yazım dilinden bağımsızdır ve
// çevrilmez (TRTENSOR imzası gibi); okuyan araçlar (İzleme Masası, tr izle) bunlara bakar.
// Tanımsız (NaN) ve sonsuz değerler gizlenmez, "NaN", "+Sonsuz", "-Sonsuz" metni olarak
// yazılır. Bir "devam" kaydı (kontrol noktasından sürdürme, bkz. Gunluk.DevamYaz) kendinden
// önce gelen ve adımı ondan büyük olan ölçü kayıtlarını geçersiz kılar (adımı eşit olan kayıt
// kaydedilen noktaya dahildir).
//
// İlerleme çubuğu standart hataya (VM.Err) yazar: terminalde tek satırda en çok saniyede 10
// kez yenilenir (yorumlayıcının canlı satır kancasıyla: yazdır ve hata çıktısı çubuğa
// karışmaz), terminal değilse 10 saniyede bir düz satır ve sonda özet satırı yazılır. Arka
// plan işinde (TR_İŞ ortam değişkeni) ayrıca iş klasörüne ilerleme.json anlık görüntüsü
// yazılır. Her İlerleme.güncelle çağrısı eğitimin bir adım sınırıdır (AdimKancasiAyarla).
//
// Gizli değerler (yurut.VM.GizliDegerEkle) günlükte, çubukta ve ilerleme.json'da ***
// olarak yazılır.
//
// Katman: motor (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package logging

import (
	"cmp"
	"slices"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// Tip adları.
var (
	GunlukTipi   = interp.TipAdi(codes.A256) // Günlük
	IlerlemeTipi = interp.TipAdi(codes.A255) // İlerleme
)

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	adGunluk = bind.KodAd(codes.G912)
	adAdim   = bind.KodAd(codes.T982)
)

// Dosya biçiminin (sürüm 1) anahtarları ve kayıt türleri; bkz. paket belgesi. Dilin adları
// değildir (çevrilmez), bu yüzden tek bir metinde tutulurlar.
var bicimAdlari = strings.Fields("tür zaman sürüm program toplam ayarlar adım hız süre_sn mesaj ölçüler kalan_sn bitti başlangıç bitiş devam")

var (
	aTur, aZaman, aSurum, aProgram   = bicimAdlari[0], bicimAdlari[1], bicimAdlari[2], bicimAdlari[3]
	aToplam, aAyarlar, aAdim, aHiz   = bicimAdlari[4], bicimAdlari[5], bicimAdlari[6], bicimAdlari[7]
	aSure, aMesaj, aOlculer, aKalan  = bicimAdlari[8], bicimAdlari[9], bicimAdlari[10], bicimAdlari[11]
	aBitti                           = bicimAdlari[12]
	turBaslangic, turBitis, turDevam = bicimAdlari[13], bicimAdlari[14], bicimAdlari[15]
)

// bicimSurumu günlük dosyası biçiminin sürümüdür (başlangıç kaydında).
const bicimSurumu = 1

// zamanBicimi kayıtların zaman damgasıdır (milisaniyeli, saat dilimli).
const zamanBicimi = "2006-01-02T15:04:05.000Z07:00"

// durum bir programın (VM'nin) günlük ve ilerleme durumudur. Kaydet her VM için bir tane
// kurar; alanları yorumlayıcı kilidi altında değişir.
type durum struct {
	surec *process.Durum
	// etkin ekranda görünen (canlı satırı olan) çubuktur; yenisi açılınca o görünür.
	etkin *Ilerleme
	// ekrandaki terminalde çizili canlı satırın karakter sayısıdır (silmek için); bütün
	// çubuklar aynı satırı paylaşır. Çıktı kilidi (VM.CanliSatirGuncelle) altında değişir.
	ekrandaki int
}

// Kaydet günlük ve ilerleme yerleşiklerini VM'ye kaydeder. s programın süreç durumudur
// (komut satırı ayarları başlangıç kaydına yazılır); nil olabilir.
func Kaydet(v *interp.VM, s *process.Durum) {
	d := &durum{surec: s}
	v.AdlisizKaydet(codes.G810, func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(codes.Ad(codes.G810), func() (interp.Value, error) { return d.gunlukAc(m, args) })
	})
	v.RegisterBuiltinKod(codes.G811, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(codes.Ad(codes.G811), func() (interp.Value, error) { return d.ilerlemeAc(m, args, named) })
	})
}

// Açık günlükler ve bitmemiş çubuklar: program bitince TumunuKapat hepsini kapatır.
var (
	acikMu       sync.Mutex
	acikGunluk   = map[*Gunluk]struct{}{}
	acikIlerleme = map[*Ilerleme]struct{}{}
)

// TumunuKapat bitmemiş çubukları bitirir (son satır ve günlükteki bitiş kaydı) ve açık
// günlük dosyalarını kapatır. Program bitince (hata ya da kesilmeyle bitse de) çağrılır;
// birden çok kez çağrılabilir. Kapatılan bir günlüğe sonra yazılırsa dosya yeniden açılır.
func TumunuKapat() {
	acikMu.Lock()
	cubuklar := make([]*Ilerleme, 0, len(acikIlerleme))
	for c := range acikIlerleme {
		cubuklar = append(cubuklar, c)
	}
	acikMu.Unlock()
	// Önce sonra açılanlar: iç içe çubuklarda görünen çubuk önce biter.
	slices.SortFunc(cubuklar, func(a, b *Ilerleme) int { return cmp.Compare(b.sira, a.sira) })
	for _, c := range cubuklar {
		c.bitir(saat())
	}
	acikMu.Lock()
	gunlukler := make([]*Gunluk, 0, len(acikGunluk))
	for g := range acikGunluk {
		gunlukler = append(gunlukler, g)
	}
	acikMu.Unlock()
	for _, g := range gunlukler {
		g.kapat()
	}
}

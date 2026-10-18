// Package kod, TR'nin kullanıcıya görünen bütün adlarını (anahtar kelimeler, tip
// adları, yerleşik fonksiyonlar, metot ve özellikler, adlı argümanlar) sabit kodlara
// bağlar. Go kodu adları değil kodları kullanır; adların kendisi dil dosyasındadır
// (Türkçe için tr.json, ikiliye gömülüdür). Bir adı değiştirmek için yalnızca dil
// dosyasındaki satır değişir; kodlar hiç değişmez. Başka bir dil (en.json, pt.json ...)
// aynı kodlara kendi adlarını verecektir.
//
// # Kod şeması
//
// Kod bir harf ve üç basamaklı bir sayıdır (A001, G100, Y400). Harf aileyi, yüzler
// basamağı aile içindeki grubu gösterir; gruplar arasında sonradan ekleme için boşluk
// bırakılmıştır.
//
//	A  dilin çekirdeği
//	   A0xx anahtar kelimeler (sözcük çözümleyicide ayrılmış)
//	        A00x koşul, A01x döngü, A02x fonksiyon, A03x bildirim, A04x değer ve mantık,
//	        A05x yapı, A06x hata yönetimi, A07x modül, A08x eşzamanlılık, A09x hata ayıklama
//	   A1xx bağlamsal sözcükler (yalnızca belli bir yerde anlamlı: olarak, test)
//	   A2xx tip adları (tip() sonucu, tip açıklamaları); A23x-A24x motor tipleri
//	G  genel yerleşikler
//	   G1xx giriş, çıkış ve dosya; G2xx değer, tip ve dönüşüm; G3xx liste;
//	   G4xx matematik ve istatistik; G5xx zaman ve eşzamanlılık; G6xx veri biçimleri
//	   (JSON, CSV); G7xx test
//	T  tensör ve yapay zekâ
//	   T1xx tensör oluşturma ve görünüm; T2xx etkinleştirme ve eleman bazlı matematik;
//	   T3xx türev; T4xx iyileştiriciler; T5xx sinir ağı katmanları, kayıplar, hazır
//	   model; T6xx vektör yardımcıları ve dil modeli; T7xx kaydetme, yükleme, veri türleri;
//	   T9xx adlı argümanlar
//	Y  yöntemler: metot ve özellikler (x.ad)
//	   Y0xx birden çok tipte ortak; Y1xx Metin; Y2xx Liste; Y3xx Sözlük; Y4xx Tensör;
//	   Y5xx iyileştirici; Y6xx veri kümesi, okuyucu, parçalayıcı;
//	   (Y7xx veritabanı, Y8xx web sunucusu, Y9xx görsel arayüz: kaldırıldı)
//	V  veri ve parçalayıcı
//	   V1xx veri kümesi; V2xx parçalayıcı; V9xx adlı argümanlar
//	W  web, veritabanı, arayüz (kaldırıldı; kodları bir daha kullanılmaz)
//	   W1xx web istemcisi ve sunucusu; W2xx veritabanı; W3xx görsel arayüz;
//	   W9xx adlı argümanlar
//	H  hata mesajları (ikinci aşama; şimdilik boş)
//
// # Ad alanları
//
// Bir dil içinde ad yalnızca kendi ad alanında benzersiz olmalıdır (bkz. Alan): genel
// adlar (anahtar kelimeler, bağlamsal sözcükler ve G/T/V/W yerleşikleri), tip adları,
// üyeler (Y) ve adlı argümanlar (x9xx). Böylece ör. Y202 sırala
// (liste metodu) ve W931 sırala (adlı argüman) ayrı kodlardır. Üye adları tipten
// bağımsız olarak benzersizdir: derleyici x.ad'ı tipe bakmadan derleme anında koda çevirir.
//
// # Dil dosyasının alanları
//
//	"G100": { "ad": "yazdır", "eski": ["yaz"], "aile": "giriş ve çıkış" }
//
// ad kodun bu dildeki adıdır. eski kaldırılmış adlardır: kabul edilmez, yalnızca hata
// mesajında ad önerilir; bir ad değiştirildiğinde ya da kaldırıldığında eski ad buraya
// eklenir. Her işin tek adı vardır: eş anlamlı ya da eş yazımlı ikinci bir ad kabul
// edilmez. "ascii": false, ASCIIKabul açılsa bile adın ASCII biçimini kapalı tutar (bugün
// yalnızca birkaç adlı argüman). aile insanlar için kısa bir açıklamadır. Girdiler koda
// göre sıralı ve her biri tek satırdır.
//
// # ASCII biçim
//
// Her adın Türkçe karakterleri ASCII'ye çevrilmiş biçimi (yazdır → yazdir) dil dosyasında
// saklanmaz, addan türetilir ve KABUL EDİLMEZ: her işin tek yazımı vardır. Bu kural tek
// bir yerden, ASCIIKabul sabitinden (kapalı) yönetilir; Yazimlar bir adın kabul edilen
// yazımlarını verir. ASCII biçim yalnızca hata mesajı içindir: Oneri ve AsilAdAlanda, bir
// ad alanında ASCII yazılmış bir adın (ya da eski bir adın) yerine Türkçe adı önerir.
//
// # Go'da kodlar
//
// Go kodu kodları dil dosyasından üretilen adlı sabitlerle yazar (kodlar_uretilmis.go):
// kod.G100, kod.A001, kod.Y400. Sabitin adı kodun kendisidir, kullanıcıya görünen
// ad değildir; ad değişse de sabit değişmez. Sözlükte olmayan (ya da taşınmış) bir kod
// yazılırsa derleme düşer. Sabitler switch deyimlerinde kullanılabilir. Dil dosyasına kod
// eklendiğinde, kaldırıldığında ya da taşındığında sabitler yeniden üretilir:
//
//	go generate ./internal/dil/kod   (ya da: go run ./bakim/uret/kod_sabitleri)
//
// Aynı komut kod listesinden üretilen öbür dosyaları da yeniler: docs/kodlar.md (her kodun ne
// işe yaradığı, imzası ve örneği) ve eklentinin renklendirme listeleri
// (duzenleyici/vscode/syntaxes/tr.tmLanguage.json).
//
// Katman: dil çekirdeği (internal/dil); yalnızca internal/dil paketlerini içe aktarır.
package kod

//go:generate go run ../../../bakim/uret/kod_sabitleri
//go:generate go run ../../../bakim/uret/kod_belgesi -kok ../../..
//go:generate go run ../../../bakim/uret/eklenti_listeleri -kok ../../..

import "fmt"

// Kod bir adın dilden bağımsız, değişmez kimliğidir. Değeri aile*1000 + sayıdır:
// A001 = A+1, Y400 = Y+400.
type Kod uint16

// Aile başlangıçları. Go kodu kodları bunlarla değil, üretilen adlı sabitlerle (G100, A001) yazar;
// aile + sayı biçimi yalnızca bu paketin içinde, üretilen dosyada kullanılır.
// Sayısal değerler yalnızca bu ikilinin içindedir; dil dosyası kodları metin olarak
// (G100) tutar.
const (
	A Kod = 1000 * (iota + 1) // dilin çekirdeği: anahtar kelimeler, tip adları
	G                         // genel yerleşikler
	H                         // hata mesajları (ikinci aşama)
	T                         // tensör ve yapay zekâ
	V                         // veri ve parçalayıcı
	W                         // web, veritabanı, arayüz (kaldırıldı)
	Y                         // yöntemler: metot ve özellikler
)

// aileHarfleri alfabe sırasındadır; böylece kodların sayısal sırası dil dosyasındaki
// (harf, sayı) sırasıyla aynıdır.
const aileHarfleri = "AGHTVWY"

// Yok, kodu olmayan ad için dönen sıfır değerdir.
const Yok Kod = 0

// Harf kodun aile harfidir ('A', 'G' ...); geçersiz kodda 0.
func (k Kod) Harf() byte {
	i := int(k)/1000 - 1
	if i < 0 || i >= len(aileHarfleri) {
		return 0
	}
	return aileHarfleri[i]
}

// Sayi kodun aile içindeki sayısıdır (0-999).
func (k Kod) Sayi() int { return int(k) % 1000 }

// Grup kodun yüzler basamağıdır (A013 → 0, T402 → 4).
func (k Kod) Grup() int { return k.Sayi() / 100 }

// String kodu dil dosyasındaki biçimiyle yazar (A001, Y400).
func (k Kod) String() string {
	if k.Harf() == 0 {
		return fmt.Sprintf("Kod(%d)", uint16(k))
	}
	return fmt.Sprintf("%c%03d", k.Harf(), k.Sayi())
}

// Coz "A001" biçimindeki metni koda çevirir.
func Coz(s string) (Kod, error) {
	if len(s) != 4 {
		return Yok, fmt.Errorf("kod %q geçersiz: bir harf ve üç basamak olmalı (ör. G100)", s)
	}
	aile := -1
	for i := 0; i < len(aileHarfleri); i++ {
		if s[0] == aileHarfleri[i] {
			aile = i
		}
	}
	if aile < 0 {
		return Yok, fmt.Errorf("kod %q geçersiz: aile harfi %s harflerinden biri olmalı", s, aileHarfleri)
	}
	n := 0
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return Yok, fmt.Errorf("kod %q geçersiz: harften sonra üç basamak gelmeli", s)
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return Yok, fmt.Errorf("kod %q geçersiz: 000 kullanılmaz", s)
	}
	return Kod(1000*(aile+1) + n), nil
}

// KodBicimi s bir kodun biçiminde mi: büyük harf ve üç rakam (G100, A001, K001)? Bu
// biçimdeki adlar ayrılmıştır (dosyalar ileride kodlarla saklanır; bkz. docs/dil/17_kodlar.md
// 17.7): kullanıcı adı olarak kullanılamaz. Aile harfi henüz kullanılmayan bir harf de
// olabilir (gelecekteki aileler için).
func KodBicimi(s string) bool {
	if len(s) != 4 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Alan bir adın benzersiz olması gereken ad alanıdır.
type Alan uint8

const (
	AlanYok       Alan = iota // ad alanı olmayan kod (H: hata mesajları)
	AlanGenel                 // anahtar kelimeler, bağlamsal sözcükler, G/T/V/W yerleşikleri
	AlanTip                   // tip adları (A2xx)
	AlanUye                   // metot ve özellikler (Y)
	AlanParametre             // adlı argümanlar (G/T/V/W içindeki x9xx grupları)
	alanSayisi
)

func (a Alan) String() string {
	switch a {
	case AlanGenel:
		return "genel adlar"
	case AlanTip:
		return "tip adları"
	case AlanUye:
		return "üyeler"
	case AlanParametre:
		return "adlı argümanlar"
	}
	return "ad alanı yok"
}

// Alan kodun ad alanıdır.
func (k Kod) Alan() Alan {
	switch k.Harf() {
	case 'A':
		if k.Grup() >= 2 {
			return AlanTip
		}
		return AlanGenel
	case 'G', 'T', 'V', 'W':
		if k.Grup() == 9 {
			return AlanParametre
		}
		return AlanGenel
	case 'Y':
		return AlanUye
	}
	return AlanYok
}

// AnahtarKelime kod sözcük çözümleyicide ayrılmış bir anahtar kelime mi (A0xx)?
func (k Kod) AnahtarKelime() bool { return k.Harf() == 'A' && k.Grup() == 0 }

// Yerlesik kod genel kapsama kaydedilen bir yerleşik fonksiyon mu (G/T/V/W, x9xx hariç)?
func (k Kod) Yerlesik() bool {
	switch k.Harf() {
	case 'G', 'T', 'V', 'W':
		return k.Grup() != 9
	}
	return false
}

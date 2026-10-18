package lexer

import (
	"fmt"

	"tr_dili/pkg/sozluk"
)

// Anahtar kelimelerin adları sözlüktedir (pkg/sozluk, tr.json'daki A0xx kodları). Bu
// dosya yalnızca hangi kodun hangi token türünü ürettiğini söyler; adı değiştirmek için
// sözlükteki satır değişir, lexer, ayrıştırıcı ve yorumlayıcı kodu değişmez. Kabul edilen
// yazımlar (ad ve varsa Türkçe karaktersiz biçimi) sozluk.Yazimlar'dan gelir.

// AnahtarKelime bir sözlük kodunu ürettiği token türüne bağlar.
type AnahtarKelime struct {
	Kod sozluk.Kod
	Tur TokenType
}

// anahtarKelimeler sözcük çözümleyicide ayrılmış bütün anahtar kelimelerdir.
var anahtarKelimeler = []AnahtarKelime{
	// Koşul ve eşleştirme
	{sozluk.A + 1, TOKEN_EGER},
	{sozluk.A + 2, TOKEN_DEGILSE},
	{sozluk.A + 3, TOKEN_ESLE},
	{sozluk.A + 4, TOKEN_VARSAYILAN},

	// Döngüler ve akış kontrolü
	{sozluk.A + 10, TOKEN_DONGU},
	{sozluk.A + 11, TOKEN_ICINDE},
	{sozluk.A + 12, TOKEN_IKEN},
	{sozluk.A + 13, TOKEN_CIK},
	{sozluk.A + 14, TOKEN_DEVAM},

	// Fonksiyon ve dönüş
	{sozluk.A + 20, TOKEN_FONKSIYON},
	{sozluk.A + 21, TOKEN_SONUC},

	// Değişken ve sabit
	{sozluk.A + 30, TOKEN_DEGISKEN},
	{sozluk.A + 31, TOKEN_SABIT},

	// Mantıksal ve boş değerler
	{sozluk.A + 40, TOKEN_DOGRU},
	{sozluk.A + 41, TOKEN_YANLIS},
	{sozluk.A + 42, TOKEN_YOK},
	{sozluk.A + 43, TOKEN_VE},
	{sozluk.A + 44, TOKEN_VEYA},
	{sozluk.A + 45, TOKEN_DEGIL},

	// Yapı ve kendi
	{sozluk.A + 50, TOKEN_YAPI},
	{sozluk.A + 51, TOKEN_KENDI},

	// Hata yönetimi, modül, eşzamanlılık, ertele, kesme noktası
	{sozluk.A + 60, TOKEN_DENE},
	{sozluk.A + 61, TOKEN_YAKALA},
	{sozluk.A + 62, TOKEN_FIRLAT},
	{sozluk.A + 70, TOKEN_AKTAR},
	{sozluk.A + 80, TOKEN_ESZAMANLI},
	{sozluk.A + 81, TOKEN_ERTELE},
	{sozluk.A + 90, TOKEN_KESME_NOKTASI},
}

// keywords yazım → token türü, turKodlari token türü → sözlük kodu haritasıdır.
var keywords, turKodlari = anahtarKelimeHaritasi(anahtarKelimeler)

// anahtarKelimeHaritasi tabloyu sözlükteki yazımlarla haritaya çevirir. Aynı yazım iki
// token türüne ya da aynı tür iki koda bağlanmışsa bu bir geliştirici hatasıdır ve
// program başlarken durur.
func anahtarKelimeHaritasi(tablo []AnahtarKelime) (map[string]TokenType, map[TokenType]sozluk.Kod) {
	harita := make(map[string]TokenType)
	kodlar := make(map[TokenType]sozluk.Kod)
	for _, ak := range tablo {
		if !ak.Kod.AnahtarKelime() {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s bir anahtar kelime kodu (A0xx) değil", ak.Kod))
		}
		if onceki, ok := kodlar[ak.Tur]; ok {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s hem %s hem %s", ak.Tur, onceki, ak.Kod))
		}
		kodlar[ak.Tur] = ak.Kod
		for _, yazim := range sozluk.Yazimlar(ak.Kod) {
			if onceki, ok := harita[yazim]; ok && onceki != ak.Tur {
				panic(fmt.Sprintf("anahtar kelime sözlüğü: %q hem %s hem %s olamaz", yazim, onceki, ak.Tur))
			}
			harita[yazim] = ak.Tur
		}
	}
	return harita, kodlar
}

// AnahtarKelimeAdi token türü bir anahtar kelimeyse onun sözlükteki adını (ör. "sonuç"),
// değilse boş metni döndürür.
func AnahtarKelimeAdi(t TokenType) string {
	if k, ok := turKodlari[t]; ok {
		return sozluk.Ad(k)
	}
	return ""
}

// KaldirilanYazimIpucu ad kaldırılmış bir anahtar kelimeyse (sözlükte bir A kodunun
// "eski" adı) hata mesajına eklenecek ipucunu, değilse boş metni döndürür. Kaldırılan
// sözcükler artık sıradan adlardır (ör. değişken sınıf = "9-A"); yalnızca bir hata
// oluştuğunda, eski alışkanlıkla yazılmış olabilecekleri için ipucu gösterilir. Eski ad
// Türkçe karaktersiz yazılmışsa (islev) yeni adın da Türkçe karaktersiz biçimi önerilir.
func KaldirilanYazimIpucu(ad string) string {
	k, ascii, ok := sozluk.EskiBul(sozluk.AlanGenel, ad, func(k sozluk.Kod) bool { return k.Harf() == 'A' })
	if !ok {
		return ""
	}
	yeni := sozluk.Ad(k)
	if ascii {
		yeni = sozluk.ASCIIBicim(k)
	}
	return fmt.Sprintf(" ('%s' TR'de anahtar kelime değil; '%s' yazmak mı istediniz?)", ad, yeni)
}

// LookupIdent verilen kelimenin anahtar kelime mi yoksa tanımlayıcı mı olduğunu bulur.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENT
}

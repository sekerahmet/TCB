package lexer

import (
	"fmt"

	"tr_dili/pkg/sozluk"
)

// Anahtar kelimelerin adları sözlüktedir (pkg/sozluk, tr.json'daki A0xx kodları). Bu
// dosya yalnızca hangi kodun hangi token türünü ürettiğini söyler; adı değiştirmek için
// sözlükteki satır değişir, lexer, ayrıştırıcı ve yorumlayıcı kodu değişmez. Kabul edilen
// yazımlar sozluk.Yazimlar'dan gelir (yalnızca Türkçe biçim; bkz. sozluk.ASCIIKabul).

// AnahtarKelime bir sözlük kodunu ürettiği token türüne bağlar.
type AnahtarKelime struct {
	Kod sozluk.Kod
	Tur TokenType
}

// anahtarKelimeler sözcük çözümleyicide ayrılmış bütün anahtar kelimelerdir.
var anahtarKelimeler = []AnahtarKelime{
	// Koşul ve eşleştirme
	{sozluk.A001, TOKEN_EGER},
	{sozluk.A002, TOKEN_DEGILSE},
	{sozluk.A003, TOKEN_ESLE},
	{sozluk.A004, TOKEN_VARSAYILAN},

	// Döngüler ve akış kontrolü
	{sozluk.A010, TOKEN_DONGU},
	{sozluk.A011, TOKEN_ICINDE},
	{sozluk.A012, TOKEN_IKEN},
	{sozluk.A013, TOKEN_CIK},
	{sozluk.A014, TOKEN_DEVAM},

	// Fonksiyon ve dönüş
	{sozluk.A020, TOKEN_FONKSIYON},
	{sozluk.A021, TOKEN_SONUC},

	// Değişken ve sabit
	{sozluk.A030, TOKEN_DEGISKEN},
	{sozluk.A031, TOKEN_SABIT},

	// Mantıksal ve boş değerler
	{sozluk.A040, TOKEN_DOGRU},
	{sozluk.A041, TOKEN_YANLIS},
	{sozluk.A042, TOKEN_YOK},
	{sozluk.A043, TOKEN_VE},
	{sozluk.A044, TOKEN_VEYA},
	{sozluk.A045, TOKEN_DEGIL},

	// Yapı ve kendi
	{sozluk.A050, TOKEN_YAPI},
	{sozluk.A051, TOKEN_KENDI},

	// Hata yönetimi, modül, eşzamanlılık, ertele, kesme noktası
	{sozluk.A060, TOKEN_DENE},
	{sozluk.A061, TOKEN_YAKALA},
	{sozluk.A062, TOKEN_FIRLAT},
	{sozluk.A070, TOKEN_AKTAR},
	{sozluk.A080, TOKEN_ESZAMANLI},
	{sozluk.A081, TOKEN_ERTELE},
	{sozluk.A090, TOKEN_KESME_NOKTASI},
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

// KaldirilanYazimIpucu ad bir anahtar kelimenin Türkçe karaktersiz (ASCII) biçimiyse
// (degisken) ya da kaldırılmış bir anahtar kelimeyse (sözlükte bir A kodunun "eski" adı:
// işlev, islev) hata mesajına eklenecek ipucunu, değilse boş metni döndürür. Bu yazımlar
// sıradan adlardır (ör. değişken sınıf = "9-A"); yalnızca bir hata oluştuğunda, alışkanlıkla
// yazılmış olabilecekleri için asıl ad önerilir. Öneri her zaman Türkçe biçimdir.
func KaldirilanYazimIpucu(ad string) string {
	k, ok := sozluk.Oneri(sozluk.AlanGenel, ad, func(k sozluk.Kod) bool { return k.Harf() == 'A' })
	if !ok {
		return ""
	}
	return fmt.Sprintf(" ('%s' TR'de anahtar kelime değil; '%s' yazmak mı istediniz?)", ad, sozluk.Ad(k))
}

// LookupIdent verilen kelimenin anahtar kelime mi yoksa tanımlayıcı mı olduğunu bulur.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENT
}

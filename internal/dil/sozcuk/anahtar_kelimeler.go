package sozcuk

import (
	"fmt"

	"tr_dili/internal/dil/kod"
)

// Anahtar kelimelerin adları kod listesindedir (internal/dil/kod, tr.json'daki A0xx kodları). Bu
// dosya yalnızca hangi kodun hangi token türünü ürettiğini söyler; adı değiştirmek için
// sözlükteki satır değişir, sözcük çözümleyici, ayrıştırıcı ve yorumlayıcı kodu değişmez. Kabul edilen
// yazımlar kod.Yazimlar'dan gelir (yalnızca Türkçe biçim; bkz. kod.ASCIIKabul).

// AnahtarKelime bir sözlük kodunu ürettiği token türüne bağlar.
type AnahtarKelime struct {
	Kod kod.Kod
	Tur TokenType
}

// anahtarKelimeler sözcük çözümleyicide ayrılmış bütün anahtar kelimelerdir.
var anahtarKelimeler = []AnahtarKelime{
	// Koşul ve eşleştirme
	{kod.A001, TOKEN_EGER},
	{kod.A002, TOKEN_DEGILSE},
	{kod.A003, TOKEN_ESLE},
	{kod.A004, TOKEN_VARSAYILAN},

	// Döngüler ve akış kontrolü
	{kod.A010, TOKEN_DONGU},
	{kod.A011, TOKEN_ICINDE},
	{kod.A012, TOKEN_IKEN},
	{kod.A013, TOKEN_CIK},
	{kod.A014, TOKEN_DEVAM},

	// Fonksiyon ve dönüş
	{kod.A020, TOKEN_FONKSIYON},
	{kod.A021, TOKEN_SONUC},

	// Değişken ve sabit
	{kod.A030, TOKEN_DEGISKEN},
	{kod.A031, TOKEN_SABIT},

	// Mantıksal ve boş değerler
	{kod.A040, TOKEN_DOGRU},
	{kod.A041, TOKEN_YANLIS},
	{kod.A042, TOKEN_YOK},
	{kod.A043, TOKEN_VE},
	{kod.A044, TOKEN_VEYA},
	{kod.A045, TOKEN_DEGIL},

	// Yapı ve kendi
	{kod.A050, TOKEN_YAPI},
	{kod.A051, TOKEN_KENDI},

	// Hata yönetimi, modül, eşzamanlılık, ertele, kesme noktası
	{kod.A060, TOKEN_DENE},
	{kod.A061, TOKEN_YAKALA},
	{kod.A062, TOKEN_FIRLAT},
	{kod.A070, TOKEN_AKTAR},
	{kod.A080, TOKEN_ESZAMANLI},
	{kod.A081, TOKEN_ERTELE},
	{kod.A090, TOKEN_KESME_NOKTASI},
}

// keywords yazım → token türü, turKodlari token türü → sözlük kodu haritasıdır.
var keywords, turKodlari = anahtarKelimeHaritasi(anahtarKelimeler)

// anahtarKelimeHaritasi tabloyu sözlükteki yazımlarla haritaya çevirir. Aynı yazım iki
// token türüne ya da aynı tür iki koda bağlanmışsa bu bir geliştirici hatasıdır ve
// program başlarken durur.
func anahtarKelimeHaritasi(tablo []AnahtarKelime) (map[string]TokenType, map[TokenType]kod.Kod) {
	harita := make(map[string]TokenType)
	kodlar := make(map[TokenType]kod.Kod)
	for _, ak := range tablo {
		if !ak.Kod.AnahtarKelime() {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s bir anahtar kelime kodu (A0xx) değil", ak.Kod))
		}
		if onceki, ok := kodlar[ak.Tur]; ok {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s hem %s hem %s", ak.Tur, onceki, ak.Kod))
		}
		kodlar[ak.Tur] = ak.Kod
		for _, yazim := range kod.Yazimlar(ak.Kod) {
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
		return kod.Ad(k)
	}
	return ""
}

// KaldirilanYazimIpucu ad bir anahtar kelimenin Türkçe karaktersiz (ASCII) biçimiyse
// (degisken) ya da kaldırılmış bir anahtar kelimeyse (sözlükte bir A kodunun "eski" adı:
// işlev, islev) hata mesajına eklenecek ipucunu, değilse boş metni döndürür. Bu yazımlar
// sıradan adlardır (ör. değişken sınıf = "9-A"); yalnızca bir hata oluştuğunda, alışkanlıkla
// yazılmış olabilecekleri için asıl ad önerilir. Öneri her zaman Türkçe biçimdir.
func KaldirilanYazimIpucu(ad string) string {
	k, ok := kod.Oneri(kod.AlanGenel, ad, func(k kod.Kod) bool { return k.Harf() == 'A' })
	if !ok {
		return ""
	}
	return fmt.Sprintf(" ('%s' TR'de anahtar kelime değil; '%s' yazmak mı istediniz?)", ad, kod.Ad(k))
}

// LookupIdent verilen kelimenin anahtar kelime mi yoksa tanımlayıcı mı olduğunu bulur.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENT
}

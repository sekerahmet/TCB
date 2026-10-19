package lexer

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Anahtar kelimelerin adları kod listesindedir (internal/lang/codes, tr.json'daki A0xx kodları). Bu
// dosya yalnızca hangi kodun hangi token türünü ürettiğini söyler; adı değiştirmek için
// sözlükteki satır değişir, sözcük çözümleyici, ayrıştırıcı ve yorumlayıcı kodu değişmez. Kabul edilen
// yazımlar kod.Yazimlar'dan gelir (yalnızca Türkçe biçim; bkz. kod.ASCIIKabul).

// AnahtarKelime bir sözlük kodunu ürettiği token türüne bağlar.
type AnahtarKelime struct {
	Kod codes.Kod
	Tur TokenType
}

// anahtarKelimeler sözcük çözümleyicide ayrılmış bütün anahtar kelimelerdir.
var anahtarKelimeler = []AnahtarKelime{
	// Koşul ve eşleştirme
	{codes.A001, TOKEN_EGER},
	{codes.A002, TOKEN_DEGILSE},
	{codes.A003, TOKEN_ESLE},
	{codes.A004, TOKEN_VARSAYILAN},

	// Döngüler ve akış kontrolü
	{codes.A010, TOKEN_DONGU},
	{codes.A011, TOKEN_ICINDE},
	{codes.A012, TOKEN_IKEN},
	{codes.A013, TOKEN_CIK},
	{codes.A014, TOKEN_DEVAM},

	// Fonksiyon ve dönüş
	{codes.A020, TOKEN_FONKSIYON},
	{codes.A021, TOKEN_SONUC},

	// Değişken ve sabit
	{codes.A030, TOKEN_DEGISKEN},
	{codes.A031, TOKEN_SABIT},

	// Mantıksal ve boş değerler
	{codes.A040, TOKEN_DOGRU},
	{codes.A041, TOKEN_YANLIS},
	{codes.A042, TOKEN_YOK},
	{codes.A043, TOKEN_VE},
	{codes.A044, TOKEN_VEYA},
	{codes.A045, TOKEN_DEGIL},

	// Yapı ve kendi
	{codes.A050, TOKEN_YAPI},
	{codes.A051, TOKEN_KENDI},

	// Hata yönetimi, modül, eşzamanlılık, ertele, kesme noktası
	{codes.A060, TOKEN_DENE},
	{codes.A061, TOKEN_YAKALA},
	{codes.A062, TOKEN_FIRLAT},
	{codes.A070, TOKEN_AKTAR},
	{codes.A080, TOKEN_ESZAMANLI},
	{codes.A081, TOKEN_ERTELE},
	{codes.A090, TOKEN_KESME_NOKTASI},
}

// keywords yazım → token türü, turKodlari token türü → sözlük kodu haritasıdır.
var keywords, turKodlari = anahtarKelimeHaritasi(anahtarKelimeler)

// anahtarKelimeHaritasi tabloyu sözlükteki yazımlarla haritaya çevirir. Aynı yazım iki
// token türüne ya da aynı tür iki koda bağlanmışsa bu bir geliştirici hatasıdır ve
// program başlarken durur.
func anahtarKelimeHaritasi(tablo []AnahtarKelime) (map[string]TokenType, map[TokenType]codes.Kod) {
	harita := make(map[string]TokenType)
	kodlar := make(map[TokenType]codes.Kod)
	for _, ak := range tablo {
		if !ak.Kod.AnahtarKelime() {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s bir anahtar kelime kodu (A0xx) değil", ak.Kod))
		}
		if onceki, ok := kodlar[ak.Tur]; ok {
			panic(fmt.Sprintf("anahtar kelime tablosu: %s hem %s hem %s", ak.Tur, onceki, ak.Kod))
		}
		kodlar[ak.Tur] = ak.Kod
		for _, yazim := range codes.Yazimlar(ak.Kod) {
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
		return codes.Ad(k)
	}
	return ""
}

// LookupIdent verilen kelimenin anahtar kelime mi yoksa tanımlayıcı mı olduğunu bulur.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENT
}

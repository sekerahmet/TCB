package lexer

import "fmt"

// Anahtar kelime sözlüğü yalnızca veridir: hangi yazımın hangi token türünü ürettiğini
// söyleyen tablolardan oluşur. Lexer bu tablolardan kurulan tek bir haritaya bakar.
//
// İleride ikinci bir yazım takımı eklemek için yeni bir []AnahtarKelime tablosu tanımlayıp
// yazimTakimlari listesine eklemek yeterlidir; lexer, ayrıştırıcı ve yorumlayıcı kodu
// değişmez. Farklı takımlar aynı yazımı farklı token türlerine bağlayamaz (bkz.
// anahtarKelimeHaritasi).

// AnahtarKelime bir token türünü onu üreten tüm yazımlarla eşler. İlk yazım asıl
// (Türkçe karakterli) yazımdır; diğerleri ASCII ve eş anlamlı yazımlardır.
type AnahtarKelime struct {
	Tur      TokenType
	Yazimlar []string
}

// turkceAnahtarKelimeler çift modlu (hem Türkçe karakterli hem ASCII) Türkçe sözlüktür.
var turkceAnahtarKelimeler = []AnahtarKelime{
	// Değişken ve sabit
	{TOKEN_DEGISKEN, []string{"değişken", "degisken"}},
	{TOKEN_SABIT, []string{"sabit"}},

	// Fonksiyon ve dönüş
	{TOKEN_FONKSIYON, []string{"fonksiyon"}},
	{TOKEN_SONUC, []string{"sonuç", "sonuc"}},

	// Koşul ve eşleştirme
	{TOKEN_EGER, []string{"eğer", "eger"}},
	{TOKEN_DEGILSE, []string{"değilse", "degilse"}},
	{TOKEN_ESLE, []string{"eşle", "esle"}},
	{TOKEN_VARSAYILAN, []string{"varsayılan", "varsayilan"}},

	// Döngüler ve akış kontrolü
	{TOKEN_DONGU, []string{"döngü", "dongu"}},
	{TOKEN_ICINDE, []string{"içinde", "icinde"}},
	{TOKEN_IKEN, []string{"iken"}},
	{TOKEN_CIK, []string{"çık", "cik"}},
	{TOKEN_DEVAM, []string{"devam"}},

	// Mantıksal ve boş değerler
	{TOKEN_DOGRU, []string{"doğru", "dogru"}},
	{TOKEN_YANLIS, []string{"yanlış", "yanlis"}},
	{TOKEN_YOK, []string{"yok"}},
	{TOKEN_VE, []string{"ve"}},
	{TOKEN_VEYA, []string{"veya"}},
	{TOKEN_DEGIL, []string{"değil", "degil"}},

	// Yapı, Sınıf, Kendi
	{TOKEN_YAPI, []string{"yapı", "yapi"}},
	{TOKEN_KENDI, []string{"kendi"}},

	// Hata yönetimi, Modül, Eşzamanlılık, Ertele, Kesme Noktası
	{TOKEN_DENE, []string{"dene"}},
	{TOKEN_YAKALA, []string{"yakala"}},
	{TOKEN_FIRLAT, []string{"fırlat", "firlat"}},
	{TOKEN_AKTAR, []string{"aktar"}},
	{TOKEN_ESZAMANLI, []string{"eşzamanlı", "eszamanli"}},
	{TOKEN_ERTELE, []string{"ertele"}},
	{TOKEN_KESME_NOKTASI, []string{"kesme_noktası", "kesme_noktasi", "dur_noktası", "dur_noktasi", "durdurma_noktası", "durdurma_noktasi"}},
}

// yazimTakimlari lexer'ın tanıdığı tüm yazım takımlarıdır.
var yazimTakimlari = [][]AnahtarKelime{
	turkceAnahtarKelimeler,
}

// keywords tüm yazım takımlarından kurulan yazım → token türü haritasıdır.
var keywords = anahtarKelimeHaritasi(yazimTakimlari)

// anahtarKelimeHaritasi takımları tek bir haritada birleştirir. Aynı yazım iki farklı
// token türüne bağlanmışsa bu bir sözlük hatasıdır ve program başlarken durur.
func anahtarKelimeHaritasi(takimlar [][]AnahtarKelime) map[string]TokenType {
	harita := make(map[string]TokenType)
	for _, takim := range takimlar {
		for _, ak := range takim {
			for _, yazim := range ak.Yazimlar {
				if onceki, ok := harita[yazim]; ok && onceki != ak.Tur {
					panic(fmt.Sprintf("anahtar kelime sözlüğü: %q hem %s hem %s olamaz", yazim, onceki, ak.Tur))
				}
				harita[yazim] = ak.Tur
			}
		}
	}
	return harita
}

// kaldirilanYazimlar TR'den kaldırılmış eş anlamlı anahtar kelimelerdir: her işin tek
// yazımı olsun diye asıl yazımlar fonksiyon, sonuç, çık, yapı ve olarak'tır. Kaldırılan
// sözcükler artık sıradan adlardır (ör. değişken sınıf = "9-A"); yalnızca bir hata
// oluştuğunda, eski alışkanlıkla yazılmış olabilecekleri için ipucu gösterilir.
var kaldirilanYazimlar = map[string]string{
	"işlev": "fonksiyon", "islev": "fonksiyon",
	"döndür": "sonuç", "dondur": "sonuc",
	"kır": "çık", "kir": "cik",
	"sınıf": "yapı", "sinif": "yapi",
	"as": "olarak",
}

// KaldirilanYazimIpucu ad kaldırılmış bir anahtar kelimeyse hata mesajına eklenecek
// ipucunu, değilse boş metni döndürür.
func KaldirilanYazimIpucu(ad string) string {
	if yeni, ok := kaldirilanYazimlar[ad]; ok {
		return fmt.Sprintf(" ('%s' TR'de anahtar kelime değil; '%s' yazmak mı istediniz?)", ad, yeni)
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

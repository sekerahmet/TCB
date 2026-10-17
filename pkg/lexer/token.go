package lexer

import "fmt"

// TokenType dildeki her bir sözcük biriminin (token) türünü temsil eder.
type TokenType string

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

func (t Token) String() string {
	return fmt.Sprintf("[%d:%d] %s (%q)", t.Line, t.Column, t.Type, t.Literal)
}

const (
	TOKEN_ILLEGAL = "GEÇERSİZ"
	TOKEN_EOF     = "DOSYA_SONU"

	// Tanımlayıcılar ve Sabit Değerler
	TOKEN_IDENT  = "TANIMLAYICI"
	TOKEN_INT    = "TAM_SAYI"
	TOKEN_FLOAT  = "ONDALIK_SAYI"
	TOKEN_STRING = "METİN"

	// Operatörler
	TOKEN_ASSIGN       = "="
	TOKEN_WALRUS       = ":="
	TOKEN_PLUS         = "+"
	TOKEN_MINUS        = "-"
	TOKEN_ASTERISK     = "*"
	TOKEN_SLASH        = "/"
	TOKEN_PERCENT      = "%"
	TOKEN_POWER        = "**"
	TOKEN_MATMUL       = "@"
	TOKEN_BANG         = "!"
	TOKEN_EQ           = "=="
	TOKEN_NOT_EQ       = "!="
	TOKEN_LT           = "<"
	TOKEN_GT           = ">"
	TOKEN_LTE          = "<="
	TOKEN_GTE          = ">="
	TOKEN_AND          = "&&"
	TOKEN_OR           = "||"
	TOKEN_PIPE         = "|>"
	TOKEN_NULL_COALESC = "??"
	TOKEN_SAFE_DOT     = "?."
	TOKEN_ARROW        = "=>"
	TOKEN_RANGE        = ".."
	TOKEN_RANGE_INCL   = "..="
	TOKEN_PLUS_ASSIGN  = "+="
	TOKEN_MINUS_ASSIGN = "-="
	TOKEN_MUL_ASSIGN   = "*="
	TOKEN_DIV_ASSIGN   = "/="

	// Ayırıcılar
	TOKEN_COMMA     = ","
	TOKEN_SEMICOLON = ";"
	TOKEN_COLON     = ":"
	TOKEN_DOT       = "."
	TOKEN_QUESTION  = "?"
	TOKEN_LPAREN    = "("
	TOKEN_RPAREN    = ")"
	TOKEN_LBRACE    = "{"
	TOKEN_RBRACE    = "}"
	TOKEN_LBRACKET  = "["
	TOKEN_RBRACKET  = "]"

	// Anahtar Kelimeler (Birlikte Seçilen TR Sözlüğü)
	TOKEN_DEGISKEN   = "DEĞİŞKEN"   // değişken / degisken
	TOKEN_SABIT      = "SABİT"      // sabit
	TOKEN_FONKSIYON  = "FONKSİYON"  // fonksiyon / işlev / islev
	TOKEN_SONUC      = "SONUÇ"      // sonuç / sonuc / döndür / dondur
	TOKEN_EGER       = "EĞER"       // eğer / eger
	TOKEN_DEGILSE    = "DEĞİLSE"    // değilse / degilse
	TOKEN_ESLE       = "EŞLE"       // eşle / esle
	TOKEN_VARSAYILAN = "VARSAYILAN" // varsayılan / varsayilan
	TOKEN_DONGU      = "DÖNGÜ"      // döngü / dongu
	TOKEN_ICINDE     = "İÇİNDE"     // içinde / icinde
	TOKEN_IKEN       = "İKEN"       // iken
	TOKEN_CIK        = "ÇIK"        // çık / cik / kır / kir
	TOKEN_DEVAM      = "DEVAM"      // devam
	TOKEN_DOGRU      = "DOĞRU"      // doğru / dogru
	TOKEN_YANLIS     = "YANLIŞ"     // yanlış / yanlis
	TOKEN_YOK        = "YOK"        // yok
	TOKEN_VE         = "VE"         // ve
	TOKEN_VEYA       = "VEYA"       // veya
	TOKEN_DEGIL      = "DEĞİL"      // değil / degil
	TOKEN_YAPI       = "YAPI"       // yapı / yapi
	TOKEN_SINIF      = "SINIF"      // sınıf / sinif
	TOKEN_KENDI      = "KENDİ"      // kendi
	TOKEN_DENE       = "DENE"       // dene
	TOKEN_YAKALA     = "YAKALA"     // yakala
	TOKEN_FIRLAT     = "FIRLAT"     // fırlat / firlat
	TOKEN_AKTAR      = "AKTAR"      // aktar
	TOKEN_ESZAMANLI     = "EŞZAMANLI"     // eşzamanlı / eszamanli
	TOKEN_ERTELE        = "ERTELE"        // ertele (defer)
	TOKEN_KESME_NOKTASI = "KESME_NOKTASI" // kesme_noktası / dur_noktası / break_point (ABAP BREAK-POINT)
)

// Çift modlu (Hem Türkçe karakterli hem ASCII) anahtar kelime haritası
var keywords = map[string]TokenType{
	// Değişken ve sabit
	"değişken": TOKEN_DEGISKEN,
	"degisken": TOKEN_DEGISKEN,
	"sabit":    TOKEN_SABIT,

	// Fonksiyon ve dönüş
	"fonksiyon": TOKEN_FONKSIYON,
	"işlev":     TOKEN_FONKSIYON,
	"islev":     TOKEN_FONKSIYON,
	"sonuç":     TOKEN_SONUC,
	"sonuc":     TOKEN_SONUC,
	"döndür":    TOKEN_SONUC,
	"dondur":    TOKEN_SONUC,

	// Koşul ve eşleştirme
	"eğer":       TOKEN_EGER,
	"eger":       TOKEN_EGER,
	"değilse":    TOKEN_DEGILSE,
	"degilse":    TOKEN_DEGILSE,
	"eşle":       TOKEN_ESLE,
	"esle":       TOKEN_ESLE,
	"varsayılan": TOKEN_VARSAYILAN,
	"varsayilan": TOKEN_VARSAYILAN,

	// Döngüler ve akış kontrolü
	"döngü":  TOKEN_DONGU,
	"dongu":  TOKEN_DONGU,
	"içinde": TOKEN_ICINDE,
	"icinde": TOKEN_ICINDE,
	"iken":   TOKEN_IKEN,
	"çık":    TOKEN_CIK,
	"cik":    TOKEN_CIK,
	"kır":    TOKEN_CIK,
	"kir":    TOKEN_CIK,
	"devam":  TOKEN_DEVAM,

	// Mantıksal ve boş değerler
	"doğru":  TOKEN_DOGRU,
	"dogru":  TOKEN_DOGRU,
	"yanlış": TOKEN_YANLIS,
	"yanlis": TOKEN_YANLIS,
	"yok":    TOKEN_YOK,
	"ve":     TOKEN_VE,
	"veya":   TOKEN_VEYA,
	"değil":  TOKEN_DEGIL,
	"degil":  TOKEN_DEGIL,

	// Yapı, Sınıf, Kendi
	"yapı":  TOKEN_YAPI,
	"yapi":  TOKEN_YAPI,
	"sınıf": TOKEN_SINIF,
	"sinif": TOKEN_SINIF,
	"kendi": TOKEN_KENDI,

	// Hata yönetimi, Modül, Eşzamanlılık, Ertele, Kesme Noktası (ABAP BREAK-POINT)
	"dene":          TOKEN_DENE,
	"yakala":        TOKEN_YAKALA,
	"fırlat":        TOKEN_FIRLAT,
	"firlat":        TOKEN_FIRLAT,
	"aktar":         TOKEN_AKTAR,
	"eşzamanlı":     TOKEN_ESZAMANLI,
	"eszamanli":     TOKEN_ESZAMANLI,
	"ertele":        TOKEN_ERTELE,
	"kesme_noktası": TOKEN_KESME_NOKTASI,
	"kesme_noktasi": TOKEN_KESME_NOKTASI,
	"dur_noktası":   TOKEN_KESME_NOKTASI,
	"dur_noktasi":   TOKEN_KESME_NOKTASI,
	"break_point":   TOKEN_KESME_NOKTASI,
}

// LookupIdent verilen kelimenin anahtar kelime mi yoksa tanımlayıcı mı olduğunu bulur.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENT
}

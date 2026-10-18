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

	// Anahtar Kelimeler (Birlikte Seçilen TR Sözlüğü). Yazımları anahtar_kelimeler.go'daki
	// tablolardadır; aşağıdaki yorumlar yalnızca hatırlatmadır.
	TOKEN_DEGISKEN      = "DEĞİŞKEN"      // değişken / degisken
	TOKEN_SABIT         = "SABİT"         // sabit
	TOKEN_FONKSIYON     = "FONKSİYON"     // fonksiyon / işlev / islev
	TOKEN_SONUC         = "SONUÇ"         // sonuç / sonuc / döndür / dondur
	TOKEN_EGER          = "EĞER"          // eğer / eger
	TOKEN_DEGILSE       = "DEĞİLSE"       // değilse / degilse
	TOKEN_ESLE          = "EŞLE"          // eşle / esle
	TOKEN_VARSAYILAN    = "VARSAYILAN"    // varsayılan / varsayilan
	TOKEN_DONGU         = "DÖNGÜ"         // döngü / dongu
	TOKEN_ICINDE        = "İÇİNDE"        // içinde / icinde
	TOKEN_IKEN          = "İKEN"          // iken
	TOKEN_CIK           = "ÇIK"           // çık / cik / kır / kir
	TOKEN_DEVAM         = "DEVAM"         // devam
	TOKEN_DOGRU         = "DOĞRU"         // doğru / dogru
	TOKEN_YANLIS        = "YANLIŞ"        // yanlış / yanlis
	TOKEN_YOK           = "YOK"           // yok
	TOKEN_VE            = "VE"            // ve
	TOKEN_VEYA          = "VEYA"          // veya
	TOKEN_DEGIL         = "DEĞİL"         // değil / degil
	TOKEN_YAPI          = "YAPI"          // yapı / yapi
	TOKEN_KENDI         = "KENDİ"         // kendi
	TOKEN_DENE          = "DENE"          // dene
	TOKEN_YAKALA        = "YAKALA"        // yakala
	TOKEN_FIRLAT        = "FIRLAT"        // fırlat / firlat
	TOKEN_AKTAR         = "AKTAR"         // aktar
	TOKEN_ESZAMANLI     = "EŞZAMANLI"     // eşzamanlı / eszamanli
	TOKEN_ERTELE        = "ERTELE"        // ertele (defer)
	TOKEN_KESME_NOKTASI = "KESME_NOKTASI" // kesme_noktası
)

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

	// Anahtar Kelimeler. Tür adları içseldir, kullanıcıya gösterilmez; kullanıcının
	// yazdığı adlar sözlüktedir (internal/dil/kod, A0xx) ve hangi kodun hangi türü ürettiği
	// anahtar_kelimeler.go'dadır.
	TOKEN_DEGISKEN      = "DEĞİŞKEN"      // A030
	TOKEN_SABIT         = "SABİT"         // A031
	TOKEN_FONKSIYON     = "FONKSİYON"     // A020
	TOKEN_SONUC         = "SONUÇ"         // A021
	TOKEN_EGER          = "EĞER"          // A001
	TOKEN_DEGILSE       = "DEĞİLSE"       // A002
	TOKEN_ESLE          = "EŞLE"          // A003
	TOKEN_VARSAYILAN    = "VARSAYILAN"    // A004
	TOKEN_DONGU         = "DÖNGÜ"         // A010
	TOKEN_ICINDE        = "İÇİNDE"        // A011
	TOKEN_IKEN          = "İKEN"          // A012
	TOKEN_CIK           = "ÇIK"           // A013
	TOKEN_DEVAM         = "DEVAM"         // A014
	TOKEN_DOGRU         = "DOĞRU"         // A040
	TOKEN_YANLIS        = "YANLIŞ"        // A041
	TOKEN_YOK           = "YOK"           // A042
	TOKEN_VE            = "VE"            // A043
	TOKEN_VEYA          = "VEYA"          // A044
	TOKEN_DEGIL         = "DEĞİL"         // A045
	TOKEN_YAPI          = "YAPI"          // A050
	TOKEN_KENDI         = "KENDİ"         // A051
	TOKEN_DENE          = "DENE"          // A060
	TOKEN_YAKALA        = "YAKALA"        // A061
	TOKEN_FIRLAT        = "FIRLAT"        // A062
	TOKEN_AKTAR         = "AKTAR"         // A070
	TOKEN_ESZAMANLI     = "EŞZAMANLI"     // A080
	TOKEN_ERTELE        = "ERTELE"        // A081
	TOKEN_KESME_NOKTASI = "KESME_NOKTASI" // A090
)

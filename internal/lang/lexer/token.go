package lexer

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// TokenType is the type of a token.
type TokenType string

// Token is one token with its position.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
	// Code is the code the word was written with in a coded file (A001, G100), otherwise
	// codes.None. Literal is then the code's name in the active language (see coded.go).
	Code codes.Code
}

func (t Token) String() string {
	return fmt.Sprintf("[%d:%d] %s (%q)", t.Line, t.Column, t.Type, t.Literal)
}

const (
	TOKEN_ILLEGAL = "GEÇERSİZ"
	TOKEN_EOF     = "DOSYA_SONU"

	// Identifiers and literals
	TOKEN_IDENT  = "TANIMLAYICI"
	TOKEN_INT    = "TAM_SAYI"
	TOKEN_FLOAT  = "ONDALIK_SAYI"
	TOKEN_STRING = "METİN"

	// Operators
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

	// Delimiters
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

	// Keywords. The type names are internal and never shown to the user; the names the user
	// writes are in the dictionary (internal/lang/codes, A0xx), and keywords.go says which
	// code produces which type.
	TOKEN_VAR        = "DEĞİŞKEN"      // A030
	TOKEN_CONST      = "SABİT"         // A031
	TOKEN_FUNCTION   = "FONKSİYON"     // A020
	TOKEN_RETURN     = "SONUÇ"         // A021
	TOKEN_IF         = "EĞER"          // A001
	TOKEN_ELSE       = "DEĞİLSE"       // A002
	TOKEN_MATCH      = "EŞLE"          // A003
	TOKEN_DEFAULT    = "VARSAYILAN"    // A004
	TOKEN_FOR        = "DÖNGÜ"         // A010
	TOKEN_IN         = "İÇİNDE"        // A011
	TOKEN_WHILE      = "İKEN"          // A012
	TOKEN_BREAK      = "ÇIK"           // A013
	TOKEN_CONTINUE   = "DEVAM"         // A014
	TOKEN_TRUE       = "DOĞRU"         // A040
	TOKEN_FALSE      = "YANLIŞ"        // A041
	TOKEN_NIL        = "YOK"           // A042
	TOKEN_KW_AND     = "VE"            // A043
	TOKEN_KW_OR      = "VEYA"          // A044
	TOKEN_KW_NOT     = "DEĞİL"         // A045
	TOKEN_STRUCT     = "YAPI"          // A050
	TOKEN_SELF       = "KENDİ"         // A051
	TOKEN_TRY        = "DENE"          // A060
	TOKEN_CATCH      = "YAKALA"        // A061
	TOKEN_THROW      = "FIRLAT"        // A062
	TOKEN_IMPORT     = "AKTAR"         // A070
	TOKEN_CONCURRENT = "EŞZAMANLI"     // A080
	TOKEN_DEFER      = "ERTELE"        // A081
	TOKEN_BREAKPOINT = "KESME_NOKTASI" // A090
)

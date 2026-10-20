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
	TokIllegal = "GEÇERSİZ"
	TokEOF     = "DOSYA_SONU"

	// Identifiers and literals
	TokIdent  = "TANIMLAYICI"
	TokInt    = "TAM_SAYI"
	TokFloat  = "ONDALIK_SAYI"
	TokString = "METİN"

	// Operators
	TokAssign      = "="
	TokWalrus      = ":="
	TokPlus        = "+"
	TokMinus       = "-"
	TokAsterisk    = "*"
	TokSlash       = "/"
	TokPercent     = "%"
	TokPower       = "**"
	TokMatmul      = "@"
	TokBang        = "!"
	TokEq          = "=="
	TokNotEq       = "!="
	TokLt          = "<"
	TokGt          = ">"
	TokLte         = "<="
	TokGte         = ">="
	TokAnd         = "&&"
	TokOr          = "||"
	TokPipe        = "|>"
	TokNullCoalesc = "??"
	TokSafeDot     = "?."
	TokArrow       = "=>"
	TokRange       = ".."
	TokRangeIncl   = "..="
	TokPlusAssign  = "+="
	TokMinusAssign = "-="
	TokMulAssign   = "*="
	TokDivAssign   = "/="

	// Delimiters
	TokComma     = ","
	TokSemicolon = ";"
	TokColon     = ":"
	TokDot       = "."
	TokQuestion  = "?"
	TokLParen    = "("
	TokRParen    = ")"
	TokLBrace    = "{"
	TokRBrace    = "}"
	TokLBracket  = "["
	TokRBracket  = "]"

	// Keywords. The type names are internal and never shown to the user; the names the user
	// writes are in the dictionary (internal/lang/codes, A0xx), and keywords.go says which
	// code produces which type.
	TokVar        = "DEĞİŞKEN"      // A030
	TokConst      = "SABİT"         // A031
	TokFunction   = "FONKSİYON"     // A020
	TokReturn     = "SONUÇ"         // A021
	TokIf         = "EĞER"          // A001
	TokElse       = "DEĞİLSE"       // A002
	TokMatch      = "EŞLE"          // A003
	TokDefault    = "VARSAYILAN"    // A004
	TokFor        = "DÖNGÜ"         // A010
	TokIn         = "İÇİNDE"        // A011
	TokWhile      = "İKEN"          // A012
	TokBreak      = "ÇIK"           // A013
	TokContinue   = "DEVAM"         // A014
	TokTrue       = "DOĞRU"         // A040
	TokFalse      = "YANLIŞ"        // A041
	TokNil        = "YOK"           // A042
	TokKwAnd      = "VE"            // A043
	TokKwOr       = "VEYA"          // A044
	TokKwNot      = "DEĞİL"         // A045
	TokStruct     = "YAPI"          // A050
	TokSelf       = "KENDİ"         // A051
	TokTry        = "DENE"          // A060
	TokCatch      = "YAKALA"        // A061
	TokThrow      = "FIRLAT"        // A062
	TokImport     = "AKTAR"         // A070
	TokConcurrent = "EŞZAMANLI"     // A080
	TokDefer      = "ERTELE"        // A081
	TokBreakpoint = "KESME_NOKTASI" // A090
)

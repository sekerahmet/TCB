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
	TokIllegal = "ILLEGAL"
	TokEOF     = "EOF"

	// Identifiers and literals
	TokIdent  = "IDENT"
	TokInt    = "INT"
	TokFloat  = "FLOAT"
	TokString = "STRING"

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
	TokVar        = "VAR"        // A030
	TokConst      = "CONST"      // A031
	TokFunction   = "FUNCTION"   // A020
	TokReturn     = "RETURN"     // A021
	TokIf         = "IF"         // A001
	TokElse       = "ELSE"       // A002
	TokMatch      = "MATCH"      // A003
	TokDefault    = "DEFAULT"    // A004
	TokFor        = "FOR"        // A010
	TokIn         = "IN"         // A011
	TokWhile      = "WHILE"      // A012
	TokBreak      = "BREAK"      // A013
	TokContinue   = "CONTINUE"   // A014
	TokTrue       = "TRUE"       // A040
	TokFalse      = "FALSE"      // A041
	TokNil        = "NIL"        // A042
	TokKwAnd      = "AND"        // A043
	TokKwOr       = "OR"         // A044
	TokKwNot      = "NOT"        // A045
	TokStruct     = "STRUCT"     // A050
	TokSelf       = "SELF"       // A051
	TokTry        = "TRY"        // A060
	TokCatch      = "CATCH"      // A061
	TokThrow      = "THROW"      // A062
	TokImport     = "IMPORT"     // A070
	TokConcurrent = "CONCURRENT" // A080
	TokDefer      = "DEFER"      // A081
	TokBreakpoint = "BREAKPOINT" // A090
)

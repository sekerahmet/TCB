package lexer

import (
	"fmt"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// TokenType is the type of a token.
type TokenType string

// IsOpener açan parantez, köşeli parantez ya da süslü parantezdir; ayrıştırıcının hata sonrası
// toparlanması ve biçimlendiricinin iç içelik sayımı bunu kullanır.
func (t TokenType) IsOpener() bool { return t == TokLBrace || t == TokLParen || t == TokLBracket }

// IsCloser kapayan parantez, köşeli parantez ya da süslü parantezdir.
func (t TokenType) IsCloser() bool { return t == TokRBrace || t == TokRParen || t == TokRBracket }

// Token is one token with its position.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
	// Code is the code the word was written with in a coded file (TR_kw_if, TR_fn_print), otherwise
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
	// TokMessageCode is a message code written in the source: H and four digits as a whole word
	// (H6903). The parser reads it as the string of the code.
	TokMessageCode = "MESSAGE_CODE"

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
	// writes are in the dictionary (internal/lang/codes, kw), and keywords.go says which
	// code produces which type.
	TokVar        = "VAR"        // TR_kw_var
	TokConst      = "CONST"      // TR_kw_const
	TokFunction   = "FUNCTION"   // TR_kw_func
	TokReturn     = "RETURN"     // TR_kw_return
	TokIf         = "IF"         // TR_kw_if
	TokElse       = "ELSE"       // TR_kw_else
	TokMatch      = "MATCH"      // TR_kw_match
	TokDefault    = "DEFAULT"    // TR_kw_default
	TokFor        = "FOR"        // TR_kw_for
	TokIn         = "IN"         // TR_kw_in
	TokWhile      = "WHILE"      // TR_kw_while
	TokBreak      = "BREAK"      // TR_kw_break
	TokContinue   = "CONTINUE"   // TR_kw_continue
	TokTrue       = "TRUE"       // TR_kw_true
	TokFalse      = "FALSE"      // TR_kw_false
	TokNil        = "NIL"        // TR_kw_none
	TokKwAnd      = "AND"        // TR_kw_and
	TokKwOr       = "OR"         // TR_kw_or
	TokKwNot      = "NOT"        // TR_kw_not
	TokStruct     = "STRUCT"     // TR_kw_struct
	TokSelf       = "SELF"       // TR_kw_self
	TokTry        = "TRY"        // TR_kw_try
	TokCatch      = "CATCH"      // TR_kw_catch
	TokThrow      = "THROW"      // TR_kw_throw
	TokImport     = "IMPORT"     // TR_kw_import
	TokConcurrent = "CONCURRENT" // TR_kw_async
	TokDefer      = "DEFER"      // TR_kw_defer
	TokBreakpoint = "BREAKPOINT" // TR_kw_breakpoint
)

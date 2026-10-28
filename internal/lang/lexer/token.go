package lexer

import (
	"fmt"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
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
	// EndLine and EndColumn are the position after the token's last character, in the file's
	// own line and column (exclusive, like LSP and rustc); 0 for a token the lexer did not make.
	// Range gives start and end together.
	EndLine, EndColumn int
	// Code is the code the word was written with (an identity TR_print, or a word of the writing
	// language the body is read in), otherwise codes.None. Literal is then the code's name in the
	// lexer's language (see identity.go).
	Code codes.Code
	// Escaped says a name was written with the '$' escape in a text read in a writing language (a shell
	// line, a file shown in a language): the token's range includes the '$', its Literal does not. A
	// coded file has no '$' (it is an error there), so its tokens are never Escaped.
	Escaped bool
	// Raw says a string token was written as a raw string (`...`, 2.6a).
	Raw bool
}

// Range is the token's place with its end (messages.Range). A token without an end of its own
// (one the parser made) ends after its text, on its line; with no text it ends one character on.
func (t Token) Range() messages.Range {
	r := messages.Range{Line: t.Line, Col: t.Column}
	switch {
	case t.Line <= 0 || t.Column <= 0:
	case t.EndLine > 0 && t.EndColumn > 0:
		r.EndLine, r.EndCol = t.EndLine, t.EndColumn
	default:
		r.EndLine, r.EndCol = t.Line, t.Column+max(utf8.RuneCountInString(t.Literal), 1)
	}
	return r
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
	TokSlashSlash  = "//"
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
	TokBitAnd      = "&"
	TokBitOr       = "|"
	TokBitXor      = "^"
	TokTilde       = "~"
	TokShl         = "<<"
	TokShr         = ">>"
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
	TokFor        = "FOR"        // TR_kw_for
	TokIn         = "IN"         // TR_kw_in
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

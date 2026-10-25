// Package source is TR's "source to program" entry point: it splits the text of a .tr file
// into its parts (BOM, coded-form header, body), picks the right lexer, parses, optionally
// type-checks, and returns errors and warnings in one shape. The command line, the REPL, the
// test runner, the debug server and the module loader read files only through here. The coded
// form (docs/spec/17_kodlar.md, 17.1-17.6) and its converter (Encode, Decode) live only in
// this package too.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package source

import (
	"slices"
	"strings"
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Header is the start of a coded file's first line (17.1, format version 2); the code of the
// file's writing language follows it: "//! tr kodlu 2 dil=en". The language is the file's, not
// the person's: a value's text follows the language of the file the operation is written in
// (3), and each person still sees and edits the file in their own writing language.
const Header = lexer.CodedHeader

// headerPrefix kodlu biçim başlıklarının ortak başıdır; bu önekle başlayıp Header ve bilinen
// bir dil kodu olmayan ilk satır, bu sürümün okuyamadığı bir biçimdir (sessizce Türkçe kaynak
// sayılmaz).
const headerPrefix = "//! tr kodlu"

const bom = "\xef\xbb\xbf"

// Parts are the parts of a source file. Join puts them back together byte for byte.
type Parts struct {
	// BOM reports whether the file starts with a UTF-8 BOM.
	BOM bool
	// Coded reports whether the file is in coded form (its first line is Header and a language).
	Coded bool
	// Language is the code of the file's writing language: the header's for a coded file,
	// Turkish (codes.SourceLanguage) for a file without the header (17.7).
	Language string
	// HeaderEOL is the end of the header line ("\n" or "\r\n"; empty when the header is the
	// file's only line).
	HeaderEOL string
	// Body is the text after the header (if any). Line numbers count from the body (17.5): line
	// 1 of a coded file is the line under the header, the same as line 1 of the Turkish view.
	Body string
}

// HeaderError is returned when a file's first line looks like a coded-form header but is
// not the header this version can read: another format version, or an unknown language.
type HeaderError struct {
	Line string
}

// Unwrap returns the message at its position (line 1, column 1), so frames and tools read the
// position from the error (messages.Pos), not from its text.
func (e *HeaderError) Unwrap() error {
	languages := messages.OrList(codes.Languages()...)
	switch language, ok := strings.CutPrefix(e.Line, Header); {
	case ok && language != "":
		_, err := codes.LoadLanguage(language)
		return messages.At(1, 1, err)
	case ok || e.Line == strings.TrimSuffix(Header, languageField):
		// Sürüm okunur, dosyanın dili eksik.
		return messages.At(1, 1, messages.New(messages.H2523, e.Line, Header, languages))
	}
	return messages.At(1, 1, messages.New(messages.H2512, e.Line, Header, languages, headerPrefix))
}

// languageField başlıkta dosyanın dilini veren alanın başıdır (Header bununla biter).
const languageField = " dil="

func (e *HeaderError) Error() string {
	return e.Unwrap().Error()
}

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *HeaderError) MessageText(lang string) string { return messages.ErrorIn(e.Unwrap(), lang) }

// Split splits text into its parts. If the first line (after the BOM) is exactly Header and the
// code of an embedded writing language the file is coded; otherwise it is source written in
// Turkish. If the first line starts with the header prefix but is not such a header (an older
// or newer format version, an unknown language), it returns *HeaderError.
func Split(text string) (Parts, error) {
	b := Parts{Language: codes.SourceLanguage}
	if strings.HasPrefix(text, bom) {
		b.BOM = true
		text = text[len(bom):]
	}
	first, rest, eol := firstLine(text)
	language, header := strings.CutPrefix(first, Header)
	switch {
	case header && slices.Contains(codes.Languages(), language):
		b.Coded, b.Language, b.HeaderEOL, b.Body = true, language, eol, rest
	case strings.HasPrefix(first, headerPrefix):
		b.Body = text
		return b, &HeaderError{Line: first}
	default:
		b.Body = text
	}
	return b, nil
}

// firstLine metnin ilk satırını, ondan sonrasını ve aradaki satır sonunu verir.
func firstLine(text string) (first, rest, eol string) {
	i := strings.IndexByte(text, '\n')
	if i < 0 {
		return text, "", ""
	}
	first, eol = text[:i], "\n"
	if strings.HasSuffix(first, "\r") {
		first, eol = first[:len(first)-1], "\r\n"
	}
	return first, text[i+1:], eol
}

// Join turns the parts back into file text (the inverse of Split).
func (b Parts) Join() string {
	var sb strings.Builder
	if b.BOM {
		sb.WriteString(bom)
	}
	if b.Coded {
		sb.WriteString(Header)
		sb.WriteString(b.Language)
		sb.WriteString(b.HeaderEOL)
	}
	sb.WriteString(b.Body)
	return sb.String()
}

// Turkish is the language's own writing language: a file without the coded header is written
// in Turkish whatever the active language is (17.7).
var Turkish = sync.OnceValue(func() *codes.Language {
	l, err := codes.LoadLanguage(codes.SourceLanguage)
	if err != nil {
		panic(messages.Internal("the embedded Turkish code list cannot be built"))
	}
	return l
})

// Lexer returns the body's lexer: for a coded file, one that recognizes codes and whose
// positions follow the file's view (17.5), the '$' of escaped user names included; for a file
// written in Turkish, one that reads it in Turkish (the canonical language, whatever TR_DIL is).
func (b Parts) Lexer() *lexer.Lexer {
	if !b.Coded {
		return lexer.NewBody(b.Body, false)
	}
	_, l := view(b)
	return l
}

// scanLexer kodları tanıyan, sütunları yalnızca kod adlarına göre kayan çözümleyicidir:
// gösterimi kuran çözümleme (analyze) bunu kullanır, gösterim henüz yoktur. Aralıklar
// (Record.Start/End) Lexer'ınkiyle aynıdır. Kodsuz gövde lang'de yazılmış okunur; kodlu
// gövdenin kodları kanonik adlarıyla okunur (gösterimin kaçışları kanonik ağaçtan, adlar
// lang'de aranarak bulunur; checker.NameBindings).
func (b Parts) scanLexer(lang *codes.Language) *lexer.Lexer {
	if b.Coded {
		return lexer.NewBody(b.Body, true)
	}
	return lexer.NewBodyIn(b.Body, false, lang)
}

// findEOL bir gövdenin satır sonu biçimidir: ilk satır sonu CRLF ise "\r\n", yoksa "\n".
// Kodla başlığı gövdenin biçimiyle yazar.
func findEOL(body string) string {
	if i := strings.IndexByte(body, '\n'); i > 0 && body[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

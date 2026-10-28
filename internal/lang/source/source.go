// Package source is TR's "source to program" entry point: it checks the header of a .tr file
// (BOM, '#!', the '# TR 4' header), picks the right lexer, parses, optionally type-checks, and
// returns errors and warnings in one shape. The command line, the REPL, the test runner, the debug
// server and the module loader read files only through here. The converter between the coded
// form and a writing language's view (Encode, Decode) lives only in this package too.
//
// Layer: language core (internal/lang); imports only internal/lang packages.
package source

import (
	"slices"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Header is the first line of every TR file (2.1; the second when the first starts with '#!'). It
// is a comment and carries no language: the file is written with identities (TR_print), and each
// person sees it in their own writing language.
const Header = "# TR 4"

// headerPrefix başlıkların ortak başıdır; bu önekle başlayıp Header olmayan satır bu sürümün
// okuyamadığı bir biçimdir (sessizce başlıksız sayılmaz).
const headerPrefix = "# TR "

// viewMarker görünüm başlığının işaretidir: 'tr göster' başlığa görünümün dilini yazar
// ('# TR 4 view=tr'); satır sayısı değişmez. Dosyanın türü dosyanın kendisindedir: görünüm
// çalışmaz (ViewError), 'tr kodla' dili oradan okur.
const viewMarker = " view="

// shebang ilk satırdaki yorumdur: dosya betik gibi doğrudan çalıştırılır (./program.tr, 2.1).
const shebang = "#!"

const bom = "\xef\xbb\xbf"

// Parts are the parts of a source file. Join puts them back together byte for byte.
type Parts struct {
	// BOM reports whether the file starts with a UTF-8 BOM (it is ignored, 2.1).
	BOM bool
	// Coded reports whether the file has the header: it is a TR file, written with identities.
	Coded bool
	// View is the writing language of a view (its header '# TR 4 view=tr', tr göster's output);
	// "" otherwise. A view is not coded and does not run (ViewError).
	View string
	// Body is the text after the BOM, the '#!' and header lines included: they are comments, and
	// positions are the file's own (line 1 is the file's first line).
	Body string
}

// HeaderError is returned when the line where the header stands looks like a header but is not
// the one this version reads (another format version).
type HeaderError struct {
	Line string
	// At is the line's number (1, or 2 after '#!').
	At int
}

// Unwrap returns the message at its position, so frames and tools read the position from the
// error (messages.Pos), not from its text.
func (e *HeaderError) Unwrap() error {
	return messages.At(e.At, 1, messages.New(messages.H2512, e.Line, Header))
}

func (e *HeaderError) Error() string {
	return e.Unwrap().Error()
}

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *HeaderError) MessageText(lang string) string { return messages.ErrorIn(e.Unwrap(), lang) }

// NoHeaderError is the error of a file without the header: it is text in a writing language (or
// not TR at all) and does not run; the message gives the encoding command, with the person's
// language as the text's language (2.1).
func NoHeaderError() error {
	return messages.At(1, 1, messages.New(messages.H2540, Header, codes.NameRef(codes.CmdEncode),
		codes.NameRef(codes.OptLang), codes.Display().Code))
}

// ViewError is returned for a view file: its header carries the view's writing language
// ('# TR 4 view=tr', tr göster's output). A view is text in a writing language and does not run;
// the message gives the encoding command.
type ViewError struct {
	Lang string
	// At is the header line's number (1, or 2 after '#!').
	At int
}

// Unwrap returns the message at its position (messages.Pos).
func (e *ViewError) Unwrap() error {
	return messages.At(e.At, 1, messages.New(messages.H2541, e.Lang, codes.NameRef(codes.CmdEncode)))
}

func (e *ViewError) Error() string { return e.Unwrap().Error() }

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (e *ViewError) MessageText(lang string) string { return messages.ErrorIn(e.Unwrap(), lang) }

// Split splits text into its parts. The header is the first line, or the second when the first
// starts with '#!'; if that line starts with the header's prefix but is not the header (another
// format version), it returns *HeaderError. A view's header ('# TR 4 view=tr') sets View and
// returns *ViewError: only the encoder reads a view.
func Split(text string) (Parts, error) {
	b := Parts{}
	if strings.HasPrefix(text, bom) {
		b.BOM = true
		text = text[len(bom):]
	}
	b.Body = text
	at := 1
	line, rest, _ := firstLine(text)
	if strings.HasPrefix(line, shebang) {
		line, _, _ = firstLine(rest)
		at = 2
	}
	switch {
	case line == Header:
		b.Coded = true
	case strings.HasPrefix(line, Header+viewMarker) && slices.Contains(codes.Languages(), line[len(Header+viewMarker):]):
		b.View = line[len(Header+viewMarker):]
		return b, &ViewError{Lang: b.View, At: at}
	case strings.HasPrefix(line, headerPrefix):
		return b, &HeaderError{Line: line, At: at}
	}
	return b, nil
}

// viewHeader lang'deki görünümün başlığıdır ('# TR 4 view=tr').
func viewHeader(lang *codes.Language) string { return Header + viewMarker + lang.Code }

// setHeader gövdenin başlık satırını (ilk satır, '#!' varsa ikincisi) header yapar; satır sonu ve
// öbür satırlar olduğu gibi kalır.
func setHeader(body, header string) string {
	prefix := ""
	line, rest, eol := firstLine(body)
	if strings.HasPrefix(line, shebang) {
		prefix = line + eol
		_, rest, eol = firstLine(rest)
	}
	return prefix + header + eol + rest
}

// HeaderLines is the number of the text's first lines that are its '#!' line and header (0 when
// the text has no header): tools that read a file's lines skip them, so the header is not a doc
// comment and a template body does not carry it.
func HeaderLines(text string) int {
	b, err := Split(text)
	if b.View == "" && (err != nil || !b.Coded) {
		return 0
	}
	if strings.HasPrefix(b.Body, shebang) {
		return 2
	}
	return 1
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
	if b.BOM {
		return bom + b.Body
	}
	return b.Body
}

// Turkish Türkçe yazım dilidir: Go testlerinin ve araçların Türkçe yazılmış metni (TextLexer)
// onunla okunur. Gömülü liste kurulamadıysa kimlik listesidir ve codes.LanguageError nedenini söyler.
func Turkish() *codes.Language { return codes.Source() }

// TextLexer is the lexer of a text written in Turkish without the header (a test's or a tool's
// snippet): the text is converted to coded form first, so the program reads codes; a text that
// cannot be converted is read as it is.
func TextLexer(text string) *lexer.Lexer { return textLexerIn(text, nil, Turkish()) }

// TextLexerWithNames is the lexer of a text the person types inside a running program's scope
// (the debugger's expression, condition and watch): it is written in the person's display
// language (codes.Display, TR_DIL), like a shell line. A name in names is the program's own and is
// not encoded as a language element (the user's toplam variable stays a variable).
func TextLexerWithNames(text string, names []string) *lexer.Lexer {
	return textLexerIn(text, names, codes.Display())
}

// textLexerIn lang'de yazılmış metnin sözcük çözümleyicisidir: metin önce kodlu biçime çevrilir,
// konumlar lang'deki gösterime göredir; çevrilemeyen metin olduğu gibi okunur.
func textLexerIn(text string, names []string, lang *codes.Language) *lexer.Lexer {
	if coded, err := toCoded(text, lang, names, false); err == nil {
		_, l := view(coded, lang)
		return l
	}
	return lexer.NewBodyIn(text, false, lang)
}

// Lexer is the body's lexer for the text tools (the formatter, tr build): a coded body is read as
// it is (identities, positions on disk); a body without the header is a view written in Turkish.
func (b Parts) Lexer() *lexer.Lexer {
	if b.Coded {
		return lexer.NewBody(b.Body, true)
	}
	return lexer.NewBodyIn(b.Body, false, Turkish())
}

// IdentityLexer returns the body's lexer as the interpreter reads it, for tools that keep a program's
// facts independent of the reader's language (the code index): names that are language elements come as
// identities (TR_fn_print), user names as written. A coded body is read as it is; a body without the
// header is a view written in Turkish and is encoded first. Positions follow the text on disk.
func (b Parts) IdentityLexer() *lexer.Lexer {
	if b.Coded {
		return lexer.NewBody(b.Body, true)
	}
	return TextLexer(b.Body)
}

// findEOL bir gövdenin satır sonu biçimidir: ilk satır sonu CRLF ise "\r\n", yoksa "\n".
// Kodlama başlığı gövdenin biçimiyle yazar.
func findEOL(body string) string {
	if i := strings.IndexByte(body, '\n'); i > 0 && body[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

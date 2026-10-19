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
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/lexer"
)

// Header is the first line of a coded file (17.1): format version 1. The display language is
// not stored in the file; each person picks their own writing language.
const Header = lexer.CodedHeader

// headerPrefix kodlu biçim başlıklarının ortak başıdır; bu önekle başlayıp Baslik olmayan
// bir ilk satır, bu sürümün okuyamadığı bir biçimdir (sessizce Türkçe kaynak sayılmaz).
const headerPrefix = "//! tr kodlu"

const bom = "\xef\xbb\xbf"

// Parts are the parts of a source file. Join puts them back together byte for byte.
type Parts struct {
	// BOM reports whether the file starts with a UTF-8 BOM.
	BOM bool
	// Coded reports whether the file is in coded form (its first line is Header).
	Coded bool
	// HeaderEOL is the end of the header line ("\n" or "\r\n"; empty when the header is the
	// file's only line).
	HeaderEOL string
	// Body is the text after the header (if any). Line numbers count from the body (17.5): line
	// 1 of a coded file is the line under the header, the same as line 1 of the Turkish view.
	Body string
}

// HeaderError is returned when a file's first line looks like a coded-form header but is
// not the header this version can read.
type HeaderError struct {
	Line string
}

func (e *HeaderError) Error() string {
	return fmt.Sprintf("Satır 1, Sütun 1: '%s' bu yorumlayıcının tanımadığı bir kodlu biçim başlığı; yalnızca '%s' okunur (dosya daha yeni bir TR sürümüyle yazılmış olabilir; '%s' ile başlayan ilk satır kodlu biçim için ayrılmıştır)",
		e.Line, Header, headerPrefix)
}

// Split splits text into its parts. If the first line (after the BOM) is exactly Header the
// file is coded; otherwise it is source written in Turkish (backward compatible). If the first
// line starts with the header prefix but is not Header, it returns *HeaderError.
func Split(text string) (Parts, error) {
	var b Parts
	if strings.HasPrefix(text, bom) {
		b.BOM = true
		text = text[len(bom):]
	}
	first, rest, eol := firstLine(text)
	switch {
	case first == Header:
		b.Coded, b.HeaderEOL, b.Body = true, eol, rest
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
		sb.WriteString(b.HeaderEOL)
	}
	sb.WriteString(b.Body)
	return sb.String()
}

// Lexer returns the body's lexer: for a coded file, one that recognizes codes.
func (b Parts) Lexer() *lexer.Lexer {
	return lexer.NewBody(b.Body, b.Coded)
}

// findEOL bir gövdenin satır sonu biçimidir: ilk satır sonu CRLF ise "\r\n", yoksa "\n".
// Kodla başlığı gövdenin biçimiyle yazar.
func findEOL(body string) string {
	if i := strings.IndexByte(body, '\n'); i > 0 && body[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

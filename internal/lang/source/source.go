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

// baslikOneki kodlu biçim başlıklarının ortak başıdır; bu önekle başlayıp Baslik olmayan
// bir ilk satır, bu sürümün okuyamadığı bir biçimdir (sessizce Türkçe kaynak sayılmaz).
const baslikOneki = "//! tr kodlu"

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
		e.Line, Header, baslikOneki)
}

// Split splits text into its parts. If the first line (after the BOM) is exactly Header the
// file is coded; otherwise it is source written in Turkish (backward compatible). If the first
// line starts with the header prefix but is not Header, it returns *HeaderError.
func Split(metin string) (Parts, error) {
	var b Parts
	if strings.HasPrefix(metin, bom) {
		b.BOM = true
		metin = metin[len(bom):]
	}
	ilk, kalan, satirSonu := ilkSatir(metin)
	switch {
	case ilk == Header:
		b.Coded, b.HeaderEOL, b.Body = true, satirSonu, kalan
	case strings.HasPrefix(ilk, baslikOneki):
		b.Body = metin
		return b, &HeaderError{Line: ilk}
	default:
		b.Body = metin
	}
	return b, nil
}

// ilkSatir metnin ilk satırını, ondan sonrasını ve aradaki satır sonunu verir.
func ilkSatir(metin string) (ilk, kalan, satirSonu string) {
	i := strings.IndexByte(metin, '\n')
	if i < 0 {
		return metin, "", ""
	}
	ilk, satirSonu = metin[:i], "\n"
	if strings.HasSuffix(ilk, "\r") {
		ilk, satirSonu = ilk[:len(ilk)-1], "\r\n"
	}
	return ilk, metin[i+1:], satirSonu
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

// satirSonuBul bir gövdenin satır sonu biçimidir: ilk satır sonu CRLF ise "\r\n", yoksa "\n".
// Kodla başlığı gövdenin biçimiyle yazar.
func satirSonuBul(govde string) string {
	if i := strings.IndexByte(govde, '\n'); i > 0 && govde[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

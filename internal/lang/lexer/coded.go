package lexer

import (
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kodlu gövde (dil tanımı 2.1, 2.3a): dosya diskte dilden bağımsız kimliklerle saklanır. Dilin her
// sözcüğü TR_<ad> kimliğiyle yazılır (TR_if, TR_print, x.TR_append, TR_lr:); kimlik olmayan her ad
// kullanıcının adıdır ve hiçbir zaman anahtar kelime sayılmaz (başka bir dilde yazılmış bir
// kullanıcı adı TR'nin bir anahtar kelimesiyle aynı olabilir). Kimliğin kodu yerinden çıkar
// (identity.go) ve çözümleyicinin dilindeki adıyla gelir. Program kimlik listesinde okunur (Active):
// ayrıştırıcı, denetleyici ve yorumlayıcı hiçbir yazım dilinin adını görmez; çalışma anına yük gelmez.

// NewBody returns the lexer for the body of a source file (the text internal/lang/source checked
// the header of); coded says the body is in coded form. internal/lang/source is the only place
// that recognizes the header and picks the lexer. Unlike New it does not strip a leading U+FEFF:
// every character of the body is part of the source.
//
// Kodlu gövdede konumlar diskteki metne göredir: kimlikler yazıldıkları gibi okunur, sütun kaymaz.
func NewBody(input string, coded bool) *Lexer {
	l := newLexer(input)
	l.coded = coded
	l.checkBidi()
	return l
}

// NewBodyIn lang'de okunan gövdenin sözcük çözümleyicisidir: lang'de yazılmış gösterim ya da
// kimlikleri lang'deki adlarıyla okunan kodlu gövde. Programlar kimliklerle okunur (codes.Active;
// NewBody); bir yazım dili yalnızca o dilde yazılmış metni çevirmek (tr encode - --lang en, kabuk
// satırı) ya da dosyayı o dilde göstermek için okunur. Gösterimde de kimlikler (TR_print) okunur.
func NewBodyIn(input string, coded bool, lang *codes.Language) *Lexer {
	l := NewBody(input, coded)
	l.lang, l.words, l.writtenIn = lang, keywordsOf(lang), lang
	return l
}

// codeTypes anahtar kelime kodu → token türü haritasıdır (typeCodes'un tersi).
var codeTypes = func() map[codes.Code]TokenType {
	m := make(map[codes.Code]TokenType, len(typeCodes))
	for t, k := range typeCodes {
		m[k] = t
	}
	return m
}()

// KeywordCode returns the code of a keyword token type, or codes.None for other types.
func KeywordCode(t TokenType) codes.Code {
	return typeCodes[t]
}

// nameToken okunan bir adın sözcük birimidir: TR_ ile başlayan ad bir kimliktir ve yerine göre
// koda çözülür (identity.go); kodlu gövdede öbür adlar kullanıcının adıdır, gösterimde gösterim
// dilinin anahtar kelimesi ya da ad.
func (l *Lexer) nameToken(lit string, line, col int) Token {
	if isIdentitySpelling(lit) {
		if k := l.identity(lit, line, col); k != codes.None {
			return l.codeToken(k, line, col)
		}
		// Kalkan sözcükten sonra TR'deki yazımıyla sürülür: ileti bir kez çıkar, ayrıştırıcı ardından
		// yanıltıcı ikinci bir hata vermez.
		switch removedWords[lit] {
		case messages.H0021:
			return l.codeToken(codes.KwFor, line, col)
		case messages.H0022:
			return Token{Type: TokIdent, Literal: "_", Line: line, Column: col}
		}
		return Token{Type: TokIdent, Literal: lit, Line: line, Column: col}
	}
	lit = nfc(lit) // aynı görünen iki yazım aynı addır (2.3)
	if identityLookalike(lit) {
		l.addError(line, col, messages.New(messages.H0030, lit))
	}
	if l.coded {
		return Token{Type: TokIdent, Literal: lit, Line: line, Column: col}
	}
	return Token{Type: l.root.lookupIdent(lit), Literal: lit, Line: line, Column: col}
}

// escapedName '$ad' kaçışını okur: '$' ardından gelen ad her zaman kullanıcının adıdır, dilin bir
// sözcüğüyle aynı olsa bile (değişken $eğer = 1). Kaçış, bir dosya başka bir yazım dilinde
// gösterilirken o dilin sözcükleriyle çakışan kullanıcı adları içindir; kodlama '$'ı atar. TR_ ile
// başlayan adlar kaçışla da ayrılmıştır (2.3a). '$' bir harfe, rakama ya da '_'ye bitişik
// yazılamaz: '$' atılınca iki sözcük birleşirdi (a$b → ab, 1$e5 → 1e5). Kodlu gövdede (dosya)
// kullanıcının adı dilin sözcüğüyle çakışmaz, '$' yoktur (2.3): yazılırsa hata adı '$'sız önerir.
func (l *Lexer) escapedName(line, col int) Token {
	adjacent := l.position > 0 && isNameChar(l.input[l.position-1])
	l.readChar() // '$'
	lit := nfc(l.readIdentifier())
	switch {
	case l.coded:
		l.addError(line, col, messages.New(messages.H0031, lit))
	case adjacent:
		l.addError(line, col, messages.New(messages.H0002))
	}
	if codes.IsReservedName(lit) {
		// Önek ASCII'dir; küçük harfli öneri bu yüzden Türkçe kuralla değil strings.ToLower ile
		// üretilir: TR_ için ır_ değil tr_ önerilir.
		l.addErrorFix(line, col+1, messages.New(messages.H0004, lit, strings.ToLower(lit)), strings.ToLower(lit))
	}
	return Token{Type: TokIdent, Literal: lit, Line: line, Column: col, Escaped: !l.coded}
}

// nulChar kaynağın ortasındaki NUL (U+0000) karakterini hata olarak bildirir. Sözcük
// çözümleyici girdinin sonunu 0 karakteriyle tanır; NUL eskiden dosyanın sonu sayılır, ardından
// gelen kod sessizce yok sayılırdı.
func (l *Lexer) nulChar(line, col int) Token {
	l.addError(line, col, messages.New(messages.H0003))
	l.readChar()
	return Token{Type: TokIllegal, Literal: "\x00", Line: line, Column: col}
}

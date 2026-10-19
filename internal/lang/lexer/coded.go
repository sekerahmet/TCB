package lexer

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Kodlu biçim (docs/spec/17_kodlar.md, 17.1-17.2): dosya diskte dilden bağımsız kodlarla
// saklanır. Kodlu kaynakta dilin her sözcüğü kodla yazılır (A001, G100, .Y004, T910:); kod
// olmayan her ad kullanıcının adıdır ve hiçbir zaman anahtar kelime sayılmaz (başka bir
// dilde yazılmış bir kullanıcı adı TR'nin bir anahtar kelimesiyle aynı olabilir). Kodlar
// burada, sözcük çözümlemede etkin dilin adlarına çevrilir: ayrıştırıcı, denetleyici ve
// yorumlayıcı kodlu dosyayı Türkçe yazılmış gibi görür; çalışma anına yük gelmez.

// NewBody returns the lexer for the body of a source file (the text left after
// internal/lang/source split off the BOM and the coded-form header); kodlu says the body is
// in coded form. internal/lang/source is the only place that recognizes the header and picks
// the lexer. Unlike New it does not strip a leading U+FEFF: every character of the body is
// part of the source.
//
// In a coded body, positions follow the file's Turkish view (17.5): a code (4 characters) is
// shown as its name, so every later column on the line shifts by the name's length. A
// pre-scan finds the shifts (codes inside interpolations included), so error messages, the
// debugger and the editor report the same line and column as the text the person sees.
func NewBody(input string, coded bool) *Lexer {
	if !coded {
		l := newLexer(input)
		l.checkBidi()
		return l
	}
	pre := newLexer(input)
	pre.coded = true
	var k []shift
	pre.scanCodes(&k, 0)
	l := newLexer(input)
	l.coded = true
	l.buildShifts(k)
	l.checkBidi()
	return l
}

// maxInterpolationNesting is how deep the pre-scan follows interpolations inside
// interpolations. It equals the parser's nesting limit: deeper code is a parse error anyway,
// and without a limit each level copies the text below it (quadratic time).
const maxInterpolationNesting = 1000

// shift, kökün girdisindeki off konumundan başlayarak satırın geri kalanının gösterimdeki
// sütun kaymasıdır (kodun adının uzunluğu - 4).
type shift struct {
	off, d int32
}

// scanCodes bütün sözcükleri (yerleştirmelerdekiler dahil) okuyup kodların sütun
// kaymalarını toplar. Bulunan hatalar önemsenmez; asıl çözümleyici onları yeniden bulur.
func (l *Lexer) scanCodes(k *[]shift, depth int) {
	for {
		tok := l.NextToken()
		switch {
		case tok.Type == TOKEN_EOF:
			return
		case tok.Code != codes.None:
			begin, end := l.LastRange()
			if d := utf8.RuneCountInString(tok.Literal) - (end - begin); d != 0 {
				*k = append(*k, shift{off: l.sourcePos(end), d: int32(d)})
			}
		case tok.Type == TOKEN_STRING && depth < maxInterpolationNesting:
			if positions := l.Interpolations(tok); positions != nil {
				value := []rune(tok.Literal)
				for _, y := range positions {
					if expr, begin := InterpolationExpr(value, y[0]+1, y[1]); expr != "" {
						l.Interpolation(tok, expr, begin).scanCodes(k, depth+1)
					}
				}
			}
		}
	}
}

// buildShifts kaymaları konuma göre sıralar ve birikimli toplamlarını kurar. Satır sonundaki
// kaymalar atılır (satır bitmiştir).
func (l *Lexer) buildShifts(k []shift) {
	sort.Slice(k, func(i, j int) bool { return k[i].off < k[j].off })
	l.shifts = k[:0]
	for _, x := range k {
		if int(x.off) >= len(l.input) || l.input[x.off] != '\n' {
			l.shifts = append(l.shifts, x)
		}
	}
	l.shiftSum = make([]int32, len(l.shifts)+1)
	for i, x := range l.shifts {
		l.shiftSum[i+1] = l.shiftSum[i] + x.d
	}
}

// applyShift kök çözümleyici bir kayma noktasına geldiğinde sütunu kaydırır.
func (l *Lexer) applyShift() {
	for l.shiftIdx < len(l.shifts) && int(l.shifts[l.shiftIdx].off) <= l.position {
		if int(l.shifts[l.shiftIdx].off) == l.position {
			l.column += int(l.shifts[l.shiftIdx].d)
		}
		l.shiftIdx++
	}
}

// lineShift kökün girdisinde satır başı bas ile i arasındaki (i dahil) kaymaların toplamıdır.
func (l *Lexer) lineShift(begin, i int) int {
	if len(l.shifts) == 0 {
		return 0
	}
	lo := sort.Search(len(l.shifts), func(j int) bool { return int(l.shifts[j].off) >= begin })
	hi := sort.Search(len(l.shifts), func(j int) bool { return int(l.shifts[j].off) > i })
	return int(l.shiftSum[hi] - l.shiftSum[lo])
}

// codeTypes anahtar kelime kodu → token türü haritasıdır (turKodlari'nın tersi).
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

// nameToken okunan bir adın sözcük birimidir: Türkçe yazılmış kaynakta anahtar kelime ya da
// ad (kod biçimli ad hatadır, 2.3a), kodlu kaynakta kodluSozcuk.
func (l *Lexer) nameToken(lit string, line, col int) Token {
	if l.coded {
		return l.codedToken(lit, line, col)
	}
	kind := LookupIdent(lit)
	if kind == TOKEN_IDENT {
		l.checkCodeForm(lit, line, col)
	}
	return Token{Type: kind, Literal: lit, Line: line, Column: col}
}

// codedToken kodlu kaynaktaki bir adı okur: kod biçimindeyse (G100) kodun etkin dildeki
// adını ve (anahtar kelimeyse) türünü taşıyan sözcük birimi, değilse kullanıcı adıdır.
// Ölü (taşınmış ya da kaldırılmış) kodlar eski adlarıyla okunur: o adın ipucu (13.6, 2.9a)
// kendiliğinden çalışır (uyumluluk sözü, 17.6).
func (l *Lexer) codedToken(lit string, line, col int) Token {
	if !codes.IsCodeForm(lit) {
		return Token{Type: TOKEN_IDENT, Literal: lit, Line: line, Column: col}
	}
	k, err := codes.Parse(lit)
	g, var_ := codes.Active().Entry(k)
	if err != nil || !var_ {
		l.addError(line, col, "'%s' bilinen bir kod değil; kodlu dosyada dilin sözcükleri kod listesindeki kodlarıyla (ör. %s), kullanıcının adları adlarıyla yazılır",
			lit, codes.G100)
		return Token{Type: TOKEN_IDENT, Literal: lit, Line: line, Column: col}
	}
	if t, ok := codeTypes[k]; ok && !g.Dead() {
		return Token{Type: t, Literal: g.Name, Line: line, Column: col, Code: k}
	}
	return Token{Type: TOKEN_IDENT, Literal: g.Name, Line: line, Column: col, Code: k}
}

// escapedName '$ad' kaçışını okur (2.11): '$' ardından gelen ad her zaman kullanıcının adıdır,
// dilin bir sözcüğüyle aynı olsa bile (değişken $eğer = 1). Kaçış, bir dosya başka bir yazım
// dilinde gösterilirken o dilin sözcükleriyle çakışan kullanıcı adları içindir; kodlama '$'ı
// atar. Kod biçimli adlar kaçışla da ayrılmıştır (2.3a). '$' bir harfe, rakama ya da '_'ye
// bitişik yazılamaz: '$' atılınca iki sözcük birleşirdi (a$b → ab, 1$e5 → 1e5).
func (l *Lexer) escapedName(line, col int) Token {
	if l.position > 0 && (isLetter(l.input[l.position-1]) || isDigit(l.input[l.position-1])) {
		l.addError(line, col, "'$' kaçışı bir adın ya da sayının hemen ardından yazılamaz; önüne bir boşluk ya da işleç koyun")
	}
	l.readChar() // '$'
	lit := l.readIdentifier()
	l.checkCodeForm(lit, line, col+1)
	return Token{Type: TOKEN_IDENT, Literal: lit, Line: line, Column: col}
}

// nulChar kaynağın ortasındaki NUL (U+0000) karakterini hata olarak bildirir. Sözcük
// çözümleyici girdinin sonunu 0 karakteriyle tanır; NUL eskiden dosyanın sonu sayılır, ardından
// gelen kod sessizce yok sayılırdı.
func (l *Lexer) nulChar(line, col int) Token {
	l.addError(line, col, "kaynakta NUL (U+0000) karakteri var; metin dosyalarında bulunmaz, dosya bozuk olabilir")
	l.readChar()
	return Token{Type: TOKEN_ILLEGAL, Literal: "\x00", Line: line, Column: col}
}

// checkCodeForm Türkçe yazılmış kaynakta kod biçimli bir adı (2.3a) hata olarak bildirir.
func (l *Lexer) checkCodeForm(lit string, line, col int) {
	if !codes.IsCodeForm(lit) {
		return
	}
	l.addError(line, col, "'%s' kod biçiminde bir ad (büyük harf ve üç rakam); bu biçimdeki adlar dilin kodları için ayrılmıştır, başka bir ad seçin (ör. %s_ ya da %s). Kodlu dosyalar ilk satırdaki '%s' başlığıyla başlar",
		lit, lit, strings.ToLower(lit), CodedHeader)
}

// CodedHeader is the first line of a coded file (format version 1). The header's rules live
// in internal/lang/source; the text lives here so the lexer's hint can show it too.
const CodedHeader = "//! tr kodlu 1"

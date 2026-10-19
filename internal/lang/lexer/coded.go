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
func NewBody(input string, kodlu bool) *Lexer {
	if !kodlu {
		return yeniCozumleyici(input)
	}
	on := yeniCozumleyici(input)
	on.kodlu = true
	var k []kayma
	on.kodTaramasi(&k)
	l := yeniCozumleyici(input)
	l.kodlu = true
	l.kaymalarKur(k)
	return l
}

// kayma, kökün girdisindeki off konumundan başlayarak satırın geri kalanının gösterimdeki
// sütun kaymasıdır (kodun adının uzunluğu - 4).
type kayma struct {
	off, d int32
}

// kodTaramasi bütün sözcükleri (yerleştirmelerdekiler dahil) okuyup kodların sütun
// kaymalarını toplar. Bulunan hatalar önemsenmez; asıl çözümleyici onları yeniden bulur.
func (l *Lexer) kodTaramasi(k *[]kayma) {
	for {
		tok := l.NextToken()
		switch {
		case tok.Type == TOKEN_EOF:
			return
		case tok.Code != codes.None:
			bas, son := l.LastRange()
			if d := utf8.RuneCountInString(tok.Literal) - (son - bas); d != 0 {
				*k = append(*k, kayma{off: l.kaynakKonumu(son), d: int32(d)})
			}
		case tok.Type == TOKEN_STRING:
			if yerler := l.Interpolations(tok); yerler != nil {
				deger := []rune(tok.Literal)
				for _, y := range yerler {
					if ifade, bas := InterpolationExpr(deger, y[0]+1, y[1]); ifade != "" {
						l.Interpolation(tok, ifade, bas).kodTaramasi(k)
					}
				}
			}
		}
	}
}

// kaymalarKur kaymaları konuma göre sıralar ve birikimli toplamlarını kurar. Satır sonundaki
// kaymalar atılır (satır bitmiştir).
func (l *Lexer) kaymalarKur(k []kayma) {
	sort.Slice(k, func(i, j int) bool { return k[i].off < k[j].off })
	l.kaymalar = k[:0]
	for _, x := range k {
		if int(x.off) >= len(l.input) || l.input[x.off] != '\n' {
			l.kaymalar = append(l.kaymalar, x)
		}
	}
	l.kaymaToplam = make([]int32, len(l.kaymalar)+1)
	for i, x := range l.kaymalar {
		l.kaymaToplam[i+1] = l.kaymaToplam[i] + x.d
	}
}

// kaymaUygula kök çözümleyici bir kayma noktasına geldiğinde sütunu kaydırır.
func (l *Lexer) kaymaUygula() {
	for l.kaymaSira < len(l.kaymalar) && int(l.kaymalar[l.kaymaSira].off) <= l.position {
		if int(l.kaymalar[l.kaymaSira].off) == l.position {
			l.column += int(l.kaymalar[l.kaymaSira].d)
		}
		l.kaymaSira++
	}
}

// satirKaymasi kökün girdisinde satır başı bas ile i arasındaki (i dahil) kaymaların toplamıdır.
func (l *Lexer) satirKaymasi(bas, i int) int {
	if len(l.kaymalar) == 0 {
		return 0
	}
	lo := sort.Search(len(l.kaymalar), func(j int) bool { return int(l.kaymalar[j].off) >= bas })
	hi := sort.Search(len(l.kaymalar), func(j int) bool { return int(l.kaymalar[j].off) > i })
	return int(l.kaymaToplam[hi] - l.kaymaToplam[lo])
}

// kodTurleri anahtar kelime kodu → token türü haritasıdır (turKodlari'nın tersi).
var kodTurleri = func() map[codes.Code]TokenType {
	m := make(map[codes.Code]TokenType, len(turKodlari))
	for t, k := range turKodlari {
		m[k] = t
	}
	return m
}()

// KeywordCode returns the code of a keyword token type, or codes.None for other types.
func KeywordCode(t TokenType) codes.Code {
	return turKodlari[t]
}

// adSozcugu okunan bir adın sözcük birimidir: Türkçe yazılmış kaynakta anahtar kelime ya da
// ad (kod biçimli ad hatadır, 2.3a), kodlu kaynakta kodluSozcuk.
func (l *Lexer) adSozcugu(lit string, line, col int) Token {
	if l.kodlu {
		return l.kodluSozcuk(lit, line, col)
	}
	tur := LookupIdent(lit)
	if tur == TOKEN_IDENT {
		l.kodBicimiDenetle(lit, line, col)
	}
	return Token{Type: tur, Literal: lit, Line: line, Column: col}
}

// kodluSozcuk kodlu kaynaktaki bir adı okur: kod biçimindeyse (G100) kodun etkin dildeki
// adını ve (anahtar kelimeyse) türünü taşıyan sözcük birimi, değilse kullanıcı adıdır.
// Ölü (taşınmış ya da kaldırılmış) kodlar eski adlarıyla okunur: o adın ipucu (13.6, 2.9a)
// kendiliğinden çalışır (uyumluluk sözü, 17.6).
func (l *Lexer) kodluSozcuk(lit string, line, col int) Token {
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
	if t, ok := kodTurleri[k]; ok && !g.Dead() {
		return Token{Type: t, Literal: g.Name, Line: line, Column: col, Code: k}
	}
	return Token{Type: TOKEN_IDENT, Literal: g.Name, Line: line, Column: col, Code: k}
}

// kacisliAd '$ad' kaçışını okur (2.11): '$' ardından gelen ad her zaman kullanıcının adıdır,
// dilin bir sözcüğüyle aynı olsa bile (değişken $eğer = 1). Kaçış, bir dosya başka bir yazım
// dilinde gösterilirken o dilin sözcükleriyle çakışan kullanıcı adları içindir; kodlama '$'ı
// atar. Kod biçimli adlar kaçışla da ayrılmıştır (2.3a). '$' bir harfe, rakama ya da '_'ye
// bitişik yazılamaz: '$' atılınca iki sözcük birleşirdi (a$b → ab, 1$e5 → 1e5).
func (l *Lexer) kacisliAd(line, col int) Token {
	if l.position > 0 && (isLetter(l.input[l.position-1]) || isDigit(l.input[l.position-1])) {
		l.addError(line, col, "'$' kaçışı bir adın ya da sayının hemen ardından yazılamaz; önüne bir boşluk ya da işleç koyun")
	}
	l.readChar() // '$'
	lit := l.readIdentifier()
	l.kodBicimiDenetle(lit, line, col+1)
	return Token{Type: TOKEN_IDENT, Literal: lit, Line: line, Column: col}
}

// nulKarakteri kaynağın ortasındaki NUL (U+0000) karakterini hata olarak bildirir. Sözcük
// çözümleyici girdinin sonunu 0 karakteriyle tanır; NUL eskiden dosyanın sonu sayılır, ardından
// gelen kod sessizce yok sayılırdı.
func (l *Lexer) nulKarakteri(line, col int) Token {
	l.addError(line, col, "kaynakta NUL (U+0000) karakteri var; metin dosyalarında bulunmaz, dosya bozuk olabilir")
	l.readChar()
	return Token{Type: TOKEN_ILLEGAL, Literal: "\x00", Line: line, Column: col}
}

// kodBicimiDenetle Türkçe yazılmış kaynakta kod biçimli bir adı (2.3a) hata olarak bildirir.
func (l *Lexer) kodBicimiDenetle(lit string, line, col int) {
	if !codes.IsCodeForm(lit) {
		return
	}
	l.addError(line, col, "'%s' kod biçiminde bir ad (büyük harf ve üç rakam); bu biçimdeki adlar dilin kodları için ayrılmıştır, başka bir ad seçin (ör. %s_ ya da %s). Kodlu dosyalar ilk satırdaki '%s' başlığıyla başlar",
		lit, lit, strings.ToLower(lit), CodedHeader)
}

// CodedHeader is the first line of a coded file (format version 1). The header's rules live
// in internal/lang/source; the text lives here so the lexer's hint can show it too.
const CodedHeader = "//! tr kodlu 1"

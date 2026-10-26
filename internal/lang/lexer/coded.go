package lexer

import (
	"sort"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kodlu biçim (docs/spec/17_kodlar.md, 17.1-17.2): dosya diskte dilden bağımsız kodlarla
// saklanır. Kodlu kaynakta dilin her sözcüğü kodla yazılır (TR_kw_if, TR_fn_print, .TR_mem_append, TR_arg_lr:); kod
// olmayan her ad kullanıcının adıdır ve hiçbir zaman anahtar kelime sayılmaz (başka bir
// dilde yazılmış bir kullanıcı adı TR'nin bir anahtar kelimesiyle aynı olabilir). Bir kod
// çözümleyicinin dilindeki adıyla gelir (codedToken). Program kimlik listesinde okunur (Active):
// kodlar kimlik kalır, ayrıştırıcı, denetleyici ve yorumlayıcı hiçbir yazım dilinin adını görmez;
// çalışma anına yük gelmez.

// NewBody returns the lexer for the body of a source file (the text left after
// internal/lang/source split off the BOM and the coded-form header); kodlu says the body is
// in coded form. internal/lang/source is the only place that recognizes the header and picks
// the lexer. Unlike New it does not strip a leading U+FEFF: every character of the body is
// part of the source.
//
// Kodlu gövdede konumlar diskteki metne göredir: kodlar kimlikleriyle okunur (Active), kodun adı
// diskteki yazımıdır, sütun kaymaz. Kişinin gördüğü görünümün konumlarını NewBodyView verir
// (source.view, 17.5).
func NewBody(input string, coded bool) *Lexer {
	l := newLexer(input)
	l.coded = coded
	l.checkBidi()
	return l
}

// NewBodyIn lang'de okunan gövdenin sözcük çözümleyicisidir: lang'de yazılmış kodsuz gövde ya da
// kodları lang'deki adlarıyla okunan kodlu gövde. Programlar kimliklerle okunur (codes.Active;
// NewBody); bir yazım dili yalnızca o dilde yazılmış metni çevirmek (tr kodla - --dil en, kabuk
// satırı) ya da dosyayı o dilde göstermek için okunur (17.7).
func NewBodyIn(input string, coded bool, lang *codes.Language) *Lexer {
	l := NewBody(input, coded)
	l.lang, l.words = lang, keywordsOf(lang)
	return l
}

// ViewEdit is one change the file's view makes to a coded body: the runes [Start, End) are
// shown as Len runes (a code as its name, a user name as '$name').
type ViewEdit struct {
	Start, End, Len int
}

// NewBodyView sütunları edits'ten kurulan görünüme göre olan kodlu gövdenin çözümleyicisidir
// (görünümü internal/lang/source kurar ve düzenlemelerini verir). Düzenlemeler görünümün kaçırdığı
// kullanıcı adlarının '$'ını da taşır; her konum kişinin gördüğü metinle birebir aynıdır.
func NewBodyView(input string, edits []ViewEdit) *Lexer {
	l := newLexer(input)
	l.coded = true
	k := make([]shift, 0, len(edits))
	for _, e := range edits {
		if d := e.Len - (e.End - e.Start); d != 0 {
			k = append(k, shift{off: int32(e.End), d: int32(d)})
		}
	}
	l.buildShifts(k)
	l.checkBidi()
	return l
}

// shift, kökün girdisindeki off konumundan başlayarak satırın geri kalanının gösterimdeki
// sütun kaymasıdır (görünümdeki uzunluk - diskteki uzunluk).
type shift struct {
	off, d int32
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
// ad (TR_ ile başlayan ad hatadır, 2.3a), kodlu kaynakta codedToken.
func (l *Lexer) nameToken(lit string, line, col int) Token {
	if isMessageCode(lit) {
		return Token{Type: TokMessageCode, Literal: lit, Line: line, Column: col}
	}
	if l.coded {
		return l.codedToken(lit, line, col)
	}
	kind := l.root.lookupIdent(lit)
	if kind == TokIdent {
		l.checkReservedName(lit, line, col)
	}
	return Token{Type: kind, Literal: lit, Line: line, Column: col}
}

// codedToken kodlu kaynaktaki bir adı okur: kimlikse (TR_fn_print) kodun çözümleyicinin
// dilindeki (Language) adını ve (anahtar kelimeyse) türünü taşıyan sözcük birimi, değilse kullanıcı
// adıdır. TR_ ile başlayıp kod listesinde olmayan ad bir kimlik değildir: sözdizimi hatası.
func (l *Lexer) codedToken(lit string, line, col int) Token {
	if !codes.IsReservedName(lit) {
		return Token{Type: TokIdent, Literal: lit, Line: line, Column: col}
	}
	k, err := codes.Parse(lit)
	g, var_ := l.Language().Entry(k)
	if err != nil || !var_ {
		l.addError(line, col, messages.New(messages.H0001, lit, codes.FnPrint))
		return Token{Type: TokIdent, Literal: lit, Line: line, Column: col}
	}
	if t, ok := codeTypes[k]; ok {
		return Token{Type: t, Literal: g.Name, Line: line, Column: col, Code: k}
	}
	return Token{Type: TokIdent, Literal: g.Name, Line: line, Column: col, Code: k}
}

// escapedName '$ad' kaçışını okur (2.11): '$' ardından gelen ad her zaman kullanıcının adıdır,
// dilin bir sözcüğüyle aynı olsa bile (değişken $eğer = 1). Kaçış, bir dosya başka bir yazım
// dilinde gösterilirken o dilin sözcükleriyle çakışan kullanıcı adları içindir; kodlama '$'ı
// atar. TR_ ile başlayan adlar kaçışla da ayrılmıştır (2.3a). '$' bir harfe, rakama ya da '_'ye
// bitişik yazılamaz: '$' atılınca iki sözcük birleşirdi (a$b → ab, 1$e5 → 1e5).
func (l *Lexer) escapedName(line, col int) Token {
	if l.position > 0 && (isLetter(l.input[l.position-1]) || isDigit(l.input[l.position-1])) {
		l.addError(line, col, messages.New(messages.H0002))
	}
	l.readChar() // '$'
	lit := l.readIdentifier()
	l.checkReservedName(lit, line, col+1)
	return Token{Type: TokIdent, Literal: lit, Line: line, Column: col}
}

// nulChar kaynağın ortasındaki NUL (U+0000) karakterini hata olarak bildirir. Sözcük
// çözümleyici girdinin sonunu 0 karakteriyle tanır; NUL eskiden dosyanın sonu sayılır, ardından
// gelen kod sessizce yok sayılırdı.
func (l *Lexer) nulChar(line, col int) Token {
	l.addError(line, col, messages.New(messages.H0003))
	l.readChar()
	return Token{Type: TokIllegal, Literal: "\x00", Line: line, Column: col}
}

// checkReservedName Türkçe yazılmış kaynakta TR_ ile başlayan bir adı (2.3a) hata olarak bildirir.
// Önek ASCII'dir; küçük harfli öneri bu yüzden bilerek Türkçe kuralla (turkish.Lower) değil
// strings.ToLower ile üretilir: TR_ için ır_ değil tr_ önerilir. Ad bilinen bir kimlikse kişi büyük
// olasılıkla başlıksız kodlu metin yapıştırmıştır: ipucu, okunan metnin dilindeki başlığı anar.
func (l *Lexer) checkReservedName(lit string, line, col int) {
	if !codes.IsReservedName(lit) {
		return
	}
	var hint *messages.Error
	if _, err := codes.Parse(lit); err == nil {
		hint = messages.New(messages.H0014, CodedHeader+l.Language().Code)
	}
	l.addError(line, col, messages.New(messages.H0004, lit, strings.ToLower(lit), hint))
}

// CodedHeader is the start of a coded file's first line (format version 3: the language's words
// are written by their identities, TR_fn_print): the code of the file's writing language follows
// it ("//! tr kodlu 3 dil=en", 17.1). The header's rules live in internal/lang/source; the text
// lives here so the lexer's hint can show it too.
const CodedHeader = "//! tr kodlu 3 dil="

// isMessageCode sözcüğün bir ileti kodu olduğudur: H ve tam dört rakam (H6903).
func isMessageCode(word string) bool {
	if len(word) != 5 || word[0] != 'H' {
		return false
	}
	for i := 1; i < 5; i++ {
		if word[i] < '0' || word[i] > '9' {
			return false
		}
	}
	return true
}

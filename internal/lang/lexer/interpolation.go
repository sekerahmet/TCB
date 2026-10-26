package lexer

import (
	"sort"
	"strings"
	"unicode"
)

// Metin içi yerleştirmelerin ('{ifade}', 2.7) konumları. Tırnaklı metnin değerinde kaçış
// dizileri çözülmüştür ("\n" tek karakterdir); bu yüzden ifadenin dosyadaki yeri değerden
// sayılarak bulunamaz. Sözcük çözümleyici yerleştirme içeren her metnin değerindeki her
// karakterin kaynaktaki konumunu saklar; ayrıştırıcı ifadeyi bir alt çözümleyiciyle okur ve
// alt çözümleyici her karakterin konumunu bu haritadan alır. Böylece yerleştirmedeki
// sözcüklerin satır, sütun ve kaynak aralıkları (Kayit) dosyadakiyle birebir aynıdır:
// hata mesajları doğru yeri gösterir, kodlu biçime çevirici (internal/lang/source) yalnızca o
// sözcüğün aralığını değiştirir.

// appendString metnin değerine bir karakter yazar ve karakterin kaynaktaki konumunu biriktirir
// (kaynak, bu çözümleyicinin girdisindeki indekstir).
func (l *Lexer) appendString(sb *strings.Builder, r rune, source int) {
	sb.WriteRune(r)
	l.strBuf = append(l.strBuf, l.sourcePos(source))
}

// stringInfo yerleştirme içeren bir metin sabitinin bilgisidir: değerdeki her karakterin
// kökün girdisindeki konumu ve yerleştirmelerin değerdeki aralıkları ('{' ve onu kapatan '}'
// indeksleri; kapanmayanda son -1). Aralıkları sözcük çözümleyici metni okurken çıkarır;
// ayrıştırıcı aynı aralıkları kullanır.
type stringInfo struct {
	srcPos    []int32
	positions [][2]int
}

// saveStringPositions okunan metnin bilgisini kökte, metnin satır ve sütunuyla saklar.
func (l *Lexer) saveStringPositions(lineNo, colNo int) {
	k := l.root
	if k.stringInfos == nil {
		k.stringInfos = map[[2]int]*stringInfo{}
	}
	k.stringInfos[[2]int{lineNo, colNo}] = &stringInfo{
		srcPos:    append([]int32(nil), l.strBuf...),
		positions: append([][2]int(nil), l.posBuf...),
	}
}

// stringMap metin sabitinin karakter konumlarıdır (yoksa nil).
func (l *Lexer) stringMap(text Token) []int32 {
	if b := l.root.stringInfos[[2]int{text.Line, text.Column}]; b != nil {
		return b.srcPos
	}
	return nil
}

// Interpolations returns the ranges of the interpolations in a string literal's value: the
// index of '{' and of the matching '}'; for an unclosed interpolation (a lexer error) the
// second is len(value).
func (l *Lexer) Interpolations(text Token) [][2]int {
	b := l.root.stringInfos[[2]int{text.Line, text.Column}]
	if b == nil {
		return nil
	}
	out := make([][2]int, len(b.positions))
	for i, y := range b.positions {
		if y[1] < 0 {
			y[1] = len(b.srcPos)
		}
		out[i] = y
	}
	return out
}

// sourcePos girdideki i. karakterin kökün girdisindeki indeksidir.
func (l *Lexer) sourcePos(i int) int32 {
	if l.posMap == nil {
		return int32(i)
	}
	if i < len(l.posMap) {
		return l.posMap[i]
	}
	if len(l.posMap) == 0 {
		return 0
	}
	return l.posMap[len(l.posMap)-1] + 1
}

// lineCol kökün girdisindeki bir indeksin satır ve sütunudur (1'den başlar; sütun karakter
// sayar). Satır başları ilk istendiğinde bir kez çıkarılır.
func (l *Lexer) lineCol(i int) (int, int) {
	if l.lineStarts == nil {
		l.lineStarts = []int{0}
		for j, r := range l.input {
			if r == '\n' {
				l.lineStarts = append(l.lineStarts, j+1)
			}
		}
	}
	s := sort.SearchInts(l.lineStarts, i+1) - 1
	return s + 1, i - l.lineStarts[s] + 1 + l.lineShift(l.lineStarts[s], i)
}

// Interpolation returns the sub-lexer for one '{expr}' part of a string literal. metin is the
// literal's token, ifade the part's text, and bas the index of the expression's first
// character in the string's value. The sub-lexer shares the root's coded mode, positions and
// record.
func (l *Lexer) Interpolation(text Token, expr string, begin int) *Lexer {
	src := []rune(expr)
	posMap := l.stringMap(text)
	sub := &Lexer{input: src, coded: l.coded, root: l.root}
	if posMap != nil && begin >= 0 && begin+len(src) <= len(posMap) {
		sub.posMap = posMap[begin : begin+len(src) : begin+len(src)]
	} else {
		// Haritası olmayan metin (başka bir çözümleyicinin sözcüğü): konum metnin başı sayılır.
		sub.line, sub.column = text.Line, text.Column
	}
	sub.readChar()
	return sub
}

// InterpolationExpr returns the expression of the interpolation in value[bas:son] (leading
// and trailing spaces trimmed) and the index of its first character in the value.
func InterpolationExpr(value []rune, begin, end int) (string, int) {
	rawStr := string(value[begin:end])
	expr := strings.TrimSpace(rawStr)
	return expr, begin + len([]rune(rawStr)) - len([]rune(strings.TrimLeftFunc(rawStr, unicode.IsSpace)))
}

// StringPos returns the line and column in the file of the i-th character of a string
// literal's value; without a position map it returns the literal's own position.
func (l *Lexer) StringPos(text Token, i int) (int, int) {
	posMap := l.stringMap(text)
	if i < 0 || i >= len(posMap) {
		return text.Line, text.Column
	}
	return l.root.lineCol(int(posMap[i]))
}

// Record is a token and its character range [Start, End) in the root's input.
type Record struct {
	Token
	Start, End int
}

// StartRecording turns on recording of every token produced from now on (those inside
// interpolations included) with its source range; read them with Records. The coded-form
// converter uses it; it is off during ordinary runs.
func (l *Lexer) StartRecording() {
	if l.root.rec == nil {
		l.root.rec = new([]Record)
	}
}

// Records returns the recorded tokens (in the order produced; the tokens of an
// interpolation are added when the parser reads it).
func (l *Lexer) Records() []Record {
	if l.root.rec == nil {
		return nil
	}
	return *l.root.rec
}

func (l *Lexer) record(tok Token) {
	begin, end := l.LastRange()
	kb := int(l.sourcePos(begin))
	ks := kb
	if end > begin {
		ks = int(l.sourcePos(end-1)) + 1
	}
	*l.root.rec = append(*l.root.rec, Record{Token: tok, Start: kb, End: ks})
}

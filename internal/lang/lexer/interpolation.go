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

// metinYaz metnin değerine bir karakter yazar ve karakterin kaynaktaki konumunu biriktirir
// (kaynak, bu çözümleyicinin girdisindeki indekstir).
func (l *Lexer) metinYaz(sb *strings.Builder, r rune, kaynak int) {
	sb.WriteRune(r)
	l.metinTampon = append(l.metinTampon, l.kaynakKonumu(kaynak))
}

// metinBilgisi yerleştirme içeren bir metin sabitinin bilgisidir: değerdeki her karakterin
// kökün girdisindeki konumu ve yerleştirmelerin değerdeki aralıkları ('{' ve onu kapatan '}'
// indeksleri; kapanmayanda son -1). Aralıkları sözcük çözümleyici metni okurken çıkarır;
// ayrıştırıcı ve kodlu biçimin ön taraması aynı aralıkları kullanır.
type metinBilgisi struct {
	konum  []int32
	yerler [][2]int
}

// metinKonumlariniSakla okunan metnin bilgisini kökte, metnin satır ve sütunuyla saklar.
func (l *Lexer) metinKonumlariniSakla(satir, sutun int) {
	k := l.kok
	if k.metinler == nil {
		k.metinler = map[[2]int]*metinBilgisi{}
	}
	k.metinler[[2]int{satir, sutun}] = &metinBilgisi{
		konum:  append([]int32(nil), l.metinTampon...),
		yerler: append([][2]int(nil), l.yerTampon...),
	}
}

// metinHaritasi metin sabitinin karakter konumlarıdır (yoksa nil).
func (l *Lexer) metinHaritasi(metin Token) []int32 {
	if b := l.kok.metinler[[2]int{metin.Line, metin.Column}]; b != nil {
		return b.konum
	}
	return nil
}

// Interpolations returns the ranges of the interpolations in a string literal's value: the
// index of '{' and of the matching '}'; for an unclosed interpolation (a lexer error) the
// second is len(value).
func (l *Lexer) Interpolations(metin Token) [][2]int {
	b := l.kok.metinler[[2]int{metin.Line, metin.Column}]
	if b == nil {
		return nil
	}
	out := make([][2]int, len(b.yerler))
	for i, y := range b.yerler {
		if y[1] < 0 {
			y[1] = len(b.konum)
		}
		out[i] = y
	}
	return out
}

// kaynakKonumu girdideki i. karakterin kökün girdisindeki indeksidir.
func (l *Lexer) kaynakKonumu(i int) int32 {
	if l.harita == nil {
		return int32(i)
	}
	if i < len(l.harita) {
		return l.harita[i]
	}
	if len(l.harita) == 0 {
		return 0
	}
	return l.harita[len(l.harita)-1] + 1
}

// satirSutun kökün girdisindeki bir indeksin satır ve sütunudur (1'den başlar; sütun karakter
// sayar). Satır başları ilk istendiğinde bir kez çıkarılır.
func (l *Lexer) satirSutun(i int) (int, int) {
	if l.satirBaslari == nil {
		l.satirBaslari = []int{0}
		for j, r := range l.input {
			if r == '\n' {
				l.satirBaslari = append(l.satirBaslari, j+1)
			}
		}
	}
	s := sort.SearchInts(l.satirBaslari, i+1) - 1
	return s + 1, i - l.satirBaslari[s] + 1 + l.satirKaymasi(l.satirBaslari[s], i)
}

// Interpolation returns the sub-lexer for one '{expr}' part of a string literal. metin is the
// literal's token, ifade the part's text, and bas the index of the expression's first
// character in the string's value. The sub-lexer shares the root's coded mode, positions and
// record.
func (l *Lexer) Interpolation(metin Token, ifade string, bas int) *Lexer {
	girdi := []rune(ifade)
	harita := l.metinHaritasi(metin)
	alt := &Lexer{input: girdi, kodlu: l.kodlu, kok: l.kok}
	if harita != nil && bas >= 0 && bas+len(girdi) <= len(harita) {
		alt.harita = harita[bas : bas+len(girdi) : bas+len(girdi)]
	} else {
		// Haritası olmayan metin (başka bir çözümleyicinin sözcüğü): konum metnin başı sayılır.
		alt.line, alt.column = metin.Line, metin.Column
	}
	alt.readChar()
	return alt
}

// InterpolationExpr returns the expression of the interpolation in value[bas:son] (leading
// and trailing spaces trimmed) and the index of its first character in the value.
func InterpolationExpr(deger []rune, bas, son int) (string, int) {
	ham := string(deger[bas:son])
	ifade := strings.TrimSpace(ham)
	return ifade, bas + len([]rune(ham)) - len([]rune(strings.TrimLeftFunc(ham, unicode.IsSpace)))
}

// StringPos returns the line and column in the file of the i-th character of a string
// literal's value; without a position map it returns the literal's own position.
func (l *Lexer) StringPos(metin Token, i int) (int, int) {
	harita := l.metinHaritasi(metin)
	if i < 0 || i >= len(harita) {
		return metin.Line, metin.Column
	}
	return l.kok.satirSutun(int(harita[i]))
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
	if l.kok.kayit == nil {
		l.kok.kayit = new([]Record)
	}
}

// Records returns the recorded tokens (in the order produced; the tokens of an
// interpolation are added when the parser reads it).
func (l *Lexer) Records() []Record {
	if l.kok.kayit == nil {
		return nil
	}
	return *l.kok.kayit
}

func (l *Lexer) kaydet(tok Token) {
	bas, son := l.LastRange()
	kb := int(l.kaynakKonumu(bas))
	ks := kb
	if son > bas {
		ks = int(l.kaynakKonumu(son-1)) + 1
	}
	*l.kok.kayit = append(*l.kok.kayit, Record{Token: tok, Start: kb, End: ks})
}

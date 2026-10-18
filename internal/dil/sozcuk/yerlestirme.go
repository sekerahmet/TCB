package sozcuk

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
// hata mesajları doğru yeri gösterir, kodlu biçime çevirici (internal/dil/kaynak) yalnızca o
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

// Yerlestirmeler metin sabitindeki yerleştirmelerin değerdeki aralıklarıdır: '{' indeksi ve
// onu kapatan '}' indeksi; kapanmayan yerleştirmede (sözcük çözümleyici hatası) ikincisi
// len(değer)'dir.
func (l *Lexer) Yerlestirmeler(metin Token) [][2]int {
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

// Yerlestirme metin sabitindeki bir '{ifade}' bölümünün alt sözcük çözümleyicisidir. metin
// sabitin sözcük birimi, ifade bölümün metni, bas da ifadenin metnin değerindeki ilk
// karakterinin indeksidir. Alt çözümleyici kökün kodlu kipini, konumlarını ve kaydını
// paylaşır.
func (l *Lexer) Yerlestirme(metin Token, ifade string, bas int) *Lexer {
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

// YerlestirmeIfadesi değerin [bas, son) bölümündeki yerleştirmenin ifadesidir (baştaki ve
// sondaki boşluklar kırpılmış) ve ifadenin değerdeki ilk karakterinin indeksi.
func YerlestirmeIfadesi(deger []rune, bas, son int) (string, int) {
	ham := string(deger[bas:son])
	ifade := strings.TrimSpace(ham)
	return ifade, bas + len([]rune(ham)) - len([]rune(strings.TrimLeftFunc(ham, unicode.IsSpace)))
}

// MetinKonumu metin sabitinin değerindeki i. karakterin dosyadaki satır ve sütunudur;
// metnin haritası yoksa metnin kendi konumu döner.
func (l *Lexer) MetinKonumu(metin Token, i int) (int, int) {
	harita := l.metinHaritasi(metin)
	if i < 0 || i >= len(harita) {
		return metin.Line, metin.Column
	}
	return l.kok.satirSutun(int(harita[i]))
}

// Kayit bir sözcük birimi ve kökün girdisindeki karakter aralığıdır: [Bas, Son).
type Kayit struct {
	Token
	Bas, Son int
}

// Kaydet bu andan sonra üretilen bütün sözcük birimlerinin (yerleştirmelerdekiler dahil)
// kaynak aralıklarıyla kaydedilmesini açar; kayıtlar Kayitlar ile okunur. Kodlu biçime
// çevirici kullanır; sıradan çalıştırmada kapalıdır.
func (l *Lexer) Kaydet() {
	if l.kok.kayit == nil {
		l.kok.kayit = new([]Kayit)
	}
}

// Kayitlar kaydedilen sözcük birimleridir (üretildikleri sırayla; bir yerleştirmenin
// sözcükleri, ayrıştırıcı onu okuduğu anda eklenir).
func (l *Lexer) Kayitlar() []Kayit {
	if l.kok.kayit == nil {
		return nil
	}
	return *l.kok.kayit
}

func (l *Lexer) kaydet(tok Token) {
	bas, son := l.SonAralik()
	kb := int(l.kaynakKonumu(bas))
	ks := kb
	if son > bas {
		ks = int(l.kaynakKonumu(son-1)) + 1
	}
	*l.kok.kayit = append(*l.kok.kayit, Kayit{Token: tok, Bas: kb, Son: ks})
}

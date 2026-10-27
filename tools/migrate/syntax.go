package main

import (
	"strings"
	"unicode"

	"github.com/sekerahmet/TCB/internal/lang/codes"
)

// Eski yazımın sözcük düzeyindeki çevirisi (dil tanımı 2. bölümünden önceki biçim → yeni biçim).
// Kaynak bir sözcük okuyucusuyla okunur, düzenli ifadeyle değil: yorum, metin, ham metin ve
// yerleştirme sınırları bugünkü sözcük çözümleyicinin kurallarıyla (internal/lang/lexer, 84ee01b8)
// bulunur; yalnız değişmesi gereken aralıklar yazılır, öbür her karakter olduğu gibi kalır.
//
//   - '//' yorumu '#', '/* */' her satırı '#' ile başlayan yorum (satır içindeki blok yorum çevrilmez);
//   - sınıflı kimlik TR_<sınıf>_<ad> → TR_<ad>; TR_kw_while → TR_for; TR_kw_default → _;
//   - gösterimde (words != nil) kalkan anahtar kelimelerin o dildeki yazımı aynı biçimde;
//   - çıplak ileti kodu H6903 → "H6903" (eski 2.12 kalktı);
//   - bloğun içindeki sabitin adı ve o bloktaki kullanımları küçük harfe (2.3b);
//   - metinler strings.go'da.

// view bir gösterim dilinin çeviride gereken sözcükleridir; kodlu metinde nil.
type view struct {
	lang *codes.Language
	// removed kalkan anahtar kelimenin bu dildeki yazımı → yerine yazılacak (removed.json).
	removed map[string]string
	// constWord sabit anahtar kelimesinin bu dildeki yazımıdır.
	constWord string
}

// problem çevirinin çeviremediği bir yerdir (satır, kaynak metne göre).
type problem struct {
	Line   int
	Reason string
}

// converter bir kod metnini (dosyanın gövdesi, yerleştirmenin ifadesi ya da Go testindeki parça)
// çevirir.
type converter struct {
	src      []rune
	i        int
	line     int // 1'den, src'nin başına göre; lineBase eklenerek bildirilir
	lineBase int
	out      strings.Builder
	v        *view
	problems []problem

	afterDot bool   // son sözcük birimi '.' ya da '?.'
	lastWord string // son sözcük birimi bir adsa o (değilse boş)
	depth    int    // açık '{' sayısı
	renames  []localConst
	// constNames bloğun içindeki sabit tanımının ad listesi okunurken açıktır (TR_kw_const a, b = ...).
	constNames bool
	// words metinde geçen bütün sözcüklerdir: küçük harfe çevrilen sabitin yeni adı başka bir adla
	// çakışıyorsa çevrilmez (anlam değişirdi).
	words map[string]bool
}

// localConst bloğun içinde tanımlanmış, adı küçük harfe çevrilen sabittir: depth bloğun düzeyi
// (blok kapanınca ad eski hâline döner).
type localConst struct {
	from, to string
	depth    int
}

// convertCode kod metnini çevirir; lineBase metnin ilk satırının dosyadaki satırından bir eksiktir.
func convertCode(src string, v *view, lineBase int) (string, []problem) {
	c := &converter{src: []rune(src), line: 1, lineBase: lineBase, v: v, words: wordsOf(src)}
	c.run()
	return c.out.String(), c.problems
}

// wordsOf metnin bütün sözcükleridir (metin ve yorum ayrımı yapılmaz: fazladan sözcük yalnız bir
// sabitin çevrilmesini önler, güvenli yandadır).
func wordsOf(src string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(src, func(r rune) bool { return !isLetter(r) && !isDigit(r) }) {
		out[w] = true
	}
	return out
}

func (c *converter) peek(n int) rune {
	if c.i+n < len(c.src) {
		return c.src[c.i+n]
	}
	return 0
}

func (c *converter) fail(reason string) {
	c.problems = append(c.problems, problem{Line: c.lineBase + c.line, Reason: reason})
}

// copyRune bir karakteri olduğu gibi yazar.
func (c *converter) copyRune() {
	r := c.src[c.i]
	if r == '\n' {
		c.line++
	}
	c.out.WriteRune(r)
	c.i++
}

func (c *converter) run() {
	for c.i < len(c.src) {
		r := c.src[c.i]
		switch {
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			c.copyRune()
		case r == '/' && c.peek(1) == '/':
			c.out.WriteString("#")
			c.i += 2
			c.copyLine()
		case r == '#':
			// Eskide hata, satırın geri kalanı atlanırdı; yeni yazımda yorum: dokunulmaz.
			c.copyLine()
		case r == '/' && c.peek(1) == '*':
			c.blockComment()
		case r == '"' || r == '\'':
			c.str(r)
			c.token("")
		case r == '`':
			c.rawString()
			c.token("")
		case isLetter(r):
			c.word()
		case isDigit(r):
			for c.i < len(c.src) && (isDigit(c.src[c.i]) || isLetter(c.src[c.i]) || c.src[c.i] == '.' && isDigit(c.peek(1))) {
				c.copyRune()
			}
			c.token("")
		default:
			c.punct(r)
		}
	}
}

// token son sözcük birimini kaydeder (ad değilse word boştur).
func (c *converter) token(word string) {
	c.afterDot, c.lastWord = false, word
}

func (c *converter) punct(r rune) {
	if r != ',' {
		c.constNames = false
	}
	dot := r == '.' && c.peek(1) != '.' || r == '?' && c.peek(1) == '.'
	if r == '?' && dot {
		c.copyRune()
	}
	switch r {
	case '{':
		c.depth++
	case '}':
		for len(c.renames) > 0 && c.renames[len(c.renames)-1].depth >= c.depth {
			c.renames = c.renames[:len(c.renames)-1]
		}
		c.depth--
	}
	if r == '.' && c.peek(1) == '.' {
		c.copyRune() // aralık: '..' iki karakter
	}
	c.copyRune()
	c.token("")
	c.afterDot = dot
}

// copyLine satır sonuna kadar (satır sonu hariç) yazar.
func (c *converter) copyLine() {
	for c.i < len(c.src) && c.src[c.i] != '\n' {
		c.copyRune()
	}
}

// blockComment '/* ... */' yorumunu her satırı '#' ile başlayan yoruma çevirir. Yorumdan önce ya
// da sonra aynı satırda kod varsa çevrilmez (yeni yazımda satır içi yorum yoktur).
func (c *converter) blockComment() {
	end := strings.Index(string(c.src[c.i:]), "*/")
	if end < 0 {
		c.fail("closing '*/' not found")
		for c.i < len(c.src) {
			c.copyRune()
		}
		return
	}
	body := []rune(string(c.src[c.i:])[:end])[2:]
	after := c.i + len([]rune(string(c.src[c.i:])[:end])) + 2
	lineStart := strings.LastIndexByte(c.out.String(), '\n') + 1
	indent := c.out.String()[lineStart:]
	rest := after
	for rest < len(c.src) && (c.src[rest] == ' ' || c.src[rest] == '\t' || c.src[rest] == '\r') {
		rest++
	}
	if strings.TrimSpace(indent) != "" || rest < len(c.src) && c.src[rest] != '\n' {
		c.fail("block comment shares its line with code")
		for c.i < after {
			c.copyRune()
		}
		return
	}
	lines := strings.Split(string(body), "\n")
	for k, l := range lines {
		l = strings.TrimRight(l, "\r")
		if k > 0 {
			c.out.WriteString("\n" + indent)
			c.line++
			l = strings.TrimLeft(l, " \t")
			l = strings.TrimPrefix(l, "* ")
			l = strings.TrimPrefix(l, "*")
		}
		if t := strings.TrimSpace(l); t == "" && k == len(lines)-1 && k > 0 {
			// '*/' kendi satırında: o satır yorumdan kalkar.
			s := c.out.String()
			c.out.Reset()
			c.out.WriteString(strings.TrimRight(s[:strings.LastIndexByte(s, '\n')], " \t"))
			c.line--
			continue
		}
		c.out.WriteString("#")
		if l != "" && !strings.HasPrefix(l, " ") {
			c.out.WriteString(" ")
		}
		c.out.WriteString(strings.TrimRight(l, " \t"))
	}
	c.i = after
	for c.i < rest {
		c.i++ // '*/'dan sonra satır sonuna kadar boşluk
	}
}

func (c *converter) rawString() {
	c.copyRune()
	for c.i < len(c.src) && c.src[c.i] != '`' {
		c.copyRune()
	}
	if c.i < len(c.src) {
		c.copyRune()
	}
}

// word bir adı okur ve çevirir.
func (c *converter) word() {
	start := c.i
	for c.i < len(c.src) && (isLetter(c.src[c.i]) || isDigit(c.src[c.i])) {
		c.i++
	}
	w := string(c.src[start:c.i])
	afterDot := c.afterDot
	named := c.namedArgAhead()
	out := w
	switch {
	case afterDot:
		out = c.identity(w)
	case isMessageCode(w):
		out = `"` + w + `"`
	case codes.IdentityClass(w) != "":
		out = c.identity(w)
	case c.v != nil && !named && c.v.removed[w] != "":
		out = c.v.removed[w]
	default:
		out = c.renamed(w)
	}
	isConst := w == "TR_kw_const" || c.v != nil && w == c.v.constWord
	if c.constNames && !afterDot {
		out = c.localConst(w, out)
	}
	c.out.WriteString(out)
	c.token(w)
	if isConst {
		c.constNames = c.depth > 0 // ad listesi '=' ya da ':' gelene kadar sürer (punct)
	}
}

// localConst bloğun içinde tanımlanan sabitin adını küçük harfe çevirir (2.3b: büyük harf yalnız
// dosyanın en dışındaki sabitte) ve bloğun kalanında kullanımlarını da çevirtir. Çevrilmez: kuralın
// matematik muafiyetine giren ad (W, QKV, W1: büyük parça en çok üç harf), yeni adı metinde başka
// bir ad olarak geçen ad (iki ad birleşip anlam değişirdi) ve I ya da İ içeren ad (küçük harfi dile
// bağlıdır); son ikisi çeviremedi listesine yazılır.
func (c *converter) localConst(w, out string) string {
	to := strings.ToLower(w)
	switch {
	case to == w || mathName(w):
		return out
	case strings.ContainsAny(w, "Iİ"):
		c.fail("local constant " + w + " has I or İ; lower it by hand")
		return out
	case c.words[to]:
		c.fail("local constant " + w + ": its lower-case name " + to + " is already a name here")
		return out
	}
	c.renames = append(c.renames, localConst{from: w, to: to, depth: c.depth})
	return to
}

// mathName adın 2.3b'nin matematik muafiyetine girdiğidir: '_' ile ayrılan her büyük harfli parça en
// çok üç harftir (W, Wq, QKV, W1, X_veri). Baştaki '_' sayılmaz.
func mathName(w string) bool {
	for _, part := range strings.Split(strings.TrimLeft(w, "_"), "_") {
		letters := 0
		upper := false
		for _, r := range part {
			if unicode.IsLetter(r) {
				letters++
			}
			upper = upper || unicode.IsUpper(r)
		}
		if upper && letters > 3 {
			return false
		}
	}
	return true
}

// renamed bloktaki sabitin yeni adıdır (yoksa ad kendisi).
func (c *converter) renamed(w string) string {
	if c.namedArgAhead() {
		return w
	}
	for k := len(c.renames) - 1; k >= 0; k-- {
		if c.renames[k].from == w {
			return c.renames[k].to
		}
	}
	return w
}

// namedArgAhead adın ardından (boşluktan sonra) tek ':' geliyor mu (adlı argüman ya da sözlük anahtarı).
func (c *converter) namedArgAhead() bool {
	j := c.i
	for j < len(c.src) && (c.src[j] == ' ' || c.src[j] == '\t') {
		j++
	}
	return j < len(c.src) && c.src[j] == ':' && (j+1 >= len(c.src) || c.src[j+1] != '=')
}

// identity sınıflı kimliği yeni yazımına çevirir; kimlik değilse olduğu gibi bırakır.
func (c *converter) identity(w string) string {
	if to, ok := removedIdentities[w]; ok {
		return to
	}
	cls := codes.IdentityClass(w)
	if cls == "" {
		return w
	}
	if _, err := codes.Parse(w); err != nil {
		c.fail("unknown identity " + w)
		return w
	}
	return codes.IdentityPrefix + strings.TrimPrefix(w, codes.IdentityPrefix+string(cls)+"_")
}

// removedIdentities sözlükten kalkan kimliklerin yeni yazımdaki karşılığıdır (2.9).
var removedIdentities = map[string]string{
	"TR_kw_while":   codes.IdentityPrefix + "for",
	"TR_kw_default": "_",
}

// isMessageCode sözcüğün çıplak ileti kodu olduğudur: H ve tam dört rakam (eski 2.12).
func isMessageCode(w string) bool {
	if len(w) != 5 || w[0] != 'H' {
		return false
	}
	for _, r := range w[1:] {
		if !isDigit(r) {
			return false
		}
	}
	return true
}

func isLetter(r rune) bool { return unicode.IsLetter(r) || r == '_' }
func isDigit(r rune) bool  { return '0' <= r && r <= '9' }

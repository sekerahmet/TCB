package tokenize

import (
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Geri izlemeli düzenli ifade motoru
// ============================================================================
//
// Parçalayıcı dosyalarındaki desenler, ileriye/geriye bakma (lookaround),
// sahiplenici niceleyiciler ve satır çapaları içerebilir; standart
// kütüphanenin doğrusal zamanlı motoru bunları desteklemez ve \s, \w, \d
// sınıflarını yalnızca ASCII olarak yorumlar. Bu motor, dosyaları üreten
// başvuru uygulamasının kullandığı sözdizimi ve anlamı izler:
//   - Seçenekler soldan sağa denenir, ilk tutan alınır (geri izlemeli).
//   - \s: Unicode White_Space; \d: Nd; \w: L + M + N + Pc;
//     \h: onaltılık rakam; \p{..}/\P{..}: genel kategori, yazı sistemi ya da özellik.
//   - "." satır sonu (\n) dışında her karakter; ^ ve $ satır başı/sonu,
//     \A, \z, \Z metin başı/sonu; \b, \B sözcük sınırı.
//   - Gruplar: (...), (?:...), (?<ad>...), (?=..), (?!..), (?<=..), (?<!..),
//     (?>..) atomik; satır içi bayraklar (?i), (?m), (?i:...), (?-i:...).
//   - Niceleyiciler: * + ? {n} {n,} {n,m} {,m}; tembel (?) ve sahiplenici (+).
// Geri başvurular (\1) ve koşullu yapılar desteklenmez; açık hata verilir.

type reNode interface{}

type (
	reChar struct {
		r    rune
		fold []rune // büyük/küçük harf duyarsızsa eşdeğer karakterler
	}
	reClass struct {
		f func(rune) bool
	}
	reDot    struct{ includeNewline bool }
	reUnion  struct{ items []reNode }
	reOption struct{ options []reNode }
	reRepeat struct {
		sub        reNode
		en, parent int // ust < 0: sınırsız
		lazy       bool
		claimer    bool
	}
	reLook struct {
		sub     reNode
		back    bool
		negated bool
		length  int // geriye bakmada en çok karakter sayısı
	}
	reAtomic struct{ sub reNode }
	reAnchor struct{ kind byte } // ^ $ A z Z b B
)

// Regex derlenmiş bir desendir.
type Regex struct {
	source   string
	root     reNode
	prog     *reProgram
	slot     int         // döngü yuvası sayısı
	first    *reFirstSet // eşleşmenin ilk karakteri (nil: süzgeç yok)
	warnings []string
}

// reMaxDepth desenin iç içe derinlik sınırıdır (gruplar, sınıflar ve
// üst üste niceleyiciler). Derleme ve çözümleme özyinelemeli olduğundan
// sınırsız derinlik yığını taşırabilirdi.
const reMaxDepth = 1000

// CompileRegex deseni derler; desteklenmeyen bir yapı görülürse hata döner.
func CompileRegex(pattern string) (*Regex, error) {
	a := &reParser{s: []rune(pattern)}
	root, err := a.option()
	if err != nil {
		return nil, messages.New(messages.H5245, reShorten(pattern), err)
	}
	if a.i < len(a.s) {
		return nil, messages.New(messages.H5246, reShorten(pattern), a.i)
	}
	if reTreeDepth(root) > 2*reMaxDepth {
		return nil, messages.New(messages.H5247, reShorten(pattern), reMaxDepth)
	}
	failure, warnings := reRiskAnalyze(root)
	if failure != "" && !CurrentRegexSettings().AllowRiskyPatterns {
		return nil, messages.New(messages.H5248, reShorten(pattern), failure)
	}
	if failure != "" {
		warnings = append([]string{failure}, warnings...)
	}
	c := &reCompiler{}
	prog := c.program(root)
	return &Regex{source: pattern, root: root, prog: prog, slot: c.slot, first: reBuildFirstSet(root), warnings: warnings}, nil
}

// reShorten hata iletileri için çok uzun desenleri kısaltır.
func reShorten(pattern string) string {
	if len(pattern) <= 200 {
		return pattern
	}
	i := 200
	for i > 0 && !utf8.RuneStart(pattern[i]) {
		i--
	}
	return pattern[:i] + "…"
}

// reTreeDepth desen ağacının derinliğini özyinelemesiz hesaplar.
func reTreeDepth(root reNode) int {
	type item struct {
		d reNode
		n int
	}
	stack := []item{{root, 1}}
	maxN := 0
	for len(stack) > 0 {
		o := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if o.n > maxN {
			maxN = o.n
		}
		switch x := o.d.(type) {
		case *reUnion:
			for _, c := range x.items {
				stack = append(stack, item{c, o.n + 1})
			}
		case *reOption:
			for _, c := range x.options {
				stack = append(stack, item{c, o.n + 1})
			}
		case *reRepeat:
			stack = append(stack, item{x.sub, o.n + 1})
		case *reLook:
			stack = append(stack, item{x.sub, o.n + 1})
		case *reAtomic:
			stack = append(stack, item{x.sub, o.n + 1})
		}
	}
	return maxN
}

func (d *Regex) String() string { return d.source }

// ---------------------------------------------------------------------------
// Ayrıştırıcı
// ---------------------------------------------------------------------------

type reParser struct {
	s           []rune
	i           int
	insensitive bool
	dotNewline  bool
	depth       int // iç içe grup ve sınıf derinliği
}

// enter bir iç içe yapıya girer; derinlik sınırı aşılırsa hata verir.
func (a *reParser) enter() error {
	a.depth++
	if a.depth > reMaxDepth {
		return messages.New(messages.H5249, reMaxDepth)
	}
	return nil
}

func (a *reParser) last() bool { return a.i >= len(a.s) }
func (a *reParser) look() rune {
	if a.last() {
		return 0
	}
	return a.s[a.i]
}
func (a *reParser) ahead(p string) bool {
	r := []rune(p)
	if a.i+len(r) > len(a.s) {
		return false
	}
	for k, c := range r {
		if a.s[a.i+k] != c {
			return false
		}
	}
	return true
}

func (a *reParser) option() (reNode, error) {
	var options []reNode
	for {
		d, err := a.arr()
		if err != nil {
			return nil, err
		}
		options = append(options, d)
		if !a.last() && a.look() == '|' {
			a.i++
			continue
		}
		break
	}
	if len(options) == 1 {
		return options[0], nil
	}
	return &reOption{options}, nil
}

func (a *reParser) arr() (reNode, error) {
	var items []reNode
	for !a.last() && a.look() != '|' && a.look() != ')' {
		d, err := a.quantified()
		if err != nil {
			return nil, err
		}
		if d != nil {
			items = append(items, d)
		}
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return &reUnion{items}, nil
}

func (a *reParser) quantified() (reNode, error) {
	d, err := a.item()
	if err != nil || d == nil {
		return d, err
	}
	for accumulated := 0; !a.last(); accumulated++ {
		if accumulated >= reMaxDepth {
			return nil, messages.New(messages.H5250, reMaxDepth)
		}
		en, parent := 0, 0
		begin := a.i
		switch a.look() {
		case '*':
			en, parent = 0, -1
			a.i++
		case '+':
			en, parent = 1, -1
			a.i++
		case '?':
			en, parent = 0, 1
			a.i++
		case '{':
			e, u, ok := a.span()
			if !ok {
				return d, nil // geçersiz aralık sözdizimi: '{' düz karakterdir
			}
			en, parent = e, u
		default:
			return d, nil
		}
		t := &reRepeat{sub: d, en: en, parent: parent}
		if !a.last() && a.look() == '?' {
			t.lazy = true
			a.i++
		} else if !a.last() && a.look() == '+' && a.s[begin] != '{' {
			t.claimer = true
			a.i++
		}
		if _, ok := d.(*reAnchor); ok {
			return nil, messages.New(messages.H5251)
		}
		d = t
	}
	return d, nil
}

// span "{n}", "{n,}", "{n,m}", "{,m}" sözdizimini okur.
func (a *reParser) span() (int, int, bool) {
	j := a.i + 1
	num := func() (int, bool) {
		k := j
		for j < len(a.s) && a.s[j] >= '0' && a.s[j] <= '9' {
			j++
		}
		if k == j {
			return 0, false
		}
		n, err := strconv.Atoi(string(a.s[k:j]))
		return n, err == nil
	}
	en, var1 := num()
	parent := en
	if j < len(a.s) && a.s[j] == ',' {
		j++
		u, var2 := num()
		if !var1 && !var2 {
			return 0, 0, false
		}
		if var2 {
			parent = u
		} else {
			parent = -1
		}
		if !var1 {
			en = 0
		}
	} else if !var1 {
		return 0, 0, false
	}
	if j >= len(a.s) || a.s[j] != '}' {
		return 0, 0, false
	}
	if parent >= 0 && parent < en {
		return 0, 0, false
	}
	a.i = j + 1
	return en, parent, true
}

func (a *reParser) charNode(r rune) reNode {
	k := &reChar{r: r}
	if a.insensitive {
		k.fold = foldOrbit(r)
	}
	return k
}

// foldOrbit büyük/küçük harf duyarsız eşleşmede r'nin eşdeğerleridir
// (başvuru uygulamasından çıkarılmış tablo; bkz. ozelliktablo.go).
func foldOrbit(r rune) []rune { return exprFold[r] }

// classFold köşeli parantezli bir sınıfı büyük/küçük harf duyarsız yapar:
// bir karakter, kendisi ya da bir eşdeğeri sınıftaysa eşleşir. Başvurudaki
// gibi yalnızca köşeli parantezli sınıflara ve olumsuzlamadan önce uygulanır
// ((?i)[^a-z] "A"yı da dışlar); köşeli parantezsiz \p{..}, \w, \d, \s
// katlanmaz.
func classFold(g func(rune) bool) func(rune) bool {
	return func(r rune) bool {
		if g(r) {
			return true
		}
		for _, c := range exprFold[r] {
			if g(c) {
				return true
			}
		}
		return false
	}
}

func (a *reParser) item() (reNode, error) {
	c := a.look()
	switch c {
	case '(':
		return a.group()
	case '[':
		f, negated, err := a.classInner()
		if err != nil {
			return nil, err
		}
		if a.insensitive {
			f = classFold(f)
		}
		if negated {
			f = reNot(f)
		}
		return &reClass{f}, nil
	case '.':
		a.i++
		return &reDot{a.dotNewline}, nil
	case '^':
		a.i++
		return &reAnchor{'^'}, nil
	case '$':
		a.i++
		return &reAnchor{'$'}, nil
	case '\\':
		return a.escape()
	case '*', '+', '?':
		return nil, messages.New(messages.H5252, a.i, c)
	}
	a.i++
	return a.charNode(c), nil
}

func (a *reParser) group() (reNode, error) {
	a.i++ // '('
	defer func() { a.depth-- }()
	if err := a.enter(); err != nil {
		return nil, err
	}
	prevD, prevN := a.insensitive, a.dotNewline
	defer func() { a.insensitive, a.dotNewline = prevD, prevN }()
	var wrap func(reNode) reNode = func(d reNode) reNode { return d }
	if a.look() == '?' {
		a.i++
		switch {
		case a.ahead(":"):
			a.i++
		case a.ahead("="), a.ahead("!"):
			negated := a.look() == '!'
			a.i++
			wrap = func(d reNode) reNode { return &reLook{sub: d, negated: negated} }
		case a.ahead("<="), a.ahead("<!"):
			negated := a.s[a.i+1] == '!'
			a.i += 2
			var failure error
			wrap = func(d reNode) reNode {
				n, bounded := reMaxLen(d)
				if !bounded {
					failure = messages.New(messages.H5253)
				}
				return &reLook{sub: d, back: true, negated: negated, length: n}
			}
			d, err := a.option()
			if err != nil {
				return nil, err
			}
			if a.look() != ')' {
				return nil, messages.New(messages.H5254)
			}
			a.i++
			s := wrap(d)
			return s, failure
		case a.ahead(">"):
			a.i++
			wrap = func(d reNode) reNode { return &reAtomic{d} }
		case a.ahead("<"), a.ahead("'"):
			closing := '>'
			if a.look() == '\'' {
				closing = '\''
			}
			a.i++
			for !a.last() && a.look() != closing {
				a.i++
			}
			if a.last() {
				return nil, messages.New(messages.H5255)
			}
			a.i++
		case a.ahead("#"):
			for !a.last() && a.look() != ')' {
				a.i++
			}
			if a.last() {
				return nil, messages.New(messages.H5256)
			}
			a.i++
			return nil, nil
		default:
			// Satır içi bayraklar: (?imx-imx) ya da (?imx-imx:...)
			opened := true
			for !a.last() && a.look() != ')' && a.look() != ':' {
				switch a.look() {
				case '-':
					opened = false
				case 'i':
					a.insensitive = opened
				case 'm':
					a.dotNewline = opened
				default:
					return nil, messages.New(messages.H5257, string(a.look()))
				}
				a.i++
			}
			if a.last() {
				return nil, messages.New(messages.H5254)
			}
			if a.look() == ')' {
				// Bayrak grubun geri kalanına uygulanır: eski değere dönülmez.
				a.i++
				yD, yN := a.insensitive, a.dotNewline
				prevD, prevN = yD, yN
				return nil, nil
			}
			a.i++ // ':'
		}
	}
	d, err := a.option()
	if err != nil {
		return nil, err
	}
	if a.look() != ')' {
		return nil, messages.New(messages.H5254)
	}
	a.i++
	return wrap(d), nil
}

// reMaxLen düğümün eşleyebileceği en çok karakter sayısını verir.
func reMaxLen(d reNode) (int, bool) {
	switch x := d.(type) {
	case nil:
		return 0, true
	case *reChar, *reClass, *reDot:
		return 1, true
	case *reAnchor, *reLook:
		return 0, true
	case *reAtomic:
		return reMaxLen(x.sub)
	case *reUnion:
		t := 0
		for _, o := range x.items {
			n, ok := reMaxLen(o)
			if !ok {
				return 0, false
			}
			t = reSatSum(t, n)
		}
		return t, true
	case *reOption:
		m := 0
		for _, o := range x.options {
			n, ok := reMaxLen(o)
			if !ok {
				return 0, false
			}
			if n > m {
				m = n
			}
		}
		return m, true
	case *reRepeat:
		if x.parent < 0 {
			return 0, false
		}
		n, ok := reMaxLen(x.sub)
		return reSatMul(n, x.parent), ok
	}
	return 0, false
}

func (a *reParser) readHex(minN, maxN int) (rune, error) {
	j := a.i
	for j < len(a.s) && j-a.i < maxN && strings.ContainsRune("0123456789abcdefABCDEF", a.s[j]) {
		j++
	}
	if j-a.i < minN {
		return 0, messages.New(messages.H5258)
	}
	n, _ := strconv.ParseUint(string(a.s[a.i:j]), 16, 32)
	a.i = j
	return rune(n), nil
}

// escapeChar sınıf içinde ve dışında ortak olan tek karakterlik kaçışları
// çözer; tanınmazsa ok=false.
func (a *reParser) escapeChar(c rune) (rune, bool, error) {
	switch c {
	case 't':
		return '\t', true, nil
	case 'n':
		return '\n', true, nil
	case 'r':
		return '\r', true, nil
	case 'f':
		return '\f', true, nil
	case 'v':
		return '\v', true, nil
	case 'a':
		return 7, true, nil
	case 'e':
		return 27, true, nil
	case 'x':
		if a.look() == '{' {
			a.i++
			r, err := a.readHex(1, 8)
			if err != nil {
				return 0, false, err
			}
			if a.look() != '}' {
				return 0, false, messages.New(messages.H5259)
			}
			a.i++
			return r, true, nil
		}
		r, err := a.readHex(1, 2)
		return r, err == nil, err
	case 'u':
		r, err := a.readHex(4, 4)
		return r, err == nil, err
	case '0':
		j := a.i
		for j < len(a.s) && j-a.i < 2 && a.s[j] >= '0' && a.s[j] <= '7' {
			j++
		}
		n, _ := strconv.ParseUint("0"+string(a.s[a.i:j]), 8, 32)
		a.i = j
		return rune(n), true, nil
	}
	return 0, false, nil
}

// escapeClass \d \s \w \h \p{..} ve olumsuzlarını çözer.
func (a *reParser) escapeClass(c rune) (func(rune) bool, bool, error) {
	switch c {
	case 'd':
		return reDigit, true, nil
	case 'D':
		return reNot(reDigit), true, nil
	case 's':
		return exprSpace, true, nil
	case 'S':
		return reNot(exprSpace), true, nil
	case 'w':
		return exprWord, true, nil
	case 'W':
		return reNot(exprWord), true, nil
	case 'h':
		return reHex, true, nil
	case 'H':
		return reNot(reHex), true, nil
	case 'p', 'P':
		if a.look() != '{' {
			return nil, false, messages.New(messages.H5260, string(c))
		}
		j := a.i + 1
		for j < len(a.s) && a.s[j] != '}' {
			j++
		}
		if j >= len(a.s) {
			return nil, false, messages.New(messages.H5261, string(c))
		}
		ad := string(a.s[a.i+1 : j])
		a.i = j + 1
		negated := c == 'P'
		if strings.HasPrefix(ad, "^") {
			negated = !negated
			ad = ad[1:]
		}
		f, err := unicodeProperty(ad)
		if err != nil {
			return nil, false, err
		}
		if negated {
			f = reNot(f)
		}
		return f, true, nil
	}
	return nil, false, nil
}

func reDigit(r rune) bool {
	if r < utf8.RuneSelf {
		return r >= '0' && r <= '9'
	}
	return tableExprDigit.inside(r)
}

func reHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func reNot(f func(rune) bool) func(rune) bool { return func(r rune) bool { return !f(r) } }

func (a *reParser) escape() (reNode, error) {
	a.i++ // '\'
	if a.last() {
		return nil, messages.New(messages.H5262)
	}
	c := a.look()
	a.i++
	switch c {
	case 'A':
		return &reAnchor{'A'}, nil
	case 'z':
		return &reAnchor{'z'}, nil
	case 'Z':
		return &reAnchor{'Z'}, nil
	case 'b':
		return &reAnchor{'b'}, nil
	case 'B':
		return &reAnchor{'B'}, nil
	case 'G', 'K', 'X', 'R', 'k', 'g', 'y', 'Y', 'N', 'O':
		return nil, messages.New(messages.H5263, string(c))
	}
	if c >= '1' && c <= '9' {
		return nil, messages.New(messages.H5264, string(c))
	}
	if f, ok, err := a.escapeClass(c); err != nil {
		return nil, err
	} else if ok {
		return &reClass{f}, nil // köşeli parantezsiz sınıflar (?i) ile katlanmaz
	}
	if r, ok, err := a.escapeChar(c); err != nil {
		return nil, err
	} else if ok {
		return a.charNode(r), nil
	}
	return a.charNode(c), nil
}

// class "[...]" karakter sınıfını (olumsuzlaması uygulanmış olarak) ayrıştırır.
func (a *reParser) class() (func(rune) bool, error) {
	f, negated, err := a.classInner()
	if err == nil && negated {
		f = reNot(f)
	}
	return f, err
}

// classInner "[...]" karakter sınıfının olumsuzlanmamış kümesini ve olumsuz
// olup olmadığını verir.
func (a *reParser) classInner() (func(rune) bool, bool, error) {
	a.i++ // '['
	defer func() { a.depth-- }()
	if err := a.enter(); err != nil {
		return nil, false, err
	}
	negated := false
	if a.look() == '^' {
		negated = true
		a.i++
	}
	var pieces []func(rune) bool
	var intersection []func(rune) bool
	first := true
	for {
		if a.last() {
			return nil, false, messages.New(messages.H5265)
		}
		c := a.look()
		if c == ']' && !first {
			a.i++
			break
		}
		first = false
		if a.ahead("&&") {
			a.i += 2
			intersection = append(intersection, reConcat(pieces))
			pieces = nil
			continue
		}
		if a.ahead("[:") {
			if f, ok := a.posixClass(); ok {
				pieces = append(pieces, f)
				continue
			}
		}
		if c == '[' {
			f, err := a.class()
			if err != nil {
				return nil, false, err
			}
			pieces = append(pieces, f)
			continue
		}
		begin, f, err := a.classItem()
		if err != nil {
			return nil, false, err
		}
		if f != nil {
			pieces = append(pieces, f)
			continue
		}
		// Aralık mı?
		if a.look() == '-' && a.i+1 < len(a.s) && a.s[a.i+1] != ']' {
			record := a.i
			a.i++
			bit, f2, err := a.classItem()
			if err != nil {
				return nil, false, err
			}
			if f2 != nil {
				// "a-\d" gibi: '-' düz karakterdir.
				a.i = record
				pieces = append(pieces, reSingle(begin))
				continue
			}
			if bit < begin {
				return nil, false, messages.New(messages.H5266)
			}
			lo, hi := begin, bit
			pieces = append(pieces, func(r rune) bool { return r >= lo && r <= hi })
			continue
		}
		pieces = append(pieces, reSingle(begin))
	}
	f := reConcat(pieces)
	if len(intersection) > 0 {
		all := append(intersection, f)
		f = func(r rune) bool {
			for _, g := range all {
				if !g(r) {
					return false
				}
			}
			return true
		}
	}
	return f, negated, nil
}

func reSingle(c rune) func(rune) bool { return func(r rune) bool { return r == c } }

func reConcat(p []func(rune) bool) func(rune) bool {
	if len(p) == 1 {
		return p[0]
	}
	return func(r rune) bool {
		for _, g := range p {
			if g(r) {
				return true
			}
		}
		return false
	}
}

// classItem sınıf içindeki tek bir öğeyi okur: ya bir karakter ya bir alt sınıf.
func (a *reParser) classItem() (rune, func(rune) bool, error) {
	c := a.look()
	a.i++
	if c != '\\' {
		return c, nil, nil
	}
	if a.last() {
		return 0, nil, messages.New(messages.H5267)
	}
	c = a.look()
	a.i++
	if c == 'b' {
		return '\b', nil, nil
	}
	if f, ok, err := a.escapeClass(c); err != nil {
		return 0, nil, err
	} else if ok {
		return 0, f, nil
	}
	if r, ok, err := a.escapeChar(c); err != nil {
		return 0, nil, err
	} else if ok {
		return r, nil, nil
	}
	return c, nil, nil
}

func (a *reParser) posixClass() (func(rune) bool, bool) {
	j := a.i + 2
	negated := false
	if j < len(a.s) && a.s[j] == '^' {
		negated = true
		j++
	}
	k := j
	for k < len(a.s) && a.s[k] >= 'a' && a.s[k] <= 'z' {
		k++
	}
	if k+1 >= len(a.s) || a.s[k] != ':' || a.s[k+1] != ']' {
		return nil, false
	}
	f, ok := rePosixes()[string(a.s[j:k])]
	if !ok {
		return nil, false
	}
	a.i = k + 2
	if negated {
		f = reNot(f)
	}
	return f, true
}

// unicodeProperty \p{ad} için sınama işlevini verir. Ad büyük/küçük harf,
// boşluk, '_' ve '-' farkı gözetmeden eşlenir.
func unicodeProperty(ad string) (func(rune) bool, error) {
	n := strings.Map(func(r rune) rune {
		if r == ' ' || r == '_' || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, ad)
	if f, ok := reProps()[n]; ok {
		return f, nil
	}
	return nil, messages.New(messages.H5268, ad)
}

var (
	rePropOne    sync.Once
	rePropTable  map[string]func(rune) bool // \p{ad}
	rePosixTable map[string]func(rune) bool // [[:ad:]]
)

func reProps() map[string]func(rune) bool {
	rePropOne.Do(reBuildProp)
	return rePropTable
}

func rePosixes() map[string]func(rune) bool {
	rePropOne.Do(reBuildProp)
	return rePosixTable
}

// reBuildProp sınıf tablolarını kurar. Bütün tablolar (genel kategoriler,
// yazı sistemleri, ikili özellikler, POSIX sınıfları) başvuru uygulamasının
// düzenli ifade motorundan çıkarılmıştır (karaktertablo.go, ozelliktablo.go):
// Unicode sürümü ve sınıf tanımları (ör. [[:punct:]] = P ve ASCII simgeleri)
// onunla aynıdır; başvurunun tanımadığı adlar burada da tanınmaz.
func reBuildProp() {
	m := make(map[string]func(rune) bool, len(exprCategory)+len(exprPropCategory)+len(exprPropNames))
	for ad, t := range exprCategory {
		if ad != "bosluk" { // \s için iç tablo; bir özellik adı değil
			m[ad] = t.inside
		}
	}
	for ad, k := range exprPropCategory {
		m[ad] = exprCategory[k].inside
	}
	for ad, i := range exprPropNames {
		m[ad] = exprPropTables[i].inside
	}
	p := make(map[string]func(rune) bool, len(exprPosixNames)+len(exprPosixCategory))
	for ad, k := range exprPosixCategory {
		p[ad] = exprCategory[k].inside
	}
	for ad, i := range exprPosixNames {
		p[ad] = exprPropTables[i].inside
	}
	rePropTable, rePosixTable = m, p
}

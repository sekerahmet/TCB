package tokenize

import (
	"unicode/utf8"
)

// ============================================================================
// Eşleştirici: yığınsız geri izlemeli sanal makine
// ============================================================================
//
// Desen ağacı, komut dizisine (program) derlenir; eşleştirici programı açık
// bir geri izleme yığınıyla yürütür. Böylece Go çağrı yığınının derinliği
// girdinin uzunluğuna bağlı değildir: "(?:ab)+" gibi grup yinelemeleri ne
// kadar uzun girdide çalışırsa çalışsın süreç yığın taşmasıyla çökmez.
// Özyineleme yalnızca bakma ve atomik gruplarda (alt program) olur ve
// derinliği desenin iç içe derinliğiyle (bkz. dfEnCokDerinlik) sınırlıdır.
//
// Arama sırası, önceki (devam geçişli) motorla birebir aynıdır: seçenekler
// soldan sağa, açgözlü niceleyiciler en uzundan, tembeller en kısadan
// denenir; sınırı aşmış bir yinelemenin boş eşleşmesi reddedilir.

type reOp uint8

const (
	opSingle       reOp = iota // tek karakter
	opSingleRepeat             // tek karakterlik düğümün yinelemesi
	opSplit                    // önce x, geri izlemede y
	opSkip                     // x'e atla
	opAnchor                   // ^ $ \A \z \Z \b \B
	opAtomic                   // alt programın ilk bitişini al, içine geri dönme
	opLook                     // ileri/geri bakma
	opLoopStart                // sayaç = 0, başlangıç = konum
	opLoopDecide               // bir yineleme daha mı, çıkış mı
	opLoopEnd                  // yineleme sonu: boş yineleme denetimi, sayaç++
	opMatched
)

// Tek karakterlik düğümlerin türleri.
const (
	singleChar uint8 = iota
	singleClass
	singleDot
)

type reInst struct {
	op         reOp
	singleKind uint8
	kind       byte // çapa türü
	lazy       bool
	owner      bool
	back       bool
	negated    bool
	dot        bool // nokta satır sonunu da kapsar
	r          rune
	fold       []rune
	f          func(rune) bool
	x, y       int // atlama hedefleri; döngüde x: sayaç yuvası, y: çıkış ya da karar
	en, parent int
	length     int
	first      *reFirstSet // opBol: x dalının ilk karakteri bunlardan biri olmalı (nil: süzgeç yok)
	sub        *reProgram
}

type reProgram struct {
	k []reInst
}

// dfIlkKume bir dalın tüketebileceği ilk karakterlerin üst kümesidir.
type reFirstSet struct {
	ascii [2]uint64
	runes []rune
	fs    []func(rune) bool
}

func (k *reFirstSet) inside(r rune) bool {
	if r < utf8.RuneSelf {
		return k.ascii[r>>6]&(1<<(uint(r)&63)) != 0
	}
	for _, c := range k.runes {
		if c == r {
			return true
		}
	}
	for _, f := range k.fs {
		if f(r) {
			return true
		}
	}
	return false
}

// dfIlkKumeKur düğüm en az bir karakter tüketiyorsa ilk karakter kümesini,
// değilse nil verir.
func reBuildFirstSet(d reNode) *reFirstSet {
	if reMinLen(d) == 0 {
		return nil
	}
	rs, fs := reFirstChars(d)
	k := &reFirstSet{runes: rs, fs: fs}
	for c := rune(0); c < utf8.RuneSelf; c++ {
		var var1 bool
		for _, x := range rs {
			if x == c {
				var1 = true
				break
			}
		}
		for _, f := range fs {
			if var1 {
				break
			}
			var1 = f(c)
		}
		if var1 {
			k.ascii[c>>6] |= 1 << (uint(c) & 63)
		}
	}
	return k
}

// dfDerleyici desen ağacını programa çevirir; döngü yuvaları bütün alt
// programlarda ortak numaralanır.
type reCompiler struct{ slot int }

func (c *reCompiler) program(d reNode) *reProgram {
	p := &reProgram{}
	c.node(p, d)
	p.k = append(p.k, reInst{op: opMatched})
	return p
}

func singleInst(d reNode) (reInst, bool) {
	switch x := d.(type) {
	case *reChar:
		return reInst{op: opSingle, singleKind: singleChar, r: x.r, fold: x.fold}, true
	case *reClass:
		return reInst{op: opSingle, singleKind: singleClass, f: x.f}, true
	case *reDot:
		return reInst{op: opSingle, singleKind: singleDot, dot: x.includeNewline}, true
	}
	return reInst{}, false
}

func (c *reCompiler) node(p *reProgram, d reNode) {
	if k, ok := singleInst(d); ok {
		p.k = append(p.k, k)
		return
	}
	switch x := d.(type) {
	case nil:
	case *reUnion:
		for _, o := range x.items {
			c.node(p, o)
		}
	case *reOption:
		var skips []int
		for i, s := range x.options {
			if i == len(x.options)-1 {
				c.node(p, s)
				break
			}
			splitFn := len(p.k)
			p.k = append(p.k, reInst{op: opSplit, x: splitFn + 1, first: reBuildFirstSet(s)})
			c.node(p, s)
			skips = append(skips, len(p.k))
			p.k = append(p.k, reInst{op: opSkip})
			p.k[splitFn].y = len(p.k)
		}
		for _, a := range skips {
			p.k[a].x = len(p.k)
		}
	case *reRepeat:
		if k, ok := singleInst(x.sub); ok {
			k.op = opSingleRepeat
			k.en, k.parent, k.lazy, k.owner = x.en, x.parent, x.lazy, x.claimer
			p.k = append(p.k, k)
			return
		}
		if x.claimer {
			ic := *x
			ic.claimer = false
			p.k = append(p.k, reInst{op: opAtomic, sub: c.program(&ic)})
			return
		}
		slot := c.slot
		c.slot += 2 // sayaç ve yinelemenin başladığı konum
		p.k = append(p.k, reInst{op: opLoopStart, x: slot})
		decision := len(p.k)
		p.k = append(p.k, reInst{op: opLoopDecide, x: slot, en: x.en, parent: x.parent, lazy: x.lazy})
		c.node(p, x.sub)
		p.k = append(p.k, reInst{op: opLoopEnd, x: slot, en: x.en, y: decision})
		p.k[decision].y = len(p.k)
	case *reAtomic:
		p.k = append(p.k, reInst{op: opAtomic, sub: c.program(x.sub)})
	case *reLook:
		p.k = append(p.k, reInst{op: opLook, sub: c.program(x.sub), back: x.back, negated: x.negated, length: x.length})
	case *reAnchor:
		p.k = append(p.k, reInst{op: opAnchor, kind: x.kind})
	default:
		panic("bilinmeyen düzenli ifade düğümü")
	}
}

// Geri izleme yığını kayıt türleri.
const (
	backChoice      uint8 = iota // pc, pos'tan sür
	writeBack                    // döngü yuvalarını eski değerlerine döndür
	backSingleOpen               // açgözlü tek karakter yinelemesi: bir karakter geri çekil
	backSingleClean              // tembel tek karakter yinelemesi: bir karakter daha al
)

type reBack struct {
	kind uint8
	pc   int32 // geriYaz: yuva
	pos  int   // geriYaz: eski başlangıç konumu
	n    int   // geriYaz: eski sayaç; geriTekAc: kalan geri adım; geriTekTem: yineleme sayısı
}

type reMatcher struct {
	s       string
	pattern string
	step    int64 // geçerli çağrıda atılan adım
	budget  int64 // geçerli çağrının kalan adım bütçesi (başlangıçta)
	ceil    int64 // bütçenin toplamı (hata iletisi için)
	begin   int   // geçerli aramanın başlangıcı (hata iletisi için)
	stack   []reBack
	stackN  int // geri izleme yığınının en çok kayıt sayısı
	reg     []int
}

func (m *reMatcher) exceeded(mem bool) {
	panic(&StepLimitError{Pattern: m.pattern, Pos: m.begin, Budget: m.ceil, Memory: mem})
}

func (m *reMatcher) it(g reBack) {
	if len(m.stack) >= m.stackN {
		m.exceeded(true)
	}
	m.stack = append(m.stack, g)
}

func (m *reMatcher) prev(pos int) (rune, int) {
	if pos <= 0 {
		return -1, 0
	}
	if c := m.s[pos-1]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeLastRuneInString(m.s[:pos])
}

func (m *reMatcher) next(pos int) (rune, int) {
	if pos >= len(m.s) {
		return -1, 0
	}
	if c := m.s[pos]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(m.s[pos:])
}

// tekEsle tek karakterlik komutu pos'ta sınar.
func (m *reMatcher) matchSingle(k *reInst, pos int) (int, bool) {
	r, w := m.next(pos)
	if w == 0 {
		return 0, false
	}
	switch k.singleKind {
	case singleChar:
		if r == k.r {
			return pos + w, true
		}
		for _, c := range k.fold {
			if r == c {
				return pos + w, true
			}
		}
		return 0, false
	case singleClass:
		return pos + w, k.f(r)
	default:
		return pos + w, r != '\n' || k.dot
	}
}

func (m *reMatcher) anchor(kind byte, pos int) bool {
	switch kind {
	case '^':
		return pos == 0 || m.s[pos-1] == '\n'
	case '$':
		return pos == len(m.s) || m.s[pos] == '\n'
	case 'A':
		return pos == 0
	case 'z':
		return pos == len(m.s)
	case 'Z':
		return pos == len(m.s) || (pos == len(m.s)-1 && m.s[pos] == '\n')
	case 'b', 'B':
		r1, _ := m.prev(pos)
		r2, _ := m.next(pos)
		limit := (r1 >= 0 && exprWord(r1)) != (r2 >= 0 && exprWord(r2))
		return limit == (kind == 'b')
	}
	return false
}

// calistir programı pos'tan yürütür ve ilk başarılı bitişi (yoksa -1) verir.
// hedef >= 0 ise yalnızca hedef konumunda biten eşleşme kabul edilir
// (geriye bakma). Dönüşte yığın, girişteki hâline indirilmiştir: başarılı
// bir alt programın seçim noktaları atılır (atomik anlam).
func (m *reMatcher) run(p *reProgram, pos, target int) int {
	base := len(m.stack)
	k := p.k
	pc := 0
	for {
		m.step++
		if m.step > m.budget {
			m.exceeded(false)
		}
		c := &k[pc]
		ok := true
		switch c.op {
		case opSingle:
			pos, ok = m.matchSingle(c, pos)
			pc++
		case opSingleRepeat:
			pc++
			if c.lazy {
				n := 0
				for ; n < c.en; n++ {
					if pos, ok = m.matchSingle(c, pos); !ok {
						break
					}
				}
				m.step += int64(n)
				if ok && (c.parent < 0 || n < c.parent) {
					m.it(reBack{kind: backSingleClean, pc: int32(pc), pos: pos, n: n})
				}
				break
			}
			n := 0
			for c.parent < 0 || n < c.parent {
				q, e := m.matchSingle(c, pos)
				if !e {
					break
				}
				pos = q
				n++
			}
			m.step += int64(n)
			if n < c.en {
				ok = false
			} else if n > c.en && !c.owner {
				m.it(reBack{kind: backSingleOpen, pc: int32(pc), pos: pos, n: n - c.en})
			}
		case opSplit:
			if c.first != nil {
				if r, w := m.next(pos); w == 0 || !c.first.inside(r) {
					pc = c.y
					break
				}
			}
			m.it(reBack{kind: backChoice, pc: int32(c.y), pos: pos})
			pc = c.x
		case opSkip:
			pc = c.x
		case opAnchor:
			ok = m.anchor(c.kind, pos)
			pc++
		case opAtomic:
			e := m.run(c.sub, pos, -1)
			ok = e >= 0
			pos = e
			pc++
		case opLook:
			held := false
			if c.back {
				begin := pos
				for n := 0; ; n++ {
					if m.run(c.sub, begin, pos) >= 0 {
						held = true
						break
					}
					if n >= c.length || begin == 0 {
						break
					}
					_, w := m.prev(begin)
					begin -= w
				}
			} else {
				held = m.run(c.sub, pos, -1) >= 0
			}
			ok = held != c.negated
			pc++
		case opLoopStart:
			m.it(reBack{kind: writeBack, pc: int32(c.x), pos: m.reg[c.x+1], n: m.reg[c.x]})
			m.reg[c.x], m.reg[c.x+1] = 0, pos
			pc++
		case opLoopDecide:
			num := m.reg[c.x]
			cont := c.parent < 0 || num < c.parent
			switch {
			case c.lazy && num >= c.en:
				if cont {
					m.it(reBack{kind: backChoice, pc: int32(pc + 1), pos: pos})
				}
				pc = c.y
			case !c.lazy && cont:
				if num >= c.en {
					m.it(reBack{kind: backChoice, pc: int32(c.y), pos: pos})
				}
				pc++
			case cont:
				pc++
			case num >= c.en:
				pc = c.y
			default:
				ok = false
			}
		case opLoopEnd:
			num := m.reg[c.x]
			if pos == m.reg[c.x+1] && num >= c.en {
				ok = false // boş yineleme sonsuz döngüye girmesin
				break
			}
			m.it(reBack{kind: writeBack, pc: int32(c.x), pos: m.reg[c.x+1], n: num})
			m.reg[c.x], m.reg[c.x+1] = num+1, pos
			pc = c.y
		case opMatched:
			if target < 0 || pos == target {
				m.stack = m.stack[:base]
				return pos
			}
			ok = false
		}
		if ok {
			continue
		}
		// Geri izle: en son seçim noktasına dön.
	back:
		for {
			n := len(m.stack) - 1
			if n < base {
				return -1
			}
			g := m.stack[n]
			m.stack = m.stack[:n]
			switch g.kind {
			case writeBack:
				m.reg[g.pc], m.reg[g.pc+1] = g.n, g.pos
			case backChoice:
				pc, pos = int(g.pc), g.pos
				break back
			case backSingleOpen:
				_, w := m.prev(g.pos)
				pos = g.pos - w
				if g.n > 1 {
					m.stack = append(m.stack, reBack{kind: backSingleOpen, pc: g.pc, pos: pos, n: g.n - 1})
				}
				pc = int(g.pc)
				break back
			case backSingleClean:
				t := &k[g.pc-1]
				q, e := m.matchSingle(t, g.pos)
				if !e {
					continue
				}
				m.step++
				if t.parent < 0 || g.n+1 < t.parent {
					m.stack = append(m.stack, reBack{kind: backSingleClean, pc: g.pc, pos: q, n: g.n + 1})
				}
				pc, pos = int(g.pc), q
				break back
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Arama
// ---------------------------------------------------------------------------

// dfButce bir çağrının (ör. bir metnin kodlanması) bütün düzenli ifade
// aramalarının paylaştığı adım bütçesidir.
type reBudget struct {
	rest int64
	ceil int64
}

func reSatAdd(a, b int64) int64 {
	if b > 0 && a > (1<<62)-b {
		return 1 << 62
	}
	return a + b
}

// yeniButce bayt uzunluğundaki bir girdi için bütçe açar:
// AdimTabani + AdimCarpani * bayt.
func newBudget(byteVal int) *reBudget {
	b := &reBudget{}
	b.ceil = regexSetting.Load().StepBase
	b.rest = b.ceil
	b.add(byteVal)
	return b
}

// ekle bütçeyi bayt uzunluğunda ek girdi için büyütür (AdimCarpani * bayt).
func (b *reBudget) add(byteVal int) {
	if byteVal <= 0 {
		return
	}
	a := regexSetting.Load().StepFactor
	n := int64(1 << 62)
	if int64(byteVal) < (1<<62)/a {
		n = a * int64(byteVal)
	}
	b.rest = reSatAdd(b.rest, n)
	b.ceil = reSatAdd(b.ceil, n)
}

func (d *Regex) matcher(s string, b *reBudget) *reMatcher {
	m := &reMatcher{s: s, pattern: reShorten(d.source), budget: b.rest, ceil: b.ceil, stackN: regexSetting.Load().BacktrackLimit}
	if d.slot > 0 {
		m.reg = make([]int, d.slot)
	}
	return m
}

// Bul s içinde bas konumundan itibaren en soldaki eşleşmeyi verir. Arama
// AdimTabani + AdimCarpani * (kalan bayt) adımlık bir bütçe kullanır; aşılırsa
// *AdimSiniriHatasi döner.
func (d *Regex) Find(s string, begin int) (b, e int, ok bool, err error) {
	err = catchStepError(func() {
		m := d.matcher(s, newBudget(len(s)-begin))
		b, e, ok = d.findM(m, begin)
	})
	return
}

// TumEslesmelerHata, TumEslesmeler'in hata döndüren biçimidir: adım bütçesi
// aşılırsa arama durur ve *AdimSiniriHatasi döner.
func (d *Regex) FindAllErr(s string, f func(begin, last int)) error {
	return catchStepError(func() { d.allMatches(s, f, nil) })
}

// TumEslesmeler çakışmayan bütün eşleşmeleri sırayla verir. Bir önceki
// eşleşmenin hemen ardından gelen boş eşleşme kabul edilmez (sonsuz
// döngüyü önler); bu durumda arama bir karakter ileriden sürer. Bütün
// aramalar tek bir AdimTabani + AdimCarpani * len(s) adımlık bütçeyi paylaşır;
// aşılırsa *AdimSiniriHatasi ile panikler. Hata isteyen çağıranlar
// TumEslesmelerHata'yı (parçalayıcıda Guvenli... işlevlerini) kullanmalıdır.
func (d *Regex) FindAll(s string, f func(begin, last int)) {
	d.allMatches(s, f, nil)
}

// tumEslesmeler eşleşmeleri verir ve atılan adım sayısını döndürür. b nil
// değilse adımlar o bütçeden düşülür (bir kodlama çağrısının bütün
// aramaları aynı bütçeyi paylaşır); nil ise s için yeni bir bütçe açılır.
func (d *Regex) allMatches(s string, f func(begin, last int), b *reBudget) int64 {
	if b == nil {
		b = newBudget(len(s))
	}
	m := d.matcher(s, b)
	defer func() { b.rest -= m.step }()
	pos := 0
	lastMatch := -1
	for pos <= len(s) {
		bs, e, ok := d.findM(m, pos)
		if !ok {
			break
		}
		if bs == e && lastMatch == e {
			if pos >= len(s) {
				break
			}
			_, w := m.next(pos)
			pos += w
			continue
		}
		f(bs, e)
		pos = e
		lastMatch = e
	}
	return m.step
}

// bulM bas'tan itibaren ilk eşleşmeyi arar.
func (d *Regex) findM(m *reMatcher, begin int) (int, int, bool) {
	m.begin = begin
	for p := begin; p <= len(m.s); {
		if d.first != nil {
			// Desenin ilk karakteri olamayacak konumları atla.
			for p < len(m.s) {
				r, w := m.next(p)
				if d.first.inside(r) {
					break
				}
				p += w
				m.step++
			}
			if p >= len(m.s) {
				break
			}
		}
		if e := m.run(d.prog, p, -1); e >= 0 {
			return p, e, true
		}
		if p == len(m.s) {
			break
		}
		_, w := m.next(p)
		p += w
	}
	return 0, 0, false
}

package parcalayici

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

type dfOp uint8

const (
	opTek        dfOp = iota // tek karakter
	opTekTekrar              // tek karakterlik düğümün yinelemesi
	opBol                    // önce x, geri izlemede y
	opAtla                   // x'e atla
	opCapa                   // ^ $ \A \z \Z \b \B
	opAtomik                 // alt programın ilk bitişini al, içine geri dönme
	opBakma                  // ileri/geri bakma
	opDonguBas               // sayaç = 0, başlangıç = konum
	opDonguKarar             // bir yineleme daha mı, çıkış mı
	opDonguSon               // yineleme sonu: boş yineleme denetimi, sayaç++
	opEslesti
)

// Tek karakterlik düğümlerin türleri.
const (
	tekKarakter uint8 = iota
	tekSinif
	tekNokta
)

type dfKomut struct {
	op      dfOp
	tekTur  uint8
	tur     byte // çapa türü
	tembel  bool
	sahip   bool
	geri    bool
	olumsuz bool
	nokta   bool // nokta satır sonunu da kapsar
	r       rune
	katla   []rune
	f       func(rune) bool
	x, y    int // atlama hedefleri; döngüde x: sayaç yuvası, y: çıkış ya da karar
	en, ust int
	uzunluk int
	ilk     *dfIlkKume // opBol: x dalının ilk karakteri bunlardan biri olmalı (nil: süzgeç yok)
	alt     *dfProgram
}

type dfProgram struct {
	k []dfKomut
}

// dfIlkKume bir dalın tüketebileceği ilk karakterlerin üst kümesidir.
type dfIlkKume struct {
	ascii [2]uint64
	runes []rune
	fs    []func(rune) bool
}

func (k *dfIlkKume) icinde(r rune) bool {
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
func dfIlkKumeKur(d dfDugum) *dfIlkKume {
	if dfEnKisa(d) == 0 {
		return nil
	}
	rs, fs := dfIlkKarakterler(d)
	k := &dfIlkKume{runes: rs, fs: fs}
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
type dfDerleyici struct{ yuva int }

func (c *dfDerleyici) program(d dfDugum) *dfProgram {
	p := &dfProgram{}
	c.dugum(p, d)
	p.k = append(p.k, dfKomut{op: opEslesti})
	return p
}

func tekKomutu(d dfDugum) (dfKomut, bool) {
	switch x := d.(type) {
	case *dfKarakter:
		return dfKomut{op: opTek, tekTur: tekKarakter, r: x.r, katla: x.katla}, true
	case *dfSinif:
		return dfKomut{op: opTek, tekTur: tekSinif, f: x.f}, true
	case *dfNokta:
		return dfKomut{op: opTek, tekTur: tekNokta, nokta: x.satirSonuDahil}, true
	}
	return dfKomut{}, false
}

func (c *dfDerleyici) dugum(p *dfProgram, d dfDugum) {
	if k, ok := tekKomutu(d); ok {
		p.k = append(p.k, k)
		return
	}
	switch x := d.(type) {
	case nil:
	case *dfBirlesim:
		for _, o := range x.ogeler {
			c.dugum(p, o)
		}
	case *dfSecenek:
		var atlamalar []int
		for i, s := range x.secenekler {
			if i == len(x.secenekler)-1 {
				c.dugum(p, s)
				break
			}
			bol := len(p.k)
			p.k = append(p.k, dfKomut{op: opBol, x: bol + 1, ilk: dfIlkKumeKur(s)})
			c.dugum(p, s)
			atlamalar = append(atlamalar, len(p.k))
			p.k = append(p.k, dfKomut{op: opAtla})
			p.k[bol].y = len(p.k)
		}
		for _, a := range atlamalar {
			p.k[a].x = len(p.k)
		}
	case *dfTekrar:
		if k, ok := tekKomutu(x.alt); ok {
			k.op = opTekTekrar
			k.en, k.ust, k.tembel, k.sahip = x.en, x.ust, x.tembel, x.sahiplenici
			p.k = append(p.k, k)
			return
		}
		if x.sahiplenici {
			ic := *x
			ic.sahiplenici = false
			p.k = append(p.k, dfKomut{op: opAtomik, alt: c.program(&ic)})
			return
		}
		yuva := c.yuva
		c.yuva += 2 // sayaç ve yinelemenin başladığı konum
		p.k = append(p.k, dfKomut{op: opDonguBas, x: yuva})
		karar := len(p.k)
		p.k = append(p.k, dfKomut{op: opDonguKarar, x: yuva, en: x.en, ust: x.ust, tembel: x.tembel})
		c.dugum(p, x.alt)
		p.k = append(p.k, dfKomut{op: opDonguSon, x: yuva, en: x.en, y: karar})
		p.k[karar].y = len(p.k)
	case *dfAtomik:
		p.k = append(p.k, dfKomut{op: opAtomik, alt: c.program(x.alt)})
	case *dfBakma:
		p.k = append(p.k, dfKomut{op: opBakma, alt: c.program(x.alt), geri: x.geri, olumsuz: x.olumsuz, uzunluk: x.uzunluk})
	case *dfCapa:
		p.k = append(p.k, dfKomut{op: opCapa, tur: x.tur})
	default:
		panic("bilinmeyen düzenli ifade düğümü")
	}
}

// Geri izleme yığını kayıt türleri.
const (
	geriSecim  uint8 = iota // pc, pos'tan sür
	geriYaz                 // döngü yuvalarını eski değerlerine döndür
	geriTekAc               // açgözlü tek karakter yinelemesi: bir karakter geri çekil
	geriTekTem              // tembel tek karakter yinelemesi: bir karakter daha al
)

type dfGeri struct {
	tur uint8
	pc  int32 // geriYaz: yuva
	pos int   // geriYaz: eski başlangıç konumu
	n   int   // geriYaz: eski sayaç; geriTekAc: kalan geri adım; geriTekTem: yineleme sayısı
}

type dfEslestirici struct {
	s      string
	desen  string
	adim   int64 // geçerli çağrıda atılan adım
	butce  int64 // geçerli çağrının kalan adım bütçesi (başlangıçta)
	tavan  int64 // bütçenin toplamı (hata iletisi için)
	bas    int   // geçerli aramanın başlangıcı (hata iletisi için)
	yigin  []dfGeri
	yiginS int // geri izleme yığınının en çok kayıt sayısı
	reg    []int
}

func (m *dfEslestirici) asildi(bellek bool) {
	panic(&AdimSiniriHatasi{Desen: m.desen, Konum: m.bas, Butce: m.tavan, Bellek: bellek})
}

func (m *dfEslestirici) it(g dfGeri) {
	if len(m.yigin) >= m.yiginS {
		m.asildi(true)
	}
	m.yigin = append(m.yigin, g)
}

func (m *dfEslestirici) onceki(pos int) (rune, int) {
	if pos <= 0 {
		return -1, 0
	}
	if c := m.s[pos-1]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeLastRuneInString(m.s[:pos])
}

func (m *dfEslestirici) sonraki(pos int) (rune, int) {
	if pos >= len(m.s) {
		return -1, 0
	}
	if c := m.s[pos]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(m.s[pos:])
}

// tekEsle tek karakterlik komutu pos'ta sınar.
func (m *dfEslestirici) tekEsle(k *dfKomut, pos int) (int, bool) {
	r, w := m.sonraki(pos)
	if w == 0 {
		return 0, false
	}
	switch k.tekTur {
	case tekKarakter:
		if r == k.r {
			return pos + w, true
		}
		for _, c := range k.katla {
			if r == c {
				return pos + w, true
			}
		}
		return 0, false
	case tekSinif:
		return pos + w, k.f(r)
	default:
		return pos + w, r != '\n' || k.nokta
	}
}

func (m *dfEslestirici) capa(tur byte, pos int) bool {
	switch tur {
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
		r1, _ := m.onceki(pos)
		r2, _ := m.sonraki(pos)
		sinir := (r1 >= 0 && ifadeSozcuk(r1)) != (r2 >= 0 && ifadeSozcuk(r2))
		return sinir == (tur == 'b')
	}
	return false
}

// calistir programı pos'tan yürütür ve ilk başarılı bitişi (yoksa -1) verir.
// hedef >= 0 ise yalnızca hedef konumunda biten eşleşme kabul edilir
// (geriye bakma). Dönüşte yığın, girişteki hâline indirilmiştir: başarılı
// bir alt programın seçim noktaları atılır (atomik anlam).
func (m *dfEslestirici) calistir(p *dfProgram, pos, hedef int) int {
	taban := len(m.yigin)
	k := p.k
	pc := 0
	for {
		m.adim++
		if m.adim > m.butce {
			m.asildi(false)
		}
		c := &k[pc]
		ok := true
		switch c.op {
		case opTek:
			pos, ok = m.tekEsle(c, pos)
			pc++
		case opTekTekrar:
			pc++
			if c.tembel {
				n := 0
				for ; n < c.en; n++ {
					if pos, ok = m.tekEsle(c, pos); !ok {
						break
					}
				}
				m.adim += int64(n)
				if ok && (c.ust < 0 || n < c.ust) {
					m.it(dfGeri{tur: geriTekTem, pc: int32(pc), pos: pos, n: n})
				}
				break
			}
			n := 0
			for c.ust < 0 || n < c.ust {
				q, e := m.tekEsle(c, pos)
				if !e {
					break
				}
				pos = q
				n++
			}
			m.adim += int64(n)
			if n < c.en {
				ok = false
			} else if n > c.en && !c.sahip {
				m.it(dfGeri{tur: geriTekAc, pc: int32(pc), pos: pos, n: n - c.en})
			}
		case opBol:
			if c.ilk != nil {
				if r, w := m.sonraki(pos); w == 0 || !c.ilk.icinde(r) {
					pc = c.y
					break
				}
			}
			m.it(dfGeri{tur: geriSecim, pc: int32(c.y), pos: pos})
			pc = c.x
		case opAtla:
			pc = c.x
		case opCapa:
			ok = m.capa(c.tur, pos)
			pc++
		case opAtomik:
			e := m.calistir(c.alt, pos, -1)
			ok = e >= 0
			pos = e
			pc++
		case opBakma:
			tuttu := false
			if c.geri {
				bas := pos
				for n := 0; ; n++ {
					if m.calistir(c.alt, bas, pos) >= 0 {
						tuttu = true
						break
					}
					if n >= c.uzunluk || bas == 0 {
						break
					}
					_, w := m.onceki(bas)
					bas -= w
				}
			} else {
				tuttu = m.calistir(c.alt, pos, -1) >= 0
			}
			ok = tuttu != c.olumsuz
			pc++
		case opDonguBas:
			m.it(dfGeri{tur: geriYaz, pc: int32(c.x), pos: m.reg[c.x+1], n: m.reg[c.x]})
			m.reg[c.x], m.reg[c.x+1] = 0, pos
			pc++
		case opDonguKarar:
			sayi := m.reg[c.x]
			devam := c.ust < 0 || sayi < c.ust
			switch {
			case c.tembel && sayi >= c.en:
				if devam {
					m.it(dfGeri{tur: geriSecim, pc: int32(pc + 1), pos: pos})
				}
				pc = c.y
			case !c.tembel && devam:
				if sayi >= c.en {
					m.it(dfGeri{tur: geriSecim, pc: int32(c.y), pos: pos})
				}
				pc++
			case devam:
				pc++
			case sayi >= c.en:
				pc = c.y
			default:
				ok = false
			}
		case opDonguSon:
			sayi := m.reg[c.x]
			if pos == m.reg[c.x+1] && sayi >= c.en {
				ok = false // boş yineleme sonsuz döngüye girmesin
				break
			}
			m.it(dfGeri{tur: geriYaz, pc: int32(c.x), pos: m.reg[c.x+1], n: sayi})
			m.reg[c.x], m.reg[c.x+1] = sayi+1, pos
			pc = c.y
		case opEslesti:
			if hedef < 0 || pos == hedef {
				m.yigin = m.yigin[:taban]
				return pos
			}
			ok = false
		}
		if ok {
			continue
		}
		// Geri izle: en son seçim noktasına dön.
	geri:
		for {
			n := len(m.yigin) - 1
			if n < taban {
				return -1
			}
			g := m.yigin[n]
			m.yigin = m.yigin[:n]
			switch g.tur {
			case geriYaz:
				m.reg[g.pc], m.reg[g.pc+1] = g.n, g.pos
			case geriSecim:
				pc, pos = int(g.pc), g.pos
				break geri
			case geriTekAc:
				_, w := m.onceki(g.pos)
				pos = g.pos - w
				if g.n > 1 {
					m.yigin = append(m.yigin, dfGeri{tur: geriTekAc, pc: g.pc, pos: pos, n: g.n - 1})
				}
				pc = int(g.pc)
				break geri
			case geriTekTem:
				t := &k[g.pc-1]
				q, e := m.tekEsle(t, g.pos)
				if !e {
					continue
				}
				m.adim++
				if t.ust < 0 || g.n+1 < t.ust {
					m.yigin = append(m.yigin, dfGeri{tur: geriTekTem, pc: g.pc, pos: q, n: g.n + 1})
				}
				pc, pos = int(g.pc), q
				break geri
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Arama
// ---------------------------------------------------------------------------

// dfButce bir çağrının (ör. bir metnin kodlanması) bütün düzenli ifade
// aramalarının paylaştığı adım bütçesidir.
type dfButce struct {
	kalan int64
	tavan int64
}

func dfDoymaliTopla(a, b int64) int64 {
	if b > 0 && a > (1<<62)-b {
		return 1 << 62
	}
	return a + b
}

// yeniButce bayt uzunluğundaki bir girdi için bütçe açar:
// AdimTabani + AdimCarpani * bayt.
func yeniButce(bayt int) *dfButce {
	b := &dfButce{}
	b.tavan = duzIfadeAyar.Load().AdimTabani
	b.kalan = b.tavan
	b.ekle(bayt)
	return b
}

// ekle bütçeyi bayt uzunluğunda ek girdi için büyütür (AdimCarpani * bayt).
func (b *dfButce) ekle(bayt int) {
	if bayt <= 0 {
		return
	}
	a := duzIfadeAyar.Load().AdimCarpani
	n := int64(1 << 62)
	if int64(bayt) < (1<<62)/a {
		n = a * int64(bayt)
	}
	b.kalan = dfDoymaliTopla(b.kalan, n)
	b.tavan = dfDoymaliTopla(b.tavan, n)
}

func (d *DuzIfade) eslestirici(s string, b *dfButce) *dfEslestirici {
	m := &dfEslestirici{s: s, desen: dfKisalt(d.kaynak), butce: b.kalan, tavan: b.tavan, yiginS: duzIfadeAyar.Load().GeriIzlemeSiniri}
	if d.yuva > 0 {
		m.reg = make([]int, d.yuva)
	}
	return m
}

// Bul s içinde bas konumundan itibaren en soldaki eşleşmeyi verir. Arama
// AdimTabani + AdimCarpani * (kalan bayt) adımlık bir bütçe kullanır; aşılırsa
// *AdimSiniriHatasi döner.
func (d *DuzIfade) Bul(s string, bas int) (b, e int, ok bool, err error) {
	err = adimHatasiniYakala(func() {
		m := d.eslestirici(s, yeniButce(len(s)-bas))
		b, e, ok = d.bulM(m, bas)
	})
	return
}

// TumEslesmelerHata, TumEslesmeler'in hata döndüren biçimidir: adım bütçesi
// aşılırsa arama durur ve *AdimSiniriHatasi döner.
func (d *DuzIfade) TumEslesmelerHata(s string, f func(bas, son int)) error {
	return adimHatasiniYakala(func() { d.tumEslesmeler(s, f, nil) })
}

// TumEslesmeler çakışmayan bütün eşleşmeleri sırayla verir. Bir önceki
// eşleşmenin hemen ardından gelen boş eşleşme kabul edilmez (sonsuz
// döngüyü önler); bu durumda arama bir karakter ileriden sürer. Bütün
// aramalar tek bir AdimTabani + AdimCarpani * len(s) adımlık bütçeyi paylaşır;
// aşılırsa *AdimSiniriHatasi ile panikler. Hata isteyen çağıranlar
// TumEslesmelerHata'yı (parçalayıcıda Guvenli... işlevlerini) kullanmalıdır.
func (d *DuzIfade) TumEslesmeler(s string, f func(bas, son int)) {
	d.tumEslesmeler(s, f, nil)
}

// tumEslesmeler eşleşmeleri verir ve atılan adım sayısını döndürür. b nil
// değilse adımlar o bütçeden düşülür (bir kodlama çağrısının bütün
// aramaları aynı bütçeyi paylaşır); nil ise s için yeni bir bütçe açılır.
func (d *DuzIfade) tumEslesmeler(s string, f func(bas, son int), b *dfButce) int64 {
	if b == nil {
		b = yeniButce(len(s))
	}
	m := d.eslestirici(s, b)
	defer func() { b.kalan -= m.adim }()
	konum := 0
	sonEslesme := -1
	for konum <= len(s) {
		bs, e, ok := d.bulM(m, konum)
		if !ok {
			break
		}
		if bs == e && sonEslesme == e {
			if konum >= len(s) {
				break
			}
			_, w := m.sonraki(konum)
			konum += w
			continue
		}
		f(bs, e)
		konum = e
		sonEslesme = e
	}
	return m.adim
}

// bulM bas'tan itibaren ilk eşleşmeyi arar.
func (d *DuzIfade) bulM(m *dfEslestirici, bas int) (int, int, bool) {
	m.bas = bas
	for p := bas; p <= len(m.s); {
		if d.ilk != nil {
			// Desenin ilk karakteri olamayacak konumları atla.
			for p < len(m.s) {
				r, w := m.sonraki(p)
				if d.ilk.icinde(r) {
					break
				}
				p += w
				m.adim++
			}
			if p >= len(m.s) {
				break
			}
		}
		if e := m.calistir(d.prog, p, -1); e >= 0 {
			return p, e, true
		}
		if p == len(m.s) {
			break
		}
		_, w := m.sonraki(p)
		p += w
	}
	return 0, 0, false
}

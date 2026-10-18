package parca

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
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

type dfDugum interface{}

type (
	dfKarakter struct {
		r     rune
		katla []rune // büyük/küçük harf duyarsızsa eşdeğer karakterler
	}
	dfSinif struct {
		f func(rune) bool
	}
	dfNokta    struct{ satirSonuDahil bool }
	dfBirlesim struct{ ogeler []dfDugum }
	dfSecenek  struct{ secenekler []dfDugum }
	dfTekrar   struct {
		alt         dfDugum
		en, ust     int // ust < 0: sınırsız
		tembel      bool
		sahiplenici bool
	}
	dfBakma struct {
		alt     dfDugum
		geri    bool
		olumsuz bool
		uzunluk int // geriye bakmada en çok karakter sayısı
	}
	dfAtomik struct{ alt dfDugum }
	dfCapa   struct{ tur byte } // ^ $ A z Z b B
)

// DuzIfade derlenmiş bir desendir.
type DuzIfade struct {
	kaynak   string
	kok      dfDugum
	prog     *dfProgram
	yuva     int        // döngü yuvası sayısı
	ilk      *dfIlkKume // eşleşmenin ilk karakteri (nil: süzgeç yok)
	uyarilar []string
}

// dfEnCokDerinlik desenin iç içe derinlik sınırıdır (gruplar, sınıflar ve
// üst üste niceleyiciler). Derleme ve çözümleme özyinelemeli olduğundan
// sınırsız derinlik yığını taşırabilirdi.
const dfEnCokDerinlik = 1000

// DuzIfadeDerle deseni derler; desteklenmeyen bir yapı görülürse hata döner.
func DuzIfadeDerle(desen string) (*DuzIfade, error) {
	a := &dfAyristirici{s: []rune(desen)}
	kok, err := a.secenek()
	if err != nil {
		return nil, fmt.Errorf("düzenli ifade %q: %w", dfKisalt(desen), err)
	}
	if a.i < len(a.s) {
		return nil, fmt.Errorf("düzenli ifade %q: %d. konumda beklenmeyen ')'", dfKisalt(desen), a.i)
	}
	if dfAgacDerinligi(kok) > 2*dfEnCokDerinlik {
		return nil, fmt.Errorf("düzenli ifade %q: iç içe derinlik sınırı (%d) aşıldı", dfKisalt(desen), dfEnCokDerinlik)
	}
	hata, uyarilar := dfRiskCozumle(kok)
	if hata != "" && !DuzIfadeAyarlariniAl().RiskliDesenIzni {
		return nil, fmt.Errorf("düzenli ifade %q reddedildi: %s (izin için DuzIfadeAyarlari.RiskliDesenIzni)", dfKisalt(desen), hata)
	}
	if hata != "" {
		uyarilar = append([]string{hata}, uyarilar...)
	}
	c := &dfDerleyici{}
	prog := c.program(kok)
	return &DuzIfade{kaynak: desen, kok: kok, prog: prog, yuva: c.yuva, ilk: dfIlkKumeKur(kok), uyarilar: uyarilar}, nil
}

// dfKisalt hata iletileri için çok uzun desenleri kısaltır.
func dfKisalt(desen string) string {
	if len(desen) <= 200 {
		return desen
	}
	i := 200
	for i > 0 && !utf8.RuneStart(desen[i]) {
		i--
	}
	return desen[:i] + "…"
}

// dfAgacDerinligi desen ağacının derinliğini özyinelemesiz hesaplar.
func dfAgacDerinligi(kok dfDugum) int {
	type oge struct {
		d dfDugum
		n int
	}
	yigin := []oge{{kok, 1}}
	enCok := 0
	for len(yigin) > 0 {
		o := yigin[len(yigin)-1]
		yigin = yigin[:len(yigin)-1]
		if o.n > enCok {
			enCok = o.n
		}
		switch x := o.d.(type) {
		case *dfBirlesim:
			for _, c := range x.ogeler {
				yigin = append(yigin, oge{c, o.n + 1})
			}
		case *dfSecenek:
			for _, c := range x.secenekler {
				yigin = append(yigin, oge{c, o.n + 1})
			}
		case *dfTekrar:
			yigin = append(yigin, oge{x.alt, o.n + 1})
		case *dfBakma:
			yigin = append(yigin, oge{x.alt, o.n + 1})
		case *dfAtomik:
			yigin = append(yigin, oge{x.alt, o.n + 1})
		}
	}
	return enCok
}

func (d *DuzIfade) String() string { return d.kaynak }

// ---------------------------------------------------------------------------
// Ayrıştırıcı
// ---------------------------------------------------------------------------

type dfAyristirici struct {
	s          []rune
	i          int
	duyarsiz   bool
	noktaSatir bool
	derinlik   int // iç içe grup ve sınıf derinliği
}

// gir bir iç içe yapıya girer; derinlik sınırı aşılırsa hata verir.
func (a *dfAyristirici) gir() error {
	a.derinlik++
	if a.derinlik > dfEnCokDerinlik {
		return fmt.Errorf("iç içe derinlik sınırı (%d) aşıldı", dfEnCokDerinlik)
	}
	return nil
}

func (a *dfAyristirici) son() bool { return a.i >= len(a.s) }
func (a *dfAyristirici) bak() rune {
	if a.son() {
		return 0
	}
	return a.s[a.i]
}
func (a *dfAyristirici) onunde(p string) bool {
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

func (a *dfAyristirici) secenek() (dfDugum, error) {
	var secenekler []dfDugum
	for {
		d, err := a.dizi()
		if err != nil {
			return nil, err
		}
		secenekler = append(secenekler, d)
		if !a.son() && a.bak() == '|' {
			a.i++
			continue
		}
		break
	}
	if len(secenekler) == 1 {
		return secenekler[0], nil
	}
	return &dfSecenek{secenekler}, nil
}

func (a *dfAyristirici) dizi() (dfDugum, error) {
	var ogeler []dfDugum
	for !a.son() && a.bak() != '|' && a.bak() != ')' {
		d, err := a.nicelikli()
		if err != nil {
			return nil, err
		}
		if d != nil {
			ogeler = append(ogeler, d)
		}
	}
	if len(ogeler) == 1 {
		return ogeler[0], nil
	}
	return &dfBirlesim{ogeler}, nil
}

func (a *dfAyristirici) nicelikli() (dfDugum, error) {
	d, err := a.oge()
	if err != nil || d == nil {
		return d, err
	}
	for yigilan := 0; !a.son(); yigilan++ {
		if yigilan >= dfEnCokDerinlik {
			return nil, fmt.Errorf("üst üste niceleyici sınırı (%d) aşıldı", dfEnCokDerinlik)
		}
		en, ust := 0, 0
		bas := a.i
		switch a.bak() {
		case '*':
			en, ust = 0, -1
			a.i++
		case '+':
			en, ust = 1, -1
			a.i++
		case '?':
			en, ust = 0, 1
			a.i++
		case '{':
			e, u, ok := a.aralik()
			if !ok {
				return d, nil // geçersiz aralık sözdizimi: '{' düz karakterdir
			}
			en, ust = e, u
		default:
			return d, nil
		}
		t := &dfTekrar{alt: d, en: en, ust: ust}
		if !a.son() && a.bak() == '?' {
			t.tembel = true
			a.i++
		} else if !a.son() && a.bak() == '+' && a.s[bas] != '{' {
			t.sahiplenici = true
			a.i++
		}
		if _, ok := d.(*dfCapa); ok {
			return nil, fmt.Errorf("çapa nicelenemez")
		}
		d = t
	}
	return d, nil
}

// aralik "{n}", "{n,}", "{n,m}", "{,m}" sözdizimini okur.
func (a *dfAyristirici) aralik() (int, int, bool) {
	j := a.i + 1
	sayi := func() (int, bool) {
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
	en, var1 := sayi()
	ust := en
	if j < len(a.s) && a.s[j] == ',' {
		j++
		u, var2 := sayi()
		if !var1 && !var2 {
			return 0, 0, false
		}
		if var2 {
			ust = u
		} else {
			ust = -1
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
	if ust >= 0 && ust < en {
		return 0, 0, false
	}
	a.i = j + 1
	return en, ust, true
}

func (a *dfAyristirici) karakterDugumu(r rune) dfDugum {
	k := &dfKarakter{r: r}
	if a.duyarsiz {
		k.katla = katlamaYorungesi(r)
	}
	return k
}

// katlamaYorungesi büyük/küçük harf duyarsız eşleşmede r'nin eşdeğerleridir
// (başvuru uygulamasından çıkarılmış tablo; bkz. ozelliktablo.go).
func katlamaYorungesi(r rune) []rune { return ifadeKatlama[r] }

// sinifKatla köşeli parantezli bir sınıfı büyük/küçük harf duyarsız yapar:
// bir karakter, kendisi ya da bir eşdeğeri sınıftaysa eşleşir. Başvurudaki
// gibi yalnızca köşeli parantezli sınıflara ve olumsuzlamadan önce uygulanır
// ((?i)[^a-z] "A"yı da dışlar); köşeli parantezsiz \p{..}, \w, \d, \s
// katlanmaz.
func sinifKatla(g func(rune) bool) func(rune) bool {
	return func(r rune) bool {
		if g(r) {
			return true
		}
		for _, c := range ifadeKatlama[r] {
			if g(c) {
				return true
			}
		}
		return false
	}
}

func (a *dfAyristirici) oge() (dfDugum, error) {
	c := a.bak()
	switch c {
	case '(':
		return a.grup()
	case '[':
		f, olumsuz, err := a.sinifIc()
		if err != nil {
			return nil, err
		}
		if a.duyarsiz {
			f = sinifKatla(f)
		}
		if olumsuz {
			f = dfDegil(f)
		}
		return &dfSinif{f}, nil
	case '.':
		a.i++
		return &dfNokta{a.noktaSatir}, nil
	case '^':
		a.i++
		return &dfCapa{'^'}, nil
	case '$':
		a.i++
		return &dfCapa{'$'}, nil
	case '\\':
		return a.kacis()
	case '*', '+', '?':
		return nil, fmt.Errorf("%d. konumda hedefsiz niceleyici %q", a.i, c)
	}
	a.i++
	return a.karakterDugumu(c), nil
}

func (a *dfAyristirici) grup() (dfDugum, error) {
	a.i++ // '('
	defer func() { a.derinlik-- }()
	if err := a.gir(); err != nil {
		return nil, err
	}
	eskiD, eskiN := a.duyarsiz, a.noktaSatir
	defer func() { a.duyarsiz, a.noktaSatir = eskiD, eskiN }()
	var sar func(dfDugum) dfDugum = func(d dfDugum) dfDugum { return d }
	if a.bak() == '?' {
		a.i++
		switch {
		case a.onunde(":"):
			a.i++
		case a.onunde("="), a.onunde("!"):
			olumsuz := a.bak() == '!'
			a.i++
			sar = func(d dfDugum) dfDugum { return &dfBakma{alt: d, olumsuz: olumsuz} }
		case a.onunde("<="), a.onunde("<!"):
			olumsuz := a.s[a.i+1] == '!'
			a.i += 2
			var hata error
			sar = func(d dfDugum) dfDugum {
				n, sinirli := dfEnUzun(d)
				if !sinirli {
					hata = fmt.Errorf("geriye bakma içinde sınırsız uzunluk desteklenmiyor")
				}
				return &dfBakma{alt: d, geri: true, olumsuz: olumsuz, uzunluk: n}
			}
			d, err := a.secenek()
			if err != nil {
				return nil, err
			}
			if a.bak() != ')' {
				return nil, fmt.Errorf("kapanmamış grup")
			}
			a.i++
			s := sar(d)
			return s, hata
		case a.onunde(">"):
			a.i++
			sar = func(d dfDugum) dfDugum { return &dfAtomik{d} }
		case a.onunde("<"), a.onunde("'"):
			kapanis := '>'
			if a.bak() == '\'' {
				kapanis = '\''
			}
			a.i++
			for !a.son() && a.bak() != kapanis {
				a.i++
			}
			if a.son() {
				return nil, fmt.Errorf("adlı grup adı kapanmamış")
			}
			a.i++
		case a.onunde("#"):
			for !a.son() && a.bak() != ')' {
				a.i++
			}
			if a.son() {
				return nil, fmt.Errorf("yorum kapanmamış")
			}
			a.i++
			return nil, nil
		default:
			// Satır içi bayraklar: (?imx-imx) ya da (?imx-imx:...)
			acik := true
			for !a.son() && a.bak() != ')' && a.bak() != ':' {
				switch a.bak() {
				case '-':
					acik = false
				case 'i':
					a.duyarsiz = acik
				case 'm':
					a.noktaSatir = acik
				default:
					return nil, fmt.Errorf("desteklenmeyen grup ya da bayrak %q", string(a.bak()))
				}
				a.i++
			}
			if a.son() {
				return nil, fmt.Errorf("kapanmamış grup")
			}
			if a.bak() == ')' {
				// Bayrak grubun geri kalanına uygulanır: eski değere dönülmez.
				a.i++
				yD, yN := a.duyarsiz, a.noktaSatir
				eskiD, eskiN = yD, yN
				return nil, nil
			}
			a.i++ // ':'
		}
	}
	d, err := a.secenek()
	if err != nil {
		return nil, err
	}
	if a.bak() != ')' {
		return nil, fmt.Errorf("kapanmamış grup")
	}
	a.i++
	return sar(d), nil
}

// dfEnUzun düğümün eşleyebileceği en çok karakter sayısını verir.
func dfEnUzun(d dfDugum) (int, bool) {
	switch x := d.(type) {
	case nil:
		return 0, true
	case *dfKarakter, *dfSinif, *dfNokta:
		return 1, true
	case *dfCapa, *dfBakma:
		return 0, true
	case *dfAtomik:
		return dfEnUzun(x.alt)
	case *dfBirlesim:
		t := 0
		for _, o := range x.ogeler {
			n, ok := dfEnUzun(o)
			if !ok {
				return 0, false
			}
			t = dfDoymaliTop(t, n)
		}
		return t, true
	case *dfSecenek:
		m := 0
		for _, o := range x.secenekler {
			n, ok := dfEnUzun(o)
			if !ok {
				return 0, false
			}
			if n > m {
				m = n
			}
		}
		return m, true
	case *dfTekrar:
		if x.ust < 0 {
			return 0, false
		}
		n, ok := dfEnUzun(x.alt)
		return dfDoymaliCarp(n, x.ust), ok
	}
	return 0, false
}

func (a *dfAyristirici) hexOku(enAz, enCok int) (rune, error) {
	j := a.i
	for j < len(a.s) && j-a.i < enCok && strings.ContainsRune("0123456789abcdefABCDEF", a.s[j]) {
		j++
	}
	if j-a.i < enAz {
		return 0, fmt.Errorf("geçersiz onaltılık kaçış")
	}
	n, _ := strconv.ParseUint(string(a.s[a.i:j]), 16, 32)
	a.i = j
	return rune(n), nil
}

// kacisKarakteri sınıf içinde ve dışında ortak olan tek karakterlik kaçışları
// çözer; tanınmazsa ok=false.
func (a *dfAyristirici) kacisKarakteri(c rune) (rune, bool, error) {
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
		if a.bak() == '{' {
			a.i++
			r, err := a.hexOku(1, 8)
			if err != nil {
				return 0, false, err
			}
			if a.bak() != '}' {
				return 0, false, fmt.Errorf("\\x{...} kapanmamış")
			}
			a.i++
			return r, true, nil
		}
		r, err := a.hexOku(1, 2)
		return r, err == nil, err
	case 'u':
		r, err := a.hexOku(4, 4)
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

// kacisSinifi \d \s \w \h \p{..} ve olumsuzlarını çözer.
func (a *dfAyristirici) kacisSinifi(c rune) (func(rune) bool, bool, error) {
	switch c {
	case 'd':
		return dfRakam, true, nil
	case 'D':
		return dfDegil(dfRakam), true, nil
	case 's':
		return ifadeBosluk, true, nil
	case 'S':
		return dfDegil(ifadeBosluk), true, nil
	case 'w':
		return ifadeSozcuk, true, nil
	case 'W':
		return dfDegil(ifadeSozcuk), true, nil
	case 'h':
		return dfOnaltilik, true, nil
	case 'H':
		return dfDegil(dfOnaltilik), true, nil
	case 'p', 'P':
		if a.bak() != '{' {
			return nil, false, fmt.Errorf("\\%c sonrasında '{' bekleniyordu", c)
		}
		j := a.i + 1
		for j < len(a.s) && a.s[j] != '}' {
			j++
		}
		if j >= len(a.s) {
			return nil, false, fmt.Errorf("\\%c{...} kapanmamış", c)
		}
		ad := string(a.s[a.i+1 : j])
		a.i = j + 1
		olumsuz := c == 'P'
		if strings.HasPrefix(ad, "^") {
			olumsuz = !olumsuz
			ad = ad[1:]
		}
		f, err := unicodeOzelligi(ad)
		if err != nil {
			return nil, false, err
		}
		if olumsuz {
			f = dfDegil(f)
		}
		return f, true, nil
	}
	return nil, false, nil
}

func dfRakam(r rune) bool {
	if r < utf8.RuneSelf {
		return r >= '0' && r <= '9'
	}
	return tabloIfadeRakam.icinde(r)
}

func dfOnaltilik(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func dfDegil(f func(rune) bool) func(rune) bool { return func(r rune) bool { return !f(r) } }

func (a *dfAyristirici) kacis() (dfDugum, error) {
	a.i++ // '\'
	if a.son() {
		return nil, fmt.Errorf("desen ters bölüyle bitiyor")
	}
	c := a.bak()
	a.i++
	switch c {
	case 'A':
		return &dfCapa{'A'}, nil
	case 'z':
		return &dfCapa{'z'}, nil
	case 'Z':
		return &dfCapa{'Z'}, nil
	case 'b':
		return &dfCapa{'b'}, nil
	case 'B':
		return &dfCapa{'B'}, nil
	case 'G', 'K', 'X', 'R', 'k', 'g', 'y', 'Y', 'N', 'O':
		return nil, fmt.Errorf("\\%c kaçışı desteklenmiyor", c)
	}
	if c >= '1' && c <= '9' {
		return nil, fmt.Errorf("geri başvuru (\\%c) desteklenmiyor", c)
	}
	if f, ok, err := a.kacisSinifi(c); err != nil {
		return nil, err
	} else if ok {
		return &dfSinif{f}, nil // köşeli parantezsiz sınıflar (?i) ile katlanmaz
	}
	if r, ok, err := a.kacisKarakteri(c); err != nil {
		return nil, err
	} else if ok {
		return a.karakterDugumu(r), nil
	}
	return a.karakterDugumu(c), nil
}

// sinif "[...]" karakter sınıfını (olumsuzlaması uygulanmış olarak) ayrıştırır.
func (a *dfAyristirici) sinif() (func(rune) bool, error) {
	f, olumsuz, err := a.sinifIc()
	if err == nil && olumsuz {
		f = dfDegil(f)
	}
	return f, err
}

// sinifIc "[...]" karakter sınıfının olumsuzlanmamış kümesini ve olumsuz
// olup olmadığını verir.
func (a *dfAyristirici) sinifIc() (func(rune) bool, bool, error) {
	a.i++ // '['
	defer func() { a.derinlik-- }()
	if err := a.gir(); err != nil {
		return nil, false, err
	}
	olumsuz := false
	if a.bak() == '^' {
		olumsuz = true
		a.i++
	}
	var parcalar []func(rune) bool
	var kesisim []func(rune) bool
	ilk := true
	for {
		if a.son() {
			return nil, false, fmt.Errorf("karakter sınıfı kapanmamış")
		}
		c := a.bak()
		if c == ']' && !ilk {
			a.i++
			break
		}
		ilk = false
		if a.onunde("&&") {
			a.i += 2
			kesisim = append(kesisim, dfBirlesik(parcalar))
			parcalar = nil
			continue
		}
		if a.onunde("[:") {
			if f, ok := a.posixSinif(); ok {
				parcalar = append(parcalar, f)
				continue
			}
		}
		if c == '[' {
			f, err := a.sinif()
			if err != nil {
				return nil, false, err
			}
			parcalar = append(parcalar, f)
			continue
		}
		bas, f, err := a.sinifOgesi()
		if err != nil {
			return nil, false, err
		}
		if f != nil {
			parcalar = append(parcalar, f)
			continue
		}
		// Aralık mı?
		if a.bak() == '-' && a.i+1 < len(a.s) && a.s[a.i+1] != ']' {
			kayit := a.i
			a.i++
			bit, f2, err := a.sinifOgesi()
			if err != nil {
				return nil, false, err
			}
			if f2 != nil {
				// "a-\d" gibi: '-' düz karakterdir.
				a.i = kayit
				parcalar = append(parcalar, dfTek(bas))
				continue
			}
			if bit < bas {
				return nil, false, fmt.Errorf("geçersiz karakter aralığı")
			}
			lo, hi := bas, bit
			parcalar = append(parcalar, func(r rune) bool { return r >= lo && r <= hi })
			continue
		}
		parcalar = append(parcalar, dfTek(bas))
	}
	f := dfBirlesik(parcalar)
	if len(kesisim) > 0 {
		hepsi := append(kesisim, f)
		f = func(r rune) bool {
			for _, g := range hepsi {
				if !g(r) {
					return false
				}
			}
			return true
		}
	}
	return f, olumsuz, nil
}

func dfTek(c rune) func(rune) bool { return func(r rune) bool { return r == c } }

func dfBirlesik(p []func(rune) bool) func(rune) bool {
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

// sinifOgesi sınıf içindeki tek bir öğeyi okur: ya bir karakter ya bir alt sınıf.
func (a *dfAyristirici) sinifOgesi() (rune, func(rune) bool, error) {
	c := a.bak()
	a.i++
	if c != '\\' {
		return c, nil, nil
	}
	if a.son() {
		return 0, nil, fmt.Errorf("sınıf ters bölüyle bitiyor")
	}
	c = a.bak()
	a.i++
	if c == 'b' {
		return '\b', nil, nil
	}
	if f, ok, err := a.kacisSinifi(c); err != nil {
		return 0, nil, err
	} else if ok {
		return 0, f, nil
	}
	if r, ok, err := a.kacisKarakteri(c); err != nil {
		return 0, nil, err
	} else if ok {
		return r, nil, nil
	}
	return c, nil, nil
}

func (a *dfAyristirici) posixSinif() (func(rune) bool, bool) {
	j := a.i + 2
	olumsuz := false
	if j < len(a.s) && a.s[j] == '^' {
		olumsuz = true
		j++
	}
	k := j
	for k < len(a.s) && a.s[k] >= 'a' && a.s[k] <= 'z' {
		k++
	}
	if k+1 >= len(a.s) || a.s[k] != ':' || a.s[k+1] != ']' {
		return nil, false
	}
	f, ok := dfPosixler()[string(a.s[j:k])]
	if !ok {
		return nil, false
	}
	a.i = k + 2
	if olumsuz {
		f = dfDegil(f)
	}
	return f, true
}

// unicodeOzelligi \p{ad} için sınama işlevini verir. Ad büyük/küçük harf,
// boşluk, '_' ve '-' farkı gözetmeden eşlenir.
func unicodeOzelligi(ad string) (func(rune) bool, error) {
	n := strings.Map(func(r rune) rune {
		if r == ' ' || r == '_' || r == '-' {
			return -1
		}
		return unicode.ToLower(r)
	}, ad)
	if f, ok := dfOzellikler()[n]; ok {
		return f, nil
	}
	return nil, fmt.Errorf("bilinmeyen Unicode özelliği %q", ad)
}

var (
	dfOzellikBir     sync.Once
	dfOzellikTablosu map[string]func(rune) bool // \p{ad}
	dfPosixTablosu   map[string]func(rune) bool // [[:ad:]]
)

func dfOzellikler() map[string]func(rune) bool {
	dfOzellikBir.Do(dfOzellikKur)
	return dfOzellikTablosu
}

func dfPosixler() map[string]func(rune) bool {
	dfOzellikBir.Do(dfOzellikKur)
	return dfPosixTablosu
}

// dfOzellikKur sınıf tablolarını kurar. Bütün tablolar (genel kategoriler,
// yazı sistemleri, ikili özellikler, POSIX sınıfları) başvuru uygulamasının
// düzenli ifade motorundan çıkarılmıştır (karaktertablo.go, ozelliktablo.go):
// Unicode sürümü ve sınıf tanımları (ör. [[:punct:]] = P ve ASCII simgeleri)
// onunla aynıdır; başvurunun tanımadığı adlar burada da tanınmaz.
func dfOzellikKur() {
	m := make(map[string]func(rune) bool, len(ifadeKategori)+len(ifadeOzellikKategori)+len(ifadeOzellikAdlari))
	for ad, t := range ifadeKategori {
		if ad != "bosluk" { // \s için iç tablo; bir özellik adı değil
			m[ad] = t.icinde
		}
	}
	for ad, k := range ifadeOzellikKategori {
		m[ad] = ifadeKategori[k].icinde
	}
	for ad, i := range ifadeOzellikAdlari {
		m[ad] = ifadeOzellikTablolari[i].icinde
	}
	p := make(map[string]func(rune) bool, len(ifadePosixAdlari)+len(ifadePosixKategori))
	for ad, k := range ifadePosixKategori {
		p[ad] = ifadeKategori[k].icinde
	}
	for ad, i := range ifadePosixAdlari {
		p[ad] = ifadeOzellikTablolari[i].icinde
	}
	dfOzellikTablosu, dfPosixTablosu = m, p
}

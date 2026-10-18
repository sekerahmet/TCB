package parcalayici

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
	uyarilar []string
}

// DuzIfadeDerle deseni derler; desteklenmeyen bir yapı görülürse hata döner.
func DuzIfadeDerle(desen string) (*DuzIfade, error) {
	a := &dfAyristirici{s: []rune(desen)}
	kok, err := a.secenek()
	if err != nil {
		return nil, fmt.Errorf("düzenli ifade %q: %w", desen, err)
	}
	if a.i < len(a.s) {
		return nil, fmt.Errorf("düzenli ifade %q: %d. konumda beklenmeyen ')'", desen, a.i)
	}
	hata, uyarilar := dfRiskCozumle(kok)
	if hata != "" && !DuzIfadeAyarlariniAl().RiskliDesenIzni {
		return nil, fmt.Errorf("düzenli ifade %q reddedildi: %s (izin için DuzIfadeAyarlari.RiskliDesenIzni)", desen, hata)
	}
	if hata != "" {
		uyarilar = append([]string{hata}, uyarilar...)
	}
	return &DuzIfade{kaynak: desen, kok: kok, uyarilar: uyarilar}, nil
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
	for !a.son() {
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

func (a *dfAyristirici) sinifDugumu(f func(rune) bool) dfDugum {
	if a.duyarsiz {
		g := f
		f = func(r rune) bool {
			if g(r) {
				return true
			}
			for c := unicode.SimpleFold(r); c != r; c = unicode.SimpleFold(c) {
				if g(c) {
					return true
				}
			}
			return false
		}
	}
	return &dfSinif{f}
}

func katlamaYorungesi(r rune) []rune {
	var out []rune
	for c := unicode.SimpleFold(r); c != r; c = unicode.SimpleFold(c) {
		out = append(out, c)
	}
	return out
}

func (a *dfAyristirici) oge() (dfDugum, error) {
	c := a.bak()
	switch c {
	case '(':
		return a.grup()
	case '[':
		f, err := a.sinif()
		if err != nil {
			return nil, err
		}
		return a.sinifDugumu(f), nil
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
			t += n
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
		return n * x.ust, ok
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
		return a.sinifDugumu(f), nil
	}
	if r, ok, err := a.kacisKarakteri(c); err != nil {
		return nil, err
	} else if ok {
		return a.karakterDugumu(r), nil
	}
	return a.karakterDugumu(c), nil
}

// sinif "[...]" karakter sınıfını ayrıştırır.
func (a *dfAyristirici) sinif() (func(rune) bool, error) {
	a.i++ // '['
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
			return nil, fmt.Errorf("karakter sınıfı kapanmamış")
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
				return nil, err
			}
			parcalar = append(parcalar, f)
			continue
		}
		bas, f, err := a.sinifOgesi()
		if err != nil {
			return nil, err
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
				return nil, err
			}
			if f2 != nil {
				// "a-\d" gibi: '-' düz karakterdir.
				a.i = kayit
				parcalar = append(parcalar, dfTek(bas))
				continue
			}
			if bit < bas {
				return nil, fmt.Errorf("geçersiz karakter aralığı")
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
	if olumsuz {
		f = dfDegil(f)
	}
	return f, nil
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
	f, err := unicodeOzelligi(string(a.s[j:k]))
	if err != nil {
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
	dfOzellikTablosu map[string]func(rune) bool
)

func dfOzellikler() map[string]func(rune) bool {
	dfOzellikBir.Do(dfOzellikKur)
	return dfOzellikTablosu
}

func dfOzellikKur() {
	m := make(map[string]func(rune) bool)
	tablo := func(t *unicode.RangeTable) func(rune) bool { return func(r rune) bool { return unicode.Is(t, r) } }
	for ad, t := range unicode.Categories {
		m[strings.ToLower(ad)] = tablo(t)
	}
	for ad, t := range unicode.Scripts {
		m[strings.ToLower(strings.ReplaceAll(ad, "_", ""))] = tablo(t)
	}
	for ad, t := range unicode.Properties {
		m[strings.ToLower(strings.ReplaceAll(ad, "_", ""))] = tablo(t)
	}
	atanmis := func(r rune) bool {
		for _, t := range []*unicode.RangeTable{unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Z, unicode.C} {
			if unicode.Is(t, r) {
				return true
			}
		}
		return false
	}
	m["cn"] = dfDegil(atanmis)
	m["c"] = func(r rune) bool { return unicode.Is(unicode.C, r) || !atanmis(r) }
	// Genel kategoriler için başvuru uygulamasından çıkarılmış tablolar
	// (Go'nun tablolarından daha yeni Unicode sürümünü yansıtabilir).
	for ad, t := range ifadeKategori {
		m[ad] = t.icinde
	}
	m["unassigned"] = m["cn"]
	m["assigned"] = dfDegil(m["cn"])
	m["other"] = m["c"]
	m["any"] = func(rune) bool { return true }
	uzun := map[string]string{
		"letter": "l", "uppercaseletter": "lu", "lowercaseletter": "ll", "titlecaseletter": "lt",
		"modifierletter": "lm", "otherletter": "lo", "mark": "m", "combiningmark": "m", "nonspacingmark": "mn",
		"spacingmark": "mc", "enclosingmark": "me", "number": "n", "decimalnumber": "nd", "letternumber": "nl",
		"othernumber": "no", "punctuation": "p", "connectorpunctuation": "pc", "dashpunctuation": "pd",
		"openpunctuation": "ps", "closepunctuation": "pe", "initialpunctuation": "pi", "finalpunctuation": "pf",
		"otherpunctuation": "po", "symbol": "s", "mathsymbol": "sm", "currencysymbol": "sc",
		"modifiersymbol": "sk", "othersymbol": "so", "separator": "z", "spaceseparator": "zs",
		"lineseparator": "zl", "paragraphseparator": "zp", "control": "cc", "cntrl": "cc", "format": "cf",
		"privateuse": "co", "surrogate": "cs",
	}
	for u, k := range uzun {
		m[u] = m[k]
	}
	m["lc"] = func(r rune) bool { return unicode.In(r, unicode.Lu, unicode.Ll, unicode.Lt) }
	m["l&"] = m["lc"]
	m["alphabetic"] = func(r rune) bool {
		return unicode.IsLetter(r) || unicode.In(r, unicode.Nl, unicode.Other_Alphabetic)
	}
	m["alpha"] = m["alphabetic"]
	m["digit"] = dfRakam
	m["alnum"] = func(r rune) bool { return m["alphabetic"](r) || dfRakam(r) }
	m["space"] = unicode.IsSpace
	m["whitespace"] = unicode.IsSpace
	m["word"] = ifadeSozcuk
	m["upper"] = func(r rune) bool { return unicode.In(r, unicode.Lu, unicode.Other_Uppercase) }
	m["uppercase"] = m["upper"]
	m["lower"] = func(r rune) bool { return unicode.In(r, unicode.Ll, unicode.Other_Lowercase) }
	m["lowercase"] = m["lower"]
	m["punct"] = tablo(unicode.P)
	m["xdigit"] = dfOnaltilik
	m["blank"] = func(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }
	m["graph"] = func(r rune) bool { return !unicode.IsSpace(r) && !unicode.Is(unicode.C, r) && atanmis(r) }
	m["print"] = func(r rune) bool { return m["graph"](r) || unicode.Is(unicode.Zs, r) }
	m["ascii"] = func(r rune) bool { return r < 128 }
	dfOzellikTablosu = m
}

// ---------------------------------------------------------------------------
// Eşleştirici
// ---------------------------------------------------------------------------

type dfEslestirici struct {
	s      string
	desen  string
	adim   int64 // geçerli aramada atılan adım
	butce  int64 // geçerli aramanın adım bütçesi
	bas    int   // geçerli aramanın başlangıcı (hata iletisi için)
	toplam int64 // bütün aramalarda atılan adım (ölçüm için)
}

// say n adımı bütçeden düşer; bütçe aşılırsa AdimSiniriHatasi ile panikler
// (dışa açık işlevlerde hataya çevrilir).
func (m *dfEslestirici) say(n int64) {
	m.adim += n
	if m.adim > m.butce {
		panic(&AdimSiniriHatasi{Desen: m.desen, Konum: m.bas, Butce: m.butce})
	}
}

func (m *dfEslestirici) onceki(pos int) (rune, int) {
	if pos <= 0 {
		return -1, 0
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

// tekKarakter düğüm tek karakter eşleyen türdense pos'taki karakteri sınar.
func (m *dfEslestirici) tekKarakter(d dfDugum, pos int) (int, bool, bool) {
	switch x := d.(type) {
	case *dfKarakter:
		r, w := m.sonraki(pos)
		if w == 0 {
			return 0, false, true
		}
		if r == x.r {
			return pos + w, true, true
		}
		for _, c := range x.katla {
			if r == c {
				return pos + w, true, true
			}
		}
		return 0, false, true
	case *dfSinif:
		r, w := m.sonraki(pos)
		if w == 0 || !x.f(r) {
			return 0, false, true
		}
		return pos + w, true, true
	case *dfNokta:
		r, w := m.sonraki(pos)
		if w == 0 || (r == '\n' && !x.satirSonuDahil) {
			return 0, false, true
		}
		return pos + w, true, true
	}
	return 0, false, false
}

// esle d düğümünü pos'ta eşlemeye çalışır; her olası bitişte k çağrılır.
func (m *dfEslestirici) esle(d dfDugum, pos int, k func(int) bool) bool {
	m.say(1)
	if p, ok, tek := m.tekKarakter(d, pos); tek {
		return ok && k(p)
	}
	switch x := d.(type) {
	case nil:
		return k(pos)
	case *dfBirlesim:
		return m.esleDizi(x.ogeler, pos, k)
	case *dfSecenek:
		for _, s := range x.secenekler {
			if m.esle(s, pos, k) {
				return true
			}
		}
		return false
	case *dfTekrar:
		if x.sahiplenici {
			son := -1
			m.tekrar(x, 0, pos, func(p int) bool { son = p; return true })
			return son >= 0 && k(son)
		}
		return m.tekrar(x, 0, pos, k)
	case *dfAtomik:
		son := -1
		m.esle(x.alt, pos, func(p int) bool { son = p; return true })
		return son >= 0 && k(son)
	case *dfBakma:
		var tuttu bool
		if x.geri {
			bas := pos
			for n := 0; ; n++ {
				if m.esle(x.alt, bas, func(p int) bool { return p == pos }) {
					tuttu = true
					break
				}
				if n >= x.uzunluk || bas == 0 {
					break
				}
				_, w := m.onceki(bas)
				bas -= w
			}
		} else {
			tuttu = m.esle(x.alt, pos, func(int) bool { return true })
		}
		if tuttu == x.olumsuz {
			return false
		}
		return k(pos)
	case *dfCapa:
		if m.capa(x.tur, pos) {
			return k(pos)
		}
		return false
	}
	panic("bilinmeyen düzenli ifade düğümü")
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

func (m *dfEslestirici) esleDizi(ogeler []dfDugum, pos int, k func(int) bool) bool {
	if len(ogeler) == 0 {
		return k(pos)
	}
	if len(ogeler) == 1 {
		return m.esle(ogeler[0], pos, k)
	}
	// Tek karakterlik öğeleri döngüyle geç (özyineleme ve kapanış maliyeti olmadan).
	for len(ogeler) > 1 {
		p, ok, tek := m.tekKarakter(ogeler[0], pos)
		if !tek {
			break
		}
		if !ok {
			return false
		}
		m.say(1)
		pos = p
		ogeler = ogeler[1:]
	}
	if len(ogeler) == 1 {
		return m.esle(ogeler[0], pos, k)
	}
	return m.esle(ogeler[0], pos, func(p int) bool { return m.esleDizi(ogeler[1:], p, k) })
}

func (m *dfEslestirici) tekrar(x *dfTekrar, sayi, pos int, k func(int) bool) bool {
	// Tek karakterlik alt düğüm: konumları döngüyle topla.
	if _, _, tek := m.tekKarakter(x.alt, pos); tek {
		var yerel [32]int
		konumlar := append(yerel[:0], pos)
		p := pos
		for x.ust < 0 || len(konumlar)-1 < x.ust {
			q, ok, _ := m.tekKarakter(x.alt, p)
			if !ok {
				break
			}
			p = q
			konumlar = append(konumlar, p)
		}
		m.say(int64(len(konumlar)))
		if len(konumlar)-1 < x.en {
			return false
		}
		if x.tembel {
			for i := x.en; i < len(konumlar); i++ {
				m.say(1)
				if k(konumlar[i]) {
					return true
				}
			}
			return false
		}
		for i := len(konumlar) - 1; i >= x.en; i-- {
			m.say(1)
			if k(konumlar[i]) {
				return true
			}
		}
		return false
	}
	devam := x.ust < 0 || sayi < x.ust
	dene := func() bool {
		return devam && m.esle(x.alt, pos, func(p int) bool {
			if p == pos && sayi >= x.en {
				return false // boş yineleme sonsuz döngüye girmesin
			}
			return m.tekrar(x, sayi+1, p, k)
		})
	}
	if x.tembel {
		if sayi >= x.en && k(pos) {
			return true
		}
		return dene()
	}
	if dene() {
		return true
	}
	return sayi >= x.en && k(pos)
}

// esleKonumda pos'ta başlayan ilk eşleşmenin bitişini verir.
func (d *DuzIfade) esleKonumda(m *dfEslestirici, pos int) (int, bool) {
	son := -1
	m.esle(d.kok, pos, func(p int) bool { son = p; return true })
	return son, son >= 0
}

// Bul s içinde bas konumundan itibaren en soldaki eşleşmeyi verir. Adım
// bütçesi aşılırsa *AdimSiniriHatasi döner.
func (d *DuzIfade) Bul(s string, bas int) (b, e int, ok bool, err error) {
	err = adimHatasiniYakala(func() {
		b, e, ok = d.bulM(&dfEslestirici{s: s, desen: d.kaynak}, bas)
	})
	return
}

// TumEslesmelerHata, TumEslesmeler'in hata döndüren biçimidir: adım bütçesi
// aşılırsa arama durur ve *AdimSiniriHatasi döner.
func (d *DuzIfade) TumEslesmelerHata(s string, f func(bas, son int)) error {
	return adimHatasiniYakala(func() { d.tumEslesmeler(s, f) })
}

// TumEslesmeler çakışmayan bütün eşleşmeleri sırayla verir. Bir önceki
// eşleşmenin hemen ardından gelen boş eşleşme kabul edilmez (sonsuz
// döngüyü önler); bu durumda arama bir karakter ileriden sürer. Adım bütçesi
// aşılırsa *AdimSiniriHatasi ile panikler; hata isteyen çağıranlar
// TumEslesmelerHata'yı (parçalayıcıda Guvenli... işlevlerini) kullanmalıdır.
func (d *DuzIfade) TumEslesmeler(s string, f func(bas, son int)) {
	d.tumEslesmeler(s, f)
}

// tumEslesmeler eşleşmeleri verir ve bütün aramalarda atılan adım sayısını döndürür.
func (d *DuzIfade) tumEslesmeler(s string, f func(bas, son int)) int64 {
	m := &dfEslestirici{s: s, desen: d.kaynak}
	konum := 0
	sonEslesme := -1
	for konum <= len(s) {
		b, e, ok := d.bulM(m, konum)
		if !ok {
			break
		}
		if b == e && sonEslesme == e {
			if konum >= len(s) {
				break
			}
			_, w := m.sonraki(konum)
			konum += w
			continue
		}
		f(b, e)
		konum = e
		sonEslesme = e
	}
	return m.toplam
}

// bulM bas'tan itibaren ilk eşleşmeyi arar. Arama (bütün başlangıç konumları
// dahil) tek bir adım bütçesi kullanır: AdimTabani + AdimCarpani * kalan bayt.
func (d *DuzIfade) bulM(m *dfEslestirici, bas int) (int, int, bool) {
	ayar := duzIfadeAyar.Load()
	m.adim, m.bas = 0, bas
	m.butce = ayar.AdimTabani + ayar.AdimCarpani*int64(len(m.s)-bas)
	defer func() { m.toplam += m.adim }()
	for p := bas; p <= len(m.s); {
		if e, ok := d.esleKonumda(m, p); ok {
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

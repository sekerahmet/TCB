package tokenize

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ============================================================================
// Ön bölücüler
// ============================================================================

func init() {
	OnBolucuKaydet("Sequence", sirOnBolKur)
	OnBolucuKaydet("Split", bolKur)
	OnBolucuKaydet("ByteLevel", baytOnBolKur)
	OnBolucuKaydet("Whitespace", func(json.RawMessage) (OnBolucu, error) { return bosluklaBol{}, nil })
	OnBolucuKaydet("WhitespaceSplit", func(json.RawMessage) (OnBolucu, error) {
		return karakterleBol{kosul: unicode.IsSpace, davranis: davranisSil}, nil
	})
	OnBolucuKaydet("Metaspace", metaspaceKur)
	OnBolucuKaydet("BertPreTokenizer", func(json.RawMessage) (OnBolucu, error) {
		return OnBolucuSirasi{
			karakterleBol{kosul: unicode.IsSpace, davranis: davranisSil},
			karakterleBol{kosul: noktalamaMi, davranis: davranisAyri},
		}, nil
	})
	OnBolucuKaydet("Punctuation", func(ham json.RawMessage) (OnBolucu, error) {
		var t struct {
			Behavior *string `json:"behavior"`
		}
		if err := katiCoz(ham, &t); err != nil {
			return nil, err
		}
		d := davranisAyri
		if t.Behavior != nil {
			var err error
			if d, err = davranisOku(*t.Behavior); err != nil {
				return nil, err
			}
		}
		return karakterleBol{kosul: noktalamaMi, davranis: d}, nil
	})
	OnBolucuKaydet("Digits", func(ham json.RawMessage) (OnBolucu, error) {
		var t struct {
			Tek bool `json:"individual_digits"`
		}
		if err := katiCoz(ham, &t); err != nil {
			return nil, err
		}
		d := davranisBitisik
		if t.Tek {
			d = davranisAyri
		}
		return karakterleBol{kosul: tabloRakam.icinde, davranis: d}, nil
	})
	OnBolucuKaydet("CharDelimiterSplit", func(ham json.RawMessage) (OnBolucu, error) {
		var t struct {
			Delimiter string `json:"delimiter"`
		}
		if err := katiCoz(ham, &t); err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(t.Delimiter) != 1 {
			return nil, fmt.Errorf("ayırıcı tek karakter olmalı: %q", t.Delimiter)
		}
		c, _ := utf8.DecodeRuneInString(t.Delimiter)
		return karakterleBol{kosul: func(r rune) bool { return r == c }, davranis: davranisSil}, nil
	})
}

// noktalamaMi ASCII noktalama ya da Unicode P kategorisi.
func noktalamaMi(r rune) bool {
	if r < utf8.RuneSelf {
		return (r >= '!' && r <= '/') || (r >= ':' && r <= '@') || (r >= '[' && r <= '`') || (r >= '{' && r <= '~')
	}
	return tabloBertNoktalama.icinde(r)
}

// OnBolucuSirasi ön bölücüleri sırayla uygular.
type OnBolucuSirasi []OnBolucu

func (s OnBolucuSirasi) OnBol(dst, parcalar []Parca) []Parca {
	for _, p := range parcalar {
		s.akis(p, func(x Parca) { dst = append(dst, x) })
	}
	return dst
}

// akis zinciri derinlik öncelikli uygular: her parça, bir sonraki adıma
// hemen aktarılır; ara dilimler kurulmaz (büyük belgelerde bellek dostu).
func (s OnBolucuSirasi) akis(p Parca, f func(Parca)) {
	adim := make([]func(Parca), len(s)+1)
	adim[len(s)] = f
	for k := len(s) - 1; k >= 0; k-- {
		o, sonraki := s[k], adim[k+1]
		adim[k] = func(x Parca) { onBolAkis(o, x, sonraki) }
	}
	adim[0](p)
}

// akisOnBolucu parçaları dilim kurmadan tek tek üretebilen ön bölücüdür.
type akisOnBolucu interface {
	akis(p Parca, f func(Parca))
}

// onBolAkis o'yu p'ye uygular ve her sonuç parçası için f'yi çağırır.
func onBolAkis(o OnBolucu, p Parca, f func(Parca)) {
	if a, ok := o.(akisOnBolucu); ok {
		a.akis(p, f)
		return
	}
	for _, x := range o.OnBol(nil, []Parca{p}) {
		f(x)
	}
}

func sirOnBolKur(ham json.RawMessage) (OnBolucu, error) {
	var t struct {
		Pretokenizers []json.RawMessage `json:"pretokenizers"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	var s OnBolucuSirasi
	for i, h := range t.Pretokenizers {
		o, err := OnBolucuKur(h)
		if err != nil {
			return nil, fmt.Errorf("%d. adım: %w", i, err)
		}
		if o != nil {
			s = append(s, o)
		}
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Bölme davranışları
// ---------------------------------------------------------------------------

type bolmeDavranisi int

const (
	davranisSil bolmeDavranisi = iota
	davranisAyri
	davranisOncekiyle
	davranisSonrakiyle
	davranisBitisik
)

func davranisOku(s string) (bolmeDavranisi, error) {
	switch s {
	case "Removed":
		return davranisSil, nil
	case "Isolated":
		return davranisAyri, nil
	case "MergedWithPrevious":
		return davranisOncekiyle, nil
	case "MergedWithNext":
		return davranisSonrakiyle, nil
	case "Contiguous":
		return davranisBitisik, nil
	}
	return 0, fmt.Errorf("bilinmeyen bölme davranışı %q", s)
}

// aralik parçanın içindeki bir bölümdür; esles bölümün desene uyup uymadığıdır.
type aralik struct {
	bas, son int
	esles    bool
}

// davranisUygula eşleşme listesini davranışa göre birleştirip parçaları üretir.
func davranisUygula(dst []Parca, p Parca, metin string, a []aralik, d bolmeDavranisi) []Parca {
	type sonuc struct {
		bas, son int
		sil      bool
	}
	var s []sonuc
	switch d {
	case davranisAyri:
		for _, x := range a {
			s = append(s, sonuc{x.bas, x.son, false})
		}
	case davranisSil:
		for _, x := range a {
			s = append(s, sonuc{x.bas, x.son, x.esles})
		}
	case davranisBitisik:
		onceki := false
		for _, x := range a {
			if x.esles == onceki && len(s) > 0 {
				s[len(s)-1].son = x.son
			} else {
				s = append(s, sonuc{x.bas, x.son, false})
			}
			onceki = x.esles
		}
	case davranisOncekiyle:
		onceki := false
		for _, x := range a {
			if x.esles && !onceki && len(s) > 0 {
				s[len(s)-1].son = x.son
			} else {
				s = append(s, sonuc{x.bas, x.son, false})
			}
			onceki = x.esles
		}
	case davranisSonrakiyle:
		onceki := false
		var ters []sonuc
		for i := len(a) - 1; i >= 0; i-- {
			x := a[i]
			if x.esles && !onceki && len(ters) > 0 {
				ters[len(ters)-1].bas = x.bas
			} else {
				ters = append(ters, sonuc{x.bas, x.son, false})
			}
			onceki = x.esles
		}
		for i := len(ters) - 1; i >= 0; i-- {
			s = append(s, ters[i])
		}
	}
	for _, x := range s {
		if x.sil || x.son <= x.bas {
			continue
		}
		dst = append(dst, Parca{Metin: metin[x.bas:x.son], Ilk: p.Ilk && x.bas == 0, butce: p.butce})
	}
	return dst
}

// eslesmeAraliklari eşleşmeleri, aralarındaki eşleşmeyen bölümlerle birlikte listeler.
func eslesmeAraliklari(n int, bul func(f func(b, e int))) []aralik {
	var a []aralik
	onceki := 0
	bul(func(b, e int) {
		if onceki != b {
			a = append(a, aralik{onceki, b, false})
		}
		a = append(a, aralik{b, e, true})
		onceki = e
	})
	if onceki != n {
		a = append(a, aralik{onceki, n, false})
	}
	return a
}

func tersCevir(a []aralik) {
	for i := range a {
		a[i].esles = !a[i].esles
	}
}

// ---------------------------------------------------------------------------
// Split
// ---------------------------------------------------------------------------

type desenleBol struct {
	ifade    *DuzIfade
	duz      *string
	hizli    bool // desen BolmeDeseni: elle yazılmış eşleştirici kullanılır
	davranis bolmeDavranisi
	ters     bool
}

func bolKur(ham json.RawMessage) (OnBolucu, error) {
	var t struct {
		Pattern  desenBicimi `json:"pattern"`
		Behavior string      `json:"behavior"`
		Invert   bool        `json:"invert"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	d, err := davranisOku(t.Behavior)
	if err != nil {
		return nil, err
	}
	b := &desenleBol{davranis: d, ters: t.Invert}
	switch {
	case t.Pattern.String != nil:
		b.duz = t.Pattern.String
	case t.Pattern.Regex != nil:
		if *t.Pattern.Regex == BolmeDeseni {
			b.hizli = true
		} else {
			ifade, err := DuzIfadeDerle(*t.Pattern.Regex)
			if err != nil {
				return nil, err
			}
			b.ifade = ifade
		}
	default:
		return nil, fmt.Errorf("desen (pattern) String ya da Regex olmalı")
	}
	return b, nil
}

func (b *desenleBol) bul(metin string, f func(bas, son int), butce *dfButce) {
	switch {
	case b.hizli:
		for i := 0; i < len(metin); {
			j := eslesmeSonu(metin, i)
			f(i, j)
			i = j
		}
	case b.duz != nil:
		d := *b.duz
		if d == "" {
			return
		}
		for i := 0; ; {
			k := strings.Index(metin[i:], d)
			if k < 0 {
				return
			}
			f(i+k, i+k+len(d))
			i += k + len(d)
		}
	default:
		b.ifade.tumEslesmeler(metin, f, butce)
	}
}

func (b *desenleBol) OnBol(dst, parcalar []Parca) []Parca {
	for _, p := range parcalar {
		b.akis(p, func(x Parca) { dst = append(dst, x) })
	}
	return dst
}

func (b *desenleBol) akis(p Parca, f func(Parca)) {
	metin := p.Icerik()
	// Hızlı yol: elle yazılmış desen her karakteri kapsar, "Isolated"
	// davranışında her eşleşme doğrudan bir parçadır.
	if b.hizli && b.davranis == davranisAyri {
		for i := 0; i < len(metin); {
			j := eslesmeSonu(metin, i)
			f(Parca{Metin: metin[i:j], Ilk: p.Ilk && i == 0, butce: p.butce})
			i = j
		}
		return
	}
	a := eslesmeAraliklari(len(metin), func(f func(b, e int)) { b.bul(metin, f, p.butce) })
	if b.ters {
		tersCevir(a)
	}
	for _, x := range davranisUygula(nil, p, metin, a, b.davranis) {
		f(x)
	}
}

// karakterleBol her uyan karakteri ayrı bir eşleşme sayarak böler.
type karakterleBol struct {
	kosul    func(rune) bool
	davranis bolmeDavranisi
}

func (k karakterleBol) OnBol(dst, parcalar []Parca) []Parca {
	for _, p := range parcalar {
		metin := p.Icerik()
		a := eslesmeAraliklari(len(metin), func(f func(b, e int)) {
			for i, r := range metin {
				if k.kosul(r) {
					f(i, i+utf8.RuneLen(r))
				}
			}
		})
		dst = davranisUygula(dst, p, metin, a, k.davranis)
	}
	return dst
}

// bosluklaBol: "\w+|[^\w\s]+" eşleşmelerini tutar, aradaki boşlukları atar.
type bosluklaBol struct{}

func (bosluklaBol) OnBol(dst, parcalar []Parca) []Parca {
	for _, p := range parcalar {
		m := p.Icerik()
		for i := 0; i < len(m); {
			r, w := utf8.DecodeRuneInString(m[i:])
			var j int
			switch {
			case sozcukKarakteri(r):
				j = i + w
				for j < len(m) {
					r2, w2 := utf8.DecodeRuneInString(m[j:])
					if !sozcukKarakteri(r2) {
						break
					}
					j += w2
				}
			case unicode.IsSpace(r):
				i += w
				continue
			default:
				j = i + w
				for j < len(m) {
					r2, w2 := utf8.DecodeRuneInString(m[j:])
					if sozcukKarakteri(r2) || unicode.IsSpace(r2) {
						break
					}
					j += w2
				}
			}
			dst = append(dst, Parca{Metin: m[i:j], Ilk: p.Ilk && i == 0, butce: p.butce})
			i = j
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// ByteLevel
// ---------------------------------------------------------------------------

// BaytDuzeyiDeseni bayt düzeyi ön bölücünün use_regex=true iken kullandığı desendir.
const BaytDuzeyiDeseni = `'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`

var baytDuzeyiIfade *DuzIfade

type baytOnBol struct {
	onekBosluk bool
	desenle    bool
	// tembel doğruysa bayt -> karakter dönüşümü yapılmaz, parça işaretlenir
	// (yalnızca zincirin son adımıyken güvenlidir; kurulumda ayarlanır).
	tembel bool
}

func baytOnBolKur(ham json.RawMessage) (OnBolucu, error) {
	t := struct {
		AddPrefixSpace *bool `json:"add_prefix_space"`
		UseRegex       *bool `json:"use_regex"`
	}{}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	b := &baytOnBol{onekBosluk: true, desenle: true}
	if t.AddPrefixSpace != nil {
		b.onekBosluk = *t.AddPrefixSpace
	}
	if t.UseRegex != nil {
		b.desenle = *t.UseRegex
	}
	if b.desenle && baytDuzeyiIfade == nil {
		ifade, err := DuzIfadeDerle(BaytDuzeyiDeseni)
		if err != nil {
			return nil, err
		}
		baytDuzeyiIfade = ifade
	}
	return b, nil
}

func (b *baytOnBol) OnBol(dst, parcalar []Parca) []Parca {
	for _, p := range parcalar {
		b.akis(p, func(x Parca) { dst = append(dst, x) })
	}
	return dst
}

func (b *baytOnBol) cevir(s string, ilk bool, butce *dfButce) Parca {
	if b.tembel {
		return Parca{Metin: s, Ilk: ilk, baytDuzeyi: true, butce: butce}
	}
	return Parca{Metin: baytlariKarakterle([]byte(s)), Ilk: ilk, butce: butce}
}

func (b *baytOnBol) akis(p Parca, f func(Parca)) {
	if p.Metin == "" {
		return
	}
	m := p.Icerik()
	if b.onekBosluk && !strings.HasPrefix(m, " ") {
		m = " " + m
	}
	if !b.desenle {
		f(b.cevir(m, p.Ilk, p.butce))
		return
	}
	onceki := 0
	baytDuzeyiIfade.tumEslesmeler(m, func(bas, son int) {
		if onceki < bas {
			f(b.cevir(m[onceki:bas], p.Ilk && onceki == 0, p.butce))
		}
		if son > bas {
			f(b.cevir(m[bas:son], p.Ilk && bas == 0, p.butce))
		}
		onceki = son
	}, p.butce)
	if onceki < len(m) {
		f(b.cevir(m[onceki:], p.Ilk && onceki == 0, p.butce))
	}
}

// ---------------------------------------------------------------------------
// Metaspace
// ---------------------------------------------------------------------------

type onekKurali int

const (
	onekHer onekKurali = iota
	onekIlk
	onekYok
)

type metaspace struct {
	yerine string
	kural  onekKurali
	bol    bool
}

func metaspaceKur(ham json.RawMessage) (OnBolucu, error) {
	m, err := metaspaceOku(ham)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func metaspaceOku(ham json.RawMessage) (*metaspace, error) {
	var t struct {
		Replacement    *string `json:"replacement"`
		PrependScheme  *string `json:"prepend_scheme"`
		Split          *bool   `json:"split"`
		AddPrefixSpace *bool   `json:"add_prefix_space"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	m := &metaspace{yerine: "▁", kural: onekHer, bol: true}
	if t.Replacement != nil {
		if utf8.RuneCountInString(*t.Replacement) != 1 {
			return nil, fmt.Errorf("replacement tek karakter olmalı: %q", *t.Replacement)
		}
		m.yerine = *t.Replacement
	}
	if t.Split != nil {
		m.bol = *t.Split
	}
	switch {
	case t.PrependScheme != nil:
		switch *t.PrependScheme {
		case "always":
			m.kural = onekHer
		case "first":
			m.kural = onekIlk
		case "never":
			m.kural = onekYok
		default:
			return nil, fmt.Errorf("bilinmeyen prepend_scheme %q", *t.PrependScheme)
		}
	case t.AddPrefixSpace != nil && !*t.AddPrefixSpace:
		m.kural = onekYok
	}
	return m, nil
}

func (m *metaspace) OnBol(dst, parcalar []Parca) []Parca {
	for _, p := range parcalar {
		if p.Metin == "" {
			continue
		}
		s := strings.ReplaceAll(p.Icerik(), " ", m.yerine)
		if !strings.HasPrefix(s, m.yerine) && (m.kural == onekHer || (m.kural == onekIlk && p.Ilk)) {
			s = m.yerine + s
		}
		if !m.bol {
			if s != "" {
				dst = append(dst, Parca{Metin: s, Ilk: p.Ilk, butce: p.butce})
			}
			continue
		}
		a := eslesmeAraliklari(len(s), func(f func(b, e int)) {
			for i := 0; ; {
				k := strings.Index(s[i:], m.yerine)
				if k < 0 {
					return
				}
				f(i+k, i+k+len(m.yerine))
				i += k + len(m.yerine)
			}
		})
		dst = davranisUygula(dst, Parca{Ilk: p.Ilk, butce: p.butce}, s, a, davranisSonrakiyle)
	}
	return dst
}

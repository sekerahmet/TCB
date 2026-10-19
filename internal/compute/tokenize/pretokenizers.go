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
	RegisterPreTokenizer("Sequence", newSeqPreTok)
	RegisterPreTokenizer("Split", newSplit)
	RegisterPreTokenizer("ByteLevel", newBytePreTok)
	RegisterPreTokenizer("Whitespace", func(json.RawMessage) (PreTokenizer, error) { return splitWhitespace{}, nil })
	RegisterPreTokenizer("WhitespaceSplit", func(json.RawMessage) (PreTokenizer, error) {
		return splitChars{cond: unicode.IsSpace, behavior: behaviorRemoved}, nil
	})
	RegisterPreTokenizer("Metaspace", newMetaspace)
	RegisterPreTokenizer("BertPreTokenizer", func(json.RawMessage) (PreTokenizer, error) {
		return PreTokenizerSequence{
			splitChars{cond: unicode.IsSpace, behavior: behaviorRemoved},
			splitChars{cond: isPunct, behavior: behaviorIsolated},
		}, nil
	})
	RegisterPreTokenizer("Punctuation", func(raw json.RawMessage) (PreTokenizer, error) {
		var t struct {
			Behavior *string `json:"behavior"`
		}
		if err := strictDecode(raw, &t); err != nil {
			return nil, err
		}
		d := behaviorIsolated
		if t.Behavior != nil {
			var err error
			if d, err = readBehavior(*t.Behavior); err != nil {
				return nil, err
			}
		}
		return splitChars{cond: isPunct, behavior: d}, nil
	})
	RegisterPreTokenizer("Digits", func(raw json.RawMessage) (PreTokenizer, error) {
		var t struct {
			Single bool `json:"individual_digits"`
		}
		if err := strictDecode(raw, &t); err != nil {
			return nil, err
		}
		d := behaviorContiguous
		if t.Single {
			d = behaviorIsolated
		}
		return splitChars{cond: tableDigit.inside, behavior: d}, nil
	})
	RegisterPreTokenizer("CharDelimiterSplit", func(raw json.RawMessage) (PreTokenizer, error) {
		var t struct {
			Delimiter string `json:"delimiter"`
		}
		if err := strictDecode(raw, &t); err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(t.Delimiter) != 1 {
			return nil, fmt.Errorf("ayırıcı tek karakter olmalı: %q", t.Delimiter)
		}
		c, _ := utf8.DecodeRuneInString(t.Delimiter)
		return splitChars{cond: func(r rune) bool { return r == c }, behavior: behaviorRemoved}, nil
	})
}

// isPunct ASCII noktalama ya da Unicode P kategorisi.
func isPunct(r rune) bool {
	if r < utf8.RuneSelf {
		return (r >= '!' && r <= '/') || (r >= ':' && r <= '@') || (r >= '[' && r <= '`') || (r >= '{' && r <= '~')
	}
	return tableBertPunct.inside(r)
}

// PreTokenizerSequence ön bölücüleri sırayla uygular.
type PreTokenizerSequence []PreTokenizer

func (s PreTokenizerSequence) PreTokenize(dst, pieces []Piece) []Piece {
	for _, p := range pieces {
		s.stream(p, func(x Piece) { dst = append(dst, x) })
	}
	return dst
}

// stream zinciri derinlik öncelikli uygular: her parça, bir sonraki adıma
// hemen aktarılır; ara dilimler kurulmaz (büyük belgelerde bellek dostu).
func (s PreTokenizerSequence) stream(p Piece, f func(Piece)) {
	step := make([]func(Piece), len(s)+1)
	step[len(s)] = f
	for k := len(s) - 1; k >= 0; k-- {
		o, next := s[k], step[k+1]
		step[k] = func(x Piece) { preTokenizeStream(o, x, next) }
	}
	step[0](p)
}

// streamPreTokenizer parçaları dilim kurmadan tek tek üretebilen ön bölücüdür.
type streamPreTokenizer interface {
	stream(p Piece, f func(Piece))
}

// preTokenizeStream o'yu p'ye uygular ve her sonuç parçası için f'yi çağırır.
func preTokenizeStream(o PreTokenizer, p Piece, f func(Piece)) {
	if a, ok := o.(streamPreTokenizer); ok {
		a.stream(p, f)
		return
	}
	for _, x := range o.PreTokenize(nil, []Piece{p}) {
		f(x)
	}
}

func newSeqPreTok(raw json.RawMessage) (PreTokenizer, error) {
	var t struct {
		Pretokenizers []json.RawMessage `json:"pretokenizers"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	var s PreTokenizerSequence
	for i, h := range t.Pretokenizers {
		o, err := NewPreTokenizer(h)
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

type splitBehavior int

const (
	behaviorRemoved splitBehavior = iota
	behaviorIsolated
	behaviorMergedWithPrevious
	behaviorMergedWithNext
	behaviorContiguous
)

func readBehavior(s string) (splitBehavior, error) {
	switch s {
	case "Removed":
		return behaviorRemoved, nil
	case "Isolated":
		return behaviorIsolated, nil
	case "MergedWithPrevious":
		return behaviorMergedWithPrevious, nil
	case "MergedWithNext":
		return behaviorMergedWithNext, nil
	case "Contiguous":
		return behaviorContiguous, nil
	}
	return 0, fmt.Errorf("bilinmeyen bölme davranışı %q", s)
}

// span parçanın içindeki bir bölümdür; esles bölümün desene uyup uymadığıdır.
type span struct {
	begin, last int
	matchFn     bool
}

// applyBehavior eşleşme listesini davranışa göre birleştirip parçaları üretir.
func applyBehavior(dst []Piece, p Piece, text string, a []span, d splitBehavior) []Piece {
	type result struct {
		begin, last int
		remove      bool
	}
	var s []result
	switch d {
	case behaviorIsolated:
		for _, x := range a {
			s = append(s, result{x.begin, x.last, false})
		}
	case behaviorRemoved:
		for _, x := range a {
			s = append(s, result{x.begin, x.last, x.matchFn})
		}
	case behaviorContiguous:
		prev := false
		for _, x := range a {
			if x.matchFn == prev && len(s) > 0 {
				s[len(s)-1].last = x.last
			} else {
				s = append(s, result{x.begin, x.last, false})
			}
			prev = x.matchFn
		}
	case behaviorMergedWithPrevious:
		prev := false
		for _, x := range a {
			if x.matchFn && !prev && len(s) > 0 {
				s[len(s)-1].last = x.last
			} else {
				s = append(s, result{x.begin, x.last, false})
			}
			prev = x.matchFn
		}
	case behaviorMergedWithNext:
		prev := false
		var reverse []result
		for i := len(a) - 1; i >= 0; i-- {
			x := a[i]
			if x.matchFn && !prev && len(reverse) > 0 {
				reverse[len(reverse)-1].begin = x.begin
			} else {
				reverse = append(reverse, result{x.begin, x.last, false})
			}
			prev = x.matchFn
		}
		for i := len(reverse) - 1; i >= 0; i-- {
			s = append(s, reverse[i])
		}
	}
	for _, x := range s {
		if x.remove || x.last <= x.begin {
			continue
		}
		dst = append(dst, Piece{Text: text[x.begin:x.last], Start: p.Start && x.begin == 0, budget: p.budget})
	}
	return dst
}

// matchSpans eşleşmeleri, aralarındaki eşleşmeyen bölümlerle birlikte listeler.
func matchSpans(n int, find func(f func(b, e int))) []span {
	var a []span
	prev := 0
	find(func(b, e int) {
		if prev != b {
			a = append(a, span{prev, b, false})
		}
		a = append(a, span{b, e, true})
		prev = e
	})
	if prev != n {
		a = append(a, span{prev, n, false})
	}
	return a
}

func reverseMap(a []span) {
	for i := range a {
		a[i].matchFn = !a[i].matchFn
	}
}

// ---------------------------------------------------------------------------
// Split
// ---------------------------------------------------------------------------

type splitWithPattern struct {
	expr     *Regex
	plain    *string
	fast     bool // desen BolmeDeseni: elle yazılmış eşleştirici kullanılır
	behavior splitBehavior
	reverse  bool
}

func newSplit(raw json.RawMessage) (PreTokenizer, error) {
	var t struct {
		Pattern  patternForm `json:"pattern"`
		Behavior string      `json:"behavior"`
		Invert   bool        `json:"invert"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	d, err := readBehavior(t.Behavior)
	if err != nil {
		return nil, err
	}
	b := &splitWithPattern{behavior: d, reverse: t.Invert}
	switch {
	case t.Pattern.String != nil:
		b.plain = t.Pattern.String
	case t.Pattern.Regex != nil:
		if *t.Pattern.Regex == SplitPattern {
			b.fast = true
		} else {
			expr, err := CompileRegex(*t.Pattern.Regex)
			if err != nil {
				return nil, err
			}
			b.expr = expr
		}
	default:
		return nil, fmt.Errorf("desen (pattern) String ya da Regex olmalı")
	}
	return b, nil
}

func (b *splitWithPattern) find(text string, f func(begin, last int), budget *reBudget) {
	switch {
	case b.fast:
		for i := 0; i < len(text); {
			j := matchEnd(text, i)
			f(i, j)
			i = j
		}
	case b.plain != nil:
		d := *b.plain
		if d == "" {
			return
		}
		for i := 0; ; {
			k := strings.Index(text[i:], d)
			if k < 0 {
				return
			}
			f(i+k, i+k+len(d))
			i += k + len(d)
		}
	default:
		b.expr.allMatches(text, f, budget)
	}
}

func (b *splitWithPattern) PreTokenize(dst, pieces []Piece) []Piece {
	for _, p := range pieces {
		b.stream(p, func(x Piece) { dst = append(dst, x) })
	}
	return dst
}

func (b *splitWithPattern) stream(p Piece, f func(Piece)) {
	text := p.Content()
	// Hızlı yol: elle yazılmış desen her karakteri kapsar, "Isolated"
	// davranışında her eşleşme doğrudan bir parçadır.
	if b.fast && b.behavior == behaviorIsolated {
		for i := 0; i < len(text); {
			j := matchEnd(text, i)
			f(Piece{Text: text[i:j], Start: p.Start && i == 0, budget: p.budget})
			i = j
		}
		return
	}
	a := matchSpans(len(text), func(f func(b, e int)) { b.find(text, f, p.budget) })
	if b.reverse {
		reverseMap(a)
	}
	for _, x := range applyBehavior(nil, p, text, a, b.behavior) {
		f(x)
	}
}

// splitChars her uyan karakteri ayrı bir eşleşme sayarak böler.
type splitChars struct {
	cond     func(rune) bool
	behavior splitBehavior
}

func (k splitChars) PreTokenize(dst, pieces []Piece) []Piece {
	for _, p := range pieces {
		text := p.Content()
		a := matchSpans(len(text), func(f func(b, e int)) {
			for i, r := range text {
				if k.cond(r) {
					f(i, i+utf8.RuneLen(r))
				}
			}
		})
		dst = applyBehavior(dst, p, text, a, k.behavior)
	}
	return dst
}

// splitWhitespace: "\w+|[^\w\s]+" eşleşmelerini tutar, aradaki boşlukları atar.
type splitWhitespace struct{}

func (splitWhitespace) PreTokenize(dst, pieces []Piece) []Piece {
	for _, p := range pieces {
		m := p.Content()
		for i := 0; i < len(m); {
			r, w := utf8.DecodeRuneInString(m[i:])
			var j int
			switch {
			case wordChar(r):
				j = i + w
				for j < len(m) {
					r2, w2 := utf8.DecodeRuneInString(m[j:])
					if !wordChar(r2) {
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
					if wordChar(r2) || unicode.IsSpace(r2) {
						break
					}
					j += w2
				}
			}
			dst = append(dst, Piece{Text: m[i:j], Start: p.Start && i == 0, budget: p.budget})
			i = j
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// ByteLevel
// ---------------------------------------------------------------------------

// ByteLevelPattern bayt düzeyi ön bölücünün use_regex=true iken kullandığı desendir.
const ByteLevelPattern = `'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`

var byteLevelRegex *Regex

type bytePreTok struct {
	prefixSpace bool
	withPattern bool
	// lazy doğruysa bayt -> karakter dönüşümü yapılmaz, parça işaretlenir
	// (yalnızca zincirin son adımıyken güvenlidir; kurulumda ayarlanır).
	lazy bool
}

func newBytePreTok(raw json.RawMessage) (PreTokenizer, error) {
	t := struct {
		AddPrefixSpace *bool `json:"add_prefix_space"`
		UseRegex       *bool `json:"use_regex"`
	}{}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	b := &bytePreTok{prefixSpace: true, withPattern: true}
	if t.AddPrefixSpace != nil {
		b.prefixSpace = *t.AddPrefixSpace
	}
	if t.UseRegex != nil {
		b.withPattern = *t.UseRegex
	}
	if b.withPattern && byteLevelRegex == nil {
		expr, err := CompileRegex(ByteLevelPattern)
		if err != nil {
			return nil, err
		}
		byteLevelRegex = expr
	}
	return b, nil
}

func (b *bytePreTok) PreTokenize(dst, pieces []Piece) []Piece {
	for _, p := range pieces {
		b.stream(p, func(x Piece) { dst = append(dst, x) })
	}
	return dst
}

func (b *bytePreTok) convert(s string, first bool, budget *reBudget) Piece {
	if b.lazy {
		return Piece{Text: s, Start: first, byteLevel: true, budget: budget}
	}
	return Piece{Text: bytesToChars([]byte(s)), Start: first, budget: budget}
}

func (b *bytePreTok) stream(p Piece, f func(Piece)) {
	if p.Text == "" {
		return
	}
	m := p.Content()
	if b.prefixSpace && !strings.HasPrefix(m, " ") {
		m = " " + m
	}
	if !b.withPattern {
		f(b.convert(m, p.Start, p.budget))
		return
	}
	prev := 0
	byteLevelRegex.allMatches(m, func(begin, last int) {
		if prev < begin {
			f(b.convert(m[prev:begin], p.Start && prev == 0, p.budget))
		}
		if last > begin {
			f(b.convert(m[begin:last], p.Start && begin == 0, p.budget))
		}
		prev = last
	}, p.budget)
	if prev < len(m) {
		f(b.convert(m[prev:], p.Start && prev == 0, p.budget))
	}
}

// ---------------------------------------------------------------------------
// Metaspace
// ---------------------------------------------------------------------------

type prefixRule int

const (
	prefixAlways prefixRule = iota
	prefixFirst
	prefixNever
)

type metaspace struct {
	instead string
	rule    prefixRule
	splitFn bool
}

func newMetaspace(raw json.RawMessage) (PreTokenizer, error) {
	m, err := readMetaspace(raw)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func readMetaspace(raw json.RawMessage) (*metaspace, error) {
	var t struct {
		Replacement    *string `json:"replacement"`
		PrependScheme  *string `json:"prepend_scheme"`
		Split          *bool   `json:"split"`
		AddPrefixSpace *bool   `json:"add_prefix_space"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	m := &metaspace{instead: "▁", rule: prefixAlways, splitFn: true}
	if t.Replacement != nil {
		if utf8.RuneCountInString(*t.Replacement) != 1 {
			return nil, fmt.Errorf("replacement tek karakter olmalı: %q", *t.Replacement)
		}
		m.instead = *t.Replacement
	}
	if t.Split != nil {
		m.splitFn = *t.Split
	}
	switch {
	case t.PrependScheme != nil:
		switch *t.PrependScheme {
		case "always":
			m.rule = prefixAlways
		case "first":
			m.rule = prefixFirst
		case "never":
			m.rule = prefixNever
		default:
			return nil, fmt.Errorf("bilinmeyen prepend_scheme %q", *t.PrependScheme)
		}
	case t.AddPrefixSpace != nil && !*t.AddPrefixSpace:
		m.rule = prefixNever
	}
	return m, nil
}

func (m *metaspace) PreTokenize(dst, pieces []Piece) []Piece {
	for _, p := range pieces {
		if p.Text == "" {
			continue
		}
		s := strings.ReplaceAll(p.Content(), " ", m.instead)
		if !strings.HasPrefix(s, m.instead) && (m.rule == prefixAlways || (m.rule == prefixFirst && p.Start)) {
			s = m.instead + s
		}
		if !m.splitFn {
			if s != "" {
				dst = append(dst, Piece{Text: s, Start: p.Start, budget: p.budget})
			}
			continue
		}
		a := matchSpans(len(s), func(f func(b, e int)) {
			for i := 0; ; {
				k := strings.Index(s[i:], m.instead)
				if k < 0 {
					return
				}
				f(i+k, i+k+len(m.instead))
				i += k + len(m.instead)
			}
		})
		dst = applyBehavior(dst, Piece{Start: p.Start, budget: p.budget}, s, a, behaviorMergedWithNext)
	}
	return dst
}

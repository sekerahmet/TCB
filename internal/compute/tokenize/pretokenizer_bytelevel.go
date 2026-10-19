package tokenize

import (
	"encoding/json"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// ByteLevel
// ---------------------------------------------------------------------------

// ByteLevelPattern bayt düzeyi ön bölücünün use_regex=true iken kullandığı desendir.
const ByteLevelPattern = `'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`

// byteLevelRegex ByteLevelPattern'in derlenmiş hâlidir; ilk bayt düzeyi ön bölücü kurulurken
// bir kez derlenir (parçalayıcılar eşzamanlı görevlerde aynı anda yüklenebilir).
var (
	byteLevelOnce  sync.Once
	byteLevelRegex *Regex
	byteLevelErr   error
)

// byteLevelExpr derlenmiş deseni verir (gerekirse derler).
func byteLevelExpr() (*Regex, error) {
	byteLevelOnce.Do(func() { byteLevelRegex, byteLevelErr = CompileRegex(ByteLevelPattern) })
	return byteLevelRegex, byteLevelErr
}

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
	if b.withPattern {
		if _, err := byteLevelExpr(); err != nil {
			return nil, err
		}
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
	re, _ := byteLevelExpr() // kurulumda derlendi (newBytePreTok)
	re.allMatches(m, func(begin, last int) {
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

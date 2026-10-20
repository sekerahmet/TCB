package tokenize

import (
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// WordPiece, WordLevel ve Unigram modelleri
// ============================================================================

func init() {
	RegisterModel("WordPiece", newWordPiece)
	RegisterModel("WordLevel", newWordLevel)
	RegisterModel("Unigram", newUnigram)
}

// ---------------------------------------------------------------------------
// WordPiece: en uzun önek önce (açgözlü) alt sözcük bölme.
// ---------------------------------------------------------------------------

type wordPiece struct {
	vocab   map[string]int32
	unknown int32
	prefix  string
	maxN    int
}

func newWordPiece(raw json.RawMessage) (Model, error) {
	m := struct {
		UnkToken *string          `json:"unk_token"`
		Prefix   *string          `json:"continuing_subword_prefix"`
		MaxChars *int             `json:"max_input_chars_per_word"`
		Vocab    map[string]int64 `json:"vocab"`
	}{}
	if err := strictDecode(raw, &m); err != nil {
		return nil, err
	}
	s, err := convertVocab(m.Vocab)
	if err != nil {
		return nil, err
	}
	w := &wordPiece{vocab: s, prefix: "##", maxN: 100}
	unk := "[UNK]"
	if m.UnkToken != nil {
		unk = *m.UnkToken
	}
	if m.Prefix != nil {
		w.prefix = *m.Prefix
	}
	if m.MaxChars != nil {
		w.maxN = *m.MaxChars
	}
	id, ok := s[unk]
	if !ok {
		return nil, messages.New(messages.H5217, unk)
	}
	w.unknown = id
	return w, nil
}

func (w *wordPiece) Vocab() map[string]int32 { return w.vocab }

func (w *wordPiece) Encode(dst []int32, p Piece) []int32 {
	m := p.Content()
	if utf8.RuneCountInString(m) > w.maxN {
		return append(dst, w.unknown)
	}
	n := len(dst)
	for begin := 0; begin < len(m); {
		last := len(m)
		wasFound := int32(-1)
		for begin < last {
			sub := m[begin:last]
			if begin > 0 {
				sub = w.prefix + sub
			}
			if id, ok := w.vocab[sub]; ok {
				wasFound = id
				break
			}
			_, k := utf8.DecodeLastRuneInString(m[begin:last])
			last -= k
		}
		if wasFound < 0 {
			return append(dst[:n], w.unknown)
		}
		dst = append(dst, wasFound)
		begin = last
	}
	return dst
}

// ---------------------------------------------------------------------------
// WordLevel: parçanın tamamı sözlükte aranır, yoksa bilinmeyen token.
// ---------------------------------------------------------------------------

type wordLevel struct {
	vocab   map[string]int32
	unknown int32
	unkName string
}

func newWordLevel(raw json.RawMessage) (Model, error) {
	m := struct {
		UnkToken *string          `json:"unk_token"`
		Vocab    map[string]int64 `json:"vocab"`
	}{}
	if err := strictDecode(raw, &m); err != nil {
		return nil, err
	}
	s, err := convertVocab(m.Vocab)
	if err != nil {
		return nil, err
	}
	w := &wordLevel{vocab: s, unknown: -1, unkName: "<unk>"}
	if m.UnkToken != nil {
		w.unkName = *m.UnkToken
	}
	if id, ok := s[w.unkName]; ok {
		w.unknown = id
	}
	return w, nil
}

func (w *wordLevel) Vocab() map[string]int32 { return w.vocab }

func (w *wordLevel) Encode(dst []int32, p Piece) []int32 {
	if id, ok := w.vocab[p.Content()]; ok {
		return append(dst, id)
	}
	if w.unknown >= 0 {
		return append(dst, w.unknown)
	}
	// Başvuru uygulaması burada hata verir; kodlama sürdürülebilsin diye
	// parça atlanır.
	return dst
}

// ---------------------------------------------------------------------------
// Unigram: puanlı sözlükte en yüksek toplam puanlı bölümleme (Viterbi).
// ---------------------------------------------------------------------------

const unigramUnknownPenalty = 10.0

type unigram struct {
	vocab        map[string]int32
	score        []float64
	tokenList    []string
	unknown      int32
	byteFallback bool
	lowest       float64
	root         *unigramNode
	fallbackID   [256]int32
	cache        *cache
}

// unigramNode bayt tabanlı önek ağacının düğümü.
type unigramNode struct {
	child map[byte]*unigramNode
	id    int32 // -1: burada biten token yok
}

func newUnigram(raw json.RawMessage) (Model, error) {
	var m struct {
		UnkID        *int              `json:"unk_id"`
		Vocab        []json.RawMessage `json:"vocab"`
		ByteFallback bool              `json:"byte_fallback"`
	}
	if err := strictDecode(raw, &m); err != nil {
		return nil, err
	}
	u := &unigram{
		vocab:        make(map[string]int32, len(m.Vocab)),
		unknown:      -1,
		byteFallback: m.ByteFallback,
		lowest:       math.Inf(1),
		root:         &unigramNode{id: -1},
		cache:        newCache(DefaultCacheCapacity),
	}
	for i, h := range m.Vocab {
		var pair []json.RawMessage
		if err := json.Unmarshal(h, &pair); err != nil || len(pair) != 2 {
			return nil, messages.New(messages.H5224, i)
		}
		var tok string
		var score float64
		if err := json.Unmarshal(pair[0], &tok); err != nil {
			return nil, messages.New(messages.H5225, i, err)
		}
		if err := json.Unmarshal(pair[1], &score); err != nil {
			return nil, messages.New(messages.H5225, i, err)
		}
		u.tokenList = append(u.tokenList, tok)
		u.score = append(u.score, score)
		u.vocab[tok] = int32(i)
		if score < u.lowest {
			u.lowest = score
		}
		d := u.root
		for k := 0; k < len(tok); k++ {
			if d.child == nil {
				d.child = make(map[byte]*unigramNode)
			}
			c, ok := d.child[tok[k]]
			if !ok {
				c = &unigramNode{id: -1}
				d.child[tok[k]] = c
			}
			d = c
		}
		d.id = int32(i) // aynı token yineleniyorsa sonuncusu geçerlidir
	}
	if m.UnkID != nil {
		if *m.UnkID < 0 || *m.UnkID >= len(u.tokenList) {
			return nil, messages.New(messages.H5226, *m.UnkID)
		}
		u.unknown = int32(*m.UnkID)
	}
	for c := 0; c < 256; c++ {
		u.fallbackID[c] = -1
		if id, ok := u.vocab[fmt.Sprintf("<0x%02X>", c)]; ok {
			u.fallbackID[c] = id
		}
	}
	return u, nil
}

func (u *unigram) Vocab() map[string]int32 { return u.vocab }

// idTokens (idListModel) kimliklerin metinleri, dosyadaki sırayla.
func (u *unigram) idTokens() []string { return u.tokenList }

func (u *unigram) Encode(dst []int32, p Piece) []int32 {
	m := p.Content()
	o := u.cache
	if o != nil && len(m) <= cacheMaxLen {
		if v, ok := o.al(m); ok {
			return append(dst, v...)
		}
		n := len(dst)
		dst = u.encode(dst, m)
		o.put(m, dst[n:])
		return dst
	}
	return u.encode(dst, m)
}

func (u *unigram) encode(dst []int32, m string) []int32 {
	if m == "" {
		return dst
	}
	type node struct {
		id    int32
		score float64
		begin int
		full  bool
	}
	n := len(m)
	best := make([]node, n+1)
	unkScore := u.lowest - unigramUnknownPenalty
	for begin := 0; begin < n; {
		_, kw := utf8.DecodeRuneInString(m[begin:])
		basic := best[begin].score
		hasSingle := false
		d := u.root
		for k := begin; k < n && d != nil; k++ {
			d = d.child[m[k]]
			if d == nil {
				break
			}
			if d.id >= 0 {
				last := k + 1
				candidate := basic + u.score[d.id]
				h := &best[last]
				if !h.full || candidate > h.score {
					*h = node{id: d.id, score: candidate, begin: begin, full: true}
				}
				if last-begin == kw {
					hasSingle = true
				}
			}
		}
		if !hasSingle {
			if u.unknown < 0 {
				// Başvuru uygulaması burada hata verir; karakter atlanmış sayılır.
				h := &best[begin+kw]
				if !h.full || basic > h.score {
					*h = node{id: -1, score: basic, begin: begin, full: true}
				}
			} else {
				candidate := basic + unkScore
				h := &best[begin+kw]
				if !h.full || candidate > h.score {
					*h = node{id: u.unknown, score: candidate, begin: begin, full: true}
				}
			}
		}
		begin += kw
	}
	// Geriye doğru yol çıkarımı; ardışık bilinmeyenler birleştirilir.
	type section struct {
		begin, last int
		id          int32
	}
	var reverse []section
	for last := n; last > 0; {
		h := best[last]
		if h.id >= 0 && h.id == u.unknown && len(reverse) > 0 && reverse[len(reverse)-1].id == u.unknown {
			reverse[len(reverse)-1].begin = h.begin
		} else {
			reverse = append(reverse, section{h.begin, last, h.id})
		}
		last = h.begin
	}
	for i := len(reverse) - 1; i >= 0; i-- {
		b := reverse[i]
		if b.id < 0 {
			continue
		}
		piece := m[b.begin:b.last]
		if id, ok := u.vocab[piece]; ok {
			dst = append(dst, id)
			continue
		}
		if u.byteFallback {
			done := true
			for k := 0; k < len(piece); k++ {
				if u.fallbackID[piece[k]] < 0 {
					done = false
					break
				}
			}
			if done {
				for k := 0; k < len(piece); k++ {
					dst = append(dst, u.fallbackID[piece[k]])
				}
				continue
			}
		}
		dst = append(dst, u.unknown)
	}
	return dst
}

package tokenize

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// ============================================================================
// BPE modeli
// ============================================================================
//
// Parça önce karakterlere (bayt düzeyi parçada baytlara) ayrılır; her karakter
// sözlükte aranır. Bulunamayan karakter için sırasıyla: bayt yedeği
// (byte_fallback: karakterin UTF-8 baytları "<0xXX>" tokenları olarak), sonra
// bilinmeyen token (unk_token; fuse_unk ile ardışık bilinmeyenler tek token)
// denenir; ikisi de yoksa karakter atlanır. Ardından birleştirme kuralları
// öncelik sırasıyla uygulanır (bkz. bpe.go).

func init() { RegisterModel("BPE", newBPE) }

// kisaEsik bu uzunluğa kadar (sembol sayısı) doğrudan tarama kullanılır.
const shortThreshold = 48

type bpeModel struct {
	vocab      map[string]int32
	table      *mergeTable
	mergeCount int

	unknown        int32 // -1: yok
	joinUnknown    bool  // fuse_unk
	byteFallback   bool  // byte_fallback
	noMerges       bool  // ignore_merges
	prefix, suffix string

	charID     map[rune]int32 // tek karakterlik tokenlar
	lowID      [0x200]int32   // karakterID'nin küçük kod noktaları için dizisi
	byteCharID [256]int32     // bayt düzeyi karakterin kimliği (-1: yok)
	fallbackID [256]int32     // "<0xXX>" tokenları (-1: yok)
	byteVocab  map[string]int32
	cache      *cache
}

func newBPE(raw json.RawMessage) (Model, error) {
	var m struct {
		Dropout                 *float64          `json:"dropout"`
		UnkToken                *string           `json:"unk_token"`
		ContinuingSubwordPrefix *string           `json:"continuing_subword_prefix"`
		EndOfWordSuffix         *string           `json:"end_of_word_suffix"`
		FuseUnk                 bool              `json:"fuse_unk"`
		ByteFallback            bool              `json:"byte_fallback"`
		IgnoreMerges            bool              `json:"ignore_merges"`
		Vocab                   map[string]int64  `json:"vocab"`
		Merges                  []json.RawMessage `json:"merges"`
	}
	if err := strictDecode(raw, &m); err != nil {
		return nil, err
	}
	if m.Dropout != nil && *m.Dropout != 0 {
		return nil, fmt.Errorf("dropout=%v desteklenmiyor (rastgele birleştirme kodlamayı belirsiz yapar)", *m.Dropout)
	}
	vocab, err := convertVocab(m.Vocab)
	if err != nil {
		return nil, err
	}
	b := &bpeModel{
		vocab:        vocab,
		unknown:      -1,
		joinUnknown:  m.FuseUnk,
		byteFallback: m.ByteFallback,
		noMerges:     m.IgnoreMerges,
		charID:       make(map[rune]int32),
		byteVocab:    make(map[string]int32),
		cache:        newCache(DefaultCacheCapacity),
	}
	if m.ContinuingSubwordPrefix != nil {
		b.prefix = *m.ContinuingSubwordPrefix
	}
	if m.EndOfWordSuffix != nil {
		b.suffix = *m.EndOfWordSuffix
	}
	if m.UnkToken != nil {
		id, ok := vocab[*m.UnkToken]
		if !ok {
			return nil, fmt.Errorf("bilinmeyen parça %q dağarcıkta yok", *m.UnkToken)
		}
		b.unknown = id
	}
	for i := range b.lowID {
		b.lowID[i] = -1
	}
	for tok, id := range vocab {
		if r, w := utf8.DecodeRuneInString(tok); w == len(tok) && w > 0 {
			b.charID[r] = id
			if r < rune(len(b.lowID)) {
				b.lowID[r] = id
			}
		}
		ok := true
		bs := make([]byte, 0, len(tok))
		for _, r := range tok {
			c, var_ := charByte[r]
			if !var_ {
				ok = false
				break
			}
			bs = append(bs, c)
		}
		if ok {
			b.byteVocab[string(bs)] = id
		}
	}
	for c := 0; c < 256; c++ {
		b.byteCharID[c] = b.chr(byteChar[c])
		if id, ok := vocab[fmt.Sprintf("<0x%02X>", c)]; ok {
			b.fallbackID[c] = id
		} else {
			b.fallbackID[c] = -1
		}
	}
	mergeList := make(map[uint64]mergeRule, len(m.Merges))
	for i, r := range m.Merges {
		x, y, err := parseMergePair(r)
		if err != nil {
			return nil, fmt.Errorf("%d. birleştirme: %w", i, err)
		}
		ia, ok1 := vocab[x]
		ib, ok2 := vocab[y]
		if !ok1 || !ok2 || len(y) < len(b.prefix) {
			return nil, fmt.Errorf("%d. birleştirme (%q, %q) dağarcıkta olmayan bir parça içeriyor", i, x, y)
		}
		fresh, ok3 := vocab[x+y[len(b.prefix):]]
		if !ok3 {
			return nil, fmt.Errorf("%d. birleştirmenin sonucu %q dağarcıkta yok", i, x+y[len(b.prefix):])
		}
		// Aynı çift birden çok kez geçerse sonuncusu geçerlidir.
		mergeList[pairKey(ia, ib)] = mergeRule{order: int32(i), fresh: fresh}
	}
	b.mergeCount = len(m.Merges)
	b.table = newMergeTable(mergeList)
	return b, nil
}

func convertVocab(v map[string]int64) (map[string]int32, error) {
	s := make(map[string]int32, len(v))
	for tok, id := range v {
		if id < 0 || id > 1<<30 {
			return nil, fmt.Errorf("dağarcıkta geçersiz kimlik: %q -> %d", tok, id)
		}
		s[tok] = int32(id)
	}
	return s, nil
}

// birlesmeCiftiCoz bir birleştirme girdisini ("a b" ya da ["a","b"]) çözer.
func parseMergePair(r json.RawMessage) (string, string, error) {
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		for i := 0; i < len(s); i++ {
			if s[i] == ' ' {
				return s[:i], s[i+1:], nil
			}
		}
		return "", "", fmt.Errorf("birleştirme %q iki parçaya ayrılamadı", s)
	}
	var c []string
	if err := json.Unmarshal(r, &c); err != nil || len(c) != 2 {
		return "", "", fmt.Errorf("birleştirme girdisi çözümlenemedi: %s", string(r))
	}
	return c[0], c[1], nil
}

func (b *bpeModel) Vocab() map[string]int32 { return b.vocab }

func (b *bpeModel) chr(r rune) int32 {
	if r >= 0 && r < rune(len(b.lowID)) {
		return b.lowID[r]
	}
	if id, ok := b.charID[r]; ok {
		return id
	}
	return -1
}

// Kodla parçayı (önbellek yardımıyla) kodlar.
func (b *bpeModel) Encode(dst []int32, p Piece) []int32 {
	o := b.cache
	if o == nil || len(p.Text) > cacheMaxLen {
		return b.encode(dst, p)
	}
	if v, ok := o.al(p.Text); ok {
		return append(dst, v...)
	}
	n := len(dst)
	dst = b.encode(dst, p)
	o.put(p.Text, dst[n:])
	return dst
}

func (b *bpeModel) encode(dst []int32, p Piece) []int32 {
	if b.noMerges {
		var id int32
		var ok bool
		if p.byteLevel {
			id, ok = b.byteVocab[p.Text]
		} else {
			id, ok = b.vocab[p.Text]
		}
		if ok {
			return append(dst, id)
		}
	}
	var local [shortThreshold]int32
	sem := local[:0]
	if len(p.Text) > shortThreshold {
		sem = make([]int32, 0, len(p.Text))
	}
	sem = b.symbols(sem, p)
	if len(sem) <= shortThreshold {
		sem = b.joinShort(sem)
	} else {
		sem = b.joinLong(sem)
	}
	return append(dst, sem...)
}

// semboller parçanın ilk sembol dizisini üretir.
func (b *bpeModel) symbols(sem []int32, p Piece) []int32 {
	m := p.Text
	// Hızlı yol: bayt düzeyi, önek/sonek yok, her bayt sözlükte.
	if p.byteLevel && b.prefix == "" && b.suffix == "" {
		i := 0
		for ; i < len(m); i++ {
			id := b.byteCharID[m[i]]
			if id < 0 {
				break
			}
			sem = append(sem, id)
		}
		if i == len(m) {
			return sem
		}
		sem = sem[:0]
	}
	pending := int32(-1) // bekleyen bilinmeyen token
	addPending := func() {
		if pending >= 0 {
			sem = append(sem, pending)
			pending = -1
		}
	}
	// Karakterleri (bayt düzeyinde: baytların temsil karakterlerini) gez.
	var chars []rune
	if p.byteLevel {
		chars = make([]rune, len(m))
		for i := 0; i < len(m); i++ {
			chars[i] = byteChar[m[i]]
		}
	} else {
		chars = []rune(m)
	}
	for i, c := range chars {
		var id int32 = -1
		var s string
		if b.prefix == "" && b.suffix == "" {
			id = b.chr(c)
		} else {
			s = string(c)
			if i > 0 {
				s = b.prefix + s
			}
			if i == len(chars)-1 {
				s += b.suffix
			}
			if v, ok := b.vocab[s]; ok {
				id = v
			}
		}
		if id >= 0 {
			addPending()
			sem = append(sem, id)
			continue
		}
		if s == "" {
			s = string(c)
		}
		if b.byteFallback {
			done := true
			for k := 0; k < len(s); k++ {
				if b.fallbackID[s[k]] < 0 {
					done = false
					break
				}
			}
			if done {
				for k := 0; k < len(s); k++ {
					sem = append(sem, b.fallbackID[s[k]])
				}
				continue
			}
		}
		if b.unknown >= 0 {
			if pending >= 0 && !b.joinUnknown {
				addPending()
			}
			pending = b.unknown
		}
	}
	addPending()
	return sem
}

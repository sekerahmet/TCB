package parcalayici

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

func init() { ModelKaydet("BPE", bpeKur) }

// kisaEsik bu uzunluğa kadar (sembol sayısı) doğrudan tarama kullanılır.
const kisaEsik = 48

type bpeModel struct {
	sozluk         map[string]int32
	tablo          *birlesmeTablosu
	birlesmeSayisi int

	bilinmeyen   int32 // -1: yok
	birlestirBil bool  // fuse_unk
	baytYedegi   bool  // byte_fallback
	birlesmeYok  bool  // ignore_merges
	onek, sonek  string

	karakterID map[rune]int32 // tek karakterlik tokenlar
	dusukID    [0x200]int32   // karakterID'nin küçük kod noktaları için dizisi
	baytKarID  [256]int32     // bayt düzeyi karakterin kimliği (-1: yok)
	yedekID    [256]int32     // "<0xXX>" tokenları (-1: yok)
	baytSozluk map[string]int32
	onbellek   *onbellek
}

func bpeKur(ham json.RawMessage) (Model, error) {
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
	if err := katiCoz(ham, &m); err != nil {
		return nil, err
	}
	if m.Dropout != nil && *m.Dropout != 0 {
		return nil, fmt.Errorf("dropout=%v desteklenmiyor (rastgele birleştirme kodlamayı belirsiz yapar)", *m.Dropout)
	}
	sozluk, err := sozlukDonustur(m.Vocab)
	if err != nil {
		return nil, err
	}
	b := &bpeModel{
		sozluk:       sozluk,
		bilinmeyen:   -1,
		birlestirBil: m.FuseUnk,
		baytYedegi:   m.ByteFallback,
		birlesmeYok:  m.IgnoreMerges,
		karakterID:   make(map[rune]int32),
		baytSozluk:   make(map[string]int32),
		onbellek:     yeniOnbellek(VarsayilanOnbellekKapasitesi),
	}
	if m.ContinuingSubwordPrefix != nil {
		b.onek = *m.ContinuingSubwordPrefix
	}
	if m.EndOfWordSuffix != nil {
		b.sonek = *m.EndOfWordSuffix
	}
	if m.UnkToken != nil {
		id, ok := sozluk[*m.UnkToken]
		if !ok {
			return nil, fmt.Errorf("bilinmeyen parça %q dağarcıkta yok", *m.UnkToken)
		}
		b.bilinmeyen = id
	}
	for i := range b.dusukID {
		b.dusukID[i] = -1
	}
	for tok, id := range sozluk {
		if r, w := utf8.DecodeRuneInString(tok); w == len(tok) && w > 0 {
			b.karakterID[r] = id
			if r < rune(len(b.dusukID)) {
				b.dusukID[r] = id
			}
		}
		ok := true
		bs := make([]byte, 0, len(tok))
		for _, r := range tok {
			c, var_ := karakterBayt[r]
			if !var_ {
				ok = false
				break
			}
			bs = append(bs, c)
		}
		if ok {
			b.baytSozluk[string(bs)] = id
		}
	}
	for c := 0; c < 256; c++ {
		b.baytKarID[c] = b.kar(baytKarakter[c])
		if id, ok := sozluk[fmt.Sprintf("<0x%02X>", c)]; ok {
			b.yedekID[c] = id
		} else {
			b.yedekID[c] = -1
		}
	}
	birlesmeler := make(map[uint64]birlesme, len(m.Merges))
	for i, r := range m.Merges {
		x, y, err := birlesmeCiftiCoz(r)
		if err != nil {
			return nil, fmt.Errorf("%d. birleştirme: %w", i, err)
		}
		ia, ok1 := sozluk[x]
		ib, ok2 := sozluk[y]
		if !ok1 || !ok2 || len(y) < len(b.onek) {
			return nil, fmt.Errorf("%d. birleştirme (%q, %q) dağarcıkta olmayan bir parça içeriyor", i, x, y)
		}
		yeni, ok3 := sozluk[x+y[len(b.onek):]]
		if !ok3 {
			return nil, fmt.Errorf("%d. birleştirmenin sonucu %q dağarcıkta yok", i, x+y[len(b.onek):])
		}
		// Aynı çift birden çok kez geçerse sonuncusu geçerlidir.
		birlesmeler[ciftAnahtar(ia, ib)] = birlesme{sira: int32(i), yeni: yeni}
	}
	b.birlesmeSayisi = len(m.Merges)
	b.tablo = yeniBirlesmeTablosu(birlesmeler)
	return b, nil
}

func sozlukDonustur(v map[string]int64) (map[string]int32, error) {
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
func birlesmeCiftiCoz(r json.RawMessage) (string, string, error) {
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

func (b *bpeModel) Sozluk() map[string]int32 { return b.sozluk }

func (b *bpeModel) kar(r rune) int32 {
	if r >= 0 && r < rune(len(b.dusukID)) {
		return b.dusukID[r]
	}
	if id, ok := b.karakterID[r]; ok {
		return id
	}
	return -1
}

// Kodla parçayı (önbellek yardımıyla) kodlar.
func (b *bpeModel) Kodla(dst []int32, p Parca) []int32 {
	o := b.onbellek
	if o == nil || len(p.Metin) > onbellekAzamiUzunluk {
		return b.kodla(dst, p)
	}
	if v, ok := o.al(p.Metin); ok {
		return append(dst, v...)
	}
	n := len(dst)
	dst = b.kodla(dst, p)
	o.koy(p.Metin, dst[n:])
	return dst
}

func (b *bpeModel) kodla(dst []int32, p Parca) []int32 {
	if b.birlesmeYok {
		var id int32
		var ok bool
		if p.baytDuzeyi {
			id, ok = b.baytSozluk[p.Metin]
		} else {
			id, ok = b.sozluk[p.Metin]
		}
		if ok {
			return append(dst, id)
		}
	}
	var yerel [kisaEsik]int32
	sem := yerel[:0]
	if len(p.Metin) > kisaEsik {
		sem = make([]int32, 0, len(p.Metin))
	}
	sem = b.semboller(sem, p)
	if len(sem) <= kisaEsik {
		sem = b.birlestirKisa(sem)
	} else {
		sem = b.birlestirUzun(sem)
	}
	return append(dst, sem...)
}

// semboller parçanın ilk sembol dizisini üretir.
func (b *bpeModel) semboller(sem []int32, p Parca) []int32 {
	m := p.Metin
	// Hızlı yol: bayt düzeyi, önek/sonek yok, her bayt sözlükte.
	if p.baytDuzeyi && b.onek == "" && b.sonek == "" {
		i := 0
		for ; i < len(m); i++ {
			id := b.baytKarID[m[i]]
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
	bekleyen := int32(-1) // bekleyen bilinmeyen token
	ekleBekleyen := func() {
		if bekleyen >= 0 {
			sem = append(sem, bekleyen)
			bekleyen = -1
		}
	}
	// Karakterleri (bayt düzeyinde: baytların temsil karakterlerini) gez.
	var karakterler []rune
	if p.baytDuzeyi {
		karakterler = make([]rune, len(m))
		for i := 0; i < len(m); i++ {
			karakterler[i] = baytKarakter[m[i]]
		}
	} else {
		karakterler = []rune(m)
	}
	for i, c := range karakterler {
		var id int32 = -1
		var s string
		if b.onek == "" && b.sonek == "" {
			id = b.kar(c)
		} else {
			s = string(c)
			if i > 0 {
				s = b.onek + s
			}
			if i == len(karakterler)-1 {
				s += b.sonek
			}
			if v, ok := b.sozluk[s]; ok {
				id = v
			}
		}
		if id >= 0 {
			ekleBekleyen()
			sem = append(sem, id)
			continue
		}
		if s == "" {
			s = string(c)
		}
		if b.baytYedegi {
			tamam := true
			for k := 0; k < len(s); k++ {
				if b.yedekID[s[k]] < 0 {
					tamam = false
					break
				}
			}
			if tamam {
				for k := 0; k < len(s); k++ {
					sem = append(sem, b.yedekID[s[k]])
				}
				continue
			}
		}
		if b.bilinmeyen >= 0 {
			if bekleyen >= 0 && !b.birlestirBil {
				ekleBekleyen()
			}
			bekleyen = b.bilinmeyen
		}
	}
	ekleBekleyen()
	return sem
}

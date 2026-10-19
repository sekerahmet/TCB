package tokenize

import (
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"
)

// ============================================================================
// WordPiece, WordLevel ve Unigram modelleri
// ============================================================================

func init() {
	ModelKaydet("WordPiece", wordPieceKur)
	ModelKaydet("WordLevel", wordLevelKur)
	ModelKaydet("Unigram", unigramKur)
}

// ---------------------------------------------------------------------------
// WordPiece: en uzun önek önce (açgözlü) alt sözcük bölme.
// ---------------------------------------------------------------------------

type wordPiece struct {
	sozluk     map[string]int32
	bilinmeyen int32
	onek       string
	enCok      int
}

func wordPieceKur(ham json.RawMessage) (Model, error) {
	m := struct {
		UnkToken *string          `json:"unk_token"`
		Prefix   *string          `json:"continuing_subword_prefix"`
		MaxChars *int             `json:"max_input_chars_per_word"`
		Vocab    map[string]int64 `json:"vocab"`
	}{}
	if err := katiCoz(ham, &m); err != nil {
		return nil, err
	}
	s, err := sozlukDonustur(m.Vocab)
	if err != nil {
		return nil, err
	}
	w := &wordPiece{sozluk: s, onek: "##", enCok: 100}
	unk := "[UNK]"
	if m.UnkToken != nil {
		unk = *m.UnkToken
	}
	if m.Prefix != nil {
		w.onek = *m.Prefix
	}
	if m.MaxChars != nil {
		w.enCok = *m.MaxChars
	}
	id, ok := s[unk]
	if !ok {
		return nil, fmt.Errorf("bilinmeyen parça %q dağarcıkta yok", unk)
	}
	w.bilinmeyen = id
	return w, nil
}

func (w *wordPiece) Sozluk() map[string]int32 { return w.sozluk }

func (w *wordPiece) Kodla(dst []int32, p Parca) []int32 {
	m := p.Icerik()
	if utf8.RuneCountInString(m) > w.enCok {
		return append(dst, w.bilinmeyen)
	}
	n := len(dst)
	for bas := 0; bas < len(m); {
		son := len(m)
		bulundu := int32(-1)
		for bas < son {
			alt := m[bas:son]
			if bas > 0 {
				alt = w.onek + alt
			}
			if id, ok := w.sozluk[alt]; ok {
				bulundu = id
				break
			}
			_, k := utf8.DecodeLastRuneInString(m[bas:son])
			son -= k
		}
		if bulundu < 0 {
			return append(dst[:n], w.bilinmeyen)
		}
		dst = append(dst, bulundu)
		bas = son
	}
	return dst
}

// ---------------------------------------------------------------------------
// WordLevel: parçanın tamamı sözlükte aranır, yoksa bilinmeyen token.
// ---------------------------------------------------------------------------

type wordLevel struct {
	sozluk     map[string]int32
	bilinmeyen int32
	unkAd      string
}

func wordLevelKur(ham json.RawMessage) (Model, error) {
	m := struct {
		UnkToken *string          `json:"unk_token"`
		Vocab    map[string]int64 `json:"vocab"`
	}{}
	if err := katiCoz(ham, &m); err != nil {
		return nil, err
	}
	s, err := sozlukDonustur(m.Vocab)
	if err != nil {
		return nil, err
	}
	w := &wordLevel{sozluk: s, bilinmeyen: -1, unkAd: "<unk>"}
	if m.UnkToken != nil {
		w.unkAd = *m.UnkToken
	}
	if id, ok := s[w.unkAd]; ok {
		w.bilinmeyen = id
	}
	return w, nil
}

func (w *wordLevel) Sozluk() map[string]int32 { return w.sozluk }

func (w *wordLevel) Kodla(dst []int32, p Parca) []int32 {
	if id, ok := w.sozluk[p.Icerik()]; ok {
		return append(dst, id)
	}
	if w.bilinmeyen >= 0 {
		return append(dst, w.bilinmeyen)
	}
	// Başvuru uygulaması burada hata verir; kodlama sürdürülebilsin diye
	// parça atlanır.
	return dst
}

// ---------------------------------------------------------------------------
// Unigram: puanlı sözlükte en yüksek toplam puanlı bölümleme (Viterbi).
// ---------------------------------------------------------------------------

const unigramBilinmeyenCezasi = 10.0

type unigram struct {
	sozluk     map[string]int32
	puan       []float64
	tokenlar   []string
	bilinmeyen int32
	baytYedegi bool
	enDusuk    float64
	kok        *unigramDugum
	yedekID    [256]int32
	onbellek   *onbellek
}

// unigramDugum bayt tabanlı önek ağacının düğümü.
type unigramDugum struct {
	cocuk map[byte]*unigramDugum
	id    int32 // -1: burada biten token yok
}

func unigramKur(ham json.RawMessage) (Model, error) {
	var m struct {
		UnkID        *int              `json:"unk_id"`
		Vocab        []json.RawMessage `json:"vocab"`
		ByteFallback bool              `json:"byte_fallback"`
	}
	if err := katiCoz(ham, &m); err != nil {
		return nil, err
	}
	u := &unigram{
		sozluk:     make(map[string]int32, len(m.Vocab)),
		bilinmeyen: -1,
		baytYedegi: m.ByteFallback,
		enDusuk:    math.Inf(1),
		kok:        &unigramDugum{id: -1},
		onbellek:   yeniOnbellek(VarsayilanOnbellekKapasitesi),
	}
	for i, h := range m.Vocab {
		var cift []json.RawMessage
		if err := json.Unmarshal(h, &cift); err != nil || len(cift) != 2 {
			return nil, fmt.Errorf("%d. dağarcık girdisi [parça, puan] biçiminde değil", i)
		}
		var tok string
		var puan float64
		if err := json.Unmarshal(cift[0], &tok); err != nil {
			return nil, fmt.Errorf("%d. dağarcık girdisi: %w", i, err)
		}
		if err := json.Unmarshal(cift[1], &puan); err != nil {
			return nil, fmt.Errorf("%d. dağarcık girdisi: %w", i, err)
		}
		u.tokenlar = append(u.tokenlar, tok)
		u.puan = append(u.puan, puan)
		u.sozluk[tok] = int32(i)
		if puan < u.enDusuk {
			u.enDusuk = puan
		}
		d := u.kok
		for k := 0; k < len(tok); k++ {
			if d.cocuk == nil {
				d.cocuk = make(map[byte]*unigramDugum)
			}
			c, ok := d.cocuk[tok[k]]
			if !ok {
				c = &unigramDugum{id: -1}
				d.cocuk[tok[k]] = c
			}
			d = c
		}
		d.id = int32(i) // aynı token yineleniyorsa sonuncusu geçerlidir
	}
	if m.UnkID != nil {
		if *m.UnkID < 0 || *m.UnkID >= len(u.tokenlar) {
			return nil, fmt.Errorf("unk_id %d dağarcık dışında", *m.UnkID)
		}
		u.bilinmeyen = int32(*m.UnkID)
	}
	for c := 0; c < 256; c++ {
		u.yedekID[c] = -1
		if id, ok := u.sozluk[fmt.Sprintf("<0x%02X>", c)]; ok {
			u.yedekID[c] = id
		}
	}
	return u, nil
}

func (u *unigram) Sozluk() map[string]int32 { return u.sozluk }

func (u *unigram) Kodla(dst []int32, p Parca) []int32 {
	m := p.Icerik()
	o := u.onbellek
	if o != nil && len(m) <= onbellekAzamiUzunluk {
		if v, ok := o.al(m); ok {
			return append(dst, v...)
		}
		n := len(dst)
		dst = u.kodla(dst, m)
		o.koy(m, dst[n:])
		return dst
	}
	return u.kodla(dst, m)
}

func (u *unigram) kodla(dst []int32, m string) []int32 {
	if m == "" {
		return dst
	}
	type dugum struct {
		id   int32
		puan float64
		bas  int
		dolu bool
	}
	n := len(m)
	enIyi := make([]dugum, n+1)
	bilPuan := u.enDusuk - unigramBilinmeyenCezasi
	for bas := 0; bas < n; {
		_, kw := utf8.DecodeRuneInString(m[bas:])
		temel := enIyi[bas].puan
		tekVar := false
		d := u.kok
		for k := bas; k < n && d != nil; k++ {
			d = d.cocuk[m[k]]
			if d == nil {
				break
			}
			if d.id >= 0 {
				son := k + 1
				aday := temel + u.puan[d.id]
				h := &enIyi[son]
				if !h.dolu || aday > h.puan {
					*h = dugum{id: d.id, puan: aday, bas: bas, dolu: true}
				}
				if son-bas == kw {
					tekVar = true
				}
			}
		}
		if !tekVar {
			if u.bilinmeyen < 0 {
				// Başvuru uygulaması burada hata verir; karakter atlanmış sayılır.
				h := &enIyi[bas+kw]
				if !h.dolu || temel > h.puan {
					*h = dugum{id: -1, puan: temel, bas: bas, dolu: true}
				}
			} else {
				aday := temel + bilPuan
				h := &enIyi[bas+kw]
				if !h.dolu || aday > h.puan {
					*h = dugum{id: u.bilinmeyen, puan: aday, bas: bas, dolu: true}
				}
			}
		}
		bas += kw
	}
	// Geriye doğru yol çıkarımı; ardışık bilinmeyenler birleştirilir.
	type bolum struct {
		bas, son int
		id       int32
	}
	var ters []bolum
	for son := n; son > 0; {
		h := enIyi[son]
		if h.id >= 0 && h.id == u.bilinmeyen && len(ters) > 0 && ters[len(ters)-1].id == u.bilinmeyen {
			ters[len(ters)-1].bas = h.bas
		} else {
			ters = append(ters, bolum{h.bas, son, h.id})
		}
		son = h.bas
	}
	for i := len(ters) - 1; i >= 0; i-- {
		b := ters[i]
		if b.id < 0 {
			continue
		}
		parca := m[b.bas:b.son]
		if id, ok := u.sozluk[parca]; ok {
			dst = append(dst, id)
			continue
		}
		if u.baytYedegi {
			tamam := true
			for k := 0; k < len(parca); k++ {
				if u.yedekID[parca[k]] < 0 {
					tamam = false
					break
				}
			}
			if tamam {
				for k := 0; k < len(parca); k++ {
					dst = append(dst, u.yedekID[parca[k]])
				}
				continue
			}
		}
		dst = append(dst, u.bilinmeyen)
	}
	return dst
}

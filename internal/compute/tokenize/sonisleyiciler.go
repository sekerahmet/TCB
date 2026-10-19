package tokenize

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// Son işleyiciler (post_processor)
// ============================================================================

func init() {
	SonIsleyiciKaydet("TemplateProcessing", sablonKur)
	SonIsleyiciKaydet("BertProcessing", func(ham json.RawMessage, _ func(string) (int32, bool)) (SonIsleyici, error) {
		return clsSepKur(ham, false)
	})
	SonIsleyiciKaydet("RobertaProcessing", func(ham json.RawMessage, _ func(string) (int32, bool)) (SonIsleyici, error) {
		return clsSepKur(ham, true)
	})
	SonIsleyiciKaydet("ByteLevel", func(json.RawMessage, func(string) (int32, bool)) (SonIsleyici, error) {
		return etkisizIsleyici{}, nil
	})
	SonIsleyiciKaydet("Sequence", sirIsleyiciKur)
}

// varsayilanIsle son işleyici yokken kullanılır: A'nın tür kimliği 0, B'ninki 1.
func varsayilanIsle(a, b *Kodlama) *Kodlama {
	if b == nil {
		return a
	}
	out := &Kodlama{}
	out.ekle(a, 0, false)
	out.ekle(b, 1, false)
	return out
}

func (k *Kodlama) ekle(d *Kodlama, tur int32, ozel bool) {
	for range d.Idler {
		k.TurIdleri = append(k.TurIdleri, tur)
		k.OzelMaske = append(k.OzelMaske, ozel)
	}
	k.Idler = append(k.Idler, d.Idler...)
}

func (k *Kodlama) ozelEkle(ids []int32, tur int32) {
	for _, id := range ids {
		k.Idler = append(k.Idler, id)
		k.TurIdleri = append(k.TurIdleri, tur)
		k.OzelMaske = append(k.OzelMaske, true)
	}
}

type etkisizIsleyici struct{}

func (etkisizIsleyici) Isle(a, b *Kodlama, _ bool) *Kodlama { return varsayilanIsle(a, b) }

// ---------------------------------------------------------------------------
// TemplateProcessing: "[CLS] $A [SEP]" gibi şablonlar.
// ---------------------------------------------------------------------------

type sablonParcasi struct {
	dizi    int // 0: A, 1: B, -1: özel token
	ozel    []int32
	turKimi int32
}

type sablon struct {
	tek, cift []sablonParcasi
}

func sablonKur(ham json.RawMessage, _ func(string) (int32, bool)) (SonIsleyici, error) {
	var t struct {
		Single        []json.RawMessage `json:"single"`
		Pair          []json.RawMessage `json:"pair"`
		SpecialTokens map[string]struct {
			ID  string  `json:"id"`
			Ids []int64 `json:"ids"`
		} `json:"special_tokens"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	coz := func(liste []json.RawMessage) ([]sablonParcasi, error) {
		var out []sablonParcasi
		for _, h := range liste {
			var p struct {
				Sequence *struct {
					ID     string `json:"id"`
					TypeID int32  `json:"type_id"`
				} `json:"Sequence"`
				SpecialToken *struct {
					ID     string `json:"id"`
					TypeID int32  `json:"type_id"`
				} `json:"SpecialToken"`
			}
			if err := json.Unmarshal(h, &p); err != nil {
				return nil, fmt.Errorf("şablon öğesi çözümlenemedi: %w", err)
			}
			switch {
			case p.Sequence != nil:
				d := 0
				switch p.Sequence.ID {
				case "A":
				case "B":
					d = 1
				default:
					return nil, fmt.Errorf("bilinmeyen dizi kimliği %q", p.Sequence.ID)
				}
				out = append(out, sablonParcasi{dizi: d, turKimi: p.Sequence.TypeID})
			case p.SpecialToken != nil:
				o, ok := t.SpecialTokens[p.SpecialToken.ID]
				if !ok {
					return nil, fmt.Errorf("şablondaki özel token %q tanımlı değil", p.SpecialToken.ID)
				}
				var ids []int32
				for _, x := range o.Ids {
					ids = append(ids, int32(x))
				}
				out = append(out, sablonParcasi{dizi: -1, ozel: ids, turKimi: p.SpecialToken.TypeID})
			default:
				return nil, fmt.Errorf("şablon öğesi Sequence ya da SpecialToken olmalı")
			}
		}
		return out, nil
	}
	s := &sablon{}
	var err error
	if s.tek, err = coz(t.Single); err != nil {
		return nil, err
	}
	if s.cift, err = coz(t.Pair); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *sablon) Isle(a, b *Kodlama, ozelEkle bool) *Kodlama {
	sb := s.tek
	if b != nil {
		sb = s.cift
	}
	out := &Kodlama{}
	for _, p := range sb {
		switch {
		case p.dizi == 0:
			out.ekle(a, p.turKimi, false)
		case p.dizi == 1 && b != nil:
			out.ekle(b, p.turKimi, false)
		case p.dizi < 0 && ozelEkle:
			out.ozelEkle(p.ozel, p.turKimi)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// BertProcessing / RobertaProcessing
// ---------------------------------------------------------------------------

type clsSep struct {
	cls, sep int32
	roberta  bool
}

func clsSepKur(ham json.RawMessage, roberta bool) (SonIsleyici, error) {
	var t struct {
		Sep []json.RawMessage `json:"sep"`
		Cls []json.RawMessage `json:"cls"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	oku := func(c []json.RawMessage, ad string) (int32, error) {
		if len(c) != 2 {
			return 0, fmt.Errorf("%s [parça, kimlik] biçiminde olmalı", ad)
		}
		var id int32
		if err := json.Unmarshal(c[1], &id); err != nil {
			return 0, fmt.Errorf("%s kimliği okunamadı: %w", ad, err)
		}
		return id, nil
	}
	c := &clsSep{roberta: roberta}
	var err error
	if c.sep, err = oku(t.Sep, "sep"); err != nil {
		return nil, err
	}
	if c.cls, err = oku(t.Cls, "cls"); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *clsSep) Isle(a, b *Kodlama, ozelEkle bool) *Kodlama {
	ikinciTur := int32(1)
	if c.roberta {
		ikinciTur = 0
	}
	out := &Kodlama{}
	if !ozelEkle {
		out.ekle(a, 0, false)
		if b != nil {
			out.ekle(b, ikinciTur, false)
		}
		return out
	}
	out.ozelEkle([]int32{c.cls}, 0)
	out.ekle(a, 0, false)
	out.ozelEkle([]int32{c.sep}, 0)
	if b != nil {
		if c.roberta {
			out.ozelEkle([]int32{c.sep}, 0)
		}
		out.ekle(b, ikinciTur, false)
		out.ozelEkle([]int32{c.sep}, ikinciTur)
	}
	return out
}

// ---------------------------------------------------------------------------
// Sequence: konumları etkileyen etkisiz adımlar atlanır; en çok bir gerçek
// son işleyiciye izin verilir.
// ---------------------------------------------------------------------------

func sirIsleyiciKur(ham json.RawMessage, sozluk func(string) (int32, bool)) (SonIsleyici, error) {
	var t struct {
		Processors []json.RawMessage `json:"processors"`
	}
	if err := katiCoz(ham, &t); err != nil {
		return nil, err
	}
	var gercek SonIsleyici = etkisizIsleyici{}
	sayi := 0
	for i, h := range t.Processors {
		s, err := SonIsleyiciKur(h, sozluk)
		if err != nil {
			return nil, fmt.Errorf("%d. adım: %w", i, err)
		}
		if _, ok := s.(etkisizIsleyici); ok || s == nil {
			continue
		}
		gercek = s
		sayi++
	}
	if sayi > 1 {
		return nil, fmt.Errorf("birden çok özel parça ekleyen son işleyici içeren dizi desteklenmiyor")
	}
	return gercek, nil
}

package tokenize

import (
	"encoding/json"
	"fmt"
	"math"
)

// ============================================================================
// Son işleyiciler (post_processor)
// ============================================================================

func init() {
	RegisterPostProcessor("TemplateProcessing", newTemplate)
	RegisterPostProcessor("BertProcessing", func(raw json.RawMessage, _ func(string) (int32, bool)) (PostProcessor, error) {
		return newClsSep(raw, false)
	})
	RegisterPostProcessor("RobertaProcessing", func(raw json.RawMessage, _ func(string) (int32, bool)) (PostProcessor, error) {
		return newClsSep(raw, true)
	})
	RegisterPostProcessor("ByteLevel", func(json.RawMessage, func(string) (int32, bool)) (PostProcessor, error) {
		return noopProcessor{}, nil
	})
	RegisterPostProcessor("Sequence", newSeqProcessor)
}

// specialIDLister son işleyicinin eklediği özel token kimliklerini verir; yükleyici hepsinin
// sözlükte olduğunu denetler (olmayan bir kimlik modele sözlük dışı bir girdi verirdi).
type specialIDLister interface{ specialIDs() []int32 }

var (
	_ specialIDLister = (*template)(nil)
	_ specialIDLister = (*clsSep)(nil)
)

// defaultProcess son işleyici yokken kullanılır: A'nın tür kimliği 0, B'ninki 1.
func defaultProcess(a, b *Encoding) *Encoding {
	if b == nil {
		return a
	}
	out := &Encoding{}
	out.add(a, 0, false)
	out.add(b, 1, false)
	return out
}

func (k *Encoding) add(d *Encoding, kind int32, special bool) {
	for range d.IDs {
		k.TypeIDs = append(k.TypeIDs, kind)
		k.SpecialMask = append(k.SpecialMask, special)
	}
	k.IDs = append(k.IDs, d.IDs...)
}

func (k *Encoding) addSpecial(ids []int32, kind int32) {
	for _, id := range ids {
		k.IDs = append(k.IDs, id)
		k.TypeIDs = append(k.TypeIDs, kind)
		k.SpecialMask = append(k.SpecialMask, true)
	}
}

type noopProcessor struct{}

func (noopProcessor) Process(a, b *Encoding, _ bool) *Encoding { return defaultProcess(a, b) }

// ---------------------------------------------------------------------------
// TemplateProcessing: "[CLS] $A [SEP]" gibi şablonlar.
// ---------------------------------------------------------------------------

type templatePiece struct {
	arr     int // 0: A, 1: B, -1: özel token
	special []int32
	kindID  int32
}

type template struct {
	single, pair []templatePiece
}

func newTemplate(raw json.RawMessage, _ func(string) (int32, bool)) (PostProcessor, error) {
	var t struct {
		Single        []json.RawMessage `json:"single"`
		Pair          []json.RawMessage `json:"pair"`
		SpecialTokens map[string]struct {
			ID  string  `json:"id"`
			Ids []int64 `json:"ids"`
		} `json:"special_tokens"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	decode := func(list []json.RawMessage) ([]templatePiece, error) {
		var out []templatePiece
		for _, h := range list {
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
				out = append(out, templatePiece{arr: d, kindID: p.Sequence.TypeID})
			case p.SpecialToken != nil:
				o, ok := t.SpecialTokens[p.SpecialToken.ID]
				if !ok {
					return nil, fmt.Errorf("şablondaki özel token %q tanımlı değil", p.SpecialToken.ID)
				}
				var ids []int32
				for _, x := range o.Ids {
					if x < 0 || x > math.MaxInt32 {
						return nil, fmt.Errorf("şablondaki özel token %q kimliği geçersiz: %d", p.SpecialToken.ID, x)
					}
					ids = append(ids, int32(x))
				}
				out = append(out, templatePiece{arr: -1, special: ids, kindID: p.SpecialToken.TypeID})
			default:
				return nil, fmt.Errorf("şablon öğesi Sequence ya da SpecialToken olmalı")
			}
		}
		return out, nil
	}
	s := &template{}
	var err error
	if s.single, err = decode(t.Single); err != nil {
		return nil, err
	}
	if s.pair, err = decode(t.Pair); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *template) specialIDs() []int32 {
	var ids []int32
	for _, list := range [][]templatePiece{s.single, s.pair} {
		for _, p := range list {
			ids = append(ids, p.special...)
		}
	}
	return ids
}

func (s *template) Process(a, b *Encoding, addSpecial bool) *Encoding {
	sb := s.single
	if b != nil {
		sb = s.pair
	}
	out := &Encoding{}
	for _, p := range sb {
		switch {
		case p.arr == 0:
			out.add(a, p.kindID, false)
		case p.arr == 1 && b != nil:
			out.add(b, p.kindID, false)
		case p.arr < 0 && addSpecial:
			out.addSpecial(p.special, p.kindID)
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

func newClsSep(raw json.RawMessage, roberta bool) (PostProcessor, error) {
	var t struct {
		Sep []json.RawMessage `json:"sep"`
		Cls []json.RawMessage `json:"cls"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	read := func(c []json.RawMessage, ad string) (int32, error) {
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
	if c.sep, err = read(t.Sep, "sep"); err != nil {
		return nil, err
	}
	if c.cls, err = read(t.Cls, "cls"); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *clsSep) specialIDs() []int32 { return []int32{c.cls, c.sep} }

func (c *clsSep) Process(a, b *Encoding, addSpecial bool) *Encoding {
	secondKind := int32(1)
	if c.roberta {
		secondKind = 0
	}
	out := &Encoding{}
	if !addSpecial {
		out.add(a, 0, false)
		if b != nil {
			out.add(b, secondKind, false)
		}
		return out
	}
	out.addSpecial([]int32{c.cls}, 0)
	out.add(a, 0, false)
	out.addSpecial([]int32{c.sep}, 0)
	if b != nil {
		if c.roberta {
			out.addSpecial([]int32{c.sep}, 0)
		}
		out.add(b, secondKind, false)
		out.addSpecial([]int32{c.sep}, secondKind)
	}
	return out
}

// ---------------------------------------------------------------------------
// Sequence: konumları etkileyen etkisiz adımlar atlanır; en çok bir gerçek
// son işleyiciye izin verilir.
// ---------------------------------------------------------------------------

func newSeqProcessor(raw json.RawMessage, vocab func(string) (int32, bool)) (PostProcessor, error) {
	var t struct {
		Processors []json.RawMessage `json:"processors"`
	}
	if err := strictDecode(raw, &t); err != nil {
		return nil, err
	}
	var actual PostProcessor = noopProcessor{}
	num := 0
	for i, h := range t.Processors {
		s, err := NewPostProcessor(h, vocab)
		if err != nil {
			return nil, fmt.Errorf("%d. adım: %w", i, err)
		}
		if _, ok := s.(noopProcessor); ok || s == nil {
			continue
		}
		actual = s
		num++
	}
	if num > 1 {
		return nil, fmt.Errorf("birden çok özel parça ekleyen son işleyici içeren dizi desteklenmiyor")
	}
	return actual, nil
}

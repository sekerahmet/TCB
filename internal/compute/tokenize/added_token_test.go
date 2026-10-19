package tokenize

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// ekliParcalayici testdata'daki bir parçalayıcıya verilen içeriklerle
// normalized=true ek tokenlar ekler (kimlikler sözlüğün sonundan).
func withAddedTokenizer(t *testing.T, file string, contents ...string) *Tokenizer {
	t.Helper()
	data, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	id := len(p.idValues)
	extras, _ := raw["added_tokens"].([]any)
	for i, c := range contents {
		extras = append(extras, map[string]any{"id": id + i, "content": c, "single_word": false, "lstrip": false,
			"rstrip": false, "normalized": true, "special": false})
	}
	raw["added_tokens"] = extras
	fresh, _ := json.Marshal(raw)
	if p, err = Parse(fresh); err != nil {
		t.Fatal(err)
	}
	return p
}

// normalized=true ek tokenlar normalleştirilmiş metinde, içerikleri de
// normalleştirilerek aranır; token metinleri normalleştirilmiş içeriktir.
// Beklenen değerler başvuru uygulamasının çıktısıdır.
func TestNormalizedAddedTokens(t *testing.T) {
	cases := []struct {
		file   string
		text   string
		idList []int
		parsed string
	}{
		// Küçük harfe çeviren normalleştirici: "ÇAY" içeriği "çay" olarak aranır.
		{"p_kelime_bpe.json", "çay ÇAY", []int{1, 802, 802, 2}, "çayçay"},
		{"p_kelime_bpe.json", " <|x|> yeni ÇAY ", []int{1, 800, 801, 802, 2}, "<|x|>yeniçay"},
		{"p_kelime_bpe.json", "Yeni\tYeni</s>x<|x|>ÇAY", []int{1, 801, 801, 2, 145, 800, 802, 2}, "yeniyenix <|x|>çay"},
		// Başa "▁" ekleyen normalleştirici: "yeni" içeriği "▁yeni" olarak aranır.
		{"p_llama_tarzi.json", "ayeni", []int{1, 214, 77, 42}, "ayeni"},
		{"p_llama_tarzi.json", "a yeni ÇAY", []int{1, 301, 648, 649}, "a yeni ÇAY"},
		{"p_llama_tarzi.json", "yeni", []int{1, 648}, "yeni"},
	}
	for _, d := range cases {
		p := withAddedTokenizer(t, d.file, "<|x|>", "yeni", "ÇAY")
		ids, err := p.SafeEncode(d.text, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, d.idList) {
			t.Errorf("%s %q: kimlikler %v, beklenen %v", d.file, d.text, ids, d.idList)
		}
		if got := p.Decode(ids, true); got != d.parsed {
			t.Errorf("%s %q: çözüm %q, beklenen %q", d.file, d.text, got, d.parsed)
		}
	}
	// Kimlik -> token normalleştirilmiş, token -> kimlik özgün içerikle.
	p := withAddedTokenizer(t, "p_kelime_bpe.json", "<|x|>", "yeni", "ÇAY")
	if s, _ := p.IDToToken(802); s != "çay" {
		t.Errorf("KimlikToken(802) = %q, beklenen \"çay\"", s)
	}
	if id, ok := p.TokenToID("ÇAY"); !ok || id != 802 {
		t.Errorf("TokenKimligi(ÇAY) = %d %v", id, ok)
	}
	if _, ok := p.TokenToID("çay"); ok {
		t.Error("TokenKimligi(çay) ek token olarak bulunmamalı")
	}
}

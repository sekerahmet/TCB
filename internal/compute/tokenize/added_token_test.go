package tokenize

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// withAddedTokenizer testdata'daki bir parçalayıcıya verilen içeriklerle
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
		{"tokenizer_word_bpe.json", "çay ÇAY", []int{1, 802, 802, 2}, "çayçay"},
		{"tokenizer_word_bpe.json", " <|x|> yeni ÇAY ", []int{1, 800, 801, 802, 2}, "<|x|>yeniçay"},
		{"tokenizer_word_bpe.json", "Yeni\tYeni</s>x<|x|>ÇAY", []int{1, 801, 801, 2, 145, 800, 802, 2}, "yeniyenix <|x|>çay"},
		// Başa "▁" ekleyen normalleştirici: "yeni" içeriği "▁yeni" olarak aranır.
		{"tokenizer_llama_style.json", "ayeni", []int{1, 214, 77, 42}, "ayeni"},
		{"tokenizer_llama_style.json", "a yeni ÇAY", []int{1, 301, 648, 649}, "a yeni ÇAY"},
		{"tokenizer_llama_style.json", "yeni", []int{1, 648}, "yeni"},
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
	// Kimlik -> token normalleştirilmiş; token -> kimlik özgün içerikle de normalleştirilmiş içerikle de
	// (gidiş-dönüş: TestIDTokenRoundTrip).
	p := withAddedTokenizer(t, "tokenizer_word_bpe.json", "<|x|>", "yeni", "ÇAY")
	if s, _ := p.IDToToken(802); s != "çay" {
		t.Errorf("KimlikToken(802) = %q, beklenen \"çay\"", s)
	}
	for _, s := range []string{"ÇAY", "çay"} {
		if id, ok := p.TokenToID(s); !ok || id != 802 {
			t.Errorf("TokenKimligi(%s) = %d %v", s, id, ok)
		}
	}
}

// Kural: her kimlik için TokenToID(IDToToken(id)) == id. normalized=true ek tokenın token metni
// normalleştirilmiş içeriğidir ("ÇAY" Lowercase'le "çay"); TokenToID onu da tanır (önce ek tokenın
// özgün içeriği, sonra sözlük, sonra normalleştirilmiş içerik). HF tokenizers iki sürümde de
// gidiş-dönüşü tutar: v0.21.0 iki yönde özgün içerikle (added_vocabulary.rs), fb9429ee iki yönde
// normalleştirilmiş içerikle (bucket_added_vocabulary.rs: token_to_id önce vocab, sonra
// normalized_vocab). TR kimlikten normalleştirilmiş metni verir, metinden ikisini de tanır.
func TestIDTokenRoundTrip(t *testing.T) {
	p, err := Parse([]byte(`{"version":"1.0","truncation":null,"padding":null,
"added_tokens":[{"id":2,"content":"ÇAY","single_word":false,"lstrip":false,"rstrip":false,"normalized":true,"special":false}],
"normalizer":{"type":"Lowercase"},"pre_tokenizer":null,"post_processor":null,"decoder":null,
"model":{"type":"WordLevel","unk_token":"<unk>","vocab":{"<unk>":0,"a":1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for id := 0; id < 3; id++ {
		s, ok := p.IDToToken(id)
		if back, found := p.TokenToID(s); !ok || !found || back != id {
			t.Errorf("IDToToken(%d) = %q, %v; TokenToID(%q) = %d, %v", id, s, ok, s, back, found)
		}
	}
	if id, ok := p.TokenToID("ÇAY"); !ok || id != 2 {
		t.Errorf("TokenToID(ÇAY) = %d, %v; özgün içerik de 2 vermeli", id, ok)
	}
}

package tokenize

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// plainFixtures eklenmiş parçası olan bütün sınama parçalayıcılarıdır (testdata ve tests/data).
func plainFixtures(t *testing.T) []string {
	t.Helper()
	a, _ := reporoot.Glob("testdata/tokenizer_*.json")
	b, _ := reporoot.Glob(filepath.Join(reporoot.Root(), "tests", "data", "tokenizer*.json"))
	files := append(a, b...)
	if len(files) < 12 {
		t.Fatalf("%d parçalayıcı bulundu, en az 12 olmalı", len(files))
	}
	return files
}

// encodePaths bir metni dört kodlama yolundan geçirir: tek metin, metin listesi, çiftin iki
// yanı ve çift listesi. Her yolun kimlikleri ayrı döner.
func encodePaths(t *testing.T, p *Tokenizer, text string, s Options) map[string][]int {
	t.Helper()
	out := map[string][]int{}
	ids, err := p.SafeEncode(text, s)
	must(t, err)
	out["tek"] = ids
	batch, errs := p.SafeEncodeBatch([]string{"a", text}, s, 2)
	mustAll(t, errs)
	out["liste"] = batch[1]
	other := "z"
	k, err := p.SafeEncodePair(text, &other, s)
	must(t, err)
	out["çift/ilk"] = ToInt(nil, k.IDs)
	k, err = p.SafeEncodePair(other, &text, s)
	must(t, err)
	out["çift/ikinci"] = ToInt(nil, k.IDs)
	pairs, errs := p.SafeEncodePairBatch([]string{"a", other}, []string{"b", text}, s, 2)
	mustAll(t, errs)
	out["çift listesi"] = pairs[1]
	return out
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustAll(t *testing.T, errs []error) {
	t.Helper()
	for _, err := range errs {
		must(t, err)
	}
}

// Kural (16.2, TR_special_tokens: TR_false): metindeki özel parça yazımı eşleşmez ve modelin
// kendi bölütlemesi de özel parçanın kimliğini üretmez; metin geri kalan parçalarla kodlanır.
// Bilinmeyen kimliği ayrıdır: bilinmeyen sözcüğün kimliği olarak çıkabilir, rol üretmez.
// Bütün fikstürlerde her eklenmiş parçanın yazımı tek başına ve metin içinde, dört yolda.
func TestPlainEncodingProducesNoSpecialID(t *testing.T) {
	plain := Options{DontParseSpecial: true, DontAddSpecial: true}
	for _, f := range plainFixtures(t) {
		p, err := Load(f)
		must(t, err)
		unk := unknownID(p.model)
		var texts []string
		for _, e := range p.AddedTokens() {
			texts = append(texts, e.Content, "x "+e.Content+" y")
		}
		if filepath.Base(f) == "tokenizer_unigram_normalized.json" {
			texts = append(texts, "x ＜|im_end|＞ y") // NFKC ile "<|im_end|>" olur: nrmMatch yolu
		}
		unknowns := 0
		for _, text := range texts {
			for path, ids := range encodePaths(t, p, text, plain) {
				for _, id := range ids {
					switch {
					case int32(id) == unk:
						unknowns++
					case p.IsSpecial(id):
						tok, _ := p.IDToToken(id)
						t.Errorf("%s, %s yolu, %q: özel parça %d (%q) çıktı: %v", filepath.Base(f), path, text, id, tok, ids)
					}
				}
			}
		}
		t.Logf("%s: %d metin, bilinmeyen kimliği %d kez", filepath.Base(f), len(texts), unknowns)
	}
}

// Kural (16.2): TR_true'nun davranışı değişmez; metindeki özel parça yazımı tek kimlik olur,
// normalleşince ona dönen yazım da (nrmMatch).
func TestSpecialTokensStillMatchByDefault(t *testing.T) {
	p, err := Load(filepath.Join(reporoot.Root(), "tests", "data", "tokenizer_unigram_normalized.json"))
	must(t, err)
	id, _ := p.TokenToID("<|im_end|>")
	for _, text := range []string{"x <|im_end|> y", "x ＜|im_end|＞ y"} {
		for path, ids := range encodePaths(t, p, text, Options{DontAddSpecial: true}) {
			if !slices.Contains(ids, id) {
				t.Errorf("%s yolu, %q: %d yok: %v", path, text, id, ids)
			}
		}
	}
}

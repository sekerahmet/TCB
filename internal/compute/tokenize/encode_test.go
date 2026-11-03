package tokenize

import (
	"encoding/json"
	"os"
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

// addedTexts her eklenmiş parçanın yazımını tek başına ve metin içinde verir.
func addedTexts(p *Tokenizer, file string) []string {
	var texts []string
	for _, e := range p.AddedTokens() {
		texts = append(texts, e.Content, "x "+e.Content+" y")
	}
	if filepath.Base(file) == "tokenizer_unigram_normalized.json" {
		texts = append(texts, "x ＜|im_end|＞ y") // NFKC ile "<|im_end|>" olur: nrmMatch yolu
	}
	return texts
}

// checkPlain bütün fikstürlerde addedTexts'i dört yoldan s ile kodlar; barred'ın tuttuğu kimlik
// çıkarsa hatadır. Bilinmeyen kimliği ayrı sayılır.
func checkPlain(t *testing.T, s Options, barred func(p *Tokenizer, id int) bool) {
	for _, f := range plainFixtures(t) {
		p, err := Load(f)
		must(t, err)
		unk := unknownID(p.model)
		texts := addedTexts(p, f)
		unknowns := 0
		for _, text := range texts {
			for path, ids := range encodePaths(t, p, text, s) {
				for _, id := range ids {
					switch {
					case int32(id) == unk:
						unknowns++
					case barred(p, id):
						tok, _ := p.IDToToken(id)
						t.Errorf("%s, %s yolu, %q: eklenmiş parça %d (%q) çıktı: %v", filepath.Base(f), path, text, id, tok, ids)
					}
				}
			}
		}
		t.Logf("%s: %d metin, bilinmeyen kimliği %d kez", filepath.Base(f), len(texts), unknowns)
	}
}

// Kural (16.2, DontParseSpecial; HF'nin split_special_tokens'ı): metindeki özel parça yazımı
// eşleşmez ve modelin kendi bölütlemesi de özel parçanın kimliğini üretmez; metin geri kalan
// parçalarla kodlanır. Bilinmeyen kimliği ayrıdır: bilinmeyen sözcüğün kimliği olarak çıkabilir,
// rol üretmez. Bütün fikstürlerde her eklenmiş parçanın yazımı tek başına ve metin içinde, dört yolda.
func TestPlainEncodingProducesNoSpecialID(t *testing.T) {
	checkPlain(t, Options{DontParseSpecial: true, DontAddSpecial: true}, func(p *Tokenizer, id int) bool { return p.IsSpecial(id) })
}

// Kural (16.2, TR_special_tokens: TR_false = DontParseAdded): özel olmayan eklenmiş parçalar da
// ("<think>" gibi) metinden kimlik olmaz, ne eşleşmeyle ne modelin bölütlemesiyle.
func TestPlainEncodingProducesNoAddedID(t *testing.T) {
	checkPlain(t, Options{DontParseAdded: true, DontAddSpecial: true}, func(p *Tokenizer, id int) bool {
		return slices.ContainsFunc(p.extras, func(e AddedToken) bool { return e.ID == id })
	})
}

// Kural (16.2): DontParseAdded'ın kimlikleri, aynı metnin özel olmayan eklenmiş parçaları
// çıkarılmış parçalayıcıda DontParseSpecial ile kodlamasıdır (fikstürlerde özel olmayan eklenmiş
// parçaların kimliği modelin dağarcığında yok, ikisi aynı olmalı).
func TestPlainEncodingMatchesTheTokenizerWithoutAddedTokens(t *testing.T) {
	for _, f := range plainFixtures(t) {
		p, err := Load(f)
		must(t, err)
		bare := withoutPlainAdded(t, f)
		for _, text := range addedTexts(p, f) {
			got := encodePaths(t, p, text, Options{DontParseAdded: true, DontAddSpecial: true})
			want := encodePaths(t, bare, text, Options{DontParseSpecial: true, DontAddSpecial: true})
			for path := range want {
				if !slices.Equal(got[path], want[path]) {
					t.Errorf("%s, %s yolu, %q: %v, beklenen %v", filepath.Base(f), path, text, got[path], want[path])
				}
			}
		}
	}
}

// withoutPlainAdded dosyadaki parçalayıcıyı özel olmayan eklenmiş parçaları olmadan kurar.
func withoutPlainAdded(t *testing.T, file string) *Tokenizer {
	t.Helper()
	data, err := os.ReadFile(file)
	must(t, err)
	var raw map[string]any
	must(t, json.Unmarshal(data, &raw))
	var kept []any
	extras, _ := raw["added_tokens"].([]any)
	for _, e := range extras {
		if e.(map[string]any)["special"] == true {
			kept = append(kept, e)
		}
	}
	raw["added_tokens"] = kept
	data, err = json.Marshal(raw)
	must(t, err)
	p, err := Parse(data)
	must(t, err)
	return p
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

// Kural (16.2): dağarcıkta parça olan özel olmayan eklenmiş parça da (Qwen3'ün "<think>"'i gibi)
// DontParseAdded ile bölütlemeden çıkmaz. Unigram'ın "</s>"'si özel olmayan yapılır: DontParseSpecial
// onu (HF gibi) eşler, DontParseAdded ne eşler ne de kafeste kullanır.
func TestPlainEncodingBarsAnAddedTokenThatIsAVocabularyPiece(t *testing.T) {
	data, err := os.ReadFile("testdata/tokenizer_unigram.json")
	must(t, err)
	var raw map[string]any
	must(t, json.Unmarshal(data, &raw))
	for _, e := range raw["added_tokens"].([]any) {
		if e := e.(map[string]any); e["content"] == "</s>" {
			e["special"] = false
		}
	}
	data, err = json.Marshal(raw)
	must(t, err)
	p, err := Parse(data)
	must(t, err)
	end, _ := p.TokenToID("</s>")
	if ids := p.EncodeWithOptions("x </s> y", Options{DontParseSpecial: true, DontAddSpecial: true}); !slices.Contains(ids, end) {
		t.Fatalf("DontParseSpecial özel olmayan </s>'yi eşlemeli: %v", ids)
	}
	for path, ids := range encodePaths(t, p, "x </s> y", Options{DontParseAdded: true, DontAddSpecial: true}) {
		if slices.Contains(ids, end) {
			t.Errorf("%s yolu: </s> (%d) çıktı: %v", path, end, ids)
		}
	}
}

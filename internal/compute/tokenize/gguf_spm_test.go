package tokenize

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// spmPiece SentencePiece sözlüğünün bir parçasıdır.
type spmPiece struct {
	text  string
	kind  int32
	score float32
}

// spmVocab Gemma 3'ün biçiminde küçük bir SentencePiece GGUF'udur: 0 <pad>, 1 <eos>, 2 <bos>, 3 <unk> denetim
// parçası; 4–259 bayt parçaları ("<0xXX>", tür 6); 260'tan başlayarak pieces. bos 2, eos 1.
type spmVocab struct {
	arch   string
	pieces []spmPiece
	addBOS bool
}

const spmFirst = 260

func (v spmVocab) builder() *gguftest.Builder {
	tokens := []string{"<pad>", "<eos>", "<bos>", "<unk>"}
	kinds := []int32{3, 3, 3, 3}
	scores := []float32{-1000, -1000, -1000, -1000}
	for b := range 256 {
		tokens, kinds, scores = append(tokens, fmt.Sprintf("<0x%02X>", b)), append(kinds, 6), append(scores, 0)
	}
	for _, p := range v.pieces {
		tokens, kinds, scores = append(tokens, p.text), append(kinds, p.kind), append(scores, p.score)
	}
	b := &gguftest.Builder{}
	b.KV(ggufKeyArch, v.arch).KV(ggufKeyModel, ggufModelSPM).KV(ggufKeyPre, "default").KV(ggufKeyTokens, tokens).
		KV(ggufKeyTypes, kinds).KV(ggufKeyScores, scores)
	return b.KV(ggufKeyAddBOS, v.addBOS).KV(ggufKeyBOS, uint32(2)).KV(ggufKeyEOS, uint32(1))
}

func mustSPM(t *testing.T, v spmVocab) *Tokenizer {
	t.Helper()
	p, err := parseGGUF(t, v.builder())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func normal(texts ...string) []spmPiece {
	out := make([]spmPiece, len(texts))
	for i, s := range texts {
		out[i] = spmPiece{s, ggufTokenNormal, -float32(i)}
	}
	return out
}

// trFalse TR_special_tokens: TR_false'un seçenekleridir (internal/stdlib/tokenizer/tokenizer.go:323).
var trFalse = Options{DontParseAdded: true, DontAddSpecial: true}

// Kural (16.1, S-5'in H6678'i): SentencePiece GGUF'u (tokenizer.ggml.model llama) yalnız gemma3 mimarisinde okunur;
// başka mimaride H6678, metni iki dilde gemma3'ü söyler. Etiket zorunludur (yoksa H6681), bilinmeyen etiket H6679,
// puan dizisi zorunludur (yoksa H6681).
func TestGGUFSPMIsReadOnlyForGemma3(t *testing.T) {
	v := spmVocab{arch: "gemma3", pieces: normal("a", "b", "▁")}
	if p := mustSPM(t, v); p.pretokenizerTag != "default" {
		t.Errorf("gemma3: etiket %q", p.pretokenizerTag)
	}
	v.arch = "llama"
	_, err := parseGGUF(t, v.builder())
	tr := "`llama` (SentencePiece) yalnız `general.architecture` `gemma3` olan dosyada okunur (16.1)"
	en := "`llama` (SentencePiece) is read only in a file whose `general.architecture` is `gemma3` (16.1)"
	if !messages.Has(err, messages.H6678) || !strings.Contains(err.Error(), tr) || !strings.Contains(messages.ErrorIn(err, "en"), en) {
		t.Errorf("llama mimarisi: %v / %s", err, messages.ErrorIn(err, "en"))
	}
	base := func() *gguftest.Builder {
		return (&gguftest.Builder{}).KV(ggufKeyArch, "gemma3").KV(ggufKeyModel, ggufModelSPM).
			KV(ggufKeyTokens, []string{"a"}).KV(ggufKeyTypes, []int32{1})
	}
	for _, c := range []struct {
		name string
		b    *gguftest.Builder
		code messages.Code
		key  string
	}{
		{"etiket yok", base().KV(ggufKeyScores, []float32{0}), messages.H6681, ggufKeyPre},
		{"bilinmeyen etiket", base().KV(ggufKeyPre, "llama-bpe").KV(ggufKeyScores, []float32{0}), messages.H6679, "llama-bpe"},
		{"puan yok", base().KV(ggufKeyPre, "default"), messages.H6681, ggufKeyScores},
	} {
		if _, err := parseGGUF(t, c.b); !messages.Has(err, c.code) || !strings.Contains(err.Error(), c.key) {
			t.Errorf("%s: %v, beklenen %s (%s)", c.name, err, c.code, c.key)
		}
	}
	// H6679'un listesi SentencePiece'in etiketini de sayar (C4 (a), TCB 2026-11-13 15:01).
	_, err = parseGGUF(t, base().KV(ggufKeyPre, "llama-bpe").KV(ggufKeyScores, []float32{0}))
	if !strings.Contains(err.Error(), "SentencePiece'te `default`") || !strings.Contains(messages.ErrorIn(err, "en"), "for SentencePiece `default`") {
		t.Errorf("H6679 SentencePiece'in etiketini saymalı: %v / %s", err, messages.ErrorIn(err, "en"))
	}
}

// Kural (D1 Gemma 3 tasarımı 1.7; SentencePiece): birleştirme yüksek puanlı parçayı önce kurar. "abc"'de ab ile bc
// yarışır: bc'nin puanı yüksekse [a, bc], ab'ninki yüksekse [ab, c].
func TestGGUFSPMMergesFollowTheScores(t *testing.T) {
	a, c, ab, bc := spmFirst, spmFirst+2, spmFirst+3, spmFirst+4
	for _, k := range []struct {
		abScore, bcScore float32
		want             []int
	}{{-5, -4, []int{a, bc}}, {-4, -5, []int{ab, c}}} {
		v := spmVocab{arch: "gemma3", pieces: []spmPiece{{"a", 1, -1}, {"b", 1, -2}, {"c", 1, -3}, {"ab", 1, k.abScore}, {"bc", 1, k.bcScore}}}
		if got := mustSPM(t, v).EncodeWithOptions("abc", trFalse); !slices.Equal(got, k.want) {
			t.Errorf("ab %v, bc %v: %v, beklenen %v", k.abScore, k.bcScore, got, k.want)
		}
	}
}

// Kural (Hugging Face'in SentencePieceExtractor'ı; gguf_spm.go): birleştirme her normal parçanın iki normal parçaya
// her bölünüşüdür (kod noktası sınırında); sıra puana göre azalan, eşit puanda dosyadaki sıra, parçanın içinde
// (sol, sağ) kimlik. Normal olmayan parça (x, tür 4) birleştirmeye girmez.
func TestSPMMergesOrder(t *testing.T) {
	tokens := []string{"a", "b", "c", "ab", "bc", "abc", "x", "y", "xy", "ç", "ö", "çö"}
	kinds := []int32{1, 1, 1, 1, 1, 1, 4, 1, 1, 1, 1, 1}
	scores := []float32{-9, -9, -9, -1, -2, -1, -1000, -9, 0, -9, -9, -3}
	want := [][2]string{{"a", "b"}, {"a", "bc"}, {"ab", "c"}, {"b", "c"}, {"ç", "ö"}}
	if got := spmMerges(tokens, scores, kinds); !slices.Equal(got, want) {
		t.Errorf("birleştirmeler %q, beklenen %q", got, want)
	}
	if got := spmMerges(nil, nil, nil); len(got) != 0 {
		t.Errorf("boş sözlük: %q", got)
	}
}

// Kural (16.1; Gemma 3'ün tokenizer.json'u): boşluk ▁ olur, önek eklenmez; sözlükte olmayan karakter UTF-8
// baytlarının parçalarına düşer; çözme ▁'yi boşluğa, bayt parçalarını karaktere çevirir.
func TestGGUFSPMNormalizesSpacesAndFallsBackToBytes(t *testing.T) {
	p := mustSPM(t, spmVocab{arch: "gemma3", pieces: normal("a", "b", "▁", "▁b")})
	a, b, sb := spmFirst, spmFirst+1, spmFirst+3
	for _, c := range []struct {
		text string
		want []int
	}{
		{"a b", []int{a, sb}}, {"b", []int{b}}, {"é", []int{4 + 0xC3, 4 + 0xA9}}, {"", nil},
	} {
		got := p.EncodeWithOptions(c.text, trFalse)
		if !slices.Equal(got, c.want) {
			t.Errorf("%q: %v, beklenen %v", c.text, got, c.want)
		}
		if back := p.Decode(got, true); back != c.text {
			t.Errorf("%q: çözme %q", c.text, back)
		}
	}
}

// Kural (D1 Gemma 3 tasarımı 1.7): eklenmiş parçalar ham metinde, normalleştirmeden önce eşlenir; GGUF'un ham
// boşluklu kullanıcı parçası "  " metindeki iki boşluğu tek kimlik yapar (TR_true). Normalleştirmeden sonra
// aransaydı hiç eşleşmezdi.
func TestGGUFSPMMatchesRawSpacePieces(t *testing.T) {
	p := mustSPM(t, spmVocab{arch: "gemma3", pieces: append(normal("a", "b", "▁"), spmPiece{"  ", ggufTokenUserDefined, -1000})})
	if got, want := p.EncodeWithOptions("a  b", Options{DontAddSpecial: true}), []int{spmFirst, spmFirst + 3, spmFirst + 1}; !slices.Equal(got, want) {
		t.Errorf("iki boşluk: %v, beklenen %v", got, want)
	}
}

// Kural (16.1 ve 24.5; DENETİM 511 N1): dosya BOS eklerken (add_bos_token) TR_special_tokens: TR_false ile kodlanan
// metinde <bos> yoktur, TR_true ile vardır. Bu test bugünkü motorun 16.2'den sapmasını sabitler: TR_false sınır
// parçasını da kapatıyor (24_sohbet_bicimi.md:93; docs/status.md:972, 16.2'nin Tur 5 satırı). TR_boundary_tokens:
// gelince test biçimle birlikte değişir.
func TestGGUFSPMTRFalseHasNoBoundaryToken(t *testing.T) {
	p := mustSPM(t, spmVocab{arch: "gemma3", pieces: normal("a"), addBOS: true})
	if got := p.EncodeWithOptions("a", trFalse); !slices.Equal(got, []int{spmFirst}) {
		t.Errorf("TR_false: %v, <bos> olmamalı", got)
	}
	if got := p.EncodeWithOptions("a", Options{}); !slices.Equal(got, []int{2, spmFirst}) {
		t.Errorf("TR_true: %v, <bos> başta olmalı", got)
	}
}

// gemma3FixtureGGUF Gemma'nın biçim parçaları fikstüründen (tests/data/gemma3_format_pieces) Gemma 3'ün GGUF'unun
// biçiminde bir sözlük kurar: eklenmiş parçalar aynı kimlikte tür 4 (boşluk parçalarının ▁'si ham boşluk), <mask>
// tür 3 (denetim); dağarcığın öbür parçaları normal ya da bayt; aradaki kimlikler yer tutucu normal parça. extra
// verilirse sona eklenmiş parça olarak girer.
func gemma3FixtureGGUF(t *testing.T, extra ...string) *gguftest.Builder {
	t.Helper()
	data, err := os.ReadFile(reporoot.Path("tests", "data", "gemma3_format_pieces", "tokenizer.json"))
	must(t, err)
	var doc struct {
		AddedTokens []AddedToken `json:"added_tokens"`
		Model       struct {
			Vocab map[string]int `json:"vocab"`
		} `json:"model"`
	}
	must(t, json.Unmarshal(data, &doc))
	size := 0
	for _, id := range doc.Model.Vocab {
		size = max(size, id+1)
	}
	tokens, kinds, scores := make([]string, size), make([]int32, size), make([]float32, size)
	for id := range tokens {
		tokens[id], kinds[id] = fmt.Sprintf("yer%d", id), ggufTokenNormal
	}
	for text, id := range doc.Model.Vocab {
		tokens[id] = text
		if strings.HasPrefix(text, "<0x") {
			kinds[id] = 6
		}
	}
	for _, a := range doc.AddedTokens {
		tokens[a.ID], kinds[a.ID], scores[a.ID] = a.Content, ggufTokenUserDefined, -1000
		if isWhitespacePiece(a.Content) {
			tokens[a.ID] = strings.ReplaceAll(a.Content, "▁", " ")
		}
		if a.Content == "<mask>" {
			kinds[a.ID] = ggufTokenControl
		}
	}
	for _, e := range extra {
		tokens, kinds, scores = append(tokens, e), append(kinds, ggufTokenUserDefined), append(scores, -1000)
	}
	b := &gguftest.Builder{}
	return b.KV(ggufKeyArch, "gemma3").KV(ggufKeyModel, ggufModelSPM).KV(ggufKeyPre, "default").KV(ggufKeyTokens, tokens).
		KV(ggufKeyTypes, kinds).KV(ggufKeyScores, scores)
}

// Kural (16.2; D1 Gemma 3 tasarımı 1.7, C2 (a)): Gemma 3'ün GGUF'u ailenin GGUF kümeleriyle (92 ham boşluklu
// boşluk parçası, <mask>'sız 58 desen parçası) aileyle açılır: TR_false'ta "\n\n" (108), "<b>" (200) ve ham "  "
// (138) tek kimliktir, <mask> denetim parçası olduğu için açılmaz. Kümesi ayrı GGUF ailesiz açılır, H7184. Aynı
// parçalar klasörde tokenizer.json'un kümeleriyle aileyle açılır (klasörün satırı değişmez).
func TestGGUFSPMGemma3FamilyUsesTheGGUFSets(t *testing.T) {
	p, err := parseGGUF(t, gemma3FixtureGGUF(t))
	must(t, err)
	if p.Family() != "gemma3" || p.Mismatch() != nil {
		t.Fatalf("aile %q, fark %+v", p.Family(), p.Mismatch())
	}
	for _, c := range []struct {
		text string
		want []int
	}{
		{"a\n\nb<b>", []int{236746, 108, 236763, 200}}, {"a  b", []int{236746, 138, 236763}},
	} {
		if got := p.EncodeWithOptions(c.text, trFalse); !slices.Equal(got, c.want) {
			t.Errorf("%q: %v, beklenen %v", c.text, got, c.want)
		}
	}
	if got := p.EncodeWithOptions("<mask>", trFalse); slices.Contains(got, 4) {
		t.Errorf("<mask> TR_false'ta açılmamalı: %v", got)
	}
	q, err := parseGGUF(t, gemma3FixtureGGUF(t, "<zz>"))
	must(t, err)
	if q.Family() != "" || q.Mismatch() == nil || *q.Mismatch() != (FamilyMismatch{"gemma3", "pattern", "count", 59, 58}) {
		t.Errorf("kümesi ayrı GGUF: aile %q, fark %+v", q.Family(), q.Mismatch())
	}
	if err := q.FamilylessError("", "", "m"); !messages.Has(err, messages.H7184) {
		t.Errorf("kümesi ayrı GGUF H7184 vermeli: %v", err)
	}
	data, err := os.ReadFile(reporoot.Path("tests", "data", "gemma3_format_pieces", "tokenizer.json"))
	must(t, err)
	folder, err := Parse(data)
	must(t, err)
	must(t, folder.ApplyFolderFamily("gemma3_text", "GemmaTokenizer"))
	if folder.Family() != "gemma3" {
		t.Errorf("fikstür klasörde aileyle açılmalı: %q, fark %+v", folder.Family(), folder.Mismatch())
	}
}

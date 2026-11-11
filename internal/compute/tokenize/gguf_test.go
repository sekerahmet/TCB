package tokenize

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// byteTokens 256 baytın bayt düzeyi karakterleridir, kimlikleri bayt değeridir.
func byteTokens() []string {
	out := make([]string, 256)
	for b := range out {
		out[b] = string(byteChar[b])
	}
	return out
}

// ggufVocab testlerin sözlüğüdür: bayt karakterleri, sonra verilen parçalar.
type ggufVocab struct {
	arch, pre string
	extra     []string // 256'dan başlayan parçalar
	kinds     map[int]int32
	merges    []string
}

// builder dosyanın üstverisini kurar; bos 1, eos 2.
func (v ggufVocab) builder() *gguftest.Builder {
	tokens := append(byteTokens(), v.extra...)
	kinds := make([]int32, len(tokens))
	for i := range kinds {
		kinds[i] = 1
	}
	for id, k := range v.kinds {
		kinds[id] = k
	}
	b := &gguftest.Builder{}
	b.KV("general.architecture", v.arch).KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, v.pre).KV(ggufKeyTokens, tokens).
		KV(ggufKeyTypes, kinds).KV(ggufKeyMerges, append([]string{}, v.merges...))
	return b.KV(ggufKeyBOS, uint32(1)).KV(ggufKeyEOS, uint32(2))
}

func parseGGUF(t *testing.T, b *gguftest.Builder) (*Tokenizer, error) {
	t.Helper()
	data := b.Bytes()
	f, err := gguf.Decode(bufio.NewReader(bytes.NewReader(data)), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return FromGGUF(f)
}

func mustGGUF(t *testing.T, v ggufVocab) *Tokenizer {
	t.Helper()
	p, err := parseGGUF(t, v.builder())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// bytesOf metnin bayt düzeyi parçasıdır: "é" → "Ã©".
func bytesOf(s string) string {
	var out []rune
	for _, c := range []byte(s) {
		out = append(out, byteChar[c])
	}
	return string(out)
}

// TestGGUFQwen2TagBringsNFC: Qwen3'ün GGUF'unda NFC yazmaz, etiket getirir (16.1). Birleşik `e` +
// U+0301 önce "é"ye normalleşir, sonra tek parça olur; aynı sözlükte smollm normalleştirmez.
func TestGGUFQwen2TagBringsNFC(t *testing.T) {
	first, second := bytesOf("é")[:2], bytesOf("é")[2:]
	v := ggufVocab{arch: "qwen3", pre: "qwen2", extra: []string{bytesOf("é")}, merges: []string{first + " " + second}}
	if got := mustGGUF(t, v).Encode("é"); !slices.Equal(got, []int{256}) {
		t.Errorf("qwen2: %v", got)
	}
	v.arch, v.pre = "llama", "smollm"
	if got := mustGGUF(t, v).Encode("é"); !slices.Equal(got, []int{'e', 0xCC, 0x81}) {
		t.Errorf("smollm: %v", got)
	}
	v.arch, v.pre = "qwen2", "qwen2"
	if got := mustGGUF(t, v).Encode("é"); !slices.Equal(got, []int{256}) {
		t.Errorf("qwen2 mimarisi: %v", got)
	}
}

// TestGGUFQwen2TagNeedsQwenArchitecture: aynı etiketi NFC'siz modeller de taşır (`Kolibri-1` biçimi).
func TestGGUFQwen2TagNeedsQwenArchitecture(t *testing.T) {
	for _, arch := range []string{"kolibri1", "llama", ""} {
		_, err := parseGGUF(t, ggufVocab{arch: arch, pre: "qwen2"}.builder())
		if !messages.Has(err, messages.H6680) {
			t.Errorf("%q: %v", arch, err)
		}
	}
}

// TestGGUFDigitsFollowTheTag: etiket ön ayırıcıyı seçer: rakam dizisi qwen2 ve smollm'de tek tek,
// llama-bpe'de üçlü gruplarla ayrılır; birleştirme yalnız grubun içinde olur.
func TestGGUFDigitsFollowTheTag(t *testing.T) {
	v := ggufVocab{extra: []string{"12"}, merges: []string{"1 2"}}
	v.arch, v.pre = "qwen3", "qwen2"
	if got := mustGGUF(t, v).Encode("12"); !slices.Equal(got, []int{'1', '2'}) {
		t.Errorf("qwen2: %v", got)
	}
	v.arch, v.pre = "llama", "smollm"
	if got := mustGGUF(t, v).Encode("12"); !slices.Equal(got, []int{'1', '2'}) {
		t.Errorf("smollm: %v", got)
	}
	v.pre = "llama-bpe"
	b := v.builder().KV(ggufKeyAddBOS, false)
	p, err := parseGGUF(t, b)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Encode("12"); !slices.Equal(got, []int{256}) {
		t.Errorf("llama-bpe: %v", got)
	}
}

// TestGGUFLlamaBPEIgnoresMerges: sözlükteki sözcük birleştirme olmadan tek parça olur.
func TestGGUFLlamaBPEIgnoresMerges(t *testing.T) {
	v := ggufVocab{arch: "llama", pre: "llama-bpe", extra: []string{"abc"}}
	p, err := parseGGUF(t, v.builder().KV(ggufKeyAddBOS, false))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Encode("abc"); !slices.Equal(got, []int{256}) {
		t.Errorf("llama-bpe: %v", got)
	}
	v.pre = "smollm"
	if got := mustGGUF(t, v).Encode("abc"); !slices.Equal(got, []int{'a', 'b', 'c'}) {
		t.Errorf("smollm: %v", got)
	}
}

// TestGGUFBosAndEos: anahtar varsa o, yoksa etiketin varsayılanı (16.1).
func TestGGUFBosAndEos(t *testing.T) {
	llama := ggufVocab{arch: "llama", pre: "llama-bpe"}
	smol := ggufVocab{arch: "llama", pre: "smollm"}
	check := func(name string, b *gguftest.Builder, want []int) {
		t.Helper()
		p, err := parseGGUF(t, b)
		if err != nil {
			t.Fatal(name, err)
		}
		if got := p.Encode("a"); !slices.Equal(got, want) {
			t.Errorf("%s: %v; %v olmalı", name, got, want)
		}
	}
	check("llama-bpe varsayılan", llama.builder(), []int{1, 'a'})
	check("llama-bpe kapalı", llama.builder().KV(ggufKeyAddBOS, false), []int{'a'})
	check("smollm varsayılan", smol.builder(), []int{'a'})
	check("smollm açık", smol.builder().KV(ggufKeyAddBOS, true), []int{1, 'a'})
	check("sona eos", smol.builder().KV(ggufKeyAddEOS, true), []int{'a', 2})
}

// TestGGUFAddedTokens: denetim parçası özeldir, kullanıcı parçası özel olmayan eklenmiş parçadır;
// ikisi de metinde tek kimlik olur, çözmede yalnız özel olan atlanır (16.2, 16.3).
func TestGGUFAddedTokens(t *testing.T) {
	v := ggufVocab{arch: "qwen3", pre: "qwen2", extra: []string{"<|im_end|>", "<think>"},
		kinds: map[int]int32{256: ggufTokenControl, 257: ggufTokenUserDefined}}
	p := mustGGUF(t, v)
	if got := p.Encode("a<think>b<|im_end|>"); !slices.Equal(got, []int{'a', 257, 'b', 256}) {
		t.Fatalf("kodlama: %v", got)
	}
	if got := p.Decode([]int{'a', 257, 256}, true); got != "a<think>" {
		t.Errorf("çözme: %q", got)
	}
}

func TestGGUFRejects(t *testing.T) {
	base := func() ggufVocab { return ggufVocab{arch: "qwen3", pre: "qwen2"} }
	bare := func() *gguftest.Builder { return (&gguftest.Builder{}).KV("general.architecture", "qwen3") }
	tokens := byteTokens()
	cases := []struct {
		name string
		b    *gguftest.Builder
		code messages.Code
	}{
		{"parçalayıcı yok", bare(), messages.H5650},
		{"model yok", bare().KV(ggufKeyPre, "qwen2"), messages.H6681},
		{"başka model", bare().KV(ggufKeyModel, "llama"), messages.H6678},
		{"etiket yok", bare().KV(ggufKeyModel, "gpt2"), messages.H6681},
		{"bilinmeyen etiket", bare().KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, "deepseek-llm"), messages.H6679},
		{"parça yok", bare().KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, "qwen2"), messages.H6681},
		{"birleştirme yok", bare().KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, "qwen2").KV(ggufKeyTokens, tokens), messages.H6681},
		{"kısa puan", base().builder().KV(ggufKeyScores, []float32{1}), messages.H6682},
		{"puan türü", base().builder().KV(ggufKeyScores, []int32{1}), messages.H6681},
		{"kısa tür", bare().KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, "qwen2").KV(ggufKeyTokens, tokens).
			KV(ggufKeyMerges, []string{}).KV(ggufKeyTypes, []int32{1}), messages.H6682},
		{"tür dizisi yok", bare().KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, "qwen2").KV(ggufKeyTokens, tokens).
			KV(ggufKeyMerges, []string{}), messages.H6681},
		{"sözlük dışı özel kimlik", base().builder().KV("tokenizer.ggml.padding_token_id", uint32(256)), messages.H6683},
		{"özel kimlik türü", base().builder().KV("tokenizer.ggml.unknown_token_id", "0"), messages.H6681},
		{"yinelenen parça", ggufVocab{arch: "qwen3", pre: "qwen2", extra: []string{"a"}}.builder(), messages.H6684},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parseGGUF(t, c.b); !messages.Has(err, c.code) {
				t.Errorf("hata %v; %s olmalı", err, c.code)
			}
		})
	}
}

// TestGGUFQwen2SpecialNames: dönüştürücü FIM parçalarını denetim türüyle yazar, modelin
// tokenizer.json'unda özel değildirler: çözme onları atlamaz, gerçek özel parçayı atlar. Başka etikette
// tür geçerlidir.
func TestGGUFQwen2SpecialNames(t *testing.T) {
	extra := []string{"<|fim_prefix|>", "<|file_sep|>", "<|im_end|>"}
	kinds := map[int]int32{256: ggufTokenControl, 257: ggufTokenControl, 258: ggufTokenControl}
	for _, c := range []struct {
		arch, pre, want string
	}{
		{"qwen3", "qwen2", "<|fim_prefix|><|file_sep|>"},
		{"llama", "smollm", ""},
	} {
		tok, err := parseGGUF(t, ggufVocab{arch: c.arch, pre: c.pre, extra: extra, kinds: kinds}.builder())
		if err != nil {
			t.Fatal(err)
		}
		if got := tok.Decode([]int{256, 257, 258}, true); got != c.want {
			t.Errorf("%s: %q; beklenen %q", c.pre, got, c.want)
		}
		ids := tok.Encode("<|fim_prefix|>")
		if !slices.Equal(ids, []int{256}) {
			t.Errorf("%s: kimlikler %v", c.pre, ids)
		}
	}
}

// TestGGUFLongValuesAreClipped: dosyanın dev etiketi iletiye girmez.
func TestGGUFLongValuesAreClipped(t *testing.T) {
	long := strings.Repeat("x", 1<<20)
	_, err := parseGGUF(t, (&gguftest.Builder{}).KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, long))
	if !messages.Has(err, messages.H6679) || len(err.Error()) > 512 {
		t.Errorf("%d baytlık ileti", len(err.Error()))
	}
}

// TestGGUFBosMissingWithAddBos: istenen özel parçanın kimliği yoksa sessizce eklenmemez.
func TestGGUFBosMissingWithAddBos(t *testing.T) {
	b := (&gguftest.Builder{}).KV("general.architecture", "llama").KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, "smollm").
		KV(ggufKeyTokens, byteTokens()).KV(ggufKeyMerges, []string{}).KV(ggufKeyAddBOS, true)
	if _, err := parseGGUF(t, b); !messages.Has(err, messages.H6681) {
		t.Errorf("hata %v", err)
	}
}

// TestLoadRecognisesGGUFFromContent: biçim uzantıdan değil içerikten anlaşılır; veri bölümü
// okunmaz, tensörlü dosya da parçalayıcı verir.
func TestLoadRecognisesGGUFFromContent(t *testing.T) {
	b := ggufVocab{arch: "qwen3", pre: "qwen2"}.builder().Tensor("w", gguf.TypeQ5_0, []uint64{32}, make([]byte, 22))
	path := filepath.Join(t.TempDir(), "model.json")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.VocabSize() != 256 {
		t.Errorf("dağarcık %d", p.VocabSize())
	}
}

// TestGGUFPretokenizerTag: GGUF'tan yüklenen parçalayıcı dosyanın etiketini taşır; GGUF'ta her
// zaman vardır, `pre`'siz dosya yüklenmez (TestGGUFRejects, "etiket yok": H6681). tokenizer.json'da
// yokluğu TR testinde (tests/veri_parcalayici_test.tr).
func TestGGUFPretokenizerTag(t *testing.T) {
	for _, v := range []ggufVocab{{arch: "qwen3", pre: "qwen2"}, {arch: "llama", pre: "llama-bpe"}, {arch: "llama", pre: "smollm"}} {
		if tag, ok := mustGGUF(t, v).PretokenizerTag(); !ok || tag != v.pre {
			t.Errorf("%s: etiket %q, %v", v.pre, tag, ok)
		}
	}
}

// TestGGUFDeclaredEosTokens: dosyanın bildirdiği durma kimlikleri eos, eot, eom sırasıyla ve
// tekrarsızdır; hiçbiri yoksa yoktur, boş liste dönmez. eot ve eom dağarcıkla denetlenmez:
// dağarcık dışı kimlik hata değildir (15.55, durma kimlikleri).
func TestGGUFDeclaredEosTokens(t *testing.T) {
	base := ggufVocab{arch: "llama", pre: "llama-bpe"}
	eot, eom := "tokenizer.ggml.eot_token_id", "tokenizer.ggml.eom_token_id"
	cases := []struct {
		name string
		b    *gguftest.Builder
		want []int32
	}{
		{"yalnız eos", base.builder(), []int32{2}},
		{"üçü sırayla", base.builder().KV(eom, uint32(4)).KV(eot, uint32(3)), []int32{2, 3, 4}},
		{"tekrar atılır", base.builder().KV(eot, uint32(2)).KV(eom, uint32(5)), []int32{2, 5}},
		{"dağarcık dışı eom", base.builder().KV(eom, uint32(999999)), []int32{2, 999999}},
		{"hiçbiri", (&gguftest.Builder{}).KV("general.architecture", "llama").KV(ggufKeyModel, "gpt2").KV(ggufKeyPre, "smollm").
			KV(ggufKeyTokens, byteTokens()).KV(ggufKeyTypes, make([]int32, 256)).KV(ggufKeyMerges, []string{}), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := parseGGUF(t, c.b)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := p.DeclaredEOSTokens()
			if ok != (c.want != nil) || !slices.Equal(got, c.want) {
				t.Errorf("%v, %v; %v olmalı", got, ok, c.want)
			}
		})
	}
	if _, err := parseGGUF(t, base.builder().KV(eot, "3")); !messages.Has(err, messages.H6681) {
		t.Errorf("metin kimlik: %v; H6681 olmalı", err)
	}
}

// FuzzGGUFTokenizer: üstveri güvenilmeyen girdidir: çözülen her dosyadan parçalayıcı kurulurken
// çökme olmaz; kurulan parçalayıcı metin kodlar.
func FuzzGGUFTokenizer(f *testing.F) {
	f.Add(ggufVocab{arch: "qwen3", pre: "qwen2", extra: []string{"ab"}, merges: []string{"a b"}}.builder().Bytes())
	f.Add(ggufVocab{arch: "llama", pre: "llama-bpe"}.builder().Bytes())
	f.Add(ggufVocab{arch: "llama", pre: "smollm"}.builder().KV(ggufKeyAddEOS, true).Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := gguf.Decode(bufio.NewReader(bytes.NewReader(data)), int64(len(data)))
		if err != nil {
			return
		}
		p, err := FromGGUF(file)
		if err != nil {
			return
		}
		if _, err := p.SafeEncode("a1 é", Options{}); err != nil {
			return
		}
	})
}

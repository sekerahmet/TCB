package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.55; Dost'un katalog tasarımı K-5): Dost'un aile adımı (tools/assistant/aile.tr, gguf_ailesi) aileyi
// kendisi seçmez, kitaplığın seçimini okur: model.TR_eos_tokens() ailesiz modelde H7170'tir, yükleme düşer. Biçimin
// adı dosyanın mimarisidir; Dost'un ailelerinden değilse H8170 (kimlik dosyanın adı).

// assistantFamilyStep tools/assistant/aile.tr'yi mutlak yoluyla aktaran programda gguf'u yükler, gövdeyi koşar.
func assistantFamilyStep(t *testing.T, gguf, body string) string {
	t.Helper()
	dir := t.TempDir()
	family := filepath.ToSlash(reporoot.Path("tools", "assistant", "aile.tr"))
	writeTR(t, dir, "adim.tr", fmt.Sprintf("TR_import %q\nTR_import \"tr/pretrained\"\nTR_var yol = %q\nTR_var m = TR_pretrained_gguf(yol)\n%s\n", family, gguf, body))
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "adim.tr"))
	return strings.TrimSpace(s.Stdout + s.Stderr)
}

// gemmaWithTag küçük Gemma 3'ün GGUF'una parçalayıcı anahtarlarını ekler: 40 tek karakterli parça, sonra Gemma'nın
// dört özel adı; etiket `pre`'dir.
func gemmaWithTag(t *testing.T, pre string) string {
	t.Helper()
	g := gemmaTiny{dim: 64, headDim: 256}
	b := g.ggufFile(g.weights(1))
	tokens := append(strings.Split("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn", ""), "<bos>", "<eos>", "<start_of_turn>", "<end_of_turn>")
	kinds := make([]int32, len(tokens))
	for i := range kinds {
		kinds[i] = map[bool]int32{false: 1, true: 3}[strings.HasPrefix(tokens[i], "<")]
	}
	addTokenizer(b, pre, tokens, kinds)
	return writeGGUF(t, t.TempDir(), b)
}

func addTokenizer(b *gguftest.Builder, pre string, tokens []string, kinds []int32) {
	b.KV("tokenizer.ggml.model", "gpt2").KV("tokenizer.ggml.pre", pre).KV("tokenizer.ggml.tokens", tokens).
		KV("tokenizer.ggml.token_type", kinds).KV("tokenizer.ggml.merges", []string{}).KV("tokenizer.ggml.bos_token_id", uint32(0))
}

// TestAssistantFamilyStepUsesTheLibrary: mimarisi gemma3, etiketi default olmayan GGUF kitaplıkta ailesizdir, Dost'un
// aile adımı H7170 verir (iki etiket); kitaplığın llama3 saydığı GGUF Dost'un ailesi değildir, H8170 dosyanın adıyla.
// Bilinen olumlu: Qwen3'ün GGUF'u (qwen2 etiketi) qwen3 adını ve kitaplığın durma kimliklerini verir.
func TestAssistantFamilyStepUsesTheLibrary(t *testing.T) {
	code := "TR_print(TR_error_code { gguf_ailesi(yol, m) })"
	for _, pre := range []string{"qwen2", "llama-bpe"} {
		if got := assistantFamilyStep(t, gemmaWithTag(t, pre), code); got != "H7170" {
			t.Errorf("gemma3, etiket %s: %q; H7170 olmalı", pre, got)
		}
	}
	qwen := familyGGUF(t, t.TempDir(), qwenTiny, "qwen2", []string{"<|endoftext|>", "<|im_end|>"}, map[string]uint32{"eos": 41})
	if got := assistantFamilyStep(t, qwen, "TR_var aile, durma = gguf_ailesi(yol, m)\nTR_print(aile, durma)"); got != "qwen3 [41, 40]" {
		t.Errorf("qwen3: %q; \"qwen3 [41, 40]\" olmalı", got)
	}
	llama := familyGGUF(t, t.TempDir(), llamaTiny, "llama-bpe", llama3, nil)
	if got := assistantFamilyStep(t, llama, "TR_print(m.TR_eos_tokens())\n"+code); got != "[42, 43, 41]\nH8170" {
		t.Errorf("llama3: %q; kitaplıkta aileli, Dost'ta H8170 olmalı", got)
	}
	got := assistantFamilyStep(t, llama, "gguf_ailesi(yol, m)")
	for _, part := range []string{"`m.gguf`", "`llama`", "`gemma3`, `qwen3`"} {
		if !strings.Contains(got, part) {
			t.Errorf("H8170'in metninde %s yok: %q", part, got)
		}
	}
}

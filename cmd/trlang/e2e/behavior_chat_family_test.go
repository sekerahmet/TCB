package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.55, sohbet tanımının seçimi ve durma kimlikleri): hazır modelin durma listesi sohbet
// tanımındaki adlardan parçalayıcının kimliğiyle kurulur; m.TR_eos_tokens() onu verir. GGUF'ta aile
// (mimari, p.TR_pretokenizer_tag), klasörde (model_type, tokenizer_config.json'un işareti) ile
// seçilir. Adı dağarcıkta olmayan durma parçası H7162, dosyanın bildirdiği ama listede olmayan
// kimlik H7163. Ailesiz modelde (parçalayıcısız GGUF, tabloda olmayan etiket, tokenizer_config'siz
// klasör) çağrı H7170'tir, dosyanın kimliği denetlenmez, TR_eos: [...] ile üretim sürer.

// familyGGUF küçük modelin GGUF'una parçalayıcı anahtarlarını ekler: tek karakterli parçalar,
// sonra özel adlar (kimlikleri 40'tan başlar). declared dosyanın bildirdiği durma kimlikleridir.
func familyGGUF(t *testing.T, dir string, c tinyConfig, pre string, special []string, declared map[string]uint32) string {
	t.Helper()
	b := c.ggufFile(c.randomWeights(1))
	tokens := make([]string, 0, 40+len(special))
	for i := range 40 {
		tokens = append(tokens, string(rune('A'+i)))
	}
	tokens = append(tokens, special...)
	kinds := make([]int32, len(tokens))
	for i := range kinds {
		kinds[i] = 1
		if i >= 40 {
			kinds[i] = 3
		}
	}
	b.KV("tokenizer.ggml.model", "gpt2").KV("tokenizer.ggml.pre", pre).KV("tokenizer.ggml.tokens", tokens).
		KV("tokenizer.ggml.token_type", kinds).KV("tokenizer.ggml.merges", []string{}).KV("tokenizer.ggml.bos_token_id", uint32(0))
	for _, key := range []string{"eos", "eot", "eom"} {
		if id, ok := declared[key]; ok {
			b.KV("tokenizer.ggml."+key+"_token_id", id)
		}
	}
	file := filepath.Join(dir, "m.gguf")
	writeBytes(t, file, b.Bytes())
	return filepath.ToSlash(file)
}

// runFamily m'yi yükler ve gövdeyi koşar; çıktıyı verir.
func runFamily(t *testing.T, dir, load, body string) string {
	t.Helper()
	writeTR(t, dir, "aile.tr", "TR_import \"tr/pretrained\"\nTR_var m = "+load+"\n"+body+"\n")
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "aile.tr"))
	return strings.TrimSpace(s.Stdout + s.Stderr)
}

var (
	llamaTiny = tinyConfig{arch: "llama", vocab: 48, dim: 64, ff: 96, layers: 1, heads: 4, kv: 2, headDim: 16, context: 32, theta: 10000}
	qwenTiny  = tinyConfig{arch: "qwen3", vocab: 48, dim: 64, ff: 96, layers: 1, heads: 4, kv: 2, headDim: 16, context: 32, theta: 10000}
	llama3    = []string{"<|begin_of_text|>", "<|end_of_text|>", "<|eot_id|>", "<|eom_id|>"}
)

func TestGGUFChatFamilyStopList(t *testing.T) {
	codeOf := "TR_print(TR_error_code { m.TR_eos_tokens() })"
	cases := []struct {
		name     string
		c        tinyConfig
		pre      string
		special  []string
		declared map[string]uint32
		body     string
		want     string
	}{
		// Llama 3'ün üç adı, sohbet tanımının sırasıyla: <|eot_id|> 42, <|eom_id|> 43, <|end_of_text|> 41.
		{"llama3", llamaTiny, "llama-bpe", llama3, map[string]uint32{"eos": 42, "eot": 42, "eom": 43}, "TR_print(m.TR_eos_tokens())", "[42, 43, 41]"},
		{"qwen3", qwenTiny, "qwen2", []string{"<|endoftext|>", "<|im_end|>"}, map[string]uint32{"eos": 41}, "TR_print(m.TR_eos_tokens())", "[41, 40]"},
		{"dosya bildirmiyor", llamaTiny, "llama-bpe", llama3, nil, "TR_print(m.TR_eos_tokens())", "[42, 43, 41]"},
		// Dağarcık dışı eom hata değildir (16.1) ama listede değildir: H7163.
		{"bildirilen listede yok", llamaTiny, "llama-bpe", llama3, map[string]uint32{"eos": 42, "eom": 7}, "", "H7163"},
		{"ad dağarcıkta yok", llamaTiny, "llama-bpe", llama3[:3], nil, "", "H7162"},
		{"tabloda olmayan etiket", llamaTiny, "smollm", llama3, map[string]uint32{"eos": 7}, codeOf, "H7170"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			file := familyGGUF(t, dir, c.c, c.pre, c.special, c.declared)
			load := fmt.Sprintf("TR_pretrained_gguf(%q)", file)
			if c.body == "" {
				load = "TR_none"
				c.body = fmt.Sprintf("TR_print(TR_error_code { TR_pretrained_gguf(%q) })", file)
			}
			if got := runFamily(t, dir, load, c.body); got != c.want {
				t.Errorf("çıktı %q; %q olmalı", got, c.want)
			}
		})
	}
}

// TestFamilylessModelGeneratesWithExplicitStops: üç ailesiz yolda (parçalayıcısız GGUF, tabloda
// olmayan etiket, tokenizer_config'siz klasör) m.TR_eos_tokens() H7170'tir, ileti kimliklerin
// açık yazımını söyler; TR_eos: [...] ile üretim sürer.
func TestFamilylessModelGeneratesWithExplicitStops(t *testing.T) {
	body := "TR_print(TR_error_code { m.TR_eos_tokens() })\nTR_print(m.TR_generate([1, 2], 3, TR_temperature: 0, TR_eos: [5]).TR_length > 2)"
	dir := t.TempDir()
	bare := filepath.Join(dir, "yalın.gguf")
	writeBytes(t, bare, llamaTiny.ggufFile(llamaTiny.randomWeights(1)).Bytes())
	folder := filepath.Join(dir, "klasör")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	llamaTiny.writeHF(t, folder, llamaTiny.randomWeights(1))
	loads := map[string]string{
		"parçalayıcısız GGUF": fmt.Sprintf("TR_pretrained_gguf(%q)", filepath.ToSlash(bare)),
		"smollm etiketi":      fmt.Sprintf("TR_pretrained_gguf(%q)", familyGGUF(t, t.TempDir(), llamaTiny, "smollm", llama3, nil)),
		"klasör":              fmt.Sprintf("TR_pretrained_model(%q)", filepath.ToSlash(folder)),
	}
	for name, load := range loads {
		t.Run(name, func(t *testing.T) {
			if got := runFamily(t, dir, load, body); got != "H7170\ntrue" {
				t.Errorf("çıktı %q", got)
			}
		})
	}
	got := runFamily(t, dir, loads["klasör"], "m.TR_eos_tokens()")
	if !strings.Contains(got, "`TR_eos: [...]`") || !strings.Contains(got, "llama3, qwen3") {
		t.Errorf("H7170'in iletisi aileleri ve açık yazımı söylemiyor:\n%s", got)
	}
}

// TestFolderChatFamily: klasörde aile (model_type, tokenizer_class'ın Fast'sız adı) ile seçilir;
// kimlikler klasörün tokenizer.json'undan, bildirilen kimlik generation_config.json'dan.
func TestFolderChatFamily(t *testing.T) {
	tok, err := os.ReadFile(reporoot.Path("tests", "data", "qwen3_chat", "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, eos, want string }{
		{"liste", "[573, 571]", "[573, 571]"},
		{"tek sayı", "573", "[573, 571]"},
		{"listede yok", "[573, 7]", "H7163"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			qwenTiny.writeHF(t, dir, qwenTiny.randomWeights(1))
			writeBytes(t, filepath.Join(dir, "tokenizer.json"), tok)
			writeBytes(t, filepath.Join(dir, "tokenizer_config.json"), []byte(`{"tokenizer_class": "Qwen2TokenizerFast"}`))
			writeBytes(t, filepath.Join(dir, "generation_config.json"), []byte(`{"eos_token_id": `+c.eos+`}`))
			load := fmt.Sprintf("TR_pretrained_model(%q)", filepath.ToSlash(dir))
			body := "TR_print(m.TR_eos_tokens())"
			if c.want == "H7163" {
				load, body = "TR_none", fmt.Sprintf("TR_print(TR_error_code { TR_pretrained_model(%q) })", filepath.ToSlash(dir))
			}
			if got := runFamily(t, dir, load, body); got != c.want {
				t.Errorf("çıktı %q; %q olmalı", got, c.want)
			}
		})
	}
}

// TestGenerateEosMethodWithoutParentheses: TR_eos:'a parantezsiz metot (fonksiyon değeri) verilince
// ileti parantezli yazımı önerir (H7173); önerilen yazım ailesi olan modelde çalışır.
func TestGenerateEosMethodWithoutParentheses(t *testing.T) {
	dir := t.TempDir()
	file := familyGGUF(t, dir, llamaTiny, "llama-bpe", llama3, nil)
	load := fmt.Sprintf("TR_pretrained_gguf(%q)", file)
	got := runFamily(t, dir, load, "TR_var model = m\nm.TR_generate([1], 2, TR_temperature: 0, TR_eos: m.TR_eos_tokens)")
	if !strings.Contains(got, "`TR_eos: model.TR_eos_tokens()`") {
		t.Fatalf("ileti parantezli yazımı önermiyor:\n%s", got)
	}
	if got := runFamily(t, dir, load, "TR_var model = m\nTR_print(model.TR_generate([1], 2, TR_temperature: 0, TR_eos: model.TR_eos_tokens()).TR_length > 1)"); got != "true" {
		t.Errorf("önerilen yazım: %q", got)
	}
}

package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/reporoot"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.55, 24.7): GGUF'un tokenizer.chat_template'i çalıştırılmaz, yalnız sha256 karması ailenin bilinen
// şablon karmalarıyla karşılaştırılır. Karma listede yoksa model ailesiz yüklenir (H7170 template); ileti şablonu
// okumayı söyler, ayrıntı karmanın ilk 12'si ve kopyalanınca çalışan yazımlardır (okuma, TR_eos:, TR_gemma3_format). Liste boş
// aile (Qwen3, Llama 3), şablonsuz dosya ve metin olmayan şablon denetlenmez. Denetim _aileyi_kur'dan sonra koşar:
// kimlik uyuşmazlığı önce raporlanır (DENETİM 875 N1). Klasörde şablon denetlenmez (status 15.55).

// gemma3KnownTemplates Gemma 3'ün bilinen iki şablonunun karmasıdır: Google'ın şablonu (google/gemma-3-4b-it) ve
// Google'ın QAT GGUF dosyalarınınki (24.7). Şablonların metni depoya girmez.
var gemma3KnownTemplates = []string{
	"7de1c58e208eda46e9c7f86397df37ec49883aeece39fb961e0a6b24088dd3c4",
	"6d2d8fdb903d7014b50a7c1430dff8e61b21d6a861d81e196197ccd6ac855d59",
}

// gemmaChatGGUF küçük Gemma 3'ün GGUF'una SentencePiece parçalayıcısını ekler: 0 <pad>, 1 <eos>, 2 <bos>,
// 3 <unk>, 4 <start_of_turn>, 5 <end_of_turn>, sonra 42 sıradan parça (48, küçük modelin dağarcığı). over
// üstverinin anahtarlarını değiştirir (gemmaTiny.metadata gibi); declared dosyanın bildirdiği durma kimlikleridir.
func gemmaChatGGUF(t *testing.T, dir string, over map[string]any, declared map[string]uint32) string {
	t.Helper()
	g := gemmaTiny{dim: 64, headDim: 256}
	b := g.ggufFileWith(g.weights(1), over)
	tokens := []string{"<pad>", "<eos>", "<bos>", "<unk>", "<start_of_turn>", "<end_of_turn>"}
	for _, s := range strings.Split("abcdefghijklmnopqrstuvwxyz0123456789ABC▁", "") {
		tokens = append(tokens, s)
	}
	tokens = append(tokens, "\n", "\n\n")
	kinds := make([]int32, len(tokens))
	scores := make([]float32, len(tokens))
	for i := range tokens {
		kinds[i], scores[i] = 1, -float32(i)
		if i < 6 {
			kinds[i], scores[i] = 3, -1000
		}
	}
	if len(tokens) != g.tiny().vocab {
		t.Fatalf("parça sayısı %d, dağarcık %d", len(tokens), g.tiny().vocab)
	}
	b.KV("tokenizer.ggml.model", "llama").KV("tokenizer.ggml.pre", "default").KV("tokenizer.ggml.tokens", tokens).
		KV("tokenizer.ggml.token_type", kinds).KV("tokenizer.ggml.scores", scores).
		KV("tokenizer.ggml.bos_token_id", uint32(2)).KV("tokenizer.ggml.eos_token_id", uint32(1))
	for key, id := range declared {
		b.KV("tokenizer.ggml."+key+"_token_id", id)
	}
	return writeGGUF(t, dir, b)
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:12]
}

func TestGGUFChatTemplateGate(t *testing.T) {
	const template = "{{ bos_token }}{% for m in messages %}{{ m['content'] }}{% endfor %}"
	h := shortHash(template)
	gemmaParts := []string{`"gemma3"`, "sha256 " + h, "`<start_of_turn>user`", "`TR_load_metadata(yol)[\"tokenizer.chat_template\"]`",
		"`TR_eos: [5, 1]`", "`TR_gemma3_format(p, iletiler, TR_add_generation_prompt: TR_true)`"}
	cases := []struct {
		name, reason, want string
		path               func(dir string) string
		parts              []string
	}{
		{"bilinmeyen şablon", "template", "", func(dir string) string {
			return gemmaChatGGUF(t, dir, map[string]any{"tokenizer.chat_template": template}, nil)
		}, gemmaParts},
		{"şablonsuz", "", "[5, 1]", func(dir string) string { return gemmaChatGGUF(t, dir, nil, nil) }, nil},
		// Liste boş aile denetlenmez: Qwen3'ün şablonu bilinmese de aile kalır.
		{"qwen3 denetlenmez", "", "[41, 40]", func(dir string) string {
			b := familyBuilder(qwenTiny, "gpt2", "qwen2", []string{"<|endoftext|>", "<|im_end|>"}, map[string]uint32{"eos": 41})
			return writeGGUF(t, dir, b.KV("tokenizer.chat_template", template))
		}, nil},
		// İki uyuşmazlıkta önce kimlik (DENETİM 875 N1): eot 7 ailenin listesinde yok.
		{"kimlik önce", "id", "", func(dir string) string {
			return gemmaChatGGUF(t, dir, map[string]any{"tokenizer.chat_template": template}, map[string]uint32{"eot": 7})
		}, []string{`"gemma3"`, `7 "b" ∉ [5, 1]`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			load := fmt.Sprintf("TR_pretrained_gguf(%q)", c.path(dir))
			if c.reason == "" {
				if got := runFamily(t, dir, load, "TR_print(m.TR_eos_tokens())"); got != c.want {
					t.Errorf("çıktı %q; %q olmalı", got, c.want)
				}
				return
			}
			checkFamilyless(t, runFamily(t, dir, load, familylessBody), c.reason, c.parts...)
		})
	}
	// İletinin yolu kopyalanınca çalışır: ayrıntının yazımlarıyla (TR_eos: [5, 1], TR_gemma3_format) üretir.
	dir := t.TempDir()
	yol := gemmaChatGGUF(t, dir, map[string]any{"tokenizer.chat_template": template}, nil)
	writeTR(t, dir, "yol.tr", fmt.Sprintf("TR_import \"tr/pretrained\"\nTR_import \"tr/chat\"\nTR_import \"tr/chat_gemma3\"\n"+
		"TR_var m = TR_pretrained_gguf(%q)\nTR_var p = TR_load_tokenizer(%q)\n"+
		"TR_var k = TR_gemma3_format(p, [TR_ChatMessage(TR_role: TR_Role.TR_User, TR_content: \"ab\")], TR_add_generation_prompt: TR_true)\n"+
		"TR_var n = m.TR_generate(k, 2, TR_temperature: 0, TR_eos: [5, 1]).TR_length - k.TR_length\nTR_print(n >= 1 TR_and n <= 2)\n", yol, yol))
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "yol.tr"))
	if got := strings.TrimSpace(s.Stdout + s.Stderr); got != "true" {
		t.Errorf("iletinin yolu çalışmadı: %q", got)
	}
}

// TestChatTableKnownTemplates: sohbet tablosunun her satırı bilinen şablon karmalarının listesini taşır; Gemma 3'ün
// iki satırı iki karmayı, öbür satırlar boş listeyi (24.7, 15.55). Tablo kitaplığın özel fonksiyonudur, TR
// testinden görünmez: test kaynağın satırlarını okur (TestChatTableIsUnique gibi).
func TestChatTableKnownTemplates(t *testing.T) {
	src, err := os.ReadFile(reporoot.Path("lib", "model_family.tr"))
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile(`_Sohbet\("([^"]*)", "[^"]*", "[^"]*", "[^"]*", \[[^\]]*\], \[([^\]]*)\]\)`).FindAllStringSubmatch(string(src), -1)
	all := regexp.MustCompile(`_Sohbet\(`).FindAllString(string(src), -1)
	if len(rows) < 4 || len(rows) != len(all) {
		t.Fatalf("tablonun satırları okunamadı: %d satır, %d _Sohbet(", len(rows), len(all))
	}
	want := `"` + strings.Join(gemma3KnownTemplates, `", "`) + `"`
	for _, r := range rows {
		if exp := map[bool]string{true: want, false: ""}[r[1] == "gemma3"]; r[2] != exp {
			t.Errorf("%s satırının şablonları %q; %q olmalı", r[1], r[2], exp)
		}
	}
}

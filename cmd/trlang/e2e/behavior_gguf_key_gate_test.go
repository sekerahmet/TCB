package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.55, bilinmeyen anahtar; DENETİM 429 Ş2): GGUF'ta general., tokenizer., split. ve
// quantize. dışındaki her anahtar yapı tanımında olmalıdır; "<mimari>." ile başlayıp tanımda
// olmayan, başka bir mimarinin öneki ve yeni bir önek H7160'tır. İleti bilinmeyenlerin hepsini ve
// dosyayı söyler. Kapı mimariden sonra, değer kapılarından (H6943) önce gelir. Serbest önekler ve
// ölçülen gerçek dosyaların anahtarları geçer.
func TestGGUFUnknownKeyGate(t *testing.T) {
	c := tinyConfig{arch: "qwen3", vocab: 48, dim: 64, ff: 96, layers: 2, heads: 4, kv: 2, headDim: 16, context: 32, theta: 1000000}
	w := c.randomWeights(5)
	full := func(edit func(b *gguftest.Builder)) []byte {
		b := c.ggufFile(w)
		edit(b)
		return b.Bytes()
	}
	kv := func(pairs ...any) func(b *gguftest.Builder) {
		return func(b *gguftest.Builder) {
			for i := 0; i < len(pairs); i += 2 {
				b.KV(pairs[i].(string), pairs[i+1])
			}
		}
	}
	cases := []struct {
		name, want string
		says       []string
		data       []byte
	}{
		{"tanımda olmayan <mimari>. anahtarı", "H7160", []string{"qwen3.attention.temperature_scale", "m0.gguf"},
			full(kv("qwen3.attention.temperature_scale", float32(2)))},
		{"başka mimarinin öneki", "H7160", []string{"llama.rope.freq_base"}, full(kv("llama.rope.freq_base", float32(10000)))},
		{"yeni önek", "H7160", []string{"adapter.lora.alpha"}, full(kv("adapter.lora.alpha", float32(16)))},
		{"hepsi sayılır", "H7160", []string{"qwen3.expert_count", "clip.has_vision_encoder", "2 anahtar"},
			full(kv("qwen3.expert_count", uint32(8), "clip.has_vision_encoder", true))},
		{"değer kapısından önce", "H7160", []string{"qwen3.yeni"}, full(kv("qwen3.rope.scaling.type", "linear", "qwen3.yeni", uint32(1)))},
	}
	dir := t.TempDir()
	for i, c := range cases {
		writeBytes(t, filepath.Join(dir, fmt.Sprintf("m%d.gguf", i)), c.data)
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			writeTR(t, dir, "kapi.tr", fmt.Sprintf("TR_import \"tr/pretrained\"\nTR_pretrained_gguf(\"m%d.gguf\")\n", i))
			s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "kapi.tr"))
			writeTR(t, dir, "kod.tr", fmt.Sprintf("TR_import \"tr/pretrained\"\nTR_print(TR_error_code { TR_pretrained_gguf(\"m%d.gguf\") })\n", i))
			k := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "kod.tr"))
			if got := strings.TrimSpace(k.Stdout); got != c.want {
				t.Errorf("kod %q; beklenen %s\n%s", got, c.want, k.Stderr)
			}
			for _, w := range c.says {
				if !strings.Contains(s.Stderr+s.Stdout, w) {
					t.Errorf("ileti %q demiyor:\n%s%s", w, s.Stdout, s.Stderr)
				}
			}
		})
	}
	// Serbest önekler kapıya girmez; ölçülen dosyalardaki bilinen anahtarlar (13 başlık, d1-aileler-1)
	// ve etkisiz ölçek değerleri geçer.
	writeBytes(t, filepath.Join(dir, "ok.gguf"), full(kv("general.name", "Qwen3 küçük", "general.file_type", uint32(7),
		"quantize.imatrix.file", "x.dat", "tokenizer.chat_template", "{{ x }}", "qwen3.attention.key_length", uint32(16),
		"qwen3.attention.value_length", uint32(16), "qwen3.vocab_size", uint32(48), "qwen3.rope.dimension_count", uint32(16),
		"qwen3.rope.scaling.type", "none", "qwen3.rope.scaling.factor", float32(1), "qwen3.rope.scale_linear", float32(0))))
	writeTR(t, dir, "ok.tr", "TR_import \"tr/pretrained\"\nTR_print(TR_pretrained_gguf(\"ok.gguf\").TR_config[\"katman\"])\n")
	if s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "ok.tr")); strings.TrimSpace(s.Stdout) != "2" {
		t.Errorf("serbest önekler ve bilinen anahtarlar:\n%s%s", s.Stdout, s.Stderr)
	}
}

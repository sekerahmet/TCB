package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// ggufGateCase bir GGUF kapısının durumudur: dosyanın baytları, beklenen hata kodu ve iletinin
// içermesi gereken parçalar.
type ggufGateCase struct {
	name, want string
	says       []string
	data       []byte
}

// runGGUFGateCases her durumun dosyasını m<i>.gguf adıyla yazar ve TR_pretrained_gguf ile açar:
// hata kodu want olmalı, gösterilen ileti says'in her parçasını içermeli.
func runGGUFGateCases(t *testing.T, cases []ggufGateCase) {
	t.Helper()
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
}

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
	cases := []ggufGateCase{
		{"tanımda olmayan <mimari>. anahtarı", "H7160", []string{"qwen3.attention.temperature_scale", "m0.gguf"},
			full(kv("qwen3.attention.temperature_scale", float32(2)))},
		{"başka mimarinin öneki", "H7160", []string{"llama.rope.freq_base"}, full(kv("llama.rope.freq_base", float32(10000)))},
		{"yeni önek", "H7160", []string{"adapter.lora.alpha"}, full(kv("adapter.lora.alpha", float32(16)))},
		{"hepsi sayılır", "H7160", []string{"qwen3.expert_count", "clip.has_vision_encoder", "2 anahtar"},
			full(kv("qwen3.expert_count", uint32(8), "clip.has_vision_encoder", true))},
		{"değer kapısından önce", "H7160", []string{"qwen3.yeni"}, full(kv("qwen3.rope.scaling.type", "linear", "qwen3.yeni", uint32(1)))},
	}
	runGGUFGateCases(t, cases)
	// Serbest önekler kapıya girmez; ölçülen dosyalardaki bilinen anahtarlar (13 başlık, d1-aileler-1)
	// ve etkisiz ölçek değerleri geçer.
	dir := t.TempDir()
	writeBytes(t, filepath.Join(dir, "ok.gguf"), full(kv("general.name", "Qwen3 küçük", "general.file_type", uint32(7),
		"quantize.imatrix.file", "x.dat", "tokenizer.chat_template", "{{ x }}", "qwen3.attention.key_length", uint32(16),
		"qwen3.attention.value_length", uint32(16), "qwen3.vocab_size", uint32(48), "qwen3.rope.dimension_count", uint32(16),
		"qwen3.rope.scaling.type", "none", "qwen3.rope.scaling.factor", float32(1), "qwen3.rope.scale_linear", float32(0))))
	writeTR(t, dir, "ok.tr", "TR_import \"tr/pretrained\"\nTR_print(TR_pretrained_gguf(\"ok.gguf\").TR_config[\"katman\"])\n")
	if s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "ok.tr")); strings.TrimSpace(s.Stdout) != "2" {
		t.Errorf("serbest önekler ve bilinen anahtarlar:\n%s%s", s.Stdout, s.Stderr)
	}
}

// Kural (15.55, H7175; Teslim B, DENETİM 473 N2): kitaplığın modeli kurarken okuduğu her GGUF üstveri
// değeri beklenen türde değilse yükleme hatasıdır; ileti hazır_gguf'u, anahtarı, iki türü ve dosyanın
// yolunu söyler, değerin üstveride olduğunu ve dosyanın başka bir dönüştürmesini önerir. Tablo
// TR_pretrained_gguf'ın 12 ve _gguf_kapıları'nın 3 dönüşümünün hepsidir; doğru türdeki dosya yüklenir.
func TestGGUFMetadataTypes(t *testing.T) {
	c := tinyConfig{arch: "qwen3", vocab: 48, dim: 64, ff: 96, layers: 2, heads: 4, kv: 2, headDim: 16, context: 32, theta: 1000000}
	w := c.randomWeights(7)
	base := []struct {
		key   string
		value any
	}{
		{"general.architecture", "qwen3"}, {"qwen3.block_count", uint32(2)}, {"qwen3.embedding_length", uint32(64)},
		{"qwen3.feed_forward_length", uint32(96)}, {"qwen3.attention.head_count", uint32(4)}, {"qwen3.attention.head_count_kv", uint32(2)},
		{"qwen3.context_length", uint32(32)}, {"qwen3.rope.freq_base", float32(1000000)},
		{"qwen3.attention.layer_norm_rms_epsilon", float32(1e-5)}, {"qwen3.attention.key_length", uint32(16)},
		{"qwen3.rope.dimension_count", uint32(16)}, {"qwen3.vocab_size", uint32(48)}, {"qwen3.rope.scaling.type", "none"},
		{"qwen3.rope.scaling.factor", float32(1)}, {"qwen3.rope.scale_linear", float32(0)},
	}
	file := func(over string, value any) []byte {
		b := &gguftest.Builder{}
		for _, kv := range base {
			v := kv.value
			if kv.key == over {
				v = value
			}
			b.KV(kv.key, v)
		}
		for _, name := range sortedNames(c.names()) {
			b.Tensor(name, gguf.TypeF32, ggufDims(w[name].shape), floatBytes(w[name].data))
		}
		return b.Bytes()
	}
	text, number := "64", uint32(3)
	cases := []struct {
		key    string
		value  any
		tr, en string
	}{
		{"general.architecture", number, "Tam; Metin", "Int, the key needs type String"},
		{"qwen3.embedding_length", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.attention.head_count", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.attention.head_count_kv", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.attention.key_length", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.rope.dimension_count", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.feed_forward_length", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.attention.layer_norm_rms_epsilon", text, "Metin; Ondalık", "String, the key needs type Float"},
		{"qwen3.block_count", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.vocab_size", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.context_length", text, "Metin; Tam", "String, the key needs type Int"},
		{"qwen3.rope.freq_base", text, "Metin; Ondalık", "String, the key needs type Float"},
		{"qwen3.rope.scaling.type", number, "Tam; Metin", "Int, the key needs type String"},
		{"qwen3.rope.scaling.factor", text, "Metin; Ondalık", "String, the key needs type Float"},
		{"qwen3.rope.scale_linear", text, "Metin; Ondalık", "String, the key needs type Float"},
	}
	dir := t.TempDir()
	for i, k := range cases {
		name := fmt.Sprintf("tur%d.gguf", i)
		writeBytes(t, filepath.Join(dir, name), file(k.key, k.value))
		writeTR(t, dir, "tur.tr", fmt.Sprintf("TR_import \"tr/pretrained\"\nTR_try { TR_pretrained_gguf(%q) } TR_catch e {\n"+
			"    TR_print(e.TR_code)\n    TR_print(e.TR_text(\"tr\"))\n    TR_print(e.TR_text(\"en\"))\n}\n", name))
		s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "tur.tr"))
		got := strings.Split(strings.TrimSpace(s.Stdout+s.Stderr), "\n")
		if all := strings.Join(got, "\n"); len(got) != 3 || got[0] != "H7175" ||
			!strings.Contains(got[1], fmt.Sprintf("%q anahtarının değerinin türü %s olmalı (", k.key, k.tr)) ||
			!strings.Contains(got[1], "üstverisindedir") || !strings.Contains(got[1], "başka bir dönüştürmesini indirin") ||
			!strings.Contains(got[2], fmt.Sprintf("the value of %q has type %s (", k.key, k.en)) ||
			!strings.Contains(got[2], "download another conversion of the file") ||
			strings.Contains(all, "config") || strings.Count(all, name) != 2 {
			t.Errorf("%s: H7175'in GGUF metni anahtarı, iki türü ve dosyanın yolunu söylemeli:\n%s", k.key, all)
		}
	}
	writeBytes(t, filepath.Join(dir, "dogru.gguf"), file("", nil))
	writeTR(t, dir, "dogru.tr", "TR_import \"tr/pretrained\"\nTR_print(TR_pretrained_gguf(\"dogru.gguf\").TR_config[\"katman\"])\n")
	if s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "dogru.tr")); strings.TrimSpace(s.Stdout) != "2" {
		t.Errorf("doğru türdeki dosya yüklenmeli:\n%s%s", s.Stdout, s.Stderr)
	}
}

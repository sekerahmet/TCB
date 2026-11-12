package e2e

import (
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// gemmaTiny küçük bir Gemma 3'tür: GGUF'un izin listesindeki 18 katman, bakış boyutu 256 (tek bakış,
// tek kv). Sıfır alanların anahtarı yazılmaz: düzen 6, yerel taban 10000, ölçek yok.
type gemmaTiny struct {
	dim, headDim     int
	pattern          int
	localBase, scale float32
}

// tiny boyutları tinyConfig'in yardımcılarına (şekil, üstveri) verir.
func (g gemmaTiny) tiny() tinyConfig {
	return tinyConfig{arch: "gemma3", vocab: 48, dim: g.dim, ff: 32, layers: 18, heads: 1, kv: 1, headDim: g.headDim, context: 32,
		tied: true, theta: 1000000}
}

// names GGUF adı → Hugging Face adı; Gemma'nın dört normu ve q/k normu.
func (g gemmaTiny) names() map[string]string {
	out := map[string]string{"token_embd.weight": "model.embed_tokens.weight", "output_norm.weight": "model.norm.weight"}
	for i := range g.tiny().layers {
		for gg, h := range map[string]string{
			"attn_q.weight": "self_attn.q_proj.weight", "attn_k.weight": "self_attn.k_proj.weight",
			"attn_v.weight": "self_attn.v_proj.weight", "attn_output.weight": "self_attn.o_proj.weight",
			"attn_q_norm.weight": "self_attn.q_norm.weight", "attn_k_norm.weight": "self_attn.k_norm.weight",
			"ffn_gate.weight": "mlp.gate_proj.weight", "ffn_up.weight": "mlp.up_proj.weight", "ffn_down.weight": "mlp.down_proj.weight",
			"attn_norm.weight": "input_layernorm.weight", "post_attention_norm.weight": "post_attention_layernorm.weight",
			"ffn_norm.weight": "pre_feedforward_layernorm.weight", "post_ffw_norm.weight": "post_feedforward_layernorm.weight",
		} {
			out[fmt.Sprintf("blk.%d.%s", i, gg)] = fmt.Sprintf("model.layers.%d.%s", i, h)
		}
	}
	return out
}

// weights her ada rastgele bir ağırlık verir; normlar Hugging Face'in değeridir (0 çevresinde: Gemma
// x · (1 + w) hesaplar).
func (g gemmaTiny) weights(seed int64) map[string]tinyTensor {
	r := rand.New(rand.NewSource(seed))
	c := g.tiny()
	out := map[string]tinyTensor{}
	for _, name := range sortedNames(g.names()) {
		shape := c.shapeOf(name)
		n := 1
		for _, d := range shape {
			n *= d
		}
		data := make([]float32, n)
		for i := range data {
			data[i] = float32(r.NormFloat64()) * 0.1
			if len(shape) == 1 {
				data[i] *= 3
			}
		}
		out[name] = tinyTensor{data, shape}
	}
	return out
}

// metadata Gemma'nın anahtarlarını yazar (pencere 4); over bir anahtarın değerini değiştirir, nil siler.
// Anahtar dosyada bir kez geçer: ikinci yazım yinelenen anahtardır (H6664).
func (g gemmaTiny) metadata(b *gguftest.Builder, over map[string]any) {
	c := g.tiny()
	kv := map[string]any{"general.architecture": "gemma3", "gemma3.block_count": uint32(c.layers),
		"gemma3.embedding_length": uint32(c.dim), "gemma3.feed_forward_length": uint32(c.ff),
		"gemma3.attention.head_count": uint32(c.heads), "gemma3.attention.head_count_kv": uint32(c.kv),
		"gemma3.context_length": uint32(c.context), "gemma3.rope.freq_base": c.theta,
		"gemma3.attention.layer_norm_rms_epsilon": float32(1e-5), "gemma3.attention.sliding_window": uint32(4)}
	if c.headDim != c.dim/c.heads {
		kv["gemma3.attention.key_length"] = uint32(c.headDim)
	}
	if g.pattern > 0 {
		kv["gemma3.attention.sliding_window_pattern"] = uint32(g.pattern)
	}
	if g.localBase > 0 {
		kv["gemma3.rope.freq_base_swa"] = g.localBase
	}
	if g.scale > 0 {
		kv["gemma3.rope.scaling.type"], kv["gemma3.rope.scaling.factor"] = "linear", g.scale
	}
	for k, v := range over {
		kv[k] = v
		if v == nil {
			delete(kv, k)
		}
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.KV(k, kv[k])
	}
}

// ggufFile ağırlıkları F32 yazar; dönüştürücü gibi normlara 1 ekler, satırları dizmez.
func (g gemmaTiny) ggufFile(w map[string]tinyTensor) *gguftest.Builder {
	b := &gguftest.Builder{}
	g.metadata(b, nil)
	for _, name := range sortedNames(g.names()) {
		t := w[name]
		if len(t.shape) == 1 {
			shifted := make([]float32, len(t.data))
			for i, x := range t.data {
				shifted[i] = x + 1
			}
			t = tinyTensor{shifted, t.shape}
		}
		b.Tensor(name, gguf.TypeF32, ggufDims(t.shape), floatBytes(t.data))
	}
	return b
}

// writeHF gemma3_text'in config.json'unu (1B'nin anahtarlarıyla) ve model.safetensors'ı yazar.
func (g gemmaTiny) writeHF(t *testing.T, dir string, w map[string]tinyTensor) {
	t.Helper()
	c := g.tiny()
	pattern, base := g.pattern, g.localBase
	if pattern == 0 {
		pattern = 6
	}
	if base == 0 {
		base = 10000
	}
	var scaling any
	if g.scale > 0 {
		scaling = map[string]any{"rope_type": "linear", "factor": float64(g.scale)}
	}
	config := map[string]any{"model_type": "gemma3_text", "vocab_size": c.vocab, "hidden_size": c.dim, "intermediate_size": c.ff,
		"num_hidden_layers": c.layers, "num_attention_heads": c.heads, "num_key_value_heads": c.kv, "head_dim": c.headDim,
		"query_pre_attn_scalar": c.headDim, "max_position_embeddings": c.context, "rms_norm_eps": float64(float32(1e-5)),
		"rope_theta": float64(c.theta), "rope_local_base_freq": float64(base), "rope_scaling": scaling, "sliding_window": 4,
		"sliding_window_pattern": pattern, "hidden_activation": "gelu_pytorch_tanh"}
	writeHFFiles(t, dir, config, g.names(), w)
}

// Kural (15.55, K3): Gemma 3'ün GGUF'tan kurulan modeli safetensors'tan kurulanla aynı logitleri verir.
// GGUF'ta normlar dönüştürücünün eklediği 1'i taşır, safetensors'ta tr/pretrained ekler; satırlar
// dizilmez; bakış boyutu 256. Düzen ve yerel taban anahtarsız (öntanımlı 6 ve 10000) ve anahtarlı
// (öntanımlıdan farklı 3 ve 20000, doğrusal ölçek 8 ile); config aynı değerleri yazar. Altı parçanın
// son ikisi pencerenin (4) dışını görür: yerel ile genel katman ayrılır.
func TestGemma3GGUFMatchesSafetensors(t *testing.T) {
	for _, g := range []gemmaTiny{{dim: 64, headDim: 256}, {dim: 64, headDim: 256, pattern: 3, localBase: 20000, scale: 8}} {
		t.Run(fmt.Sprintf("düzen%d-taban%v-ölçek%v", g.pattern, g.localBase, g.scale), func(t *testing.T) {
			w := g.weights(5)
			ggufMatchesFolder(t, func(dir string) { g.writeHF(t, dir, w) }, g.ggufFile(w).Bytes())
		})
	}
}

// gemmaWindows modelin katman başına penceresini yazar.
const gemmaWindows = `TR_import "tr/pretrained"
TR_var m = TR_pretrained_gguf("m.gguf")
TR_print([b.TR_attention.TR_window TR_for b TR_in m.TR_blocks])
`

// Kural (15.55): Gemma 3'ün GGUF kapıları. Dikkat ölçeği dosyada yok: yalnız block_count 18, 26, 34,
// 48 ve bakış boyutu 256 yüklenir; 27B (62) ile listede olmayan boyun iletisi ayrıdır, bakış boyutu
// anahtardan ya da türetilerek. Düzen < 1, pozitif olmayan yerel taban ve pencere dosya hatasıdır
// (H7192), ileti başka bir dönüştürmeyi önerir; dizi biçimli düzen tür hatasıdır (H7175). NaN ve sonsuz
// taban üstveride zaten okunmaz (H6685, TR_JSON'a sığmaz). Logit kırpma ve linear dışı ölçek açık hata.
func TestGemma3GGUFGates(t *testing.T) {
	base := gemmaTiny{dim: 64, headDim: 256}
	meta := func(over map[string]any) []byte {
		b := &gguftest.Builder{}
		base.metadata(b, over)
		return b.Bytes()
	}
	kv := func(key string, value any) map[string]any { return map[string]any{key: value} }
	nan, inf := math.Float32frombits(0x7fc00000), float32(math.Inf(1))
	redo := "dosyanın başka bir dönüştürmesini indirin"
	runGGUFGateCases(t, []ggufGateCase{
		{"27B", "H7177", []string{"27B", "gemma3.block_count", "1B, 4B ve 12B"}, meta(kv("gemma3.block_count", uint32(62)))},
		{"32 katman", "H7177", []string{"bu boyu GGUF'tan yüklenmez", "gemma3.block_count", "= 32", "18, 26, 34 ya da 48"},
			meta(kv("gemma3.block_count", uint32(32)))},
		{"listede yok", "H7177", []string{"= 20"}, meta(kv("gemma3.block_count", uint32(20)))},
		{"bakış 128", "H7177", []string{"gemma3.attention.key_length", "= 128"}, meta(kv("gemma3.attention.key_length", uint32(128)))},
		{"türetilen 288", "H7177", []string{"gemma3.embedding_length / gemma3.attention.head_count", "= 288"}, func() []byte {
			b := &gguftest.Builder{}
			gemmaTiny{dim: 288, headDim: 288}.metadata(b, nil)
			return b.Bytes()
		}()},
		{"düzen 0", "H7192", []string{"gemma3.attention.sliding_window_pattern", "düzeni en az 1 olmalı", redo},
			meta(kv("gemma3.attention.sliding_window_pattern", uint32(0)))},
		{"düzen eksi", "H7192", []string{"sliding_window_pattern", redo}, meta(kv("gemma3.attention.sliding_window_pattern", int32(-1)))},
		{"düzen dizi", "H7175", []string{"gemma3.attention.sliding_window_pattern"}, meta(kv("gemma3.attention.sliding_window_pattern", []int32{1, 1, 0}))},
		{"taban 0", "H7192", []string{"gemma3.rope.freq_base_swa", "sıfırdan büyük", redo}, meta(kv("gemma3.rope.freq_base_swa", float32(0)))},
		{"taban eksi", "H7192", []string{"freq_base_swa", redo}, meta(kv("gemma3.rope.freq_base_swa", float32(-1)))},
		{"taban NaN", "H6685", []string{"freq_base_swa"}, meta(kv("gemma3.rope.freq_base_swa", nan))},
		{"taban sonsuz", "H6685", []string{"freq_base_swa"}, meta(kv("gemma3.rope.freq_base_swa", inf))},
		{"pencere 0", "H7192", []string{"gemma3.attention.sliding_window", "kayan pencere en az 1 olmalı", redo},
			meta(kv("gemma3.attention.sliding_window", uint32(0)))},
		{"pencere eksi", "H7192", []string{"gemma3.attention.sliding_window", "kayan pencere en az 1 olmalı", redo},
			meta(kv("gemma3.attention.sliding_window", int32(-1)))},
		{"pencere yok", "H6699", []string{"gemma3.attention.sliding_window"}, meta(kv("gemma3.attention.sliding_window", nil))},
		// GGUF'ta config yok: ileti hazır_gguf'u ve dosyanın üstverisini anar (A20).
		{"kırpma", "H7171", []string{"hazır_gguf:", "gemma3.final_logit_softcapping", "dosyanın üstverisini değiştirmek"},
			meta(kv("gemma3.final_logit_softcapping", float32(30)))},
		{"ölçek türü", "H6943", []string{"gemma3.rope.scaling.type"}, meta(kv("gemma3.rope.scaling.type", "yarn"))},
		{"ölçek eksi", "H6943", []string{"gemma3.rope.scaling.factor"},
			meta(map[string]any{"gemma3.rope.scaling.type": "linear", "gemma3.rope.scaling.factor": float32(-8)})},
	})
}

// Kural (15.55): düzen 1'de her katman genel, katman sayısından büyük düzende her katman yereldir
// (llama.cpp'nin formülü); anahtarsız bakış boyutu embedding_length / head_count'tan türer, 256 ise yüklenir.
func TestGemma3GGUFLayout(t *testing.T) {
	local, global := "4", "0"
	all := func(w string) string { return "[" + strings.TrimSuffix(strings.Repeat(w+", ", 18), ", ") + "]" }
	defaults := make([]string, 18)
	for i := range defaults {
		defaults[i] = local
		if i%6 == 5 {
			defaults[i] = global
		}
	}
	for _, c := range []struct {
		name string
		g    gemmaTiny
		want string
	}{
		{"öntanımlı", gemmaTiny{dim: 64, headDim: 256}, "[" + strings.Join(defaults, ", ") + "]"},
		{"düzen 1", gemmaTiny{dim: 64, headDim: 256, pattern: 1}, all(global)},
		{"düzen 100", gemmaTiny{dim: 64, headDim: 256, pattern: 100}, all(local)},
		{"türetilen 256", gemmaTiny{dim: 256, headDim: 256}, "[" + strings.Join(defaults, ", ") + "]"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeBytes(t, filepath.Join(dir, "m.gguf"), c.g.ggufFile(c.g.weights(6)).Bytes())
			writeTR(t, dir, "p.tr", gemmaWindows)
			s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "p.tr"))
			if s.Code != 0 || strings.TrimSpace(s.Stdout) != c.want {
				t.Errorf("çıkış %d, pencereler %q; beklenen %q\n%s", s.Code, s.Stdout, c.want, s.Stderr)
			}
		})
	}
}

package e2e

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// tinyConfig küçük bir modelin boyutlarıdır.
type tinyConfig struct {
	arch                        string
	vocab, dim, ff, layers      int
	heads, kv, headDim, context int
	tied                        bool
	theta                       float32
}

// tinyTensor Hugging Face düzeninde bir ağırlıktır.
type tinyTensor struct {
	data  []float32
	shape []int
}

// names GGUF adı → Hugging Face adı.
func (c tinyConfig) names() map[string]string {
	out := map[string]string{"token_embd.weight": "model.embed_tokens.weight", "output_norm.weight": "model.norm.weight"}
	if !c.tied {
		out["output.weight"] = "lm_head.weight"
	}
	for i := range c.layers {
		for g, h := range map[string]string{
			"attn_q.weight": "self_attn.q_proj.weight", "attn_k.weight": "self_attn.k_proj.weight",
			"attn_v.weight": "self_attn.v_proj.weight", "attn_output.weight": "self_attn.o_proj.weight",
			"ffn_gate.weight": "mlp.gate_proj.weight", "ffn_up.weight": "mlp.up_proj.weight",
			"ffn_down.weight": "mlp.down_proj.weight", "attn_norm.weight": "input_layernorm.weight",
			"ffn_norm.weight": "post_attention_layernorm.weight",
		} {
			out[fmt.Sprintf("blk.%d.%s", i, g)] = fmt.Sprintf("model.layers.%d.%s", i, h)
		}
		if c.arch == "qwen3" {
			out[fmt.Sprintf("blk.%d.attn_q_norm.weight", i)] = fmt.Sprintf("model.layers.%d.self_attn.q_norm.weight", i)
			out[fmt.Sprintf("blk.%d.attn_k_norm.weight", i)] = fmt.Sprintf("model.layers.%d.self_attn.k_norm.weight", i)
		}
	}
	return out
}

// shapeOf GGUF adının [çıkış, giriş] şeklidir.
func (c tinyConfig) shapeOf(name string) []int {
	qRows, kvRows := c.heads*c.headDim, c.kv*c.headDim
	switch {
	case name == "token_embd.weight", name == "output.weight":
		return []int{c.vocab, c.dim}
	case strings.HasSuffix(name, "attn_q.weight"):
		return []int{qRows, c.dim}
	case strings.HasSuffix(name, "attn_k.weight"), strings.HasSuffix(name, "attn_v.weight"):
		return []int{kvRows, c.dim}
	case strings.HasSuffix(name, "attn_output.weight"):
		return []int{c.dim, qRows}
	case strings.HasSuffix(name, "ffn_gate.weight"), strings.HasSuffix(name, "ffn_up.weight"):
		return []int{c.ff, c.dim}
	case strings.HasSuffix(name, "ffn_down.weight"):
		return []int{c.dim, c.ff}
	case strings.HasSuffix(name, "q_norm.weight"), strings.HasSuffix(name, "k_norm.weight"):
		return []int{c.headDim}
	}
	return []int{c.dim}
}

// randomWeights her GGUF adına rastgele bir ağırlık verir; normlar 1 çevresindedir.
func (c tinyConfig) randomWeights(seed int64) map[string]tinyTensor {
	r := rand.New(rand.NewSource(seed))
	out := map[string]tinyTensor{}
	for _, name := range sortedNames(c.names()) {
		shape := c.shapeOf(name)
		n := 1
		for _, d := range shape {
			n *= d
		}
		data := make([]float32, n)
		for i := range data {
			data[i] = float32(r.NormFloat64()) * 0.1
			if len(shape) == 1 {
				data[i] += 1
			}
		}
		out[name] = tinyTensor{data, shape}
	}
	return out
}

func sortedNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// converterPermute convert_hf_to_gguf.py'nin LlamaModel.permute'udur: bakış içinde
// reshape(n, 2, satır/n/2, ...).swapaxes(1, 2). Yeni satır (h, j, i) eski (h, i, j)'dir; bakış
// boyutu 8'de sıra [0, 4, 1, 5, 2, 6, 3, 7].
func converterPermute(t tinyTensor, heads int) tinyTensor {
	rows, cols := t.shape[0], t.shape[1]
	half := rows / heads / 2
	out := make([]float32, len(t.data))
	for h := range heads {
		for j := range half {
			for i := range 2 {
				from := h*2*half + i*half + j
				to := h*2*half + j*2 + i
				copy(out[to*cols:(to+1)*cols], t.data[from*cols:(from+1)*cols])
			}
		}
	}
	return tinyTensor{out, t.shape}
}

func floatBytes(v []float32) []byte {
	out := make([]byte, 0, 4*len(v))
	for _, x := range v {
		out = binary.LittleEndian.AppendUint32(out, math.Float32bits(x))
	}
	return out
}

// ggufDims dosyanın eksen sırasıdır: TR şeklinin tersi.
func ggufDims(shape []int) []uint64 {
	out := make([]uint64, len(shape))
	for i, d := range shape {
		out[len(shape)-1-i] = uint64(d)
	}
	return out
}

// metadata c'nin anahtarlarını yazar.
func (c tinyConfig) metadata(b *gguftest.Builder) {
	a := c.arch
	b.KV("general.architecture", a).KV(a+".block_count", uint32(c.layers)).KV(a+".embedding_length", uint32(c.dim)).
		KV(a+".feed_forward_length", uint32(c.ff)).KV(a+".attention.head_count", uint32(c.heads)).
		KV(a+".attention.head_count_kv", uint32(c.kv)).KV(a+".context_length", uint32(c.context)).
		KV(a+".rope.freq_base", c.theta).KV(a+".attention.layer_norm_rms_epsilon", float32(1e-5))
	if c.headDim != c.dim/c.heads {
		b.KV(a+".attention.key_length", uint32(c.headDim))
	}
}

// ggufFile ağırlıkları F32 olarak yazar; llama'da satırlar dönüştürücünün sırasıyla.
func (c tinyConfig) ggufFile(w map[string]tinyTensor) *gguftest.Builder {
	b := &gguftest.Builder{}
	c.metadata(b)
	for _, name := range sortedNames(c.names()) {
		t := w[name]
		if c.arch == "llama" && strings.HasSuffix(name, "attn_q.weight") {
			t = converterPermute(t, c.heads)
		}
		if c.arch == "llama" && strings.HasSuffix(name, "attn_k.weight") {
			t = converterPermute(t, c.kv)
		}
		b.Tensor(name, gguf.TypeF32, ggufDims(t.shape), floatBytes(t.data))
	}
	return b
}

// writeHF config.json ve model.safetensors yazar.
func (c tinyConfig) writeHF(t *testing.T, dir string, w map[string]tinyTensor) {
	t.Helper()
	config := map[string]any{"model_type": c.arch, "vocab_size": c.vocab, "hidden_size": c.dim, "intermediate_size": c.ff,
		"num_hidden_layers": c.layers, "num_attention_heads": c.heads, "num_key_value_heads": c.kv, "head_dim": c.headDim,
		"max_position_embeddings": c.context, "rms_norm_eps": float64(float32(1e-5)), "tie_word_embeddings": c.tied, "rope_theta": float64(c.theta)}
	text, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), text, 0o644); err != nil {
		t.Fatal(err)
	}
	header := map[string]any{}
	var data []byte
	names := c.names()
	for _, name := range sortedNames(names) {
		start := len(data)
		data = append(data, floatBytes(w[name].data)...)
		header[names[name]] = map[string]any{"dtype": "F32", "shape": w[name].shape, "data_offsets": []int{start, len(data)}}
	}
	head, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	file := binary.LittleEndian.AppendUint64(nil, uint64(len(head)))
	file = append(append(file, head...), data...)
	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), file, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ggufCompare iki modelin logitlerinin en büyük mutlak farkını yazar.
const ggufCompare = `TR_import "tr/pretrained"
TR_var arg = TR_command_line({"folder": "", "file": "", "other": ""})
TR_var a = TR_none
TR_if arg["folder"] != "" { a = TR_pretrained_model(arg["folder"]) } TR_else { a = TR_pretrained_gguf(arg["other"]) }
TR_var b = TR_pretrained_gguf(arg["file"])
TR_var kimlikler = TR_Tensor([[1, 5, 2, 7, 3, 9]])
TR_print(TR_max(TR_abs(a.TR_forward(kimlikler) - b.TR_forward(kimlikler))).TR_tolist)
TR_print(TR_max(TR_abs(a.TR_forward(kimlikler))).TR_tolist)
TR_print(b.TR_generate([1, 5], 4, TR_temperature: 0.0))
`

// runCompare iki modelin farkını ve logitlerin büyüklüğünü verir.
func runCompare(t *testing.T, dir string, args ...string) (diff, size float64, generated string) {
	t.Helper()
	s := testkit.RunBinary(t, trBinary, dir, "", nil, append([]string{filepath.Join(dir, "karsilastir.tr")}, args...)...)
	if s.Code != 0 {
		t.Fatalf("TR çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
	lines := strings.Split(strings.TrimSpace(s.Stdout), "\n")
	diff, err := strconv.ParseFloat(lines[0], 64)
	if err != nil {
		t.Fatal(s.Stdout)
	}
	size, err = strconv.ParseFloat(lines[1], 64)
	if err != nil {
		t.Fatal(s.Stdout)
	}
	return diff, size, lines[2]
}

// Kural (15.55): GGUF'tan kurulan model safetensors'tan kurulanla aynı logitleri verir. llama'da
// dosyanın satırları dönüştürücünün sırasındadır: bakış boyutu 8, bakış sayısı KV'den farklı (bakış
// boyutu 4'te sıralama kendi tersidir, yön hatası geçerdi). qwen3'te bakış boyutu gizli boyuttan
// bağımsız ve satırlar sırasındadır. Bağlaşık gömme output.weight'in yokluğudur.
func TestGGUFModelMatchesSafetensors(t *testing.T) {
	for _, c := range []tinyConfig{
		{arch: "llama", vocab: 48, dim: 64, ff: 96, layers: 2, heads: 8, kv: 2, headDim: 8, context: 32, tied: true, theta: 100000},
		{arch: "llama", vocab: 48, dim: 64, ff: 96, layers: 2, heads: 4, kv: 2, headDim: 16, context: 32, tied: false, theta: 10000},
		{arch: "qwen3", vocab: 48, dim: 64, ff: 96, layers: 2, heads: 4, kv: 2, headDim: 32, context: 32, tied: false, theta: 1000000},
		{arch: "qwen3", vocab: 48, dim: 64, ff: 96, layers: 1, heads: 8, kv: 4, headDim: 8, context: 32, tied: true, theta: 130000},
	} {
		t.Run(fmt.Sprintf("%s-%d-%v", c.arch, c.headDim, c.tied), func(t *testing.T) {
			dir := t.TempDir()
			writeTR(t, dir, "karsilastir.tr", ggufCompare)
			w := c.randomWeights(1)
			c.writeHF(t, dir, w)
			file := filepath.Join(dir, "m.gguf")
			writeBytes(t, file, c.ggufFile(w).Bytes())
			diff, size, _ := runCompare(t, dir, "--folder", dir, "--file", file)
			if diff > 1e-5 || size == 0 {
				t.Errorf("en büyük logit farkı %v (logit %v)", diff, size)
			}
		})
	}
}

// randomQuantized kurucu tensörü verir: küçük ölçekli rastgele bloklar.
func randomQuantized(r *rand.Rand, kind gguf.TensorType, shape []int) []byte {
	dt := map[gguf.TensorType]tensor.DType{gguf.TypeQ8_0: tensor.Q8, gguf.TypeQ4_K: tensor.Q4K, gguf.TypeQ6_K: tensor.Q6K}[kind]
	elems, size := dt.QuantBlock()
	blocks := shape[0] * shape[1] / elems
	raw := make([]byte, blocks*size)
	_, _ = r.Read(raw)
	for b := range blocks {
		block := raw[b*size : (b+1)*size]
		scale := func(p []byte, v uint16) { binary.LittleEndian.PutUint16(p, v) }
		switch kind {
		case gguf.TypeQ8_0:
			scale(block, 0x1A00+uint16(r.Intn(256))) // ~0.003
		case gguf.TypeQ4_K:
			scale(block, 0x1400+uint16(r.Intn(256)))
			scale(block[2:], 0x1400+uint16(r.Intn(256)))
		case gguf.TypeQ6_K:
			scale(block[208:], 0x0C00+uint16(r.Intn(256)))
		}
	}
	return raw
}

// Kural (15.53, 15.55): nicemlenmiş model, ağırlıkları açılmış float32 modelle 15.41'in sınırında
// aynıdır. Türler Q4_K_M dosyaları gibi tensör başına karışıktır; llama'da satır sırası nicemlenmiş
// tensörde tensörle indeksle geri alınır.
func TestGGUFQuantizedModelMatchesOpened(t *testing.T) {
	kinds := func(name string) gguf.TensorType {
		switch {
		case name == "token_embd.weight", strings.HasSuffix(name, "attn_q.weight"):
			return gguf.TypeQ8_0
		case name == "output.weight", strings.HasSuffix(name, "attn_v.weight"), strings.HasSuffix(name, "ffn_down.weight"):
			return gguf.TypeQ6_K
		}
		return gguf.TypeQ4_K
	}
	for _, c := range []tinyConfig{
		{arch: "llama", vocab: 48, dim: 256, ff: 256, layers: 2, heads: 8, kv: 2, headDim: 32, context: 32, tied: false, theta: 100000},
		{arch: "qwen3", vocab: 48, dim: 256, ff: 512, layers: 1, heads: 2, kv: 1, headDim: 128, context: 32, tied: true, theta: 1000000},
	} {
		t.Run(c.arch, func(t *testing.T) {
			dir := t.TempDir()
			writeTR(t, dir, "karsilastir.tr", ggufCompare)
			r := rand.New(rand.NewSource(2))
			q, f := &gguftest.Builder{}, &gguftest.Builder{}
			c.metadata(q)
			c.metadata(f)
			for _, name := range sortedNames(c.names()) {
				shape := c.shapeOf(name)
				if len(shape) == 1 {
					data := make([]float32, shape[0])
					for i := range data {
						data[i] = 1
					}
					q.Tensor(name, gguf.TypeF32, ggufDims(shape), floatBytes(data))
					f.Tensor(name, gguf.TypeF32, ggufDims(shape), floatBytes(data))
					continue
				}
				kind := kinds(name)
				raw := randomQuantized(r, kind, shape)
				q.Tensor(name, kind, ggufDims(shape), raw)
				dt := map[gguf.TensorType]tensor.DType{gguf.TypeQ8_0: tensor.Q8, gguf.TypeQ4_K: tensor.Q4K, gguf.TypeQ6_K: tensor.Q6K}[kind]
				opened := tensor.FromQuantized(dt, raw, shape...).Open(tensor.Float32).Data()
				f.Tensor(name, gguf.TypeF32, ggufDims(shape), floatBytes(opened))
			}
			qf, ff := filepath.Join(dir, "q.gguf"), filepath.Join(dir, "f.gguf")
			writeBytes(t, qf, q.Bytes())
			writeBytes(t, ff, f.Bytes())
			diff, size, generated := runCompare(t, dir, "--other", ff, "--file", qf)
			if size == 0 || diff > size*1e-4 {
				t.Errorf("en büyük logit farkı %v (logit %v)", diff, size)
			}
			if !strings.HasPrefix(generated, "[1, 5, ") {
				t.Errorf("üretim %q", generated)
			}
		})
	}
}

// Kural (15.55, DENETİM 129): safetensors kapıları config.json'a baktığı için GGUF'ta kendiliğinden
// geçmez. RoPE ölçeğinin dört koşulu ayrı ayrı; Qwen3-4B-128K biçimi tensörsüz gelir. Eşlenmeyen ya
// da eksik tensör hatadır: `blk.N.attn_q.bias` atlansaydı model yanlış metin üretirdi.
func TestGGUFModelGates(t *testing.T) {
	c := tinyConfig{arch: "qwen3", vocab: 48, dim: 64, ff: 96, layers: 2, heads: 4, kv: 2, headDim: 16, context: 32, theta: 1000000}
	w := c.randomWeights(3)
	full := func(edit func(b *gguftest.Builder)) []byte {
		b := c.ggufFile(w)
		if edit != nil {
			edit(b)
		}
		return b.Bytes()
	}
	without := func(name string) []byte {
		b := &gguftest.Builder{}
		c.metadata(b)
		for _, n := range sortedNames(c.names()) {
			if n != name {
				b.Tensor(n, gguf.TypeF32, ggufDims(w[n].shape), floatBytes(w[n].data))
			}
		}
		return b.Bytes()
	}
	kv := func(key string, value any) func(b *gguftest.Builder) {
		return func(b *gguftest.Builder) { b.KV(key, value) }
	}
	one := []uint64{4}
	four := floatBytes(make([]float32, 4))
	row := func(name string, dims []uint64) func(b *gguftest.Builder) {
		return func(b *gguftest.Builder) {
			n := 1
			for _, d := range dims {
				n *= int(d)
			}
			b.Tensor(name, gguf.TypeF32, dims, floatBytes(make([]float32, n)))
		}
	}
	cases := []struct {
		name, want, says string
		data             []byte
	}{
		{"mimari", "H6940", "gemma3", func() []byte {
			d := c
			d.arch = "gemma3"
			b := &gguftest.Builder{}
			d.metadata(b)
			return b.Bytes()
		}()},
		{"mimarisiz", "H6940", "", (&gguftest.Builder{}).Bytes()},
		{"rope_freqs", "H6943", "rope_freqs.weight", full(func(b *gguftest.Builder) { b.Tensor("rope_freqs.weight", gguf.TypeF32, one, four) })},
		{"ölçek türü", "H6943", "qwen3.rope.scaling.type", full(kv("qwen3.rope.scaling.type", "linear"))},
		{"ölçek çarpanı", "H6943", "qwen3.rope.scaling.factor", full(kv("qwen3.rope.scaling.factor", float32(4)))},
		{"doğrusal ölçek", "H6943", "qwen3.rope.scale_linear", full(kv("qwen3.rope.scale_linear", float32(2)))},
		{"128K biçimi", "H6943", "qwen3.rope.scaling.type", full(func(b *gguftest.Builder) {
			b.KV("qwen3.rope.scaling.type", "yarn").KV("qwen3.rope.scaling.factor", float32(4)).
				KV("qwen3.rope.scaling.original_context_length", uint32(32768))
		})},
		{"RoPE bakış boyutundan az", "H6943", "qwen3.rope.dimension_count", full(kv("qwen3.rope.dimension_count", uint32(8)))},
		{"kayan pencere", "H6944", "qwen3.attention.sliding_window", full(kv("qwen3.attention.sliding_window", uint32(4096)))},
		{"sapma", "H6945", "blk.0.attn_q.bias", full(row("blk.0.attn_q.bias", []uint64{64}))},
		{"fazla katman", "H6945", "blk.2.attn_norm.weight", full(row("blk.2.attn_norm.weight", []uint64{64}))},
		{"eksik tensör", "H6697", "blk.1.ffn_up.weight", without("blk.1.ffn_up.weight")},
		{"eksik gömme", "H6697", "token_embd.weight", without("token_embd.weight")},
		{"eksik anahtar", "H6699", "qwen3.block_count", (&gguftest.Builder{}).KV("general.architecture", "qwen3").
			KV("qwen3.embedding_length", uint32(64)).KV("qwen3.attention.head_count", uint32(4)).KV("qwen3.feed_forward_length", uint32(96)).
			KV("qwen3.attention.layer_norm_rms_epsilon", float32(1e-5)).Bytes()},
		{"eps yok", "H6699", "qwen3.attention.layer_norm_rms_epsilon", (&gguftest.Builder{}).KV("general.architecture", "qwen3").
			KV("qwen3.embedding_length", uint32(64)).KV("qwen3.attention.head_count", uint32(4)).KV("qwen3.feed_forward_length", uint32(96)).Bytes()},
	}
	dir := t.TempDir()
	for i, c := range cases {
		writeBytes(t, filepath.Join(dir, fmt.Sprintf("m%d.gguf", i)), c.data)
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := fmt.Sprintf("TR_import \"tr/pretrained\"\nTR_pretrained_gguf(\"m%d.gguf\")\n", i)
			writeTR(t, dir, "kapi.tr", text)
			s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "kapi.tr"))
			code := fmt.Sprintf("TR_import \"tr/pretrained\"\nTR_print(TR_error_code { TR_pretrained_gguf(\"m%d.gguf\") })\n", i)
			writeTR(t, dir, "kod.tr", code)
			k := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "kod.tr"))
			if got := strings.TrimSpace(k.Stdout); got != c.want {
				t.Errorf("kod %q; beklenen %s\n%s", got, c.want, k.Stderr)
			}
			if !strings.Contains(s.Stderr+s.Stdout, c.says) {
				t.Errorf("ileti %q demiyor:\n%s%s", c.says, s.Stdout, s.Stderr)
			}
		})
	}
	// Etkisiz ölçekler kabul edilir: "none", 0 ve 1 (llama.cpp'de etkisiz).
	writeBytes(t, filepath.Join(dir, "ok.gguf"), full(func(b *gguftest.Builder) {
		b.KV("qwen3.rope.scaling.type", "none").KV("qwen3.rope.scaling.factor", float32(1)).KV("qwen3.rope.scale_linear", float32(0)).
			KV("qwen3.rope.dimension_count", uint32(16))
	}))
	writeTR(t, dir, "ok.tr", "TR_import \"tr/pretrained\"\nTR_print(TR_pretrained_gguf(\"ok.gguf\").TR_config[\"katman\"])\n")
	if s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "ok.tr")); strings.TrimSpace(s.Stdout) != "2" {
		t.Errorf("etkisiz ölçek:\n%s%s", s.Stdout, s.Stderr)
	}
}

// Kural (15.55): vocab_size, key_length ve freq_base yoksa dosyadan ve varsayılandan gelir; bölüm bakış
// boyutunu verir. hazır_model'e .gguf yolu vermek hatadır, ileti doğru adı söyler.
func TestGGUFModelDefaults(t *testing.T) {
	c := tinyConfig{arch: "llama", vocab: 48, dim: 64, ff: 96, layers: 1, heads: 4, kv: 4, headDim: 16, context: 32, tied: true}
	w := c.randomWeights(4)
	b := (&gguftest.Builder{}).KV("general.architecture", "llama").KV("llama.block_count", uint32(1)).KV("llama.embedding_length", uint32(64)).
		KV("llama.feed_forward_length", uint32(96)).KV("llama.attention.head_count", uint32(4)).
		KV("llama.attention.layer_norm_rms_epsilon", float32(1e-5))
	for _, name := range sortedNames(c.names()) {
		b.Tensor(name, gguf.TypeF32, ggufDims(w[name].shape), floatBytes(w[name].data))
	}
	dir := t.TempDir()
	writeBytes(t, filepath.Join(dir, "m.gguf"), b.Bytes())
	writeTR(t, dir, "p.tr", `TR_import "tr/pretrained"
TR_var m = TR_pretrained_gguf("m.gguf")
TR_print(m.TR_config["dağarcık"], m.TR_config["rope_tabanı"], m.TR_config["head_dim"], m.TR_config["num_kv_heads"], m.TR_logit_layer == TR_none)
TR_print(TR_error_code { TR_pretrained_model("m.gguf") })
`)
	s := testkit.RunBinary(t, trBinary, dir, "", nil, filepath.Join(dir, "p.tr"))
	if s.Code != 0 {
		t.Fatalf("TR çıkış %d:\n%s\n%s", s.Code, s.Stdout, s.Stderr)
	}
	want := "48 10000.0 16 4 true\nH6996\n"
	if s.Stdout != want {
		t.Errorf("çıktı %q; beklenen %q", s.Stdout, want)
	}
}

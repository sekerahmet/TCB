package e2e

import (
	"encoding/binary"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Nicemlenmiş GGUF modeli (15.53, 15.55): aynı ağırlıkların nicemli ve açılmış F32 dosyaları; türler
// Q4_K_M dosyaları gibi karışık ya da hepsi Q4_0 (Gemma 3 QAT dosyası gibi). Dosyalar Go ile kurulur.

// ggufQuantTypes nicemli GGUF türlerinin TR karşılıklarıdır.
var ggufQuantTypes = map[gguf.TensorType]tensor.DType{gguf.TypeQ8_0: tensor.Q8, gguf.TypeQ4_0: tensor.Q4, gguf.TypeQ4_K: tensor.Q4K,
	gguf.TypeQ6_K: tensor.Q6K}

// randomQuantized kurucu tensörü verir: küçük ölçekli rastgele bloklar.
func randomQuantized(r *rand.Rand, kind gguf.TensorType, shape []int) []byte {
	dt := ggufQuantTypes[kind]
	elems, size := dt.QuantBlock()
	blocks := shape[0] * shape[1] / elems
	raw := make([]byte, blocks*size)
	_, _ = r.Read(raw)
	for b := range blocks {
		block := raw[b*size : (b+1)*size]
		scale := func(p []byte, v uint16) { binary.LittleEndian.PutUint16(p, v) }
		switch kind {
		case gguf.TypeQ8_0, gguf.TypeQ4_0:
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
// aynıdır. Türler Q4_K_M dosyaları gibi tensör başına karışıktır ya da hepsi Q4_0'dır (q4 yapılandırması);
// llama'da satır sırası nicemlenmiş tensörde tensörle indeksle geri alınır.
func TestGGUFQuantizedModelMatchesOpened(t *testing.T) {
	for _, c := range quantizedConfigs {
		t.Run(c.quantName(), func(t *testing.T) {
			dir := t.TempDir()
			writeTR(t, dir, "karsilastir.tr", ggufCompare)
			qf, ff := c.writeQuantized(t, dir)
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

// quantizedConfigs nicemli sınamaların modelleridir: her ağırlık türü geçer.
var quantizedConfigs = []tinyConfig{
	{arch: "llama", vocab: 48, dim: 256, ff: 256, layers: 2, heads: 8, kv: 2, headDim: 32, context: 32, tied: false, theta: 100000},
	{arch: "qwen3", vocab: 48, dim: 256, ff: 512, layers: 1, heads: 2, kv: 1, headDim: 128, context: 32, tied: true, theta: 1000000},
	{arch: "qwen3", vocab: 48, dim: 256, ff: 512, layers: 1, heads: 2, kv: 1, headDim: 128, context: 32, tied: true, theta: 1000000, q4: true},
}

// quantName sınamanın adıdır: mimari, q4 yapılandırmasında "_q4" ekiyle.
func (c tinyConfig) quantName() string {
	if c.q4 {
		return c.arch + "_q4"
	}
	return c.arch
}

// writeQuantized aynı ağırlıkların nicemli ve açılmış F32 GGUF dosyalarını dir'e yazar.
func (c tinyConfig) writeQuantized(t *testing.T, dir string) (quantized, opened string) {
	t.Helper()
	kinds := func(name string) gguf.TensorType {
		switch {
		case c.q4:
			return gguf.TypeQ4_0
		case name == "token_embd.weight", strings.HasSuffix(name, "attn_q.weight"):
			return gguf.TypeQ8_0
		case name == "output.weight", strings.HasSuffix(name, "attn_v.weight"), strings.HasSuffix(name, "ffn_down.weight"):
			return gguf.TypeQ6_K
		}
		return gguf.TypeQ4_K
	}
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
		dt := ggufQuantTypes[kind]
		values := tensor.FromQuantized(dt, raw, shape...).Open(tensor.Float32).Data()
		f.Tensor(name, gguf.TypeF32, ggufDims(shape), floatBytes(values))
	}
	quantized, opened = filepath.Join(dir, "q.gguf"), filepath.Join(dir, "f.gguf")
	writeBytes(t, quantized, q.Bytes())
	writeBytes(t, opened, f.Bytes())
	return quantized, opened
}

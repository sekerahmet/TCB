package e2e

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/testkit"
)

// Kural (15.22a, 15.53): nicemlenmiş modelin logiti ve üretimi işçi sayısından bağımsızdır: logit
// değerleri tam yazılır, her adımın vektörü karşılaştırılır.
func TestGGUFLogitsIndependentOfWorkers(t *testing.T) {
	c := tinyConfig{arch: "qwen3", vocab: 512, dim: 256, ff: 512, layers: 2, heads: 8, kv: 2, headDim: 32, context: 64, tied: true, theta: 1000000}
	r := rand.New(rand.NewSource(3))
	b := &gguftest.Builder{}
	c.metadata(b)
	for _, name := range sortedNames(c.names()) {
		shape := c.shapeOf(name)
		if len(shape) == 1 {
			data := make([]float32, shape[0])
			for i := range data {
				data[i] = 1
			}
			b.Tensor(name, gguf.TypeF32, ggufDims(shape), floatBytes(data))
			continue
		}
		b.Tensor(name, gguf.TypeQ8_0, ggufDims(shape), randomQuantized(r, gguf.TypeQ8_0, shape))
	}
	dir := t.TempDir()
	writeBytes(t, filepath.Join(dir, "m.gguf"), b.Bytes())
	writeTR(t, dir, "p.tr", `TR_import "tr/pretrained"
TR_var m = TR_pretrained_gguf("m.gguf")
TR_var ids = [(i * 37 + 11) % 512 TR_for i TR_in 0..20]
TR_print(m.TR_forward(TR_Tensor([ids])).TR_tolist)
TR_var ö = m.TR_build_cache()
TR_no_grad {
    TR_print(m.TR_forward(TR_Tensor([ids]), TR_cache: ö).TR_tolist)
    TR_for i TR_in 0..6 { TR_print(m.TR_forward(TR_Tensor([[ids[i]]]), TR_cache: ö).TR_tolist) }
}
TR_print(m.TR_generate(ids, 8, TR_temperature: 0.0))
`)
	var want string
	for _, n := range []int{1, 2, 3, 8} {
		s := testkit.RunBinary(t, trBinary, dir, "", []string{fmt.Sprintf("GOMAXPROCS=%d", n)}, filepath.Join(dir, "p.tr"))
		if s.Code != 0 {
			t.Fatalf("GOMAXPROCS=%d: çıkış %d:\n%s\n%s", n, s.Code, s.Stdout, s.Stderr)
		}
		if n == 1 {
			want = s.Stdout
		} else if s.Stdout != want {
			t.Errorf("GOMAXPROCS=%d: çıktı farklı", n)
		}
	}
	if !strings.Contains(want, ".") {
		t.Errorf("logit yazılmadı: %.200q", want)
	}
}

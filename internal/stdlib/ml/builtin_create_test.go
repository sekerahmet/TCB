package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/lib"
)

// cacheLogits sabit önbellekli ileri hesabın logitleridir: başlangıç ve üç tek parça. Önbelleğin
// tek tamponu [2 · 2 · 1, 256, 2, 128] float32, 1 MiB: blok eşiğinde.
const cacheLogits = `TR_import "tr/layers"
TR_var m = TR_GPT(20, TR_context: 256, TR_embed_dim: 256, TR_num_layers: 2, TR_num_heads: 2, TR_num_kv_heads: 2, TR_seed: 3)
TR_no_grad {
    TR_var ö = m.TR_build_cache(TR_capacity: 256)
    TR_var l = [m.TR_forward([[3, 7, 1, 9]], TR_cache: ö).TR_tolist]
    TR_for k TR_in [4, 12, 0] { l.TR_append(m.TR_forward([[k]], TR_cache: ö).TR_tolist) }
    TR_print(l)
}`

// Önbellek işlemcide tek blokta kurulur (bütün katmanlar için bir blok) ve logitleri öbekte
// kurulan önbelleğinkiyle bayt bayt aynıdır (A1, A5). Kartta önbellek kartta kalır, blok almaz (A3).
func TestKVCacheLogitsSameInBlockAndHeap(t *testing.T) {
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	before := tensor.CacheBlocksSpent()
	inBlock := mustRun(t, cacheLogits)
	spent := tensor.CacheBlocksSpent() - before
	if spent == 0 {
		t.Skip("bu platformda öbek dışı blok yok")
	}
	if spent != 1 {
		t.Fatalf("önbellek %d blok aldı, beklenen 1 (bütün katmanlar tek blokta)", spent)
	}
	restore := tensor.LimitCacheShareForTest()
	onHeap := mustRun(t, cacheLogits)
	restore()
	if tensor.CacheBlocksSpent() != before+1 {
		t.Fatal("pay doluyken önbellek blok aldı")
	}
	if inBlock != onHeap {
		t.Fatal("blokta ve öbekte kurulan önbelleğin logitleri farklı")
	}
	gpu.RegisterSimulator()
	defer tensor.SetDefaultDevice(tensor.CPU)
	mustRun(t, strings.Replace(cacheLogits, "\n", "\nTR_default_device(\"gpu\")\n", 1))
	if tensor.CacheBlocksSpent() != before+1 {
		t.Fatal("kartta kurulan önbellek işlemcide blok aldı")
	}
}

// Yerleşiğin kendisi: şekil listedir, tür adıyla verilir, sonuç sıfırlıdır.
func TestCacheBufferBuiltin(t *testing.T) {
	out, err := run(t, `TR_var t = TR_cache_buffer([2, 3], TR_dtype: "bf16")
TR_print(t.TR_shape, t.TR_dtype, t.TR_tolist)`)
	if err != nil || out != "[2, 3] bf16 [[0.0, 0.0, 0.0], [0.0, 0.0, 0.0]]\n" {
		t.Fatalf("önbellek_tamponu: %q, %v", out, err)
	}
	expectCodes(t, []codeCase{
		{"TR_cache_buffer(3)", messages.H6021},
		{"TR_cache_buffer([2], TR_requires_grad: TR_true)", messages.H7901},
	})
}

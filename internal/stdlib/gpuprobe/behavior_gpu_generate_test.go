package gpuprobe_test

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
	"github.com/sekerahmet/TCB/lib"
)

// Kural (15.2, 15.5, 15.15; 2a adım 6 ve 8): kitaplığın üretim yolu GPU'da cinsli tensörlerle
// çalışır: kimlik ve konum TR_Int, KV önbelleğinin maskesi TR_Bool (pencere dahil), cihaz adımı
// TR_replay kaydında. Aynı tohumla CPU'da, GPU'da çizgesiz ve GPU'da çizgeyle aynı parçalar
// üretilir. Gerçek GPU yoksa PTX benzetimidir (yavaş; model küçük).
const gpuGenerateProgram = `
TR_import "tr/layers"
TR_func üret(cihaz: TR_String, çizge: TR_Bool) -> TR_List[TR_Int] {
    TR_default_device(cihaz)
    TR_var m = TR_GPT(16, TR_context: 8, TR_embed_dim: 16, TR_num_layers: 2, TR_num_heads: 2, TR_num_kv_heads: 1, TR_windows: [2, 0], TR_seed: 8)
    TR_var r = m.TR_generate([1, 2, 3], 4, TR_temperature: 0, TR_cuda_graph: çizge)
    TR_default_device("cpu")
    TR_return r
}
TR_var c = üret("cpu", TR_false)
TR_print(c)
TR_print(c == üret("gpu", TR_false), c == üret("gpu", TR_true))
`

func TestGPUGenerateWithTypedPositions(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	out := trrun.MustExec(t, gpuGenerateProgram).Out
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || lines[1] != "true true" {
		t.Fatalf("GPU'daki üretim CPU'dakiyle aynı olmalı (çizgesiz ve çizgeyle):\n%s", out)
	}
}

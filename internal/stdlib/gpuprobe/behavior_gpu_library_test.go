package gpuprobe_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
	"github.com/sekerahmet/TCB/lib"
)

// Dilin kitaplığındaki GPT ve TR ile yazılmış iyileştiriciler (tr/katman, tr/iyilestirici)
// GPU'da CPU ile aynı kaybı verir: küçük bir GPT aynı veriyle önce CPU'da, sonra
// varsayılan_cihaz("gpu") ile GPU'da Muon + AdamW (ve NorMuon) ile birkaç adım eğitilir.
// Gerçek GPU yoksa PTX benzetimidir (yavaş; şekiller küçük).
const gpuLibraryProgram = `
TR_import "tr/katman"
TR_import "tr/iyilestirici"
TR_func eğit(cihaz, normuon) {
    TR_default_device(cihaz)
    TR_var m = TR_GPT(16, TR_context: 4, TR_embed_dim: 8, TR_num_layers: 1, TR_num_heads: 2, TR_num_kv_heads: 1, TR_seed: 3)
    TR_var op = TR_MuonAndAdamW(m, TR_normuon: normuon, TR_adamw_lr: 0.01)
    TR_var girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    TR_var hedefler = [[[1, 4, 1, 5], [9, 2, 6, 5]], [[3, 5, 8, 9], [7, 9, 3, 2]], [[3, 8, 4, 6], [2, 6, 4, 3]]]
    TR_var kayıplar = []
    TR_for adım TR_in 0..3 {
        op.TR_set_multiplier(TR_lr_schedule(adım, 3, TR_warmup: 1))
        op.TR_zero_grad()
        TR_var kayıp = m.TR_loss(TR_tensor(girdiler[adım]), TR_tensor(hedefler[adım]))
        TR_backward(kayıp)
        op.TR_step()
        kayıplar.TR_append(kayıp.TR_tolist)
    }
    TR_var c = m.TR_embedding.TR_table.TR_device + " " + m.TR_blocks[0].TR_attention.TR_q_proj.TR_weight.TR_device + " " + op.TR_muon.TR_state.TR_momentum[0].TR_device
    TR_default_device("cpu")
    TR_return [kayıplar, c]
}
TR_for normuon TR_in [TR_false, TR_true] {
    TR_var c = eğit("cpu", normuon)
    TR_var g = eğit("gpu", normuon)
    TR_print(c[1])
    TR_print(g[1])
    TR_for i TR_in 0..3 { TR_print(c[0][i], g[0][i]) }
}
`

func TestGPULibraryGPT(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	interp.SetLibrary(lib.Files)
	defer interp.SetLibrary(nil)
	out := trrun.MustExec(t, gpuLibraryProgram).Out
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 10 {
		t.Fatalf("beklenmeyen çıktı:\n%s", out)
	}
	for block := 0; block < 2; block++ {
		s := lines[block*5:]
		if s[0] != "cpu cpu cpu" || s[1] != "gpu gpu gpu" {
			t.Fatalf("ağırlıklar ve iyileştirici durumu seçilen cihazda olmalı:\n%s", out)
		}
		for _, line := range s[2:5] {
			f := strings.Fields(line)
			c, _ := strconv.ParseFloat(f[0], 64)
			g, _ := strconv.ParseFloat(f[1], 64)
			if d := c - g; d > 1e-4*c || d < -1e-4*c {
				t.Fatalf("kayıplar uyuşmuyor (CPU, GPU):\n%s", out)
			}
		}
	}
}

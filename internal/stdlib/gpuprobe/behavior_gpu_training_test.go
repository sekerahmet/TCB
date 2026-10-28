// Package gpuprobe_test, GPU arka ucunu dil düzeyinde uçtan uca sınar (ayrı süreçte koşar:
// benzetimin kaydı ve varsayılan cihaz süreç geneli durumdur).
package gpuprobe_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	_ "github.com/sekerahmet/TCB/internal/stdlib" // bütün motorlar
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// GPU'da eğitimin dil düzeyinde uçtan uca sınaması: kıyas modelinin (tools/gpu/compare_gpu.tr)
// küçültülmüşü aynı veri ve tohumla önce CPU'da, sonra varsayılan_cihaz("gpu") ile GPU'da
// eğitilir; kayıplar karşılaştırılır. Gerçek GPU yoksa GPU PTX benzetimidir (yavaş; şekiller
// küçük).
const gpuTrainingProgram = `
TR_var B = 2
TR_var T = 4
TR_var D = 8
TR_var H = 2
TR_var Dh = 4
TR_var FF = 8
TR_var V = 16
TR_var tohum_sayacı = 0
TR_func ağırlık(şekil, ölçek) {
    tohum_sayacı += 1
    TR_return TR_Tensor(TR_no_grad(TR_randn(şekil, TR_seed: tohum_sayacı) * ölçek), TR_requires_grad: TR_true)
}
TR_func başlar(x) { TR_return x.TR_reshape([B, T, H, Dh]).TR_permute([0, 2, 1, 3]) }
TR_func eğit(cihaz) {
    TR_default_device(cihaz)
    tohum_sayacı = 0
    TR_var E = ağırlık([V, D], 0.02)
    TR_var g1 = TR_Tensor(TR_ones([D]), TR_requires_grad: TR_true)
    TR_var Wq = ağırlık([D, D], 0.2)
    TR_var Wk = ağırlık([D, D], 0.2)
    TR_var Wv = ağırlık([D, D], 0.2)
    TR_var Wg = ağırlık([D, FF], 0.2)
    TR_var Wu = ağırlık([D, FF], 0.2)
    TR_var W2 = ağırlık([FF, D], 0.2)
    TR_var ağırlıklar = [E, g1, Wq, Wk, Wv, Wg, Wu, W2]
    TR_var op = TR_AdamW(ağırlıklar, TR_lr: 0.01, TR_weight_decay: 0.0)
    # Toplu işler varsayılan cihazda kurulur (veri kümesi örnekleyicisi de böyle yapar).
    TR_var girdiler = [[[3, 1, 4, 1], [5, 9, 2, 6]], [[5, 3, 5, 8], [9, 7, 9, 3]], [[2, 3, 8, 4], [6, 2, 6, 4]]]
    TR_var hedefler = [[1, 4, 1, 5, 9, 2, 6, 5], [3, 5, 8, 9, 7, 9, 3, 2], [3, 8, 4, 6, 2, 6, 4, 3]]
    TR_var kayıplar = []
    TR_var son_x = TR_none
    TR_var son_k = TR_none
    TR_for adım TR_in 0..3 {
        TR_var xg = TR_Tensor(girdiler[adım])
        TR_var yh = TR_Tensor(hedefler[adım])
        op.TR_zero_grad()
        TR_var x = TR_embedding_lookup(E, xg)
        TR_var h = TR_rms_norm(x, g1)
        TR_var y = TR_attention(başlar(h @ Wq), başlar(h @ Wk), başlar(h @ Wv), TR_causal: TR_true)
        TR_var s = x + y.TR_permute([0, 2, 1, 3]).TR_reshape([B, T, D])
        TR_var a = s @ Wg
        s = s + (a * TR_sigmoid(a) * (s @ Wu)) @ W2
        TR_var kayıp = TR_cross_entropy(s @ E.TR_transpose(), yh)
        TR_backward(kayıp)
        TR_clip_grad(ağırlıklar, TR_max: 1.0)
        op.TR_step()
        kayıplar.TR_append(kayıp.TR_tolist)
        son_x = xg
        son_k = kayıp
    }
    TR_var son_cihaz = E.TR_device + " " + son_x.TR_device + " " + son_k.TR_device
    TR_default_device("cpu")
    TR_return [kayıplar, son_cihaz]
}
TR_var c = eğit("cpu")
TR_var g = eğit("gpu")
TR_print(c[1])
TR_print(g[1])
TR_for i TR_in 0..3 { TR_print(c[0][i], g[0][i]) }
`

func TestGPUTrainingLanguageLevel(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := trrun.MustExec(t, gpuTrainingProgram).Out
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 || lines[0] != "cpu cpu cpu" || lines[1] != "gpu gpu gpu" {
		t.Fatalf("beklenmeyen çıktı:\n%s", out)
	}
	for _, s := range lines[2:] {
		f := strings.Fields(s)
		c, _ := strconv.ParseFloat(f[0], 64)
		g, _ := strconv.ParseFloat(f[1], 64)
		if d := c - g; d > 1e-4*c || d < -1e-4*c {
			t.Fatalf("kayıplar uyuşmuyor (CPU, GPU):\n%s", out)
		}
	}
}

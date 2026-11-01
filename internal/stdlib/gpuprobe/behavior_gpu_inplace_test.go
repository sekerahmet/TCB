package gpuprobe_test

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Kural (15.6d, 15.50): bileşik atama GPU'da da yerindedir ve CPU ile aynı sonucu verir:
// paylaşan adlar değişikliği görür, hassasiyet hedefinkidir, örtüşen kaynak önce kopyalanır,
// yayın yalnız sağdadır; türevin yazma kuralları cihazdan bağımsızdır. Eksensiz TR_argmax
// bütün öğelerdedir, TR_min ve TR_argmax'te −0.0 0.0'dan küçüktür (15.19, 15.22b); GPU'daki bf16
// tensör .safetensors'a konağa indirilip yazılır, geri okunur (15.47a). Gerçek GPU yoksa PTX
// benzetimidir.
const gpuInPlaceProgram = `
TR_func dene(cihaz: TR_String) -> TR_String {
    TR_default_device(cihaz)
    TR_var p = TR_Tensor([1.0, 2.0, 3.0, 4.0])
    TR_var q = p
    p -= 0.5
    p[1..4] += p[0..3]
    TR_var b = TR_Tensor([1.0, 2.0], TR_dtype: "bf16")
    b += TR_Tensor([0.5, 0.5])
    TR_var m = TR_zeros([2, 2])
    m += TR_Tensor([1.0, 2.0])
    TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
    TR_var h = w * 2.0
    TR_var kodlar = [TR_error_code { w -= 1.0 }, TR_error_code { TR_no_grad { h[0] += 1.0 } }, TR_error_code { m += TR_ones([2, 2, 2]) }]
    TR_no_grad { w *= 3.0 }
    TR_var z = TR_Tensor([[-0.0, 0.0], [0.0, -0.0]])
    TR_var en_küçük = [TR_String(TR_min(z).TR_tolist), TR_String(TR_min(z, TR_dim: 1).TR_tolist), TR_String(TR_max(z).TR_tolist)]
    TR_var yol = TR_Path(TR_temp_dir(), cihaz + ".safetensors")
    TR_save_tensors(yol, {"w": TR_Tensor([1.5, -2.0, 0.1], TR_dtype: "bf16"), "f": TR_Tensor([3.0])})
    TR_var y = TR_load_tensors(yol)
    TR_var dosya = [y.TR_keys(), y["w"].TR_dtype, y["w"].TR_tolist, y["f"].TR_tolist]
    TR_var sonuç = TR_String([q.TR_tolist, b.TR_dtype, b.TR_tolist, m.TR_tolist, w.TR_tolist, kodlar, TR_argmax(z).TR_tolist, TR_argmax(z, TR_dim: 1).TR_tolist, TR_argsort(TR_Tensor([0.0, -0.0])).TR_tolist, en_küçük, dosya])
    TR_default_device("cpu")
    TR_return sonuç
}
TR_print(dene("cpu"))
TR_print(dene("gpu"))
`

func TestGPUInPlaceCompoundAssign(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	out := trrun.MustExec(t, gpuInPlaceProgram).Out
	want := `[[0.5, 2.0, 4.0, 6.0], "bf16", [1.5, 2.5], [[1.0, 2.0], [1.0, 2.0]], [3.0, 6.0], ["H6716", "H6955", "H6958"], 1, [1, 0], [1, 0], ["-0.0", "[-0.0, -0.0]", "0.0"], [["f", "w"], "bf16", [1.5, -2.0, 0.100097656], [3.0]]]` + "\n"
	if out != want+want {
		t.Fatalf("CPU ve GPU aynı olmalı:\n%s\nbeklenen (iki kez):\n%s", out, want)
	}
}

package ml

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tensör başına tür: oluşturucular, .tür, .türe ve yazdırma.
func TestDTypeChoice(t *testing.T) {
	out := mustRun(t, `TR_var a = TR_tensor([[1.001, 2], [3, 4]], TR_dtype: "bf16")
TR_print(a.TR_dtype, a.TR_tolist)
TR_print(a)
TR_print(TR_tensor(2.5, TR_dtype: "fp16"), TR_tensor(2.5))
TR_print(TR_zeros([2], TR_dtype: "fp16").TR_dtype, TR_ones([2], TR_dtype: "bf16").TR_dtype, TR_randn([2], TR_seed: 1, TR_dtype: "bf16").TR_dtype)
TR_print(TR_tensor(a).TR_dtype, a.TR_to_dtype("float32").TR_dtype, (a @ a).TR_dtype, (a + TR_tensor([1.0, 1.0])).TR_dtype)
TR_var w = TR_tensor([1.0, 3.0], TR_requires_grad: TR_true)
TR_var h = w.TR_to_dtype("bf16")
TR_print(h.TR_dtype, h)
TR_backward(TR_sum(h * h))
TR_print(w.TR_grad.TR_tolist, w.TR_grad.TR_dtype)`)
	want := "bf16 [[1.0, 2.0], [3.0, 4.0]]\n" +
		"Tensör[2×2] bf16 [\n  [1.0000, 2.0000],\n  [3.0000, 4.0000]\n]\n" +
		"Tensör(2.5000) fp16 Tensör(2.5000)\n" +
		"fp16 bf16 bf16\n" +
		"bf16 float32 bf16 float32\n" +
		"bf16 Tensör[2] bf16 (türev izleniyor) [1.0000, 3.0000]\n" +
		"[2.0, 6.0] float32\n"
	if out != want {
		t.Fatalf("veri türleri:\n%q\nbeklenen:\n%q", out, want)
	}
	// İzleme Masası etiketi de türü gösterir (yalnızca float32 dışında).
	if e := New(tensor.NewOf(tensor.BFloat16, []float32{1, 2, 3, 4, 5, 6}, 2, 3)).WatchLabel(); e != "Tensör [2×3] bf16" {
		t.Errorf("izleme etiketi: %q", e)
	}
	if e := New(tensor.New([]float32{1, 2}, 2)).WatchLabel(); e != "Tensör [2]" {
		t.Errorf("izleme etiketi (float32): %q", e)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{`TR_tensor([1], TR_dtype: "bfloat16")`, messages.H6811},
		{`TR_tensor([1], TR_dtype: "bf61")`, messages.H6811},
		{`TR_ones([1], TR_dtype: "half")`, messages.H6811},
		{`TR_zeros([1], TR_dtype: "float64")`, messages.H6811},
		{`TR_tensor([1.0]).TR_to_dtype("fp32")`, messages.H6811},
		{`TR_tensor([1.0]).TR_to_dtype()`, messages.H6806},
		{`TR_tensor([1], TR_dtype: TR_none)`, messages.H6803},
		{`TR_tensor([1], dtype: "bf16")`, messages.H7901},
		{`TR_tensor([1.0]).tür_`, messages.H7903},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

// karışık_hassasiyet: blok değeri, iç içe bloklar, hatada geri dönüş ve geri yayılım.
func TestMixedPrecisionBlock(t *testing.T) {
	out := mustRun(t, `TR_var x = TR_tensor([[1.001]])
TR_var y = TR_tensor([[1.0]])
TR_var r = TR_mixed_precision("bf16") {
    TR_print((x @ y).TR_tolist, (x @ y).TR_dtype)
    TR_mixed_precision("fp16") { TR_print((x @ y).TR_tolist) }
    TR_mixed_precision("float32") { TR_print((x @ y).TR_tolist) }
    TR_print(([1.001] @ y).TR_tolist)
    (x @ y) * 3
}
TR_print(r.TR_tolist, (x @ y).TR_tolist)
TR_try { TR_mixed_precision("bf16") { TR_throw "dur" } } TR_catch h { TR_print(h) }
TR_print((x @ y).TR_tolist)
TR_print(TR_mixed_precision("bf16") { 5 }, TR_mixed_precision("bf16") {})`)
	want := "[[1.0]] bf16\n[[1.0009766]]\n[[1.001]]\n[1.0]\n[[3.0]] [[1.001]]\ndur\n[[1.001]]\n5 yok\n" // yakalanan fırlat öneksiz (8.2)
	if out != want {
		t.Fatalf("karışık_hassasiyet:\n%q\nbeklenen:\n%q", out, want)
	}
	expectCodes(t, []codeCase{
		{`TR_mixed_precision("bf16")`, messages.H6807},
		{`TR_mixed_precision("bf17") { 1 }`, messages.H6811},
		{`TR_mixed_precision("bf16", 3)`, messages.H6809},
		{`TR_mixed_precision() { 1 }`, messages.H6808},
	})
}

// Blok içindeki gradyanlar float32 gradyanlarına yakındır; ağırlıklar float32 kalır.
func TestMixedPrecisionGradient(t *testing.T) {
	out := mustRun(t, `TR_func hesap(karışık) {
    TR_var x = TR_randn([16, 32], TR_seed: 1)
    TR_var w = TR_tensor(TR_randn([32, 8], TR_seed: 2) * 0.2, TR_requires_grad: TR_true)
    TR_var g = TR_tensor(TR_ones([8]), TR_requires_grad: TR_true)
    TR_func ileri() {
        TR_var h = TR_layer_norm(TR_gelu(x @ w), g)
        TR_return TR_mean(TR_log_softmax(h, TR_dim: -1) * -1)
    }
    TR_var k = TR_none
    TR_if karışık { k = TR_mixed_precision("bf16") { ileri() } } TR_else { k = ileri() }
    TR_backward(k)
    TR_return [k.TR_tolist, w.TR_grad.TR_tolist, g.TR_grad.TR_tolist, w.TR_dtype, k.TR_dtype]
}
TR_var a = hesap(TR_false)
TR_var b = hesap(TR_true)
TR_print(a[3], b[3], b[4])
TR_var en = 0.0
TR_var ölçek = 0.0
TR_for i TR_in 0..32 {
    TR_for j TR_in 0..8 {
        en = TR_max(en, TR_abs(a[1][i][j] - b[1][i][j]))
        ölçek = TR_max(ölçek, TR_abs(a[1][i][j]))
    }
}
TR_print(TR_abs(a[0] - b[0]) < 0.01, en < 0.05 * ölçek)`)
	if out != "float32 float32 float32\ndoğru doğru\n" {
		t.Fatalf("karışık hassasiyet gradyanı:\n%s", out)
	}
}

// Karışık hassasiyet durumu göreve bağlıdır: blok GIL'i bırakan bir çağrıdayken (bekle)
// çalışan başka görev bloktan etkilenmez; görev beklemeden sonra kendi türünü görür.
func TestMixedPrecisionTaskIsolation(t *testing.T) {
	out := mustRun(t, `TR_var x = TR_tensor([[1.001]])
TR_var y = TR_tensor([[1.0]])
TR_var görev = []
TR_var başla = []
TR_async {
    TR_mixed_precision("bf16") {
        görev.TR_append((x @ y).TR_tolist[0][0])
        TR_sleep(0.3)
        görev.TR_append((x @ y).TR_tolist[0][0])
    }
}
TR_async {
    # Bu görev bloğun dışında: float32.
    TR_for görev.TR_length == 0 { TR_sleep(0.01) }
    başla.TR_append((x @ y).TR_tolist[0][0])
}
TR_for görev.TR_length == 0 { TR_sleep(0.01) }
TR_var ana = [(x @ y).TR_tolist[0][0], TR_mixed_precision("fp16") { TR_sleep(0.05); (x @ y).TR_tolist[0][0] }]
TR_wait_all()
TR_print(görev, başla, ana, (x @ y).TR_tolist[0][0])`)
	if out != "[1.0, 1.0] [1.001] [1.001, 1.0009766] 1.001\n" {
		t.Fatalf("görev izolasyonu:\n%s", out)
	}
}

// Kayıt sürüm 2: tür kodu başlıkta, 16 bitlik veri 2 bayt; başka sürüm reddedilir.
func TestTensorSaveVersion2(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.ToSlash(filepath.Join(dir, "turler.trt"))
	out := mustRun(t, `TR_var p = {"a": TR_tensor([[1.5, -2.25], [3.0, 0.1]], TR_dtype: "bf16"), "b": TR_tensor([0.1, 65504.0, 0.00001], TR_dtype: "fp16"), "c": TR_tensor([0.1])}
TR_save_tensors("`+fpath+`", p)
TR_var y = TR_load_tensors("`+fpath+`", TR_requires_grad: TR_true)
TR_print(y["a"].TR_dtype, y["b"].TR_dtype, y["c"].TR_dtype)
TR_print(y["a"] == p["a"], y["b"] == p["b"], y["c"] == p["c"], y["a"].TR_tolist)
TR_print(y["b"].TR_grad, y["b"])`)
	want := "bf16 fp16 float32\n" +
		"true true true [[1.5, -2.25], [3.0, 0.100097656]]\n" +
		"none Tensor[3] fp16 (türev izleniyor) [9.9976e-02, 6.5504e+04, 1.0014e-05]\n"
	if out != want {
		t.Fatalf("sürüm 2 gidiş-dönüş:\n%q\nbeklenen:\n%q", out, want)
	}
	data, err := os.ReadFile(filepath.FromSlash(fpath))
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(data[8:10]) != 2 {
		t.Fatalf("sürüm 2 bekleniyordu: % x", data[:10])
	}
	// Başlık 14 + her girdi (4 + ad + 2 + 8·boyut + veri) + sağlama 4.
	wantLen := 14 + (4 + 1 + 2 + 16 + 4*2) + (4 + 1 + 2 + 8 + 3*2) + (4 + 1 + 2 + 8 + 4) + 4
	if len(data) != wantLen {
		t.Fatalf("dosya boyu %d, beklenen %d (16 bitlik veri 2 bayt yazılmalı)", len(data), wantLen)
	}
	// İlk girdinin tür kodu: başlık (14) + ad uzunluğu (4) + "a".
	if data[14+4+1] != 2 {
		t.Fatalf("bf16 tür kodu 2 olmalı, %d", data[14+4+1])
	}

	// Yalnızca sürüm 2 okunur; başka sürüm açık bir hatayla reddedilir. Öğüt yapılabilir olandır
	// (B18): eski dosya okunamadığı için tensör_kaydet ile yeniden yazılamaz, veri yeniden üretilir.
	v1 := version1File(t, "w", []int{2}, []float32{0.5, -1.25}, 1)
	v1Path := filepath.ToSlash(filepath.Join(dir, "eski.trt"))
	if err := os.WriteFile(filepath.FromSlash(v1Path), v1, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, `TR_load_tensors("`+v1Path+`")`); err == nil || !messages.Has(err, messages.H6611) || strings.Contains(err.Error(), "tensör_kaydet") || !strings.Contains(err.Error(), "yeniden üretin") {
		t.Errorf("sürüm 1 dosyası reddedilmeli, öğüt veriyi yeniden üretmek: %v", err)
	}
	// Eksik 16 bitlik veri bayt sayısıyla bildirilir.
	if _, err := run(t, `TR_load_tensors("`+writeClipped(t, dir, data, 14+4+1+2+16+4)+`")`); err == nil || !messages.Has(err, messages.H6623) {
		t.Errorf("eksik bf16 verisi: %v", err)
	}
}

// version1File tek girdili, artık okunmayan eski sürüm 1 biçiminde bir dosya üretir.
func version1File(t *testing.T, ad string, shape []int, data []float32, dtypeCodeVal byte) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("TRTENSOR")
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(1))
	binary.Write(&b, binary.LittleEndian, uint32(len(ad)))
	b.WriteString(ad)
	b.WriteByte(dtypeCodeVal)
	b.WriteByte(byte(len(shape)))
	for _, d := range shape {
		binary.Write(&b, binary.LittleEndian, uint64(d))
	}
	for _, x := range data {
		binary.Write(&b, binary.LittleEndian, math.Float32bits(x))
	}
	binary.Write(&b, binary.LittleEndian, crc32.ChecksumIEEE(b.Bytes()))
	return b.Bytes()
}

// writeClipped verinin ilk n baytını geçici bir dosyaya yazar ve yolunu verir.
func writeClipped(t *testing.T, dir string, data []byte, n int) string {
	t.Helper()
	p := filepath.Join(dir, "kirpik"+strconv.Itoa(n)+".trt")
	if err := os.WriteFile(p, data[:n], 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(p)
}

// İyileştiricilerin ana_kopya seçeneği SetMasterWeights'e bağlanır.
func TestOptimizerMasterCopy(t *testing.T) {
	out := mustRun(t, `TR_func eğit(ad, ana) {
    TR_var p = TR_tensor([1.0], TR_dtype: "bf16", TR_requires_grad: TR_true)
    TR_var o = TR_none
    TR_if ad == "SGD" { o = TR_SGD([p], TR_lr: 0.001, TR_master_weights: ana) } TR_else { o = TR_AdamW([p], TR_lr: 0.001, TR_weight_decay: 0.0, TR_master_weights: ana) }
    TR_for i TR_in 0..20 {
        o.TR_zero_grad()
        TR_backward(TR_sum(p))
        o.TR_step()
    }
    TR_return p.TR_tolist[0]
}
TR_print(eğit("SGD", TR_false), eğit("AdamW", TR_false), eğit("SGD", TR_true) < 0.99, eğit("AdamW", TR_true) < 0.99)`)
	if out != "1.0 1.0 doğru doğru\n" {
		t.Fatalf("ana_kopya: %q", out)
	}
	if _, err := run(t, `TR_AdamW([TR_tensor([1.0], TR_requires_grad: TR_true)], TR_master_weights: "evet")`); err == nil || !messages.Has(err, messages.H7905) {
		t.Errorf("ana_kopya türü: %v", err)
	}
}

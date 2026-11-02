package ml

import (
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (2a adım 8; 15.1, 15.3, 15.5, 15.15, 15.19, 15.48): GPU'da bugün çalışan yollar cinsli
// tensörle de çalışır ve CPU ile aynı cinsi ve değeri verir: argmax'in TR_Int sonucu cihazda,
// TR_Int ve TR_Bool taşınır, tam sayı içinde .TR_to_dtype ve tam sayıya tip çağrısı (sığma ve NaN
// denetimiyle), karşılaştırma TR_Bool verir, maskeler & ve ~ ile birleşir, TR_where mantık
// koşulla seçer. Bu dilimin dışındaki cinsli işlem GPU'da hassasiyet hatasıdır (H6969).
func TestTypedPathsOnGPU(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)

	const program = `TR_func yollar(cihaz: TR_String) -> TR_List[TR_String] {
    TR_var x = TR_Tensor([[1.0, 5.0], [7.0, 2.0]]).TR_to_device(cihaz)
    TR_var i = TR_argmax(x, TR_dim: -1)
    TR_var m = (x > 1.5) & ~(x > 6.0)
    TR_var sonuç = [i.TR_dtype + " " + m.TR_dtype, TR_String(i.TR_tolist)]
    sonuç.TR_append(TR_String(i.TR_to_dtype("int32").TR_to_dtype("int64").TR_to_device("cpu").TR_tolist))
    sonuç.TR_append(TR_String(TR_Tensor[TR_Int](x * -0.5).TR_tolist) + " " + TR_Tensor[TR_Float](i).TR_dtype)
    sonuç.TR_append(TR_String(m.TR_tolist))
    sonuç.TR_append(TR_String(TR_where(m, x, 0.0).TR_tolist) + " " + TR_String(TR_where(i == 1, i, i).TR_tolist))
    sonuç.TR_append(TR_String(TR_Tensor([TR_true, TR_false]).TR_to_device(cihaz).TR_to_device("cpu").TR_tolist))
    TR_return sonuç
}
TR_var c = yollar("cpu")
TR_var g = yollar("gpu")
TR_var x = TR_ones([1]).TR_to_device("gpu")
TR_print(c == g, TR_argmax(x, TR_dim: 0).TR_device, (x > 0.0).TR_device)
TR_for s TR_in g { TR_print(s) }`
	out := mustRun(t, program)
	want := "true gpu gpu\nint64 bool\n[1, 0]\n[1, 0]\n[[0, -2], [-3, -1]] float32\n[[false, true], [false, true]]\n[[0.0, 5.0], [0.0, 2.0]] [1, 0]\n[true, false]\n"
	if out != want {
		t.Fatalf("GPU'nun cinsli yolları:\n%s\nbeklenen:\n%s", out, want)
	}
	for code, expr := range map[messages.Code]string{
		messages.H6964: `TR_Tensor([300]).TR_to_device("gpu").TR_to_dtype("uint8")`,
		messages.H6969: `TR_Tensor([1, 2]).TR_to_device("gpu") + 1`,
	} {
		if _, err := run(t, expr); !messages.Has(err, code) {
			t.Errorf("%s: %v, beklenen %s", expr, err, code)
		}
	}
	// H6969'un önerdiği yazım (`t.TR_to_device("cpu")`) koşar ve işlemin sonucunu verir.
	if out := mustRun(t, `TR_var t = TR_Tensor([1, 2]).TR_to_device("gpu")
TR_print((t.TR_to_device("cpu") + 1).TR_tolist)`); out != "[2, 3]\n" {
		t.Errorf("H6969'un önerisi: %q, beklenen [2, 3]", out)
	}
	// Cinsli hedefe atama (15.6d) GPU'da CPU ile aynıdır: sayı, başka hassasiyetten tensör, mantık;
	// maskede `&=` yerindedir (takma ad u değişikliği görür).
	if out := mustRun(t, `TR_func ata(cihaz: TR_String) -> TR_String {
    TR_var t = TR_Tensor([0, 0, 0]).TR_to_device(cihaz)
    t[2] = 5
    t[0..2] = TR_Tensor([7, 8], TR_dtype: "int32").TR_to_device(cihaz)
    TR_var m = TR_Tensor([TR_true, TR_false]).TR_to_device(cihaz)
    m[1] = TR_true
    TR_var u = m
    m &= TR_Tensor([TR_true, TR_false]).TR_to_device(cihaz)
    TR_return TR_String(t.TR_to_device("cpu").TR_tolist) + " " + TR_String(u.TR_to_device("cpu").TR_tolist)
}
TR_print(ata("cpu"), ata("gpu") == ata("cpu"))`); out != "[7, 8, 5] [true, false] true\n" {
		t.Errorf("GPU'da cinsli atama: %q", out)
	}
}

package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestForwardOps(t *testing.T) {
	code := `TR_var a = TR_Tensor([[1, 2, 3], [4, 5, 6]])
TR_var b = TR_Tensor([[1, 0], [0, 1], [1, 1]])
`
	cases := []struct {
		expr  string
		data  []float32
		shape []int
	}{
		{"a @ b", []float32{4, 5, 10, 11}, []int{2, 2}},
		{"a + [10, 20, 30]", []float32{11, 22, 33, 14, 25, 36}, []int{2, 3}},
		{"2 * a - 1", []float32{1, 3, 5, 7, 9, 11}, []int{2, 3}},
		{"a / 2", []float32{0.5, 1, 1.5, 2, 2.5, 3}, []int{2, 3}},
		{"a * TR_Tensor([[2], [3]])", []float32{2, 4, 6, 12, 15, 18}, []int{2, 3}},
		{"1 + a", []float32{2, 3, 4, 5, 6, 7}, []int{2, 3}},
		{"TR_sum(a)", []float32{21}, []int{}},
		{"TR_sum(a, TR_dim: 1)", []float32{6, 15}, []int{2}},
		{"TR_mean(a, TR_dim: 0)", []float32{2.5, 3.5, 4.5}, []int{3}},
		{"TR_max(a)", []float32{6}, []int{}},
		{"TR_min(a, TR_dim: -1)", []float32{1, 4}, []int{2}},
		{"TR_max(a, TR_dim: 0)", []float32{4, 5, 6}, []int{3}},
		{"TR_relu(a - 3)", []float32{0, 0, 0, 1, 2, 3}, []int{2, 3}},
		{"TR_sqrt(TR_Tensor([4, 9]))", []float32{2, 3}, []int{2}},
		{"TR_exp(TR_Tensor([0]))", []float32{1}, []int{1}},
		{"TR_log(TR_Tensor([1]))", []float32{0}, []int{1}},
		{"a.TR_reshape([3, 2]).TR_shape.TR_length", nil, nil},
		{"a.TR_transpose()", []float32{1, 4, 2, 5, 3, 6}, []int{3, 2}},
		{"a.TR_transpose()", []float32{1, 4, 2, 5, 3, 6}, []int{3, 2}},
		{"a.TR_reshape([-1])", []float32{1, 2, 3, 4, 5, 6}, []int{6}},
		{"a.TR_slice(1, 1, 3)", []float32{2, 3, 5, 6}, []int{2, 2}},
		{"a.TR_slice(0, -1, 2)", []float32{4, 5, 6}, []int{1, 3}},
		{"TR_Tensor([[[1, 2], [3, 4]]]).TR_permute([0, 2, 1])", []float32{1, 3, 2, 4}, []int{1, 2, 2}},
		{"TR_Tensor([1, 2, 3]) @ TR_Tensor([1, 1, 1])", []float32{6}, []int{}},
		{"TR_zeros([2, 3])", []float32{0, 0, 0, 0, 0, 0}, []int{2, 3}},
		{"TR_ones([2])", []float32{1, 1}, []int{2}},
		{"TR_embedding_lookup(TR_Tensor([[1, 1], [2, 2], [3, 3]]), [2, 0])", []float32{3, 3, 1, 1}, []int{2, 2}},
		{"TR_embedding_lookup(TR_Tensor([[1, 1], [2, 2], [3, 3]]), 1)", []float32{2, 2}, []int{2}},
		{"TR_layer_norm(TR_Tensor([[1, 2, 3]]), TR_ones([3]), TR_zeros([3]))", []float32{-1.2247, 0, 1.2247}, []int{1, 3}},
		{"TR_rms_norm(TR_Tensor([[3, 4]]), TR_none)", []float32{0.8485, 1.1314}, []int{1, 2}},
	}
	for _, d := range cases {
		if d.data == nil {
			continue
		}
		v := value(t, code+"TR_var cikti = "+d.expr)
		data, sh := tensorData(t, v)
		shapeEqual(t, d.expr, sh, d.shape)
		near(t, d.expr, data, d.data, 1e-3)
	}

	// softmax satırları 1'e toplanır; log_softmax'ın üsteli softmax'tır.
	v := value(t, code+"TR_var cikti = TR_sum(TR_softmax(a, TR_dim: -1), TR_dim: 1)")
	data, _ := tensorData(t, v)
	near(t, "softmax toplamı", data, []float32{1, 1}, 1e-5)
	v = value(t, code+"TR_var cikti = TR_exp(TR_log_softmax(a)) - TR_softmax(a)")
	data, _ = tensorData(t, v)
	near(t, "log_softmax", data, make([]float32, 6), 1e-5)

	// çapraz entropi = -log(softmax[hedef])
	v = value(t, "TR_var cikti = TR_cross_entropy(TR_Tensor([[2.0, 1.0, 0.1]]), [0])")
	data, _ = tensorData(t, v)
	near(t, "çapraz_entropi", data, []float32{0.4170}, 1e-3)

	// Özellikler.
	out := mustRun(t, code+`TR_print(a.TR_shape, a.TR_device, TR_Type(a))
TR_print(a.TR_tolist)
TR_print(TR_sum(a).TR_tolist)
TR_print(TR_randn([2, 3], TR_seed: 7).TR_shape)`)
	want := "[2, 3] cpu Tensor\n[[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]]\n21.0\n[2, 3]\n"
	if out != want {
		t.Fatalf("özellik çıktısı:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestPrintFormat(t *testing.T) {
	out := mustRun(t, `TR_print(TR_Tensor([[1, 2], [3, 4]]))
TR_print(TR_Tensor([1.5, -2]))
TR_print(TR_sum(TR_Tensor([1, 2], TR_requires_grad: TR_true)))`)
	want := "Tensor[2×2] [\n  [1.0000, 2.0000],\n  [3.0000, 4.0000]\n]\n" +
		"Tensor[2] [ 1.5000, -2.0000]\n" +
		"Tensor(3.0000, requires grad)\n"
	if out != want {
		t.Fatalf("yazdırma:\n%s\nbeklenen:\n%s", out, want)
	}
	// Büyük tensörlerde eksenler kısaltılır.
	out = mustRun(t, `TR_print(TR_zeros([100, 100]))`)
	if !strings.Contains(out, "…") || strings.Count(out, "\n") > 12 {
		t.Fatalf("büyük tensör kısaltılmadı:\n%s", out)
	}
}

func TestOldBehaviourKept(t *testing.T) {
	// Tek sayıdan tensör 0 boyutludur; eski tek sayılık otomatik türev kullanımı aynı
	// sonuçları verir (gradyan da 0 boyutlu tensördür). Matris adlı oluşturucular 2 boyutlu
	// tensör verir; eleman bazlı fonksiyonlar sayıda Ondalık verir.
	out := mustRun(t, `TR_var x = TR_Tensor(3.0, TR_requires_grad: TR_true)
TR_var y = x * x + 2 * x + 1
TR_backward(y)
TR_print(x.TR_tolist, y.TR_tolist, x.TR_grad, TR_Type(x))
TR_print(TR_zeros([2, 3]).TR_shape, TR_Type(TR_ones([2, 2])), TR_Type(TR_zeros([3])))
TR_print(TR_sum([1, 2, 3]), TR_min([3, 1, 2]), TR_max(4, 9), TR_sqrt(9), TR_mean([1, 2, 3]))
TR_print(TR_softmax(TR_Tensor([1.0, 1.0])).TR_tolist, TR_relu(-2), TR_Type(TR_Tensor([[1, 2]]).TR_transpose()))`)
	want := "3.0 16.0 Tensor(8.0000) Tensor\n[2, 3] Tensor Tensor\n6 1 9 3.0 2.0\n[0.5, 0.5] 0.0 Tensor\n"
	if out != want {
		t.Fatalf("eski davranış:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestBackwardGradient(t *testing.T) {
	// k = ortalama((x @ w)^2), dk/dw = 2/N · xᵀ (x @ w)
	v := value(t, `TR_var w = TR_Tensor([[0.5], [-0.5]], TR_requires_grad: TR_true)
TR_var x = TR_Tensor([[1, 2], [3, 4]])
TR_var y = x @ w
TR_backward(TR_mean(y * y))
TR_var cikti = w.TR_grad`)
	data, sh := tensorData(t, v)
	shapeEqual(t, "gradyan", sh, []int{2, 1})
	// y = [-0.5, -0.5]; dk/dy = y; dk/dw = xᵀ y = [[-2], [-3]]
	near(t, "gradyan", data, []float32{-2, -3}, 1e-5)

	// Gradyanlar birikir; iyileştiricinin .sıfırla() metodu sıfırlar.
	out := mustRun(t, `TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(w * 3))
TR_backward(TR_sum(w * 3))
TR_print(w.TR_grad.TR_tolist)
TR_var op = TR_SGD([w], TR_lr: 0.1)
op.TR_zero_grad()
TR_print(w.TR_grad.TR_tolist)
TR_backward(TR_sum(w * w))
op.TR_step()
TR_print(w.TR_tolist)`)
	want := "[6.0, 6.0]\n[0.0, 0.0]\n[0.8, 1.6]\n"
	if out != want {
		t.Fatalf("birikim/SGD:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestNoGrad(t *testing.T) {
	out := mustRun(t, `TR_var w = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var a = TR_no_grad { TR_return w * 2 }
TR_var b = TR_no_grad(w * 2)
TR_var c = w * 2
TR_print(a, b, c)
TR_try { TR_backward(TR_sum(a)) } TR_catch h { TR_print("hata") }`)
	want := "Tensor[2] [2.0000, 4.0000] Tensor[2] [2.0000, 4.0000] Tensor[2] (requires grad) [2.0000, 4.0000]\nhata\n"
	if out != want {
		t.Fatalf("türevsiz:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestLinearRegressionAdamW(t *testing.T) {
	// y = 2·x1 - 3·x2 + 0.5 verisinden ağırlıklar AdamW ile öğrenilir.
	out := mustRun(t, `TR_var X = TR_Tensor([[0, 0], [1, 0], [0, 1], [1, 1], [2, 1], [1, 2]])
TR_var Y = TR_Tensor([[0.5], [2.5], [-2.5], [-0.5], [1.5], [-3.5]])
TR_var w = TR_Tensor([[0.0], [0.0]], TR_requires_grad: TR_true)
TR_var b = TR_Tensor([0.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w, b], TR_lr: 0.05, TR_weight_decay: 0)
TR_for i TR_in 0..1500 {
    op.TR_zero_grad()
    TR_var fark = X @ w + b - Y
    TR_var kayıp = TR_mean(fark * fark)
    TR_backward(kayıp)
    op.TR_step()
}
TR_print(TR_round(w.TR_tolist[0][0], 2), TR_round(w.TR_tolist[1][0], 2), TR_round(b.TR_tolist[0], 2))`)
	if out != "2.0 -3.0 0.5\n" {
		t.Fatalf("regresyon öğrenilemedi: %q", out)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		code string
		want messages.Code
	}{
		{"TR_zeros([32, 1024]) @ TR_zeros([512, 10])", messages.H6126},
		{"TR_zeros([2, 3, 4]) @ TR_zeros([5, 4, 6])", messages.H6115},
		{"TR_zeros([2, 3]) + TR_zeros([4])", messages.H6100},
		{"TR_zeros([2, 3]) * TR_zeros([3, 2])", messages.H6100},
		{"TR_print(TR_zeros([2, 3]).TR_to_device(\"tpu\"))", messages.H6800},
		{"TR_print(TR_zeros([2, 3]).TR_slice(1, 2, 5))", messages.H6720},
		{"TR_print(TR_zeros([2, 3]).TR_permute([0, 0]))", messages.H6715},
		{"TR_print(TR_Tensor([[1, 2], [3]]))", messages.H6027},
		{"TR_print(TR_Tensor([1, \"a\"]))", messages.H6007},
		{"TR_backward(TR_zeros([2], TR_requires_grad: TR_true) * 2)", messages.H6401},
		{"TR_backward(TR_sum(TR_zeros([2])))", messages.H6402},
		{"TR_print(TR_embedding_lookup(TR_zeros([3, 2]), [5]))", messages.H6305},
		{"TR_print(TR_cross_entropy(TR_zeros([2, 4]), [1]))", messages.H6313},
		{"TR_print(TR_cross_entropy(TR_zeros([1, 4]), [4]))", messages.H6314},
		{"TR_print(TR_layer_norm(TR_zeros([2, 3]), TR_ones([4])))", messages.H6319},
		{"TR_AdamW([TR_zeros([2])])", messages.H6513},
		{"TR_AdamW([TR_zeros([2], TR_requires_grad: TR_true) * 2])", messages.H6514},
		{"TR_SGD([TR_Tensor([1.0], TR_requires_grad: TR_true)], öğrenme_oran: 0.1)", messages.H7900},
		{"TR_zeros([2, -1])", messages.H6024},
	}
	for _, d := range cases {
		_, err := run(t, d.code)
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", d.code)
			continue
		}
		if !messages.Has(err, d.want) {
			t.Errorf("%s:\n  hata  : %v\n  beklenen: %s", d.code, err, d.want)
		}
		if strings.Contains(err.Error(), "İç hata") || strings.Contains(err.Error(), "tensor:") {
			t.Errorf("%s: panik sızdı: %v", d.code, err)
		}
	}
	// Satır öneki iletinin başındadır (biçim denetimi).
	for _, d := range []struct{ code, expected string }{
		{"TR_print(TR_sum(TR_zeros([2, 3]), TR_dim: 2))", "Satır 1: toplam: eksen 2, [2, 3] şeklindeki 2 boyutlu tensör için geçersiz"},
		{"TR_print(TR_softmax(TR_zeros([2]), TR_dim: -3))", "Satır 1: softmax: eksen -3"},
		{"TR_print(TR_zeros([2, 3]).TR_to_device(\"gpu:9\"))", "Satır 1: gpu:9 cihazı kullanılamıyor"},
		{"TR_print(TR_zeros([2, 3]).TR_reshape([4, 2]))", "Satır 1: [2, 3] şeklindeki tensör (6 eleman) [4, 2] şekline sokulamaz"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
	// Hatalar dene/yakala ile yakalanabilir.
	out := mustRun(t, `TR_try { TR_var k = TR_zeros([2, 3]) @ TR_zeros([2, 3]) } TR_catch h { TR_print("yakalandı") }
TR_try { TR_print(TR_zeros([1]).TR_to_device("gpu:9")) } TR_catch h { TR_print("yakalandı") }`)
	if out != "yakalandı\nyakalandı\n" {
		t.Fatalf("dene/yakala: %q", out)
	}
}

func TestPanicToError(t *testing.T) {
	// Çekirdeğin panikleri sıradan hataya çevrilir.
	_, err := safe(func() int { panic("tensor: deneme") })
	if err == nil || err.Error() != "tensör işlemi başarısız: deneme" {
		t.Fatalf("panik çevirisi: %v", err)
	}
}

func TestWatchPanel(t *testing.T) {
	v := value(t, "TR_var cikti = TR_Tensor([[1, 2, 3], [4, 5, 6]])")
	d := v.(*TensorValue)
	if d.WatchLabel() != "Tensör [2×3]" || d.WatchKind() != "tensor" {
		t.Fatalf("etiket/tür: %q %q", d.WatchLabel(), d.WatchKind())
	}
	cols, rows, err := d.TableData()
	if err != nil || len(cols) != 3 || len(rows) != 2 || rows[1][2] != "6" {
		t.Fatalf("tablo: %v %v %v", cols, rows, err)
	}
	v = value(t, "TR_var cikti = TR_zeros([2, 2, 2])")
	if _, _, err := v.(*TensorValue).TableData(); err == nil {
		t.Fatal("3 boyutlu tensör tablo olmamalı")
	}
}

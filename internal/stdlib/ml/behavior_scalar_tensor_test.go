package ml

import (
	"path/filepath"
	"testing"
)

// Tek sayıdan oluşturulan tensör 0 boyutlu sıradan bir tensördür: eski tek sayılık türev
// değerinin yerini alır ve diğer tensörlerle aynı işlemlere girer.
func TestScalarTensor(t *testing.T) {
	fpath := filepath.ToSlash(filepath.Join(t.TempDir(), "s.trt"))
	out := mustRun(t, `TR_var x = TR_tensor(3.0, TR_requires_grad: TR_true)
TR_var y = x * x + 2 * x + 1
TR_backward(y)
TR_print(y.TR_tolist, x.TR_grad.TR_tolist, x.TR_grad == 8.0, x.TR_shape, TR_type(x.TR_grad))
TR_print(x)
TR_print(TR_tensor(3.0), TR_tensor(2), TR_tensor(2, TR_true))
# İki tensör türü yoktur: 0 boyutlu tensör çok boyutlularla işleme girer.
TR_print((TR_tensor(3.0) + TR_tensor([1.0, 2.0])).TR_tolist, TR_exp(TR_tensor(0.0)).TR_tolist, TR_sum([TR_tensor(1.0), TR_tensor([2.0])]).TR_tolist)
# Eleman bazlı fonksiyonların türevi: σ'(0) = 0.25, tanh'(0) = 1, relu'(2) = 1.
TR_var a = TR_tensor(0.0, TR_requires_grad: TR_true)
TR_backward(TR_sigmoid(a) + TR_tanh(a))
TR_var b = TR_tensor(2.0, TR_requires_grad: TR_true)
TR_backward(TR_relu(b) * b)
TR_print(a.TR_grad.TR_tolist, b.TR_grad.TR_tolist)
# Bölme: x / y ve sayı / x türevleri.
TR_var p = TR_tensor(2.0, TR_requires_grad: TR_true)
TR_var q = TR_tensor(4.0, TR_requires_grad: TR_true)
TR_backward(p / q + 1 / p - 3 - p)
TR_print(p.TR_grad.TR_tolist, q.TR_grad.TR_tolist)
# Gradyanlar birikir (çok boyutlu tensörlerle aynı kural).
TR_var c = TR_tensor(1.0, TR_requires_grad: TR_true)
TR_backward(c * 3)
TR_backward(c * 3)
TR_print(c.TR_grad.TR_tolist)
# 0 boyutlu tensör iyileştirici parametresi olabilir ve kaydedilip yüklenebilir.
TR_var w = TR_tensor(5.0, TR_requires_grad: TR_true)
TR_var op = TR_SGD([w], TR_lr: 0.25)
TR_backward(w * w)
op.TR_step()
TR_print(w.TR_tolist)
TR_save_tensors("`+fpath+`", {"w": w})
TR_print(TR_load_tensors("`+fpath+`")["w"].TR_tolist, TR_tensor(7.0) == 7, TR_tensor(7.0) != 7.5, TR_tensor([7.0]) == 7)`)
	want := "16.0 8.0 true [] Tensor\n" +
		"Tensor(3.0000, türev izleniyor)\n" +
		"Tensor(3.0000) Tensor(2.0000) Tensor(2.0000, türev izleniyor)\n" +
		"[4.0, 5.0] 1.0 [3.0]\n" +
		"1.25 4.0\n" +
		"-1.0 -0.125\n" +
		"6.0\n" +
		"2.5\n" +
		"2.5 true true false\n"
	if out != want {
		t.Fatalf("skaler tensör:\n%q\nbeklenen:\n%q", out, want)
	}
}

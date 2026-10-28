package ml

import (
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestArgMax(t *testing.T) {
	out := mustRun(t, `TR_var x = TR_Tensor([[1, 9, 3], [7, 2, 7]])
TR_print(TR_argmax(x).TR_tolist, TR_argmax(x, TR_dim: 0).TR_tolist, TR_argmax(TR_Tensor([4, 8, 1])).TR_tolist)
TR_print(TR_Int(TR_argmax(TR_Tensor([4, 8, 1])).TR_tolist), TR_argmax(x).TR_shape)
TR_print(TR_embedding_lookup(TR_Tensor([[0, 0], [1, 1], [2, 2]]), TR_argmax(TR_Tensor([[0, 0, 5]]))).TR_tolist)
TR_var harfler = ["a", "b", "c"]
TR_var i = TR_argmax(TR_Tensor([[[0, 9, 1], [5, 0, 0]]]), TR_dim: -1)
TR_print(harfler[i.TR_tolist[0][0]] + harfler[i.TR_tolist[0][1]], TR_Type(i.TR_tolist[0][1]), TR_Type(TR_argmax(TR_Tensor([1, 3])).TR_tolist))
TR_print(i, TR_argmax(TR_Tensor([2, 1])))
TR_print(TR_Type((i * 2).TR_tolist[0][0]), TR_Type(TR_Tensor(i).TR_tolist[0][0]))`)
	want := "[1, 0] [1, 0, 1] 1\n1 [2]\n[[2.0, 2.0]]\n" +
		"ba Int Int\n" +
		"Tensor[1×2] [\n  [1, 0]\n] Tensor(0)\n" +
		"Float Float\n"
	if out != want {
		t.Fatalf("argmax:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"TR_argmax([1, 2])", messages.H6217},
		{"TR_argmax(TR_Tensor([1, 2]), 0)", messages.H6215},
		{"TR_argmax(TR_Tensor([1, 2]), TR_dim: 1)", messages.H6016},
		{"TR_argmax(TR_zeros([0]))", messages.H6209},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.want)
		}
	}
}

func TestListReductions(t *testing.T) {
	// Listedeki tensörler çizgeyi koruyarak katlanır.
	out := mustRun(t, `TR_var a = TR_Tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var b = TR_Tensor([3.0, 4.0])
TR_print(TR_sum([a, b]).TR_tolist, TR_mean([1, 2, b]).TR_shape)
TR_backward(TR_sum(TR_sum([a, a, 1])))
TR_print(a.TR_grad.TR_tolist)
TR_print(TR_sum([1, 2, 3]), TR_max(4, 9), TR_min([3, 1]), TR_mean([1, 2, 3]))`)
	want := "[4.0, 6.0] [2]\n[2.0, 2.0]\n6 9 1 2.0\n"
	if out != want {
		t.Fatalf("liste indirgemesi:\n%q\nbeklenen:\n%q", out, want)
	}
	v := value(t, "TR_var cikti = TR_mean([1, 2, TR_Tensor([3.0, 6.0])])")
	data, _ := tensorData(t, v)
	near(t, "ortalama listesi", data, []float32{2, 3}, 1e-6)

	errs := []struct {
		code string
		want messages.Code
	}{
		{"TR_sum([[1, 2], [3, 4]])", messages.H6220},
		{"TR_max([[1, 2], [3, 4]])", messages.H6220},
		{"TR_mean([[1, 2]])", messages.H6220},
		{"TR_max([TR_Tensor([1]), TR_Tensor([2])])", messages.H6224},
		{"TR_sum([1, 2], TR_dim: 0)", messages.H6219},
		{"TR_max(TR_Tensor([1, 2]), 0)", messages.H6205},
		{"TR_min(TR_Tensor([1, 2]), 0)", messages.H6205},
		{"TR_zeros(2, 3, TR_requires_grad: TR_true)", messages.H6020},
		{"TR_randn(2, 3)", messages.H6020},
		{"TR_AdamW([TR_Tensor([1.0], TR_requires_grad: TR_true)], lr: 0.1)", messages.H7900},
		{"TR_AdamW([TR_Tensor([1.0], TR_requires_grad: TR_true)], weight_decay: 0.1)", messages.H7900},
		{"TR_sum(TR_Tensor([1.0]), axis: 0)", messages.H7900},
		{"TR_zeros([2], requires_grad: TR_true)", messages.H7900},
		{"TR_randn([2], seed: 1)", messages.H7900},
		{"TR_layer_norm(TR_ones([1, 2]), eps: 0.1)", messages.H7900},
	}
	for _, d := range errs {
		_, err := run(t, d.code)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.code, err, d.want)
		}
	}
	// toplam/ortalama için konumlu eksen kalır.
	v = value(t, "TR_var cikti = TR_sum(TR_Tensor([[1, 2], [3, 4]]), 0)")
	data, _ = tensorData(t, v)
	near(t, "konumlu eksen", data, []float32{4, 6}, 0)
}

func BenchmarkMaxAll(b *testing.B) {
	t := tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = t.Reshape(-1).Max(0, false)
	}
}

func BenchmarkMinAll(b *testing.B) {
	t := tensor.Randn(rand.New(rand.NewSource(1)), 1000, 1000)
	for i := 0; i < b.N; i++ {
		_ = minAll(t)
	}
}

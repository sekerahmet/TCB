package ml

import (
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Matris ayrı bir tür değildir: matris 2 boyutlu tensördür (tensör([[...]]), birim_matris,
// sıfırlar([2, 3])), işleçler ve eleman bazlı fonksiyonlar tensör yolundan geçer, sonuçlar eski
// float64 matrisle (float32 duyarlığında) aynıdır.
func TestMatrixIsTensor(t *testing.T) {
	out := mustRun(t, `TR_var G = TR_tensor([[1.0, 2.0], [3.0, 4.0]])
TR_var A = TR_tensor([[0.5, -0.5], [1.0, 2.0]])
TR_print(TR_type(G), G.TR_shape, TR_type(G @ A), TR_type(TR_zeros([2, 3])), TR_type(TR_eye(2)), TR_type(TR_randn([2, 2])))
TR_print(TR_relu(G @ A))
TR_print((G @ A).TR_tolist, (G + A).TR_tolist, (G - 1).TR_tolist, (2 * G).TR_tolist, (G / 2).TR_tolist, (G * A).TR_tolist)
TR_print((G + TR_tensor([10, 20])).TR_tolist, G.TR_transpose().TR_tolist, G.TR_transpose().TR_tolist, TR_tensor([[1, 2]]).TR_transpose().TR_shape)
TR_print(TR_zeros([2, 3]).TR_shape, TR_zeros([1, 4]).TR_shape, TR_ones([2, 2]).TR_tolist, TR_eye(3).TR_tolist)
TR_print(TR_softmax(TR_tensor([[1, 2], [3, 3]])).TR_tolist, TR_sigmoid(TR_zeros([1, 2])).TR_tolist, TR_tanh(TR_zeros([1, 1])).TR_tolist)
TR_print(TR_tensor(G) == G)`)
	want := "Tensor [2, 2] Tensor Tensor Tensor Tensor\n" +
		"Tensor[2×2] [\n  [2.5000, 3.5000],\n  [5.5000, 6.5000]\n]\n" +
		"[[2.5, 3.5], [5.5, 6.5]] [[1.5, 1.5], [4.0, 6.0]] [[0.0, 1.0], [2.0, 3.0]] [[2.0, 4.0], [6.0, 8.0]] [[0.5, 1.0], [1.5, 2.0]] [[0.5, -1.0], [3.0, 8.0]]\n" +
		"[[11.0, 22.0], [13.0, 24.0]] [[1.0, 3.0], [2.0, 4.0]] [[1.0, 3.0], [2.0, 4.0]] [2, 1]\n" +
		"[2, 3] [1, 4] [[1.0, 1.0], [1.0, 1.0]] [[1.0, 0.0, 0.0], [0.0, 1.0, 0.0], [0.0, 0.0, 1.0]]\n" +
		"[[0.26894143, 0.7310586], [0.5, 0.5]] [[0.5, 0.5]] [[0.0]]\n" +
		"true\n"
	if out != want {
		t.Fatalf("matris:\n%q\nbeklenen:\n%q", out, want)
	}
	// Başka dillerin matris adları (NumPy/PyTorch .T) tensördeki karşılıklarını önerir.
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"TR_tensor([[1]]).T", messages.H7903},
		{"TR_var A = TR_tensor([[1]])\nA.TR_transpose @ A", messages.H6033},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%q: %v (beklenen: %s)", d.code, err, d.want)
		}
	}
}

// Matris hücresine ve satırına indeksle atama, eski matris gibi çalışır.
func TestMatrixIndexAndAssign(t *testing.T) {
	out := mustRun(t, `TR_var M = TR_tensor([[1, 2], [3, 4]])
M[0][1] = 10
M[1][0] += 5
M[-1] = [8, 9]
TR_print(M[0][1] == 10, M[1][0].TR_tolist, M[1].TR_tolist, M.TR_tolist)
TR_var T = TR_zeros([2, 2, 2])
T[1] = 7
T[0][1] = [1, 2]
T[0][0][0] -= 3
TR_print(T.TR_tolist)
TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_no_grad { w[0] = 5 }
TR_print(w.TR_tolist)`)
	want := "true 8.0 [8.0, 9.0] [[1.0, 10.0], [8.0, 9.0]]\n" +
		"[[[-3.0, 0.0], [1.0, 2.0]], [[7.0, 7.0], [7.0, 7.0]]]\n" +
		"[5.0, 2.0]\n"
	if out != want {
		t.Fatalf("indeks ve atama:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct {
		code string
		want messages.Code
	}{
		{"TR_var M = TR_tensor([[1, 2], [3, 4]])\nM[0][0] = \"a\"", messages.H6032},
		{"TR_var t = TR_tensor(1.0)\nt[0] = 2", messages.H6704},
	} {
		if _, err := run(t, d.code); err == nil || !messages.Has(err, d.want) {
			t.Errorf("%q: %v (beklenen: %s)", d.code, err, d.want)
		}
	}
	// Satır öneki iletinin başındadır (biçim denetimi).
	for _, d := range []struct{ code, expected string }{
		{"TR_var M = TR_tensor([[1, 2], [3, 4]])\nM[2][0] = 1", "Satır 2: tensör indeksi sınır dışı: 2"},
		{"TR_var M = TR_tensor([[1, 2], [3, 4]])\nM[0] = [1, 2, 3]", "Satır 2: tensör ataması: [2] şeklindeki yere [3] şeklinde veri atanamaz"},
		{"TR_var w = TR_tensor([1.0], TR_requires_grad: TR_true)\nw[0] = 2", "Satır 2: türev izlenen tensöre indeksle atama yapılamaz"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%q: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
	// Yerinde değişiklik, değeri kullanmış ve henüz geri yayılmamış çizgeyi geçersiz kılar.
	_, err := run(t, `TR_var w = TR_tensor([1.0, 2.0], TR_requires_grad: TR_true)
TR_var x = TR_tensor([3.0, 4.0])
TR_var kayip = TR_sum(w * x)
x[0] = 9
TR_backward(kayip)`)
	if err == nil {
		t.Fatal("yerinde değişiklikten sonra geri yayılım hata vermeliydi")
	}
}

func TestMatrixShapeCheck(t *testing.T) {
	for _, code := range []string{
		`TR_zeros([-2, 2])`,
		`TR_zeros(2, 3)`,
		`TR_ones(3)`,
		`TR_zeros([2], [3])`,
		`TR_eye(-1)`,
		`TR_eye(2.5)`,
		`TR_randn([2, -7])`,
		`TR_randn([2, 2], TR_seed: "a")`,
		`TR_zeros([100000, 100000])`,
		`TR_tensor([[1, 2], [3]])`,
		`TR_tensor([["a"]])`,
		`TR_tensor([1, 2]).TR_transpose()`,
	} {
		_, err := run(t, code)
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", code)
		} else if messages.Has(err, messages.H9000) {
			t.Errorf("%s: panik yerine düzgün hata bekleniyordu: %v", code, err)
		}
	}
}

// Tensörler JSON ve veritabanı gibi dış biçimlere iç içe sayı listesi olarak yazılır.
func TestTensorPlainValue(t *testing.T) {
	for _, d := range []struct {
		t    *tensor.Tensor
		want string
	}{
		{tensor.New([]float32{1, 0.5, -2, 3}, 2, 2), "[[1.0, 0.5], [-2.0, 3.0]]"},
		{tensor.Scalar(0.1), "0.1"},
		{tensor.New([]float32{1, 2, 3}, 3), "[1.0, 2.0, 3.0]"},
	} {
		if got := New(d.t).PlainValue().Inspect(); got != d.want {
			t.Errorf("%v: %s, %s bekleniyordu", d.t.Shape(), got, d.want)
		}
	}
	// Eski float64 matris aritmetiğiyle aynı sonuç (float32 duyarlığında).
	got := value(t, `TR_var cikti = TR_sigmoid(TR_tensor([[0.3, -1.2]]) @ TR_tensor([[2.0], [0.7]]))`).(*TensorValue).t.Item()
	if want := 1 / (1 + math.Exp(-(0.3*2.0 + -1.2*0.7))); math.Abs(float64(got)-want) > 1e-6 {
		t.Fatalf("sigmoid(@): %v, %v bekleniyordu", got, want)
	}
}

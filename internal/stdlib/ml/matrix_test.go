package ml

import (
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Matris ayrı bir tür değildir: matris 2 boyutlu tensördür (tensör([[...]]), birim_matris,
// sıfırlar([2, 3])), işleçler ve eleman bazlı fonksiyonlar tensör yolundan geçer, sonuçlar eski
// float64 matrisle (float32 duyarlığında) aynıdır.
func TestMatrixIsTensor(t *testing.T) {
	out := mustRun(t, `değişken G = tensör([[1.0, 2.0], [3.0, 4.0]])
değişken A = tensör([[0.5, -0.5], [1.0, 2.0]])
yazdır(tip(G), G.şekil, tip(G @ A), tip(sıfırlar([2, 3])), tip(birim_matris(2)), tip(rastgele_normal([2, 2])))
yazdır(relu(G @ A))
yazdır((G @ A).değer, (G + A).değer, (G - 1).değer, (2 * G).değer, (G / 2).değer, (G * A).değer)
yazdır((G + tensör([10, 20])).değer, G.devrik().değer, G.devrik().değer, tensör([[1, 2]]).devrik().şekil)
yazdır(sıfırlar([2, 3]).şekil, sıfırlar([1, 4]).şekil, birler([2, 2]).değer, birim_matris(3).değer)
yazdır(softmax(tensör([[1, 2], [3, 3]])).değer, sigmoid(sıfırlar([1, 2])).değer, tanh(sıfırlar([1, 1])).değer)
yazdır(tensör(G) == G)`)
	want := "Tensör [2, 2] Tensör Tensör Tensör Tensör\n" +
		"Tensör[2×2] [\n  [2.5000, 3.5000],\n  [5.5000, 6.5000]\n]\n" +
		"[[2.5, 3.5], [5.5, 6.5]] [[1.5, 1.5], [4.0, 6.0]] [[0.0, 1.0], [2.0, 3.0]] [[2.0, 4.0], [6.0, 8.0]] [[0.5, 1.0], [1.5, 2.0]] [[0.5, -1.0], [3.0, 8.0]]\n" +
		"[[11.0, 22.0], [13.0, 24.0]] [[1.0, 3.0], [2.0, 4.0]] [[1.0, 3.0], [2.0, 4.0]] [2, 1]\n" +
		"[2, 3] [1, 4] [[1.0, 1.0], [1.0, 1.0]] [[1.0, 0.0, 0.0], [0.0, 1.0, 0.0], [0.0, 0.0, 1.0]]\n" +
		"[[0.26894143, 0.7310586], [0.5, 0.5]] [[0.5, 0.5]] [[0.0]]\n" +
		"doğru\n"
	if out != want {
		t.Fatalf("matris:\n%q\nbeklenen:\n%q", out, want)
	}
	// Kaldırılan matris üyeleri tensördeki karşılıklarını önerir.
	for _, d := range []struct{ code, expected string }{
		{"tensör([[1]]).boyut", "\"boyut\" özelliği ya da metodu bulunamadı. Bunu mu demek istediniz: 'şekil'?"},
		{"tensör([[1]]).T", "Bunu mu demek istediniz: 'devrik()'?"},
		{"tensör([[1]]).toplam", "'tensör.toplam()' TR'de yok; 'toplam(tensör)' yazın"},
		{"değişken A = tensör([[1]])\nA.devrik @ A", "metodu çağırmak için parantez ekleyin: devrik()"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%q: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
}

// matris ve rastgele_matris kaldırıldı (2 boyutlu tensörün ikinci adlarıydı): kullanılırsa
// hata tensör ve rastgele_normal'i önerir.
func TestMatrixNamesRemoved(t *testing.T) {
	for _, d := range []struct{ code, expected string }{
		{"matris([[1, 2]])", "'matris' TR'de yok; 'tensör' yazmak mı istediniz?"},
		{"rastgele_matris(2, 2)", "'rastgele_matris' TR'de yok; 'rastgele_normal' yazmak mı istediniz?"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%q: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
}

// Matris hücresine ve satırına indeksle atama, eski matris gibi çalışır.
func TestMatrixIndexAndAssign(t *testing.T) {
	out := mustRun(t, `değişken M = tensör([[1, 2], [3, 4]])
M[0][1] = 10
M[1][0] += 5
M[-1] = [8, 9]
yazdır(M[0][1] == 10, M[1][0].değer, M[1].değer, M.değer)
değişken T = sıfırlar([2, 2, 2])
T[1] = 7
T[0][1] = [1, 2]
T[0][0][0] -= 3
yazdır(T.değer)
değişken w = tensör([1.0, 2.0], türev_izle: doğru)
türevsiz { w[0] = 5 }
yazdır(w.değer)`)
	want := "doğru 8.0 [8.0, 9.0] [[1.0, 10.0], [8.0, 9.0]]\n" +
		"[[[-3.0, 0.0], [1.0, 2.0]], [[7.0, 7.0], [7.0, 7.0]]]\n" +
		"[5.0, 2.0]\n"
	if out != want {
		t.Fatalf("indeks ve atama:\n%q\nbeklenen:\n%q", out, want)
	}
	for _, d := range []struct{ code, expected string }{
		{"değişken M = tensör([[1, 2], [3, 4]])\nM[2][0] = 1", "Satır 2: tensör indeksi sınır dışı: 2"},
		{"değişken M = tensör([[1, 2], [3, 4]])\nM[0] = [1, 2, 3]", "Satır 2: tensör ataması: [2] şeklindeki yere [3] şeklinde veri atanamaz"},
		{"değişken M = tensör([[1, 2], [3, 4]])\nM[0][0] = \"a\"", "tensör ataması: Metin tipi tensöre dönüştürülemez"},
		{"değişken w = tensör([1.0], türev_izle: doğru)\nw[0] = 2", "Satır 2: türev izlenen tensöre indeksle atama yapılamaz"},
		{"değişken t = tensör(1.0)\nt[0] = 2", "skaler (0 boyutlu) tensör indekslenemez"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%q: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
	// Yerinde değişiklik, değeri kullanmış ve henüz geri yayılmamış çizgeyi geçersiz kılar.
	_, err := run(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken x = tensör([3.0, 4.0])
değişken kayip = toplam(w * x)
x[0] = 9
geri_yayılım(kayip)`)
	if err == nil {
		t.Fatal("yerinde değişiklikten sonra geri yayılım hata vermeliydi")
	}
}

func TestMatrixShapeCheck(t *testing.T) {
	for _, code := range []string{
		`sıfırlar([-2, 2])`,
		`sıfırlar(2, 3)`,
		`birler(3)`,
		`sıfırlar([2], [3])`,
		`birim_matris(-1)`,
		`birim_matris(2.5)`,
		`rastgele_normal([2, -7])`,
		`rastgele_normal([2, 2], tohum: "a")`,
		`sıfırlar([100000, 100000])`,
		`tensör([[1, 2], [3]])`,
		`tensör([["a"]])`,
		`tensör([1, 2]).devrik()`,
	} {
		_, err := run(t, code)
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", code)
		} else if strings.Contains(err.Error(), "çökme önlendi") {
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
	got := value(t, `değişken cikti = sigmoid(tensör([[0.3, -1.2]]) @ tensör([[2.0], [0.7]]))`).(*TensorValue).t.Item()
	if want := 1 / (1 + math.Exp(-(0.3*2.0 + -1.2*0.7))); math.Abs(float64(got)-want) > 1e-6 {
		t.Fatalf("sigmoid(@): %v, %v bekleniyordu", got, want)
	}
}

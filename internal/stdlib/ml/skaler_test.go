package ml

import (
	"path/filepath"
	"testing"
)

// Tek sayıdan oluşturulan tensör 0 boyutlu sıradan bir tensördür: eski tek sayılık türev
// değerinin yerini alır ve diğer tensörlerle aynı işlemlere girer.
func TestSkalerTensor(t *testing.T) {
	yol := filepath.ToSlash(filepath.Join(t.TempDir(), "s.trt"))
	out := mustCalistir(t, `değişken x = tensör(3.0, türev_izle: doğru)
değişken y = x * x + 2 * x + 1
geri_yayılım(y)
yazdır(y.değer, x.gradyan.değer, x.gradyan == 8.0, x.şekil, tip(x.gradyan))
yazdır(x)
yazdır(tensör(3.0), tensör(2), tensör(2, doğru))
// İki tensör türü yoktur: 0 boyutlu tensör çok boyutlularla işleme girer.
yazdır((tensör(3.0) + tensör([1.0, 2.0])).değer, üstel(tensör(0.0)).değer, toplam([tensör(1.0), tensör([2.0])]).değer)
// Eleman bazlı fonksiyonların türevi: σ'(0) = 0.25, tanh'(0) = 1, relu'(2) = 1.
değişken a = tensör(0.0, türev_izle: doğru)
geri_yayılım(sigmoid(a) + tanh(a))
değişken b = tensör(2.0, türev_izle: doğru)
geri_yayılım(relu(b) * b)
yazdır(a.gradyan.değer, b.gradyan.değer)
// Bölme: x / y ve sayı / x türevleri.
değişken p = tensör(2.0, türev_izle: doğru)
değişken q = tensör(4.0, türev_izle: doğru)
geri_yayılım(p / q + 1 / p - 3 - p)
yazdır(p.gradyan.değer, q.gradyan.değer)
// Gradyanlar birikir (çok boyutlu tensörlerle aynı kural).
değişken c = tensör(1.0, türev_izle: doğru)
geri_yayılım(c * 3)
geri_yayılım(c * 3)
yazdır(c.gradyan.değer)
// 0 boyutlu tensör iyileştirici parametresi olabilir ve kaydedilip yüklenebilir.
değişken w = tensör(5.0, türev_izle: doğru)
değişken op = SGD([w], öğrenme_oranı: 0.25)
geri_yayılım(w * w)
op.adım()
yazdır(w.değer)
tensör_kaydet("`+yol+`", {"w": w})
yazdır(tensör_yükle("`+yol+`")["w"].değer, tensör(7.0) == 7, tensör(7.0) != 7.5, tensör([7.0]) == 7)`)
	want := "16.0 8.0 doğru [] Tensör\n" +
		"Tensör(3.0000, türev izleniyor)\n" +
		"Tensör(3.0000) Tensör(2.0000) Tensör(2.0000, türev izleniyor)\n" +
		"[4.0, 5.0] 1.0 [3.0]\n" +
		"1.25 4.0\n" +
		"-1.0 -0.125\n" +
		"6.0\n" +
		"2.5\n" +
		"2.5 doğru doğru yanlış\n"
	if out != want {
		t.Fatalf("skaler tensör:\n%q\nbeklenen:\n%q", out, want)
	}
}

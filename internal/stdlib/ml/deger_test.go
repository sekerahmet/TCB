package ml

import (
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

func TestDogrulukDegeri(t *testing.T) {
	out := mustCalistir(t, `eğer tensör([0.0]) { yazdır("yanlış") } değilse { yazdır("sıfır yanlıştır") }
eğer toplam(tensör([1, -1])) { yazdır("yanlış") } değilse { yazdır("skaler sıfır yanlıştır") }
eğer tensör([2.5]) { yazdır("sıfır olmayan doğrudur") }
eğer sıfırlar([0]) { yazdır("yanlış") } değilse { yazdır("boş yanlıştır") }`)
	want := "sıfır yanlıştır\nskaler sıfır yanlıştır\nsıfır olmayan doğrudur\nboş yanlıştır\n"
	if out != want {
		t.Fatalf("doğruluk:\n%q", out)
	}
}

func TestDegerKisaGosterim(t *testing.T) {
	out := mustCalistir(t, `değişken a = tensör([0.1, 0.2, 1.5, 3])
yazdır(a.değer, a.değer[0] == 0.1, toplam(tensör([0.1])).değer)
değişken m = tensör([[1, 2], [3, 4]]).değer
m[0].ekle(9)
yazdır(m)`)
	want := "[0.1, 0.2, 1.5, 3.0] doğru 0.1\n[[1.0, 2.0, 9], [3.0, 4.0]]\n"
	if out != want {
		t.Fatalf("değer:\n%q\nbeklenen:\n%q", out, want)
	}
	// Yapı: 3 boyutlu tensör doğru iç içe listeye dönüşür.
	d := Yeni(tensor.Arange(24).Reshape(2, 3, 4))
	l := d.degerListesi().(*interp.ListVal)
	if got := l.Elements[1].(*interp.ListVal).Elements[2].(*interp.ListVal).Elements[3].(*interp.FloatVal).Val; got != 23 {
		t.Fatalf("[1][2][3] = %v", got)
	}
	if got := l.Inspect(); !strings.HasPrefix(got, "[[[0.0, 1.0, 2.0, 3.0], [4.0") {
		t.Fatalf("liste: %s", got)
	}
	// Boş eksenli tensör.
	if got := Yeni(tensor.Zeros(2, 0)).degerListesi().Inspect(); got != "[[], []]" {
		t.Fatalf("boş: %s", got)
	}
}

func TestDegerAyirmaSayisi(t *testing.T) {
	d := Yeni(tensor.Zeros(100, 40))
	ayirma := testing.AllocsPerRun(5, func() { d.degerListesi() })
	if ayirma > 10 {
		t.Fatalf(".değer %v ayırma yaptı; önceden ayrılmış dilimler bekleniyordu", ayirma)
	}
}

func TestAraTensorGradyani(t *testing.T) {
	_, err := calistir(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
geri_yayılım(toplam(w * w))
w = w - w.gradyan * 0.1
yazdır(w.gradyan)`)
	if err == nil || !strings.Contains(err.Error(), "bir işlemin sonucu; gradyan yalnızca türev_izle: doğru ile oluşturulan tensörlerde tutulur") {
		t.Fatalf("ara tensör gradyanı: %v", err)
	}
	// Geri yayılımdan sonra da ara tensör ara sayılır ve iyileştiriciye verilemez.
	_, err = calistir(t, `değişken w = tensör([1.0], türev_izle: doğru)
değişken y = w * 2
geri_yayılım(toplam(y))
AdamW([y])`)
	if err == nil || !strings.Contains(err.Error(), "yaprak değil") {
		t.Fatalf("ara tensör parametre: %v", err)
	}
	// Henüz geri yayılım yapılmamış yaprakta gradyan yoktur.
	out := mustCalistir(t, `yazdır(tensör([1.0], türev_izle: doğru).gradyan, sıfırlar([2]).gradyan)`)
	if out != "yok yok\n" {
		t.Fatalf("yaprak gradyanı: %q", out)
	}
}

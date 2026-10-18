package tensor

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"tr_dili/pkg/lexer"
	"tr_dili/pkg/motor/yz"
	"tr_dili/pkg/parser"
	"tr_dili/pkg/vm"
)

// Testlerdeki VM'ler, gerçek programdaki sırayla yapay zekâ ve tensör motorlarını içersin.
func init() {
	vm.MotorEkle(yz.Kaydet)
	vm.MotorEkle(Kaydet)
}

// calistir TR kodunu çalıştırır; çıktıyı ve hatayı döndürür.
func calistir(t *testing.T, kod string) (string, error) {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := vm.New()
	var out bytes.Buffer
	m.Out = &out
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

func mustCalistir(t *testing.T, kod string) string {
	t.Helper()
	out, err := calistir(t, kod)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\nkod:\n%s", err, kod)
	}
	return out
}

// deger kodu çalıştırır ve son satırdaki 'cikti' değişkeninin değerini döndürür.
func deger(t *testing.T, kod string) vm.Value {
	t.Helper()
	p := parser.New(lexer.New(kod))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := vm.New()
	m.Out = &bytes.Buffer{}
	if _, err := m.Run(prog); err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	v, ok := m.GlobalEnv.Get("cikti")
	if !ok {
		t.Fatal("'cikti' tanımlı değil")
	}
	return v
}

func tensorVeri(t *testing.T, v vm.Value) ([]float32, []int) {
	t.Helper()
	d, ok := v.(*Deger)
	if !ok {
		t.Fatalf("tensör beklendi, %s (%s) geldi", v.Inspect(), v.Type())
	}
	return d.t.Data(), d.t.Shape()
}

func yakin(t *testing.T, ad string, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d eleman, %d bekleniyordu (%v)", ad, len(got), len(want), got)
	}
	for i := range got {
		if math.Abs(float64(got[i]-want[i])) > tol {
			t.Fatalf("%s: [%d] = %v, %v bekleniyordu (tümü %v)", ad, i, got[i], want[i], got)
		}
	}
}

func sekilEsit(t *testing.T, ad string, got, want []int) {
	t.Helper()
	if sekilMetni(got) != sekilMetni(want) {
		t.Fatalf("%s: şekil %v, %v bekleniyordu", ad, got, want)
	}
}

func TestIleriIslemler(t *testing.T) {
	kod := `değişken a = tensör([[1, 2, 3], [4, 5, 6]])
değişken b = tensör([[1, 0], [0, 1], [1, 1]])
`
	durumlar := []struct {
		ifade string
		veri  []float32
		sekil []int
	}{
		{"a @ b", []float32{4, 5, 10, 11}, []int{2, 2}},
		{"a + [10, 20, 30]", []float32{11, 22, 33, 14, 25, 36}, []int{2, 3}},
		{"2 * a - 1", []float32{1, 3, 5, 7, 9, 11}, []int{2, 3}},
		{"a / 2", []float32{0.5, 1, 1.5, 2, 2.5, 3}, []int{2, 3}},
		{"a * tensör([[2], [3]])", []float32{2, 4, 6, 12, 15, 18}, []int{2, 3}},
		{"1 + a", []float32{2, 3, 4, 5, 6, 7}, []int{2, 3}},
		{"toplam(a)", []float32{21}, []int{}},
		{"toplam(a, eksen: 1)", []float32{6, 15}, []int{2}},
		{"ortalama(a, eksen: 0)", []float32{2.5, 3.5, 4.5}, []int{3}},
		{"maks(a)", []float32{6}, []int{}},
		{"min(a, eksen: -1)", []float32{1, 4}, []int{2}},
		{"en_büyük(a, eksen: 0)", []float32{4, 5, 6}, []int{3}},
		{"relu(a - 3)", []float32{0, 0, 0, 1, 2, 3}, []int{2, 3}},
		{"karekök(tensör([4, 9]))", []float32{2, 3}, []int{2}},
		{"üstel(tensör([0]))", []float32{1}, []int{1}},
		{"log(tensör([1]))", []float32{0}, []int{1}},
		{"a.şekillendir([3, 2]).şekil.uzunluk", nil, nil},
		{"a.devrik()", []float32{1, 4, 2, 5, 3, 6}, []int{3, 2}},
		{"devrik(a)", []float32{1, 4, 2, 5, 3, 6}, []int{3, 2}},
		{"a.şekillendir([-1])", []float32{1, 2, 3, 4, 5, 6}, []int{6}},
		{"a.dilim(1, 1, 3)", []float32{2, 3, 5, 6}, []int{2, 2}},
		{"a.dilim(0, -1, 2)", []float32{4, 5, 6}, []int{1, 3}},
		{"tensör([[[1, 2], [3, 4]]]).eksenleri_sırala([0, 2, 1])", []float32{1, 3, 2, 4}, []int{1, 2, 2}},
		{"tensör([1, 2, 3]) @ tensör([1, 1, 1])", []float32{6}, []int{}},
		{"sıfırlar([2, 3])", []float32{0, 0, 0, 0, 0, 0}, []int{2, 3}},
		{"birler([2])", []float32{1, 1}, []int{2}},
		{"gömme(tensör([[1, 1], [2, 2], [3, 3]]), [2, 0])", []float32{3, 3, 1, 1}, []int{2, 2}},
		{"gömme(tensör([[1, 1], [2, 2], [3, 3]]), 1)", []float32{2, 2}, []int{2}},
		{"katman_normu(tensör([[1, 2, 3]]), birler([3]), sıfırlar([3]))", []float32{-1.2247, 0, 1.2247}, []int{1, 3}},
		{"rms_normu(tensör([[3, 4]]), yok)", []float32{0.8485, 1.1314}, []int{1, 2}},
	}
	for _, d := range durumlar {
		if d.veri == nil {
			continue
		}
		v := deger(t, kod+"değişken cikti = "+d.ifade)
		veri, sh := tensorVeri(t, v)
		sekilEsit(t, d.ifade, sh, d.sekil)
		yakin(t, d.ifade, veri, d.veri, 1e-3)
	}

	// softmax satırları 1'e toplanır; log_softmax'ın üsteli softmax'tır.
	v := deger(t, kod+"değişken cikti = toplam(softmax(a, eksen: -1), eksen: 1)")
	veri, _ := tensorVeri(t, v)
	yakin(t, "softmax toplamı", veri, []float32{1, 1}, 1e-5)
	v = deger(t, kod+"değişken cikti = üstel(log_softmax(a)) - softmax(a)")
	veri, _ = tensorVeri(t, v)
	yakin(t, "log_softmax", veri, make([]float32, 6), 1e-5)

	// çapraz entropi = -log(softmax[hedef])
	v = deger(t, "değişken cikti = çapraz_entropi(tensör([[2.0, 1.0, 0.1]]), [0])")
	veri, _ = tensorVeri(t, v)
	yakin(t, "çapraz_entropi", veri, []float32{0.4170}, 1e-3)

	// Özellikler.
	out := mustCalistir(t, kod+`yazdır(a.şekil, a.cihaz, tip(a))
yazdır(a.değer)
yazdır(toplam(a).değer)
yazdır(rastgele_normal([2, 3], tohum: 7).şekil)`)
	want := "[2, 3] cpu Tensör\n[[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]]\n21.0\n[2, 3]\n"
	if out != want {
		t.Fatalf("özellik çıktısı:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestYazdirmaBicimi(t *testing.T) {
	out := mustCalistir(t, `yazdır(tensör([[1, 2], [3, 4]]))
yazdır(tensör([1.5, -2]))
yazdır(toplam(tensör([1, 2], türev_izle: doğru)))`)
	want := "Tensör[2×2] [\n  [1.0000, 2.0000],\n  [3.0000, 4.0000]\n]\n" +
		"Tensör[2] [ 1.5000, -2.0000]\n" +
		"Tensör(3.0000, türev izleniyor)\n"
	if out != want {
		t.Fatalf("yazdırma:\n%s\nbeklenen:\n%s", out, want)
	}
	// Büyük tensörlerde eksenler kısaltılır.
	out = mustCalistir(t, `yazdır(sıfırlar([100, 100]))`)
	if !strings.Contains(out, "…") || strings.Count(out, "\n") > 12 {
		t.Fatalf("büyük tensör kısaltılmadı:\n%s", out)
	}
}

func TestEskiDavranisKorunur(t *testing.T) {
	// Tek sayılık tensör eski otomatik türev değeridir.
	out := mustCalistir(t, `değişken x = tensör(3.0, türev_izle: doğru)
değişken y = x * x + 2 * x + 1
geri_yayılım(y)
yazdır(x.değer, y.değer, x.gradyan, tip(x))
yazdır(sıfırlar(2, 3).şekil, tip(birler(2, 2)), tip(sıfırlar(3)))
yazdır(toplam([1, 2, 3]), min([3, 1, 2]), maks(4, 9), karekök(9), ortalama([1, 2, 3]))
yazdır(softmax([1.0, 1.0]), relu(-2), tip(devrik([[1, 2]])))`)
	want := "3.0 16.0 8.0 Tensör\n[2, 3] Matris Matris\n6 1 9 3.0 2.0\n[0.5, 0.5] 0.0 Matris\n"
	if out != want {
		t.Fatalf("eski davranış:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestGeriYayilimGradyan(t *testing.T) {
	// k = ortalama((x @ w)^2), dk/dw = 2/N · xᵀ (x @ w)
	v := deger(t, `değişken w = tensör([[0.5], [-0.5]], türev_izle: doğru)
değişken x = tensör([[1, 2], [3, 4]])
değişken y = x @ w
geri_yayılım(ortalama(y * y))
değişken cikti = w.gradyan`)
	veri, sh := tensorVeri(t, v)
	sekilEsit(t, "gradyan", sh, []int{2, 1})
	// y = [-0.5, -0.5]; dk/dy = y; dk/dw = xᵀ y = [[-2], [-3]]
	yakin(t, "gradyan", veri, []float32{-2, -3}, 1e-5)

	// Gradyanlar birikir; iyileştiricinin .sıfırla() metodu sıfırlar.
	out := mustCalistir(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
geri_yayılım(toplam(w * 3))
geri_yayılım(toplam(w * 3))
yazdır(w.gradyan.değer)
değişken op = SGD([w], öğrenme_oranı: 0.1)
op.sıfırla()
yazdır(w.gradyan.değer)
geri_yayılım(toplam(w * w))
op.adım()
yazdır(w.değer)`)
	want := "[6.0, 6.0]\n[0.0, 0.0]\n[0.800000011920929, 1.600000023841858]\n"
	if out != want {
		t.Fatalf("birikim/SGD:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestTurevsiz(t *testing.T) {
	out := mustCalistir(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
değişken a = türevsiz { sonuç w * 2 }
değişken b = türevsiz(w * 2)
değişken c = w * 2
yazdır(a, b, c)
dene { geri_yayılım(toplam(a)) } yakala h { yazdır("hata") }`)
	want := "Tensör[2] [2.0000, 4.0000] Tensör[2] [2.0000, 4.0000] Tensör[2] (türev izleniyor) [2.0000, 4.0000]\nhata\n"
	if out != want {
		t.Fatalf("türevsiz:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestDogrusalRegresyonAdamW(t *testing.T) {
	// y = 2·x1 - 3·x2 + 0.5 verisinden ağırlıklar AdamW ile öğrenilir.
	out := mustCalistir(t, `değişken X = tensör([[0, 0], [1, 0], [0, 1], [1, 1], [2, 1], [1, 2]])
değişken Y = tensör([[0.5], [2.5], [-2.5], [-0.5], [1.5], [-3.5]])
değişken w = tensör([[0.0], [0.0]], türev_izle: doğru)
değişken b = tensör([0.0], türev_izle: doğru)
değişken op = AdamW([w, b], öğrenme_oranı: 0.05, ağırlık_çürümesi: 0)
döngü i içinde 0..1500 {
    op.sıfırla()
    değişken fark = X @ w + b - Y
    değişken kayıp = ortalama(fark * fark)
    geri_yayılım(kayıp)
    op.adım()
}
yazdır(yuvarla(w.değer[0][0], 2), yuvarla(w.değer[1][0], 2), yuvarla(b.değer[0], 2))`)
	if out != "2.0 -3.0 0.5\n" {
		t.Fatalf("regresyon öğrenilemedi: %q", out)
	}
}

func TestHatalar(t *testing.T) {
	durumlar := []struct{ kod, beklenen string }{
		{"sıfırlar([32, 1024]) @ sıfırlar([512, 10])", "[32, 1024] ile [512, 10] matris çarpımı yapılamaz; 1024 ≠ 512"},
		{"sıfırlar([2, 3, 4]) @ sıfırlar([5, 4, 6])", "toplu matris çarpımı yapılamaz; öndeki eksenler eşit olmalı ([2] ≠ [5])"},
		{"sıfırlar([2, 3]) + sıfırlar([4])", "[2, 3] ile [4] şekilleri toplama için yayınlanamaz; sondan 1. eksende 3 ≠ 4"},
		{"sıfırlar([2, 3]) * sıfırlar([3, 2])", "[2, 3] ile [3, 2] şekilleri çarpma için yayınlanamaz"},
		{"yazdır(toplam(sıfırlar([2, 3]), eksen: 2))", "Satır 1: toplam: eksen 2, [2, 3] şeklindeki 2 boyutlu tensör için geçersiz"},
		{"yazdır(softmax(sıfırlar([2]), eksen: -3))", "Satır 1: softmax: eksen -3"},
		{"yazdır(sıfırlar([2, 3]).cihaza(\"gpu\"))", "Satır 1: gpu:0 cihazı kullanılamıyor"},
		{"yazdır(sıfırlar([2, 3]).cihaza(\"tpu\"))", "bilinmeyen cihaz \"tpu\""},
		{"yazdır(sıfırlar([2, 3]).şekillendir([4, 2]))", "Satır 1: [2, 3] şeklindeki tensör (6 eleman) [4, 2] şekline sokulamaz"},
		{"yazdır(sıfırlar([2, 3]).dilim(1, 2, 5))", "dilim: 1. eksenin uzunluğu 3"},
		{"yazdır(sıfırlar([2, 3]).eksenleri_sırala([0, 0]))", "birden çok kez"},
		{"yazdır(sıfırlar([2, 3]).boyut)", "Tensör üzerinde \"boyut\" özelliği ya da metodu bulunamadı"},
		{"yazdır(tensör([[1, 2], [3]]))", "iç içe liste düzensiz"},
		{"yazdır(tensör([1, \"a\"]))", "tensör elemanları sayı olmalıdır"},
		{"geri_yayılım(sıfırlar([2], türev_izle: doğru) * 2)", "geri_yayılım tek elemanlı (skaler) bir kayıp ister, [2] şeklinde"},
		{"geri_yayılım(toplam(sıfırlar([2])))", "türev izlenen hiçbir tensöre bağlı değil"},
		{"yazdır(gömme(sıfırlar([3, 2]), [5]))", "gömme: indeks 5, [3, 2] şeklindeki tablonun dışında"},
		{"yazdır(çapraz_entropi(sıfırlar([2, 4]), [1]))", "[2, 4] şeklindeki çıktı için 2 hedef bekleniyor, 1 verildi"},
		{"yazdır(çapraz_entropi(sıfırlar([1, 4]), [4]))", "hedef sınıf 4, 4 sınıflı çıktının dışında"},
		{"yazdır(katman_normu(sıfırlar([2, 3]), birler([4])))", "katman_normu: ağırlık [3] şeklinde olmalı"},
		{"AdamW([sıfırlar([2])])", "AdamW: 1. parametre ([2]) türev izlemiyor"},
		{"AdamW([sıfırlar([2], türev_izle: doğru) * 2])", "yaprak değil"},
		{"SGD([tensör([1.0], türev_izle: doğru)], öğrenme_oran: 0.1)", "SGD 'öğrenme_oran' adında bir parametre almaz. Bunu mu demek istediniz: 'öğrenme_oranı'?"},
		{"sıfırlar([2, -1])", "şeklin 2. elemanı negatif olmayan bir tam sayı olmalıdır"},
	}
	for _, d := range durumlar {
		_, err := calistir(t, d.kod)
		if err == nil {
			t.Errorf("%s: hata bekleniyordu", d.kod)
			continue
		}
		if !strings.Contains(err.Error(), d.beklenen) {
			t.Errorf("%s:\n  hata  : %v\n  içermeli: %s", d.kod, err, d.beklenen)
		}
		if strings.Contains(err.Error(), "İç hata") || strings.Contains(err.Error(), "tensor:") {
			t.Errorf("%s: panik sızdı: %v", d.kod, err)
		}
	}
	// Hatalar dene/yakala ile yakalanabilir.
	out := mustCalistir(t, `dene { değişken k = sıfırlar([2, 3]) @ sıfırlar([2, 3]) } yakala h { yazdır("yakalandı") }
dene { yazdır(sıfırlar([1]).cihaza("gpu")) } yakala h { yazdır("yakalandı") }`)
	if out != "yakalandı\nyakalandı\n" {
		t.Fatalf("dene/yakala: %q", out)
	}
}

func TestPanikHataya(t *testing.T) {
	// Çekirdeğin panikleri sıradan hataya çevrilir.
	_, err := guvenli(func() int { panic("tensor: deneme") })
	if err == nil || err.Error() != "tensör işlemi başarısız: deneme" {
		t.Fatalf("panik çevirisi: %v", err)
	}
}

func TestIzlemeMasasi(t *testing.T) {
	v := deger(t, "değişken cikti = tensör([[1, 2, 3], [4, 5, 6]])")
	d := v.(*Deger)
	if d.IzlemeEtiketi() != "Tensör [2×3]" || d.IzlemeTuru() != "tensor" {
		t.Fatalf("etiket/tür: %q %q", d.IzlemeEtiketi(), d.IzlemeTuru())
	}
	cols, rows, err := d.TabloVerisi()
	if err != nil || len(cols) != 3 || len(rows) != 2 || rows[1][2] != "6" {
		t.Fatalf("tablo: %v %v %v", cols, rows, err)
	}
	v = deger(t, "değişken cikti = sıfırlar([2, 2, 2])")
	if _, _, err := v.(*Deger).TabloVerisi(); err == nil {
		t.Fatal("3 boyutlu tensör tablo olmamalı")
	}
}

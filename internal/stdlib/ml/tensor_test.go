package ml

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/checker"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
)

// Testlerdeki VM'ler tensör motorunu içersin.
func init() {
	interp.AddEngine(Register)
}

// run TR kodunu çalıştırır; çıktıyı ve hatayı döndürür.
func run(t *testing.T, code string) (string, error) {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	var out bytes.Buffer
	m.Out = &out
	_, err := m.Run(prog)
	m.WaitAsync()
	return out.String(), err
}

// mustRun kodu hatasız çalıştırır. Önce, tr komutunun yaptığı gibi tip denetiminden
// geçirir: tanımsız ya da kaldırılmış bir ad (ör. 'çık'ın eski adı) yalnızca zamanlamaya bağlı
// olarak çalışan bir satırda olsa da test her makinede kalır.
func mustRun(t *testing.T, code string) string {
	t.Helper()
	p := parser.New(lexer.New(code))
	if errs := checker.New().CheckProgram(p.ParseProgram()); len(errs) > 0 {
		t.Fatalf("tip denetimi hatası: %v\nkod:\n%s", errs, code)
	}
	out, err := run(t, code)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v\nkod:\n%s", err, code)
	}
	return out
}

// value kodu çalıştırır ve son satırdaki 'cikti' değişkeninin değerini döndürür.
func value(t *testing.T, code string) interp.Value {
	t.Helper()
	p := parser.New(lexer.New(code))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
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

func tensorData(t *testing.T, v interp.Value) ([]float32, []int) {
	t.Helper()
	d, ok := v.(*TensorValue)
	if !ok {
		t.Fatalf("tensör beklendi, %s (%s) geldi", v.Inspect(), v.Type())
	}
	return d.t.Data(), d.t.Shape()
}

func near(t *testing.T, ad string, got, want []float32, tol float64) {
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

func shapeEqual(t *testing.T, ad string, got, want []int) {
	t.Helper()
	if shapeText(got) != shapeText(want) {
		t.Fatalf("%s: şekil %v, %v bekleniyordu", ad, got, want)
	}
}

func TestForwardOps(t *testing.T) {
	code := `değişken a = tensör([[1, 2, 3], [4, 5, 6]])
değişken b = tensör([[1, 0], [0, 1], [1, 1]])
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
		{"a * tensör([[2], [3]])", []float32{2, 4, 6, 12, 15, 18}, []int{2, 3}},
		{"1 + a", []float32{2, 3, 4, 5, 6, 7}, []int{2, 3}},
		{"toplam(a)", []float32{21}, []int{}},
		{"toplam(a, eksen: 1)", []float32{6, 15}, []int{2}},
		{"ortalama(a, eksen: 0)", []float32{2.5, 3.5, 4.5}, []int{3}},
		{"maks(a)", []float32{6}, []int{}},
		{"min(a, eksen: -1)", []float32{1, 4}, []int{2}},
		{"maks(a, eksen: 0)", []float32{4, 5, 6}, []int{3}},
		{"relu(a - 3)", []float32{0, 0, 0, 1, 2, 3}, []int{2, 3}},
		{"karekök(tensör([4, 9]))", []float32{2, 3}, []int{2}},
		{"üstel(tensör([0]))", []float32{1}, []int{1}},
		{"log(tensör([1]))", []float32{0}, []int{1}},
		{"a.şekillendir([3, 2]).şekil.uzunluk", nil, nil},
		{"a.devrik()", []float32{1, 4, 2, 5, 3, 6}, []int{3, 2}},
		{"a.devrik()", []float32{1, 4, 2, 5, 3, 6}, []int{3, 2}},
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
	for _, d := range cases {
		if d.data == nil {
			continue
		}
		v := value(t, code+"değişken cikti = "+d.expr)
		data, sh := tensorData(t, v)
		shapeEqual(t, d.expr, sh, d.shape)
		near(t, d.expr, data, d.data, 1e-3)
	}

	// softmax satırları 1'e toplanır; log_softmax'ın üsteli softmax'tır.
	v := value(t, code+"değişken cikti = toplam(softmax(a, eksen: -1), eksen: 1)")
	data, _ := tensorData(t, v)
	near(t, "softmax toplamı", data, []float32{1, 1}, 1e-5)
	v = value(t, code+"değişken cikti = üstel(log_softmax(a)) - softmax(a)")
	data, _ = tensorData(t, v)
	near(t, "log_softmax", data, make([]float32, 6), 1e-5)

	// çapraz entropi = -log(softmax[hedef])
	v = value(t, "değişken cikti = çapraz_entropi(tensör([[2.0, 1.0, 0.1]]), [0])")
	data, _ = tensorData(t, v)
	near(t, "çapraz_entropi", data, []float32{0.4170}, 1e-3)

	// Özellikler.
	out := mustRun(t, code+`yazdır(a.şekil, a.cihaz, tip(a))
yazdır(a.değer)
yazdır(toplam(a).değer)
yazdır(rastgele_normal([2, 3], tohum: 7).şekil)`)
	want := "[2, 3] cpu Tensör\n[[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]]\n21.0\n[2, 3]\n"
	if out != want {
		t.Fatalf("özellik çıktısı:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestPrintFormat(t *testing.T) {
	out := mustRun(t, `yazdır(tensör([[1, 2], [3, 4]]))
yazdır(tensör([1.5, -2]))
yazdır(toplam(tensör([1, 2], türev_izle: doğru)))`)
	want := "Tensör[2×2] [\n  [1.0000, 2.0000],\n  [3.0000, 4.0000]\n]\n" +
		"Tensör[2] [ 1.5000, -2.0000]\n" +
		"Tensör(3.0000, türev izleniyor)\n"
	if out != want {
		t.Fatalf("yazdırma:\n%s\nbeklenen:\n%s", out, want)
	}
	// Büyük tensörlerde eksenler kısaltılır.
	out = mustRun(t, `yazdır(sıfırlar([100, 100]))`)
	if !strings.Contains(out, "…") || strings.Count(out, "\n") > 12 {
		t.Fatalf("büyük tensör kısaltılmadı:\n%s", out)
	}
}

func TestOldBehaviourKept(t *testing.T) {
	// Tek sayıdan tensör 0 boyutludur; eski tek sayılık otomatik türev kullanımı aynı
	// sonuçları verir (gradyan da 0 boyutlu tensördür). Matris adlı oluşturucular 2 boyutlu
	// tensör verir; eleman bazlı fonksiyonlar sayıda Ondalık verir.
	out := mustRun(t, `değişken x = tensör(3.0, türev_izle: doğru)
değişken y = x * x + 2 * x + 1
geri_yayılım(y)
yazdır(x.değer, y.değer, x.gradyan, tip(x))
yazdır(sıfırlar([2, 3]).şekil, tip(birler([2, 2])), tip(sıfırlar([3])))
yazdır(toplam([1, 2, 3]), min([3, 1, 2]), maks(4, 9), karekök(9), ortalama([1, 2, 3]))
yazdır(softmax(tensör([1.0, 1.0])).değer, relu(-2), tip(tensör([[1, 2]]).devrik()))`)
	want := "3.0 16.0 Tensör(8.0000) Tensör\n[2, 3] Tensör Tensör\n6 1 9 3.0 2.0\n[0.5, 0.5] 0.0 Tensör\n"
	if out != want {
		t.Fatalf("eski davranış:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestBackwardGradient(t *testing.T) {
	// k = ortalama((x @ w)^2), dk/dw = 2/N · xᵀ (x @ w)
	v := value(t, `değişken w = tensör([[0.5], [-0.5]], türev_izle: doğru)
değişken x = tensör([[1, 2], [3, 4]])
değişken y = x @ w
geri_yayılım(ortalama(y * y))
değişken cikti = w.gradyan`)
	data, sh := tensorData(t, v)
	shapeEqual(t, "gradyan", sh, []int{2, 1})
	// y = [-0.5, -0.5]; dk/dy = y; dk/dw = xᵀ y = [[-2], [-3]]
	near(t, "gradyan", data, []float32{-2, -3}, 1e-5)

	// Gradyanlar birikir; iyileştiricinin .sıfırla() metodu sıfırlar.
	out := mustRun(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
geri_yayılım(toplam(w * 3))
geri_yayılım(toplam(w * 3))
yazdır(w.gradyan.değer)
değişken op = SGD([w], öğrenme_oranı: 0.1)
op.sıfırla()
yazdır(w.gradyan.değer)
geri_yayılım(toplam(w * w))
op.adım()
yazdır(w.değer)`)
	want := "[6.0, 6.0]\n[0.0, 0.0]\n[0.8, 1.6]\n"
	if out != want {
		t.Fatalf("birikim/SGD:\n%q\nbeklenen:\n%q", out, want)
	}
}

func TestNoGrad(t *testing.T) {
	out := mustRun(t, `değişken w = tensör([1.0, 2.0], türev_izle: doğru)
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

func TestLinearRegressionAdamW(t *testing.T) {
	// y = 2·x1 - 3·x2 + 0.5 verisinden ağırlıklar AdamW ile öğrenilir.
	out := mustRun(t, `değişken X = tensör([[0, 0], [1, 0], [0, 1], [1, 1], [2, 1], [1, 2]])
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

func TestErrors(t *testing.T) {
	cases := []struct {
		code string
		want messages.Code
	}{
		{"sıfırlar([32, 1024]) @ sıfırlar([512, 10])", messages.H6126},
		{"sıfırlar([2, 3, 4]) @ sıfırlar([5, 4, 6])", messages.H6115},
		{"sıfırlar([2, 3]) + sıfırlar([4])", messages.H6100},
		{"sıfırlar([2, 3]) * sıfırlar([3, 2])", messages.H6100},
		{"yazdır(sıfırlar([2, 3]).cihaza(\"tpu\"))", messages.H6800},
		{"yazdır(sıfırlar([2, 3]).dilim(1, 2, 5))", messages.H6720},
		{"yazdır(sıfırlar([2, 3]).eksenleri_sırala([0, 0]))", messages.H6715},
		{"yazdır(sıfırlar([2, 3]).boyut)", messages.H7903},
		{"yazdır(tensör([[1, 2], [3]]))", messages.H6027},
		{"yazdır(tensör([1, \"a\"]))", messages.H6007},
		{"geri_yayılım(sıfırlar([2], türev_izle: doğru) * 2)", messages.H6401},
		{"geri_yayılım(toplam(sıfırlar([2])))", messages.H6402},
		{"yazdır(gömme(sıfırlar([3, 2]), [5]))", messages.H6305},
		{"yazdır(çapraz_entropi(sıfırlar([2, 4]), [1]))", messages.H6313},
		{"yazdır(çapraz_entropi(sıfırlar([1, 4]), [4]))", messages.H6314},
		{"yazdır(katman_normu(sıfırlar([2, 3]), birler([4])))", messages.H6319},
		{"AdamW([sıfırlar([2])])", messages.H6513},
		{"AdamW([sıfırlar([2], türev_izle: doğru) * 2])", messages.H6514},
		{"SGD([tensör([1.0], türev_izle: doğru)], öğrenme_oran: 0.1)", messages.H7900},
		{"sıfırlar([2, -1])", messages.H6024},
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
		{"yazdır(toplam(sıfırlar([2, 3]), eksen: 2))", "Satır 1: toplam: eksen 2, [2, 3] şeklindeki 2 boyutlu tensör için geçersiz"},
		{"yazdır(softmax(sıfırlar([2]), eksen: -3))", "Satır 1: softmax: eksen -3"},
		{"yazdır(sıfırlar([2, 3]).cihaza(\"gpu:9\"))", "Satır 1: gpu:9 cihazı kullanılamıyor"},
		{"yazdır(sıfırlar([2, 3]).şekillendir([4, 2]))", "Satır 1: [2, 3] şeklindeki tensör (6 eleman) [4, 2] şekline sokulamaz"},
	} {
		if _, err := run(t, d.code); err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%s: %v (içermeli: %s)", d.code, err, d.expected)
		}
	}
	// Hatalar dene/yakala ile yakalanabilir.
	out := mustRun(t, `dene { değişken k = sıfırlar([2, 3]) @ sıfırlar([2, 3]) } yakala h { yazdır("yakalandı") }
dene { yazdır(sıfırlar([1]).cihaza("gpu:9")) } yakala h { yazdır("yakalandı") }`)
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
	v := value(t, "değişken cikti = tensör([[1, 2, 3], [4, 5, 6]])")
	d := v.(*TensorValue)
	if d.WatchLabel() != "Tensör [2×3]" || d.WatchKind() != "tensor" {
		t.Fatalf("etiket/tür: %q %q", d.WatchLabel(), d.WatchKind())
	}
	cols, rows, err := d.TableData()
	if err != nil || len(cols) != 3 || len(rows) != 2 || rows[1][2] != "6" {
		t.Fatalf("tablo: %v %v %v", cols, rows, err)
	}
	v = value(t, "değişken cikti = sıfırlar([2, 2, 2])")
	if _, _, err := v.(*TensorValue).TableData(); err == nil {
		t.Fatal("3 boyutlu tensör tablo olmamalı")
	}
}

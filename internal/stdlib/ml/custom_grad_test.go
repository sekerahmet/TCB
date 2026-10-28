package ml

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/lang/source"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// özel_türev ve türev_denetle'nin Go tarafı testleri; dil düzeyindeki kurallar
// tests/ozel_turev_test.tr'dedir.

func TestCustomGradSquare(t *testing.T) {
	out := mustRun(t, `TR_var kare = TR_custom_grad(
    TR_forward: TR_func(x) { TR_return [x * x, [x]] },
    TR_backward: TR_func(saklananlar, g) { TR_return [g * 2 * saklananlar[0]] }
)
TR_var x = TR_tensor([3.0, -1.0], TR_requires_grad: TR_true)
TR_backward(TR_sum(kare(x)))
TR_print(x.TR_grad.TR_tolist)
TR_print(TR_grad_check(kare, [TR_tensor([[0.3, -1.2]], TR_requires_grad: TR_true)]))`)
	if want := "[6.0, -2.0]\ndoğru\n"; out != want {
		t.Fatalf("çıktı %q, beklenen %q", out, want)
	}
}

// geri, geri_yayılım'ı çağıran görevin VM'iyle çalışır: ileri ana görevde blok dışında,
// geri yayılım ise karışık_hassasiyet("bf16") bloğu kurmuş bir eşzamanlı görevde yapılınca
// geri'nin içindeki '@' bloğun türüyle (bf16) hesaplanır. Tersi de doğrudur.
func TestCustomGradBackwardRunsInCallerTask(t *testing.T) {
	out := mustRun(t, `TR_var görülen = []
TR_var f = TR_custom_grad(
    TR_forward: TR_func(x) { TR_return [x * 1, []] },
    TR_backward: TR_func(k, g) {
        görülen.TR_append((TR_tensor([[1.001]]) @ TR_tensor([[1.0]])).TR_tolist[0][0])
        TR_return [g]
    }
)
TR_var y = f(TR_tensor([1.0], TR_requires_grad: TR_true))
TR_async { TR_mixed_precision("bf16") { TR_backward(TR_sum(y)) } }
TR_wait_all()
TR_var z = TR_mixed_precision("bf16") { f(TR_tensor([1.0], TR_requires_grad: TR_true)) }
TR_async { TR_backward(TR_sum(z)) }
TR_wait_all()
TR_print(görülen)`)
	if want := "[1.0, 1.001]\n"; out != want {
		t.Fatalf("çıktı %q, beklenen %q", out, want)
	}
}

func TestCustomGradErrorMessages(t *testing.T) {
	prefixVal := `TR_var x = TR_tensor([[1.0, 2.0]], TR_requires_grad: TR_true)
`
	cases := []struct {
		code string
		want messages.Code
	}{
		{`TR_custom_grad(TR_forward: TR_func(x) { TR_return x }, TR_backward: TR_func(k, g) { TR_return [g] })(x)`,
			messages.H6409},
		{`TR_custom_grad(TR_forward: TR_func(x) { TR_return [[x, 3], []] }, TR_backward: TR_func(k, g) { TR_return [g] })(x)`,
			messages.H6412},
		{`TR_backward(TR_sum(TR_custom_grad(TR_forward: TR_func(x) { TR_return [x, []] }, TR_backward: TR_func(k, g) { TR_return g })(x)))`,
			messages.H6414},
		{`TR_backward(TR_sum(TR_custom_grad(TR_forward: TR_func(x) { TR_return [x, []] }, TR_backward: TR_func(k, g) { TR_return [g.TR_reshape([2, 1])] })(x)))`,
			messages.H6418},
		{`TR_backward(TR_sum(TR_custom_grad(TR_forward: TR_func(x) { TR_return [x, []] }, TR_backward: TR_func(k, g) { TR_return [3] })(x)))`,
			messages.H6416},
		{`TR_custom_grad(TR_func(x) { TR_return [x, []] })`, messages.H6405},
		{`TR_custom_grad()`, messages.H6431},
		{`TR_grad_check(x, [x])`, messages.H6420},
		{`TR_grad_check((a) => a, x)`, messages.H6421},
		{`TR_grad_check((a) => 3, [x])`, messages.H6428},
		{`TR_grad_check((a) => a, [x], TR_tol: -1)`, messages.H6429},
		{`TR_no_grad { TR_grad_check((a) => a, [x]) }`, messages.H6422},
	}
	for _, d := range cases {
		_, err := run(t, prefixVal+d.code)
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.code, err, d.want)
		}
	}
}

// Kullanıcının geri hesabındaki hata satır bilgisiyle ve dene/yakala ile yakalanabilir
// olarak gelir; geri yayılım yarıda kalsa da program çökmez.
func TestCustomGradBackwardError(t *testing.T) {
	_, err := run(t, `TR_var f = TR_custom_grad(TR_forward: TR_func(x) { TR_return [x, []] }, TR_backward: TR_func(k, g) {
    TR_return [g @ g]
})
TR_backward(TR_sum(f(TR_tensor([[1.0, 2.0]], TR_requires_grad: TR_true))))`)
	if err == nil || !strings.Contains(err.Error(), "Satır 2:") || !messages.Has(err, messages.H6126) {
		t.Fatalf("geri'deki hata satırıyla gelmeli: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Satıra özel kapılı nöron: özel türev ile bileşik sürümün karşılaştırması
// ----------------------------------------------------------------------------

// gatedVersions ölçülen sürümlerin TR fonksiyon adlarıdır (tests/ozel_turev_test.tr).
var gatedVersions = []struct{ ad, fn string }{
	{"bileşik", "kapılı_bileşik"},
	{"özel", "kapılı_nöron"},         // gömme + tek düğümlü kapı çekirdeği
	{"özel_tam", "kapılı_nöron_tam"}, // tablo gradyanı da elle (tek-sıcak çarpım)
}

// gatedDefs üç sürümün tanımlarını tests/ozel_turev_test.tr'den (işaretler arası)
// okur; böylece ölçülen kod dil testinde doğrulanan kodla aynıdır.
func gatedDefs(tb testing.TB) string {
	tb.Helper()
	data, err := os.ReadFile(reporoot.Path("tests/ozel_turev_test.tr"))
	if err != nil {
		tb.Fatalf("test dosyası okunamadı: %v", err)
	}
	s, err := source.Show(string(data), source.Turkish()) // test dosyası kodlu saklanır: Türkçe görünümü
	if err != nil {
		tb.Fatalf("test dosyası gösterilemedi: %v", err)
	}
	head := strings.Index(s, "// --- kapılı nöron tanımları başı ---")
	last := strings.Index(s, "// --- kapılı nöron tanımları sonu ---")
	if head < 0 || last < head {
		tb.Fatal("kapılı nöron tanımlarının işaretleri bulunamadı")
	}
	return s[head:last]
}

// gatedVM tanımları ve N×d girdiyi, R×3d tabloyu kurar; her sürüm için kayıp_<fn>
// yalnızca ileri hesabı, adım_<fn> ileri+geri adımını yapar. Her sürümün kendi x ve W
// kopyası (x_<fn>, W_<fn>) vardır.
func gatedVM(tb testing.TB, N, d, R int) *interp.VM {
	tb.Helper()
	var sb strings.Builder
	sb.WriteString(gatedDefs(tb))
	fmt.Fprintf(&sb, `
değişken x0 = rastgele_normal([%[1]d, %[2]d], tohum: 1)
değişken W0 = rastgele_normal([%[3]d, %[4]d], tohum: 2) * 0.1
değişken indeks = tensör([(i * 7) %% %[3]d döngü i içinde 0..%[1]d])
değişken ağırlık = rastgele_normal([%[1]d, %[2]d], tohum: 3)
`, N, d, R, 3*d)
	for _, s := range gatedVersions {
		fmt.Fprintf(&sb, `değişken x_%[1]s = tensör(x0, türev_izle: doğru)
değişken W_%[1]s = tensör(W0, türev_izle: doğru)
fonksiyon kayıp_%[1]s() { sonuç toplam(%[1]s(x_%[1]s, W_%[1]s, indeks) * ağırlık) }
fonksiyon adım_%[1]s() { geri_yayılım(kayıp_%[1]s()) }
`, s.fn)
	}
	p := parser.New(source.TextLexer(sb.String()))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		tb.Fatalf("sözdizimi hatası: %v", p.Errors())
	}
	m := interp.New()
	m.Out = &bytes.Buffer{}
	if _, err := m.Run(prog); err != nil {
		tb.Fatalf("beklenmeyen hata: %v", err)
	}
	return m
}

func gatedCall(tb testing.TB, m *interp.VM, ad string) interp.Value {
	tb.Helper()
	fn, ok := m.GlobalEnv.Get(ad)
	if !ok {
		tb.Fatalf("%s tanımlı değil", ad)
	}
	v, err := m.ApplyFunction(fn, nil, nil, 0)
	if err != nil {
		tb.Fatalf("%s: %v", ad, err)
	}
	return v
}

func gatedTensor(tb testing.TB, m *interp.VM, ad string) *tensor.Tensor {
	tb.Helper()
	v, _ := m.GlobalEnv.Get(ad)
	d, ok := v.(*TensorValue)
	if !ok {
		tb.Fatalf("%s tensör değil", ad)
	}
	return d.t
}

// Özel türevli sürümler aynı kaybı ve gradyanı verir; çizgeleri bileşik sürümünkinden
// küçüktür. Ölçümler -v ile yazdırılır; hız için BenchmarkGatedNeuron.
func TestGatedNeuronMeasure(t *testing.T) {
	const N, d, R = 256, 64, 128
	m := gatedVM(t, N, d, R)
	losses := make([]*tensor.Tensor, len(gatedVersions))
	node := make([]int, len(gatedVersions))
	nBytes := make([]int64, len(gatedVersions))
	for i, s := range gatedVersions {
		losses[i] = gatedCall(t, m, "kayıp_"+s.fn).(*TensorValue).t
		node[i], nBytes[i] = tensor.GraphStats(losses[i])
		t.Logf("%-9s çizge düğümü %2d, ara tensör %7d bayt", s.ad, node[i], nBytes[i])
	}
	for i := 1; i < len(gatedVersions); i++ {
		if node[i] >= node[0] || nBytes[i] >= nBytes[0] {
			t.Errorf("%s sürümü bileşik sürümden küçük bir çizge kurmalı: %d düğüm/%d bayt, bileşik %d/%d",
				gatedVersions[i].ad, node[i], nBytes[i], node[0], nBytes[0])
		}
	}
	if node[2] != 3 { // tek özel düğüm + çarpım + toplam
		t.Errorf("tamamen elle yazılan sürümde %d düğüm, 3 bekleniyordu", node[2])
	}
	for i, s := range gatedVersions {
		near(t, s.ad+" kaybı", losses[i].Data(), losses[0].Data(), 1e-2)
		if err := backprop(m, losses[i], nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"x_", "W_"} {
		ref := gatedTensor(t, m, p+gatedVersions[0].fn).Grad().Data()
		for _, s := range gatedVersions[1:] {
			near(t, s.ad+" "+p+" gradyanı", gatedTensor(t, m, p+s.fn).Grad().Data(), ref, 1e-3)
		}
	}
	// Kaba süre (ayrıntılı ölçüm: go test -bench KapiliNoron ./internal/stdlib/ml).
	for _, s := range gatedVersions {
		const repeat = 5
		head := time.Now()
		for i := 0; i < repeat; i++ {
			gatedCall(t, m, "adım_"+s.fn)
		}
		t.Logf("%-9s ileri+geri adımı %v", s.ad, time.Since(head)/repeat)
	}
}

func BenchmarkGatedNeuron(b *testing.B) {
	for _, size := range []struct{ N, d, R int }{{256, 64, 128}, {2048, 128, 512}} {
		m := gatedVM(b, size.N, size.d, size.R)
		for _, s := range gatedVersions {
			b.Run(fmt.Sprintf("%s/N%d_d%d_R%d", s.ad, size.N, size.d, size.R), func(b *testing.B) {
				node, nBytes := tensor.GraphStats(gatedCall(b, m, "kayıp_"+s.fn).(*TensorValue).t)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					gatedCall(b, m, "adım_"+s.fn)
				}
				b.ReportMetric(float64(node), "düğüm")
				b.ReportMetric(float64(nBytes), "ara_bayt")
			})
		}
	}
}

type auditSecondDevice struct {
	tensor.Backend
	released int
}

func (d *auditSecondDevice) Device() tensor.Device { return tensor.GPU(99) }

func (d *auditSecondDevice) Release(s tensor.Storage) {
	d.released++
	if r, ok := d.Backend.(tensor.Releaser); ok {
		r.Release(s)
	}
}

// Kural: türev_denetle girdi kopyalarını ve yön vektörlerini tensörlerin kendi cihazında kurar.
// Neden: varsayılan cihazda kurulursa başka cihazdaki tensörlerle işlem cihaz uyuşmazlığı verir.
func TestCheckGradBuildsCopiesOnTheInputDevice(t *testing.T) {
	tensor.RegisterBackend(&auditSecondDevice{Backend: tensor.BackendFor(tensor.CPU)})
	out, err := run(t, `TR_var w = TR_tensor([2.0, 3.0]).TR_to_device("gpu:99")
TR_var x = TR_tensor([1.0, 4.0], TR_requires_grad: TR_true).TR_to_device("gpu:99")
TR_print(TR_grad_check((a) => a * w, [x]))`)
	if err != nil || out != "true\n" {
		t.Fatalf("varsayılan dışı cihazda türev_denetle başarısız: err=%v out=%q", err, out)
	}
}

// Kural: adlı argümanlı tensör metodu çağrısı (x.böl(2, eksen: 0)) alıcıyı kaçırmaz; yerel tensör
// kapsam sonunda konumlu çağrıdaki gibi bırakılır (15.43).
func TestNamedArgumentMethodCallReleasesReceiverAtScopeExit(t *testing.T) {
	m := interp.New()
	m.Out = &bytes.Buffer{}
	var captured *tensor.Tensor
	m.GlobalEnv.Define("izle", &interp.BuiltinVal{
		Name: "izle",
		Pure: true,
		Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			captured = args[0].(*TensorValue).t
			return interp.Null, nil
		},
	}, true)
	p := parser.New(source.TextLexer(`
TR_func sına() {
    TR_var x = TR_zeros([4, 4])
    izle(x)
    TR_var parça = x.TR_split(2, TR_dim: 0)
}
sına()
`))
	if _, err := m.Run(p.ParseProgram()); err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if captured == nil || !captured.IsDropped() {
		t.Fatal("adlı argümanla metot çağrılan yerel tensör kapsam sonunda bırakılmadı")
	}
}

// Kural: var olan başlığı aynen döndüren işlemler (aynı cihaza cihaza, tek tensörlü toplam)
// başlığı ikinci kez sarmaz; yerel tensör kapsam sonunda bırakılır (15.43). Neden: ikinci
// sarıcı sahipliği alamaz ve başlık hiç düşmez.
func TestRewrappingBuiltinsKeepSingleOwner(t *testing.T) {
	for _, expr := range []string{`x.cihaza("cpu")`, `TR_sum([x])`} {
		m := interp.New()
		m.Out = &bytes.Buffer{}
		var captured *tensor.Tensor
		m.GlobalEnv.Define("izle", &interp.BuiltinVal{
			Name: "izle",
			Pure: true,
			Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
				captured = args[0].(*TensorValue).t
				return interp.Null, nil
			},
		}, true)
		code := fmt.Sprintf("TR_func sına() {\n    TR_var x = TR_zeros([4, 4])\n    izle(x)\n    TR_var y = %s\n    TR_print(1)\n}\nsına()\n", expr)
		p := parser.New(source.TextLexer(code))
		if _, err := m.Run(p.ParseProgram()); err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if captured == nil || !captured.IsDropped() {
			t.Errorf("%s: aynı başlığı yeniden saran işlem yerel tensörün bırakılmasını engelledi", expr)
		}
	}
}

// Kural: x[i] ve max(x) ara görünüm başlıklarını bırakır; yerel tensör bırakılınca depolaması
// arka uca iade edilir (15.43). Not: sondaki 'değişken' deyimi dönüş değeridir, bu yüzden
// fonksiyonun son satırı başka bir deyimdir.
func TestIndexAndWholeReductionsReturnStorageAtScopeExit(t *testing.T) {
	for _, stmt := range []string{`TR_var y = x[0]`, `TR_var y = TR_max(x)`} {
		dev := &auditSecondDevice{Backend: tensor.BackendFor(tensor.CPU)}
		tensor.RegisterBackend(dev)
		tensor.SetDefaultDevice(tensor.GPU(99))
		code := fmt.Sprintf("TR_func sına() {\n    TR_var x = TR_zeros([4, 4])\n    %s\n    TR_print(1)\n}\nsına()\n", stmt) // sondaki 'değişken' deyimi dönüş değeridir: son satır başka olmalı
		_, err := run(t, code)
		tensor.SetDefaultDevice(tensor.CPU)
		if err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
		if dev.released == 0 {
			t.Errorf("%s: kapsam bittikten sonra x'in depolaması arka uca iade edilmedi", stmt)
		}
	}
}

// Kural: ileti, verilen değeri dilin ortak "Tip (özet)" biçiminde anar; uzun değerler özetlenir.
func TestCustomGradMessagesUseTypeAndSummary(t *testing.T) {
	_, err := run(t, `TR_var x = TR_tensor([[1.0, 2.0]], TR_requires_grad: TR_true)
TR_grad_check((a) => 3, [x])`)
	if err == nil || !strings.Contains(err.Error(), "Tam (3)") {
		t.Errorf("verilen değer \"Tam (3)\" biçiminde anılmalı: %v", err)
	}
}

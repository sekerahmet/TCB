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
	"github.com/sekerahmet/TCB/internal/lang/lexer"
	"github.com/sekerahmet/TCB/internal/lang/parser"
	"github.com/sekerahmet/TCB/internal/reporoot"
)

// özel_türev ve türev_denetle'nin Go tarafı testleri; dil düzeyindeki kurallar
// tests/ozel_turev_test.tr'dedir.

func TestCustomGradSquare(t *testing.T) {
	out := mustRun(t, `değişken kare = özel_türev(
    ileri: fonksiyon(x) { sonuç [x * x, [x]] },
    geri: fonksiyon(saklananlar, g) { sonuç [g * 2 * saklananlar[0]] }
)
değişken x = tensör([3.0, -1.0], türev_izle: doğru)
geri_yayılım(toplam(kare(x)))
yazdır(x.gradyan.değer)
yazdır(türev_denetle(kare, [tensör([[0.3, -1.2]], türev_izle: doğru)]))`)
	if want := "[6.0, -2.0]\ndoğru\n"; out != want {
		t.Fatalf("çıktı %q, beklenen %q", out, want)
	}
}

// geri, geri_yayılım'ı çağıran görevin VM'iyle çalışır: ileri ana görevde blok dışında,
// geri yayılım ise karışık_hassasiyet("bf16") bloğu kurmuş bir eşzamanlı görevde yapılınca
// geri'nin içindeki '@' bloğun türüyle (bf16) hesaplanır. Tersi de doğrudur.
func TestCustomGradBackwardRunsInCallerTask(t *testing.T) {
	out := mustRun(t, `değişken görülen = []
değişken f = özel_türev(
    ileri: fonksiyon(x) { sonuç [x * 1, []] },
    geri: fonksiyon(k, g) {
        görülen.ekle((tensör([[1.001]]) @ tensör([[1.0]])).değer[0][0])
        sonuç [g]
    }
)
değişken y = f(tensör([1.0], türev_izle: doğru))
eşzamanlı { karışık_hassasiyet("bf16") { geri_yayılım(toplam(y)) } }
bekle_hepsi()
değişken z = karışık_hassasiyet("bf16") { f(tensör([1.0], türev_izle: doğru)) }
eşzamanlı { geri_yayılım(toplam(z)) }
bekle_hepsi()
yazdır(görülen)`)
	if want := "[1.0, 1.001]\n"; out != want {
		t.Fatalf("çıktı %q, beklenen %q", out, want)
	}
}

func TestCustomGradErrorMessages(t *testing.T) {
	prefixVal := `değişken x = tensör([[1.0, 2.0]], türev_izle: doğru)
`
	cases := []struct{ code, expected string }{
		{`özel_türev(ileri: fonksiyon(x) { sonuç x }, geri: fonksiyon(k, g) { sonuç [g] })(x)`,
			"özel_türev: ileri [çıktı, saklananlar] döndürmeli (ör. sonuç [x * x, [x]]"},
		{`özel_türev(ileri: fonksiyon(x) { sonuç [[x, 3], []] }, geri: fonksiyon(k, g) { sonuç [g] })(x)`,
			"ileri'nin çıktı listesinin 2. öğesi tensör olmalıdır, Tam verildi"},
		{`geri_yayılım(toplam(özel_türev(ileri: fonksiyon(x) { sonuç [x, []] }, geri: fonksiyon(k, g) { sonuç g })(x)))`,
			"geri her girdi için bir gradyan (ya da yok) içeren bir liste döndürmeli"},
		{`geri_yayılım(toplam(özel_türev(ileri: fonksiyon(x) { sonuç [x, []] }, geri: fonksiyon(k, g) { sonuç [g.şekillendir([2, 1])] })(x)))`,
			"geri 1. girdi için [2, 1] şeklinde gradyan döndürdü, beklenen [1, 2] (girdinin şekli); gradyan girdiyle aynı şekilde olmalı, yayınlama yapılmaz"},
		{`geri_yayılım(toplam(özel_türev(ileri: fonksiyon(x) { sonuç [x, []] }, geri: fonksiyon(k, g) { sonuç [3] })(x)))`,
			"geri'nin 1. öğesi (1. girdinin gradyanı) tensör ya da yok olmalıdır, Tam verildi"},
		{`özel_türev(fonksiyon(x) { sonuç [x, []] })`, "özel_türev yalnızca adlı argüman alır (ileri: ve geri:)"},
		{`özel_türev()`, "özel_türev: ileri: ve geri: eksik"},
		{`türev_denetle(x, [x])`, "türev_denetle: ilk parametre bir fonksiyon olmalıdır, Tensör verildi"},
		{`türev_denetle((a) => a, x)`, "türev_denetle: girdiler bir liste olmalıdır"},
		{`türev_denetle((a) => 3, [x])`, "türev_denetle: f bir tensör ya da tensör listesi döndürmeli, 3 (Tam) verildi"},
		{`türev_denetle((a) => a, [x], tolerans: -1)`, "türev_denetle: tolerans pozitif bir sayı olmalıdır"},
		{`türevsiz { türev_denetle((a) => a, [x]) }`, "türev_denetle türevsiz { ... } bloğu içinde çalışamaz"},
	}
	for _, d := range cases {
		_, err := run(t, prefixVal+d.code)
		if err == nil || !strings.Contains(err.Error(), d.expected) {
			t.Errorf("%s:\n  hata    : %v\n  içermeli: %s", d.code, err, d.expected)
		}
	}
}

// Kullanıcının geri hesabındaki hata satır bilgisiyle ve dene/yakala ile yakalanabilir
// olarak gelir; geri yayılım yarıda kalsa da program çökmez.
func TestCustomGradBackwardError(t *testing.T) {
	_, err := run(t, `değişken f = özel_türev(ileri: fonksiyon(x) { sonuç [x, []] }, geri: fonksiyon(k, g) {
    sonuç [g @ g]
})
geri_yayılım(toplam(f(tensör([[1.0, 2.0]], türev_izle: doğru))))`)
	if err == nil || !strings.Contains(err.Error(), "Satır 2:") || !strings.Contains(err.Error(), "matris çarpımı yapılamaz") {
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
	s := string(data)
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
	p := parser.New(lexer.New(sb.String()))
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

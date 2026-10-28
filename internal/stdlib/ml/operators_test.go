package ml

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// Kural (dil tanımı 15.49): iki tarafın da genişlediği yayınlama programın hata akışına bir
// kez uyarı yazar (işlem ve şekil çifti başına); tek taraflı yayınlamada uyarı yoktur.
func TestBroadcastWarning(t *testing.T) {
	o := trrun.MustExecChecked(t, `TR_var a = TR_zeros([4, 1])
TR_var b = TR_zeros([4])
TR_for i TR_in 0..3 { TR_var c = a + b }
TR_print((a - b).TR_shape)
TR_var d = TR_zeros([2, 3]) + TR_zeros([3])
TR_var e = TR_zeros([5, 3]) * TR_zeros([1, 3])`)
	if o.Out != "[4, 4]\n" {
		t.Fatalf("çıktı: %q", o.Out)
	}
	want := "Uyarı: [4, 1] ile [4] yayınlanarak [4, 4] üretildi (iki taraf da genişletildi); bilerek yapmadıysanız şekilleri kontrol edin.\n"
	// Toplama için döngüde bir kez, çıkarma için bir kez; tek taraflı yayınlamada uyarı yok.
	if strings.Count(o.ErrOut, want) != 2 || strings.Count(o.ErrOut, "Uyarı") != 2 {
		t.Fatalf("uyarılar:\n%s", o.ErrOut)
	}
}

// Kural: gösterilen uyarılar programa bağlıdır. Aynı süreçte art arda çalışan iki program (tr
// test'in dosyaları, düzenleyici oturumu) aynı uyarıyı ayrı ayrı gösterir; süreç geneli bir
// durumda ikincisi susardı.
func TestBroadcastWarningPerProgram(t *testing.T) {
	const code = `TR_var c = TR_zeros([4, 1]) + TR_zeros([4])`
	for i := 0; i < 2; i++ {
		if n := strings.Count(trrun.MustExecChecked(t, code).ErrOut, "Uyarı"); n != 1 {
			t.Fatalf("%d. program %d uyarı yazdı, 1 bekleniyordu", i+1, n)
		}
	}
}

func TestBothSidesWiden(t *testing.T) {
	cases := []struct {
		a, b []int
		want bool
	}{
		{[]int{4, 1}, []int{4}, true},
		{[]int{2, 1}, []int{1, 3}, true},
		{[]int{2, 3}, []int{3}, false},
		{[]int{2, 3}, []int{2, 3}, false},
		{[]int{2, 3}, []int{}, false},
		{[]int{1}, []int{5}, false},
		{[]int{3}, []int{1, 3}, false},
		{[]int{8, 1, 4}, []int{8, 4}, true},
	}
	for _, d := range cases {
		if got := bothWiden(tensor.Zeros(d.a...), tensor.Zeros(d.b...)); got != d.want {
			t.Errorf("%v ile %v: %v, %v bekleniyordu", d.a, d.b, got, d.want)
		}
	}
}

func BenchmarkBinaryAdd(b *testing.B) {
	x := New(tensor.Zeros(64, 64))
	y := New(tensor.Zeros(64))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := x.binary(nil, "+", y); err != nil {
			b.Fatal(err)
		}
	}
}

// Bulgu 20: yayın uyarısı haritası sınırlıdır: değişken uzunluklu şekillerle sınırsız
// büyümez; sınıra varılınca bir kez bildirilir ve sonraki yeni uyarılar susturulur.
func TestBroadcastWarningLimited(t *testing.T) {
	o := trrun.MustExecChecked(t, fmt.Sprintf(`TR_for n TR_in 1..%d {
    TR_var c = TR_zeros([n, 1]) + TR_zeros([n + 1])
}`, maxBroadcastWarnings+50))
	w := warningsOf(o.VM)
	w.mu.Lock()
	size := len(w.given)
	w.mu.Unlock()
	if size > maxBroadcastWarnings {
		t.Fatalf("uyarı haritası %d anahtar (sınır %d)", size, maxBroadcastWarnings)
	}
	if n := strings.Count(o.ErrOut, "\n"); n != maxBroadcastWarnings+1 {
		t.Fatalf("%d satır uyarı; %d uyarı ve bir susturma bildirimi bekleniyordu", n, maxBroadcastWarnings)
	}
	if !strings.Contains(o.ErrOut, "sonraki yayın uyarıları gösterilmeyecek") {
		t.Fatalf("susturma bildirimi yok:\n%s", o.ErrOut[len(o.ErrOut)-300:])
	}
}

// Kural (dil tanımı 15.49): iki taraf birden genişleyen yayında uyarı bütün yollarda yazılır:
// liste − tensör, karşılaştırma, atan2, koşullu ve toplam([a, b]).
// Neden: uyarı yalnızca bazı işlemlerin yolunda yazılıyordu.
func TestBroadcastWarningOnEveryPath(t *testing.T) {
	for _, tc := range []struct{ name, code string }{
		{"liste - tensör", `TR_var _ = [1.0, 2.0] - TR_zeros([2, 1])`},
		{"karşılaştırma <", `TR_var _ = TR_zeros([3, 1]) < TR_zeros([3])`},
		{"atan2", `TR_var _ = TR_atan2(TR_zeros([4, 1]), TR_ones([4]))`},
		{"koşullu", `TR_var _ = TR_where(TR_ones([5, 1]), TR_zeros([5]), TR_ones([5]))`},
		{"TR_sum([t1, t2])", `TR_var _ = TR_sum([TR_zeros([6, 1]), TR_zeros([6])])`},
	} {
		if n := strings.Count(trrun.MustExecChecked(t, tc.code).ErrOut, "Uyarı"); n != 1 {
			t.Errorf("%s: %d yayın uyarısı yazıldı, 1 bekleniyordu", tc.name, n)
		}
	}
}

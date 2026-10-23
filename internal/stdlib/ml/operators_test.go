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
	o := trrun.MustExecChecked(t, `değişken a = sıfırlar([4, 1])
değişken b = sıfırlar([4])
döngü i içinde 0..3 { değişken c = a + b }
yazdır((a - b).şekil)
değişken d = sıfırlar([2, 3]) + sıfırlar([3])
değişken e = sıfırlar([5, 3]) * sıfırlar([1, 3])`)
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
	const code = `değişken c = sıfırlar([4, 1]) + sıfırlar([4])`
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
	o := trrun.MustExecChecked(t, fmt.Sprintf(`döngü n içinde 1..%d {
    değişken c = sıfırlar([n, 1]) + sıfırlar([n + 1])
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

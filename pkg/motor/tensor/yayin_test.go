package tensor

import (
	"bytes"
	"strings"
	"testing"

	ct "tr_dili/pkg/tensor"
)

func TestYayinUyarisi(t *testing.T) {
	var buf bytes.Buffer
	uyariMu.Lock()
	eski := uyariCikisi
	uyariCikisi = &buf
	verilenUyari = map[string]bool{}
	uyariMu.Unlock()
	defer func() {
		uyariMu.Lock()
		uyariCikisi = eski
		uyariMu.Unlock()
	}()

	out := mustCalistir(t, `değişken a = sıfırlar([4, 1])
değişken b = sıfırlar([4])
döngü i içinde 0..3 { değişken c = a + b }
yazdır((a - b).şekil)
değişken d = sıfırlar([2, 3]) + sıfırlar([3])
değişken e = sıfırlar([5, 3]) * sıfırlar([1, 3])`)
	if out != "[4, 4]\n" {
		t.Fatalf("çıktı: %q", out)
	}
	uyari := buf.String()
	want := "Uyarı: [4, 1] ile [4] yayınlanarak [4, 4] üretildi (iki taraf da genişletildi); bilerek yapmadıysanız şekilleri kontrol edin.\n"
	// Toplama için döngüde bir kez, çıkarma için bir kez; tek taraflı yayınlamada uyarı yok.
	if strings.Count(uyari, want) != 2 || strings.Count(uyari, "Uyarı") != 2 {
		t.Fatalf("uyarılar:\n%s", uyari)
	}
}

func TestIkiTarafGenisler(t *testing.T) {
	durumlar := []struct {
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
	for _, d := range durumlar {
		if got := ikiTarafGenisler(ct.Zeros(d.a...), ct.Zeros(d.b...)); got != d.want {
			t.Errorf("%v ile %v: %v, %v bekleniyordu", d.a, d.b, got, d.want)
		}
	}
}

func BenchmarkIkiliToplama(b *testing.B) {
	x := Yeni(ct.Zeros(64, 64))
	y := Yeni(ct.Zeros(64))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := x.ikili("+", y); err != nil {
			b.Fatal(err)
		}
	}
}

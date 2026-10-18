package ogrenme

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"tr_dili/internal/hesap/tensor"
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
		if got := ikiTarafGenisler(tensor.Zeros(d.a...), tensor.Zeros(d.b...)); got != d.want {
			t.Errorf("%v ile %v: %v, %v bekleniyordu", d.a, d.b, got, d.want)
		}
	}
}

func BenchmarkIkiliToplama(b *testing.B) {
	x := Yeni(tensor.Zeros(64, 64))
	y := Yeni(tensor.Zeros(64))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := x.ikili("+", y); err != nil {
			b.Fatal(err)
		}
	}
}

// Bulgu 20: yayın uyarısı haritası sınırlıdır: değişken uzunluklu şekillerle sınırsız
// büyümez; sınıra varılınca bir kez bildirilir ve sonraki yeni uyarılar susturulur.
func TestYayinUyarisiSinirli(t *testing.T) {
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
	mustCalistir(t, fmt.Sprintf(`döngü n içinde 1..%d {
    değişken c = sıfırlar([n, 1]) + sıfırlar([n + 1])
}`, enFazlaYayinUyarisi+50))
	uyariMu.Lock()
	boy := len(verilenUyari)
	uyariMu.Unlock()
	if boy > enFazlaYayinUyarisi {
		t.Fatalf("uyarı haritası %d anahtar (sınır %d)", boy, enFazlaYayinUyarisi)
	}
	if n := strings.Count(buf.String(), "\n"); n != enFazlaYayinUyarisi+1 {
		t.Fatalf("%d satır uyarı; %d uyarı ve bir susturma bildirimi bekleniyordu", n, enFazlaYayinUyarisi)
	}
	if !strings.Contains(buf.String(), "sonraki yayın uyarıları gösterilmeyecek") {
		t.Fatalf("susturma bildirimi yok:\n%s", buf.String()[len(buf.String())-300:])
	}
}

// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"fmt"
	"testing"
)

// S13'ün üç işi verilen boyda bütün işi yapar ve sonucunu kullanır (TCB 2026-11-23 10:26, madde 2): sonuç değeri
// bağımsız bir hesapla aynıdır ve boyla değişir; derleyici döngüyü atsaydı ya da --n okunmasaydı değer tutmazdı.
func TestCompareWorksUseTheirSize(t *testing.T) {
	for _, n := range []int{0, 1, 10, 1000, 123457} {
		if got, want := dongu(n), n*(n-1)/2; n > 0 && got != want {
			t.Errorf("dongu(%d) = %d, want n(n-1)/2 = %d", n, got, want)
		}
	}
	a, b := 0, 1 // fib iteratif
	for n := 0; n <= 30; n++ {
		if got := fib(n); got != a {
			t.Errorf("fib(%d) = %d, want %d", n, got, a)
		}
		a, b = b, a+b
	}
	// matris_dongu: iz ve bütün öğelerin toplamı (DENETİM 953 Ş2: köşegen dışı da sonuca girer).
	for _, n := range []int{1, 7, 35} {
		iz, toplam := 0, 0
		for i := range n {
			for j := range n {
				for k := range n {
					p := ((i * k) % 7) * ((k + j) % 5)
					toplam += p
					if i == j {
						iz += p
					}
				}
			}
		}
		if got, _ := matrisIz(n); got != fmt.Sprintf("%d/%d", iz, toplam) {
			t.Errorf("matrisIz(%d) = %s, want %d/%d", n, got, iz, toplam)
		}
	}
	if x, y := dongu(100), dongu(200); x == y {
		t.Errorf("dongu gives %d at both sizes", x)
	}
}

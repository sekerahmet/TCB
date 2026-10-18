// kiyas, hız kıyas setinin (tools/kiyas) derlenmiş başvuru sürümüdür: TR ve Python
// programlarının yaptığı işleri aynı sonuç değerleriyle yapar ve sürelerini yazar.
// tools/kiyas/kos.py derleyip çalıştırır.
//
// Katman: araçlar (tools); kullanıcıya gitmez.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"time"
)

func ms(t time.Time) int64 { return time.Since(t).Milliseconds() }

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func main() {
	switch os.Args[1] {
	case "dongu":
		t := time.Now()
		toplam := 0
		for i := 0; i < 3000000; i++ {
			toplam += i
		}
		fmt.Println("dongu", toplam, ms(t))
	case "fib":
		t := time.Now()
		fmt.Println("fib", fib(27), ms(t))
	case "sirala":
		t := time.Now()
		x := 42
		l := []int{}
		for i := 0; i < 300000; i++ {
			x = (x*1103515245 + 12345) % 2147483648
			l = append(l, x)
		}
		sort.Ints(l)
		fmt.Println("sirala", l[0]+l[150000]+l[299999], ms(t))
	case "metin":
		t := time.Now()
		toplam := 0
		for i := 0; i < 200000; i++ {
			s := "kelime" + strconv.Itoa(i)
			toplam += len(s)
		}
		fmt.Println("metin", toplam, ms(t))
	case "sozluk":
		t := time.Now()
		d := map[string]int{}
		for i := 0; i < 200000; i++ {
			d["k"+strconv.Itoa(i)] = i
		}
		toplam := 0
		for i := 0; i < 200000; i++ {
			toplam += d["k"+strconv.Itoa(i)]
		}
		fmt.Println("sozluk", toplam, ms(t))
	case "matris_dongu":
		n := 120
		a := make([][]int, n)
		b := make([][]int, n)
		for i := range n {
			a[i] = make([]int, n)
			b[i] = make([]int, n)
			for j := range n {
				a[i][j] = (i * j) % 7
				b[i][j] = (i + j) % 5
			}
		}
		t := time.Now()
		iz := 0
		for i := range n {
			for j := range n {
				s := 0
				for k := range n {
					s += a[i][k] * b[k][j]
				}
				if i == j {
					iz += s
				}
			}
		}
		fmt.Println("matris_dongu", iz, ms(t))
	case "matris_yerlesik":
		n := 512
		a := make([]float64, n*n)
		b := make([]float64, n*n)
		for i := range a {
			a[i], b[i] = rand.Float64(), rand.Float64()
		}
		mul := func(x, y []float64) []float64 {
			c := make([]float64, n*n)
			for i := range n {
				for k := range n {
					v := x[i*n+k]
					for j := range n {
						c[i*n+j] += v * y[k*n+j]
					}
				}
			}
			return c
		}
		t := time.Now()
		c := mul(a, b)
		_ = mul(c, b)
		fmt.Println("matris_yerlesik", "-", ms(t))
	}
}

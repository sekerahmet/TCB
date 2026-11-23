// compare, hız kıyas setinin (tools/bench) derlenmiş başvuru sürümüdür: TR ve Python
// programlarının yaptığı işleri aynı sonuç değerleriyle yapar ve sürelerini yazar.
// tools/bench/run.py derleyip çalıştırır; temel ölçü setinin S13'ü (tools/bench/baseline) de.
//
//	compare <iş> [--n <boy>]
//
// --n işin boyudur (döngünün tur sayısı, fib'in argümanı, matrisin kenarı); verilmezse tools/bench/run.py'nin boyu.
// S13 boyu büyütür: Go'nun süresi dizüstünün saat adımının çok üstünde olsun (BC27); TR programı aynı --n'i alır, iki
// tarafın sonuç değeri aynıdır (uyuşma). Sonuç değeri her zaman yazdırılır: derleyici işi atamaz.
//
// Layer: tools (tools); kullanıcıya gitmez.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"time"
)

// ms geçen süreyi mikrosaniye çözünürlükte, milisaniye olarak yazar: derlenmiş sürümlerde süre çoğu kez 1 ms'nin
// altındadır ve tam sayıya kesilince 0 görünürdü.
func ms(t time.Time) string {
	return strconv.FormatFloat(float64(time.Since(t).Microseconds())/1000, 'f', 3, 64)
}

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

// dongu 0'dan n-1'e kadar toplar.
func dongu(n int) int {
	toplam := 0
	for i := 0; i < n; i++ {
		toplam += i
	}
	return toplam
}

// matrisIz n×n iki tamsayı matrisinin (a[i][j] = i·j mod 7, b[i][j] = (i+j) mod 5) çarpımının izi ve bütün öğelerinin
// toplamı: her iç çarpım sonuca girer, derleyici köşegen dışını atamaz (DENETİM 953 Ş2). Matrisler süre dışında kurulur.
func matrisIz(n int) (string, string) {
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
	iz, toplam := 0, 0
	for i := range n {
		for j := range n {
			s := 0
			for k := range n {
				s += a[i][k] * b[k][j]
			}
			toplam += s
			if i == j {
				iz += s
			}
		}
	}
	took := ms(t)
	return strconv.Itoa(iz) + "/" + strconv.Itoa(toplam), took
}

// timed bir işin sonuç değerini ve süresini verir.
func timed(f func(int) int, n int) (int, string) {
	t := time.Now()
	v := f(n)
	return v, ms(t)
}

func sirala() (int, string) {
	t := time.Now()
	x := 42
	l := []int{}
	for i := 0; i < 300000; i++ {
		x = (x*1103515245 + 12345) % 2147483648
		l = append(l, x)
	}
	sort.Ints(l)
	return l[0] + l[150000] + l[299999], ms(t)
}

func metin() (int, string) {
	t := time.Now()
	toplam := 0
	for i := 0; i < 200000; i++ {
		s := "kelime" + strconv.Itoa(i)
		toplam += len(s)
	}
	return toplam, ms(t)
}

func sozluk() (int, string) {
	t := time.Now()
	d := map[string]int{}
	for i := 0; i < 200000; i++ {
		d["k"+strconv.Itoa(i)] = i
	}
	toplam := 0
	for i := 0; i < 200000; i++ {
		toplam += d["k"+strconv.Itoa(i)]
	}
	return toplam, ms(t)
}

func matrisYerlesik() string {
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
	return ms(t)
}

// size --n'in değeri ya da işin öntanımlı boyu.
func size(def int) int {
	if len(os.Args) == 4 && os.Args[2] == "--n" {
		if n, err := strconv.Atoi(os.Args[3]); err == nil && n > 0 {
			return n
		}
	}
	if len(os.Args) > 2 {
		fmt.Fprintf(os.Stderr, "compare %s: --n wants a positive whole number, not %q. Example: compare %s --n %d\n",
			os.Args[1], os.Args[2:], os.Args[1], def)
		os.Exit(2)
	}
	return def
}

func main() {
	var v any // the result value: a number, matris_dongu's "<trace>/<sum of all elements>"
	var took string
	switch os.Args[1] {
	case "dongu":
		v, took = timed(dongu, size(3000000))
	case "fib":
		v, took = timed(fib, size(27))
	case "matris_dongu":
		v, took = matrisIz(size(120))
	case "sirala":
		v, took = sirala()
	case "metin":
		v, took = metin()
	case "sozluk":
		v, took = sozluk()
	case "matris_yerlesik":
		fmt.Println("matris_yerlesik", "-", matrisYerlesik())
		return
	default:
		fmt.Fprintf(os.Stderr, "compare: unknown work %q\n", os.Args[1])
		os.Exit(2)
	}
	fmt.Println(os.Args[1], v, took)
}

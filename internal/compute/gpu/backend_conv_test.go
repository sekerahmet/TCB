package gpu

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Kural (15.23c, 15.41; 25.9): TR_conv1d kartta çalışır. Açma ve katlama işlemciyle bit bit aynıdır
// (15.41'in 1. basamağı: kopya ve sabit sıralı toplama); evrişimin sonucu ve gradyanları `@`'ın
// basamağındadır (3. basamak). Sınır girdileri: boş girdi, dolgulu tam çekirdek, girdiden büyük
// dolgu, çekirdekten büyük adım, genişleme ile grup birlikte, grup başına tek çıkış kanalı.

// convCase bir evrişimin şekilleri ve ayarlarıdır.
type convCase struct {
	batch, cin, l, cout, k int
	o                      tensor.Conv1dOptions
}

func (c convCase) String() string {
	return fmt.Sprintf("B%d Cin%d L%d Cout%d K%d %+v", c.batch, c.cin, c.l, c.cout, c.k, c.o)
}

var convCases = []convCase{
	{2, 4, 9, 6, 3, tensor.Conv1dOptions{Stride: 1, Padding: 1, Dilation: 1, Groups: 2}},
	{1, 3, 5, 2, 5, tensor.Conv1dOptions{Stride: 1, Padding: 2, Dilation: 1, Groups: 1}},  // K = L, dolgulu
	{1, 2, 3, 2, 2, tensor.Conv1dOptions{Stride: 1, Padding: 5, Dilation: 1, Groups: 1}},  // dolgu > L
	{2, 2, 20, 3, 2, tensor.Conv1dOptions{Stride: 5, Padding: 0, Dilation: 1, Groups: 1}}, // adım > K
	{1, 6, 17, 6, 3, tensor.Conv1dOptions{Stride: 2, Padding: 3, Dilation: 2, Groups: 3}}, // genişleme ve grup
	{1, 4, 8, 2, 3, tensor.Conv1dOptions{Stride: 1, Padding: 1, Dilation: 1, Groups: 2}},  // Cout / g = 1
	{0, 2, 5, 2, 3, tensor.Conv1dOptions{Stride: 1, Padding: 0, Dilation: 1, Groups: 1}},  // boş toplu iş
	{1, 2, 0, 2, 1, tensor.Conv1dOptions{Stride: 1, Padding: 1, Dilation: 1, Groups: 1}},  // L = 0: yalnız dolgu
}

// convRun evrişimi dev'de kurar; açılmış satırları (cols) ve sonucu verir. x ve w türevlidir.
func convRun(dev tensor.Device, c convCase, xd, wd []float32, dt tensor.DType) (x, w, cols, y *tensor.Tensor) {
	x = tensor.NewOn(dev, xd, c.batch, c.cin, c.l).Cast(dt).SetRequiresGrad(true)
	w = tensor.NewOn(dev, wd, c.cout, c.cin/c.o.Groups, c.k).Cast(dt).SetRequiresGrad(true)
	y = tensor.Conv1d(x, w, nil, c.o, func(a, b *tensor.Tensor) *tensor.Tensor {
		cols = a
		return tensor.MatMul(a, b)
	})
	return x, w, cols, y
}

// specialValues girdinin birkaç öğesini özel değerlerle değiştirir (15.50): açma onları kopyalar.
func specialValues(d []float32) {
	for i, v := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Copysign(0, -1))} {
		if j := 3*i + 1; j < len(d) {
			d[j] = v
		}
	}
}

// convSameBits açmanın çıktısının ve sabit tohumlu katlamanın bitlerini iki cihazda karşılaştırır.
func convSameBits(t *testing.T, c convCase, seed int64, dt tensor.DType) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	xd := random(rng, c.batch*c.cin*c.l, 1)
	wd := random(rng, c.cout*c.cin/c.o.Groups*c.k, 1)
	name := fmt.Sprintf("%s %s", dt, c)
	var bits [2][2][]uint32 // cihaz; açma, katlama
	for d, dev := range []tensor.Device{tensor.CPU, gpuDevice} {
		x, _, cols, _ := convRun(dev, c, xd, wd, dt)
		bits[d][0] = bitsOf(cols)
		// Katlama: türev doğrudan açmanın çıktısından (aynı tohum iki cihazda); NaN'sız tohum,
		// toplamın bitleri iki cihazda aynı olmalı.
		r := tensor.NewOn(dev, random(rand.New(rand.NewSource(7)), cols.Numel(), 1), cols.Shape()...).Cast(dt)
		tensor.Mul(cols, r).SumAll().Backward()
		bits[d][1] = bitsOf(x.Grad())
		if x.Grad().Device() != dev {
			t.Fatalf("%s: türev %s'de, %s bekleniyordu", name, x.Grad().Device(), dev)
		}
	}
	sameBitsAt(t, name+" açma", bits[1][0], bits[0][0])
	sameBitsAt(t, name+" katlama", bits[1][1], bits[0][1])
}

// convClose evrişimin sonucunu ve türevlerini iki cihazda `@`'ın basamağında karşılaştırır.
func convClose(t *testing.T, c convCase, seed int64) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	xd := random(rng, c.batch*c.cin*c.l, 1)
	wd := random(rng, c.cout*c.cin/c.o.Groups*c.k, 1)
	bd := random(rng, c.cout, 1)
	compare(t, c.String(), 1e-5, func(d int) (*tensor.Tensor, []*tensor.Tensor) {
		dev := []tensor.Device{tensor.CPU, gpuDevice}[d]
		x := tensor.NewOn(dev, xd, c.batch, c.cin, c.l).SetRequiresGrad(true)
		w := tensor.NewOn(dev, wd, c.cout, c.cin/c.o.Groups, c.k).SetRequiresGrad(true)
		b := tensor.NewOn(dev, bd, c.cout).SetRequiresGrad(true)
		return tensor.Conv1d(x, w, b, c.o, tensor.MatMul), []*tensor.Tensor{x, w, b}
	})
}

func TestConv1dUnfoldAndFoldMatchCPUBits(t *testing.T) {
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16} {
		for i, c := range convCases {
			convSameBits(t, c, int64(40+i), dt)
		}
	}
}

// Kural (15.23c: adım ve genişleme kırpılmaz; S-717): 32 bite sığmayan adım ve çekirdek 1'in
// genişlemesi kartta da işlemciyle aynı sonucu verir. Adım dolgulu uzunluğu aşınca tek kare vardır.
func TestConv1dHugeSettings(t *testing.T) {
	var cases []convCase
	for _, v := range []int{1<<31 - 1, 1 << 31, 1<<32 - 2, 1<<32 - 1, 1 << 32, math.MaxInt} {
		for k := 1; k <= 2; k++ {
			cases = append(cases,
				convCase{1, 1, 5, 1, k, tensor.Conv1dOptions{Stride: v, Dilation: 1, Groups: 1}},
				convCase{1, 1, 5, 1, k, tensor.Conv1dOptions{Stride: v, Padding: 1, Dilation: 1, Groups: 1}})
		}
		cases = append(cases, convCase{1, 1, 5, 1, 1, tensor.Conv1dOptions{Stride: 1, Padding: 1, Dilation: v, Groups: 1}})
	}
	both := tensor.Conv1dOptions{Stride: 1<<32 - 1, Padding: 2, Dilation: 1 << 40, Groups: 1}
	cases = append(cases, convCase{3, 1, 5, 2, 1, both}, convCase{1, 2, 5, 2, 1, both},
		convCase{1, 2, 5, 2, 2, tensor.Conv1dOptions{Stride: 1<<32 - 1, Padding: 2, Dilation: 1, Groups: 2}})
	for i, c := range cases {
		if f := c.o.Length(c.l, c.k); f > 1 && c.o.Stride > 1 {
			t.Fatalf("%s: %d kare, tek kare bekleniyordu", c, f)
		}
		convSameBits(t, c, int64(80+i), tensor.Float32)
		convClose(t, c, int64(80+i))
	}
}

// Kural (15.23c: dolgu kırpılmaz; 15.22a, 15.41; S-717, S-718): dolgulu uzunluk 2³¹ ve üstünde kart 64
// bitlik eş çekirdekle işlemcinin bitlerini verir; altında dar çekirdek koşar (convWide).
func TestConv1dHugePadding(t *testing.T) {
	for _, c := range []struct {
		l, pad int
		want   bool
	}{
		{5, 0, false}, {5, 1<<30 - 3, false}, {4, 1<<30 - 2, true}, {5, 1<<30 - 2, true},
		{0, 1<<30 - 1, false}, {0, 1 << 30, true}, {5, 1 << 32, true}, {5, 1<<62 - 3, true},
	} {
		if got := convWide(tensor.Conv1dLayout{Length: c.l, Conv1dOptions: tensor.Conv1dOptions{Padding: c.pad}}); got != c.want {
			t.Errorf("convWide(L %d, dolgu %d) = %v, beklenen %v", c.l, c.pad, got, c.want)
		}
	}
	o := func(stride, pad, dil, groups int) tensor.Conv1dOptions {
		return tensor.Conv1dOptions{Stride: stride, Padding: pad, Dilation: dil, Groups: groups}
	}
	cases := []convCase{
		{1, 1, 5, 1, 2, o(1<<29, 1<<30-3, 1, 1)},    // dar çekirdeğin son uzunluğu (2³¹ − 1)
		{1, 1, 4, 1, 2, o(1<<29, 1<<30-2, 1, 1)},    // geniş çekirdeğin ilk uzunluğu (2³¹)
		{1, 1, 4, 1, 1, o(1<<40, 1431655764, 1, 1)}, // dolgulu uzunluk 2 863 311 532
		{1, 1, 4, 1, 2, o(1<<40, 1431655764, 1, 1)},
		{3, 1, 5, 2, 2, o(1<<32, 1<<32, 1, 1)},
		{1, 4, 5, 2, 2, o(1<<32, 1<<32, 1, 2)},
		{2, 2, 6, 2, 3, o(1<<33, 1<<33+1, 3, 1)},
		{1, 1, 5, 1, 1, o(math.MaxInt, 1<<62-3, 1, 1)}, // dolgulu uzunluk 2⁶³ − 1, tek kare
		{1, 1, 5, 1, 2, o(1<<62-5, 1<<62-3, 1<<62, 1)}, // açıklık 2⁶²
	}
	for _, p := range []int{1 << 32, 1 << 40, 1<<60 - 1, 1 << 60, 1<<60 + 2, 1 << 61, 1<<62 - 3} {
		for k := 1; k <= 2; k++ {
			cases = append(cases, convCase{1, 1, 5, 1, k, o(p, p, 1, 1)})
		}
	}
	for _, d := range []int{1<<31 + 3, 1<<32 - 2, 1<<32 - 1, 1 << 32} { // aynı boy dolgusu
		cases = append(cases, convCase{1, 2, 8, 2, 3, o(1, d, d, 1)})
	}
	for i, c := range cases {
		convSameBits(t, c, int64(120+i), tensor.Float32)
		convClose(t, c, int64(120+i))
	}
}

func TestConv1dSpecialValuesAreCopied(t *testing.T) {
	c := convCases[4]
	rng := rand.New(rand.NewSource(5))
	xd := random(rng, c.batch*c.cin*c.l, 1)
	specialValues(xd)
	wd := random(rng, c.cout*c.cin/c.o.Groups*c.k, 1)
	_, _, colsCPU, _ := convRun(tensor.CPU, c, xd, wd, tensor.Float32)
	_, _, colsGPU, _ := convRun(gpuDevice, c, xd, wd, tensor.Float32)
	sameBitsAt(t, "özel değerler", bitsOf(colsGPU), bitsOf(colsCPU))
}

func TestConv1dMatchesCPUWithinMatmulStep(t *testing.T) {
	for i, c := range convCases[:6] {
		convClose(t, c, int64(60+i))
	}
}

func TestConv1dSameBitsFromRunToRun(t *testing.T) {
	c := convCases[0]
	rng := rand.New(rand.NewSource(3))
	xd := random(rng, c.batch*c.cin*c.l, 1)
	wd := random(rng, c.cout*c.cin/c.o.Groups*c.k, 1)
	var first [2][]uint32
	for run := range 2 {
		x, _, _, y := convRun(gpuDevice, c, xd, wd, tensor.Float32)
		y.SumAll().Backward()
		got := [2][]uint32{bitsOf(y), bitsOf(x.Grad())}
		if run == 0 {
			first = got
			continue
		}
		sameBitsAt(t, "sonuç", got[0], first[0])
		sameBitsAt(t, "türev", got[1], first[1])
	}
}

package gpu

import (
	"fmt"
	"maps"
	"math"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// int64Edges sınır değerleri: sıfır ve birin çevresi, 32 bitin sınırı, karesi int64'ün
// sınırında olanlar (3037000499² < 2⁶³ < 3037000500²), 2⁶²'nin ve 2⁶³'ün iki yanı.
var int64Edges = []int64{0, 1, -1, 2, -2, 3, -3, 7, -7, 1 << 31, -(1 << 31), 3037000499, 3037000500, -3037000500,
	1 << 32, 1 << 62, -(1 << 62), math.MaxInt64, math.MaxInt64 - 1, math.MinInt64, math.MinInt64 + 1}

func int64On(dev tensor.Device, vals []int64, shape ...int) *tensor.Tensor {
	if shape == nil {
		shape = []int{len(vals)}
	}
	return tensor.Int64sOn(dev, tensor.Int64, vals, shape...)
}

// arithBoth a op b'yi işlemcide ve kartta yapar: hata yoksa değerler aynı olmalı, varsa hata
// aynı (çeşit, konum, işlenenler, şekil). Hatalı öğe zararsız bir çiftle (0, 1) değiştirilip
// yinelenir: her hatalı öğe sırayla, en küçük konumdan başlayarak sınanır.
func arithBoth(t *testing.T, name string, op tensor.ArithOp, xs, ys []int64) {
	t.Helper()
	xs, ys = slices.Clone(xs), slices.Clone(ys)
	for {
		c, cf := tensor.Arith(int64On(tensor.CPU, xs), int64On(tensor.CPU, ys), op, tensor.Int64)
		g, gf := tensor.Arith(int64On(gpuDevice, xs), int64On(gpuDevice, ys), op, tensor.Int64)
		if !sameFault(cf, gf) {
			t.Fatalf("%s: kart %+v, işlemci %+v", name, gf, cf)
		}
		if cf == nil {
			if !slices.Equal(g.Int64s(), c.Int64s()) || g.Device() != gpuDevice {
				t.Fatalf("%s: kart %v, işlemci %v", name, g.Int64s(), c.Int64s())
			}
			return
		}
		xs[cf.Index], ys[cf.Index] = 0, 1
	}
}

// Kural (15.3, 15.6; T1 dilim 2b): kartta "int64" + - * // % işlemcinin değerini ve hatasını
// verir: taşmanın iki ucu, MinInt64 · −1, MinInt64 // −1 (taşma), MinInt64 % −1 (0, hata değil),
// sıfıra bölme; // aşağı yuvarlar, % bölenin işaretini alır. Bütün sınır çiftleri, her hata
// konum sırasıyla.
func TestInt64ArithMatchesCPU(t *testing.T) {
	var xs, ys []int64
	for _, x := range int64Edges {
		for _, y := range int64Edges {
			xs, ys = append(xs, x), append(ys, y)
		}
	}
	for _, c := range []struct {
		name string
		op   tensor.ArithOp
	}{{"+", tensor.ArithAdd}, {"-", tensor.ArithSub}, {"*", tensor.ArithMul}, {"//", tensor.ArithFloorDiv}, {"%", tensor.ArithMod}} {
		arithBoth(t, c.name, c.op, xs, ys)
	}
	// Üs: eksi üs hatadır; işlemcinin üssü tek tek çarptığı için üsler küçük tutulur.
	xs, ys = nil, nil
	for _, x := range int64Edges {
		for _, y := range []int64{-1, 0, 1, 2, 3, 31, 32, 62, 63, 64, 100} {
			xs, ys = append(xs, x), append(ys, y)
		}
	}
	arithBoth(t, "**", tensor.ArithPow, xs, ys)
	arithBoth(t, "** (-2)^63", tensor.ArithPow, []int64{-2, 2, -2}, []int64{63, 62, 64})
	// Yol kartın çekirdeğinden geçti (işlemciye düşmedi): her işlecin çekirdeği derlendi.
	g := gpuBackend()
	g.enter()
	defer g.exit()
	for _, op := range []tensor.ArithOp{tensor.ArithAdd, tensor.ArithSub, tensor.ArithMul, tensor.ArithFloorDiv, tensor.ArithMod, tensor.ArithPow} {
		prefix := fmt.Sprintf("cinsli:%d:%d:", tensor.TArith, op)
		if !slices.ContainsFunc(slices.Collect(maps.Keys(g.kernels)), func(k string) bool { return strings.HasPrefix(k, prefix) }) {
			t.Fatalf("%s çekirdeği derlenmedi", prefix)
		}
	}
}

// Kural (15.3, 15.6; T1 dilim 2b): tekli eksi ve mutlak değer kartta işlemcininkidir; MinInt64'te
// taşma. Yayınlama (sayı ile tensör, Dost'un `t - 1` biçimi), 0 boyutlu ve boş tensör.
func TestInt64UnaryAndBroadcastMatchCPU(t *testing.T) {
	for _, neg := range []bool{true, false} {
		xs := slices.Clone(int64Edges)
		for {
			var c, g *tensor.Tensor
			var cf, gf *tensor.ElemFault
			if neg {
				c, cf = tensor.Neg(int64On(tensor.CPU, xs))
				g, gf = tensor.Neg(int64On(gpuDevice, xs))
			} else {
				c, cf = tensor.AbsTyped(int64On(tensor.CPU, xs))
				g, gf = tensor.AbsTyped(int64On(gpuDevice, xs))
			}
			if !sameFault(cf, gf) {
				t.Fatalf("tekli (eksi %v): kart %+v, işlemci %+v", neg, gf, cf)
			}
			if cf == nil {
				if !slices.Equal(g.Int64s(), c.Int64s()) {
					t.Fatalf("tekli (eksi %v): kart %v, işlemci %v", neg, g.Int64s(), c.Int64s())
				}
				break
			}
			xs[cf.Index] = 0
		}
	}
	one := int64On(gpuDevice, []int64{1}, []int{}...)
	for _, x := range [][]int64{{5, math.MinInt64, 0}, {math.MinInt64 + 1, math.MinInt64}} {
		c, cf := tensor.Arith(int64On(tensor.CPU, x), int64On(tensor.CPU, []int64{1}, []int{}...), tensor.ArithSub, tensor.Int64)
		g, gf := tensor.Arith(int64On(gpuDevice, x), one, tensor.ArithSub, tensor.Int64)
		if !sameFault(cf, gf) || cf == nil && !slices.Equal(g.Int64s(), c.Int64s()) {
			t.Fatalf("%v - 1: kart %v %+v, işlemci %v %+v", x, g, gf, c, cf)
		}
	}
	scalar := func(dev tensor.Device, v int64) *tensor.Tensor { return int64On(dev, []int64{v}, []int{}...) }
	_, cf := tensor.Arith(scalar(tensor.CPU, math.MaxInt64), scalar(tensor.CPU, 1), tensor.ArithAdd, tensor.Int64)
	_, gf := tensor.Arith(scalar(gpuDevice, math.MaxInt64), scalar(gpuDevice, 1), tensor.ArithAdd, tensor.Int64)
	if gf == nil || !sameFault(cf, gf) || gf.Index != 0 || len(gf.Shape) != 0 {
		t.Fatalf("0 boyutlu taşma: kart %+v, işlemci %+v", gf, cf)
	}
	e, ef := tensor.Arith(int64On(gpuDevice, nil, 0, 3), int64On(gpuDevice, nil, 0, 3), tensor.ArithFloorDiv, tensor.Int64)
	if ef != nil || !slices.Equal(e.Shape(), []int{0, 3}) {
		t.Fatalf("boş: %v %+v", e, ef)
	}
}

// Kural (15.6; T1 dilim 2b): birden çok öğe hatalıysa kart düz konumu en küçük olanı söyler, iki
// cihazda aynı: daha küçük konumdaki sıfıra bölme büyükteki taşmadan önce gelir; büyük tensörde
// rastgele konumlardaki taşmalarda her koşuda en küçük konum (atomların sırası sonucu
// değiştirmez). Benzeticide 2¹², kartta 2²⁰ öğe.
func TestInt64ArithFirstFaultIsSmallestPosition(t *testing.T) {
	_, cf := tensor.Arith(int64On(tensor.CPU, []int64{4, 1, math.MinInt64, 7}), int64On(tensor.CPU, []int64{2, 0, -1, 0}), tensor.ArithFloorDiv, tensor.Int64)
	_, gf := tensor.Arith(int64On(gpuDevice, []int64{4, 1, math.MinInt64, 7}), int64On(gpuDevice, []int64{2, 0, -1, 0}), tensor.ArithFloorDiv, tensor.Int64)
	if !sameFault(cf, gf) || gf.Index != 1 || gf.Kind != tensor.FaultDivZero {
		t.Fatalf("küçük: kart %+v, işlemci %+v; beklenen sıfıra bölme, konum 1", gf, cf)
	}
	n := 1 << 20
	if inSim() {
		n = 1 << 12
	}
	ones := int64On(gpuDevice, slices.Repeat([]int64{1}, n))
	rng := rand.New(rand.NewSource(13))
	for run := 0; run < 3; run++ {
		x := make([]int64, n)
		want := n
		for k := 0; k < 64; k++ {
			at := rng.Intn(n)
			x[at] = math.MaxInt64
			want = min(want, at)
		}
		_, gf := tensor.Arith(int64On(gpuDevice, x), ones, tensor.ArithAdd, tensor.Int64)
		if gf == nil || gf.Index != want || gf.Kind != tensor.FaultOverflow || gf.A != math.MaxInt64 || gf.B != 1 {
			t.Fatalf("koşu %d: kart %+v; beklenen konum %d", run, gf, want)
		}
	}
}

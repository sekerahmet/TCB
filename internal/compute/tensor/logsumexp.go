package tensor

import (
	"math"
	"slices"
)

// log Σ exp (15.18). CPU'da satır en büyük öğesi çıkarılarak toplanır; toplam float64'tedir, satırlar
// birbirinden bağımsızdır, bir satırın toplama sırası yalnız şekle bağlıdır (15.22a). Kartta hesap
// arka ucun satır çekirdeğindedir (RowLogSumExper; float32 telafili toplam), CPU'ya düşmez (25.9).

// lseCost bir öğenin yaklaşık maliyetidir (işçi sayısı için).
const lseCost = 16

// lsePlan indirgemenin düzenidir: korunan eksenler önde, indirgenenler sonda satır satır okunur.
type lsePlan struct {
	order    []int // girdinin eksenlerinin okuma sırası
	permuted []int // bu sıradaki şekil
	out      []int // sonucun şekli
	rows, n  int   // satır sayısı ve satırın öğe sayısı
}

// newLsePlan eksenleri düzenler; axes nil ise bütün eksenler indirgenir.
func newLsePlan(shape, axes []int, keep bool) lsePlan {
	p := lsePlan{rows: 1, n: 1}
	reduced := func(i int) bool { return axes == nil || slices.Contains(axes, i) }
	for i, d := range shape {
		if !reduced(i) {
			p.order = append(p.order, i)
			p.rows *= d
			p.out = append(p.out, d)
		} else if keep {
			p.out = append(p.out, 1)
		}
	}
	for i, d := range shape {
		if reduced(i) {
			p.order = append(p.order, i)
			p.n *= d
		}
	}
	for _, i := range p.order {
		p.permuted = append(p.permuted, shape[i])
	}
	return p
}

// read t'nin satır satır görünümüdür; sıfır boyutlu tensörde kendisidir.
func (p lsePlan) read(t *Tensor) *Tensor {
	if len(p.order) == 0 {
		return t
	}
	return t.Permute(p.order...)
}

// LogSumExp t'nin axes eksenlerinde log Σ exp'idir; axes nil ise bütün öğelerde. Sonuç girdinin
// hassasiyetindedir, 16 bitlik girdi float32'de toplanır ve bir kez yuvarlanır.
func LogSumExp(t *Tensor, axes []int, keep bool) *Tensor {
	p := newLsePlan(t.shape, axes, keep)
	allocSizeOn(t.be, isCreate, p.out, Float32)
	if t.Device() != CPU {
		return lseOnDevice(t, p)
	}
	v := p.read(t.Detach())
	if t.dtyp == Float64 {
		return Float64sOn(CPU, lseRows(v.Float64s(), p, lseRow64), p.out...)
	}
	y := lseRows(v.Data(), p, lseRow32)
	out := NewOn(CPU, y, p.out...)
	if t.dtyp != Float32 {
		out = out.CastShared(t.dtyp)
	}
	return trackLse(t, out, y, p)
}

// lseRows satırları işçilere böler; her satır yalnız kendi öğelerini okur.
func lseRows[T floatType](x []T, p lsePlan, row func(x, work []T) T) []T {
	y := make([]T, p.rows)
	parallelFor(p.rows, p.n*lseCost, func(lo, hi int) {
		w := make([]T, p.n)
		for r := lo; r < hi; r++ {
			y[r] = row(x[r*p.n:(r+1)*p.n], w)
		}
	})
	return y
}

// lseHead en büyük öğeyi bulur; NaN varsa (+sonsuzla birlikte de) sonuç NaN'dır, öğe yoksa -sonsuz.
func lseHead[T floatType](x []T) (m T, done bool) {
	m = T(math.Inf(-1))
	for _, v := range x {
		if v != v {
			return v, true
		}
		if v > m {
			m = v
		}
	}
	return m, math.IsInf(float64(m), 0)
}

// lseRow32 bir satırın sonucudur; üsler expVec ile, toplam float64'te alınır.
func lseRow32(x, w []float32) float32 {
	m, done := lseHead(x)
	if done {
		return m
	}
	for i, v := range x {
		w[i] = v - m
	}
	expVec(w, w)
	return float32(float64(m) + math.Log(total64(w)))
}

// lseRow64 float64 satırın sonucudur.
func lseRow64(x, _ []float64) float64 {
	m, done := lseHead(x)
	if done {
		return m
	}
	s := 0.0
	for _, v := range x {
		s += math.Exp(v - m)
	}
	return m + math.Log(s)
}

// trackLse sonucu türev çizgesine bağlar; float64 cinsli yoldadır, türev izlemez (15.48).
func trackLse(t, out *Tensor, y []float32, p lsePlan) *Tensor {
	if !GradEnabled() || !t.RequiresGrad() {
		return out
	}
	sx := t.Detach() // geçici işlenen bu çağrıdan sonra bırakılır
	dt := t.dtyp
	return CustomOp("logsumexp", []*Tensor{t}, []*Tensor{out}, []*Tensor{sx}, func(_ any, g []*Tensor) []*Tensor {
		return []*Tensor{lseGrad(sx, y, g[0], p, dt)}
	})[0]
}

// lseGrad girdinin gradyanıdır: ∂y/∂x[i] = exp(x[i] − m) / Σ exp(x − m). Sonuç −sonsuzsa türev 0, +sonsuzsa bütün
// türev ilk +sonsuz öğeye gider (15.18).
func lseGrad(sx *Tensor, y []float32, g *Tensor, p lsePlan, dt DType) *Tensor {
	x, gy := p.read(sx).Data(), g.Data()
	dx := make([]float32, len(x))
	parallelFor(p.rows, p.n*lseCost, func(lo, hi int) {
		for r := lo; r < hi; r++ {
			lseRowGrad(dx[r*p.n:(r+1)*p.n], x[r*p.n:(r+1)*p.n], y[r], gy[r])
		}
	})
	r := NewOn(CPU, dx, p.permuted...)
	if len(p.order) > 0 {
		r = r.Permute(inversePerm(p.order)...).Contiguous()
	}
	if dt != Float32 {
		r = r.CastShared(dt)
	}
	return r
}

// lseRowGrad bir satırın gradyanını yazar. Üsler en büyük öğeden hesaplanır: y yuvarlanmıştır ve
// büyük öğede log Σ'yı yutar, exp(x[i] − y) [1e7, 1e7]'de 0,5 yerine 0,3679 verirdi.
func lseRowGrad(dx, x []float32, y, g float32) {
	switch {
	case math.IsInf(float64(y), -1):
	case math.IsInf(float64(y), 1):
		dx[slices.Index(x, y)] = g
	default:
		m, _ := lseHead(x)
		for i, v := range x {
			dx[i] = v - m
		}
		expVec(dx, dx)
		sum := total64(dx)
		for i, e := range dx {
			dx[i] = g * float32(float64(e)/sum)
		}
	}
}

// lseOnDevice kartın yoludur: indirgenen eksenler sona alınır (bitişik kopya), satır başına arka
// ucun çekirdeği (RowLogSumExper). Türev kopyanın üzerinden girdiye döner (Permute, Contiguous).
// float64 kartta yoktur; çağıran (ml) onu önce reddeder.
func lseOnDevice(t *Tensor, p lsePlan) *Tensor {
	be, ok := t.be.(RowLogSumExper)
	if !ok || t.dtyp == Float64 || t.dtyp.IsTyped() || t.dtyp.IsQuantized() {
		panic("tensor: LogSumExp: the backend has no row log-sum-exp for " + t.dtyp.Name())
	}
	v := p.read(t).Contiguous()
	var out *Tensor
	if p.n == 0 { // boş satır −sonsuz, türevi 0 (işlemcideki gibi izlenir)
		out = FullOn(t.Device(), float32(math.Inf(-1)), p.out...).CastShared(t.dtyp)
	} else {
		out = emptyOf(isReduce, t.be, p.out, t.dtyp)
		gateRowLogSumExp(be, out.storage, v.view(), p.rows, p.n)
	}
	if !GradEnabled() || !t.RequiresGrad() {
		return out
	}
	rows, n := p.rows, p.n
	return CustomOp("logsumexp", []*Tensor{v}, []*Tensor{out}, []*Tensor{v}, func(_ any, g []*Tensor) []*Tensor {
		gx := ZerosOn(v.Device(), v.shape...)
		gateRowLogSumExpGrad(be, gx.storage, g[0].view(), v.view(), rows, n)
		if v.dtyp != Float32 {
			gx = gx.CastShared(v.dtyp)
		}
		return []*Tensor{gx}
	})[0]
}

// inversePerm order'ın tersidir: girdinin eksen sırasına geri döner.
func inversePerm(order []int) []int {
	inv := make([]int, len(order))
	for i, a := range order {
		inv[a] = i
	}
	return inv
}

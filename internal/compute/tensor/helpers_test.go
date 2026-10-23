package tensor

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/testkit"
	"math"
	"math/rand"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Test yardımcıları.

func near(t *testing.T, ad string, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: uzunluk %d, beklenen %d", ad, len(got), len(want))
	}
	for i := range got {
		g, w := float64(got[i]), float64(want[i])
		if math.IsNaN(g) || math.Abs(g-w) > tol*(1+math.Abs(w)) {
			t.Fatalf("%s: [%d] = %v, beklenen %v (tüm sonuç %v)", ad, i, got[i], want[i], got)
		}
	}
}

func shapeEqual(t *testing.T, ad string, got, want []int) {
	t.Helper()
	if !sameShape(got, want) {
		t.Fatalf("%s: şekil %v, beklenen %v", ad, got, want)
	}
}

func expectPanic(t *testing.T, ad string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: panik bekleniyordu", ad)
		}
	}()
	fn()
}

// sorted 1, 2, 3, ... değerleriyle tensör.
func sorted(shape ...int) *Tensor {
	d := make([]float32, numelOf(shape))
	for i := range d {
		d[i] = float32(i + 1)
	}
	return New(d, shape...)
}

// naiveMatMul float64 birikimli başvuru çarpımı.
func naiveMatMul(a, b []float32, M, N, K int) []float32 {
	c := make([]float32, M*N)
	for i := 0; i < M; i++ {
		for j := 0; j < N; j++ {
			var s float64
			for k := 0; k < K; k++ {
				s += float64(a[i*K+k]) * float64(b[k*N+j])
			}
			c[i*N+j] = float32(s)
		}
	}
	return c
}

// random [lo,hi) aralığında; mutlak değeri enAz'dan küçük değerleri iter
// (türevi süreksiz noktalardan uzak durmak için).
func random(rng *rand.Rand, lo, hi, minN float32, shape ...int) *Tensor {
	d := make([]float32, numelOf(shape))
	for i := range d {
		for {
			v := lo + (hi-lo)*rng.Float32()
			if v < 0 && -v < minN || v >= 0 && v < minN {
				continue
			}
			d[i] = v
			break
		}
	}
	return New(d, shape...)
}

// Sayısal gradyan denetimi ayarları (float32 için uygun tolerans).
const (
	gcEps  = 5e-3
	gcAtol = 3e-3
	gcRtol = 2e-2
)

// checkGrad, f'nin analitik gradyanlarını merkezi sonlu farklarla
// karşılaştırır. Kayıp L = Σ f(xs)·w (w sabit rastgele) float64 ile
// hesaplanır; böylece her çıktı elemanının türevi denetlenmiş olur.
func checkGrad(t *testing.T, ad string, f func(xs []*Tensor) *Tensor, xs ...*Tensor) {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	for _, x := range xs {
		x.SetRequiresGrad(true)
		x.n.grad = nil
	}
	out := f(xs)
	if !out.RequiresGrad() {
		t.Fatalf("%s: çıktı gradyan istemiyor", ad)
	}
	w := Randn(rng, out.shape...)
	wd := w.Data()
	out.BackwardWith(w)
	analytic := make([][]float32, len(xs))
	for k, x := range xs {
		analytic[k] = make([]float32, x.Numel())
		if x.n.grad != nil {
			copy(analytic[k], cd(x.n.grad))
		}
	}
	lossVal := func() float64 {
		var o *Tensor
		NoGrad(func() { o = f(xs) })
		d := o.Data()
		var s float64
		for i, v := range d {
			s += float64(v) * float64(wd[i])
		}
		return s
	}
	for k, x := range xs {
		if !x.IsContiguous() {
			t.Fatalf("%s: girdi %d bitişik olmalı", ad, k)
		}
		d := cd(x.storage)[x.offset : x.offset+x.Numel()]
		for j := range d {
			orig := d[j]
			xp, xm := orig+gcEps, orig-gcEps
			d[j] = xp
			lp := lossVal()
			d[j] = xm
			lm := lossVal()
			d[j] = orig
			num := (lp - lm) / (float64(xp) - float64(xm))
			an := float64(analytic[k][j])
			if math.IsNaN(an) || math.Abs(num-an) > gcAtol+gcRtol*math.Max(math.Abs(num), math.Abs(an)) {
				t.Fatalf("%s: girdi %d eleman %d: analitik %.6g, sayısal %.6g", ad, k, j, an, num)
			}
		}
	}
}

// panicCode fn'in, code ileti koduyla panik ettiğini doğrular.
func panicCode(t *testing.T, ad string, code messages.Code, fn func()) {
	t.Helper()
	err := testkit.PanicError(fn)
	if err == nil {
		t.Fatalf("%s: %s kodlu panik bekleniyordu, işlem başarılı oldu", ad, code)
	}
	if !messages.Has(err, code) {
		t.Fatalf("%s: %s kodlu panik bekleniyordu, alınan: %v", ad, code, err)
	}
}

// nanSame iki dilimin NaN'lar dahil bit düzeyinde değil değer düzeyinde aynı olduğunu
// (NaN yerinde NaN) doğrular.
func nanSame(t *testing.T, ad string, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: uzunluk %d, beklenen %d", ad, len(got), len(want))
	}
	for i := range got {
		gn, wn := got[i] != got[i], want[i] != want[i]
		if gn != wn || (!gn && got[i] != want[i]) {
			t.Fatalf("%s: %v, beklenen %v", ad, got, want)
		}
	}
}

// ============================================================================
// Gradyan kapısı: her türevli işlem için float64 başvuru ve bf16 tolerans testi
// ============================================================================

// refFn bir işlemin float64 başvuru hesabıdır: düz (satır öncelikli) girdilerden düz çıktı.
// Girdileri değiştirmez.
type refFn func(in [][]float64) []float64

// gradMode bir işlemin türev türü.
type gradMode int

const (
	gradDiff gradMode = iota // türevli: sayısal türevle uyuşur
	gradZero                 // parçalı sabit: çıktı çizgede, gradyan her yerde 0
	gradNone                 // türevsiz: çıktı çizgeye girmez (karşılaştırmalar)
)

// Gradyan kapısının varsayılan toleransları.
//
// float64: analitik gradyan float32 hesaplanır (göreli hata ~1e-6), başvuru sayısal türevi
// float64'te h=1e-6 adımıyla alınır (hata ~1e-10); 2e-4 float32 toplama sırası farklarına
// ve softmax/norm gibi bileşik işlemlerdeki birkaç kat hata birikimine yeter.
//
// bf16: yuvarlama başına göreli hata en çok 2⁻⁹ ≈ 2e-3'tür (8 bitlik anlamlı kısım, en
// yakına yuvarlama). Tolerans kayıt başınadır ve ölçülür: bf16Tolerances (behavior_grad_bf16_tolerances_test.go)
// en büyük ölçülen sapmanın 2 katıdır; ölçüm TestMeasureBF16'dadır. Hata, elemanın kendisine
// göre değil tensörün en büyük mutlak gradyanına göre ölçülür: toplamlardaki iptallerde küçük
// elemanın göreli hatası büyür ama tensörün ölçeğini aşmaz.
const (
	gradF64Tol = 2e-4
	gradFwdTol = 1e-4
)

// gradSpec bir işlemin (ya da işlem ailesinin) gradyan kapısı kaydıdır.
type gradSpec struct {
	name    string   // kayıt adı
	ops     []string // kapsadığı işlem adları (autograd düğüm etiketleri, BinOp/UnOp adları)
	mode    gradMode
	build   func(xs []*Tensor) *Tensor // türden bağımsız kurulum (float32 ve bf16 girdiyle çağrılır)
	inputs  func(rng *rand.Rand) []*Tensor
	ref     refFn
	fwdTol  float64 // ileri değer toleransı (0: gradFwdTol)
	gradTol float64 // float64 gradyan toleransı (0: gradF64Tol)
	setup   func(t *testing.T)
}

var gradGroups = map[string][]gradSpec{}

// registerGrad kayıtları işlemin kendi test dosyasının init'inden ekler; grup adı
// çalıştıran testin (runGradGroup) anahtarıdır.
func registerGrad(group string, specs ...gradSpec) {
	gradGroups[group] = append(gradGroups[group], specs...)
}

func orDefault(v, d float64) float64 {
	if v == 0 {
		return d
	}
	return v
}

// runGradGroup bir grubun bütün kayıtlarını float64 ve bf16 alt testleriyle koşar.
func runGradGroup(t *testing.T, group string) {
	t.Helper()
	specs := gradGroups[group]
	if len(specs) == 0 {
		t.Fatalf("%q grubunda gradyan kaydı yok", group)
	}
	for _, s := range specs {
		s := s
		t.Run(s.name+"/float64", func(t *testing.T) { checkGradFloat64(t, s) })
		t.Run(s.name+"/bf16", func(t *testing.T) { checkGradBF16(t, group+"/"+s.name, s) })
	}
}

func specRng(s gradSpec) *rand.Rand { return specRngSeed(s, 0) }

// specRngSeed kaydın k'inci tohumlu üreteci: k=0 float64 testinin kullandığıyla aynıdır.
func specRngSeed(s gradSpec, k int) *rand.Rand {
	var h int64 = 17
	for _, c := range s.name {
		h = h*31 + int64(c)
	}
	return rand.New(rand.NewSource(h + int64(k)*7919))
}

func toFloat64(xs []*Tensor) [][]float64 {
	in := make([][]float64, len(xs))
	for k, x := range xs {
		in[k] = x.Float64s()
	}
	return in
}

func dot64(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// checkFwd çıktıyı float64 başvuruyla karşılaştırır.
func checkFwd(t *testing.T, got []float32, want []float64, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ileri değer uzunluğu %d, başvuru %d", len(got), len(want))
	}
	for i := range got {
		g := float64(got[i])
		if math.IsNaN(g) || math.Abs(g-want[i]) > tol*(1+math.Abs(want[i])) {
			t.Fatalf("ileri değer [%d] = %v, başvuru %v", i, g, want[i])
		}
	}
}

// checkGradFloat64 analitik gradyanı başvurunun float64 merkezi sonlu farkıyla karşılaştırır.
func checkGradFloat64(t *testing.T, s gradSpec) {
	if s.setup != nil {
		s.setup(t)
	}
	rng := specRng(s)
	xs := s.inputs(rng)
	for _, x := range xs {
		x.SetRequiresGrad(true)
	}
	out := s.build(xs)
	in := toFloat64(xs)
	checkFwd(t, out.Data(), s.ref(in), orDefault(s.fwdTol, gradFwdTol))
	switch s.mode {
	case gradNone:
		if out.RequiresGrad() {
			t.Fatal("türevsiz işlemin çıktısı çizgeye girdi")
		}
		return
	case gradZero:
		out.BackwardWith(Randn(rng, out.shape...))
		for k, x := range xs {
			for i, g := range x.Grad().Data() {
				if g != 0 {
					t.Fatalf("girdi %d eleman %d: gradyan %v, 0 bekleniyordu", k, i, g)
				}
			}
		}
		return
	}
	if !out.RequiresGrad() {
		t.Fatal("çıktı gradyan istemiyor")
	}
	w := Randn(rng, out.shape...)
	out.BackwardWith(w)
	wd := w.Float64s()
	tol := orDefault(s.gradTol, gradF64Tol)
	for k := range xs {
		an := xs[k].Grad().Data()
		for j := range in[k] {
			orig := in[k][j]
			h := 1e-6 * math.Max(1, math.Abs(orig))
			in[k][j] = orig + h
			lp := dot64(s.ref(in), wd)
			in[k][j] = orig - h
			lm := dot64(s.ref(in), wd)
			in[k][j] = orig
			num := (lp - lm) / (2 * h)
			if math.IsNaN(float64(an[j])) || math.Abs(float64(an[j])-num) > tol*(1+math.Abs(num)) {
				t.Fatalf("girdi %d eleman %d: analitik %.7g, float64 sayısal %.7g", k, j, an[j], num)
			}
		}
	}
}

// gradRun işlemi dt türünde girdilerle koşar; çıktıyı (float32'ye çevrilmiş) ve girdi
// gradyanlarını verir. Tohum gradyanı her koşuda aynıdır.
func gradRun(dt DType, s gradSpec, rounded [][]float32, shapes [][]int) (out []float32, grads [][]float32, res *Tensor) {
	xs := make([]*Tensor, len(rounded))
	for i := range xs {
		xs[i] = NewOf(dt, rounded[i], shapes[i]...).SetRequiresGrad(true)
	}
	res = s.build(xs).Cast(Float32)
	out = res.Data()
	if res.RequiresGrad() {
		res.BackwardWith(Randn(rand.New(rand.NewSource(99)), res.shape...))
	}
	grads = make([][]float32, len(xs))
	for i, x := range xs {
		grads[i] = make([]float32, x.Numel())
		if x.n.grad != nil {
			copy(grads[i], x.Grad().Data())
		}
	}
	return out, grads, res
}

func maxAbs32(d []float32) float64 {
	var m float64
	for _, v := range d {
		if a := math.Abs(float64(v)); a > m && !math.IsInf(a, 0) {
			m = a
		}
	}
	return m
}

// bf16Seeds bf16 kapısının her kayıt için denediği girdi tohumu sayısıdır (ölçüm de bunlarla).
const bf16Seeds = 4

// bf16Deviation aynı işlemi k'inci tohumlu girdiyle bf16'da ve (bf16'ya yuvarlanmış aynı
// girdilerle) float32'de koşar; çıktı ile gradyanların, kendi tensörünün en büyük mutlak
// değerine göre en büyük sapmasını ve nerede olduğunu verir. NaN uyuşmazlığı +Inf sayılır.
func bf16Deviation(t *testing.T, s gradSpec, k int) (worst float64, where string) {
	t.Helper()
	if s.setup != nil {
		s.setup(t)
	}
	xs := s.inputs(specRngSeed(s, k))
	rounded := make([][]float32, len(xs))
	shapes := make([][]int, len(xs))
	for i, x := range xs {
		rounded[i] = x.Cast(BFloat16).Cast(Float32).Data()
		shapes[i] = x.Shape()
	}
	out32, g32, _ := gradRun(Float32, s, rounded, shapes)
	out16, g16, res := gradRun(BFloat16, s, rounded, shapes)
	if s.mode == gradNone && res.RequiresGrad() {
		t.Fatal("türevsiz işlemin bf16 çıktısı çizgeye girdi")
	}
	cmp := func(ad string, got, want []float32) {
		scale := math.Max(maxAbs32(want), 1e-6)
		for i := range want {
			d := math.Abs(float64(got[i])-float64(want[i])) / scale
			if math.IsNaN(float64(want[i])) != math.IsNaN(float64(got[i])) {
				d = math.Inf(1)
			}
			if d > worst {
				worst, where = d, fmt.Sprintf("%s [%d]: bf16 %v, float32 %v (ölçek %g)", ad, i, got[i], want[i], scale)
			}
		}
	}
	cmp("çıktı", out16, out32)
	for j := range g32 {
		cmp(fmt.Sprintf("girdi %d gradyanı", j), g16[j], g32[j])
	}
	return worst, where
}

// checkGradBF16 bf16Seeds girdi tohumunun hepsinde bf16 sonucunun (çıktı ve gradyanlar)
// bf16'ya yuvarlanmış girdilerle float32 koşudan belgelenmiş toleransı aşmadığını doğrular.
func checkGradBF16(t *testing.T, key string, s gradSpec) {
	tol, ok := bf16Tolerances[key]
	if !ok {
		t.Fatalf("%q için bf16 toleransı yok: TR_MEASURE_BF16=write ile ölçüp tabloyu yenileyin", key)
	}
	for k := 0; k < bf16Seeds; k++ {
		if worst, where := bf16Deviation(t, s, k); worst > tol {
			t.Fatalf("tohum %d: %s, sapma %.3g > tolerans %g", k, where, worst, tol)
		}
	}
}

// ---- kayıt kurucuları ----

// gen [lo,hi) aralığında, mutlak değeri minAbs'tan büyük rastgele tensör üreten girdi kurucusu.
func gen(lo, hi, minAbs float32, shape ...int) func(*rand.Rand) *Tensor {
	return func(r *rand.Rand) *Tensor { return random(r, lo, hi, minAbs, shape...) }
}

// genInputs girdi kurucularını sırayla çalıştırır.
func genInputs(gs ...func(*rand.Rand) *Tensor) func(*rand.Rand) []*Tensor {
	return func(r *rand.Rand) []*Tensor {
		xs := make([]*Tensor, len(gs))
		for i, g := range gs {
			xs[i] = g(r)
		}
		return xs
	}
}

// unarySpec tekli bir işlemin kaydı: (3,4) şekilli, [lo,hi) aralığında girdi.
func unarySpec(name string, f func(*Tensor) *Tensor, ref func(float64) float64, lo, hi, minAbs float32) gradSpec {
	return gradSpec{name: name, ops: []string{name}, build: func(xs []*Tensor) *Tensor { return f(xs[0]) },
		inputs: genInputs(gen(lo, hi, minAbs, 3, 4)), ref: map64(ref)}
}

// ---- float64 başvuru yardımcıları ----

// strides64 satır öncelikli adımlar.
func strides64(shape []int) []int {
	st := make([]int, len(shape))
	s := 1
	for i := len(shape) - 1; i >= 0; i-- {
		st[i] = s
		s *= shape[i]
	}
	return st
}

// bcastIndex çıktı koordinatının, yayınlanan girdi şeklindeki düz indeksi.
func bcastIndex(coord, shape []int) int {
	off := len(coord) - len(shape)
	idx, st := 0, 1
	for i := len(shape) - 1; i >= 0; i-- {
		if shape[i] != 1 {
			idx += coord[off+i] * st
		}
		st *= shape[i]
	}
	return idx
}

// bcastShape iki şeklin yayınlanmış şekli.
func bcastShape(a, b []int) []int {
	n := max(len(a), len(b))
	out := make([]int, n)
	for i := range out {
		out[i] = 1
		for _, sh := range [][]int{a, b} {
			if j := i - (n - len(sh)); j >= 0 && sh[j] > out[i] {
				out[i] = sh[j]
			}
		}
	}
	return out
}

// bcast64 iki girdiyi yayınlayıp f'yi eleman eleman uygulayan başvuru.
func bcast64(ash, bsh []int, f func(a, b float64) float64) refFn {
	out := bcastShape(ash, bsh)
	return func(in [][]float64) []float64 {
		res := make([]float64, numelOf(out))
		st := strides64(out)
		coord := make([]int, len(out))
		for i := range res {
			for k := range coord {
				coord[k] = i / st[k] % out[k]
			}
			res[i] = f(in[0][bcastIndex(coord, ash)], in[1][bcastIndex(coord, bsh)])
		}
		return res
	}
}

// map64 tekli eleman bazlı başvuru.
func map64(f func(float64) float64) refFn {
	return func(in [][]float64) []float64 {
		res := make([]float64, len(in[0]))
		for i, v := range in[0] {
			res[i] = f(v)
		}
		return res
	}
}

// lanes64 verilen eksen boyunca her şeridi (ekseni boyunca değerler) f ile dönüştürür;
// f'nin verdiği dilimin uzunluğu çıktıda o eksenin uzunluğu olur (indirgemede 1,
// softmax'ta aynı, topk'da k).
func lanes64(shape []int, axis int, f func(lane []float64) []float64) func(x []float64) []float64 {
	outer, n, inner := 1, shape[axis], 1
	for i, d := range shape {
		if i < axis {
			outer *= d
		} else if i > axis {
			inner *= d
		}
	}
	return func(x []float64) []float64 {
		var res []float64
		cols := make([][]float64, inner)
		for o := 0; o < outer; o++ {
			for c := 0; c < inner; c++ {
				lane := make([]float64, n)
				for a := range lane {
					lane[a] = x[(o*n+a)*inner+c]
				}
				cols[c] = f(lane)
			}
			for a := range cols[0] {
				for c := 0; c < inner; c++ {
					res = append(res, cols[c][a])
				}
			}
		}
		return res
	}
}

// randomSlice n elemanlı standart normal float32 dilimidir.
func randomSlice(rng *rand.Rand, n int) []float32 {
	d := make([]float32, n)
	for i := range d {
		d[i] = float32(rng.NormFloat64())
	}
	return d
}

// laneRef bir eksen boyunca şeritlere f uygulayan başvuruyu tek girdili refFn'e çevirir.
func laneRef(shape []int, axis int, f func([]float64) []float64) refFn {
	g := lanes64(shape, axis, f)
	return func(in [][]float64) []float64 { return g(in[0]) }
}

// genDisjoint elemanları ayrık (eşitliksiz) bir tensör üreten girdi üreteci verir (max/min gradyanları).
func genDisjoint(shape ...int) func(*rand.Rand) *Tensor {
	return func(r *rand.Rand) *Tensor { return disjoint(r, shape...) }
}

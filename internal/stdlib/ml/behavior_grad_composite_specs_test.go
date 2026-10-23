package ml

import (
	"math"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// rows64 düz diziyi son eksen uzunluğu d olan satırlara böler ve her satıra f uygular.
func rows64(x []float64, d int, f func(row, out []float64)) []float64 {
	out := make([]float64, len(x))
	for r := 0; r+d <= len(x); r += d {
		f(x[r:r+d], out[r:r+d])
	}
	return out
}

func rows64Sum(x []float64, d int) []float64 {
	var out []float64
	for r := 0; r+d <= len(x); r += d {
		var s float64
		for _, v := range x[r : r+d] {
			s += v
		}
		out = append(out, s)
	}
	return out
}

func softmax64(row, out []float64) {
	mx := math.Inf(-1)
	for _, v := range row {
		mx = math.Max(mx, v)
	}
	var z float64
	for i, v := range row {
		out[i] = math.Exp(v - mx)
		z += out[i]
	}
	for i := range out {
		out[i] /= z
	}
}

func logSoftmax64(row, out []float64) {
	softmax64(row, out)
	for i := range out {
		out[i] = math.Log(out[i])
	}
}

// layerNorm64 x [3,5], γ [5], β [5]; eps 1e-5 (yerleşiğin varsayılanı).
func layerNorm64(in [][]float64) []float64 {
	return rows64(in[0], 5, func(row, out []float64) {
		var mu, v float64
		for _, e := range row {
			mu += e / 5
		}
		for _, e := range row {
			v += (e - mu) * (e - mu) / 5
		}
		for d, e := range row {
			out[d] = (e-mu)/math.Sqrt(v+1e-5)*in[1][d] + in[2][d]
		}
	})
}

// rmsNorm64 x [3,5], γ [5]; eps 1e-5.
func rmsNorm64(in [][]float64) []float64 {
	return rows64(in[0], 5, func(row, out []float64) {
		var ss float64
		for _, e := range row {
			ss += e * e / 5
		}
		for d, e := range row {
			out[d] = e / math.Sqrt(ss+1e-5) * in[1][d]
		}
	})
}

// attention64 q, k, v [3 konum, 2 bakış, 4 boyut] (dikkat düzeni [T, H, dh]); nedensel; ölçek 1/√4.
func attention64(in [][]float64) []float64 {
	const H, T, D = 2, 3, 4
	at := func(t, h, d int) int { return (t*H+h)*D + d }
	out := make([]float64, H*T*D)
	for h := 0; h < H; h++ {
		q, k, v := in[0], in[1], in[2]
		for i := 0; i < T; i++ {
			sc := make([]float64, i+1)
			for j := 0; j <= i; j++ {
				for d := 0; d < D; d++ {
					sc[j] += q[at(i, h, d)] * k[at(j, h, d)] / 2
				}
			}
			p := make([]float64, i+1)
			softmax64(sc, p)
			for j := 0; j <= i; j++ {
				for d := 0; d < D; d++ {
					out[at(i, h, d)] += p[j] * v[at(j, h, d)]
				}
			}
		}
	}
	return out
}

func matmul64(in [][]float64) []float64 {
	out := make([]float64, 3*2)
	for i := 0; i < 3; i++ {
		for j := 0; j < 2; j++ {
			for k := 0; k < 4; k++ {
				out[i*2+j] += in[0][i*4+k] * in[1][k*2+j]
			}
		}
	}
	return out
}

func extreme64(x []float64, better func(a, b float64) bool) float64 {
	m := x[0]
	for _, v := range x[1:] {
		if better(v, m) {
			m = v
		}
	}
	return m
}

// indexTensor tam sayı indekslerden tensör kurar (yerleşikler indeksi tensör olarak da okur).
func indexTensor(idx []int, shape ...int) *tensor.Tensor {
	d := make([]float32, len(idx))
	for i, v := range idx {
		d[i] = float32(v)
	}
	return tensor.New(d, shape...)
}

// listSum sumTensorList'i iki tensör ve bir sayı (0.5) içeren listeyle çağırır.
func listSum(ad string, mean bool, xs []*tensor.Tensor) (*tensor.Tensor, error) {
	v, err := sumTensorList(nil, ad, mean, []interp.Value{tv(xs[0]), tv(xs[1]), &interp.FloatVal{Val: 0.5}})
	if err != nil {
		return nil, err
	}
	return v.(*TensorValue).t, nil
}

// compositeSpecs bileşik yerleşiklerin gradyan kayıtlarıdır. bf16Tol değerleri
// TestMeasureCompositeBF16 ölçümünden (32 tohum) gelir.
func compositeSpecs() []compositeSpec {
	ceTargets := []int{2, -1, 0, 3, -1, 1}
	axis1 := &interp.IntVal{Val: 1}
	listRef := func(div float64) func(in [][]float64) []float64 {
		return func(in [][]float64) []float64 {
			out := make([]float64, 6)
			for i := range out {
				out[i] = (in[0][i] + in[1][i%3] + 0.5) / div
			}
			return out
		}
	}
	call := func(kd codes.Code) func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) {
		return func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) {
			args := make([]interp.Value, len(xs))
			for i, x := range xs {
				args[i] = tv(x)
			}
			return callBuiltin(m, kd, args...)
		}
	}
	withAxis := func(kd codes.Code) func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) {
		return func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) {
			return callBuiltin(m, kd, tv(xs[0]), axis1)
		}
	}
	return []compositeSpec{
		{name: "softmax", funcs: []string{"registerSoftmax"}, shapes: [][]int{{3, 4}}, bf16Tol: 0.004,
			run: withAxis(codes.T210),
			ref: func(in [][]float64) []float64 { return rows64(in[0], 4, softmax64) }},
		{name: "log_softmax", funcs: []string{"registerSoftmax"}, shapes: [][]int{{3, 4}}, bf16Tol: 0.004,
			run: withAxis(codes.T211),
			ref: func(in [][]float64) []float64 { return rows64(in[0], 4, logSoftmax64) }},
		{name: "layer_norm", funcs: []string{"layerNorm"}, shapes: [][]int{{3, 5}, {5}, {5}}, bf16Tol: 0.004,
			run: call(codes.T501), ref: layerNorm64},
		{name: "rms_norm", funcs: []string{"rmsNorm"}, shapes: [][]int{{3, 5}, {5}}, bf16Tol: 0.004,
			run: call(codes.T502), ref: rmsNorm64},
		{name: "cross_entropy", funcs: []string{"crossEntropy"}, shapes: [][]int{{2, 3, 4}}, bf16Tol: 0.004,
			run: func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) {
				return callBuiltin(m, codes.T510, tv(xs[0]), tv(indexTensor(ceTargets, 2, 3)))
			},
			ref: func(in [][]float64) []float64 {
				var loss float64
				valid := 0
				for r, tg := range ceTargets {
					if tg < 0 {
						continue
					}
					lp := make([]float64, 4)
					logSoftmax64(in[0][r*4:r*4+4], lp)
					loss -= lp[tg]
					valid++
				}
				return []float64{loss / float64(valid)}
			}},
		{name: "embedding", funcs: []string{"embedding"}, shapes: [][]int{{4, 3}}, bf16Tol: 0.004,
			run: func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) {
				return callBuiltin(m, codes.T500, tv(xs[0]), tv(indexTensor([]int{1, 3, 1, 0}, 2, 2)))
			},
			ref: func(in [][]float64) []float64 {
				var out []float64
				for _, ix := range []int{1, 3, 1, 0} {
					out = append(out, in[0][ix*3:ix*3+3]...)
				}
				return out
			}},
		{name: "attention", funcs: []string{"attentionBuiltin"}, shapes: [][]int{{3, 2, 4}, {3, 2, 4}, {3, 2, 4}}, bf16Tol: 0.038,
			run: call(codes.T503), ref: attention64},
		{name: "matmul", funcs: []string{"matmulDType"}, shapes: [][]int{{3, 4}, {4, 2}}, bf16Tol: 0.008,
			run: func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) {
				return matmulDType(xs[0], xs[1], mixedDType(m))
			}, ref: matmul64},
		{name: "list_sum", funcs: []string{"sumTensorList"}, shapes: [][]int{{2, 3}, {3}}, bf16Tol: 0.009,
			run: func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) { return listSum("sum", false, xs) },
			ref: listRef(1)},
		{name: "list_mean", funcs: []string{"sumTensorList"}, shapes: [][]int{{2, 3}, {3}}, bf16Tol: 0.018,
			run: func(m *interp.VM, xs []*tensor.Tensor) (*tensor.Tensor, error) { return listSum("mean", true, xs) },
			ref: listRef(3)},
		{name: "sum_axis", funcs: []string{"reduction"}, shapes: [][]int{{3, 4}}, bf16Tol: 0.004,
			run: withAxis(codes.G400),
			ref: func(in [][]float64) []float64 { return rows64Sum(in[0], 4) }},
		{name: "mean_axis", funcs: []string{"reduction"}, shapes: [][]int{{3, 4}}, bf16Tol: 0.004,
			run: withAxis(codes.G410),
			ref: func(in [][]float64) []float64 {
				s := rows64Sum(in[0], 4)
				for i := range s {
					s[i] /= 4
				}
				return s
			}},
		{name: "min_all", funcs: []string{"minAll"}, shapes: [][]int{{3, 4}}, bf16Tol: 0.004,
			run: call(codes.G401),
			ref: func(in [][]float64) []float64 {
				return []float64{extreme64(in[0], func(a, b float64) bool { return a < b })}
			}},
		{name: "max_all", funcs: []string{"reduction"}, shapes: [][]int{{3, 4}}, bf16Tol: 0.004,
			run: call(codes.G402),
			ref: func(in [][]float64) []float64 {
				return []float64{extreme64(in[0], func(a, b float64) bool { return a > b })}
			}},
	}
}

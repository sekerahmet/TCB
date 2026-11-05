package tensor

// Fourier dönüşümünün türevi (23.1, 23.3). Dönüşüm doğrusaldır: y[j] = Σ_k x[k]·e^(−iθ_jk) için
// ∂L/∂x[k] = Re Σ_j (gr[j] + i·gi[j])·e^(iθ_jk), yani gradyanın eşleniğinin ileri dönüşümünün gerçek
// kısmıdır. Yalnız verilen n/2+1 kutu toplanır: negatif frekanslar çıktıda yoktur, çift sayılmaz.
// Hesap float32'dedir; float64 cinsli yoldadır, türev izlemez (15.48). Toplama sırası sabittir: bir
// satırın çerçeveleri sırayla eklenir, satırlar birbirinden bağımsızdır.

// conjugateFrame gradyanın eşleniğini w.xr, w.xi'ye yazar ve dönüştürür: sonuç w.yr'dedir.
func conjugateFrame(p *fftPlan[float32], w *realWork[float32], gr, gi []float32, base, step, bins int) {
	clear(w.xr)
	clear(w.xi)
	for b := range bins {
		w.xr[b] = gr[base+b*step]
		w.xi[b] = -gi[base+b*step]
	}
	p.transform(w.xr, w.xi, w.yr, w.yi, w.s)
}

// trackRfft sonucu türev çizgesine bağlar.
func trackRfft(t, re, im *Tensor, n int, tr Trig) (*Tensor, *Tensor) {
	if t.dtyp == Float64 || !GradEnabled() || !t.RequiresGrad() {
		return re, im
	}
	shape := copyInts(t.shape)
	outs := CustomOp("rfft", []*Tensor{t}, []*Tensor{re, im}, nil, func(_ any, g []*Tensor) []*Tensor {
		return []*Tensor{rfftGrad(shape, n, tr, g[0], g[1])}
	})
	return outs[0], outs[1]
}

// rfftGrad girdinin gradyanıdır.
func rfftGrad(shape []int, n int, tr Trig, gre, gim *Tensor) *Tensor {
	last := shape[len(shape)-1]
	rows := 1
	for _, d := range shape[:len(shape)-1] {
		rows *= d
	}
	bins := n/2 + 1
	gr, gi := gre.Data(), gim.Data()
	dx := make([]float32, rows*last)
	plan := newPlan[float32](tr, n)
	parallelFor(rows, n*fourierCost, func(lo, hi int) {
		w := plan.newRealWork()
		for r := lo; r < hi; r++ {
			conjugateFrame(plan, w, gr, gi, r*bins, 1, bins)
			copy(dx[r*last:(r+1)*last], w.yr[:last])
		}
	})
	return NewOn(CPU, dx, shape...)
}

// trackStft sonucu türev çizgesine bağlar: sinyalin ve pencerenin gradyanı.
func trackStft(x, window, re, im *Tensor, o StftOptions, tr Trig) (*Tensor, *Tensor) {
	wantX, wantW := x.RequiresGrad(), window.RequiresGrad()
	if x.dtyp == Float64 || !GradEnabled() || !wantX && !wantW {
		return re, im
	}
	sx, sw := x.Detach(), window.Detach() // geçici işlenen bu çağrıdan sonra bırakılır
	outs := CustomOp("stft", []*Tensor{x, window}, []*Tensor{re, im}, []*Tensor{sx, sw}, func(_ any, g []*Tensor) []*Tensor {
		defer dropAll(sx, sw)
		return stftGrad(sx, sw, g[0], g[1], o, tr, wantX, wantW)
	})
	return outs[0], outs[1]
}

// stftGrad sinyalin ve pencerenin gradyanlarıdır.
func stftGrad(x, window, gre, gim *Tensor, o StftOptions, tr Trig, wantX, wantW bool) []*Tensor {
	last := x.shape[len(x.shape)-1]
	rows := 1
	for _, d := range x.shape[:len(x.shape)-1] {
		rows *= d
	}
	_, frames := StftFrames(last, o)
	bins := o.N/2 + 1
	sig, win := signalOf[float32](x), signalOf[float32](window)
	gr, gi := gre.Data(), gim.Data()
	dx, dwRows := make([]float32, rows*last), make([]float32, rows*o.N)
	plan := newPlan[float32](tr, o.N)
	parallelFor(rows, frames*o.N*fourierCost, func(lo, hi int) {
		w := plan.newRealWork()
		for r := lo; r < hi; r++ {
			row, drow, dw := sig[r*last:(r+1)*last], dx[r*last:(r+1)*last], dwRows[r*o.N:(r+1)*o.N]
			for f := range frames {
				conjugateFrame(plan, w, gr, gi, r*bins*frames+f, frames, bins)
				stftFrameGrad(w.yr, row, drow, dw, win, f*o.Hop-centerOffset(o), o.Reflect)
			}
		}
	})
	res := make([]*Tensor, 2)
	if wantX {
		res[0] = NewOn(CPU, dx, x.shape...)
	}
	if wantW {
		dw := make([]float32, o.N)
		for r := range rows { // satır sırası sabittir
			for i := range o.N {
				dw[i] += dwRows[r*o.N+i]
			}
		}
		res[1] = NewOn(CPU, dw, o.N)
	}
	return res
}

// stftFrameGrad bir çerçevenin gradyanını sinyale ve pencereye ekler.
func stftFrameGrad(d, row, drow, dw, win []float32, start int, reflect bool) {
	for i, g := range d[:len(win)] {
		j, inside := paddedIndex(len(row), start+i, reflect)
		if !inside {
			continue
		}
		drow[j] += float32(g * win[i])
		dw[i] += float32(g * row[j])
	}
}

package tensor

// Fourier dönüşümünün türevi (23.1, 23.3). Dönüşüm doğrusaldır: y[j] = Σ_k x[k]·e^(−iθ_jk) için
// ∂L/∂x[k] = Re Σ_j (gr[j] + i·gi[j])·e^(iθ_jk), yani gradyanın eşleniğinin ileri dönüşümünün gerçek
// kısmıdır. Yalnız verilen n/2+1 kutu toplanır: negatif frekanslar çıktıda yoktur, çift sayılmaz.
// Hesap float32'dedir; float64 sinyal türev izlemez (15.48); türevi izlenen pencereyle H6040'tır (Stft). Toplama sırası sabittir: bir
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

// Ters dönüşümün türevi (23.7, 23.8): y[k] = Σ_j a_j·(re[j]·cos θ_jk − im[j]·sin θ_jk) için (a_0 ve çift n'de
// a_{n/2} 1/n, öbürleri 2/n) ∂L/∂re[j] ve ∂L/∂im[j] gradyanın ileri dönüşümünün a_j katıdır: gerçek ve
// sanal kısmı. Sıfırıncı ve çift n'de son kutunun sanal kısmı sonuca girmez, türevi 0'dır.

// irfftFrameGrad w.xr'deki n gradyanı kutuların gradyanına çevirir (base'den step adımla); nil olan
// yazılmaz.
func irfftFrameGrad(p *fftPlan[float32], w *realWork[float32], dre, dim []float32, base, step int) {
	clear(w.xi)
	p.real(w)
	n := float32(p.n)
	for j := 0; 2*j <= p.n; j++ {
		edge := j == 0 || 2*j == p.n
		scale := float32(2)
		if edge {
			scale = 1
		}
		if dre != nil {
			dre[base+j*step] = scale * w.yr[j] / n
		}
		if dim != nil && !edge {
			dim[base+j*step] = scale * w.yi[j] / n
		}
	}
}

// gradBuffer istenen gradyanın tamponudur; istenmezse nil.
func gradBuffer(want bool, n int) []float32 {
	if !want {
		return nil
	}
	return make([]float32, n)
}

// gradTensor tampondan CPU tensörü kurar; tampon nil'se nil.
func gradTensor(d []float32, shape ...int) *Tensor {
	if d == nil {
		return nil
	}
	return NewOn(CPU, d, shape...)
}

// trackIrfft sonucu türev çizgesine bağlar.
func trackIrfft(re, im, y *Tensor, n int, tr Trig) *Tensor {
	wantRe, wantIm := re.RequiresGrad(), im.RequiresGrad()
	if y.dtyp == Float64 || !GradEnabled() || !wantRe && !wantIm {
		return y
	}
	shape, rows := copyInts(re.shape), rowsOf(re)
	return CustomOp("irfft", []*Tensor{re, im}, []*Tensor{y}, nil, func(_ any, g []*Tensor) []*Tensor {
		bins := n/2 + 1
		gy := g[0].Data()
		dre, dim := gradBuffer(wantRe, rows*bins), gradBuffer(wantIm, rows*bins)
		plan := newPlan[float32](tr, n)
		parallelFor(rows, n*fourierCost, func(lo, hi int) {
			w := plan.newRealWork()
			for r := lo; r < hi; r++ {
				copy(w.xr, gy[r*n:(r+1)*n])
				irfftFrameGrad(plan, w, dre, dim, r*bins, 1)
			}
		})
		return []*Tensor{gradTensor(dre, shape...), gradTensor(dim, shape...)}
	})[0]
}

// trackIstft sonucu türev çizgesine bağlar: gerçek, sanal ve pencerenin gradyanı.
func trackIstft(re, im, window, y *Tensor, o IstftOptions, c FourierCall) *Tensor {
	want := [3]bool{re.RequiresGrad(), im.RequiresGrad(), window.RequiresGrad()}
	if y.dtyp == Float64 || !GradEnabled() || want == [3]bool{} {
		return y
	}
	saved := []*Tensor{re.Detach(), im.Detach(), window.Detach(), y.Detach()} // bu çağrıdan sonra bırakılır
	return CustomOp("istft", []*Tensor{re, im, window}, []*Tensor{y}, saved, func(_ any, g []*Tensor) []*Tensor {
		defer dropAll(saved...)
		return istftGrad(saved[0], saved[1], saved[2], saved[3], g[0], o, c, want)
	})[0]
}

// istftGrad gerçek, sanal ve pencerenin gradyanlarıdır. q = gy / zarf için çerçevenin gradyanı q · pencere,
// pencereninki Σ q · (çerçeve − 2 · y · pencere)'dir (bölümün türevi dahil).
func istftGrad(re, im, window, y, gy *Tensor, o IstftOptions, c FourierCall, want [3]bool) []*Tensor {
	rows, bins, frames, _ := istftShape(re, o)
	n, size := o.N, rows*bins*frames
	dre, dim := gradBuffer(want[0], size), gradBuffer(want[1], size)
	dwRows := gradBuffer(want[2], rows*n)
	if rows > 0 {
		g := istftBack{o: o, frames: frames, bins: bins, win: signalOf[float32](window), y: y.Data(), gy: gy.Data(), dre: dre, dim: dim, dwRows: dwRows}
		g.env = istftEnvelope(g.win, frames, o, c.Fn)
		if want[2] {
			g.xr, g.xi = signalOf[float32](re), signalOf[float32](im)
		}
		plan := newPlan[float32](c.Trig, n)
		parallelFor(rows, frames*n*fourierCost, func(from, to int) {
			w, q := plan.newRealWork(), make([]float32, o.Length)
			for r := from; r < to; r++ {
				g.row(plan, w, q, r)
			}
		})
	}
	dw := gradBuffer(want[2], n)
	for i := range dwRows { // satır sırası sabittir
		dw[i%n] += dwRows[i]
	}
	return []*Tensor{gradTensor(dre, re.shape...), gradTensor(dim, re.shape...), gradTensor(dw, n)}
}

// istftBack ters STFT'nin geri hesabının paylaşılan verisidir; nil olan gradyan yazılmaz.
type istftBack struct {
	o                IstftOptions
	frames, bins     int
	xr, xi, win, env []float32
	y, gy            []float32
	dre, dim, dwRows []float32
}

// row r'inci satırın gradyanlarını yazar; q işçinin alanıdır.
func (g *istftBack) row(p *fftPlan[float32], w *realWork[float32], q []float32, r int) {
	o, n := g.o, g.o.N
	for i := range q {
		q[i] = g.gy[r*o.Length+i] / g.env[i]
	}
	for f := range g.frames {
		at, lo, hi := o.overlap(f)
		base := r*g.bins*g.frames + f
		if g.dwRows != nil {
			hermitianFrame(p, w, g.xr, g.xi, base, g.frames)
			dw := g.dwRows[r*n : (r+1)*n]
			for i := lo; i < hi; i++ {
				rest := float32(w.yr[i]/float32(n)) - float32(2*float32(g.y[r*o.Length+at+i]*g.win[i]))
				dw[i] += float32(q[at+i] * rest)
			}
		}
		if g.dre != nil || g.dim != nil {
			clear(w.xr)
			for i := lo; i < hi; i++ {
				w.xr[i] = q[at+i] * g.win[i]
			}
			irfftFrameGrad(p, w, g.dre, g.dim, base, g.frames)
		}
	}
}

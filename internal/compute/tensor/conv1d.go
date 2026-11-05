package tensor

// Bir boyutlu evrişim (15.23c). Girdi pencereleri bir kez satırlara açılır, çarpımı `@` yapar:
// hesap, biriktirme ve türev onunla aynıdır. Açma CPU'dadır; GPU'daki tensör çağıranda hata verir,
// CPU'ya düşmez (25.9). Açmanın türevi her girdi satırını sabit sırayla toplar (15.22a).

// convCost bir öğenin yaklaşık maliyetidir (işçi sayısı için).
const convCost = 4

// Conv1dOptions TR_conv1d'in ayarlarıdır.
type Conv1dOptions struct {
	Stride, Padding, Dilation, Groups int
}

// Span çekirdeğin girdide kapladığı uzunluktur.
func (o Conv1dOptions) Span(k int) int { return o.Dilation*(k-1) + 1 }

// Length çıkışın uzunluğudur; çekirdek dolgulu girdiye sığmalıdır.
func (o Conv1dOptions) Length(l, k int) int { return (l+2*o.Padding-o.Span(k))/o.Stride + 1 }

// Conv1d x'in [B, Cin, L] w'nin [Cout, Cin / g, K] ile evrişimidir; b nil olabilir. mm çarpımı yapar:
// çağıran `@`'ın kuralını verir (15.4).
func Conv1d(x, w, b *Tensor, o Conv1dOptions, mm func(a, b *Tensor) *Tensor) *Tensor {
	batch, g := x.shape[0], o.Groups
	cout, inner := w.shape[0], w.shape[1]*w.shape[2]
	frames := o.Length(x.shape[2], w.shape[2])
	cols := unfold1d(x, w.shape[2], frames, o)
	y := mm(cols, w.Reshape(g, cout/g, inner).Transpose(1, 2))
	r := y.Reshape(g, batch, frames, cout/g).Permute(1, 0, 3, 2).Contiguous().Reshape(batch, cout, frames)
	if b == nil {
		return r
	}
	if b.dtyp != r.dtyp {
		b = b.CastShared(r.dtyp)
	}
	return r.Add(b.Reshape(1, cout, 1))
}

// foldShape açılmış satırların düzenidir: girdiyle aynı adımla okunur ve türevde geri toplanır.
type foldShape struct {
	batch, cin, l int // girdi
	k, frames     int // çekirdek ve çıkış uzunluğu
	o             Conv1dOptions
}

// cg bir gruptaki kanal sayısıdır.
func (f foldShape) cg() int { return f.cin / f.o.Groups }

// inside p konumu dolgunun dışında, girdinin içindeyse doğrudur.
func (f foldShape) inside(p int) bool { return p >= 0 && p < f.l }

// unfold1d x'in pencerelerini [g, B · çıkış, Cin / g · K] satırlarına açar.
func unfold1d(x *Tensor, k, frames int, o Conv1dOptions) *Tensor {
	f := foldShape{batch: x.shape[0], cin: x.shape[1], l: x.shape[2], k: k, frames: frames, o: o}
	shape := []int{o.Groups, f.batch * frames, f.cg() * k}
	allocSizeOn(x.be, isCreate, shape, Float32)
	cols := openRows(x.Detach().Data(), f)
	out := NewOn(CPU, cols, shape...)
	if x.dtyp != Float32 {
		out = out.CastShared(x.dtyp)
	}
	if !GradEnabled() || !x.RequiresGrad() {
		return out
	}
	dt := x.dtyp
	return CustomOp("unfold1d", []*Tensor{x}, []*Tensor{out}, nil, func(_ any, g []*Tensor) []*Tensor {
		dx := NewOn(CPU, foldRows(g[0].Data(), f), f.batch, f.cin, f.l)
		if dt != Float32 {
			dx = dx.CastShared(dt)
		}
		return []*Tensor{dx}
	})[0]
}

// openRows satırları kurar; dolgu sıfırdır. Her satır yalnız kendi yerine yazar.
func openRows(src []float32, f foldShape) []float32 {
	cg, width := f.cg(), f.cg()*f.k
	rows := f.o.Groups * f.batch * f.frames
	cols := make([]float32, rows*width)
	parallelFor(rows, width*convCost, func(lo, hi int) {
		for u := lo; u < hi; u++ {
			gi, rest := u/(f.batch*f.frames), u%(f.batch*f.frames)
			b, start := rest/f.frames, rest%f.frames*f.o.Stride-f.o.Padding
			dst := cols[u*width : (u+1)*width]
			for c := range cg {
				row := src[(b*f.cin+gi*cg+c)*f.l : (b*f.cin+gi*cg+c+1)*f.l]
				for j := range f.k {
					if p := start + j*f.o.Dilation; f.inside(p) {
						dst[c*f.k+j] = row[p]
					}
				}
			}
		}
	})
	return cols
}

// foldRows açmanın türevidir: her girdi satırı kendi konumlarını çıkış sırasıyla toplar.
func foldRows(grad []float32, f foldShape) []float32 {
	cg, width := f.cg(), f.cg()*f.k
	dx := make([]float32, f.batch*f.cin*f.l)
	parallelFor(f.batch*f.cin, f.frames*f.k*convCost, func(lo, hi int) {
		for r := lo; r < hi; r++ {
			b, ci := r/f.cin, r%f.cin
			gi, c := ci/cg, ci%cg
			row := dx[r*f.l : (r+1)*f.l]
			for t := range f.frames {
				at := ((gi*f.batch+b)*f.frames+t)*width + c*f.k
				start := t*f.o.Stride - f.o.Padding
				for j := range f.k {
					if p := start + j*f.o.Dilation; f.inside(p) {
						row[p] += grad[at+j]
					}
				}
			}
		}
	})
	return dx
}

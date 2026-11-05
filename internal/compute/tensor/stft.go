package tensor

import (
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Fourier dönüşümü, pencere ve STFT (23.1–23.3). Ağırlıklı toplamlar CPU'dadır; GPU'daki tensör
// hata verir, CPU'ya düşmez (23.5). Sonuç (gerçek, sanal) iki ondalık tensördür.

// fourierCost bir satırın yaklaşık öğe maliyetidir (işçi sayısı için).
const fourierCost = 64

// FourierCall çağıranın verdikleridir: işlevin adı, cihaz iletisinin önerisi ve kökler.
type FourierCall struct {
	Fn   any
	Move string
	Trig Trig
}

// checkDevice girdinin CPU'da olmasını ister: GPU'da hata verir, CPU'ya düşmez (23.5).
func (c FourierCall) checkDevice(t *Tensor) {
	if t.Device() != CPU {
		panic(messages.New(messages.H7076, c.Fn, t.Device().String(), "gpu", c.Move))
	}
}

// signalOf girdinin satır sırasıyla öğeleridir; 16 bit float32'ye açılır.
func signalOf[T floatType](t *Tensor) []T {
	var zero T
	if _, wide := any(zero).(float64); wide {
		return any(t.Float64s()).([]T)
	}
	return any(t.Data()).([]T)
}

// fromSignal hesap türünden CPU tensörü kurar.
func fromSignal[T floatType](data []T, shape []int) *Tensor {
	switch d := any(data).(type) {
	case []float64:
		return Float64sOn(CPU, d, shape...)
	case []float32:
		return NewOn(CPU, d, shape...)
	}
	return nil
}

// Rfft t'nin son ekseni boyunca n uzunluklu Fourier dönüşümüdür (23.1): kısa sinyal sıfırla dolar.
func Rfft(t *Tensor, n int, c FourierCall) (*Tensor, *Tensor) {
	c.checkDevice(t)
	if t.dtyp == Float64 {
		return rfft[float64](t, n, c.Trig)
	}
	return rfft[float32](t, n, c.Trig)
}

func rfft[T floatType](t *Tensor, n int, tr Trig) (*Tensor, *Tensor) {
	last := t.shape[len(t.shape)-1]
	rows := 1
	for _, d := range t.shape[:len(t.shape)-1] {
		rows *= d
	}
	bins := n/2 + 1
	shape := append(copyInts(t.shape[:len(t.shape)-1]), bins)
	allocSizeOn(t.be, isCreate, shape, Float32) // sınır aşılıyorsa önce iletisi gelsin
	x := signalOf[T](t)
	re, im := make([]T, rows*bins), make([]T, rows*bins)
	plan := newPlan[T](tr, n)
	parallelFor(rows, n*fourierCost, func(lo, hi int) {
		w := plan.newRealWork()
		for r := lo; r < hi; r++ {
			copy(w.xr, x[r*last:(r+1)*last])
			clear(w.xr[last:])
			clear(w.xi)
			plan.real(w)
			copy(re[r*bins:(r+1)*bins], w.yr[:bins])
			copy(im[r*bins:(r+1)*bins], w.yi[:bins])
		}
	})
	return fromSignal(re, shape), fromSignal(im, shape)
}

// HannWindow n uzunlukta Hann penceresidir (23.2): hesap float64'te, sonuç float32'dir.
func HannWindow(n int, periodic bool, tr Trig) *Tensor {
	d := n
	if !periodic {
		d = n - 1
	}
	out := make([]float32, n)
	for k := range out {
		if n == 1 {
			out[k] = 1
			continue
		}
		c, _ := unitRoot(tr, k, d)
		out[k] = float32(0.5 - 0.5*c)
	}
	return NewOn(CPU, out, n)
}

// StftOptions TR_stft'nin ayarlarıdır (23.3).
type StftOptions struct {
	N, Hop  int
	Center  bool
	Reflect bool
}

// StftFrames dolgulu uzunluk ve çerçeve sayısıdır.
func StftFrames(signal int, o StftOptions) (padded, frames int) {
	padded = signal
	if o.Center {
		padded += 2 * (o.N / 2)
	}
	return padded, 1 + (padded-o.N)/o.Hop
}

// Stft x'in son ekseninin kısa süreli dönüşümüdür; şekil [..., n/2+1, çerçeve]'dir.
func Stft(x, window *Tensor, o StftOptions, c FourierCall) (*Tensor, *Tensor) {
	c.checkDevice(x)
	c.checkDevice(window)
	if x.dtyp == Float64 {
		return stft[float64](x, window, o, c.Trig)
	}
	return stft[float32](x, window, o, c.Trig)
}

func stft[T floatType](x, window *Tensor, o StftOptions, tr Trig) (*Tensor, *Tensor) {
	last := x.shape[len(x.shape)-1]
	rows := 1
	for _, d := range x.shape[:len(x.shape)-1] {
		rows *= d
	}
	_, frames := StftFrames(last, o)
	bins := o.N/2 + 1
	shape := append(copyInts(x.shape[:len(x.shape)-1]), bins, frames)
	allocSizeOn(x.be, isCreate, shape, Float32)
	sig, win := signalOf[T](x), signalOf[T](window)
	re, im := make([]T, rows*bins*frames), make([]T, rows*bins*frames)
	plan := newPlan[T](tr, o.N)
	half := 0
	if o.Center {
		half = o.N / 2
	}
	parallelFor(rows*frames, o.N*fourierCost, func(lo, hi int) {
		w := plan.newRealWork()
		for u := lo; u < hi; u++ {
			r, f := u/frames, u%frames
			row := sig[r*last : (r+1)*last]
			for i := range o.N {
				w.xr[i] = T(paddedAt(row, f*o.Hop+i-half, o.Reflect) * win[i])
			}
			clear(w.xi)
			plan.real(w)
			base := r * bins * frames
			for b := range bins {
				re[base+b*frames+f], im[base+b*frames+f] = w.yr[b], w.yi[b]
			}
		}
	})
	return fromSignal(re, shape), fromSignal(im, shape)
}

// paddedAt dolgulu sinyalin i konumudur: dışarısı yansıtmada aynadan, öbüründe sıfırdır.
func paddedAt[T floatType](row []T, i int, reflect bool) T {
	n := len(row)
	if i >= 0 && i < n {
		return row[i]
	}
	if !reflect {
		return 0
	}
	if i < 0 {
		return row[-i]
	}
	return row[2*(n-1)-i]
}

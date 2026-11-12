package tensor

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kartta Fourier dönüşümü (23.1, 23.3, 23.5): taban matrisli DFT. Dönüşüm var olan türevli tensör
// işlemlerinden kurulur: çerçeveler satır seçmedir (TakeRows; yansıtma ve sıfır dolgu konum
// tablosunda), sonra pencereyle çarpım ve [satır, n] @ B[n, 2F]. Çarpım MatMul'dandır, bloğu
// izleyen MatMulVendor'dan değil: ileri ve geri yönde TR'nin kendi çekirdeği (23.5, 15.53b).
// Türev bu işlemlerin kendi türevidir (gömmenin sabit sıralı türevi, matMulGrad).

// FourierCardLimit kartta dönüşümün en büyük uzunluğudur (15.41'in istisnası, 23.5): taban
// [n, 2F] float32'dir, n = 4096'da 67 MB. Üstü H4134'tür.
const FourierCardLimit = 4096

// fourierCacheBytes taban önbelleğinin sınırıdır: dolunca en eski taban bırakılır.
var fourierCacheBytes = 128 << 20

// basisKey bir tabanın anahtarıdır: uzunluk ve cihaz (pencere tabana katılmaz).
type basisKey struct {
	n   int
	dev Device
}

// fourierBases kurulmuş tabanlardır; order eklenme sırasıdır (en eskisi önde).
var fourierBases = struct {
	sync.Mutex
	m     map[basisKey]*Tensor
	order []basisKey
	bytes int
}{m: map[basisKey]*Tensor{}}

// fourierBasis n uzunluğun dev'deki tabanıdır: B[k, j] = cos, B[k, F+j] = e^(−2πi·jk/n)'nin sanal
// kısmı (j < F = n/2+1). Konakta float64'te unitRoot ile kurulur, float32'ye bir kez yuvarlanır.
func fourierBasis(n int, dev Device, tr Trig) *Tensor {
	key := basisKey{n, dev}
	fourierBases.Lock()
	defer fourierBases.Unlock()
	if b, ok := fourierBases.m[key]; ok {
		return b
	}
	F := n/2 + 1
	data := make([]float32, n*2*F)
	for k := range n {
		row := data[k*2*F : (k+1)*2*F]
		for j := range F {
			c, s := unitRoot(tr, j*k%n, n)
			row[j], row[F+j] = float32(c), float32(s)
		}
	}
	size := 4 * len(data)
	for fourierBases.bytes+size > fourierCacheBytes && len(fourierBases.order) > 0 {
		old := fourierBases.order[0]
		fourierBases.order = fourierBases.order[1:]
		fourierBases.bytes -= 4 * fourierBases.m[old].Numel()
		delete(fourierBases.m, old)
	}
	b := NewOn(dev, data, n, 2*F)
	fourierBases.m[key] = b
	fourierBases.order = append(fourierBases.order, key)
	fourierBases.bytes += size
	return b
}

// onCard girdinin kartta dönüşeceğini söyler: CPU'da yanlış; kartta float64 yoktur (H7076, CPU'ya
// düşmez), n sınırın üstündeyse H4134.
func (c FourierCall) onCard(t *Tensor, n int) bool {
	switch {
	case t.Device() == CPU:
		return false
	case t.dtyp == Float64:
		panic(messages.New(messages.H7076, c.Fn, t.Device().String(), "gpu", c.Move))
	case n > FourierCardLimit:
		panic(messages.New(messages.H4134, c.Fn, n, FourierCardLimit))
	}
	return true
}

// rowsOf t'nin son ekseni dışındaki öğe sayısıdır.
func rowsOf(t *Tensor) int {
	rows := 1
	for _, d := range t.shape[:len(t.shape)-1] {
		rows *= d
	}
	return rows
}

// halves y'nin [..., 2F] son eksenini (gerçek, sanal) iki bitişik tensöre böler.
func halves(y *Tensor, F int) (*Tensor, *Tensor) {
	ax := len(y.shape) - 1
	return y.Slice(ax, 0, F).Contiguous(), y.Slice(ax, F, 2*F).Contiguous()
}

// rfftOnCard Rfft'in kart yoludur: son eksen n'ye sıfırla doldurulur (çağıran uzun sinyali reddeder).
func rfftOnCard(t *Tensor, n int, tr Trig) (*Tensor, *Tensor) {
	last, rows := t.shape[len(t.shape)-1], rowsOf(t)
	F := n/2 + 1
	x := t.Cast(Float32).Contiguous().Reshape(rows, last)
	if last < n {
		x = Concat([]*Tensor{x, ZerosOn(t.Device(), rows, n-last)}, 1)
	}
	re, im := halves(MatMul(x, fourierBasis(n, t.Device(), tr)), F)
	shape := append(copyInts(t.shape[:len(t.shape)-1]), F)
	return re.Reshape(shape...), im.Reshape(shape...)
}

// frameIndex çerçevelerin dolgulu sinyaldeki konum tablosudur: [çerçeve·n]; sıfır dolgu sinyalin
// sonundaki sıfır satırını (last) gösterir.
func frameIndex(last, frames int, o StftOptions) []int {
	half := centerOffset(o)
	idx := make([]int, frames*o.N)
	for f := range frames {
		for i := range o.N {
			j, inside := paddedIndex(last, f*o.Hop+i-half, o.Reflect)
			if !inside {
				j = last
			}
			idx[f*o.N+i] = j
		}
	}
	return idx
}

// stftOnCard Stft'in kart yoludur; sonuç [..., n/2+1, çerçeve]'dir.
func stftOnCard(x, window *Tensor, o StftOptions, tr Trig) (*Tensor, *Tensor) {
	last, rows := x.shape[len(x.shape)-1], rowsOf(x)
	_, frames := StftFrames(last, o)
	F := o.N/2 + 1
	dev := x.Device()
	sig := x.Cast(Float32).Contiguous().Reshape(rows, last).Permute(1, 0)
	sig = Concat([]*Tensor{sig, ZerosOn(dev, 1, rows)}, 0) // [last+1, satır]
	fr := TakeRows(sig, frameIndex(last, frames, o), []int{frames * o.N})
	fr = fr.Reshape(frames, o.N, rows).Permute(2, 0, 1) // [satır, çerçeve, n]
	fr = Mul(fr, window.Cast(Float32)).Contiguous().Reshape(rows*frames, o.N)
	y := MatMul(fr, fourierBasis(o.N, dev, tr)).Reshape(rows, frames, 2*F)
	re, im := halves(y, F)
	shape := append(copyInts(x.shape[:len(x.shape)-1]), F, frames)
	bt := func(t *Tensor) *Tensor { return t.Permute(0, 2, 1).Contiguous().Reshape(shape...) }
	return bt(re), bt(im)
}

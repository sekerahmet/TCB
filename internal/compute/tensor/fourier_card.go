package tensor

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kartta Fourier dönüşümü (23.1, 23.3, 23.5, 23.7, 23.8): taban matrisli DFT. Dönüşüm var olan türevli tensör
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

// basisCache kurulmuş tabanlardır; order eklenme sırasıdır (en eskisi önde).
type basisCache struct {
	sync.Mutex
	m     map[basisKey]*Tensor
	order []basisKey
	bytes int
}

// get anahtarın tabanıdır; yoksa fill'in doldurduğu [rows, cols] veriyle kurulur.
func (c *basisCache) get(key basisKey, rows, cols int, fill func(data []float32)) *Tensor {
	c.Lock()
	defer c.Unlock()
	if b, ok := c.m[key]; ok {
		return b
	}
	data := make([]float32, rows*cols)
	fill(data)
	size := 4 * len(data)
	for c.bytes+size > fourierCacheBytes && len(c.order) > 0 {
		old := c.order[0]
		c.order = c.order[1:]
		c.bytes -= 4 * c.m[old].Numel()
		delete(c.m, old)
	}
	b := NewOn(key.dev, data, rows, cols)
	c.m[key] = b
	c.order = append(c.order, key)
	c.bytes += size
	return b
}

// İleri ve ters taban ayrı önbellektedir: 4096'lık ikisi tek sınıra sığmaz, birbirini düşürürdü.
var (
	fourierBases        = basisCache{m: map[basisKey]*Tensor{}}
	inverseFourierBases = basisCache{m: map[basisKey]*Tensor{}}
)

// fourierBasis n uzunluğun dev'deki tabanıdır: B[k, j] = cos, B[k, F+j] = e^(−2πi·jk/n)'nin sanal
// kısmı (j < F = n/2+1). Konakta float64'te unitRoot ile kurulur, float32'ye bir kez yuvarlanır.
func fourierBasis(n int, dev Device, tr Trig) *Tensor {
	F := n/2 + 1
	return fourierBases.get(basisKey{n, dev}, n, 2*F, func(data []float32) {
		for k := range n {
			row := data[k*2*F : (k+1)*2*F]
			for j := range F {
				c, s := unitRoot(tr, j*k%n, n)
				row[j], row[F+j] = float32(c), float32(s)
			}
		}
	})
}

// inverseFourierBasis ters dönüşümün tabanıdır: satır j gerçek kutunun, F+j sanal kutunun n örneğe
// katkısı; sıfırıncı ve çift n'de son kutuda 1/n (sanalı 0), öbürlerinde 2/n ölçeklidir.
func inverseFourierBasis(n int, dev Device, tr Trig) *Tensor {
	F := n/2 + 1
	return inverseFourierBases.get(basisKey{n, dev}, 2*F, n, func(data []float32) {
		for j := range F {
			re, im := 2/float64(n), 2/float64(n)
			if j == 0 || 2*j == n {
				re, im = 1/float64(n), 0
			}
			for k := range n {
				c, s := unitRoot(tr, j*k%n, n)
				data[j*n+k], data[(F+j)*n+k] = float32(re*c), float32(im*s)
			}
		}
	})
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

// irfftOnCard Irfft'in kart yoludur: [satır, 2F] @ ters taban.
func irfftOnCard(re, im *Tensor, n int, tr Trig) *Tensor {
	F, rows := re.shape[len(re.shape)-1], rowsOf(re)
	flat := func(t *Tensor) *Tensor { return t.Cast(Float32).Contiguous().Reshape(rows, F) }
	y := MatMul(Concat([]*Tensor{flat(re), flat(im)}, 1), inverseFourierBasis(n, re.Device(), tr))
	return y.Reshape(append(copyInts(re.shape[:len(re.shape)-1]), n)...)
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

// overlapIndex kurulan her örneğe değen (çerçeve·n + konum) satırlarının tablosudur: [boy·slots]; boş
// yuva son satırı (çerçeve·n, sıfır satırı) gösterir. slots bir örneğe değen çerçevelerin en çoğudur.
func overlapIndex(frames int, o IstftOptions) (idx []int, slots int) {
	slots = min(frames, 1+(o.N-1)/o.Hop)
	idx = make([]int, o.Length*slots)
	for i := range idx {
		idx[i] = frames * o.N
	}
	used := make([]int, o.Length)
	for f := range frames {
		at, lo, hi := o.overlap(f)
		for i := lo; i < hi; i++ {
			p := at + i
			idx[p*slots+used[p]] = f*o.N + i
			used[p]++
		}
	}
	return idx, slots
}

// overlapAdd [satır, çerçeve, n] parçaları üst üste ekler: [satır, boy].
func overlapAdd(parts *Tensor, idx []int, slots int, o IstftOptions) *Tensor {
	rows, frames := parts.shape[0], parts.shape[1]
	flat := parts.Contiguous().Reshape(rows, frames*o.N).Permute(1, 0)
	flat = Concat([]*Tensor{flat, ZerosOn(parts.Device(), 1, rows)}, 0) // [çerçeve·n + 1, satır]
	return TakeRows(flat, idx, []int{o.Length, slots}).Sum(1, false).Permute(1, 0)
}

// istftOnCard Istft'in kart yoludur; pencere karelerinin toplamı da kartta hesaplanır, türevi izlenir.
// Toplamın sıfır denetimi pencereyi konakta okur.
func istftOnCard(re, im, window *Tensor, o IstftOptions, c FourierCall) *Tensor {
	rows, F, frames, shape := istftShape(re, o)
	dev, w := re.Device(), window.Cast(Float32)
	if rows == 0 {
		return ZerosOn(dev, shape...)
	}
	istftEnvelope(w.Detach().To(CPU).Data(), frames, o, c.Fn)
	flat := func(t *Tensor) *Tensor {
		return t.Cast(Float32).Contiguous().Reshape(rows, F, frames).Permute(0, 2, 1).Contiguous().Reshape(rows*frames, F)
	}
	y := MatMul(Concat([]*Tensor{flat(re), flat(im)}, 1), inverseFourierBasis(o.N, dev, c.Trig)).Reshape(rows, frames, o.N)
	idx, slots := overlapIndex(frames, o)
	sum := overlapAdd(Mul(y, w), idx, slots, o)
	squares := Add(ZerosOn(dev, 1, frames, o.N), Mul(w, w))
	return Div(sum, overlapAdd(squares, idx, slots, o)).Contiguous().Reshape(shape...)
}

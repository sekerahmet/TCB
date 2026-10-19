package gpu

import (
	"math"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: bellek yöntemleri (Alloc, FromHost, ToHost, Copy, Fill, CopyStrided, Release)
// ve 16 bitlik türler için geçici float32 tamponlar.

func f32bit(v float32) uint32 { return math.Float32bits(v) }

// temp, bir çağrının açtığı geçici tamponları toplar; bitir hepsini havuza iade eder.
// Hesap çekirdekleri float32 bitişik görür: 16 bitlik (ya da adımlı) girdiler float32
// bitişik geçici tampona açılır, 16 bitlik hedefler float32 tamponda hesaplanıp bir kez
// yuvarlanarak geri yazılır (CPU arka ucuyla aynı düzen).
type temp struct {
	g *backend
	t []*storage
}

func (g *backend) temp() *temp { return &temp{g: g} }

func (t *temp) buffer(n int, reset bool) *storage {
	d := t.g.allocate(n, tensor.Float32, reset)
	t.t = append(t.t, d)
	return d
}

func (t *temp) finish() {
	for _, d := range t.t {
		t.g.release(d)
	}
	t.t = nil
}

// read görünümü float32 bitişik görünüme çevirir (zaten öyleyse aynen).
func (t *temp) read(v tensor.View) tensor.View {
	d := t.g.storageOf(v.S)
	if d.dtype == tensor.Float32 && v.IsContiguous() {
		return v
	}
	n := v.Numel()
	tmp := t.buffer(n, false)
	hv := tensor.View{S: tmp, Shape: v.Shape, Strides: contiguous(v.Shape)}
	t.g.copyStrided(hv, v, false, 0)
	return hv
}

// readP isteğe bağlı görünüm.
func (t *temp) readP(v *tensor.View) *tensor.View {
	if v == nil {
		return nil
	}
	w := t.read(*v)
	return &w
}

// readD düz depolamanın float32 karşılığı (nil için nil).
func (t *temp) readD(s tensor.Storage) *storage {
	if s == nil {
		return nil
	}
	d := t.g.storageOf(s)
	if d.dtype == tensor.Float32 {
		return d
	}
	return t.g.storageOf(t.read(tensor.View{S: d, Shape: []int{d.n}, Strides: []int{1}}).S)
}

// write hedefin float32 karşılığını ve geri yazma işlevini verir; eski doğruysa geçici
// tampon mevcut içerikle başlar (biriktirme), değilse sıfırdır. nil hedef nil döner.
func (t *temp) write(s tensor.Storage, old bool) (*storage, func()) {
	if s == nil {
		return nil, func() {}
	}
	d := t.g.storageOf(s)
	if d.dtype == tensor.Float32 {
		return d, func() {}
	}
	tmp := t.buffer(d.n, !old)
	plainV := func(x *storage) tensor.View { return tensor.View{S: x, Shape: []int{x.n}, Strides: []int{1}} }
	if old {
		t.g.copyStrided(plainV(tmp), plainV(d), false, 0)
	}
	return tmp, func() { t.g.copyStrided(plainV(d), plainV(tmp), false, 0) }
}

// Alloc sıfırlanmış depolama.
func (g *backend) Alloc(n int, dt tensor.DType) tensor.Storage {
	g.enter()
	defer g.exit()
	return g.allocate(n, dt, true)
}

// AllocUninit (tensor.UninitAllocator) içeriği belirsiz depolama; her elemanı yazılacak
// çıktılar içindir.
func (g *backend) AllocUninit(n int, dt tensor.DType) tensor.Storage {
	g.enter()
	defer g.exit()
	return g.allocate(n, dt, false)
}

// Release (tensor.Releaser) depolamayı havuza iade eder.
func (g *backend) Release(s tensor.Storage) {
	d, ok := s.(*storage)
	if !ok || d.g != g {
		return
	}
	g.enter()
	defer g.exit()
	if d.release2 {
		panic("tensor: depolama havuza iki kez iade edildi")
	}
	g.release(d)
}

// FromHost konak verisini dt türünde yeni depolamaya yükler. Küçük float32 depolamalar
// konak gölgesini tutar.
func (g *backend) FromHost(data []float32, dt tensor.DType) tensor.Storage {
	g.enter()
	defer g.exit()
	n := len(data)
	d := g.allocate(n, dt, false)
	if n == 0 {
		return d
	}
	if dt == tensor.Float32 {
		g.loadHost(d, unsafe.Pointer(&data[0]), 4*n)
		if n <= shadowLimit {
			d.host = append([]float32(nil), data...)
		}
		return d
	}
	gt := g.temp()
	defer gt.finish()
	tmp := gt.buffer(n, false)
	g.loadHost(tmp, unsafe.Pointer(&data[0]), 4*n)
	plain := []int{n}
	g.copyStrided(tensor.View{S: d, Shape: plain, Strides: []int{1}}, tensor.View{S: tmp, Shape: plain, Strides: []int{1}}, false, 0)
	return d
}

// ToHost görünümün elemanlarını float32 olarak konağa kopyalar. Gölgesi olan depolama
// eşzamanlama olmadan konaktan okunur.
func (g *backend) ToHost(src tensor.View) []float32 {
	g.enter()
	defer g.exit()
	d := g.storageOf(src.S)
	n := src.Numel()
	out := make([]float32, n)
	if n == 0 {
		return out
	}
	if d.host != nil {
		sumFromShadow(out, d.host, src)
		return out
	}
	gt := g.temp()
	defer gt.finish()
	v := gt.read(src)
	g.download(unsafe.Pointer(&out[0]), g.addressOf(v), 4*n)
	return out
}

// sumFromShadow görünümün elemanlarını konak gölgesinden mantıksal sırayla toplar.
func sumFromShadow(out, source []float32, v tensor.View) {
	nd := len(v.Shape)
	idx := make([]int, nd)
	for i := range out {
		o := v.Offset
		for d := 0; d < nd; d++ {
			o += idx[d] * v.Strides[d]
		}
		out[i] = source[o]
		for d := nd - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < v.Shape[d] {
				break
			}
			idx[d] = 0
		}
	}
}

// Copy src'yi dst'ye dstOff'tan başlayarak bitişik yazar (türler farklıysa dönüştürür).
func (g *backend) Copy(dst tensor.Storage, dstOff int, src tensor.View) {
	g.enter()
	defer g.exit()
	hv := tensor.View{S: dst, Offset: dstOff, Shape: src.Shape, Strides: contiguous(src.Shape)}
	g.copyStrided(hv, src, false, 0)
}

// CopyStrided src'yi dst görünümüne yazar (src yayınlanmış olabilir).
func (g *backend) CopyStrided(dst, src tensor.View) {
	g.enter()
	defer g.exit()
	g.copyStrided(dst, src, false, 0)
}

// Fill bütün elemanları v yapar.
func (g *backend) Fill(dst tensor.Storage, v float32) {
	g.enter()
	defer g.exit()
	g.fill(g.storageOf(dst), v)
}

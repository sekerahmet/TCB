package gpu

import (
	"math"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: bellek yöntemleri (Alloc, FromHost, ToHost, Copy, Fill, CopyStrided, Release)
// ve 16 bitlik türler için geçici float32 tamponlar.

func f32bit(v float32) uint32 { return math.Float32bits(v) }

// gecici, bir çağrının açtığı geçici tamponları toplar; bitir hepsini havuza iade eder.
// Hesap çekirdekleri float32 bitişik görür: 16 bitlik (ya da adımlı) girdiler float32
// bitişik geçici tampona açılır, 16 bitlik hedefler float32 tamponda hesaplanıp bir kez
// yuvarlanarak geri yazılır (CPU arka ucuyla aynı düzen).
type gecici struct {
	g *arkaUc
	t []*depo
}

func (g *arkaUc) gecici() *gecici { return &gecici{g: g} }

func (t *gecici) tampon(n int, sifirla bool) *depo {
	d := t.g.ayir(n, tensor.Float32, sifirla)
	t.t = append(t.t, d)
	return d
}

func (t *gecici) bitir() {
	for _, d := range t.t {
		t.g.birak(d)
	}
	t.t = nil
}

// oku görünümü float32 bitişik görünüme çevirir (zaten öyleyse aynen).
func (t *gecici) oku(v tensor.View) tensor.View {
	d := t.g.depoOf(v.S)
	if d.tur == tensor.Float32 && v.IsContiguous() {
		return v
	}
	n := v.Numel()
	tmp := t.tampon(n, false)
	hv := tensor.View{S: tmp, Shape: v.Shape, Strides: bitisik(v.Shape)}
	t.g.kopyaAdimli(hv, v, false, 0)
	return hv
}

// okuP isteğe bağlı görünüm.
func (t *gecici) okuP(v *tensor.View) *tensor.View {
	if v == nil {
		return nil
	}
	w := t.oku(*v)
	return &w
}

// okuD düz depolamanın float32 karşılığı (nil için nil).
func (t *gecici) okuD(s tensor.Storage) *depo {
	if s == nil {
		return nil
	}
	d := t.g.depoOf(s)
	if d.tur == tensor.Float32 {
		return d
	}
	return t.g.depoOf(t.oku(tensor.View{S: d, Shape: []int{d.n}, Strides: []int{1}}).S)
}

// yaz hedefin float32 karşılığını ve geri yazma işlevini verir; eski doğruysa geçici
// tampon mevcut içerikle başlar (biriktirme), değilse sıfırdır. nil hedef nil döner.
func (t *gecici) yaz(s tensor.Storage, eski bool) (*depo, func()) {
	if s == nil {
		return nil, func() {}
	}
	d := t.g.depoOf(s)
	if d.tur == tensor.Float32 {
		return d, func() {}
	}
	tmp := t.tampon(d.n, !eski)
	duzV := func(x *depo) tensor.View { return tensor.View{S: x, Shape: []int{x.n}, Strides: []int{1}} }
	if eski {
		t.g.kopyaAdimli(duzV(tmp), duzV(d), false, 0)
	}
	return tmp, func() { t.g.kopyaAdimli(duzV(d), duzV(tmp), false, 0) }
}

// Alloc sıfırlanmış depolama.
func (g *arkaUc) Alloc(n int, dt tensor.DType) tensor.Storage {
	g.gir()
	defer g.cik()
	return g.ayir(n, dt, true)
}

// AllocUninit (tensor.UninitAllocator) içeriği belirsiz depolama; her elemanı yazılacak
// çıktılar içindir.
func (g *arkaUc) AllocUninit(n int, dt tensor.DType) tensor.Storage {
	g.gir()
	defer g.cik()
	return g.ayir(n, dt, false)
}

// Release (tensor.Releaser) depolamayı havuza iade eder.
func (g *arkaUc) Release(s tensor.Storage) {
	d, ok := s.(*depo)
	if !ok || d.g != g {
		return
	}
	g.gir()
	defer g.cik()
	if d.iade {
		panic("tensor: depolama havuza iki kez iade edildi")
	}
	g.birak(d)
}

// FromHost konak verisini dt türünde yeni depolamaya yükler. Küçük float32 depolamalar
// konak gölgesini tutar.
func (g *arkaUc) FromHost(data []float32, dt tensor.DType) tensor.Storage {
	g.gir()
	defer g.cik()
	n := len(data)
	d := g.ayir(n, dt, false)
	if n == 0 {
		return d
	}
	if dt == tensor.Float32 {
		g.yukleKonak(d, unsafe.Pointer(&data[0]), 4*n)
		if n <= golgeSiniri {
			d.konak = append([]float32(nil), data...)
		}
		return d
	}
	gt := g.gecici()
	defer gt.bitir()
	tmp := gt.tampon(n, false)
	g.yukleKonak(tmp, unsafe.Pointer(&data[0]), 4*n)
	duz := []int{n}
	g.kopyaAdimli(tensor.View{S: d, Shape: duz, Strides: []int{1}}, tensor.View{S: tmp, Shape: duz, Strides: []int{1}}, false, 0)
	return d
}

// ToHost görünümün elemanlarını float32 olarak konağa kopyalar. Gölgesi olan depolama
// eşzamanlama olmadan konaktan okunur.
func (g *arkaUc) ToHost(src tensor.View) []float32 {
	g.gir()
	defer g.cik()
	d := g.depoOf(src.S)
	n := src.Numel()
	out := make([]float32, n)
	if n == 0 {
		return out
	}
	if d.konak != nil {
		golgedenTopla(out, d.konak, src)
		return out
	}
	gt := g.gecici()
	defer gt.bitir()
	v := gt.oku(src)
	g.indir(unsafe.Pointer(&out[0]), g.adresi(v), 4*n)
	return out
}

// golgedenTopla görünümün elemanlarını konak gölgesinden mantıksal sırayla toplar.
func golgedenTopla(out, kaynak []float32, v tensor.View) {
	nd := len(v.Shape)
	idx := make([]int, nd)
	for i := range out {
		o := v.Offset
		for d := 0; d < nd; d++ {
			o += idx[d] * v.Strides[d]
		}
		out[i] = kaynak[o]
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
func (g *arkaUc) Copy(dst tensor.Storage, dstOff int, src tensor.View) {
	g.gir()
	defer g.cik()
	hv := tensor.View{S: dst, Offset: dstOff, Shape: src.Shape, Strides: bitisik(src.Shape)}
	g.kopyaAdimli(hv, src, false, 0)
}

// CopyStrided src'yi dst görünümüne yazar (src yayınlanmış olabilir).
func (g *arkaUc) CopyStrided(dst, src tensor.View) {
	g.gir()
	defer g.cik()
	g.kopyaAdimli(dst, src, false, 0)
}

// Fill bütün elemanları v yapar.
func (g *arkaUc) Fill(dst tensor.Storage, v float32) {
	g.gir()
	defer g.cik()
	g.doldur(g.depoOf(dst), v)
}

package gpu

import (
	"encoding/binary"
	"math"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// Arka uç: çok tensörlü iyileştirici adımları, ölçekleme ve kareler toplamı (tek başlatma;
// bkz. cekirdek_cok.go). Tek tensörlü yöntemler de aynı yoldan geçer.

// cokOge çok tensörlü işin bir tensörü: göstericiler (kipe göre p, g, m, v), eleman sayısı ve
// tensöre özgü iki sayı.
type multiItem struct {
	d      [4]*storage
	n      int
	f1, f2 float32
}

// cokBaslat tabloları kurup yükler ve çekirdeği başlatır; blok sayısını verir. Yazılan
// depolamaların (yaz) gölgesi silinir. kismi yalnızca kare kipinde verilir.
func (g *backend) multiLaunch(gt *temp, mode2 int, items []multiItem, write []int, shared []float32, partial *storage) int {
	table := make([]byte, multiTensor*max(len(items), 1))
	var blocksT []byte
	for i, o := range items {
		checkSize(o.n)
		t := table[multiTensor*i:]
		for j, d := range o.d {
			if d != nil {
				binary.LittleEndian.PutUint64(t[8*j:], d.addr)
			}
		}
		binary.LittleEndian.PutUint32(t[32:], uint32(o.n))
		binary.LittleEndian.PutUint32(t[36:], math.Float32bits(o.f1))
		binary.LittleEndian.PutUint32(t[40:], math.Float32bits(o.f2))
		for begin := 0; begin < o.n; begin += multiPart {
			blocksT = binary.LittleEndian.AppendUint32(blocksT, uint32(i))
			blocksT = binary.LittleEndian.AppendUint32(blocksT, uint32(begin))
		}
		for _, j := range write {
			if o.d[j] != nil {
				o.d[j].host = nil
			}
		}
	}
	nb := len(blocksT) / 8
	if nb == 0 {
		return 0
	}
	td := gt.buffer(len(table)/4, false)
	bd := gt.buffer(len(blocksT)/4, false)
	g.loadHost(td, unsafe.Pointer(&table[0]), len(table))
	g.loadHost(bd, unsafe.Pointer(&blocksT[0]), len(blocksT))
	c := g.takeKernel(multiNames[mode2], multiKernel(mode2))
	p := g.param().read(td, 0).read(bd, 0)
	switch mode2 {
	case multiSquare:
		p.write(partial, 0)
	case multiScaleMem:
		p.read(partial, 0) // ölçek (tek float32)
	}
	for _, v := range shared {
		p.f32(v)
	}
	g.launch(c, [3]uint32{uint32(nb), 1, 1}, [3]uint32{multiBlock, 1, 1}, p)
	return nb
}

// cokBlokSayisi n elemanın blok sayısı.
func multiBlockCount(n int) int { return (n + multiPart - 1) / multiPart }

// AdamWStepAll (tensor.MultiTensorOps) bütün parametrelerin AdamW adımı tek başlatmada.
func (g *backend) AdamWStepAll(ps, gs, ms, vs []tensor.Storage, lr, b1, b2, eps, wd float32, bc1, bc2 []float32) {
	g.enter()
	defer g.exit()
	g.markStep()
	g.adamW(ps, gs, ms, vs, lr, b1, b2, eps, wd, bc1, bc2)
}

func (g *backend) AdamWStep(p, gs, m, v tensor.Storage, lr, b1, b2, eps, wd, bc1, bc2 float32) {
	g.enter()
	defer g.exit()
	g.adamW([]tensor.Storage{p}, []tensor.Storage{gs}, []tensor.Storage{m}, []tensor.Storage{v}, lr, b1, b2, eps, wd,
		[]float32{bc1}, []float32{bc2})
}

// cokGeciciSinir çok tensörlü işte aynı anda canlı float32 geçici kopyaların (16 bitlik
// parametre ya da gradyanların açılması) en çok baytı: aşılınca iş parçalara bölünür (her parça
// tek başlatma), böylece tepe bellek tek tensörlü yoldakinden çok büyümez.
var multiTempLimit = 256 << 20

// cokYazici bir parçanın geçici tamponlarını ve geri yazma işlevlerini toplar.
type multiWriter struct {
	g     *backend
	gt    *temp
	lasts []func()
	byteN int
}

// yaz depolamanın float32 karşılığı (16 bitlikse geçici kopya, sonda geri yazılır).
func (c *multiWriter) write(s tensor.Storage) *storage {
	d, last := c.gt.write(s, true)
	c.lasts = append(c.lasts, last)
	if s != nil && d != c.g.storageOf(s) {
		c.byteN += 4 * d.n
	}
	return d
}

// oku yalnızca okunan depolamanın float32 karşılığı.
func (c *multiWriter) read(s tensor.Storage) *storage {
	d := c.gt.readD(s)
	if d != c.g.storageOf(s) {
		c.byteN += 4 * d.n
	}
	return d
}

// cokParcali n öğeyi ekle ile toplar; geçici baytlar cokGeciciSinir'i aşınca ya da sonda
// baslat'ı çağırır (toplanan öğeleri başlatır), geçici kopyaları geri yazar ve bırakır.
func (g *backend) multiChunked(n int, add func(c *multiWriter, i int), launch func(c *multiWriter)) {
	c := &multiWriter{g: g, gt: g.temp()}
	pending := false
	finish := func() {
		if pending {
			launch(c)
		}
		for _, last := range c.lasts {
			last()
		}
		c.gt.finish()
		c.gt, c.lasts, c.byteN, pending = g.temp(), nil, 0, false
	}
	defer func() { c.gt.finish() }()
	for i := 0; i < n; i++ {
		add(c, i)
		pending = true
		if c.byteN > multiTempLimit {
			finish()
		}
	}
	finish()
}

func (g *backend) adamW(ps, gs, ms, vs []tensor.Storage, lr, b1, b2, eps, wd float32, bc1, bc2 []float32) {
	var items []multiItem
	g.multiChunked(len(ps), func(c *multiWriter, i int) {
		pd, md, vd := c.write(ps[i]), c.write(ms[i]), c.write(vs[i])
		items = append(items, multiItem{d: [4]*storage{pd, c.read(gs[i]), md, vd}, n: pd.n,
			f1: lr / bc1[i], f2: float32(math.Sqrt(float64(bc2[i])))})
	}, func(c *multiWriter) {
		g.multiLaunch(c.gt, multiAdamW, items, []int{0, 2, 3}, []float32{b1, 1 - b1, b2, 1 - b2, eps, 1 - lr*wd}, nil)
		items = items[:0]
	})
}

// SGDStepAll (tensor.MultiTensorOps) bütün parametrelerin SGD adımı tek başlatmada; vel nil
// ise (ya da elemanları nil ise) momentum yoktur.
func (g *backend) SGDStepAll(ps, gs, vels []tensor.Storage, lr, momentum, wd float32) {
	g.enter()
	defer g.exit()
	g.markStep()
	g.sgd(ps, gs, vels, lr, momentum, wd)
}

func (g *backend) SGDStep(p, gs, vel tensor.Storage, lr, momentum, wd float32) {
	g.enter()
	defer g.exit()
	g.sgd([]tensor.Storage{p}, []tensor.Storage{gs}, []tensor.Storage{vel}, lr, momentum, wd)
}

func (g *backend) sgd(ps, gs, vels []tensor.Storage, lr, momentum, wd float32) {
	var plain, mom []multiItem
	shared := []float32{lr, momentum, wd}
	g.multiChunked(len(ps), func(c *multiWriter, i int) {
		pd := c.write(ps[i])
		var vel tensor.Storage
		if i < len(vels) {
			vel = vels[i]
		}
		vd := c.write(vel)
		o := multiItem{d: [4]*storage{pd, c.read(gs[i]), vd}, n: pd.n}
		if vd != nil {
			mom = append(mom, o)
		} else {
			plain = append(plain, o)
		}
	}, func(c *multiWriter) {
		g.multiLaunch(c.gt, multiSGD, plain, []int{0}, shared, nil)
		g.multiLaunch(c.gt, multiSGDMomentum, mom, []int{0, 2}, shared, nil)
		plain, mom = plain[:0], mom[:0]
	})
}

// ScaleAll (tensor.MultiTensorOps) her depolamayı k ile çarpar (tek başlatma).
func (g *backend) ScaleAll(ss []tensor.Storage, k float32) {
	g.enter()
	defer g.exit()
	g.scaleBy(ss, k)
}

func (g *backend) Scale(s tensor.Storage, k float32) {
	g.enter()
	defer g.exit()
	g.scaleBy([]tensor.Storage{s}, k)
}

func (g *backend) scaleBy(ss []tensor.Storage, k float32) {
	var items []multiItem
	g.multiChunked(len(ss), func(c *multiWriter, i int) {
		d := c.write(ss[i])
		items = append(items, multiItem{d: [4]*storage{d}, n: d.n})
	}, func(c *multiWriter) {
		g.multiLaunch(c.gt, multiScale, items, []int{0}, []float32{k}, nil)
		items = items[:0]
	})
}

// SumSquaresAll (tensor.SquareSummer) depolamaların kareler toplamı: tek başlatma, tek
// eşzamanlama; her depolamanın blok toplamları sırayla, sonra depolamalar sırayla toplanır.
func (g *backend) SumSquaresAll(ss []tensor.Storage) float64 {
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	var items []multiItem
	totalB := 0
	for _, s := range ss {
		d := gt.readD(s)
		items = append(items, multiItem{d: [4]*storage{d}, n: d.n})
		totalB += multiBlockCount(d.n)
	}
	if totalB == 0 {
		return 0
	}
	partial := gt.buffer(2*totalB, false) // float64: iki float32 yeri
	g.multiLaunch(gt, multiSquare, items, nil, nil, partial)
	f := make([]float64, totalB)
	g.download(unsafe.Pointer(&f[0]), partial.addr, 8*totalB)
	var s float64
	o := 0
	for _, og := range items {
		var t float64
		n := multiBlockCount(og.n)
		for _, v := range f[o : o+n] {
			t += v
		}
		s += t
		o += n
	}
	return s
}

// ClipGradAll (tensor.GradClipper) gradyanların toplam normunu ve kırpmayı konak beklemeden
// yapar: kareler toplamı (cok_kare), norm ve ölçek (kirp_norm, tek blok), ölçekleme
// (cok_olcek_bellek; ölçek 1 ise yazılmaz). Yalnızca float32 gradyanlar.
func (g *backend) ClipGradAll(ss []tensor.Storage, most float32) (tensor.Storage, bool) {
	for _, s := range ss {
		if s.DType() != tensor.Float32 {
			return nil, false
		}
	}
	g.enter()
	defer g.exit()
	gt := g.temp()
	defer gt.finish()
	norm := g.allocate(1, tensor.Float32, false)
	var items []multiItem
	totalB := 0
	for _, s := range ss {
		d := g.storageOf(s)
		items = append(items, multiItem{d: [4]*storage{d}, n: d.n})
		totalB += multiBlockCount(d.n)
	}
	if totalB == 0 {
		g.fill(norm, 0)
		return norm, true
	}
	partial := gt.buffer(2*totalB, false) // float64: iki float32 yeri
	g.multiLaunch(gt, multiSquare, items, nil, nil, partial)
	scaleF := gt.buffer(1, false)
	c := g.takeKernel("kirp_norm", clipNormKernel)
	g.launch(c, [3]uint32{1, 1, 1}, [3]uint32{squareBlock, 1, 1}, g.param().read(partial, 0).u32(totalB).f32(most).write(norm, 0).write(scaleF, 0))
	g.multiLaunch(gt, multiScaleMem, items, []int{0}, nil, scaleF)
	return norm, true
}

func (g *backend) SumSquares(s tensor.Storage) float64 {
	return g.SumSquaresAll([]tensor.Storage{s})
}

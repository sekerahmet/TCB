package gpu

import (
	"encoding/binary"
	"math"
	"unsafe"

	"tr_dili/internal/hesap/tensor"
)

// Arka uç: çok tensörlü iyileştirici adımları, ölçekleme ve kareler toplamı (tek başlatma;
// bkz. cekirdek_cok.go). Tek tensörlü yöntemler de aynı yoldan geçer.

// cokOge çok tensörlü işin bir tensörü: göstericiler (kipe göre p, g, m, v), eleman sayısı ve
// tensöre özgü iki sayı.
type cokOge struct {
	d      [4]*depo
	n      int
	f1, f2 float32
}

// cokBaslat tabloları kurup yükler ve çekirdeği başlatır; blok sayısını verir. Yazılan
// depolamaların (yaz) gölgesi silinir. kismi yalnızca kare kipinde verilir.
func (g *arkaUc) cokBaslat(gt *gecici, kip int, ogeler []cokOge, yaz []int, ortak []float32, kismi *depo) int {
	tablo := make([]byte, cokTensor*max(len(ogeler), 1))
	var bloklarT []byte
	for i, o := range ogeler {
		boyDenetle(o.n)
		t := tablo[cokTensor*i:]
		for j, d := range o.d {
			if d != nil {
				binary.LittleEndian.PutUint64(t[8*j:], d.adr)
			}
		}
		binary.LittleEndian.PutUint32(t[32:], uint32(o.n))
		binary.LittleEndian.PutUint32(t[36:], math.Float32bits(o.f1))
		binary.LittleEndian.PutUint32(t[40:], math.Float32bits(o.f2))
		for bas := 0; bas < o.n; bas += cokParca {
			bloklarT = binary.LittleEndian.AppendUint32(bloklarT, uint32(i))
			bloklarT = binary.LittleEndian.AppendUint32(bloklarT, uint32(bas))
		}
		for _, j := range yaz {
			if o.d[j] != nil {
				o.d[j].konak = nil
			}
		}
	}
	nb := len(bloklarT) / 8
	if nb == 0 {
		return 0
	}
	td := gt.tampon(len(tablo)/4, false)
	bd := gt.tampon(len(bloklarT)/4, false)
	g.yukleKonak(td, unsafe.Pointer(&tablo[0]), len(tablo))
	g.yukleKonak(bd, unsafe.Pointer(&bloklarT[0]), len(bloklarT))
	c := g.cekirdekAl(cokAdlari[kip], cokCekirdegi(kip))
	p := g.param().oku(td, 0).oku(bd, 0)
	if kip == cokKare {
		p.yaz(kismi, 0)
	}
	for _, v := range ortak {
		p.f32(v)
	}
	g.baslat(c, [3]uint32{uint32(nb), 1, 1}, [3]uint32{cokBlok, 1, 1}, p)
	return nb
}

// cokBlokSayisi n elemanın blok sayısı.
func cokBlokSayisi(n int) int { return (n + cokParca - 1) / cokParca }

// AdamWStepAll (tensor.MultiTensorOps) bütün parametrelerin AdamW adımı tek başlatmada.
func (g *arkaUc) AdamWStepAll(ps, gs, ms, vs []tensor.Storage, lr, b1, b2, eps, wd float32, bc1, bc2 []float32) {
	g.gir()
	defer g.cik()
	g.adimIsaretle()
	g.adamW(ps, gs, ms, vs, lr, b1, b2, eps, wd, bc1, bc2)
}

func (g *arkaUc) AdamWStep(p, gs, m, v tensor.Storage, lr, b1, b2, eps, wd, bc1, bc2 float32) {
	g.gir()
	defer g.cik()
	g.adamW([]tensor.Storage{p}, []tensor.Storage{gs}, []tensor.Storage{m}, []tensor.Storage{v}, lr, b1, b2, eps, wd,
		[]float32{bc1}, []float32{bc2})
}

// cokGeciciSinir çok tensörlü işte aynı anda canlı float32 geçici kopyaların (16 bitlik
// parametre ya da gradyanların açılması) en çok baytı: aşılınca iş parçalara bölünür (her parça
// tek başlatma), böylece tepe bellek tek tensörlü yoldakinden çok büyümez.
var cokGeciciSinir = 256 << 20

// cokYazici bir parçanın geçici tamponlarını ve geri yazma işlevlerini toplar.
type cokYazici struct {
	g      *arkaUc
	gt     *gecici
	sonlar []func()
	bayt   int
}

// yaz depolamanın float32 karşılığı (16 bitlikse geçici kopya, sonda geri yazılır).
func (c *cokYazici) yaz(s tensor.Storage) *depo {
	d, son := c.gt.yaz(s, true)
	c.sonlar = append(c.sonlar, son)
	if s != nil && d != c.g.depoOf(s) {
		c.bayt += 4 * d.n
	}
	return d
}

// oku yalnızca okunan depolamanın float32 karşılığı.
func (c *cokYazici) oku(s tensor.Storage) *depo {
	d := c.gt.okuD(s)
	if d != c.g.depoOf(s) {
		c.bayt += 4 * d.n
	}
	return d
}

// cokParcali n öğeyi ekle ile toplar; geçici baytlar cokGeciciSinir'i aşınca ya da sonda
// baslat'ı çağırır (toplanan öğeleri başlatır), geçici kopyaları geri yazar ve bırakır.
func (g *arkaUc) cokParcali(n int, ekle func(c *cokYazici, i int), baslat func(c *cokYazici)) {
	c := &cokYazici{g: g, gt: g.gecici()}
	bekleyen := false
	bitir := func() {
		if bekleyen {
			baslat(c)
		}
		for _, son := range c.sonlar {
			son()
		}
		c.gt.bitir()
		c.gt, c.sonlar, c.bayt, bekleyen = g.gecici(), nil, 0, false
	}
	defer func() { c.gt.bitir() }()
	for i := 0; i < n; i++ {
		ekle(c, i)
		bekleyen = true
		if c.bayt > cokGeciciSinir {
			bitir()
		}
	}
	bitir()
}

func (g *arkaUc) adamW(ps, gs, ms, vs []tensor.Storage, lr, b1, b2, eps, wd float32, bc1, bc2 []float32) {
	var ogeler []cokOge
	g.cokParcali(len(ps), func(c *cokYazici, i int) {
		pd, md, vd := c.yaz(ps[i]), c.yaz(ms[i]), c.yaz(vs[i])
		ogeler = append(ogeler, cokOge{d: [4]*depo{pd, c.oku(gs[i]), md, vd}, n: pd.n,
			f1: lr / bc1[i], f2: float32(math.Sqrt(float64(bc2[i])))})
	}, func(c *cokYazici) {
		g.cokBaslat(c.gt, cokAdamW, ogeler, []int{0, 2, 3}, []float32{b1, 1 - b1, b2, 1 - b2, eps, 1 - lr*wd}, nil)
		ogeler = ogeler[:0]
	})
}

// SGDStepAll (tensor.MultiTensorOps) bütün parametrelerin SGD adımı tek başlatmada; vel nil
// ise (ya da elemanları nil ise) momentum yoktur.
func (g *arkaUc) SGDStepAll(ps, gs, vels []tensor.Storage, lr, momentum, wd float32) {
	g.gir()
	defer g.cik()
	g.adimIsaretle()
	g.sgd(ps, gs, vels, lr, momentum, wd)
}

func (g *arkaUc) SGDStep(p, gs, vel tensor.Storage, lr, momentum, wd float32) {
	g.gir()
	defer g.cik()
	g.sgd([]tensor.Storage{p}, []tensor.Storage{gs}, []tensor.Storage{vel}, lr, momentum, wd)
}

func (g *arkaUc) sgd(ps, gs, vels []tensor.Storage, lr, momentum, wd float32) {
	var duz, mom []cokOge
	ortak := []float32{lr, momentum, wd}
	g.cokParcali(len(ps), func(c *cokYazici, i int) {
		pd := c.yaz(ps[i])
		var vel tensor.Storage
		if i < len(vels) {
			vel = vels[i]
		}
		vd := c.yaz(vel)
		o := cokOge{d: [4]*depo{pd, c.oku(gs[i]), vd}, n: pd.n}
		if vd != nil {
			mom = append(mom, o)
		} else {
			duz = append(duz, o)
		}
	}, func(c *cokYazici) {
		g.cokBaslat(c.gt, cokSGD, duz, []int{0}, ortak, nil)
		g.cokBaslat(c.gt, cokSGDMomentum, mom, []int{0, 2}, ortak, nil)
		duz, mom = duz[:0], mom[:0]
	})
}

// ScaleAll (tensor.MultiTensorOps) her depolamayı k ile çarpar (tek başlatma).
func (g *arkaUc) ScaleAll(ss []tensor.Storage, k float32) {
	g.gir()
	defer g.cik()
	g.olcekle(ss, k)
}

func (g *arkaUc) Scale(s tensor.Storage, k float32) {
	g.gir()
	defer g.cik()
	g.olcekle([]tensor.Storage{s}, k)
}

func (g *arkaUc) olcekle(ss []tensor.Storage, k float32) {
	var ogeler []cokOge
	g.cokParcali(len(ss), func(c *cokYazici, i int) {
		d := c.yaz(ss[i])
		ogeler = append(ogeler, cokOge{d: [4]*depo{d}, n: d.n})
	}, func(c *cokYazici) {
		g.cokBaslat(c.gt, cokOlcek, ogeler, []int{0}, []float32{k}, nil)
		ogeler = ogeler[:0]
	})
}

// SumSquaresAll (tensor.SquareSummer) depolamaların kareler toplamı: tek başlatma, tek
// eşzamanlama; her depolamanın blok toplamları sırayla, sonra depolamalar sırayla toplanır.
func (g *arkaUc) SumSquaresAll(ss []tensor.Storage) float64 {
	g.gir()
	defer g.cik()
	gt := g.gecici()
	defer gt.bitir()
	var ogeler []cokOge
	toplamB := 0
	for _, s := range ss {
		d := gt.okuD(s)
		ogeler = append(ogeler, cokOge{d: [4]*depo{d}, n: d.n})
		toplamB += cokBlokSayisi(d.n)
	}
	if toplamB == 0 {
		return 0
	}
	kismi := gt.tampon(2*toplamB, false) // float64: iki float32 yeri
	g.cokBaslat(gt, cokKare, ogeler, nil, nil, kismi)
	f := make([]float64, toplamB)
	g.indir(unsafe.Pointer(&f[0]), kismi.adr, 8*toplamB)
	var s float64
	o := 0
	for _, og := range ogeler {
		var t float64
		n := cokBlokSayisi(og.n)
		for _, v := range f[o : o+n] {
			t += v
		}
		s += t
		o += n
	}
	return s
}

func (g *arkaUc) SumSquares(s tensor.Storage) float64 {
	return g.SumSquaresAll([]tensor.Storage{s})
}

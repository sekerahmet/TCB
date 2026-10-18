package tensor

import (
	"fmt"
	"math"
)

// İyileştiriciler (cihazdan bağımsız katman). Durum tamponları (momentum,
// moment tahminleri) parametrenin cihazında tutulur; güncelleme arka uçta
// yapılır.

// Optimizer parametreleri gradyanlarına göre güncelleyen iyileştirici.
type Optimizer interface {
	Step()
	ZeroGrad()
}

func checkParams(params []*Tensor) {
	for _, p := range params {
		if !p.requiresGrad {
			panic("tensor: iyileştirici parametreleri gradyan istemeli (SetRequiresGrad(true))")
		}
		if !p.IsContiguous() || p.offset != 0 || p.storage.Len() != p.Numel() {
			panic(fmt.Sprintf("tensor: iyileştirici parametresi kendi depolamasına sahip bitişik bir tensör olmalı (şekil %v)", p.shape))
		}
	}
}

// anaAgirliklar 16 bitlik parametreler için float32 ana ağırlık kopyalarını
// yönetir. Güncelleme ana kopyada yapılır ve parametreye yuvarlanarak yazılır;
// böylece parametrenin 16 bitlik çözünürlüğünden küçük adımlar kaybolmaz.
// Parametre iyileştirici dışında değiştirilirse (sürümü son yazımdan farklıysa)
// ana kopya parametreden yeniden alınır.
type anaAgirliklar struct {
	kapali bool
	ana    []Storage // parametre başına float32 kopya (gerekmiyorsa nil)
	surum  []uint64  // son yazımdan sonraki parametre sürümü
}

func yeniAnaAgirliklar(n int) anaAgirliklar {
	return anaAgirliklar{ana: make([]Storage, n), surum: make([]uint64, n)}
}

// hedef k. parametrenin güncelleneceği depolama.
func (a *anaAgirliklar) hedef(k int, p *Tensor) Storage {
	if a.kapali || p.tur == Float32 {
		return p.storage
	}
	if a.ana[k] == nil || p.storage.Version() != a.surum[k] {
		if a.ana[k] == nil {
			a.ana[k] = p.be.Alloc(p.Numel(), Float32)
		}
		p.be.Copy(a.ana[k], 0, p.view())
	}
	return a.ana[k]
}

// yaz güncellenmiş hedefi (ana kopyaysa yuvarlayarak) parametreye yazar ve
// parametrenin sürümünü artırır (yerinde değişiklik).
func (a *anaAgirliklar) yaz(k int, p *Tensor, h Storage) {
	if h != p.storage {
		p.be.Copy(p.storage, 0, View{S: h, Shape: p.shape, Strides: contiguousStrides(p.shape)})
	}
	p.storage.BumpVersion()
	a.surum[k] = p.storage.Version()
}

// ayarla ana kopya kullanımını açar/kapatır.
func (a *anaAgirliklar) ayarla(acik bool) {
	a.kapali = !acik
	if !acik {
		clear(a.ana)
	}
}

// SGD momentumlu (isteğe bağlı) olasılıksal gradyan inişi. WeightDecay,
// gradyana L2 cezası olarak eklenir.
type SGD struct {
	Params      []*Tensor
	LR          float32
	Momentum    float32
	WeightDecay float32
	hiz         []Storage
	ana         anaAgirliklar
}

// NewSGD yeni bir SGD iyileştirici kurar.
func NewSGD(params []*Tensor, lr, momentum float32) *SGD {
	checkParams(params)
	return &SGD{Params: params, LR: lr, Momentum: momentum, hiz: make([]Storage, len(params)),
		ana: yeniAnaAgirliklar(len(params))}
}

// SetMasterWeights 16 bitlik parametreler için float32 ana ağırlık kopyasını
// açar ya da kapatır (varsayılan açık). Kapalıyken güncelleme doğrudan 16
// bitlik parametre üzerinde (float32'de hesaplanıp yuvarlanarak) yapılır.
func (o *SGD) SetMasterWeights(on bool) { o.ana.ayarla(on) }

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır.
// Parametreler yerinde değiştiği için sürümleri artar.
func (o *SGD) Step() {
	for k, p := range o.Params {
		if p.grad == nil {
			continue
		}
		if o.Momentum != 0 && o.hiz[k] == nil {
			o.hiz[k] = p.be.Alloc(p.Numel(), Float32)
		}
		h := o.ana.hedef(k, p)
		p.be.SGDStep(h, p.grad, o.hiz[k], o.LR, o.Momentum, o.WeightDecay)
		o.ana.yaz(k, p, h)
	}
}

// ZeroGrad tüm parametre gradyanlarını sıfırlar.
func (o *SGD) ZeroGrad() { ZeroGrads(o.Params) }

// AdamW ayrık ağırlık çürümeli uyarlanır moment tahmini iyileştiricisi:
//
//	p ← p - lr·wd·p
//	m ← β1·m + (1-β1)·g,  v ← β2·v + (1-β2)·g²
//	p ← p - lr · m̂ / (√v̂ + eps)   (m̂, v̂ sapma düzeltmeli)
type AdamW struct {
	Params      []*Tensor
	LR          float32
	Beta1       float32
	Beta2       float32
	Eps         float32
	WeightDecay float32
	m, v        []Storage
	adim        int   // Step çağrısı sayısı
	adimlar     []int // parametre başına güncelleme sayısı (sapma düzeltmesi bunu kullanır)
	ana         anaAgirliklar
}

// NewAdamW varsayılan β1=0.9, β2=0.999, eps=1e-8 ile AdamW kurar.
func NewAdamW(params []*Tensor, lr, weightDecay float32) *AdamW {
	checkParams(params)
	return &AdamW{
		Params: params, LR: lr, Beta1: 0.9, Beta2: 0.999, Eps: 1e-8, WeightDecay: weightDecay,
		m: make([]Storage, len(params)), v: make([]Storage, len(params)), adimlar: make([]int, len(params)),
		ana: yeniAnaAgirliklar(len(params)),
	}
}

// SetMasterWeights 16 bitlik parametreler için float32 ana ağırlık kopyasını
// açar ya da kapatır (varsayılan açık). Moment tahminleri her zaman float32'dir.
func (o *AdamW) SetMasterWeights(on bool) { o.ana.ayarla(on) }

// StepCount şimdiye kadar yapılan adım sayısı.
func (o *AdamW) StepCount() int { return o.adim }

// Step bir güncelleme adımı uygular. Gradyanı olmayan parametreler atlanır. Sapma
// düzeltmesi parametrenin kendi güncelleme sayısıyla yapılır: bazı adımlarda gradyan
// almayan bir parametre (ör. o adımda kullanılmayan gömme satırları değil, hiç gradyan
// akmayan bir parametre) ilk güncellemesinde yeni bir iyileştiricideki gibi davranır.
func (o *AdamW) Step() {
	o.adim++
	for k, p := range o.Params {
		if p.grad == nil {
			continue
		}
		o.adimlar[k]++
		t := float64(o.adimlar[k])
		bc1 := float32(1 - math.Pow(float64(o.Beta1), t))
		bc2 := float32(1 - math.Pow(float64(o.Beta2), t))
		if o.m[k] == nil {
			o.m[k] = p.be.Alloc(p.Numel(), Float32)
			o.v[k] = p.be.Alloc(p.Numel(), Float32)
		}
		h := o.ana.hedef(k, p)
		p.be.AdamWStep(h, p.grad, o.m[k], o.v[k], o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, bc1, bc2)
		o.ana.yaz(k, p, h)
	}
}

// ZeroGrad tüm parametre gradyanlarını sıfırlar.
func (o *AdamW) ZeroGrad() { ZeroGrads(o.Params) }

// ClipGradNorm parametre gradyanlarının toplam L2 normunu en fazla maxNorm
// olacak şekilde ölçekler ve kırpma öncesi normu döndürür.
func ClipGradNorm(params []*Tensor, maxNorm float64) float64 {
	var s float64
	for _, p := range params {
		if p.grad != nil {
			s += p.be.SumSquares(p.grad)
		}
	}
	norm := math.Sqrt(s)
	if norm > maxNorm && norm > 0 {
		k := float32(maxNorm / norm)
		for _, p := range params {
			if p.grad != nil {
				p.be.Scale(p.grad, k)
				p.grad.BumpVersion()
			}
		}
	}
	return norm
}

// ----------------------------------------------------------------------------
// İyileştirici durumu (sürdürme)
// ----------------------------------------------------------------------------
//
// State iyileştiricinin bütün durumunu adlandırılmış tensörler olarak (kopya) verir;
// LoadState aynı parametre listesiyle kurulmuş yeni bir iyileştiriciye yükler. Böylece
// kontrol noktasından sürdürülen eğitim, kesintisiz eğitimle bit düzeyinde aynı devam eder.
// Anahtarlar parametrenin listedeki sırasını taşır: "m.k", "v.k" (AdamW momentleri),
// "hiz.k" (SGD momentumu), "adim.k" ve "adim" (adım sayıları), "ana.k" (16 bitlik
// parametrelerin float32 ana kopyası). Sözlük doğrudan dosyaya yazılabilir. Öğrenme oranı
// gibi ayarlar duruma dahil değildir (kurulumda verilir).

// sayacTensoru n'yi 2^24'e bölünmüş iki float32 parça olarak kayıpsız taşır.
func sayacTensoru(n int) *Tensor {
	return New([]float32{float32(n >> 24), float32(n & (1<<24 - 1))}, 2)
}

func sayacOku(ad string, t *Tensor) int {
	if t == nil || !sameShape(t.shape, []int{2}) {
		panic(fmt.Sprintf("tensor: iyileştirici durumu uyumsuz: %q adım sayacı [2] şeklinde olmalı", ad))
	}
	d := t.Data()
	return int(d[0])<<24 | int(d[1])
}

// durumKopya depolamanın bitişik float32 kopyasını tensör olarak verir.
func durumKopya(p *Tensor, s Storage) *Tensor {
	c := p.be.Alloc(s.Len(), s.DType())
	p.be.Copy(c, 0, View{S: s, Shape: p.shape, Strides: contiguousStrides(p.shape)})
	return fromStorage(p.be, c, p.shape)
}

// durumYukle st[ad]'ı p'nin şeklinde, p'nin cihazında yeni bir float32 depolamaya
// kopyalar; yoksa nil.
func durumYukle(st map[string]*Tensor, ad string, p *Tensor) Storage {
	t, ok := st[ad]
	if !ok {
		return nil
	}
	if !sameShape(t.shape, p.shape) {
		panic(fmt.Sprintf("tensor: iyileştirici durumu uyumsuz: %q şekli %v, parametre şekli %v", ad, t.shape, p.shape))
	}
	sameDevice("LoadState", p, t)
	c := p.be.Alloc(p.Numel(), Float32)
	p.be.Copy(c, 0, t.view())
	return c
}

// durumAnahtarDenetle durumdaki her anahtarın bilinen biçimde ve parametre sayısıyla
// uyumlu olduğunu denetler (başka bir modelin durumu sessizce kısmen yüklenmesin).
func durumAnahtarDenetle(st map[string]*Tensor, n int, onekler ...string) {
	for ad := range st {
		if ad == "adim" {
			continue
		}
		gecerli := false
		for _, on := range onekler {
			var k int
			if c, err := fmt.Sscanf(ad, on+".%d", &k); err == nil && c == 1 && fmt.Sprintf("%s.%d", on, k) == ad {
				if k < 0 || k >= n {
					panic(fmt.Sprintf("tensor: iyileştirici durumu uyumsuz: %q anahtarı %d parametreli iyileştiriciye uymuyor", ad, n))
				}
				gecerli = true
			}
		}
		if !gecerli {
			panic(fmt.Sprintf("tensor: iyileştirici durumu uyumsuz: bilinmeyen anahtar %q", ad))
		}
	}
}

// anaDurum ana kopyaları duruma ekler.
func (a *anaAgirliklar) durum(st map[string]*Tensor, params []*Tensor) {
	for k, p := range params {
		if a.ana[k] != nil && !a.kapali {
			st[fmt.Sprintf("ana.%d", k)] = durumKopya(p, a.ana[k])
		}
	}
}

// anaYukle ana kopyaları durumdan yükler; parametre o andaki sürümüyle eşleşir (ana kopya
// parametreden yeniden alınmaz).
func (a *anaAgirliklar) yukle(st map[string]*Tensor, params []*Tensor) {
	for k, p := range params {
		a.ana[k] = nil
		if c := durumYukle(st, fmt.Sprintf("ana.%d", k), p); c != nil && !a.kapali && p.tur != Float32 {
			a.ana[k] = c
			a.surum[k] = p.storage.Version()
		}
	}
}

// State AdamW'nin durumunu verir (bkz. yukarıdaki açıklama).
func (o *AdamW) State() map[string]*Tensor {
	st := map[string]*Tensor{"adim": sayacTensoru(o.adim)}
	for k, p := range o.Params {
		if o.m[k] != nil {
			st[fmt.Sprintf("m.%d", k)] = durumKopya(p, o.m[k])
			st[fmt.Sprintf("v.%d", k)] = durumKopya(p, o.v[k])
		}
		if o.adimlar[k] != 0 {
			st[fmt.Sprintf("adim.%d", k)] = sayacTensoru(o.adimlar[k])
		}
	}
	o.ana.durum(st, o.Params)
	return st
}

// LoadState State ile alınmış durumu yükler; parametre sayısı ve şekilleri aynı olmalıdır.
func (o *AdamW) LoadState(st map[string]*Tensor) {
	durumAnahtarDenetle(st, len(o.Params), "m", "v", "adim", "ana")
	o.adim = 0
	if t, ok := st["adim"]; ok {
		o.adim = sayacOku("adim", t)
	}
	for k, p := range o.Params {
		o.m[k] = durumYukle(st, fmt.Sprintf("m.%d", k), p)
		o.v[k] = durumYukle(st, fmt.Sprintf("v.%d", k), p)
		if (o.m[k] == nil) != (o.v[k] == nil) {
			panic(fmt.Sprintf("tensor: iyileştirici durumu uyumsuz: %d. parametrenin m ve v momentlerinden biri eksik", k))
		}
		o.adimlar[k] = 0
		if t, ok := st[fmt.Sprintf("adim.%d", k)]; ok {
			o.adimlar[k] = sayacOku(fmt.Sprintf("adim.%d", k), t)
		}
	}
	o.ana.yukle(st, o.Params)
}

// State SGD'nin durumunu (momentum hızları ve ana kopyalar) verir.
func (o *SGD) State() map[string]*Tensor {
	st := map[string]*Tensor{}
	for k, p := range o.Params {
		if o.hiz[k] != nil {
			st[fmt.Sprintf("hiz.%d", k)] = durumKopya(p, o.hiz[k])
		}
	}
	o.ana.durum(st, o.Params)
	return st
}

// LoadState State ile alınmış durumu yükler; parametre sayısı ve şekilleri aynı olmalıdır.
func (o *SGD) LoadState(st map[string]*Tensor) {
	durumAnahtarDenetle(st, len(o.Params), "hiz", "ana")
	for k, p := range o.Params {
		o.hiz[k] = durumYukle(st, fmt.Sprintf("hiz.%d", k), p)
	}
	o.ana.yukle(st, o.Params)
}

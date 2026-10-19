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
	sy := adimSayaci(o.Params, IslemSGD).baslat()
	defer sy.bitir()
	sirali := tekrarli(o.Params)
	var adimlar []iyiAdim
	for k, p := range o.Params {
		if p.grad == nil {
			continue
		}
		if o.Momentum != 0 && o.hiz[k] == nil {
			o.hiz[k] = p.be.Alloc(p.Numel(), Float32)
		}
		if sirali {
			h := o.ana.hedef(k, p)
			p.be.SGDStep(h, p.grad, o.hiz[k], o.LR, o.Momentum, o.WeightDecay)
			o.ana.yaz(k, p, h)
			continue
		}
		adimlar = append(adimlar, iyiAdim{k: k, p: p, h: o.ana.hedef(k, p), m: o.hiz[k]})
	}
	cokluUygula(adimlar, func(be MultiTensorOps, a []iyiAdim) {
		h, g, v := a[0].sutunlar(a)
		be.SGDStepAll(h, g, v, o.LR, o.Momentum, o.WeightDecay)
	}, func(a iyiAdim) {
		a.p.be.SGDStep(a.h, a.p.grad, a.m, o.LR, o.Momentum, o.WeightDecay)
	})
	for _, a := range adimlar {
		o.ana.yaz(a.k, a.p, a.h)
	}
}

// adimSayaci iyileştirici adımının profil sayacını başlatır: bir adım (bütün parametreler,
// çok tensörlü yolda tek arka uç çağrısı) bir çağrı sayılır. Cihaz ilk parametreninkidir.
func adimSayaci(params []*Tensor, is Islem) sayacOlcumu {
	be := Backend(cpuBackend{})
	if len(params) > 0 {
		be = params[0].be
	}
	return sayacBasla(be, is)
}

// iyiAdim bir parametrenin bu adımdaki güncellemesi: güncellenecek depolama (h; ana kopya
// olabilir), durum depolamaları (m, v) ve sapma düzeltmeleri.
type iyiAdim struct {
	k        int
	p        *Tensor
	h, m, v  Storage
	bc1, bc2 float32
}

// sutunlar adımların h, gradyan ve m depolamaları.
func (iyiAdim) sutunlar(a []iyiAdim) (h, g, m []Storage) {
	for _, x := range a {
		h, g, m = append(h, x.h), append(g, x.p.grad), append(m, x.m)
	}
	return h, g, m
}

// cokluUygula ardışık, aynı arka uçtaki ve çok tensörlü işi destekleyen adımları tek çağrıda
// (coklu), kalanları tek tek (tekli) uygular; sıra korunur.
//
// Aynı parametre listede iki kez geçerse (aynı depolama) grup orada bölünür: tek başlatmada iki
// blok aynı belleği aynı anda güncellerdi; bölünce güncellemeler tek tek yoldaki gibi sırayla
// yapılır.
func cokluUygula(adimlar []iyiAdim, coklu func(MultiTensorOps, []iyiAdim), tekli func(iyiAdim)) {
	for i := 0; i < len(adimlar); {
		mt, ok := adimlar[i].p.be.(MultiTensorOps)
		if !ok {
			tekli(adimlar[i])
			i++
			continue
		}
		goruldu := map[Storage]bool{adimlar[i].p.grad: true}
		j := i + 1
		for j < len(adimlar) && adimlar[j].p.be == adimlar[i].p.be && !goruldu[adimlar[j].p.grad] {
			goruldu[adimlar[j].p.grad] = true
			j++
		}
		coklu(mt, adimlar[i:j])
		i = j
	}
}

// tekrarli listede aynı parametrenin (depolamanın) birden çok kez geçip geçmediği. Öyleyse
// iyileştiriciler adımı parametre parametre (hedef, güncelleme, geri yazma sırasıyla) yapar:
// ikinci geçiş birincinin sonucunu görür.
func tekrarli(params []*Tensor) bool {
	goruldu := make(map[Storage]bool, len(params))
	for _, p := range params {
		if goruldu[p.storage] {
			return true
		}
		goruldu[p.storage] = true
	}
	return false
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
	sy := adimSayaci(o.Params, IslemAdamW).baslat()
	defer sy.bitir()
	o.adim++
	sirali := tekrarli(o.Params)
	var adimlar []iyiAdim
	for k, p := range o.Params {
		if p.grad == nil {
			continue
		}
		o.adimlar[k]++
		t := float64(o.adimlar[k])
		if o.m[k] == nil {
			o.m[k] = p.be.Alloc(p.Numel(), Float32)
			o.v[k] = p.be.Alloc(p.Numel(), Float32)
		}
		a := iyiAdim{k: k, p: p, m: o.m[k], v: o.v[k],
			bc1: float32(1 - math.Pow(float64(o.Beta1), t)), bc2: float32(1 - math.Pow(float64(o.Beta2), t))}
		a.h = o.ana.hedef(k, p)
		if sirali {
			p.be.AdamWStep(a.h, p.grad, a.m, a.v, o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, a.bc1, a.bc2)
			o.ana.yaz(k, p, a.h)
			continue
		}
		adimlar = append(adimlar, a)
	}
	cokluUygula(adimlar, func(be MultiTensorOps, a []iyiAdim) {
		h, g, m := a[0].sutunlar(a)
		v, bc1, bc2 := make([]Storage, len(a)), make([]float32, len(a)), make([]float32, len(a))
		for i, x := range a {
			v[i], bc1[i], bc2[i] = x.v, x.bc1, x.bc2
		}
		be.AdamWStepAll(h, g, m, v, o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, bc1, bc2)
	}, func(a iyiAdim) {
		a.p.be.AdamWStep(a.h, a.p.grad, a.m, a.v, o.LR, o.Beta1, o.Beta2, o.Eps, o.WeightDecay, a.bc1, a.bc2)
	})
	for _, a := range adimlar {
		o.ana.yaz(a.k, a.p, a.h)
	}
}

// ZeroGrad tüm parametre gradyanlarını sıfırlar.
func (o *AdamW) ZeroGrad() { ZeroGrads(o.Params) }

// ClipGradNorm parametre gradyanlarının toplam L2 normunu en fazla maxNorm
// olacak şekilde ölçekler ve kırpma öncesi normu döndürür.
func ClipGradNorm(params []*Tensor, maxNorm float64) float64 {
	if len(params) == 0 {
		return 0
	}
	sy := sayacBasla(params[0].be, IslemGradyanKirpma).baslat()
	defer sy.bitir()
	norm := math.Sqrt(gradKareToplami(params))
	if norm > maxNorm && norm > 0 {
		k := float32(maxNorm / norm)
		var adimlar []iyiAdim
		for _, p := range params {
			if p.grad != nil {
				adimlar = append(adimlar, iyiAdim{p: p})
			}
		}
		cokluUygula(adimlar, func(be MultiTensorOps, a []iyiAdim) {
			_, g, _ := a[0].sutunlar(a)
			be.ScaleAll(g, k)
		}, func(a iyiAdim) { a.p.be.Scale(a.p.grad, k) })
		for _, a := range adimlar {
			a.p.grad.BumpVersion()
		}
	}
	return norm
}

// GradClipper gradyanların toplam L2 normunu ve kırpmayı cihazda yapan arka uçların isteğe
// bağlı arayüzüdür: konak beklemez (GPU'da norm ve ölçek de sıradaki çekirdeklerce
// hesaplanır). Dönen tek elemanlı float32 depolama kırpma öncesi normdur; ok yanlışsa
// (ör. 16 bitlik gradyan) arka uç işi yapmamıştır.
type GradClipper interface {
	ClipGradAll(grads []Storage, maxNorm float32) (norm Storage, ok bool)
}

// ClipGradNormTensor ClipGradNorm'dur, kırpma öncesi normu 0 boyutlu tensör olarak verir.
// Bütün gradyanlar GradClipper olan tek arka uçtaysa norm ve kırpma cihazda yapılır ve
// konak beklemez; norm ancak okunduğunda (.değer, yazdırma) beklenir. Değilse norm konakta
// hesaplanır (ClipGradNorm) ve CPU tensörü olarak döner.
func ClipGradNormTensor(params []*Tensor, maxNorm float64) *Tensor {
	var be Backend
	var grads []Storage
	tek := true
	for _, p := range params {
		if p.grad == nil {
			continue
		}
		if be == nil {
			be = p.be
		} else if p.be != be {
			tek = false
		}
		grads = append(grads, p.grad)
	}
	if gc, ok := be.(GradClipper); ok && tek {
		sy := sayacBasla(be, IslemGradyanKirpma).baslat()
		norm, ok := gc.ClipGradAll(grads, float32(maxNorm))
		sy.bitir()
		if ok {
			for _, g := range grads {
				g.BumpVersion()
			}
			return fromStorage(be, norm, nil)
		}
	}
	return NewOn(CPU, []float32{float32(ClipGradNorm(params, maxNorm))})
}

// gradKareToplami gradyanların kareler toplamını verir. Toplu kareler toplamı veren
// (SquareSummer) arka uçlarda aynı arka uçtaki ardışık gradyanlar tek çağrıda
// toplanır (GPU'da tek eşzamanlama); toplama sırası parametre sırasıdır.
func gradKareToplami(params []*Tensor) float64 {
	var s float64
	for i := 0; i < len(params); {
		p := params[i]
		if p.grad == nil {
			i++
			continue
		}
		ss, ok := p.be.(SquareSummer)
		if !ok {
			s += p.be.SumSquares(p.grad)
			i++
			continue
		}
		var grup []Storage
		for ; i < len(params) && (params[i].grad == nil || params[i].be == p.be); i++ {
			if params[i].grad != nil {
				grup = append(grup, params[i].grad)
			}
		}
		s += ss.SumSquaresAll(grup)
	}
	return s
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

// durumTensoru s'yi p'nin şeklinde tensör olarak verir: kopya ise bitişik bir kopyası
// (State), değilse depolamanın kendisinin görünümü (DurumGorunumu).
func durumTensoru(p *Tensor, s Storage, kopya bool) *Tensor {
	if !kopya {
		return fromStorage(p.be, s, p.shape)
	}
	return durumKopya(p, s)
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
func (a *anaAgirliklar) durum(st map[string]*Tensor, params []*Tensor, kopya bool) {
	for k, p := range params {
		if a.ana[k] != nil && !a.kapali {
			st[fmt.Sprintf("ana.%d", k)] = durumTensoru(p, a.ana[k], kopya)
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
func (o *AdamW) State() map[string]*Tensor { return o.durum(true) }

// DurumGorunumu State gibidir ama kopya almaz: moment ve ana kopya tensörleri
// iyileştiricinin kendi depolarının görünümleridir (adım sayaçları yeni tensördür). Yalnızca
// okunmalı ve bir sonraki Step'ten önce kullanılmalıdır. Kontrol noktası durumu cihazda
// ikinci bir kopya ayırmadan konağa indirmek için kullanır (büyük modelde GPU belleği).
func (o *AdamW) DurumGorunumu() map[string]*Tensor { return o.durum(false) }

func (o *AdamW) durum(kopya bool) map[string]*Tensor {
	st := map[string]*Tensor{"adim": sayacTensoru(o.adim)}
	for k, p := range o.Params {
		if o.m[k] != nil {
			st[fmt.Sprintf("m.%d", k)] = durumTensoru(p, o.m[k], kopya)
			st[fmt.Sprintf("v.%d", k)] = durumTensoru(p, o.v[k], kopya)
		}
		if o.adimlar[k] != 0 {
			st[fmt.Sprintf("adim.%d", k)] = sayacTensoru(o.adimlar[k])
		}
	}
	o.ana.durum(st, o.Params, kopya)
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
func (o *SGD) State() map[string]*Tensor { return o.durum(true) }

// DurumGorunumu State gibidir ama kopya almaz (bkz. AdamW.DurumGorunumu).
func (o *SGD) DurumGorunumu() map[string]*Tensor { return o.durum(false) }

func (o *SGD) durum(kopya bool) map[string]*Tensor {
	st := map[string]*Tensor{}
	for k, p := range o.Params {
		if o.hiz[k] != nil {
			st[fmt.Sprintf("hiz.%d", k)] = durumTensoru(p, o.hiz[k], kopya)
		}
	}
	o.ana.durum(st, o.Params, kopya)
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

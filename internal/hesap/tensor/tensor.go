// Package tensor, TR dilinin yapay zekâ katmanı için n-boyutlu tensör ve
// ters yönlü otomatik türev çekirdeğidir. Yalnızca Go standart kütüphanesini
// kullanır.
//
// Veri tipi olarak float32 seçilmiştir: dil modeli eğitiminde bellek ve bellek
// bant genişliği asıl darboğazdır; float32 float64'e göre yarı bellekle iki kat
// eleman taşır, önbelleğe ve hızlandırıcı belleğine iki kat veri sığdırır.
// Hassasiyetin önemli olduğu yerlerde (toplam, ortalama, softmax, normlar,
// çapraz entropi) arka uçlar ara birikimi float64 ile yapar.
//
// Cihaz soyutlaması: tensör verisi bir cihaza ait Storage içinde durur ve tüm
// hesaplar Backend arayüzü üzerinden yapılır (bkz. cihaz.go). Bu dosyadaki ve
// otomatik türevdeki kod cihazdan bağımsızdır; yalnızca şekil/adım bilgisi
// tutar. CPU arka ucu (cpu_*.go) başvuru uygulamasıdır.
//
// Bellek düzeni: her tensör depolamaya bir ofset, şekil (shape) ve adımlar
// (strides) ile bakar. Görünüm işlemleri (Reshape, Transpose, Permute, Slice)
// veriyi kopyalamaz; yalnızca yeni ofset/adım üretir. Gradyanlar her zaman
// tensörün kendi şekline göre bitişik ayrı bir depolamada tutulur.
//
// Katman: hesap çekirdeği (internal/hesap); yalnızca internal/hesap paketlerini içe aktarır.
package tensor

import (
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
)

// Tensor n-boyutlu float32 dizisi ve otomatik türev düğümüdür.
type Tensor struct {
	storage Storage // paylaşılan depolama (bir cihazda)
	be      Backend // depolamanın cihazının arka ucu
	offset  int     // depodaki ilk elemanın konumu
	shape   []int   // her eksenin uzunluğu
	strides []int   // her eksende bir adım ilerlemenin depodaki karşılığı
	tur     DType   // eleman türü (depolamanın türü)
	// islem, düğümü üreten işlemin profil sayacı türüdür (geri yayılım bu adla sayılır).
	islem Islem

	grad         Storage // bitişik gradyan (şekil = shape), gerekince ayrılır
	requiresGrad bool

	// Hesap çizgesi: bu tensörü üreten işlemin girdileri ve geri yayılım işlevi.
	// Yaprak tensörlerde (kullanıcının oluşturdukları) ikisi de boştur.
	parents  []*Tensor
	backward func()
	op       string // hata ayıklama için işlem adı
	// kayitli, geri yayılımın okuyacağı girdilerin ileri hesap anındaki
	// depolama sürümleri (yerinde değişiklik denetimi).
	kayitli []kayitliSurum
	// serbest, geri yayılımdan sonra çizge bağları bırakılmış bir ara tensörü
	// işaretler; böyle bir tensör üzerinden ikinci kez geri yayılım yapılamaz.
	serbest bool
	// dis, dış (çekirdek dışında yazılmış) bir işlemin düğümünde geri hesabı taşır
	// (bkz. ozel_turev.go).
	dis *disDugum
	// geride, düğümün şu an süren bir geri yayılımın çizgesinde olduğunu işaretler; aynı
	// düğümden ikinci bir geri yayılım (ör. dış düğümün geri hesabı kilidi bıraktığında
	// başka görevin başlattığı) açık hata verir.
	geride atomic.Bool
}

// ----------------------------------------------------------------------------
// Şekil yardımcıları
// ----------------------------------------------------------------------------

// numelOf bir şeklin toplam eleman sayısını verir (0 boyutlu şekil için 1).
func numelOf(shape []int) int {
	n := 1
	for _, d := range shape {
		n *= d
	}
	return n
}

// contiguousStrides satır öncelikli (son eksen en hızlı) adımları üretir.
func contiguousStrides(shape []int) []int {
	st := make([]int, len(shape))
	s := 1
	for i := len(shape) - 1; i >= 0; i-- {
		st[i] = s
		s *= shape[i]
	}
	return st
}

// isContiguous şekil/adımların satır öncelikli ve boşluksuz olup olmadığı.
func isContiguous(shape, strides []int) bool {
	s := 1
	for i := len(shape) - 1; i >= 0; i-- {
		if shape[i] != 1 && strides[i] != s {
			return false
		}
		s *= shape[i]
	}
	return true
}

func copyInts(a []int) []int {
	b := make([]int, len(a))
	copy(b, a)
	return b
}

func sameShape(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// normAxis negatif eksen indeksini (sondan sayma) pozitife çevirir.
func normAxis(axis, ndim int) int {
	if axis < 0 {
		axis += ndim
	}
	if axis < 0 || axis >= ndim {
		panic(fmt.Sprintf("tensor: eksen %d, %d boyutlu tensör için geçersiz", axis, ndim))
	}
	return axis
}

// ----------------------------------------------------------------------------
// Oluşturucular
// ----------------------------------------------------------------------------

// fromStorage depolamayı bitişik tensör olarak sarar.
func fromStorage(be Backend, s Storage, shape []int) *Tensor {
	sh := copyInts(shape)
	return &Tensor{storage: s, be: be, shape: sh, strides: contiguousStrides(sh), tur: s.DType()}
}

// zerosOn verilen arka uçta sıfırlarla dolu bitişik float32 tensör ayırır.
func zerosOn(be Backend, shape []int) *Tensor { return zerosOf(isOlusturma, be, shape, Float32) }

// zerosOf verilen arka uçta dt türünde sıfırlarla dolu bitişik tensör ayırır; is,
// ayırma sınırı aşılırsa hata mesajında adlandırılan işlemdir (bkz. bellek_siniri.go).
func zerosOf(is islem, be Backend, shape []int, dt DType) *Tensor {
	return fromStorage(be, be.Alloc(ayirmaBoyuOn(be, is, shape, dt), dt), shape)
}

// bosAyirici, içeriği belirsiz (sıfırlanmamış) depolama verebilen arka
// uçların iç arayüzüdür. Backend.Alloc sözleşmesi (sıfırlanmış depolama)
// değişmez; bu yol yalnızca çekirdeğin çıktının her elemanını yazdığı
// işlemlerde (emptyOf) kullanılır ve havuzdan gelen depolamayı yeniden
// sıfırlama maliyetini önler (karakter modelinde havuz sıfırlaması CPU
// zamanının ~%4'üydü; Go'nun yeni ayırmalarda yaptığı sıfırlama ayrıdır ve
// bu yolla önlenemez).
type bosAyirici interface {
	allocBos(n int, dt DType) Storage
}

// emptyOf, her elemanı çağıranca yazılacak bitişik bir tensör ayırır; içerik
// belirsizdir (arka uç bosAyirici değilse sıfırdır). Yalnızca çekirdek tüm
// çıktıyı yazıyorsa kullanılmalıdır (ör. eleman bazlı işlemler, kopyalar,
// softmax, normalleştirmeler); biriktiren çekirdekler (çarpım, gradyanlar)
// zerosOf kullanır.
func emptyOf(is islem, be Backend, shape []int, dt DType) *Tensor {
	return fromStorage(be, allocBos(be, ayirmaBoyuOn(be, is, shape, dt), dt), shape)
}

// allocBos be destekliyorsa sıfırlanmamış, değilse sıfırlanmış depolama verir.
func allocBos(be Backend, n int, dt DType) Storage {
	switch b := be.(type) {
	case bosAyirici:
		return b.allocBos(n, dt)
	case UninitAllocator:
		return b.AllocUninit(n, dt)
	}
	return be.Alloc(n, dt)
}

func defaultBackend() Backend { return BackendFor(DefaultDevice()) }

// NewOn veriyi kopyalayarak verilen cihazda tensör oluşturur.
func NewOn(dev Device, data []float32, shape ...int) *Tensor {
	be := BackendFor(dev)
	sy := sayacBasla(be, IslemOlusturma).baslat()
	if len(data) != ayirmaBoyuOn(be, isOlusturma, shape, Float32) {
		panic(fmt.Sprintf("tensor: %d eleman %v şekline uymuyor", len(data), shape))
	}
	t := fromStorage(be, be.FromHost(data, Float32), shape)
	sy.bitir()
	return t
}

// New veriyi kopyalayarak varsayılan cihazda tensör oluşturur.
func New(data []float32, shape ...int) *Tensor {
	return NewOn(DefaultDevice(), data, shape...)
}

// FromFloat64 float64 dilimden tensör oluşturur.
func FromFloat64(data []float64, shape ...int) *Tensor {
	d := make([]float32, len(data))
	for i, v := range data {
		d[i] = float32(v)
	}
	return New(d, shape...)
}

// Scalar 0 boyutlu (skaler) tensör oluşturur.
func Scalar(v float32) *Tensor { return New([]float32{v}) }

// Zeros sıfırlarla dolu tensör.
func Zeros(shape ...int) *Tensor { return zerosOn(defaultBackend(), shape) }

// ZerosOn verilen cihazda sıfırlarla dolu tensör.
func ZerosOn(dev Device, shape ...int) *Tensor { return zerosOn(BackendFor(dev), shape) }

// Ones birlerle dolu tensör.
func Ones(shape ...int) *Tensor { return Full(1, shape...) }

// Full tüm elemanları v olan tensör.
func Full(v float32, shape ...int) *Tensor {
	t := Zeros(shape...)
	t.be.Fill(t.storage, v)
	return t
}

// Arange 0, 1, ..., n-1 değerlerini içeren 1 boyutlu tensör.
func Arange(n int) *Tensor {
	d := make([]float32, ayirmaBoyu(isOlusturma, []int{n}, Float32))
	for i := range d {
		d[i] = float32(i)
	}
	return New(d, n)
}

// Randn standart normal dağılımdan örneklenmiş tensör (örnekleme konakta
// yapılır, sonuç varsayılan cihaza aktarılır).
func Randn(rng *rand.Rand, shape ...int) *Tensor {
	d := make([]float32, ayirmaBoyu(isOlusturma, shape, Float32))
	for i := range d {
		d[i] = float32(rng.NormFloat64())
	}
	return New(d, shape...)
}

// Rand [lo, hi) aralığında düzgün dağılımlı tensör.
func Rand(rng *rand.Rand, lo, hi float32, shape ...int) *Tensor {
	d := make([]float32, ayirmaBoyu(isOlusturma, shape, Float32))
	for i := range d {
		d[i] = lo + (hi-lo)*rng.Float32()
	}
	return New(d, shape...)
}

// ----------------------------------------------------------------------------
// Erişimciler
// ----------------------------------------------------------------------------

// view tensörün arka uca verilecek görünüm tanımı.
func (t *Tensor) view() View {
	return View{S: t.storage, Offset: t.offset, Shape: t.shape, Strides: t.strides}
}

// Device tensörün bulunduğu cihaz.
func (t *Tensor) Device() Device { return t.be.Device() }

// Shape tensörün şeklinin bir kopyasını verir.
func (t *Tensor) Shape() []int { return copyInts(t.shape) }

// Strides adımların bir kopyasını verir.
func (t *Tensor) Strides() []int { return copyInts(t.strides) }

// Dim boyut (eksen) sayısı.
func (t *Tensor) Dim() int { return len(t.shape) }

// Size verilen eksenin uzunluğu (negatif eksen sondan sayar).
func (t *Tensor) Size(axis int) int { return t.shape[normAxis(axis, len(t.shape))] }

// Numel toplam eleman sayısı.
func (t *Tensor) Numel() int { return numelOf(t.shape) }

// IsContiguous tensörün depoda satır öncelikli ve boşluksuz durup durmadığı.
func (t *Tensor) IsContiguous() bool { return isContiguous(t.shape, t.strides) }

// RequiresGrad gradyan izlenip izlenmediği.
func (t *Tensor) RequiresGrad() bool { return t.requiresGrad }

// SetRequiresGrad yaprak tensör için gradyan izlemeyi açar/kapatır; zincirleme
// kullanım için tensörün kendisini döndürür.
func (t *Tensor) SetRequiresGrad(b bool) *Tensor {
	if t.backward != nil && !b {
		panic("tensor: ara (yaprak olmayan) tensörde gradyan izleme kapatılamaz; Detach kullanın")
	}
	t.requiresGrad = b
	return t
}

// IsLeaf tensörün bir işlem sonucu değil, doğrudan oluşturulmuş olup olmadığı.
func (t *Tensor) IsLeaf() bool { return t.backward == nil }

// Detach aynı veriyi paylaşan, çizgeden kopuk ve gradyansız bir görünüm verir.
func (t *Tensor) Detach() *Tensor {
	return &Tensor{storage: t.storage, be: t.be, offset: t.offset, shape: copyInts(t.shape), strides: copyInts(t.strides), tur: t.tur}
}

// DType tensörün eleman türü.
func (t *Tensor) DType() DType { return t.tur }

// Version tensörün depolamasının sürüm sayacı (yerinde her değişiklikte artar;
// aynı depolamayı paylaşan görünümlerde ortaktır).
func (t *Tensor) Version() uint64 { return t.storage.Version() }

// Data elemanların mantıksal sırayla (satır öncelikli) konaktaki bir kopyasını verir.
func (t *Tensor) Data() []float32 {
	sy := sayacBasla(t.be, IslemKonagaOkuma).baslat()
	d := t.be.ToHost(t.view())
	sy.bitir()
	return d
}

// Float64s elemanları float64 olarak verir.
func (t *Tensor) Float64s() []float64 {
	d := t.Data()
	out := make([]float64, len(d))
	for i, v := range d {
		out[i] = float64(v)
	}
	return out
}

// Item tek elemanlı tensörün değerini verir.
func (t *Tensor) Item() float32 {
	if t.Numel() != 1 {
		panic(fmt.Sprintf("tensor: Item yalnızca tek elemanlı tensörde çalışır, şekil %v", t.shape))
	}
	return t.Data()[0]
}

// At verilen çok boyutlu indeksteki değeri verir.
func (t *Tensor) At(idx ...int) float32 {
	return t.be.ToHost(View{S: t.storage, Offset: t.index(idx)})[0]
}

// Set verilen indeksteki değeri yazar (çizgeye kaydedilmez; yaprak verisi
// düzenlemek içindir). Yerinde değişiklik olduğundan depolamanın sürümü artar:
// bu tensörü ileri hesapta kullanmış ve henüz geri yayılmamış bir çizge varsa
// geri yayılım hata verir.
func (t *Tensor) Set(v float32, idx ...int) {
	t.be.Copy(t.storage, t.index(idx), View{S: t.be.FromHost([]float32{v}, Float32)})
	t.storage.BumpVersion()
}

// CopyFrom src'nin elemanlarını t'nin elemanlarına yerinde yazar; t bitişik olmayan bir
// görünüm olabilir (yazma kaynak tensöre yansır). src t'nin şekline yayınlanabilmelidir
// (ör. 0 boyutlu tensör tüm elemanlara yazılır); türler farklıysa dönüştürülür. Tek bir
// arka uç kopyasıdır, çizgeye kaydedilmez ve depolamanın sürümü bir kez artar (Set gibi).
// src t ile aynı depolamayı paylaşıyorsa (örtüşebilir) önce kopyalanır.
func (t *Tensor) CopyFrom(src *Tensor) {
	sameDevice("CopyFrom", t, src)
	nd := len(t.shape)
	uygun := len(src.shape) <= nd
	for i := 1; uygun && i <= len(src.shape); i++ {
		if d := src.shape[len(src.shape)-i]; d != 1 && d != t.shape[nd-i] {
			uygun = false
		}
	}
	if !uygun {
		panic(fmt.Sprintf("tensor: CopyFrom: %v şeklindeki kaynak %v şeklindeki hedefe yazılamaz (şekiller aynı ya da "+
			"kaynak hedefe yayınlanabilir olmalı)", src.shape, t.shape))
	}
	sv := src.view()
	if src.storage == t.storage {
		s := allocBos(src.be, src.Numel(), src.tur)
		src.be.Copy(s, 0, sv)
		sv = View{S: s, Shape: src.shape, Strides: contiguousStrides(src.shape)}
	}
	sv.Strides = broadcastStrides(sv.Strides, sv.Shape, t.shape)
	sv.Shape = t.shape
	t.be.CopyStrided(t.view(), sv)
	t.storage.BumpVersion()
}

func (t *Tensor) index(idx []int) int {
	if len(idx) != len(t.shape) {
		panic(fmt.Sprintf("tensor: %d indeks verildi, tensör %d boyutlu", len(idx), len(t.shape)))
	}
	p := t.offset
	for i, v := range idx {
		if v < 0 {
			v += t.shape[i]
		}
		if v < 0 || v >= t.shape[i] {
			panic(fmt.Sprintf("tensor: indeks %v şekil %v dışında", idx, t.shape))
		}
		p += v * t.strides[i]
	}
	return p
}

// Grad birikmiş gradyanı tensör olarak verir (yoksa nil). Dönen tensör
// gradyan depolamasının canlı bir takma adıdır (kopya değildir): sonraki geri
// yayılımlar, ZeroGrad ve ClipGradNorm onu da değiştirir. Bu işlemler deponun
// sürümünü artırdığından, dönen tensörü kaydetmiş bir çizge gradyan değiştikten
// sonra geri yayılırsa açık hata verir; saklamak için Clone kullanın.
func (t *Tensor) Grad() *Tensor {
	if t.grad == nil {
		return nil
	}
	return fromStorage(t.be, t.grad, t.shape)
}

// ZeroGrad gradyanı sıfırlar. Gradyan deposunun sürümü artar: Grad ile alınmış (deponun
// takma adı olan) bir gradyanı kaydetmiş çizgenin geri yayılımı hata verir.
func (t *Tensor) ZeroGrad() {
	if t.grad != nil {
		sy := sayacBasla(t.be, IslemGradyanSifirlama).baslat()
		t.be.Fill(t.grad, 0)
		sy.bitir()
		t.grad.BumpVersion()
	}
}

// To tensörü verilen cihaza taşır. Aynı cihazdaysa kendisini döndürür; aksi
// halde veriyi kopyalar (türevlenebilir: gradyan kaynak cihaza geri taşınır).
// Aktarım konak üzerinden yapılır (cihazlar arası doğrudan kopya yoktur).
func (t *Tensor) To(dev Device) *Tensor {
	if t.Device() == dev {
		return t
	}
	hedef := BackendFor(dev)
	sy := sayacBasla(hedef, IslemCihazaTasima).baslat()
	out := fromStorage(hedef, hedef.FromHost(t.be.ToHost(t.view()), t.tur), t.shape)
	r := track(out, "to", []*Tensor{t}, func() {
		g := t.be.FromHost(hedef.ToHost(View{S: out.grad, Shape: out.shape, Strides: contiguousStrides(out.shape)}), gradTuru(t.tur))
		t.be.AddTo(t.accGrad(), g, 1)
	})
	sy.bitir()
	return r
}

// String kısa bir metin gösterimi verir.
func (t *Tensor) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tensor(şekil=%v, cihaz=%s", t.shape, t.Device())
	if t.tur != Float32 {
		fmt.Fprintf(&sb, ", tür=%s", t.tur)
	}
	if t.requiresGrad {
		sb.WriteString(", gradyan")
	}
	sb.WriteString(") ")
	d := t.Data()
	const sinir = 16
	sb.WriteString("[")
	for i, v := range d {
		if i == sinir {
			fmt.Fprintf(&sb, " ... (%d eleman)", len(d))
			break
		}
		if i > 0 {
			sb.WriteString(" ")
		}
		fmt.Fprintf(&sb, "%.4g", v)
	}
	sb.WriteString("]")
	return sb.String()
}

// sameDevice işlem girdilerinin aynı cihazda olduğunu denetler ve arka ucu verir.
func sameDevice(op string, ts ...*Tensor) Backend {
	be := ts[0].be
	for _, t := range ts[1:] {
		if t != nil && t.be.Device() != be.Device() {
			panic(fmt.Sprintf("tensor: %s girdileri farklı cihazlarda (%s, %s); önce To ile taşıyın", op, be.Device(), t.Device()))
		}
	}
	return be
}

// contigView tensörün bitişik bir görünümünü verir (gerekirse arka uçta
// kopyalayarak; kopya çizgeye kaydedilmez).
func (t *Tensor) contigView() View {
	if t.IsContiguous() {
		return t.view()
	}
	s := allocBos(t.be, t.Numel(), t.tur)
	t.be.Copy(s, 0, t.view())
	return View{S: s, Shape: t.shape, Strides: contiguousStrides(t.shape)}
}

// denseStorage ofseti 0 olan bitişik bir depolama verir (gerekirse kopyalar).
func (t *Tensor) denseStorage() Storage {
	if t.IsContiguous() && t.offset == 0 {
		return t.storage
	}
	s := allocBos(t.be, t.Numel(), t.tur)
	t.be.Copy(s, 0, t.view())
	return s
}

// gradView gradyan depolamasını bitişik görünüm olarak verir.
func (t *Tensor) gradView() View {
	return View{S: t.grad, Shape: t.shape, Strides: contiguousStrides(t.shape)}
}

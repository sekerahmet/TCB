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
// Layer: compute core (internal/compute); yalnızca internal/compute paketlerini içe aktarır.
package tensor

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tensor n-boyutlu float32 dizisi ve otomatik türev düğümüdür.
type Tensor struct {
	storage Storage // paylaşılan depolama (bir cihazda)
	be      Backend // depolamanın cihazının arka ucu
	offset  int     // depodaki ilk elemanın konumu
	shape   []int   // her eksenin uzunluğu
	strides []int   // her eksende bir adım ilerlemenin depodaki karşılığı
	dtyp    DType   // eleman türü (depolamanın türü)
	// n, tensörün otomatik türev düğümüdür (gradyan, geri hesap, girdiler; autograd.go).
	// Gradyan istemeyen tensörde nil'dir. Düğüm verinin kendisini tutmaz: çizge yalnızca
	// düğümleri birbirine bağlar, veri yalnızca geri hesabın okuyacağı tensörler için
	// (kaydedilenler) saklanır. Kullanıcının bıraktığı ve kimsenin kaydetmediği bir ara
	// sonucun verisi böylece geri yayılımı beklemeden serbest kalır (PyTorch'un grad_fn
	// ile saved tensors ayrımı).
	n *node
	// Ömür (lifetime.go): oluşturulma sırası, bırakıldı mı ve başlığı saran değer sayısı.
	serial  uint64
	dropped atomic.Bool
	wraps   atomic.Int32
	// sym iz tensörünün (kaynaştırma, fuse_trace.go) program düğümüdür; iz tensörünün
	// depolaması yoktur.
	sym *FuseNode
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

// SameShape iki şeklin aynı olup olmadığıdır (gpu ve ml de kullanır).
func SameShape(a, b []int) bool { return slices.Equal(a, b) }

func sameShape(a, b []int) bool { return SameShape(a, b) }

// normAxis negatif eksen indeksini (sondan sayma) pozitife çevirir.
func normAxis(axis, ndim int) int {
	if axis < 0 {
		axis += ndim
	}
	if axis < 0 || axis >= ndim {
		panic(messages.New(messages.H3066, axis, ndim))
	}
	return axis
}

// ----------------------------------------------------------------------------
// Oluşturucular
// ----------------------------------------------------------------------------

// fromStorage depolamayı bitişik tensör olarak sarar.
func fromStorage(be Backend, s Storage, shape []int) *Tensor {
	sh := copyInts(shape)
	return newHeader(&Tensor{storage: s, be: be, shape: sh, strides: contiguousStrides(sh), dtyp: s.DType()})
}

// zerosOn verilen arka uçta sıfırlarla dolu bitişik float32 tensör ayırır.
func zerosOn(be Backend, shape []int) *Tensor { return zerosOf(isCreate, be, shape, Float32) }

// zerosOf verilen arka uçta dt türünde sıfırlarla dolu bitişik tensör ayırır; is,
// ayırma sınırı aşılırsa hata mesajında adlandırılan işlemdir (bkz. bellek_siniri.go).
func zerosOf(is opKind, be Backend, shape []int, dt DType) *Tensor {
	return fromStorage(be, be.Alloc(allocSizeOn(be, is, shape, dt), dt), shape)
}

// emptyAllocator, içeriği belirsiz (sıfırlanmamış) depolama verebilen arka
// uçların iç arayüzüdür. Backend.Alloc sözleşmesi (sıfırlanmış depolama)
// değişmez; bu yol yalnızca çekirdeğin çıktının her elemanını yazdığı
// işlemlerde (emptyOf) kullanılır ve havuzdan gelen depolamayı yeniden
// sıfırlama maliyetini önler (karakter modelinde havuz sıfırlaması CPU
// zamanının ~%4'üydü; Go'nun yeni ayırmalarda yaptığı sıfırlama ayrıdır ve
// bu yolla önlenemez).
type emptyAllocator interface {
	allocUninit(n int, dt DType) Storage
}

// emptyOf, her elemanı çağıranca yazılacak bitişik bir tensör ayırır; içerik
// belirsizdir (arka uç bosAyirici değilse sıfırdır). Yalnızca çekirdek tüm
// çıktıyı yazıyorsa kullanılmalıdır (ör. eleman bazlı işlemler, kopyalar,
// softmax, normalleştirmeler); biriktiren çekirdekler (çarpım, gradyanlar)
// zerosOf kullanır.
func emptyOf(is opKind, be Backend, shape []int, dt DType) *Tensor {
	return fromStorage(be, allocUninit(be, allocSizeOn(be, is, shape, dt), dt), shape)
}

// allocUninit be destekliyorsa sıfırlanmamış, değilse sıfırlanmış depolama verir.
// scalarStorage değeri v olan tek elemanlı float32 depolama. Cihazda doldurulur (konaktan
// yüklemez): GPU kaydında da kurulabilir, değer kayda sabit olarak girer.
func scalarStorage(be Backend, v float32) Storage {
	s := allocUninit(be, 1, Float32)
	be.Fill(s, v)
	return s
}

func allocUninit(be Backend, n int, dt DType) Storage {
	switch b := be.(type) {
	case emptyAllocator:
		return b.allocUninit(n, dt)
	case UninitAllocator:
		return b.AllocUninit(n, dt)
	}
	return be.Alloc(n, dt)
}

func defaultBackend() Backend { return BackendFor(DefaultDevice()) }

// NewOn veriyi kopyalayarak verilen cihazda tensör oluşturur.
func NewOn(dev Device, data []float32, shape ...int) *Tensor {
	be := BackendFor(dev)
	sy := counterStart(be, OpKindCreate).start()
	if len(data) != allocSizeOn(be, isCreate, shape, Float32) {
		panic(messages.New(messages.H3035, len(data), shape))
	}
	t := fromStorage(be, be.FromHost(data, Float32), shape)
	sy.finish()
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

// FullOn verilen cihazda tüm elemanları v olan tensör. Konaktan veri yüklemez (cihazda
// doldurulur): GPU kaydında da kurulabilir.
func FullOn(dev Device, v float32, shape ...int) *Tensor {
	t := ZerosOn(dev, shape...)
	t.be.Fill(t.storage, v)
	return t
}

// Arange 0, 1, ..., n-1 değerlerini içeren 1 boyutlu tensör.
func Arange(n int) *Tensor {
	d := make([]float32, allocSize(isCreate, []int{n}, Float32))
	for i := range d {
		d[i] = float32(i)
	}
	return New(d, n)
}

// Randn standart normal dağılımdan örneklenmiş tensör (örnekleme konakta
// yapılır, sonuç varsayılan cihaza aktarılır).
func Randn(rng *rand.Rand, shape ...int) *Tensor {
	d := make([]float32, allocSize(isCreate, shape, Float32))
	for i := range d {
		d[i] = float32(rng.NormFloat64())
	}
	return New(d, shape...)
}

// ----------------------------------------------------------------------------
// Erişimciler
// ----------------------------------------------------------------------------

// view tensörün arka uca verilecek görünüm tanımı.
func (t *Tensor) view() View {
	t.mustLive()
	if t.sym != nil {
		panic(messages.New(messages.H3107)) // kaynaştırılamayan işlem verinin kendisini istedi
	}
	return View{S: t.storage, Offset: t.offset, Shape: t.shape, Strides: t.strides}
}

// shapeView verisiz görünümdür (depolama nil): geri hesabın yalnızca şeklini ve adımlarını
// okuduğu, verisini kaydetmediği bir girdi için (verisi çizgede tutulmaz).
func (t *Tensor) shapeView() View {
	return View{Offset: t.offset, Shape: t.shape, Strides: t.strides}
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
func (t *Tensor) RequiresGrad() bool { return t.n.wants() }

// SetRequiresGrad yaprak tensör için gradyan izlemeyi açar/kapatır; zincirleme
// kullanım için tensörün kendisini döndürür.
func (t *Tensor) SetRequiresGrad(b bool) *Tensor {
	if !t.IsLeaf() && !b {
		panic(messages.New(messages.H3067))
	}
	if t.n == nil {
		if !b {
			return t
		}
		t.n = newNode(t)
	}
	t.n.requiresGrad = b
	return t
}

// IsLeaf tensörün bir işlem sonucu değil, doğrudan oluşturulmuş olup olmadığı.
func (t *Tensor) IsLeaf() bool { return t.n == nil || t.n.backward == nil }

// Detach aynı veriyi paylaşan, çizgeden kopuk ve gradyansız bir görünüm verir.
func (t *Tensor) Detach() *Tensor {
	return newHeader(&Tensor{storage: t.storage, be: t.be, offset: t.offset, shape: copyInts(t.shape), strides: copyInts(t.strides), dtyp: t.dtyp})
}

// DType tensörün eleman türü.
func (t *Tensor) DType() DType { return t.dtyp }

// SharesStorage t ile u aynı depolamaya bakıyor mu (biri öbürünün görünümüyse ya da ikisi
// aynı tensörün görünümleriyse).
func (t *Tensor) SharesStorage(u *Tensor) bool { return t.storage == u.storage }

// Version tensörün depolamasının sürüm sayacı (yerinde her değişiklikte artar;
// aynı depolamayı paylaşan görünümlerde ortaktır).
func (t *Tensor) Version() uint64 { return t.storage.Version() }

// Data elemanların mantıksal sırayla (satır öncelikli) konaktaki bir kopyasını verir.
func (t *Tensor) Data() []float32 {
	t.mustLive()
	if t.sym != nil {
		panic(messages.New(messages.H3107))
	}
	sy := counterStart(t.be, OpKindToHost).start()
	d := t.be.ToHost(t.view())
	sy.finish()
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
		panic(messages.New(messages.H3068, t.shape))
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
	sy := counterStart(t.be, OpKindAssign).start()
	t.be.Copy(t.storage, t.index(idx), View{S: scalarStorage(t.be, v)})
	t.storage.BumpVersion()
	sy.finish()
}

// CopyFrom src'nin elemanlarını t'nin elemanlarına yerinde yazar; t bitişik olmayan bir
// görünüm olabilir (yazma kaynak tensöre yansır). src t'nin şekline yayınlanabilmelidir
// (ör. 0 boyutlu tensör tüm elemanlara yazılır); türler farklıysa dönüştürülür. Tek bir
// arka uç kopyasıdır, çizgeye kaydedilmez ve depolamanın sürümü bir kez artar (Set gibi).
// src t ile aynı depolamayı paylaşıyorsa (örtüşebilir) önce kopyalanır.
func (t *Tensor) CopyFrom(src *Tensor) {
	sameDevice("CopyFrom", t, src)
	nd := len(t.shape)
	suitable := len(src.shape) <= nd
	for i := 1; suitable && i <= len(src.shape); i++ {
		if d := src.shape[len(src.shape)-i]; d != 1 && d != t.shape[nd-i] {
			suitable = false
		}
	}
	if !suitable {
		panic(messages.New(messages.H3069, src.shape, t.shape))
	}
	sy := counterStart(t.be, OpKindAssign).start()
	defer sy.finish()
	sv := src.view()
	if src.storage == t.storage {
		s := allocUninit(src.be, src.Numel(), src.dtyp)
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
		panic(messages.New(messages.H3070, len(idx), len(t.shape)))
	}
	p := t.offset
	for i, v := range idx {
		if v < 0 {
			v += t.shape[i]
		}
		if v < 0 || v >= t.shape[i] {
			panic(messages.New(messages.H3071, idx, t.shape))
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
	if t.n == nil || t.n.grad == nil {
		return nil
	}
	return fromStorage(t.be, t.n.grad, t.shape)
}

// gradStorage gradyan depolamasıdır (gradyan yoksa ya da tensör gradyan izlemiyorsa nil).
func (t *Tensor) gradStorage() Storage {
	if t.n == nil {
		return nil
	}
	return t.n.grad
}

// ZeroGrad gradyanı sıfırlar. Gradyan deposunun sürümü artar: Grad ile alınmış (deponun
// takma adı olan) bir gradyanı kaydetmiş çizgenin geri yayılımı hata verir.
func (t *Tensor) ZeroGrad() {
	if t.n != nil && t.n.grad != nil {
		sy := counterStart(t.be, OpKindZeroGrad).start()
		t.be.Fill(t.n.grad, 0)
		sy.finish()
		t.n.grad.BumpVersion()
	}
}

// To tensörü verilen cihaza taşır. Aynı cihazdaysa kendisini döndürür; aksi
// halde veriyi kopyalar (türevlenebilir: gradyan kaynak cihaza geri taşınır).
// Aktarım konak üzerinden yapılır (cihazlar arası doğrudan kopya yoktur).
func (t *Tensor) To(dev Device) *Tensor {
	if t.Device() == dev {
		return t
	}
	target := BackendFor(dev)
	sy := counterStart(target, OpKindToDevice).start()
	out := fromStorage(target, target.FromHost(t.be.ToHost(t.view()), t.dtyp), t.shape)
	src := t.be
	r := trackPass(out, "to", t, func(o *node) {
		in := o.parents[0]
		g := src.FromHost(target.ToHost(o.gradView()), in.gradDType())
		src.AddTo(in.accGrad(), g, 1)
	})
	sy.finish()
	return r
}

// String hata ayıklama için kısa bir metin gösterimi verir (İngilizce; dilin yazdırma biçimi ayrıdır).
func (t *Tensor) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tensor(shape=%v, device=%s", t.shape, t.Device())
	if t.dtyp != Float32 {
		fmt.Fprintf(&sb, ", dtype=%s", t.dtyp)
	}
	if t.RequiresGrad() {
		sb.WriteString(", grad")
	}
	sb.WriteString(") ")
	d := t.Data()
	const limit = 16
	sb.WriteString("[")
	for i, v := range d {
		if i == limit {
			fmt.Fprintf(&sb, " ... (%d elements)", len(d))
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
			panic(messages.New(messages.H3072, op, be.Device(), t.Device()))
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
	s := allocUninit(t.be, t.Numel(), t.dtyp)
	t.be.Copy(s, 0, t.view())
	return View{S: s, Shape: t.shape, Strides: contiguousStrides(t.shape)}
}

// copyRelease v, t'nin bitişik kopyasıysa (contigView'ün ayırdığı) onu arka uca iade eden
// işlevi verir; t'nin kendi depolamasıysa hiçbir şey yapmaz. t'yi yakalamaz (geri hesap
// kapanışlarında kullanılır: kapanış verisini kaydetmediği bir tensörü tutmamalıdır).
func copyRelease(t *Tensor, v View) func() {
	if v.S == t.storage {
		return func() {}
	}
	be, s := t.be, v.S
	return func() { giveBack(be, s) }
}

// denseStorage ofseti 0 olan bitişik bir depolama verir (gerekirse kopyalar).
func (t *Tensor) denseStorage() Storage {
	if t.IsContiguous() && t.offset == 0 {
		return t.storage
	}
	s := allocUninit(t.be, t.Numel(), t.dtyp)
	t.be.Copy(s, 0, t.view())
	return s
}

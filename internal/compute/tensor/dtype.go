package tensor

import (
	"sync/atomic"
	"weak"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Veri türleri ve karışık hassasiyet (cihazdan bağımsız katman).
//
// Her tensörün bir eleman türü vardır (varsayılan Float32). Hesap türü,
// tensörlerin (ağırlıklar, etkinleştirmeler) türüdür; ör. bfloat16 seçilirse
// bellek ve bant genişliği yarıya iner. Biriktirme ayrıdır:
//   - Arka uçlar iç toplamaları (matris çarpımı, indirgemeler, normlar,
//     softmax, çapraz entropi) en az float32'de yapar; sonuç çıktı türüne bir
//     kez yuvarlanır.
//   - 16 bitlik yaprakların (parametrelerin) gradyanları GradDType türünde
//     (varsayılan float32), 16 bitlik ara sonuçlarınki kendi türünde tutulur;
//     dönüşüm ve görünümlerin çıktısı girdinin gradyan türünü izler. float32
//     tensörlerin gradyanları her zaman float32'dir (bkz. gradDType).
//   - İyileştiriciler 16 bitlik parametreler için float32 ana ağırlık kopyası
//     tutar (SetMasterWeights); güncelleme ana kopyada yapılır ve parametreye
//     yuvarlanarak yazılır, böylece küçük adımlar yuvarlamada kaybolmaz.
//   - Çapraz entropi kaybı her zaman float32'dir; MatMulOut ile 16 bitlik
//     girdilerden float32 çıktı alınabilir.

// gradDTypeSetting 16 bitlik yaprak tensörlerin gradyan türü.
var gradDTypeSetting atomic.Uint32 // DType; sıfır değer Float32

// SetGradDType 16 bitlik (bfloat16/float16) yaprak tensörlerin (parametrelerin)
// gradyan biriktirme türünü ayarlar (varsayılan Float32). Ara sonuçların gradyanı
// bu ayara bağlı değildir; float32 tensörlerin gradyanı her zaman float32'dir.
// Süreç geneli bir ayardır.
func SetGradDType(dt DType) {
	if !dt.IsFloat() {
		panic(messages.New(messages.H3034, dt))
	}
	gradDTypeSetting.Store(uint32(dt))
}

// GradDType 16 bitlik yaprak tensörlerin gradyan biriktirme türü.
func GradDType() DType { return DType(gradDTypeSetting.Load()) }

// resultDType iki işlenenli işlemin çıktı türü: türler aynıysa o tür, aksi halde
// float32 (daha geniş tür).
func resultDType(a, b DType) DType {
	if a == b {
		return a
	}
	return Float32
}

// NewOf veriyi dt türüne (en yakına-çifte yuvarlayarak) dönüştürüp varsayılan
// cihazda tensör oluşturur.
func NewOf(dt DType, data []float32, shape ...int) *Tensor {
	be := defaultBackend()
	if len(data) != allocSizeOn(be, isCreate, shape, dt) {
		panic(messages.New(messages.H3035, len(data), shape))
	}
	return fromStorage(be, be.FromHost(data, dt), shape)
}

// ZerosOf dt türünde sıfırlarla dolu tensör.
func ZerosOf(dt DType, shape ...int) *Tensor {
	return zerosOf(isCreate, defaultBackend(), shape, dt)
}

// Cast tensörün dt türüne dönüştürülmüş bitişik bir kopyasını verir (en
// yakına-çifte yuvarlama). Türevlenebilir: gradyan kaynak türün gradyanına
// geri aktarılır. Tür zaten dt ise tensörün kendisi döner.
func (t *Tensor) Cast(dt DType) *Tensor {
	if t.dtyp == dt {
		return t
	}
	if !dt.IsFloat() {
		panic(messages.New(messages.H3036, dt))
	}
	if t.sym != nil {
		return symCast(t, dt)
	}
	sy := counterStart(t.be, OpKindCast).start()
	out := emptyOf(isCast, t.be, t.shape, dt)
	t.be.Copy(out.storage, 0, t.view())
	r := trackPass(out, "cast", t, func(o *node) { handOffGrad(o.parents[0], o) })
	sy.finish()
	return r
}

// castMemo bir ara sonucun saklanan dönüşümü ve o andaki depolama sürümleri. Dönüşüm zayıf
// göstericiyle tutulur: önbellek onu canlı tutmaz (kimse kullanmıyorsa verisi serbest kalır,
// sonraki çağrı yeniden dönüştürür).
type castMemo struct {
	out      weak.Pointer[Tensor]
	src, dst uint64 // kaynağın ve sonucun depolama sürümü
}

// CastShared Cast gibidir; ancak bir ara sonucun aynı türe dönüşümünü, kaynak ve
// sonuç o zamandan beri değişmediyse yeniden kullanır (ör. karışık hassasiyette
// normun çıktısı q, k ve v çarpımları için üç kez değil bir kez dönüştürülür).
// Sonuç başka çağrılarla paylaşılabilir; yerinde değiştirilmemelidir (değiştirilirse
// sonraki çağrı yeniden dönüştürür). Yalnızca türev izlenen ara sonuçlarda saklanır:
// yapraklarda (parametreler, veriler) ve türevsiz hesapta Cast'in kendisidir.
func (t *Tensor) CastShared(dt DType) *Tensor {
	if t.dtyp == dt || t.IsLeaf() || !GradEnabled() {
		return t.Cast(dt)
	}
	if c := t.n.castMemo.Load(); c != nil {
		if o := c.out.Value(); o != nil && !o.IsDropped() && o.dtyp == dt && o.n != nil && !o.n.free &&
			c.src == t.storage.Version() && c.dst == o.storage.Version() {
			return o
		}
	}
	out := t.Cast(dt)
	t.n.castMemo.Store(&castMemo{out: weak.Make(out), src: t.storage.Version(), dst: out.storage.Version()})
	return out
}

// Bits16 v'yi 16 bitlik dt türüne (BFloat16 ya da Float16) en yakına-çifte
// yuvarlayarak bit deseni olarak verir. Dosyaya 16 bitlik veri yazmak içindir:
// o türden bir tensörün Data() değerleri kayıpsız geri çevrilir.
func Bits16(dt DType, v float32) uint16 {
	if dt != BFloat16 && dt != Float16 {
		panic(messages.New(messages.H3037, dt))
	}
	return narrowOne(v, dt)
}

// FromBits16 16 bitlik dt türündeki (BFloat16 ya da Float16) bit desenini
// float32'ye kayıpsız açar.
func FromBits16(dt DType, h uint16) float32 {
	switch dt {
	case BFloat16:
		return bf16ToF32(h)
	case Float16:
		return f16ToF32(h)
	}
	panic(messages.New(messages.H3038, dt))
}

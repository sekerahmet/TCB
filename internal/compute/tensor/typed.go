package tensor

import (
	"slices"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Cinsli tensör (15.1): float64, TR_Int ve TR_Bool (cihazdan bağımsız katman)
// ============================================================================
//
// Hesap ondalıkları (float32, bf16, fp16) Backend'in ondalık yöntemleriyle hesaplanır.
// Öbür türlerin depolaması ham bayttır (cpu_typed.go) ve işlemleri burada, tek bir
// TypedBackend yöntemiyle arka uca gider. CPU bütün işlemleri yapar; GPU yalnız adım 8'in
// yollarını (kıyas, maske birleştirme, TR_where, taşıma) yapar, öbürlerinde ok=false döner
// ve bu katman 15.48'in hassasiyet hatasını verir (H6960). Cinsli işlemler türev izlemez.

// RawBackend ham baytla depolama kuran ve okuyan arka uç (cinsli tensörlerin taşınması).
type RawBackend interface {
	// FromRaw ham baytlardan (küçük uçlu, mantıksal sırayla) dt türünde depolama kurar.
	FromRaw(raw []byte, dt DType) Storage
	// ToRaw görünümün öğelerini mantıksal sırayla ham bayt olarak verir.
	ToRaw(src View) []byte
	// FillRaw bütün öğeleri elem'in (bir öğenin ham baytları) değeri yapar.
	FillRaw(dst Storage, elem []byte)
}

// TypedKind cinsli işlemin çeşidi; TypedOp.Sub alt işlemi seçer.
type TypedKind uint8

const (
	// TCompare a op b → Bool (Sub: CmpOp); a ve b aynı cinsten.
	TCompare TypedKind = iota
	// TLogic Bool'da ve/veya/dışlayan veya, Int'te bit işleçleri ve kaydırma (Sub: LogicOp).
	TLogic
	// TNot Bool'da değil, Int'te tümleme.
	TNot
	// TArith Int ve float64 aritmetiği (Sub: ArithOp).
	TArith
	// TNeg tekli eksi.
	TNeg
	// TConvert cins ya da hassasiyet dönüşümü: ins[0] → out'un türü.
	TConvert
	// TWhere koşul (Bool) doğruysa a, değilse b.
	TWhere
	// TAbs ve TSign Int ve float64'te mutlak değer ve işaret.
	TAbs
	TSign
)

// CmpOp karşılaştırma.
type CmpOp uint8

const (
	CmpEq CmpOp = iota
	CmpNe
	CmpLt
	CmpGt
	CmpLe
	CmpGe
)

// LogicOp mantık ve bit işleçleri.
type LogicOp uint8

const (
	LogicAnd LogicOp = iota
	LogicOr
	LogicXor
	LogicShl
	LogicShr
)

// ArithOp cinsli aritmetik.
type ArithOp uint8

const (
	ArithAdd ArithOp = iota
	ArithSub
	ArithMul
	ArithDiv // yalnız float64
	ArithFloorDiv
	ArithMod
	ArithPow
)

// TypedOp bir cinsli işlem.
type TypedOp struct {
	Kind TypedKind
	Sub  uint8
}

// TypedBackend cinsli işlemleri yapan arka uç. out bitişiktir, şekli outShape; girdiler
// yayınlanmış olabilir. Desteklemediği işlemde ok=false döner (çağıran hassasiyet hatası
// verir). Öğeye bağlı hata (taşma, sıfıra bölme ...) ilk hatalı öğeyle döner.
type TypedBackend interface {
	Typed(op TypedOp, out Storage, outShape []int, ins []View) (fault *ElemFault, ok bool)
}

// FaultKind öğeye bağlı cinsli işlem hatasının çeşidi.
type FaultKind uint8

const (
	// FaultOverflow sonuç hassasiyete sığmıyor (aritmetik, tekli eksi, mutlak değer).
	FaultOverflow FaultKind = iota
	// FaultNotFit değer hedef hassasiyete sığmıyor (dönüşüm).
	FaultNotFit
	// FaultDivZero // ya da %'de sıfıra bölme.
	FaultDivZero
	// FaultNegExp TR_Int'te eksi üs.
	FaultNegExp
	// FaultNaN tam sayıya dönüşümde NaN; FaultInf sonsuz.
	FaultNaN
	FaultInf
	// FaultShift kaydırma miktarı aralık dışında.
	FaultShift
)

// ElemFault cinsli öğe öğe işlemin ilk hatalı öğesi; ml katmanı iletiye çevirir.
type ElemFault struct {
	Kind  FaultKind
	Index int   // sonucun şeklinde düz (satır öncelikli) konum
	Shape []int // sonucun şekli
	Int   int64 // hatalı değer ya da miktar (tam sayıda)
	Float float64
	A, B  int64 // aritmetikte işlenenler (taşma, sıfıra bölme, eksi üs)
	DType DType // hedef hassasiyet
}

// Elem tensörün öğe cinsi.
func (t *Tensor) Elem() ElemKind { return t.dtyp.Elem() }

// typedBackend arka ucun cinsli işlem yüzü; olmayan arka uçta hassasiyet hatası.
func typedBackend(be Backend, dt DType) TypedBackend {
	if tb, ok := be.(TypedBackend); ok {
		return tb
	}
	panic(PrecisionError(dt, be.Device()))
}

// rawBackend arka ucun ham bayt yüzü; olmayan arka uçta hassasiyet hatası.
func rawBackend(be Backend, dt DType) RawBackend {
	if rb, ok := be.(RawBackend); ok {
		return rb
	}
	panic(PrecisionError(dt, be.Device()))
}

// PrecisionError cihazın bu işlemi bu türde yapamadığını söyleyen hassasiyet hatası (15.48).
func PrecisionError(dt DType, dev Device) *messages.Error {
	if dev.Kind == KindGPU {
		return messages.New(messages.H6969, dt.Name(), dev.String())
	}
	kind := "other"
	switch {
	case dt.Elem() == ElemInt:
		kind = "int"
	case dt.Elem() == ElemBool:
		kind = "bool"
	}
	return messages.New(messages.H6960, dt.Name(), dev.String(), kind)
}

// FromRawOn ham baytlardan dev cihazında dt türünde bitişik tensör kurar.
func FromRawOn(dev Device, dt DType, raw []byte, shape ...int) *Tensor {
	be := BackendFor(dev)
	n := allocSizeOn(be, isCreate, shape, dt)
	if len(raw) != n*elemBytes(dt) {
		panic(messages.New(messages.H3035, len(raw)/max(elemBytes(dt), 1), shape))
	}
	return fromStorage(be, rawBackend(be, dt).FromRaw(raw, dt), shape)
}

// Int64sOn tam sayı (dt: TR_Int hassasiyeti) ya da mantık (dt: Bool, değerler 0/1) tensörü
// kurar. Değerlerin hassasiyete sığdığını çağıran denetler.
func Int64sOn(dev Device, dt DType, vals []int64, shape ...int) *Tensor {
	eb := elemBytes(dt)
	raw := make([]byte, len(vals)*eb)
	for i, v := range vals {
		PutRawInt(dt, raw[i*eb:(i+1)*eb], v)
	}
	return FromRawOn(dev, dt, raw, shape...)
}

// Float64sOn float64 tensörü kurar.
func Float64sOn(dev Device, vals []float64, shape ...int) *Tensor {
	raw := make([]byte, len(vals)*8)
	for i, v := range vals {
		PutRawFloat64(raw[i*8:], v)
	}
	return FromRawOn(dev, Float64, raw, shape...)
}

// Raw tensörün öğelerini mantıksal sırayla ham bayt olarak verir (yalnız ham bayt türleri).
func (t *Tensor) Raw() []byte {
	if !t.dtyp.IsTyped() {
		panic(messages.Internal("Raw on " + t.dtyp.String()))
	}
	return rawBackend(t.be, t.dtyp).ToRaw(t.view())
}

// Int64s tam sayı ya da mantık tensörünün öğeleri (mantık 0/1).
func (t *Tensor) Int64s() []int64 {
	if t.Elem() == ElemFloat {
		panic(messages.Internal("Int64s on " + t.dtyp.String()))
	}
	raw, eb := t.Raw(), elemBytes(t.dtyp)
	out := make([]int64, len(raw)/eb)
	for i := range out {
		out[i] = RawInt(t.dtyp, raw[i*eb:(i+1)*eb])
	}
	return out
}

// float64s float64 tensörün öğeleri (Float64s'in ham bayt yolu).
func (t *Tensor) float64s() []float64 {
	raw := t.Raw()
	out := make([]float64, len(raw)/8)
	for i := range out {
		out[i] = RawFloat64(raw[i*8:])
	}
	return out
}

// typed cinsli işlemi out türünde, outShape şeklinde yapar; ins tensörleri aynı cihazdadır.
func typed(op TypedOp, dt DType, outShape []int, ins ...*Tensor) (*Tensor, *ElemFault) {
	be := ins[0].be
	for _, x := range ins[1:] {
		sameDevice("typed", ins[0], x)
	}
	out := zerosOf(isCreate, be, outShape, dt)
	views := make([]View, len(ins))
	for i, x := range ins {
		views[i] = x.view()
	}
	fault, ok := typedBackend(be, dt).Typed(op, out.storage, outShape, views)
	if !ok {
		out.Drop()
		bad := dt
		for _, x := range ins {
			if x.dtyp.IsTyped() {
				bad = x.dtyp
			}
		}
		panic(PrecisionError(bad, be.Device()))
	}
	if fault != nil {
		out.Drop()
		fault.Shape = slices.Clone(outShape)
		return nil, fault
	}
	return out, nil
}

// CompareTyped a op b öğe öğe karşılaştırmasıdır (yayınla); sonuç Bool, türev izlemez. a ve b aynı
// cinstendir (ondalıkta hassasiyetler farklı olabilir; değerler kayıpsız açılıp karşılaştırılır).
func CompareTyped(a, b *Tensor, op CmpOp) *Tensor {
	r, _ := typed(TypedOp{TCompare, uint8(op)}, Bool, broadcastShapes(a.shape, b.shape), a, b)
	return r
}

// Logic mantık (Bool) ya da bit (Int) işlecidir; Int'te sonucun hassasiyeti dt'dir (kaydırmada
// sol işleneninki, öbürlerinde birleşmiş hassasiyet, 15.6c).
func Logic(a, b *Tensor, op LogicOp, dt DType) (*Tensor, *ElemFault) {
	return typed(TypedOp{TLogic, uint8(op)}, dt, broadcastShapes(a.shape, b.shape), a, b)
}

// Not mantık tensöründe değil, tam sayıda tümlemedir.
func Not(t *Tensor) *Tensor {
	r, _ := typed(TypedOp{Kind: TNot}, t.dtyp, t.shape, t)
	return r
}

// Arith Int ya da float64 aritmetiğidir; sonuç dt türünde (Int'te birleşmiş hassasiyet).
func Arith(a, b *Tensor, op ArithOp, dt DType) (*Tensor, *ElemFault) {
	return typed(TypedOp{TArith, uint8(op)}, dt, broadcastShapes(a.shape, b.shape), a, b)
}

// Neg tekli eksidir (Int'te en küçük değer taşar).
func Neg(t *Tensor) (*Tensor, *ElemFault) { return typed(TypedOp{Kind: TNeg}, t.dtyp, t.shape, t) }

// AbsTyped ve SignTyped Int ve float64'te mutlak değer ve işarettir.
func AbsTyped(t *Tensor) (*Tensor, *ElemFault) {
	return typed(TypedOp{Kind: TAbs}, t.dtyp, t.shape, t)
}

func SignTyped(t *Tensor) *Tensor {
	r, _ := typed(TypedOp{Kind: TSign}, t.dtyp, t.shape, t)
	return r
}

// Convert t'yi dt türüne çevirir (cins ya da hassasiyet): ondalıktan tam sayıya sıfıra doğru
// keser (NaN, sonsuz ve sığmayan değer hata), tam sayıdan ondalığa en yakına yuvarlar, tam
// sayıdan tam sayıya sığmalıdır. Türev izlemez; iki hesap ondalığı arasında Cast kullanılır.
func Convert(t *Tensor, dt DType) (*Tensor, *ElemFault) {
	if t.dtyp == dt {
		return t, nil
	}
	if t.dtyp.IsFloat() && dt.IsFloat() {
		return t.Cast(dt), nil
	}
	return typed(TypedOp{Kind: TConvert}, dt, t.shape, t)
}

// WhereTyped c (Bool) doğruysa a'yı, değilse b'yi seçer (üçü yayınlanır); a ve b aynı türdendir.
// Türev izlemez: ondalık dalların türevi Where iledir.
func WhereTyped(c, a, b *Tensor) *Tensor {
	r, _ := typed(TypedOp{Kind: TWhere}, a.dtyp, broadcastShapes(c.shape, broadcastShapes(a.shape, b.shape)), c, a, b)
	return r
}

// MergeInt iki TR_Int hassasiyetinin birleşmesi (15.6c): iki işaretli ya da iki işaretsizde
// geniş olan; işaretsiz ile işaretlide ikisini de tutan en dar işaretli hassasiyet ("uint8" ile
// "int8" "int16", "uint32" ile herhangi bir işaretli "int64").
func MergeInt(a, b DType) DType {
	if a == b {
		return a
	}
	if a.IsUnsigned() == b.IsUnsigned() {
		return intOfBits(max(a.Bits(), b.Bits()), a.IsUnsigned())
	}
	s, u := a, b
	if a.IsUnsigned() {
		s, u = b, a
	}
	return intOfBits(min(max(s.Bits(), 2*u.Bits()), 64), false)
}

// intOfBits bit genişliğindeki TR_Int hassasiyeti.
func intOfBits(bits int, unsigned bool) DType {
	if unsigned {
		switch bits {
		case 8:
			return UInt8
		case 16:
			return UInt16
		}
		return UInt32
	}
	switch bits {
	case 8:
		return Int8
	case 16:
		return Int16
	case 32:
		return Int32
	}
	return Int64
}

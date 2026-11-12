package tensor

import (
	le "encoding/binary"
	"runtime"
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
	// FromRaw ham baytlardan (küçük uçlu, mantıksal sırayla) dt türünde depolama kurar; dt ham bayt
	// türü ya da 16 bitlik ondalıktır (öğelerin bitleri, fromBits16).
	FromRaw(raw []byte, dt DType) Storage
	// ToRaw görünümün öğelerini mantıksal sırayla ham bayt olarak verir; 16 bitlik ondalıkta da.
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
// verir). Öğeye bağlı hata (taşma, sıfıra bölme ...) ilk hatalı öğeyle (düz konumu en küçük
// hatalı çıktı öğesi, 15.6) döner; kayıtta yuvası kayda giren işlem FaultRecorded döner.
type TypedBackend interface {
	Typed(op TypedOp, out Storage, outShape []int, ins []View) (fault *ElemFault, ok bool)
}

// TypedReducer (isteğe bağlı) "int64" toplamını cihazda yapan arka uç (T1 dilim 2b): in bitişik
// "int64" görünümü, (outer, n, inner) şeklinde orta eksen boyunca toplanır; out bitişik
// "int64", outer·inner öğe. Toplam 128 bitte kesindir; sığmayan çıktıların ilki (düz konumu en
// küçük, 15.6) Sum'ıyla döner, kayıtta FaultRecorded. Desteklemediği girdide ok=false.
type TypedReducer interface {
	SumInt64(out Storage, in View, outer, n, inner int) (fault *ElemFault, ok bool)
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
	// FaultIndexNotInt cihazdaki indeks tam sayı değil; FaultIndexRange sınırın dışında
	// (TR_replay kaydında gömme ve dilim yazma, IndexReader; Float ya da Int değerdir).
	FaultIndexNotInt
	FaultIndexRange
	// FaultRecorded hata değil: TR_replay kaydında hata yuvası kayda girdi, sonuç geçerlidir;
	// Index kaydın kaçıncı yuvası (fault_slot.go). Yalnız tam sayıya dönüşüm döner.
	FaultRecorded
)

// ElemFault cinsli öğe öğe işlemin ilk hatalı öğesi; ml katmanı iletiye çevirir.
type ElemFault struct {
	Kind  FaultKind
	Index int   // sonucun şeklinde düz (satır öncelikli) konum
	Shape []int // sonucun şekli
	Int   int64 // hatalı değer ya da miktar (tam sayıda)
	Float float64
	A, B  int64  // aritmetikte işlenenler (taşma, sıfıra bölme, eksi üs)
	Sum   string // sığmayan toplamın kesin değeri (onluk; yalnız toplamda)
	DType DType  // hedef hassasiyet
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
	if dt.IsQuantized() {
		return messages.New(messages.H6687, dt.Name())
	}
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

// FromBits16On 16 bitlik ondalık (bfloat16, float16) tensörü bitlerinden kurar; CPU'da bits'in
// kendisi depolamadır (çağıran bırakır), hiçbir cihazda float32'ye açılıp yeniden yuvarlanmaz.
func FromBits16On(dev Device, dt DType, bits []uint16, shape ...int) *Tensor {
	be := BackendFor(dev)
	n := allocSizeOn(be, isCreate, shape, dt)
	if len(bits) != n {
		panic(messages.New(messages.H3035, len(bits), shape))
	}
	return fromStorage(be, fromBits16(be, dt, bits), shape)
}

// fromBits16 bits'ten be'de 16 bitlik depolama kurar: CPU'da bits'in kendisi depolamadır, öbür
// arka uçlara ham bayt gider; float32'den geçseydi sinyal NaN'ı susardı.
func fromBits16(be Backend, dt DType, bits []uint16) Storage {
	if _, cpu := be.(cpuBackend); cpu {
		return &cpuStorage{dtyp: dt, h: bits}
	}
	raw := make([]byte, 2*len(bits))
	for i, h := range bits {
		le.LittleEndian.PutUint16(raw[2*i:], h)
	}
	return rawBackend(be, dt).FromRaw(raw, dt)
}

// Bits16s 16 bitlik ondalık tensörün öğelerini mantıksal sırayla bit deseni olarak verir: depodaki
// bitlerdir, float32'den geçseydi sinyal NaN'ı susardı.
func (t *Tensor) Bits16s() []uint16 {
	if t.dtyp != BFloat16 && t.dtyp != Float16 {
		panic(messages.Internal("Bits16s on " + t.dtyp.String()))
	}
	t.mustLive()
	if t.sym != nil {
		panic(messages.New(messages.H3107))
	}
	if _, cpu := t.be.(cpuBackend); !cpu {
		sy := counterStart(t.be, OpKindToHost).start()
		raw := gateToRaw(rawBackend(t.be, t.dtyp), t.view())
		sy.finish()
		out := make([]uint16, len(raw)/2)
		for i := range out {
			out[i] = le.LittleEndian.Uint16(raw[2*i:])
		}
		return out
	}
	src := t.view()
	if unlockedOps.Load() != 0 {
		gate(nil, []Storage{src.S})
	}
	out := make([]uint16, src.Numel())
	copyRows(out, cpuStore(src.S).h, View{Shape: src.Shape, Strides: ContiguousStrides(src.Shape)}, src)
	runtime.KeepAlive(t)
	return out
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

// Raw tensörün öğelerini mantıksal sırayla ham bayt olarak verir (ham bayt türleri); nicemlenmiş
// türde bloklardır, devrik görünümde H6688.
func (t *Tensor) Raw() []byte {
	if t.dtyp.IsQuantized() {
		t.refuseTransposed()
		defer runtime.KeepAlive(t)
		return slices.Clone(t.quantRaw(t.offset, t.Numel()))
	}
	if !t.dtyp.IsTyped() {
		panic(messages.Internal("Raw on " + t.dtyp.String()))
	}
	return gateToRaw(rawBackend(t.be, t.dtyp), t.view())
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
	checkNotQuantized(dt)
	for _, x := range ins {
		checkNotQuantized(x.dtyp)
	}
	out := zerosOf(isCreate, be, outShape, dt)
	views := make([]View, len(ins))
	for i, x := range ins {
		views[i] = x.view()
	}
	fault, ok := gateTyped(typedBackend(be, dt), op, out.storage, outShape, views)
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
	if fault != nil && fault.Kind == FaultRecorded {
		return out, fault
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
// Kartta TR_replay kaydında hata beklenmez: sonuç ve Kind'ı FaultRecorded olan hata birlikte
// döner (fault_slot.go).
func Convert(t *Tensor, dt DType) (*Tensor, *ElemFault) {
	if t.dtyp == dt {
		return t, nil
	}
	if t.dtyp.IsQuantized() && (dt.IsFloat() || dt == Float64) {
		return t.Open(dt), nil
	}
	if t.dtyp.IsFloat() && dt.IsFloat() {
		return t.Cast(dt), nil
	}
	return typed(TypedOp{Kind: TConvert}, dt, t.shape, t)
}

// ConvertIntegral, TConvert'in alt türü (TypedOp.Sub): kaynağın değerleri tam sayıdır ve hedefe
// sığar, dönüşüm hata veremez. Kartta yuvası ve beklemesi yoktur (TR_replay kaydında da
// çalışır); işlemcide Convert'in yoludur.
const ConvertIntegral uint8 = 1

// ConvertSortIndex sıralamanın float32 indeksini (Backend.Sort'un sözleşmesi: değerler tam sayı,
// sıralanan eksen en çok MaxSortLen = 2²⁴ öğe, float32'de tam) dt'ye çevirir; T1 dilim 2b, A15.
// Yalnız sıralamanın indeksi içindir: başka bir kaynakta NaN ya da aralık dışı değer kartta
// hatasız yazılırdı (DENETİM 503 N1).
func ConvertSortIndex(t *Tensor, dt DType) *Tensor {
	r, f := typed(TypedOp{Kind: TConvert, Sub: ConvertIntegral}, dt, t.shape, t)
	if f != nil {
		panic("tensor: ConvertSortIndex: the sort index is not integral (Backend.Sort contract broken)")
	}
	return r
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

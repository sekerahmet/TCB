package tensor

import (
	le "encoding/binary"
	"math"
	"unsafe"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// CPU arka ucu: ham bayt depolama (float64, TR_Int, TR_Bool; 15.1)
// ============================================================================
//
// Hesap ondalıkları dışındaki türlerin (DType.IsTyped) depolaması ham bayttır: öğeler küçük
// uçlu, hassasiyetin genişliğinde, mantık öğesi 0 ya da 1 bayt. Düzen safetensors ve TR
// dosya biçimiyle aynıdır (15.46, 15.47). Bayt dilimi 8 baytlık sözcüklerin üstündedir;
// öğelere generic dilimle (typedSlice) hizalı erişilir. Ondalık hesap çekirdekleri bu
// depolamayı görmez: temp.read/write onu hassasiyet hatasıyla reddeder (typedComputeError).

// elemBytes türün bir öğesinin bayt sayısı (ham bayt türlerinde).
func elemBytes(dt DType) int { return dt.Bits() / 8 }

// newTypedStorage n öğelik sıfırlanmış ham depolama.
func newTypedStorage(n int, dt DType) *cpuStorage {
	b := n * elemBytes(dt)
	w := make([]uint64, (b+7)/8)
	var raw []byte
	if b > 0 {
		raw = unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(w))), b)
	}
	return &cpuStorage{dtyp: dt, raw: raw, n: n}
}

// FromRaw ham baytlardan (küçük uçlu, mantıksal sırayla) dt türünde depolama kurar; 16 bitlik
// ondalıkta baytlar öğelerin bitleridir.
func (cpuBackend) FromRaw(raw []byte, dt DType) Storage {
	if dt == BFloat16 || dt == Float16 {
		bits := make([]uint16, len(raw)/2)
		checkElem(len(bits), dt)
		for i := range bits {
			bits[i] = le.LittleEndian.Uint16(raw[2*i:])
		}
		return &cpuStorage{dtyp: dt, h: bits}
	}
	if !dt.IsTyped() {
		panic(messages.New(messages.H3025, dt))
	}
	n := len(raw) / elemBytes(dt)
	checkElem(n, dt)
	s := newTypedStorage(n, dt)
	copy(s.raw, raw)
	return s
}

// ToRaw görünümün öğelerini mantıksal sırayla ham bayt olarak verir.
func (cpuBackend) ToRaw(src View) []byte {
	c := cpuStore(src.S)
	eb := elemBytes(c.dtyp)
	n := src.Numel()
	out := make([]byte, n*eb)
	if is16(src) {
		bits := make([]uint16, n)
		copyRows(bits, c.h, View{Shape: src.Shape, Strides: ContiguousStrides(src.Shape)}, src)
		for i, h := range bits {
			le.LittleEndian.PutUint16(out[2*i:], h)
		}
		return out
	}
	if src.IsContiguous() {
		copy(out, c.raw[src.Offset*eb:(src.Offset+n)*eb])
		return out
	}
	iterRowsPar(src.Shape, []int{src.Offset, 0}, [][]int{src.Strides, ContiguousStrides(src.Shape)}, 1, func(off []int, m int, in []int) {
		q, h, qs := off[0], off[1], in[0]
		for i := 0; i < m; i++ {
			copy(out[(h+i)*eb:(h+i+1)*eb], c.raw[(q+i*qs)*eb:(q+i*qs+1)*eb])
		}
	})
	return out
}

// copyTyped dst görünümüne src'nin öğelerini aynı türde yazar (src yayınlanmış olabilir).
func copyTyped(dst, src View) {
	if dst.DType() != src.DType() {
		// Cinsli tensör ondalık bir hesaba giriyor (ör. dikkatin maskesi): 15.48'in hatası.
		switch {
		case dst.DType().IsFloat():
			panic(typedComputeError(src.DType()))
		case src.DType().IsFloat():
			panic(typedComputeError(dst.DType()))
		}
		panic(typedCopyError(dst.DType(), src.DType()))
	}
	d, k := cpuStore(dst.S), cpuStore(src.S)
	eb := elemBytes(d.dtyp)
	iterRowsPar(dst.Shape, []int{dst.Offset, src.Offset}, [][]int{dst.Strides, src.Strides}, 0, func(off []int, m int, in []int) {
		h, q, hs, qs := off[0], off[1], in[0], in[1]
		if hs == 1 && qs == 1 {
			copy(d.raw[h*eb:(h+m)*eb], k.raw[q*eb:(q+m)*eb])
			return
		}
		for i := 0; i < m; i++ {
			copy(d.raw[(h+i*hs)*eb:(h+i*hs+1)*eb], k.raw[(q+i*qs)*eb:(q+i*qs+1)*eb])
		}
	})
}

// typedCopyError iki cinsli türün arasındaki yazmanın hatasıdır: yazma dönüştürmez (15.3); ileti
// kaynağın dönüşümünü önerir. İkisi de ondalık hesap türü değildir (copyTyped).
func typedCopyError(dst, src DType) *messages.Error {
	number := func(dt DType) string {
		if dt.Elem() == ElemInt {
			return "int"
		}
		return "float"
	}
	switch {
	case dst.Elem() == ElemBool:
		return messages.New(messages.H3112, dst.Name(), src.Name(), number(src))
	case src.Elem() == ElemBool:
		return messages.New(messages.H3113, dst.Name(), src.Name(), number(dst))
	case dst.Elem() == ElemInt:
		return messages.New(messages.H3114, dst.Name(), src.Name())
	}
	return messages.New(messages.H3115, dst.Name(), src.Name())
}

// FillRaw depolamanın bütün öğelerini elem'in (bir öğenin ham baytları) değeriyle doldurur.
func (cpuBackend) FillRaw(dst Storage, elem []byte) {
	c := cpuStore(dst)
	eb := elemBytes(c.dtyp)
	for i := 0; i < c.n; i++ {
		copy(c.raw[i*eb:(i+1)*eb], elem)
	}
}

// typedComputeError ondalık hesap çekirdeğine ham bayt türü gelince verilen hassasiyet
// hatası (15.48): işlem bu türde yapılamaz.
func typedComputeError(dt DType) *messages.Error { return PrecisionError(dt, CPU) }

// ----------------------------------------------------------------------------
// Öğe okuma ve yazma (ham baytın değeri; typed.go'nun işlemleri)
// ----------------------------------------------------------------------------

// RawInt ham baytlardaki dt türündeki tam sayı ya da mantık öğesini int64 olarak okur.
func RawInt(dt DType, b []byte) int64 {
	switch dt {
	case Int64:
		return int64(le.LittleEndian.Uint64(b))
	case Int32:
		return int64(int32(le.LittleEndian.Uint32(b)))
	case Int16:
		return int64(int16(le.LittleEndian.Uint16(b)))
	case Int8:
		return int64(int8(b[0]))
	case UInt32:
		return int64(le.LittleEndian.Uint32(b))
	case UInt16:
		return int64(le.LittleEndian.Uint16(b))
	case UInt8, Bool:
		return int64(b[0])
	}
	panic(messages.Internal("RawInt on " + dt.String()))
}

// PutRawInt v'yi dt türünde (tam sayı ya da mantık) b'ye yazar; v'nin sığdığını çağıran
// denetler.
func PutRawInt(dt DType, b []byte, v int64) {
	switch dt {
	case Int64:
		le.LittleEndian.PutUint64(b, uint64(v))
	case Int32, UInt32:
		le.LittleEndian.PutUint32(b, uint32(v))
	case Int16, UInt16:
		le.LittleEndian.PutUint16(b, uint16(v))
	case Int8, UInt8, Bool:
		b[0] = byte(v)
	default:
		panic(messages.Internal("PutRawInt on " + dt.String()))
	}
}

// RawFloat64 ham baytlardaki float64 öğeyi okur.
func RawFloat64(b []byte) float64 { return math.Float64frombits(le.LittleEndian.Uint64(b)) }

// PutRawFloat64 v'yi float64 olarak b'ye yazar.
func PutRawFloat64(b []byte, v float64) { le.LittleEndian.PutUint64(b, math.Float64bits(v)) }

// readCond seçimin (Where) koşul görünümünü float32 okunabilir yapar: TR_Bool koşul 0/1
// ondalık geçici tampona açılır (yalnız koşul için; öbür çekirdekler mantık tensörünü almaz).
func (g *temp) readCond(v View) View {
	if v.S == nil || v.S.DType() != Bool {
		return g.read(v)
	}
	tmp := g.buffer(v.Numel(), false)
	e := newElems(v, v.Shape)
	for k := range tmp.d {
		tmp.d[k] = float32(e.i(k))
	}
	return View{S: tmp, Shape: v.Shape, Strides: ContiguousStrides(v.Shape)}
}

package tensor

import (
	le "encoding/binary"
	"math"
	"slices"
)

// Baytlarla tensör arasında kodlama (22.1, 22.2). Depolama küçük uçludur (FromRaw); büyük uçlu
// veri öğe öğe çevrilir. Sonuç her yönde kopyadır.

// ElemSize dt'nin bir öğesinin bayt boyudur; ham kodlanmayan (nicemlenmiş) türde 0'dır.
func ElemSize(dt DType) int {
	if dt.IsQuantized() {
		return 0
	}
	return elemBytes(dt)
}

// CheckSize shape'in bellek sınırını aşmadığını denetler; aşarsa sınırın iletisi gelir.
func CheckSize(shape []int, dt DType) { allocSizeOn(BackendFor(DefaultDevice()), isCreate, shape, dt) }

// FromBytesOn raw'ı dt öğeleri olarak okuyup tek boyutlu tensör kurar. raw'ın uzunluğu öğe boyunun
// katıdır; big büyük uçlu okumadır.
func FromBytesOn(dev Device, dt DType, raw []byte, big bool) *Tensor {
	size := elemBytes(dt)
	n := len(raw) / size
	if big {
		raw = swapped(raw, size)
	}
	switch {
	case dt == Float32:
		data := make([]float32, n)
		for i := range data {
			data[i] = math.Float32frombits(le.LittleEndian.Uint32(raw[4*i:]))
		}
		return NewOn(dev, data, n)
	case dt == BFloat16 || dt == Float16:
		bits := make([]uint16, n)
		for i := range bits {
			bits[i] = le.LittleEndian.Uint16(raw[2*i:])
		}
		return FromBits16On(dev, dt, bits, n)
	}
	return FromRawOn(dev, dt, raw, n)
}

// Bytes t'nin öğelerini kendi hassasiyetinde, satır sırasıyla baytlara yazar.
func (t *Tensor) Bytes(big bool) []byte {
	var raw []byte
	switch {
	case t.dtyp == Float32:
		data := t.Data()
		raw = make([]byte, 4*len(data))
		for i, f := range data {
			le.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(f))
		}
	case t.dtyp == BFloat16 || t.dtyp == Float16:
		bits := t.Bits16s()
		raw = make([]byte, 2*len(bits))
		for i, h := range bits {
			le.LittleEndian.PutUint16(raw[2*i:], h)
		}
	default:
		raw = slices.Clone(t.Raw())
	}
	if big {
		return swapped(raw, elemBytes(t.dtyp))
	}
	return raw
}

// swapped raw'ın size baytlık her öğesini ters çevrilmiş olarak yeni bir dilime yazar.
func swapped(raw []byte, size int) []byte {
	out := make([]byte, len(raw))
	for i := 0; i+size <= len(raw); i += size {
		for j := range size {
			out[i+j] = raw[i+size-1-j]
		}
	}
	return out
}

// FloatToBits16 x'i dt'nin (BFloat16, Float16) en yakın değerine bir kez yuvarlar. float32 ara adım
// tek yönlü yuvarlanır ve yapışkan bit taşır: iki kez yuvarlama sonucu değiştirmez.
func FloatToBits16(dt DType, x float64) uint16 {
	f := float32(x)
	if float64(f) != x && x == x {
		bits := math.Float32bits(f)
		if math.Abs(float64(f)) > math.Abs(x) {
			bits--
		}
		f = math.Float32frombits(bits | 1)
	}
	if dt == BFloat16 {
		return f32ToBF16(f)
	}
	return f32ToF16(f)
}

// Bits16ToFloat bits'in değeridir (BFloat16, Float16); her değer float64'te tamdır.
func Bits16ToFloat(dt DType, bits uint16) float64 {
	if dt == BFloat16 {
		return float64(bf16ToF32(bits))
	}
	return float64(f16ToF32(bits))
}

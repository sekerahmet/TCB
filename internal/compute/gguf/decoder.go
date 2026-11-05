package gguf

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"math/bits"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// decoder dosyayı sırayla okur; ilk hata err'de kalır.
type decoder struct {
	r         io.Reader
	remaining int64 // dosyada henüz okunmamış bayt
	offset    int64 // okunan bayt
	err       error
	scratch   [8]byte
}

// bytes p'yi doldurur; dosya biterse H6658, okuma hatası olursa H6677 err'e yazılır. kind
// iletinin seçimidir: dosyanın neresi okunuyordu.
func (d *decoder) bytes(kind string, p []byte) bool {
	if _, err := io.ReadFull(d.r, p); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			d.err = messages.New(messages.H6658, kind, d.offset)
		} else {
			d.err = messages.New(messages.H6677, d.offset, err)
		}
		return false
	}
	d.remaining -= int64(len(p))
	d.offset += int64(len(p))
	return true
}

func (d *decoder) u32(kind string) (uint32, bool) {
	ok := d.bytes(kind, d.scratch[:4])
	return binary.LittleEndian.Uint32(d.scratch[:4]), ok
}

func (d *decoder) u64(kind string) (uint64, bool) {
	ok := d.bytes(kind, d.scratch[:8])
	return binary.LittleEndian.Uint64(d.scratch[:8]), ok
}

// length bildirilen count öğelik (öğe başına en az width bayt) verinin dosyada kalanı
// aşmadığını denetler: aşan boyla ayırma yapılmaz (ollama CVE-2025-0315).
func (d *decoder) length(count, width uint64) bool {
	hi, lo := bits.Mul64(count, width)
	if hi != 0 || lo > uint64(d.remaining) {
		d.err = messages.New(messages.H6659, d.offset, count, d.remaining)
		return false
	}
	return true
}

// text boy önekli UTF-8 metin okur.
func (d *decoder) text(kind string) (string, bool) {
	n, ok := d.u64(kind)
	if !ok || !d.length(n, 1) {
		return "", false
	}
	b := make([]byte, n)
	if !d.bytes(kind, b) {
		return "", false
	}
	if !utf8.Valid(b) {
		d.err = messages.New(messages.H6663, d.offset-int64(n))
		return "", false
	}
	return string(b), true
}

// value bir değeri okur; tür 0..12 olmalıdır, dizi iç içe olamaz.
func (d *decoder) value(key string) (Value, error) {
	kind, ok := d.u32("value")
	if !ok {
		return Value{}, d.err
	}
	t := ValueType(kind)
	if t >= valueTypeCount {
		return Value{}, messages.New(messages.H6660, key, t.String())
	}
	if t == Array {
		return d.array(key)
	}
	data, err := d.scalar(key, t)
	if err != nil {
		return Value{}, err
	}
	return Value{Type: t, Data: data}, nil
}

// scalar t türünde tek değer okur.
func (d *decoder) scalar(key string, t ValueType) (any, error) {
	if t == String {
		s, ok := d.text("value")
		if !ok {
			return nil, d.err
		}
		return s, nil
	}
	p := d.scratch[:t.width()]
	if !d.bytes("value", p) {
		return nil, d.err
	}
	if t == Bool && p[0] > 1 {
		return nil, messages.New(messages.H6662, key, p[0])
	}
	return fixedValue(t, p), nil
}

// fixedValue sabit boylu değeri çözer.
func fixedValue(t ValueType, p []byte) any {
	switch t {
	case Uint8:
		return p[0]
	case Int8:
		return int8(p[0])
	case Uint16:
		return binary.LittleEndian.Uint16(p)
	case Int16:
		return int16(binary.LittleEndian.Uint16(p))
	case Uint32:
		return binary.LittleEndian.Uint32(p)
	case Int32:
		return int32(binary.LittleEndian.Uint32(p))
	case Float32:
		return math.Float32frombits(binary.LittleEndian.Uint32(p))
	case Bool:
		return p[0] == 1
	case Uint64:
		return binary.LittleEndian.Uint64(p)
	case Int64:
		return int64(binary.LittleEndian.Uint64(p))
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(p))
}

// array öğe türünü, boyunu ve öğeleri okur.
func (d *decoder) array(key string) (Value, error) {
	kind, ok := d.u32("value")
	if !ok {
		return Value{}, d.err
	}
	elem := ValueType(kind)
	if elem >= valueTypeCount {
		return Value{}, messages.New(messages.H6660, key, elem.String())
	}
	if elem == Array {
		return Value{}, messages.New(messages.H6661, key)
	}
	n, ok := d.u64("value")
	if !ok {
		return Value{}, d.err
	}
	if !d.length(n, elem.width()) {
		return Value{}, d.err
	}
	data, err := d.elements(key, elem, int(n))
	if err != nil {
		return Value{}, err
	}
	return Value{Type: Array, Elem: elem, Data: data}, nil
}

// elements n öğeyi Go diliminde okur. Boy length'ten geçmiştir.
func (d *decoder) elements(key string, t ValueType, n int) (any, error) {
	switch t {
	case Uint8:
		return list[uint8](d, key, t, n)
	case Int8:
		return list[int8](d, key, t, n)
	case Uint16:
		return list[uint16](d, key, t, n)
	case Int16:
		return list[int16](d, key, t, n)
	case Uint32:
		return list[uint32](d, key, t, n)
	case Int32:
		return list[int32](d, key, t, n)
	case Float32:
		return list[float32](d, key, t, n)
	case Bool:
		return list[bool](d, key, t, n)
	case String:
		return list[string](d, key, t, n)
	case Uint64:
		return list[uint64](d, key, t, n)
	case Int64:
		return list[int64](d, key, t, n)
	}
	return list[float64](d, key, t, n)
}

// list n öğeyi sırayla okur.
func list[T any](d *decoder, key string, t ValueType, n int) ([]T, error) {
	out := make([]T, n)
	for i := range out {
		v, err := d.scalar(key, t)
		if err != nil {
			return nil, err
		}
		out[i] = v.(T)
	}
	return out, nil
}

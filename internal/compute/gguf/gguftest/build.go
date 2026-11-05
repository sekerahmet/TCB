// Package gguftest testler için GGUF dosyası kurar. Yalnızca testler içe aktarır: depoya model
// dosyası girmez, her test kendi dosyasını elle üretir (`Kolibri-1`, `Qwen3-4B-128K` biçimleri
// dahil).
//
// Layer: compute core (internal/compute); yalnızca testler içe aktarır.
package gguftest

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
)

// Tensor kurulacak dosyanın bir tensörüdür.
type Tensor struct {
	Name string
	Dims []uint64 // dosyadaki sıra: Dims[0] son eksendir
	Type gguf.TensorType
	Data []byte
}

// Builder geçerli bir GGUF sürüm 3 dosyası kurar.
type Builder struct {
	Alignment uint64 // 0 ise varsayılan 32; anahtar yazılmaz
	keys      [][]byte
	tensors   []Tensor
}

// KV bir üstveri anahtarı ekler; değerin Go türü GGUF türünü seçer (dilimler dizidir).
func (b *Builder) KV(key string, value any) *Builder {
	b.keys = append(b.keys, KV(key, value))
	return b
}

// Tensor bir tensör ekler.
func (b *Builder) Tensor(name string, t gguf.TensorType, dims []uint64, data []byte) *Builder {
	b.tensors = append(b.tensors, Tensor{Name: name, Dims: dims, Type: t, Data: data})
	return b
}

// Bytes dosyanın baytlarıdır.
func (b *Builder) Bytes() []byte {
	alignment := b.Alignment
	if alignment == 0 {
		alignment = 32
	}
	out := Header(3, uint64(len(b.tensors)), uint64(len(b.keys)))
	for _, kv := range b.keys {
		out = append(out, kv...)
	}
	offset := uint64(0)
	for _, t := range b.tensors {
		out = AppendTensor(out, t.Name, t.Dims, uint32(t.Type), offset)
		offset += padded(uint64(len(t.Data)), alignment)
	}
	if len(b.tensors) > 0 {
		out = append(out, make([]byte, padded(uint64(len(out)), alignment)-uint64(len(out)))...)
	}
	for i, t := range b.tensors {
		out = append(out, t.Data...)
		if i+1 < len(b.tensors) {
			out = append(out, make([]byte, padded(uint64(len(t.Data)), alignment)-uint64(len(t.Data)))...)
		}
	}
	return out
}

func padded(n, alignment uint64) uint64 {
	return (n + alignment - 1) / alignment * alignment
}

// Header dosyanın 24 baytlık başıdır.
func Header(version uint32, tensors, keys uint64) []byte {
	out := []byte(gguf.Magic)
	out = binary.LittleEndian.AppendUint32(out, version)
	out = binary.LittleEndian.AppendUint64(out, tensors)
	return binary.LittleEndian.AppendUint64(out, keys)
}

// AppendString boy önekli metni ekler.
func AppendString(out []byte, s string) []byte {
	out = binary.LittleEndian.AppendUint64(out, uint64(len(s)))
	return append(out, s...)
}

// AppendTensor bir tensör kaydı ekler; tür elle bozulabilsin diye sayıdır.
func AppendTensor(out []byte, name string, dims []uint64, kind uint32, offset uint64) []byte {
	out = AppendString(out, name)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(dims)))
	for _, d := range dims {
		out = binary.LittleEndian.AppendUint64(out, d)
	}
	out = binary.LittleEndian.AppendUint32(out, kind)
	return binary.LittleEndian.AppendUint64(out, offset)
}

// KV bir anahtar kaydının baytlarıdır.
func KV(key string, value any) []byte {
	out := AppendString(nil, key)
	t, body := encode(value)
	out = binary.LittleEndian.AppendUint32(out, uint32(t))
	return append(out, body...)
}

// encode değerin türünü ve baytlarını verir.
func encode(value any) (gguf.ValueType, []byte) {
	le := binary.LittleEndian
	switch v := value.(type) {
	case uint8:
		return gguf.Uint8, []byte{v}
	case int8:
		return gguf.Int8, []byte{byte(v)}
	case uint16:
		return gguf.Uint16, le.AppendUint16(nil, v)
	case int16:
		return gguf.Int16, le.AppendUint16(nil, uint16(v))
	case uint32:
		return gguf.Uint32, le.AppendUint32(nil, v)
	case int32:
		return gguf.Int32, le.AppendUint32(nil, uint32(v))
	case float32:
		return gguf.Float32, le.AppendUint32(nil, math.Float32bits(v))
	case bool:
		if v {
			return gguf.Bool, []byte{1}
		}
		return gguf.Bool, []byte{0}
	case string:
		return gguf.String, AppendString(nil, v)
	case uint64:
		return gguf.Uint64, le.AppendUint64(nil, v)
	case int64:
		return gguf.Int64, le.AppendUint64(nil, uint64(v))
	case float64:
		return gguf.Float64, le.AppendUint64(nil, math.Float64bits(v))
	case []string:
		return encodeArray(v)
	case []int32:
		return encodeArray(v)
	case []float32:
		return encodeArray(v)
	case []uint8:
		return encodeArray(v)
	case []bool:
		return encodeArray(v)
	}
	panic(fmt.Sprintf("gguftest: unsupported value %T", value))
}

// encodeArray dizinin baytlarıdır.
func encodeArray[T any](items []T) (gguf.ValueType, []byte) {
	var zero T
	elem, _ := encode(zero)
	out := binary.LittleEndian.AppendUint32(nil, uint32(elem))
	out = binary.LittleEndian.AppendUint64(out, uint64(len(items)))
	for _, it := range items {
		_, body := encode(it)
		out = append(out, body...)
	}
	return gguf.Array, out
}

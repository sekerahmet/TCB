// Package onnxtest testler için ONNX dosyası (ModelProto, protobuf) kurar. Yalnızca testler içe
// aktarır: depoya model dosyası girmez, her test kendi dosyasını elle üretir; Piper sesinin adları
// ve yapısı dahil (`sid` gömmesi, adsız `onnx::Conv_*` ağırlıkları).
//
// Layer: compute core (internal/compute); yalnızca testler içe aktarır.
package onnxtest

import (
	"encoding/binary"
	"math"

	"github.com/sekerahmet/TCB/internal/compute/onnx"
)

// Tel türleri (protobuf).
const (
	WireVarint  = 0
	WireFixed64 = 1
	WireLen     = 2
	WireFixed32 = 5
)

// Tensor kurulacak bir TensorProto'dur. Raw doluysa raw_data'ya, değilse türün alanına yazılır
// (Floats → float_data, Int32s → int32_data, Int64s → int64_data, Doubles → double_data).
type Tensor struct {
	Name     string
	Dims     []int64
	Type     onnx.DataType
	Raw      []byte
	Floats   []float32
	Int32s   []int32
	Int64s   []int64
	Doubles  []float64
	External bool // external_data'ya bir giriş ve data_location = EXTERNAL yazılır
}

// Node kurulacak bir NodeProto'dur; Attrs hazır AttributeProto'lardır (Attr*).
type Node struct {
	Name    string
	Op      string
	Domain  string
	Inputs  []string
	Outputs []string
	Attrs   [][]byte
}

// Builder geçerli bir ModelProto kurar: ir_version 8, opset 15, tek çizge.
type Builder struct {
	nodes, inits [][]byte
}

// Initializer bir ağırlık ekler.
func (b *Builder) Initializer(t Tensor) *Builder {
	b.inits = append(b.inits, TensorProto(t))
	return b
}

// Node bir düğüm ekler.
func (b *Builder) Node(n Node) *Builder {
	b.nodes = append(b.nodes, NodeProto(n))
	return b
}

// Constant değeri t olan bir Constant düğümü ekler; çıkışı output'tur.
func (b *Builder) Constant(name, output string, t Tensor) *Builder {
	return b.Node(Node{Name: name, Op: "Constant", Outputs: []string{output}, Attrs: [][]byte{AttrTensor("value", t)}})
}

// Bytes dosyanın baytlarıdır. Alanlar onnx'in yazdığı sırayladır: düğümler ağırlıklardan önce.
func (b *Builder) Bytes() []byte {
	var g []byte
	for _, n := range b.nodes {
		g = Len(g, 1, n)
	}
	g = Len(g, 2, []byte("g"))
	for _, t := range b.inits {
		g = Len(g, 5, t)
	}
	m := VarintField(nil, 1, 8)
	m = Len(m, 2, []byte("onnxtest"))
	m = Len(m, 7, g)
	return Len(m, 8, VarintField(nil, 2, 15)) // opset_import: domain "", version 15
}

// Tag bir alan etiketini ekler.
func Tag(b []byte, field, wire int) []byte {
	return binary.AppendUvarint(b, uint64(field)<<3|uint64(wire))
}

// VarintField bir varint alanı ekler.
func VarintField(b []byte, field int, v uint64) []byte {
	return binary.AppendUvarint(Tag(b, field, WireVarint), v)
}

// Len boylu bir alan ekler.
func Len(b []byte, field int, payload []byte) []byte {
	b = binary.AppendUvarint(Tag(b, field, WireLen), uint64(len(payload)))
	return append(b, payload...)
}

// TensorProto t'nin baytlarıdır.
func TensorProto(t Tensor) []byte {
	var b []byte
	if len(t.Dims) > 0 {
		var dims []byte
		for _, d := range t.Dims {
			dims = binary.AppendUvarint(dims, uint64(d))
		}
		b = Len(b, 1, dims) // paketli
	}
	b = VarintField(b, 2, uint64(int64(t.Type)))
	switch {
	case len(t.Floats) > 0:
		b = Len(b, 4, F32(t.Floats...))
	case len(t.Int32s) > 0:
		var p []byte
		for _, v := range t.Int32s {
			p = binary.AppendUvarint(p, uint64(int64(v)))
		}
		b = Len(b, 5, p)
	case len(t.Int64s) > 0:
		var p []byte
		for _, v := range t.Int64s {
			p = binary.AppendUvarint(p, uint64(v))
		}
		b = Len(b, 7, p)
	}
	b = Len(b, 8, []byte(t.Name))
	if t.Raw != nil {
		b = Len(b, 9, t.Raw)
	}
	if len(t.Doubles) > 0 {
		var p []byte
		for _, v := range t.Doubles {
			p = binary.LittleEndian.AppendUint64(p, math.Float64bits(v))
		}
		b = Len(b, 10, p)
	}
	if t.External {
		entry := Len(Len(nil, 1, []byte("location")), 2, []byte("weights.bin"))
		b = Len(b, 13, entry)
		b = VarintField(b, 14, 1)
	}
	return b
}

// NodeProto n'nin baytlarıdır.
func NodeProto(n Node) []byte {
	var b []byte
	for _, s := range n.Inputs {
		b = Len(b, 1, []byte(s))
	}
	for _, s := range n.Outputs {
		b = Len(b, 2, []byte(s))
	}
	if n.Name != "" {
		b = Len(b, 3, []byte(n.Name))
	}
	b = Len(b, 4, []byte(n.Op))
	for _, a := range n.Attrs {
		b = Len(b, 5, a)
	}
	if n.Domain != "" {
		b = Len(b, 7, []byte(n.Domain))
	}
	return b
}

// AttrTensor tensör değerli bir öznitelik kurar (tür 4 = TENSOR).
func AttrTensor(name string, t Tensor) []byte {
	return VarintField(Len(Len(nil, 1, []byte(name)), 5, TensorProto(t)), 20, 4)
}

// AttrFloat tek ondalık değerli bir öznitelik kurar (tür 1 = FLOAT).
func AttrFloat(name string, f float32) []byte {
	b := Tag(Len(nil, 1, []byte(name)), 2, WireFixed32)
	return VarintField(binary.LittleEndian.AppendUint32(b, math.Float32bits(f)), 20, 1)
}

// AttrFloats ondalık dizisi değerli bir öznitelik kurar (tür 6 = FLOATS).
func AttrFloats(name string, f ...float32) []byte {
	return VarintField(Len(Len(nil, 1, []byte(name)), 7, F32(f...)), 20, 6)
}

// AttrInt tek tam sayı değerli bir öznitelik kurar (tür 2 = INT).
func AttrInt(name string, v int64) []byte {
	return VarintField(VarintField(Len(nil, 1, []byte(name)), 3, uint64(v)), 20, 2)
}

// AttrInts tam sayı dizisi değerli bir öznitelik kurar (tür 7 = INTS).
func AttrInts(name string, v ...int64) []byte {
	var p []byte
	for _, x := range v {
		p = binary.AppendUvarint(p, uint64(x))
	}
	return VarintField(Len(Len(nil, 1, []byte(name)), 8, p), 20, 7)
}

// F32 ondalıkların küçük uçlu baytlarıdır.
func F32(v ...float32) []byte {
	var b []byte
	for _, x := range v {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(x))
	}
	return b
}

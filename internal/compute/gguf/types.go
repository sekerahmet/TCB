package gguf

import "strconv"

// TensorType ggml'in tensör türü numarasıdır (ggml.h, ggml_type); dosyadaki değerdir.
type TensorType uint32

// TR'nin okuduğu türlerin numaraları (15.55).
const (
	TypeF32  TensorType = 0
	TypeF16  TensorType = 1
	TypeQ4_0 TensorType = 2
	TypeQ5_0 TensorType = 6
	TypeQ8_0 TensorType = 8
	TypeQ4_K TensorType = 12
	TypeQ6_K TensorType = 14
	TypeBF16 TensorType = 30
)

// typeInfo bir türün adı, blok öğe sayısı ve blok baytıdır.
type typeInfo struct {
	name  string
	block uint64
	bytes uint64
}

// typeTable bütün ggml türleridir (llama.cpp cd959bae, ggml.c type_traits ve ggml-common.h'nin
// static_assert'leri). TR'nin okumadığı türler de buradadır: üstveri ve parçalayıcı okumak
// için bütün dosyanın düzeni denetlenir, tensörün boyu türün boyundan gelir. Numaralar
// boşlukludur (4, 5, 31–33, 36–38 ggml'den kaldırıldı).
var typeTable = map[TensorType]typeInfo{
	0: {"F32", 1, 4}, 1: {"F16", 1, 2}, 2: {"Q4_0", 32, 18}, 3: {"Q4_1", 32, 20},
	6: {"Q5_0", 32, 22}, 7: {"Q5_1", 32, 24}, 8: {"Q8_0", 32, 34}, 9: {"Q8_1", 32, 36},
	10: {"Q2_K", 256, 84}, 11: {"Q3_K", 256, 110}, 12: {"Q4_K", 256, 144}, 13: {"Q5_K", 256, 176},
	14: {"Q6_K", 256, 210}, 15: {"Q8_K", 256, 292}, 16: {"IQ2_XXS", 256, 66}, 17: {"IQ2_XS", 256, 74},
	18: {"IQ3_XXS", 256, 98}, 19: {"IQ1_S", 256, 50}, 20: {"IQ4_NL", 32, 18}, 21: {"IQ3_S", 256, 110},
	22: {"IQ2_S", 256, 82}, 23: {"IQ4_XS", 256, 136}, 24: {"I8", 1, 1}, 25: {"I16", 1, 2},
	26: {"I32", 1, 4}, 27: {"I64", 1, 8}, 28: {"F64", 1, 8}, 29: {"IQ1_M", 256, 56},
	30: {"BF16", 1, 2}, 34: {"TQ1_0", 256, 54}, 35: {"TQ2_0", 256, 66}, 39: {"MXFP4", 32, 17},
	40: {"NVFP4", 64, 36}, 41: {"Q1_0", 128, 18}, 42: {"Q2_0", 64, 18}, 43: {"F8_E4M3", 1, 1},
}

// String türün GGUF'taki adıdır; bilinmeyen numara sayı olarak yazılır.
func (t TensorType) String() string {
	if info, ok := typeTable[t]; ok {
		return info.name
	}
	return strconv.FormatUint(uint64(t), 10)
}

// ValueType üstveri değerinin türüdür (gguf.h, gguf_type).
type ValueType uint32

// Üstveri türleri, dosyadaki numaralarıyla.
const (
	Uint8 ValueType = iota
	Int8
	Uint16
	Int16
	Uint32
	Int32
	Float32
	Bool
	String
	Array
	Uint64
	Int64
	Float64
	valueTypeCount
)

var valueNames = [valueTypeCount]string{"UINT8", "INT8", "UINT16", "INT16", "UINT32", "INT32", "FLOAT32",
	"BOOL", "STRING", "ARRAY", "UINT64", "INT64", "FLOAT64"}

// String türün gguf.h'deki adıdır; bilinmeyen numara sayı olarak yazılır.
func (v ValueType) String() string {
	if v < valueTypeCount {
		return valueNames[v]
	}
	return strconv.FormatUint(uint64(v), 10)
}

// width türün dosyadaki bayt boyudur; metinde boy alanının boyu (metnin en küçük boyu).
func (v ValueType) width() uint64 {
	switch v {
	case Uint8, Int8, Bool:
		return 1
	case Uint16, Int16:
		return 2
	case Uint32, Int32, Float32:
		return 4
	}
	return 8
}

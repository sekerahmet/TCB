package onnx_test

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"math"
	"math/big"
	"slices"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/onnx"
	"github.com/sekerahmet/TCB/internal/compute/onnx/onnxtest"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func decode(data []byte) ([]onnx.Tensor, error) {
	return onnx.Decode(bufio.NewReader(bytes.NewReader(data)), int64(len(data)), onnx.Options{})
}

// Kural (15.47): veri raw_data'da değilse türün alanından okunur (onnx.proto3): int32_data INT32,
// INT16, INT8, UINT8, BOOL, FLOAT16 ve BFLOAT16'yı taşır (değer türün bitlerine kesilir, mantıkta
// sıfırdan farklı değer 1), int64_data INT64'ü, float_data FLOAT'u, double_data DOUBLE'ı. Paketli
// ve paketsiz yazım aynı sonucu verir.
func TestDecodeTypedFields(t *testing.T) {
	for _, c := range []struct {
		name string
		t    onnxtest.Tensor
		want []byte
	}{
		{"int32", onnxtest.Tensor{Type: onnx.Int32, Dims: []int64{2}, Int32s: []int32{-1, 7}}, []byte{0xff, 0xff, 0xff, 0xff, 7, 0, 0, 0}},
		{"int16", onnxtest.Tensor{Type: onnx.Int16, Dims: []int64{2}, Int32s: []int32{-2, 300}}, []byte{0xfe, 0xff, 0x2c, 0x01}},
		{"int8", onnxtest.Tensor{Type: onnx.Int8, Dims: []int64{2}, Int32s: []int32{-3, 5}}, []byte{0xfd, 5}},
		{"uint8", onnxtest.Tensor{Type: onnx.Uint8, Dims: []int64{1}, Int32s: []int32{200}}, []byte{200}},
		{"bool", onnxtest.Tensor{Type: onnx.Bool, Dims: []int64{3}, Int32s: []int32{0, 1, 5}}, []byte{0, 1, 1}},
		{"fp16", onnxtest.Tensor{Type: onnx.Float16, Dims: []int64{1}, Int32s: []int32{0x3C00}}, []byte{0x00, 0x3C}},
		{"bf16", onnxtest.Tensor{Type: onnx.BFloat16, Dims: []int64{1}, Int32s: []int32{0xC000}}, []byte{0x00, 0xC0}},
		{"int64", onnxtest.Tensor{Type: onnx.Int64, Dims: []int64{1}, Int64s: []int64{-2}}, binary.LittleEndian.AppendUint64(nil, math.MaxUint64-1)},
		{"float", onnxtest.Tensor{Type: onnx.Float, Dims: []int64{2}, Floats: []float32{1, -0.5}}, onnxtest.F32(1, -0.5)},
		{"double", onnxtest.Tensor{Type: onnx.Double, Dims: []int64{1}, Doubles: []float64{2.5}}, binary.LittleEndian.AppendUint64(nil, math.Float64bits(2.5))},
		// raw_data varsa türün alanı okunmaz (numpy_helper.to_array gibi).
		{"raw önce", onnxtest.Tensor{Type: onnx.Float, Dims: []int64{1}, Raw: onnxtest.F32(7), Floats: []float32{1}}, onnxtest.F32(7)},
		// Türe ait olmayan alan okunmaz: INT64'ün verisi int64_data'dadır.
		{"başka alan", onnxtest.Tensor{Type: onnx.Int64, Dims: []int64{0}, Floats: []float32{1}}, nil},
	} {
		c.t.Name = c.name
		got, err := decode((&onnxtest.Builder{}).Initializer(c.t).Bytes())
		if err != nil || len(got) != 1 || !bytes.Equal(got[0].Data, c.want) {
			t.Errorf("%s: %v %v, beklenen % x", c.name, got, err, c.want)
		}
	}
	// Paketsiz: her öğe kendi etiketiyle (eksenler ve float_data).
	var tp []byte
	tp = onnxtest.VarintField(tp, 1, 2)
	tp = onnxtest.VarintField(tp, 2, uint64(onnx.Float))
	for _, x := range []float32{3, 4} {
		tp = binary.LittleEndian.AppendUint32(onnxtest.Tag(tp, 4, onnxtest.WireFixed32), math.Float32bits(x))
	}
	tp = onnxtest.Len(tp, 8, []byte("p"))
	file := onnxtest.Len(onnxtest.VarintField(nil, 1, 8), 7, onnxtest.Len(nil, 5, tp))
	got, err := decode(file)
	if err != nil || len(got) != 1 || !slices.Equal(got[0].Dims, []uint64{2}) || !bytes.Equal(got[0].Data, onnxtest.F32(3, 4)) {
		t.Errorf("paketsiz: %v %v", got, err)
	}
}

// Kural: Options.Alloc yalnız ağırlığın raw_data'sına yer verir, sahibi tensörle döner; Check
// veri boyundan önce koşar ve hatası olduğu gibi döner.
func TestDecodeOptions(t *testing.T) {
	file := (&onnxtest.Builder{}).
		Constant("c", "c_out", onnxtest.Tensor{Type: onnx.Float, Raw: onnxtest.F32(1)}).
		Initializer(onnxtest.Tensor{Name: "w", Dims: []int64{1}, Type: onnx.Float, Raw: onnxtest.F32(2)}).Bytes()
	calls := 0
	got, err := onnx.Decode(bufio.NewReader(bytes.NewReader(file)), int64(len(file)), onnx.Options{
		Alloc: func(n int) ([]byte, any) { calls++; return make([]byte, n), "sahip" },
	})
	if err != nil || calls != 1 || got[0].Owner != nil || got[1].Owner != "sahip" {
		t.Fatalf("Alloc: %d çağrı, %v %v", calls, got, err)
	}
	stop := messages.New(messages.H6748, "denetim")
	_, err = onnx.Decode(bufio.NewReader(bytes.NewReader(file)), int64(len(file)), onnx.Options{
		Check: func(*onnx.Tensor) error { return stop },
	})
	if err != stop {
		t.Fatalf("Check'in hatası dönmedi: %v", err)
	}
}

// Kural: Decode size'ı aşmaz: akış uzun olsa da yalnız size bayt okunur ve ardından gelen bayt
// dosyanın parçası sayılmaz.
func TestDecodeHoldsToSize(t *testing.T) {
	file := (&onnxtest.Builder{}).Initializer(onnxtest.Tensor{Name: "w", Dims: []int64{1}, Type: onnx.Float, Raw: onnxtest.F32(1)}).Bytes()
	stream := append(slices.Clone(file), 0xff, 0xff, 0xff)
	got, err := onnx.Decode(bufio.NewReader(bytes.NewReader(stream)), int64(len(file)), onnx.Options{})
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
}

// Kural: kesilmiş dosya (her önek) çökmez; ya geçerli bir dosyadır (alan sınırında kesilmiş) ya
// da ONNX okuyucusunun kodlu hatasıdır.
func TestDecodeEveryPrefix(t *testing.T) {
	file := (&onnxtest.Builder{}).
		Node(onnxtest.Node{Name: "n", Op: "Conv", Inputs: []string{"x", "onnx::W"}}).
		Constant("c", "c_out", onnxtest.Tensor{Dims: []int64{2}, Type: onnx.Int64, Int64s: []int64{1, -1}}).
		Initializer(onnxtest.Tensor{Name: "onnx::W", Dims: []int64{2}, Type: onnx.Float, Raw: onnxtest.F32(1, 2)}).Bytes()
	codes := []messages.Code{messages.H6742, messages.H6743, messages.H6747}
	for n := range len(file) {
		_, err := decode(file[:n])
		if err != nil && !slices.ContainsFunc(codes, func(c messages.Code) bool { return messages.Has(err, c) }) {
			t.Errorf("%d bayt: %v", n, err)
		}
	}
}

// FuzzLoadOnnx: ONNX okuyucusu hiçbir baytta çökmez; kabul ettiği her tensörün verisi şeklinin
// istediği boydadır ve adlar tektir. Derin arama:
//
//	go test ./internal/compute/onnx -run '^$' -fuzz FuzzLoadOnnx -fuzztime 5m
func FuzzLoadOnnx(f *testing.F) {
	f.Add((&onnxtest.Builder{}).
		Node(onnxtest.Node{Name: "n", Op: "Conv", Inputs: []string{"x", "onnx::W"}}).
		Node(onnxtest.Node{Name: "k", Op: "Constant", Outputs: []string{"k"}, Attrs: [][]byte{onnxtest.AttrInts("value_ints", 1, 2)}}).
		Initializer(onnxtest.Tensor{Name: "onnx::W", Dims: []int64{2}, Type: onnx.Float, Raw: onnxtest.F32(1, 2)}).
		Initializer(onnxtest.Tensor{Name: "h", Dims: []int64{1}, Type: onnx.Float16, Int32s: []int32{0x3C00}}).Bytes())
	f.Add((&onnxtest.Builder{}).Bytes())
	f.Add([]byte{0x08})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decode(data)
		if err != nil {
			return
		}
		seen := map[string]bool{}
		for _, x := range got {
			want := big.NewInt(int64(map[onnx.DataType]int{onnx.Float: 4, onnx.Uint8: 1, onnx.Int8: 1, onnx.Int16: 2,
				onnx.Int32: 4, onnx.Int64: 8, onnx.Bool: 1, onnx.Float16: 2, onnx.Double: 8, onnx.BFloat16: 2}[x.Type]))
			for _, d := range x.Dims {
				want.Mul(want, new(big.Int).SetUint64(d))
			}
			if !want.IsUint64() || want.Uint64() != uint64(len(x.Data)) || seen[x.Name] {
				t.Fatalf("%q: tür %v, şekil %v, %d bayt", x.Name, x.Type, x.Dims, len(x.Data))
			}
			seen[x.Name] = true
		}
	})
}

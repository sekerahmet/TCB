package ml

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func decodeGGUFBytes(data []byte) ([]fileEntry, error) {
	return decodeTensorFile(bytes.NewReader(data), int64(len(data)))
}

func float32Bytes(values ...float32) []byte {
	var out []byte
	for _, v := range values {
		out = binary.LittleEndian.AppendUint32(out, math.Float32bits(v))
	}
	return out
}

// TestGGUFLoadsFloatTensors: F32, F16 ve BF16 içerikten tanınır, şekil dosyadakinin tersidir,
// 16 bitlik tensörler bitleriyle gelir (15.55).
func TestGGUFLoadsFloatTensors(t *testing.T) {
	b := (&gguftest.Builder{}).KV("general.architecture", "qwen3")
	b.Tensor("f32", gguf.TypeF32, []uint64{3, 2}, float32Bytes(1, 2, 3, 4, 5, 6))
	b.Tensor("f16", gguf.TypeF16, []uint64{2}, []byte{0x00, 0x3C, 0x00, 0xC0})
	b.Tensor("bf16", gguf.TypeBF16, []uint64{2}, []byte{0x80, 0x3F, 0x00, 0xC0})
	b.Tensor("skaler", gguf.TypeF32, nil, float32Bytes(9))
	b.Tensor("boş", gguf.TypeF32, []uint64{0, 4}, nil)
	entries, err := decodeGGUFBytes(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	w := entries[0].tensor()
	if !slices.Equal(w.Shape(), []int{2, 3}) || !slices.Equal(w.Data(), []float32{1, 2, 3, 4, 5, 6}) {
		t.Errorf("f32: %v %v", w.Shape(), w.Data())
	}
	if g := entries[1].tensor(); g.DType() != tensor.Float16 || !slices.Equal(g.Data(), []float32{1, -2}) {
		t.Errorf("f16: %v %v", g.DType(), g.Data())
	}
	if g := entries[2].tensor(); g.DType() != tensor.BFloat16 || !slices.Equal(g.Data(), []float32{1, -2}) {
		t.Errorf("bf16: %v %v", g.DType(), g.Data())
	}
	if g := entries[3]; len(g.shape) != 0 || g.data[0] != 9 {
		t.Errorf("skaler: %+v", g)
	}
	if g := entries[4]; !slices.Equal(g.shape, []int{4, 0}) || g.count() != 0 {
		t.Errorf("boş: %+v", g)
	}
}

// TestGGUFRejects: 15.55'in dosyaya bağlı hataları. Nicemli türler bu adımda "başka tür"dür.
func TestGGUFRejects(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		code messages.Code
	}{
		{"Q4_0 henüz yok", (&gguftest.Builder{}).Tensor("w", gguf.TypeQ4_0, []uint64{32}, make([]byte, 18)).Bytes(), messages.H6674},
		{"Q5_0", (&gguftest.Builder{}).Tensor("blk.0.attn_q.weight", gguf.TypeQ5_0, []uint64{32}, make([]byte, 22)).Bytes(), messages.H6674},
		{"bölünmüş", (&gguftest.Builder{}).KV("split.count", uint16(3)).Bytes(), messages.H6675},
		{"bölünmüş uint8", (&gguftest.Builder{}).KV("split.count", uint8(2)).Bytes(), messages.H6675},
		{"bölünmüş uint32", (&gguftest.Builder{}).KV("split.count", uint32(2)).Bytes(), messages.H6675},
		{"bölünmüş uint64", (&gguftest.Builder{}).KV("split.count", uint64(2)).Bytes(), messages.H6675},
		{"bölünmüş int16", (&gguftest.Builder{}).KV("split.count", int16(2)).Bytes(), messages.H6675},
		{"bölünmüş int32", (&gguftest.Builder{}).KV("split.count", int32(2)).Bytes(), messages.H6675},
		{"bölünmüş int64", (&gguftest.Builder{}).KV("split.count", int64(2)).Bytes(), messages.H6675},
		{"parça sayısı metin", (&gguftest.Builder{}).KV("split.count", "2").Bytes(), messages.H6693},
		{"parça sayısı eksi", (&gguftest.Builder{}).KV("split.count", int32(-2)).Bytes(), messages.H6693},
		{"parça sayısı ondalık", (&gguftest.Builder{}).KV("split.count", float32(2)).Bytes(), messages.H6693},
		{"sıfır eksenle dev şekil", (&gguftest.Builder{}).Tensor("w", gguf.TypeF32, []uint64{0, 1 << 40}, nil).Bytes(), messages.H6649},
		{"kesik GGUF", []byte("GGUF"), messages.H6658},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := decodeGGUFBytes(c.data); !messages.Has(err, c.code) {
				t.Errorf("hata %v; %s olmalı", err, c.code)
			}
		})
	}
}

// TestGGUFDataShorterThanTheStatedSize: boy akıştan büyükse veri okuması hatayı tensörün adıyla söyler.
func TestGGUFDataShorterThanTheStatedSize(t *testing.T) {
	data := (&gguftest.Builder{}).Tensor("w", gguf.TypeF32, []uint64{64}, make([]byte, 256)).Bytes()
	cut := bufio.NewReader(bytes.NewReader(data[:len(data)-100]))
	if _, err := decodeGGUF(cut, int64(len(data))); !messages.Has(err, messages.H6653) {
		t.Errorf("hata %v", err)
	}
	padded := (&gguftest.Builder{}).Tensor("a", gguf.TypeF32, []uint64{1}, make([]byte, 4)).
		Tensor("w", gguf.TypeF32, []uint64{64}, make([]byte, 256)).Bytes()
	gap := bufio.NewReader(bytes.NewReader(padded[:len(padded)-256-10]))
	if _, err := decodeGGUF(gap, int64(len(padded))); !messages.Has(err, messages.H6653) {
		t.Errorf("hata %v", err)
	}
}

// TestGGUFSingleSplitIsAFile: tek parçalı "bölünmüş" dosya tam dosyadır.
func TestGGUFSingleSplitIsAFile(t *testing.T) {
	data := (&gguftest.Builder{}).KV("split.count", uint16(1)).Bytes()
	if _, err := decodeGGUFBytes(data); err != nil {
		t.Fatal(err)
	}
}

// TestGGUFThroughLoadTensors uçtan uca yerleşikten geçer: biçim uzantıdan değil içerikten
// tanınır, hata dosyanın yoluyla sarılır.
func TestGGUFThroughLoadTensors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uzantisiz")
	data := (&gguftest.Builder{}).Tensor("w", gguf.TypeF32, []uint64{2}, float32Bytes(1, 2)).Bytes()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, readErr, osErr := readFromFile(path)
	if readErr != nil || osErr != nil {
		t.Fatal(readErr, osErr)
	}
	if entries[0].ad != "w" || !slices.Equal(entries[0].data, []float32{1, 2}) {
		t.Errorf("%+v", entries[0])
	}
}

// FuzzGGUFDecode: GGUF dosyası güvenilmeyen girdidir (indirilen model). Çözücü hiçbir baytta
// çökmemeli, kabul ettiği her dosyadan tensör kurulabilmelidir ve öğe sayısı dosyayla orantılı
// kalmalıdır. Ayırmanın ölçüsü bu testte değil, TestDecodeAllocatesWithinTheFile'dadır.
// Normal `go test` yalnızca tohumları koşar; derin arama için:
//
//	go test ./internal/stdlib/ml -run '^$' -fuzz FuzzGGUFDecode -fuzztime 5m
func FuzzGGUFDecode(f *testing.F) {
	full := (&gguftest.Builder{}).KV("general.architecture", "qwen3").KV("n", uint32(2)).
		KV("t", []string{"a", "b"}).KV("s", []float32{1}).KV("y", []int32{1}).
		Tensor("w", gguf.TypeF32, []uint64{2, 2}, make([]byte, 16)).
		Tensor("h", gguf.TypeF16, []uint64{4}, make([]byte, 8))
	f.Add(full.Bytes())
	f.Add((&gguftest.Builder{}).Bytes())
	f.Add((&gguftest.Builder{}).KV("general.alignment", uint32(0)).Bytes())
	f.Add((&gguftest.Builder{}).Tensor("q", gguf.TypeQ8_0, []uint64{32}, make([]byte, 34)).Bytes())
	f.Add([]byte("GGUF"))
	f.Add(gguftest.Header(3, math.MaxUint64, math.MaxUint64))
	f.Add(gguftest.AppendTensor(gguftest.Header(3, 1, 0), "w", []uint64{1 << 40, 1 << 20}, 0, 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		entries, err := decodeGGUFBytes(data)
		if err != nil {
			return
		}
		total := 0
		for _, g := range entries {
			n, limit := 1, 1
			for _, d := range g.shape {
				n *= d
				limit *= max(d, 1)
			}
			if limit > interp.MaxCollectionSize && n == 0 {
				t.Fatalf("%q: sıfır eksenli şekil %v liste sınırını aşıyor", g.ad, g.shape)
			}
			if n != g.count() {
				t.Fatalf("%q: şekil %v ile %d eleman tutmuyor", g.ad, g.shape, g.count())
			}
			total += g.count()
			if tt := g.tensor(); tt.Numel() != g.count() {
				t.Fatalf("%q: tensör %d eleman, veri %d", g.ad, tt.Numel(), g.count())
			}
		}
		if total*2 > len(data) {
			t.Fatalf("%d bayttan %d eleman okundu: veri dosyadan büyük", len(data), total)
		}
	})
}

// q8Row bir Q8_0 satırıdır: ölçek 2.0, ilk kodlar 3, −1, −128.
func q8Row() []byte {
	raw := make([]byte, 34)
	binary.LittleEndian.PutUint16(raw, 0x4000)
	raw[2], raw[3], raw[4] = 3, 0xFF, 0x80
	return raw
}

// TestGGUFLoadsQuantizedTensors: nicemlenmiş tensör dosyanın bitleriyle gelir, açılmaz; şekil
// dosyadakinin tersidir; TR_requires_grad yalnız nicemlenmemiş ondalığı izler (15.46, 15.55).
func TestGGUFLoadsQuantizedTensors(t *testing.T) {
	dir := t.TempDir()
	b := (&gguftest.Builder{}).Tensor("q", gguf.TypeQ8_0, []uint64{32, 2}, append(q8Row(), make([]byte, 34)...)).
		Tensor("k", gguf.TypeQ4_K, []uint64{256}, make([]byte, 144)).
		Tensor("s", gguf.TypeQ6_K, []uint64{256}, make([]byte, 210)).
		Tensor("f", gguf.TypeF32, []uint64{1}, float32Bytes(1))
	writeGGUF(t, dir, "m.gguf", b)
	out, err := fromProgram(t, dir, `
TR_var d = TR_load_tensors("m.gguf", TR_requires_grad: TR_true)
TR_var q = d["q"]
TR_print(q.TR_dtype, q.TR_shape, d["k"].TR_dtype, d["s"].TR_dtype)
TR_print(q[0].TR_to_dtype("float32")[0..3].TR_tolist)
TR_print(TR_sum(q[1].TR_to_dtype("float32")).TR_item())
TR_print(d["f"].TR_requires_grad)
TR_print(q.TR_requires_grad)
TR_print(q)
`)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out, "\n")
	for i, want := range []string{"q8 [2, 32] q4_k q6_k", "[6.0, -2.0, -256.0]", "0.0", "true", "false"} {
		if lines[i] != want {
			t.Errorf("satır %d: %q; beklenen %q", i, lines[i], want)
		}
	}
	if print := strings.Join(lines[5:], "\n"); !strings.Contains(print, "q8") || !strings.Contains(print, "-256") {
		t.Errorf("yazdırma:\n%s", print)
	}
}

// TestGGUFQuantRejects: NaN ve sonsuz ölçek yükleme hatasıdır, ileti tensörü ve bloğu söyler; sıfır
// ölçek geçerlidir (15.55). Son eksen bloğun katı değilse dosya başlıkta durur.
func TestGGUFQuantRejects(t *testing.T) {
	bad := func(kind gguf.TensorType, dim uint64, size, at int, scale uint16) []byte {
		raw := make([]byte, 2*size)
		binary.LittleEndian.PutUint16(raw[size+at:], scale)
		return (&gguftest.Builder{}).Tensor("blk.3.w", kind, []uint64{dim, 2}, raw).Bytes()
	}
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"q8 NaN", bad(gguf.TypeQ8_0, 32, 34, 0, 0x7E00)},
		{"q8 sonsuz", bad(gguf.TypeQ8_0, 32, 34, 0, 0x7C00)},
		{"q4_k d", bad(gguf.TypeQ4_K, 256, 144, 0, 0xFC00)},
		{"q4_k dmin", bad(gguf.TypeQ4_K, 256, 144, 2, 0x7E00)},
		{"q6_k", bad(gguf.TypeQ6_K, 256, 210, 208, 0x7C00)},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := decodeGGUFBytes(c.data)
			if !messages.Has(err, messages.H6686) || !strings.Contains(err.Error(), "blk.3.w") || !strings.Contains(err.Error(), "1") {
				t.Errorf("hata %v", err)
			}
		})
	}
	zero := (&gguftest.Builder{}).Tensor("w", gguf.TypeQ8_0, []uint64{32}, make([]byte, 34)).Bytes()
	if _, err := decodeGGUFBytes(zero); err != nil {
		t.Errorf("sıfır ölçek: %v", err)
	}
	short := (&gguftest.Builder{}).Tensor("w", gguf.TypeQ8_0, []uint64{33}, make([]byte, 34)).Bytes()
	if _, err := decodeGGUFBytes(short); !messages.Has(err, messages.H6671) {
		t.Errorf("blok katı: %v", err)
	}
}

// TestGGUFQuantEntryIsRaw: girdinin öğe sayısı ve tensörü blok baytından gelir.
func TestGGUFQuantEntryIsRaw(t *testing.T) {
	b := (&gguftest.Builder{}).Tensor("w", gguf.TypeQ6_K, []uint64{256, 3}, make([]byte, 630))
	entries, err := decodeGGUFBytes(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	g := entries[0]
	if g.count() != 768 || len(g.raw) != 630 || g.dtype != tensor.Q6K {
		t.Errorf("girdi: %d öğe, %d bayt, %v", g.count(), len(g.raw), g.dtype)
	}
	if got := g.tensor(); got.DType() != tensor.Q6K || got.Shape()[0] != 3 || got.Shape()[1] != 256 {
		t.Errorf("tensör %v %v", got.DType(), got.Shape())
	}
}

// Kural (15.53): dosyanın büyük tensörü öbek dışı bloğa gelir, değerleri ve bitleri öbekteki yolla
// aynıdır; küçük tensör öbekte kalır. GGUF'un üç yolu: nicemli, float32, 16 bit.
func TestGGUFLargeTensorsGoToBlocks(t *testing.T) {
	needBlocks(t)
	values := bigFloats()
	bits := make([]uint16, 1<<20)
	wide := make([]byte, 2*len(bits))
	for i := range bits {
		bits[i] = uint16(i)*2 + 0x3C00 | 1
		wide[2*i], wide[2*i+1] = byte(bits[i]), byte(bits[i]>>8)
	}
	const rows, cols = 1024, 1024
	quant := make([]byte, rows*cols/32*34)
	for at := 0; at < len(quant); at += 34 {
		quant[at+1] = 0x3C // ölçek 1
		for j := 2; j < 34; j++ {
			quant[at+j] = byte(at + j)
		}
	}
	b := (&gguftest.Builder{}).KV("general.architecture", "qwen3")
	b.Tensor("q", gguf.TypeQ8_0, []uint64{cols, rows}, quant)
	b.Tensor("f", gguf.TypeF32, []uint64{uint64(len(values))}, float32Bytes(values...))
	b.Tensor("h", gguf.TypeF16, []uint64{uint64(len(bits))}, wide)
	b.Tensor("norm", gguf.TypeF32, []uint64{4}, float32Bytes(1, 2, 3, 4))
	entries, err := decodeGGUFBytes(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for i, inBlock := range []bool{true, true, true, false} {
		if (entries[i].block != nil) != inBlock {
			t.Errorf("%s: blokta %v, %v olmalıydı", entries[i].ad, entries[i].block != nil, inBlock)
		}
	}
	if q := entries[0].tensor(); !slices.Equal(q.Raw(), quant) || !slices.Equal(q.Shape(), []int{rows, cols}) {
		t.Errorf("q: baytlar ya da şekil farklı %v", q.Shape())
	}
	if f := entries[1].tensor(); !slices.Equal(f.Data(), values) {
		t.Error("f: değerler farklı")
	}
	if h := entries[2].tensor(); h.DType() != tensor.Float16 || !slices.Equal(h.Bits16s(), bits) {
		t.Errorf("h: %v", h.DType())
	}
	if n := entries[3].tensor(); !slices.Equal(n.Data(), []float32{1, 2, 3, 4}) {
		t.Errorf("norm: %v", n.Data())
	}
}

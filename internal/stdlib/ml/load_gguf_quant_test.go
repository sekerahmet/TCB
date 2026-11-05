package ml

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/gguf/gguftest"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

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

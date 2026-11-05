package checkpoint_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// sixteenBitFile values'u tek tensörlü ("w") bir safetensors dosyasına yazar: TR programı sinyal
// NaN'ı başka yoldan kuramaz.
func sixteenBitFile(t *testing.T, fpath, dtype string, values []uint16) {
	t.Helper()
	header := fmt.Sprintf(`{"w":{"dtype":%q,"shape":[%d],"data_offsets":[0,%d]}}`, dtype, len(values), 2*len(values))
	file := binary.LittleEndian.AppendUint64(nil, uint64(len(header)))
	file = append(file, header...)
	for _, h := range values {
		file = binary.LittleEndian.AppendUint16(file, h)
	}
	if err := os.WriteFile(fpath, file, 0o644); err != nil {
		t.Fatal(err)
	}
}

// sixteenBitFileValues sixteenBitFile'ın yazdığı biçimde bir dosyanın son n öğesini okur.
func sixteenBitFileValues(t *testing.T, fpath string, n int) []uint16 {
	t.Helper()
	file, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	tail := file[len(file)-2*n:]
	out := make([]uint16, n)
	for i := range out {
		out[i] = binary.LittleEndian.Uint16(tail[2*i:])
	}
	return out
}

// TestSixteenBitTensorKeepsItsBits: 16 bitlik parametrenin bitleri noktaya yazılır ve sürdürmede
// geri gelir; sinyal NaN'ı da susmaz (20.17 "bit düzeyinde aynı parametreler").
func TestSixteenBitTensorKeepsItsBits(t *testing.T) {
	for _, c := range []struct {
		dtype  string
		values []uint16
	}{
		{"BF16", []uint16{0x7F81, 0xFF81, 0x7FC1, 0x3F80}},
		{"F16", []uint16{0x7C01, 0xFC01, 0x7E01, 0x3C00}},
	} {
		t.Run(c.dtype, func(t *testing.T) {
			dir := t.TempDir()
			runDir := filepath.Join(dir, "run")
			source, zero, back := filepath.Join(dir, "a.safetensors"), filepath.Join(dir, "z.safetensors"), filepath.Join(dir, "b.safetensors")
			sixteenBitFile(t, source, c.dtype, c.values)
			sixteenBitFile(t, zero, c.dtype, make([]uint16, len(c.values)))
			if out, err := run(t, fmt.Sprintf(`
TR_var model = {"w": TR_load_tensors(%q)["w"]}
TR_var kn = TR_Checkpoint(%q, {"model": model})
kn.TR_update(1)
kn.TR_save()
`, trPath(source), trPath(runDir))); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			file, err := os.ReadFile(filepath.Join(runDir, "checkpoints", "step_0000000001", "tensors.trt"))
			if err != nil {
				t.Fatal(err)
			}
			var want []byte
			for _, h := range c.values {
				want = binary.LittleEndian.AppendUint16(want, h)
			}
			if !bytes.Contains(file, want) {
				t.Errorf("nokta dosyası %x bitlerini tutmuyor", want)
			}
			if out, err := run(t, fmt.Sprintf(`
TR_var model = {"w": TR_load_tensors(%q)["w"]}
TR_var kn = TR_Checkpoint(%q, {"model": model})
TR_assert_equal(1, kn.TR_resume_step)
TR_save_tensors(%q, model)
`, trPath(zero), trPath(runDir), trPath(back))); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			if got := sixteenBitFileValues(t, back, len(c.values)); !slices.Equal(got, c.values) {
				t.Errorf("sürdürülen bitler %x, beklenen %x", got, c.values)
			}
		})
	}
}

// TestSixteenBitStateEntryKeepsItsValue: iyileştirici durumu noktaya float32 yazılır; 16 bitlik bir
// durum girdisi (elle yazılmış nokta) sessizce boş kalmaz, değeri float32 duruma açılır.
func TestSixteenBitStateEntryKeepsItsValue(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "run")
	program := func(tail string) string {
		return fmt.Sprintf(`
TR_var w = TR_Tensor([1.0, 2.0, 3.0], TR_requires_grad: TR_true)
TR_var op = TR_AdamW([w])
TR_var kn = TR_Checkpoint(%q, {"w": w, "op": op})
`, trPath(runDir)) + tail
	}
	if out, err := run(t, program(`
TR_backward(TR_sum(w * w))
op.TR_step()
kn.TR_update(1)
kn.TR_save()
`)); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	step := filepath.Join(runDir, "checkpoints", "step_0000000001")
	if out, err := run(t, fmt.Sprintf(`
TR_var y = TR_load_tensors(%[1]q)
y["op/m.0"] = TR_Tensor([0.5, -1.5, 2.0], TR_dtype: "bf16")
TR_save_tensors(%[1]q, y)
`, trPath(filepath.Join(step, "tensors.trt")))); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	resignCheckpoint(t, step, 1)
	out, err := run(t, program(`TR_print(op.TR_state["m.0"].TR_dtype, op.TR_state["m.0"].TR_tolist)`))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if want := "float32 [0.5, -1.5, 2.0]"; !strings.Contains(out, want) {
		t.Errorf("durum %q, beklenen %q", out, want)
	}
}

// TestBitsChangeOnlySignalingNaNs: bitlerle yazmak disk biçimini değiştirmez. Bütün 16 bitlik
// desenler iki yolla yazılır: dosyalar aynı boydadır, ayrılan yalnız sinyal NaN'larıdır.
func TestBitsChangeOnlySignalingNaNs(t *testing.T) {
	for _, c := range []struct {
		dtype  tensor.DType
		signal func(h uint16) bool
		count  int
	}{
		{tensor.BFloat16, func(h uint16) bool { return h&0x7F80 == 0x7F80 && h&0x7F != 0 && h&0x40 == 0 }, 126},
		{tensor.Float16, func(h uint16) bool { return h&0x7C00 == 0x7C00 && h&0x3FF != 0 && h&0x200 == 0 }, 1022},
	} {
		bits := make([]uint16, 1<<16)
		wide := make([]float32, len(bits))
		for i := range bits {
			bits[i] = uint16(i)
			wide[i] = tensor.FromBits16(c.dtype, bits[i])
		}
		var byValue, byBits bytes.Buffer
		for _, write := range []func() error{
			func() error {
				y := ml.NewTensorWriter(&byValue, 1)
				if err := y.Write("w", []int{len(bits)}, c.dtype, wide); err != nil {
					return err
				}
				return y.Finish()
			},
			func() error {
				y := ml.NewTensorWriter(&byBits, 1)
				if err := y.WriteBits("w", []int{len(bits)}, c.dtype, bits); err != nil {
					return err
				}
				return y.Finish()
			},
		} {
			if err := write(); err != nil {
				t.Fatal(err)
			}
		}
		if byValue.Len() != byBits.Len() {
			t.Fatalf("%v: boylar ayrı: %d, %d", c.dtype, byValue.Len(), byBits.Len())
		}
		start := byBits.Len() - 4 - 2*len(bits) // sonda 4 baytlık sağlama
		if !bytes.Equal(byValue.Bytes()[:start], byBits.Bytes()[:start]) {
			t.Fatalf("%v: başlıklar ayrı", c.dtype)
		}
		different := 0
		for i, h := range bits {
			at := start + 2*i
			same := binary.LittleEndian.Uint16(byValue.Bytes()[at:]) == binary.LittleEndian.Uint16(byBits.Bytes()[at:])
			if same == c.signal(h) {
				t.Errorf("%v: %04x aynı: %v", c.dtype, h, same)
			}
			if !same {
				different++
			}
		}
		if different != c.count {
			t.Errorf("%v: %d desen ayrı, beklenen %d", c.dtype, different, c.count)
		}
	}
}

package gpuprobe_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/gpu"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/testkit/trrun"
)

// sixteenBitSide bütün 16 bitlik desenleri tutan kare tensörün kenarı.
const sixteenBitSide = 256

// sixteenBitDTypes safetensors'ın iki 16 bitlik türü.
var sixteenBitDTypes = []string{"BF16", "F16"}

// sixteenBitFile values'u tek tensörlü ("w") kare bir safetensors dosyasına yazar: TR programı
// sinyal NaN'ı başka yoldan kuramaz.
func sixteenBitFile(t *testing.T, fpath, dtype string, values []uint16) {
	t.Helper()
	header := fmt.Sprintf(`{"w":{"dtype":%q,"shape":[%d,%d],"data_offsets":[0,%d]}}`, dtype, sixteenBitSide, sixteenBitSide, 2*len(values))
	file := binary.LittleEndian.AppendUint64(nil, uint64(len(header)))
	file = append(file, header...)
	if err := os.WriteFile(fpath, append(file, sixteenBitBytes(values)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sixteenBitBytes(values []uint16) []byte {
	var raw []byte
	for _, h := range values {
		raw = binary.LittleEndian.AppendUint16(raw, h)
	}
	return raw
}

// everySixteenBit bütün desenler, sırayla.
func everySixteenBit() []uint16 {
	bits := make([]uint16, sixteenBitSide*sixteenBitSide)
	for i := range bits {
		bits[i] = uint16(i)
	}
	return bits
}

// fileEnds dosyanın want'ın baytlarıyla bitip bitmediğine bakar; değişen desen sayısını söyler.
func fileEnds(t *testing.T, name, fpath string, want []uint16) {
	t.Helper()
	file, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	tail := file[len(file)-2*len(want):]
	changed := 0
	for i, h := range want {
		if binary.LittleEndian.Uint16(tail[2*i:]) != h {
			changed++
		}
	}
	if changed > 0 {
		t.Errorf("%s: %d/%d desen değişti", name, changed, len(want))
	}
}

// Kural (15.47a "bit bit aynıdır"): kartta yüklenen 16 bitlik tensör dosyaya bitleriyle yazılır:
// düz, devrik, dilim, .TR_to_device'la gidip gelmiş. Neden (gerçek kart, DENETİM istek 134): bf16'da
// 254 NaN deseninin 126'sı, fp16'da 2 046'nın 2 045'i değişiyordu.
func TestGPUSixteenBitFileKeepsItsBits(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	bits := everySixteenBit()
	turned := make([]uint16, len(bits))
	for r := range sixteenBitSide {
		for c := range sixteenBitSide {
			turned[c*sixteenBitSide+r] = bits[r*sixteenBitSide+c]
		}
	}
	for _, dtype := range sixteenBitDTypes {
		t.Run(dtype, func(t *testing.T) {
			dir := filepath.ToSlash(t.TempDir())
			source := dir + "/a.safetensors"
			sixteenBitFile(t, source, dtype, bits)
			out := trrun.MustExec(t, fmt.Sprintf(`
TR_default_device("gpu")
TR_var w = TR_load_tensors(%[1]q)["w"]
TR_print(w.TR_device)
TR_save_tensors(%[2]q + "/duz.safetensors", {"w": w})
TR_save_tensors(%[2]q + "/devrik.safetensors", {"w": w.TR_transpose()})
TR_save_tensors(%[2]q + "/dilim.safetensors", {"w": w[1..255]})
TR_save_tensors(%[2]q + "/tasima.safetensors", {"w": w.TR_to_device("cpu").TR_to_device("gpu")})
TR_default_device("cpu")
`, source, dir)).Out
			if !strings.Contains(out, "gpu") {
				t.Fatalf("tensör kartta değil: %q", out)
			}
			fileEnds(t, "düz", dir+"/duz.safetensors", bits)
			fileEnds(t, "devrik", dir+"/devrik.safetensors", turned)
			fileEnds(t, "dilim", dir+"/dilim.safetensors", bits[sixteenBitSide:255*sixteenBitSide])
			fileEnds(t, "taşıma", dir+"/tasima.safetensors", bits)
		})
	}
}

// Kural (20.17 "bit düzeyinde aynı parametreler"): kartta duran 16 bitlik parametrenin bütün
// desenleri kontrol noktasına yazılır ve sürdürmede karta geri gelir.
func TestGPUCheckpointKeepsSixteenBitParameters(t *testing.T) {
	gpu.RegisterSimulator() // gerçek GPU varsa o kullanılır
	defer tensor.SetDefaultDevice(tensor.CPU)
	bits := everySixteenBit()
	for _, dtype := range sixteenBitDTypes {
		t.Run(dtype, func(t *testing.T) {
			dir := filepath.ToSlash(t.TempDir())
			source, zero, back := dir+"/a.safetensors", dir+"/z.safetensors", dir+"/b.safetensors"
			sixteenBitFile(t, source, dtype, bits)
			sixteenBitFile(t, zero, dtype, make([]uint16, len(bits)))
			trrun.MustExec(t, fmt.Sprintf(`
TR_default_device("gpu")
TR_var model = {"w": TR_load_tensors(%q)["w"]}
TR_var kn = TR_Checkpoint(%q, {"model": model})
kn.TR_update(1)
kn.TR_save()
TR_default_device("cpu")
`, source, dir+"/run"))
			point, err := os.ReadFile(filepath.Join(dir, "run", "checkpoints", "step_0000000001", "tensors.trt"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(point, sixteenBitBytes(bits)) {
				t.Errorf("nokta dosyası bitleri tutmuyor")
			}
			out := trrun.MustExec(t, fmt.Sprintf(`
TR_default_device("gpu")
TR_var model = {"w": TR_load_tensors(%q)["w"]}
TR_var kn = TR_Checkpoint(%q, {"model": model})
TR_assert_equal(1, kn.TR_resume_step)
TR_print(model["w"].TR_device)
TR_save_tensors(%q, model)
TR_default_device("cpu")
`, zero, dir+"/run", back)).Out
			if !strings.Contains(out, "gpu") {
				t.Fatalf("parametre kartta değil: %q", out)
			}
			fileEnds(t, "sürdürülen", back, bits)
		})
	}
}

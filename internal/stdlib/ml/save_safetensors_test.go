package ml

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// safetensorsSample üç hassasiyetten, boş ve 0 boyutlu tensörlü karışık bir girdi listesidir;
// 16 bitlik değerler kendi türlerinde tam yazılır (gidiş dönüş bit bit karşılaştırılır).
func safetensorsSample() []fileEntry {
	return []fileEntry{
		{ad: "b16", shape: []int{2}, dtype: tensor.BFloat16, data: []float32{1.5, -2}},
		{ad: "w", shape: []int{2, 2}, dtype: tensor.Float32, data: []float32{0.1, float32(math.Copysign(0, -1)), float32(math.Inf(1)), 3}},
		{ad: "h", shape: []int{}, dtype: tensor.Float16, data: []float32{65504}},
		{ad: "boş", shape: []int{0, 3}, dtype: tensor.Float32, data: []float32{}},
		{ad: "<a&b>", shape: []int{1}, dtype: tensor.BFloat16, data: []float32{0.5}},
	}
}

// safetensorsTypedSample cinsli türlerin örnekleri (15.47a): ham bayt, küçük uçlu; uç değerler.
func safetensorsTypedSample() []fileEntry {
	ints := func(dt tensor.DType, vals ...int64) []byte {
		eb := dt.Bits() / 8
		raw := make([]byte, len(vals)*eb)
		for i, v := range vals {
			tensor.PutRawInt(dt, raw[i*eb:(i+1)*eb], v)
		}
		return raw
	}
	f64 := make([]byte, 16)
	tensor.PutRawFloat64(f64, 0.1)
	tensor.PutRawFloat64(f64[8:], math.Inf(-1))
	return []fileEntry{
		{ad: "i64", shape: []int{3}, dtype: tensor.Int64, raw: ints(tensor.Int64, 1, -2, 1<<53+1)},
		{ad: "i32", shape: []int{1}, dtype: tensor.Int32, raw: ints(tensor.Int32, math.MinInt32)},
		{ad: "i16", shape: []int{1}, dtype: tensor.Int16, raw: ints(tensor.Int16, 300)},
		{ad: "i8", shape: []int{1}, dtype: tensor.Int8, raw: ints(tensor.Int8, -128)},
		{ad: "u32", shape: []int{1}, dtype: tensor.UInt32, raw: ints(tensor.UInt32, math.MaxUint32)},
		{ad: "u16", shape: []int{1}, dtype: tensor.UInt16, raw: ints(tensor.UInt16, math.MaxUint16)},
		{ad: "u8", shape: []int{1}, dtype: tensor.UInt8, raw: ints(tensor.UInt8, 255)},
		{ad: "mantık", shape: []int{3}, dtype: tensor.Bool, raw: ints(tensor.Bool, 1, 0, 1)},
		{ad: "f64", shape: []int{2}, dtype: tensor.Float64, raw: f64},
	}
}

func encodeSafetensors(t *testing.T, inputs []fileEntry) []byte {
	t.Helper()
	if err := checkSafetensorsEntries("f", inputs); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := writeSafetensors(&b, inputs); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func entryBits(g fileEntry) string {
	if g.dtype.IsTyped() {
		return hex.EncodeToString(g.raw)
	}
	var b bytes.Buffer
	if err := writeValues(&b, g.dtype, g.data, make([]byte, 0, 64)); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b.Bytes())
}

// Kural (15.47a): tensörler hizaya göre yazılır, önce geniş türler, aynı genişlikte verilen
// sırayla; başlık 8 baytın katına boşlukla tamamlanır, __metadata__ {"format": "pt"} taşır;
// kendi okuyucuyla (15.47) gidiş dönüşte ad, tür, şekil ve veri bit bit aynıdır.
func TestSafetensorsWriteRoundTrip(t *testing.T) {
	in := safetensorsSample()
	file := encodeSafetensors(t, in)
	n := binary.LittleEndian.Uint64(file[:8])
	header := string(file[8 : 8+n])
	if n%8 != 0 || file[8] != '{' || !strings.HasPrefix(header, `{"__metadata__":{"format":"pt"},`) {
		t.Fatalf("başlık: %d bayt %q", n, header)
	}
	if !strings.Contains(header, `"<a&b>"`) {
		t.Fatalf("ad kaçışsız yazılmalı: %q", header)
	}
	out, err := decodeTensorFile(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"w", "boş", "b16", "h", "<a&b>"}
	if len(out) != len(wantOrder) {
		t.Fatalf("%d tensör, beklenen %d", len(out), len(wantOrder))
	}
	byName := map[string]fileEntry{}
	for _, g := range in {
		byName[g.ad] = g
	}
	for i, g := range out {
		want := byName[wantOrder[i]]
		if g.ad != want.ad || g.dtype != want.dtype || tensor.ShapeText(g.shape) != tensor.ShapeText(want.shape) || entryBits(g) != entryBits(want) {
			t.Errorf("%d. tensör %q %v %v %s; beklenen %q %v %v %s", i, g.ad, g.dtype, g.shape, entryBits(g), want.ad, want.dtype, want.shape, entryBits(want))
		}
	}
}

// Sınır: boş sözlük yalnız __metadata__'lı bir başlık yazar ve boş sözlük olarak okunur.
func TestSafetensorsWriteEmptyDict(t *testing.T) {
	file := encodeSafetensors(t, nil)
	if got := strings.TrimRight(string(file[8:]), " "); got != `{"__metadata__":{"format":"pt"}}` {
		t.Fatalf("başlık %q", got)
	}
	out, err := decodeTensorFile(bytes.NewReader(file), int64(len(file)))
	if err != nil || len(out) != 0 {
		t.Fatalf("boş sözlük: %v %v", out, err)
	}
}

// Kural (15.55, 15.47a): nicemlenmiş tensör safetensors'a yazılmaz (H6950); __metadata__ başlığın
// ayrılmış anahtarıdır, tensör adı olamaz (H6951). Nicemlenmiş tensör TR'den kurulamadığı için
// denetim türüyle doğrudan sınanır.
func TestSafetensorsWriteRejects(t *testing.T) {
	cases := []struct {
		in   fileEntry
		want messages.Code
	}{
		{fileEntry{ad: "q", shape: []int{1}, dtype: tensor.Q8}, messages.H6950},
		{fileEntry{ad: "q", shape: []int{2}, dtype: tensor.Q4}, messages.H6950},
		{fileEntry{ad: "__metadata__", shape: []int{1}, dtype: tensor.Float32, data: []float32{1}}, messages.H6951},
	}
	for _, c := range cases {
		if err := checkSafetensorsEntries("f", []fileEntry{c.in}); !messages.Has(err, c.want) {
			t.Errorf("%q %v: %v, beklenen %s", c.in.ad, c.in.dtype, err, c.want)
		}
	}
}

// Kural (15.46): yolun uzantısı biçimi seçer; .safetensors safetensors, öbürleri TR biçimi.
// Yazma atomiktir: hata veren kayıt eski dosyayı bozmaz.
func TestSaveTensorsChoosesFormatByExtension(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	out := mustRun(t, `TR_var d = {"b": TR_Tensor([1.5], TR_dtype: "bf16"), "w": TR_Tensor([[1.0, 2.0]])}
TR_save_tensors("`+dir+`/m.safetensors", d)
TR_save_tensors("`+dir+`/m.trt", d)
TR_print(TR_load_tensors("`+dir+`/m.safetensors").TR_keys(), TR_load_tensors("`+dir+`/m.trt").TR_keys())
TR_print(TR_error_code { TR_save_tensors("`+dir+`/m.safetensors", {"__metadata__": TR_Tensor([1.0])}) })
TR_print(TR_load_tensors("`+dir+`/m.safetensors")["b"].TR_dtype)`)
	want := "[\"w\", \"b\"] [\"b\", \"w\"]\nH6951\nbf16\n"
	if out != want {
		t.Fatalf("çıktı %q, beklenen %q", out, want)
	}
	data, err := os.ReadFile(filepath.Join(dir, "m.safetensors"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:8]) == fileMagic || data[8] != '{' {
		t.Fatalf("safetensors başlığı yok: % x", data[:9])
	}
}

// pythonSafetensorsScript dosyayı Python'un safetensors'ıyla okur, her tensörün adını, türünü,
// şeklini ve ham baytlarını yazar, sonra aynı sözlüğü ikinci yola yazar.
const pythonSafetensorsScript = `import sys
from safetensors import safe_open
from safetensors.torch import load_file, save_file
import torch
src, dst = sys.argv[1], sys.argv[2]
out = []
with safe_open(src, framework="pt") as f:
    out.append("meta " + str(f.metadata()))
d = load_file(src)
for k in sorted(d):
    t = d[k]
    out.append(" ".join([k, str(t.dtype), str(list(t.shape)), t.contiguous().reshape(-1).view(torch.uint8).numpy().tobytes().hex()]))
sys.stdout.buffer.write(("".join(line + chr(10) for line in out)).encode("utf-8"))
save_file(d, dst)
`

// Kural (15.47a Test satırı): yazılan dosya Python'un safetensors'ından geçer ve Python'un yazdığı
// dosya TR'nin okuyucusundan geçer; her yönde ad, tür, şekil ve veri bit bit aynıdır.
func TestSafetensorsPythonRoundTrip(t *testing.T) {
	py, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python yok")
	}
	if exec.Command(py, "-I", "-c", "import safetensors.torch").Run() != nil {
		t.Skip("python safetensors ya da torch yok")
	}
	dir := t.TempDir()
	src, dst, script := filepath.Join(dir, "tr.safetensors"), filepath.Join(dir, "py.safetensors"), filepath.Join(dir, "rt.py")
	in := append(safetensorsSample(), safetensorsTypedSample()...)
	if err := os.WriteFile(src, encodeSafetensors(t, in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte(pythonSafetensorsScript), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(py, "-I", script, src, dst)
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python: %v\n%s", err, got)
	}
	torchNames := map[tensor.DType]string{tensor.Float32: "torch.float32", tensor.BFloat16: "torch.bfloat16", tensor.Float16: "torch.float16",
		tensor.Float64: "torch.float64", tensor.Int64: "torch.int64", tensor.Int32: "torch.int32", tensor.Int16: "torch.int16", tensor.Int8: "torch.int8",
		tensor.UInt32: "torch.uint32", tensor.UInt16: "torch.uint16", tensor.UInt8: "torch.uint8", tensor.Bool: "torch.bool"}
	lines := []string{"meta {'format': 'pt'}"}
	names := make([]string, 0, len(in))
	for _, g := range in {
		names = append(names, g.ad)
	}
	sort.Strings(names) // Python'un sorted'ı: kod noktası sırası (UTF-8 bayt sırasıyla aynı)
	for _, name := range names {
		for _, g := range in {
			if g.ad == name {
				lines = append(lines, name+" "+torchNames[g.dtype]+" "+strings.ReplaceAll(fmt.Sprint(g.shape), " ", ", ")+" "+entryBits(g))
			}
		}
	}
	if want := strings.Join(lines, "\n") + "\n"; strings.ReplaceAll(string(got), "\r\n", "\n") != want {
		t.Fatalf("python okudu:\n%s\nbeklenen:\n%s", got, want)
	}
	back, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decodeTensorFile(bytes.NewReader(back), int64(len(back)))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) {
		t.Fatalf("python dosyasında %d tensör, beklenen %d", len(out), len(in))
	}
	for _, g := range out {
		for _, w := range in {
			if g.ad == w.ad && (g.dtype != w.dtype || tensor.ShapeText(g.shape) != tensor.ShapeText(w.shape) || entryBits(g) != entryBits(w)) {
				t.Errorf("python'un yazdığı %q: %v %v %s; beklenen %v %v %s", g.ad, g.dtype, g.shape, entryBits(g), w.dtype, w.shape, entryBits(w))
			}
		}
	}
}

// Kural (15.46, 15.55): TR biçimi kodu olmayan türü (nicemlenmiş Int8, Int4) float32 koduyla
// yazmaz, açık hata verir (H6959); hiçbir girdi yazılmadan durur. Nicemlenmiş tensör TR'den
// kurulamadığı için yazıcı türle doğrudan sınanır.
func TestTensorFileRejectsTypeWithoutCode(t *testing.T) {
	for _, dt := range []tensor.DType{tensor.Q8, tensor.Q4} {
		var b bytes.Buffer
		err := writeEntries(&b, []fileEntry{{ad: "q", shape: []int{2}, dtype: dt, data: []float32{1, 2}}})
		if !messages.Has(err, messages.H6959) {
			t.Fatalf("%v: %v, beklenen H6959", dt, err)
		}
	}
	for _, dt := range []tensor.DType{tensor.Float32, tensor.BFloat16, tensor.Float16, tensor.Float64, tensor.Int64, tensor.Int32,
		tensor.Int16, tensor.Int8, tensor.UInt32, tensor.UInt16, tensor.UInt8, tensor.Bool} {
		if code, ok := dtypeCode(dt); !ok {
			t.Fatalf("%v kodu yok", dt)
		} else if back, ok := dtypeFromCode(code); !ok || back != dt {
			t.Fatalf("%v kodu %d geri %v", dt, code, back)
		}
	}
}

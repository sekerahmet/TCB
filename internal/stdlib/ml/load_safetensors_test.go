package ml

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/onnx"
	"github.com/sekerahmet/TCB/internal/compute/onnx/onnxtest"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// safetensorsBytes belirtimdeki düzende bir dosya kurar: 8 baytlık little-endian başlık boyu,
// UTF-8 JSON başlık, veri tamponu (https://github.com/huggingface/safetensors#format).
func safetensorsBytes(header string, data []byte) []byte {
	b := binary.LittleEndian.AppendUint64(nil, uint64(len(header)))
	b = append(b, header...)
	return append(b, data...)
}

func f32Bytes(v ...float32) []byte {
	var b []byte
	for _, x := range v {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(x))
	}
	return b
}

func u16Bytes(v ...uint16) []byte {
	var b []byte
	for _, x := range v {
		b = binary.LittleEndian.AppendUint16(b, x)
	}
	return b
}

// Kural (15.47): tensör_yükle safetensors dosyasını da okur; biçim uzantıdan değil içerikten
// anlaşılır. F32, BF16 ve F16 kendi türünde gelir; 0 boyutlu ve boş tensör geçerlidir;
// __metadata__ (metin → metin) tensör değildir; tensörler dosyadaki veri sırasıyla gelir.
// Neden: Hugging Face ve PyTorch ağırlıkları bu biçimdedir (15.47).
func TestSafetensorsLoad(t *testing.T) {
	header := `{"__metadata__":{"format":"pt"},` +
		`"b":{"dtype":"BF16","shape":[2],"data_offsets":[16,20]},` +
		`"a":{"dtype":"F32","shape":[2,2],"data_offsets":[0,16]},` +
		`"c":{"dtype":"F16","shape":[],"data_offsets":[20,22]},` +
		`"boş":{"dtype":"F32","shape":[0,3],"data_offsets":[22,22]}}   `
	data := append(f32Bytes(1, 2, 3, 4), u16Bytes(0x3F80, 0xC000)...) // bf16: 1, -2
	data = append(data, u16Bytes(0x3C00)...)                          // fp16: 1
	fpath := filepath.ToSlash(filepath.Join(t.TempDir(), "model.bin"))
	if err := os.WriteFile(fpath, safetensorsBytes(header, data), 0o644); err != nil {
		t.Fatal(err)
	}
	out := mustRun(t, `TR_var y = TR_load_tensors("`+fpath+`")
TR_print(y.TR_keys())
TR_print(y["a"].TR_tolist, y["a"].TR_dtype, y["b"].TR_tolist, y["b"].TR_dtype, y["c"].TR_tolist, y["c"].TR_dtype, y["boş"].TR_shape)`)
	want := "[\"a\", \"b\", \"c\", \"boş\"]\n[[1.0, 2.0], [3.0, 4.0]] float32 [1.0, -2.0] bf16 1.0 fp16 [0, 3]\n"
	if out != want {
		t.Fatalf("safetensors:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Kural (15.47): safetensors dosyası güvenilmeyen girdidir; belirtimin ve TR'nin sınırlarına
// uymayan her dosya açık bir hatayla reddedilir (hiçbiri çökmez, dosya boyunu aşan ayırma
// yapılmaz). Kaynaklar: safetensors README "Format" ve safetensors/src/tensor.rs validate.
func TestSafetensorsRejects(t *testing.T) {
	one := f32Bytes(1)
	entry := func(s string) string { return `{"w":` + s + `}` }
	big := binary.LittleEndian.AppendUint64(nil, 200_000_000)
	for _, d := range []struct {
		name string
		file []byte
		want messages.Code
	}{
		{"kısa", []byte{1, 2, 3}, messages.H6610},
		{"başlık dosyadan uzun", append(binary.LittleEndian.AppendUint64(nil, 50), '{'), messages.H6639},
		{"başlık 100 MB'tan büyük", append(big, '{'), messages.H6639},
		{"{ ile başlamıyor", safetensorsBytes(` {"w":{}}`, nil), messages.H6610},
		{"geçersiz UTF-8", safetensorsBytes("{\"\xff\":1}", nil), messages.H6640},
		{"JSON değil", safetensorsBytes(`{"w":`, nil), messages.H6641},
		{"bilinmeyen alan", safetensorsBytes(entry(`{"dtype":"F32","shape":[1],"data_offsets":[0,4],"x":1}`), one), messages.H6654},
		{"yinelenen anahtar", safetensorsBytes(`{"w":{"dtype":"F32","shape":[1],"data_offsets":[0,4]},"w":{"dtype":"F32","shape":[1],"data_offsets":[4,8]}}`, append(one, one...)), messages.H6642},
		{"desteklenmeyen tür", safetensorsBytes(entry(`{"dtype":"U64","shape":[1],"data_offsets":[0,8]}`), make([]byte, 8)), messages.H6643},
		{"boşluk", safetensorsBytes(entry(`{"dtype":"F32","shape":[1],"data_offsets":[4,8]}`), append(one, one...)), messages.H6644},
		{"ters aralık", safetensorsBytes(entry(`{"dtype":"F32","shape":[1],"data_offsets":[4,0]}`), one), messages.H6644},
		{"örtüşme", safetensorsBytes(`{"a":{"dtype":"F32","shape":[2],"data_offsets":[0,8]},"b":{"dtype":"F32","shape":[1],"data_offsets":[4,8]}}`, append(one, one...)), messages.H6644},
		{"boy tutmuyor", safetensorsBytes(entry(`{"dtype":"F32","shape":[2],"data_offsets":[0,4]}`), one), messages.H6645},
		{"fazladan veri", safetensorsBytes(entry(`{"dtype":"F32","shape":[1],"data_offsets":[0,4]}`), append(one, 0)), messages.H6646},
		{"eksik veri", safetensorsBytes(entry(`{"dtype":"F32","shape":[2],"data_offsets":[0,8]}`), one), messages.H6646},
		{"metadata metin değil", safetensorsBytes(`{"__metadata__":{"a":1}}`, nil), messages.H6647},
		{"çok eksen", safetensorsBytes(entry(`{"dtype":"F32","shape":[`+strings.Repeat("1,", 32)+`1],"data_offsets":[0,4]}`), one), messages.H6648},
		{"dev eksen", safetensorsBytes(entry(`{"dtype":"F32","shape":[4294967296,4294967296],"data_offsets":[0,4]}`), one), messages.H6649},
		{"negatif eksen", safetensorsBytes(entry(`{"dtype":"F32","shape":[-1],"data_offsets":[0,4]}`), one), messages.H6655},
	} {
		_, err := decodeTensorFile(bytes.NewReader(d.file), int64(len(d.file)))
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.name, err, d.want)
		}
	}
}

// FuzzSafetensorsDecode: safetensors çözücüsü hiçbir baytta çökmez; kabul ettiği her dosyanın
// şekilleri eleman sınırında ve verisiyle tutarlıdır, tensörü kurulabilir. Derin arama:
//
//	go test ./internal/stdlib/ml -run '^$' -fuzz FuzzSafetensorsDecode -fuzztime 5m
func FuzzSafetensorsDecode(f *testing.F) {
	f.Add(safetensorsBytes(`{"w":{"dtype":"F32","shape":[2],"data_offsets":[0,8]}}`, f32Bytes(1, 2)))
	f.Add(safetensorsBytes(`{"__metadata__":{"k":"v"},"b":{"dtype":"BF16","shape":[1,1],"data_offsets":[0,2]}}`, u16Bytes(0x3F80)))
	f.Add(safetensorsBytes(`{}`, nil))
	f.Fuzz(func(t *testing.T, data []byte) {
		inputs, err := decodeTensorFile(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		for _, g := range inputs {
			n, limit := 1, 1
			for _, d := range g.shape {
				n *= d
				limit *= max(d, 1)
			}
			if limit > interp.MaxCollectionSize || n != g.count() {
				t.Fatalf("%q: şekil %v, %d eleman", g.ad, g.shape, g.count())
			}
			g.tensor()
		}
	})
}

// bigFloats blok eşiğinin üstünde bir float32 dizisidir.
func bigFloats() []float32 {
	v := make([]float32, 1<<19)
	for i := range v {
		v[i] = float32(i%1009) - 500.5
	}
	return v
}

// needBlocks bu platformda öbek dışı blok yoksa testi atlar.
func needBlocks(t *testing.T) {
	t.Helper()
	if tensor.NewHostBlock(1<<20) == nil {
		t.Skip("bu platformda öbek dışı blok yok")
	}
}

// Safetensors'ın iki yolu: Whisper'ın float32'si ve 16 bitlik dosyalar. Girdi bırakılınca tensör
// bloğu tutar: yükleyicinin dilimi değil, depo sahibidir.
func TestSafetensorsLargeTensorsGoToBlocks(t *testing.T) {
	needBlocks(t)
	values := bigFloats()
	bits := make([]uint16, 1<<20)
	for i := range bits {
		bits[i] = uint16(i) | 0x7F81 // sinyal NaN desenleri dosyanın bitleriyle gelmeli
	}
	f, h := f32Bytes(values...), u16Bytes(bits...)
	header := fmt.Sprintf(`{"f":{"dtype":"F32","shape":[%d],"data_offsets":[0,%d]},"h":{"dtype":"BF16","shape":[%d],"data_offsets":[%d,%d]}}`,
		len(values), len(f), len(bits), len(f), len(f)+len(h))
	file := safetensorsBytes(header, append(f, h...))
	entries, err := decodeGGUFBytes(file)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].block == nil || entries[1].block == nil {
		t.Fatal("tensörler blokta değil")
	}
	a, b := entries[0].tensor(), entries[1].tensor()
	entries = nil
	runtime.GC()
	runtime.GC()
	if !slices.Equal(a.Data(), values) {
		t.Error("float32 değerleri farklı")
	}
	if b.DType() != tensor.BFloat16 || !slices.Equal(b.Bits16s(), bits) {
		t.Errorf("16 bitlik desen farklı: %v", b.DType())
	}
}

// writeTemp dosyayı geçici klasöre yazar ve TR yolunu verir.
func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.ToSlash(filepath.Join(t.TempDir(), name))
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Kural (15.47): tensör_yükle ONNX modelinin ağırlıklarını adlarıyla, dosyadaki sırayla ve kendi
// türlerinde okur; veri raw_data'da ya da türün alanında (float_data, int64_data) olabilir; 0
// boyutlu ve boş tensör geçerlidir. Neden: Piper sesleri ONNX dosyasıdır.
func TestLoadOnnxInitializers(t *testing.T) {
	file := (&onnxtest.Builder{}).
		Initializer(onnxtest.Tensor{Name: "w", Dims: []int64{2, 2}, Type: onnx.Float, Raw: f32Bytes(1, 2, 3, 4)}).
		Initializer(onnxtest.Tensor{Name: "h", Dims: []int64{2}, Type: onnx.Float16, Raw: u16Bytes(0x3C00, 0xC000)}).
		Initializer(onnxtest.Tensor{Name: "i", Dims: []int64{3}, Type: onnx.Int64, Int64s: []int64{1, -2, 3}}).
		Initializer(onnxtest.Tensor{Name: "b", Dims: []int64{2}, Type: onnx.Bool, Raw: []byte{0, 2}}).
		Initializer(onnxtest.Tensor{Name: "s", Type: onnx.Float, Floats: []float32{1.5}}).
		Initializer(onnxtest.Tensor{Name: "e", Dims: []int64{0, 3}, Type: onnx.Float, Raw: []byte{}}).
		Bytes()
	out := mustRun(t, `TR_var y = TR_load_tensors("`+writeTemp(t, "ses.onnx", file)+`")
TR_print(y.TR_keys())
TR_print(y["w"].TR_tolist, y["w"].TR_dtype, y["h"].TR_tolist, y["h"].TR_dtype, y["i"].TR_tolist, y["i"].TR_dtype)
TR_print(y["b"].TR_tolist, y["b"].TR_dtype, y["s"].TR_tolist, y["e"].TR_shape)`)
	want := `["w", "h", "i", "b", "s", "e"]` + "\n[[1.0, 2.0], [3.0, 4.0]] float32 [1.0, -2.0] fp16 [1, -2, 3] int64\n" +
		"[false, true] bool 1.5 [0, 3]\n"
	if out != want {
		t.Fatalf("onnx:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Kural (15.47): adı `onnx::` ile başlayan ağırlık, onu girdi olarak kullanan ilk düğümün adıyla ve
// girdi sırasıyla adlanır; adsız düğümün ya da hiç kullanılmayan ağırlığın adı değişmez. Neden:
// Piper'ın akışlarındaki WaveNet evrişimleri adsız dışa verilir (`onnx::Conv_8168`).
func TestLoadOnnxAnonymousNamedByNode(t *testing.T) {
	one := func(name string) onnxtest.Tensor {
		return onnxtest.Tensor{Name: name, Dims: []int64{1}, Type: onnx.Float, Raw: f32Bytes(1)}
	}
	file := (&onnxtest.Builder{}).
		Node(onnxtest.Node{Name: "/flow/flows.0/enc/in_layers.0/Conv", Op: "Conv", Inputs: []string{"x", "onnx::Conv_8168", "onnx::Conv_8169"}}).
		Node(onnxtest.Node{Name: "/sonraki/Conv", Op: "Conv", Inputs: []string{"x", "onnx::Conv_8168"}}).
		Node(onnxtest.Node{Op: "Mul", Inputs: []string{"onnx::Mul_7"}}).
		Initializer(one("onnx::Conv_8168")).Initializer(one("onnx::Conv_8169")).
		Initializer(one("onnx::Mul_7")).Initializer(one("onnx::Kullanılmayan")).Initializer(one("sid")).
		Bytes()
	out := mustRun(t, `TR_print(TR_load_tensors("`+writeTemp(t, "ses.onnx", file)+`").TR_keys())`)
	want := `["/flow/flows.0/enc/in_layers.0/Conv:1", "/flow/flows.0/enc/in_layers.0/Conv:2", "onnx::Mul_7", "onnx::Kullanılmayan", "sid"]` + "\n"
	if out != want {
		t.Fatalf("adlar:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Kural (15.47): Constant düğümlerinin değerleri çıkış adlarıyla gelir: `value` tensörü ve
// `value_float`, `value_floats`, `value_int`, `value_ints`. Constant olmayan düğümün tensör
// özniteliği (ConstantOfShape'in `value`'su) ve başka alan adındaki Constant okunmaz.
func TestLoadOnnxConstantNodes(t *testing.T) {
	v := onnxtest.Tensor{Name: "iç ad", Dims: []int64{2}, Type: onnx.Float, Raw: f32Bytes(0.5, -1)}
	c := func(output string, attr []byte) onnxtest.Node {
		return onnxtest.Node{Name: "/" + output, Op: "Constant", Outputs: []string{output}, Attrs: [][]byte{attr}}
	}
	file := (&onnxtest.Builder{}).
		Constant("/Constant", "/Constant_output_0", v).
		Node(c("f", onnxtest.AttrFloat("value_float", 2.5))).
		Node(c("fs", onnxtest.AttrFloats("value_floats", 1, 2))).
		Node(c("i", onnxtest.AttrInt("value_int", -7))).
		Node(c("is", onnxtest.AttrInts("value_ints", 3, 4, 5))).
		Node(onnxtest.Node{Name: "/ConstantOfShape", Op: "ConstantOfShape", Outputs: []string{"cos"}, Attrs: [][]byte{onnxtest.AttrTensor("value", v)}}).
		Node(onnxtest.Node{Name: "/özel", Op: "Constant", Domain: "com.example", Outputs: []string{"özel"}, Attrs: [][]byte{onnxtest.AttrTensor("value", v)}}).
		Bytes()
	out := mustRun(t, `TR_var y = TR_load_tensors("`+writeTemp(t, "ses.onnx", file)+`")
TR_print(y.TR_keys())
TR_print(y["/Constant_output_0"].TR_tolist, y["f"].TR_tolist, y["fs"].TR_tolist, y["i"].TR_tolist, y["i"].TR_dtype, y["is"].TR_tolist)`)
	want := `["/Constant_output_0", "f", "fs", "i", "is"]` + "\n[0.5, -1.0] 2.5 [1.0, 2.0] -7 int64 [3, 4, 5]\n"
	if out != want {
		t.Fatalf("Constant:\n%q\nbeklenen:\n%q", out, want)
	}
}

// Kural (15.47): ONNX dosyası güvenilmeyen girdidir; her sınır ve her bozukluk kodlu bir hatadır
// (H6742–H6748, eleman sınırı H6649, tanınmayan biçim H6610), hiçbiri çökmez.
func TestLoadOnnxRejectsBroken(t *testing.T) {
	head := onnxtest.VarintField(nil, 1, 8) // ir_version
	graph := func(inits ...onnxtest.Tensor) []byte {
		b := &onnxtest.Builder{}
		for _, x := range inits {
			b.Initializer(x)
		}
		return b.Bytes()
	}
	f := func(name string, dims ...int64) onnxtest.Tensor {
		return onnxtest.Tensor{Name: name, Dims: dims, Type: onnx.Float, Raw: f32Bytes(1)}
	}
	many := make([]onnxtest.Tensor, onnx.MaxTensors+1)
	for i := range many {
		many[i] = f(fmt.Sprint(i), 1)
	}
	dims33 := make([]int64, 33)
	for i := range dims33 {
		dims33[i] = 1
	}
	overlong := append(onnxtest.Tag(slices.Clone(head), 1, onnxtest.WireVarint), bytes.Repeat([]byte{0xff}, 10)...)
	for _, d := range []struct {
		name string
		file []byte
		want messages.Code
	}{
		{"boy okunurken dosya bitti", onnxtest.Tag(slices.Clone(head), 7, onnxtest.WireLen), messages.H6742},
		{"grup tel türü", append(slices.Clone(head), 7<<3|3), messages.H6742},
		{"11 baytlık varint", append(overlong, 0x01), messages.H6742},
		{"çizge varint olarak", onnxtest.VarintField(slices.Clone(head), 7, 1), messages.H6742},
		{"boy kalanı aşıyor", append(onnxtest.Tag(slices.Clone(head), 7, onnxtest.WireLen), 100, 1, 2), messages.H6743},
		{"çok tensör", graph(many...), messages.H6744},
		{"çok eksen", graph(f("w", dims33...)), messages.H6745},
		{"metin türü", graph(onnxtest.Tensor{Name: "s", Type: 8}), messages.H6746},
		{"uint16 türü", graph(onnxtest.Tensor{Name: "u", Type: 4, Raw: []byte{1, 0}}), messages.H6746},
		{"dış veri", graph(onnxtest.Tensor{Name: "w", Dims: []int64{1}, Type: onnx.Float, External: true}), messages.H6747},
		{"veri boyu tutmuyor", graph(f("w", 2)), messages.H6747},
		{"türün alanı eksik", graph(onnxtest.Tensor{Name: "w", Dims: []int64{2}, Type: onnx.Float, Floats: []float32{1}}), messages.H6747},
		{"yinelenen ad", graph(f("w", 1), f("w", 1)), messages.H6748},
		{"dev eksen", graph(f("w", 1<<32, 1<<32)), messages.H6649},
		{"eksi eksen", graph(f("w", -1)), messages.H6649},
		{"ilk bayt 0x09", []byte{0x09, 1}, messages.H6610},
	} {
		_, err := decodeTensorFile(bytes.NewReader(d.file), int64(len(d.file)))
		if err == nil || !messages.Has(err, d.want) {
			t.Errorf("%s: %v (içermeli: %s)", d.name, err, d.want)
			continue
		}
		t.Logf("%s: %v", d.name, err) // metin katalogdan kurulur: şablonun parametreleri tutar
	}
	// Adsız ağırlığın yeni adı var olan bir adla çakışırsa da yinelenen addır.
	file := (&onnxtest.Builder{}).Node(onnxtest.Node{Name: "n", Op: "Conv", Inputs: []string{"x", "onnx::W"}}).
		Initializer(f("onnx::W", 1)).Initializer(f("n:1", 1)).Bytes()
	if _, err := decodeTensorFile(bytes.NewReader(file), int64(len(file))); !messages.Has(err, messages.H6748) {
		t.Errorf("adsızın yeni adı çakışıyor: %v (içermeli: H6748)", err)
	}
}

// Kural (15.47): safetensors'un başlık boyu küçük uçludur ve ilk baytı 0x08 olabilir (264 =
// 0x108); 9. baytı { olan dosya ONNX değil safetensors olarak okunur: tanımada ONNX sondadır.
func TestSafetensorsWithLeadingEightStaysSafetensors(t *testing.T) {
	header := `{"w":{"dtype":"F32","shape":[1],"data_offsets":[0,4]}}`
	header += strings.Repeat(" ", 264-len(header))
	file := safetensorsBytes(header, f32Bytes(3))
	if file[0] != 0x08 || file[8] != '{' {
		t.Fatalf("kurulum: ilk bayt %#x, 9. bayt %q", file[0], file[8])
	}
	entries, err := decodeTensorFile(bytes.NewReader(file), int64(len(file)))
	if err != nil || len(entries) != 1 || entries[0].ad != "w" || entries[0].data[0] != 3 {
		t.Fatalf("safetensors okunmadı: %v %v", entries, err)
	}
}

// ONNX'in büyük ağırlığı doğrudan öbek dışı bloğa okunur; bırakılınca tensör bloğu tutar.
func TestLoadOnnxLargeTensorsGoToBlocks(t *testing.T) {
	needBlocks(t)
	values := bigFloats()
	file := (&onnxtest.Builder{}).
		Initializer(onnxtest.Tensor{Name: "w", Dims: []int64{int64(len(values))}, Type: onnx.Float, Raw: f32Bytes(values...)}).Bytes()
	entries, err := decodeTensorFile(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].block == nil {
		t.Fatal("tensör blokta değil")
	}
	a := entries[0].tensor()
	entries = nil
	runtime.GC()
	if !slices.Equal(a.Data(), values) {
		t.Error("float32 değerleri farklı")
	}
}

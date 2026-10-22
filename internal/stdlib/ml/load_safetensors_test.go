package ml

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
// Neden: Hugging Face ve PyTorch ağırlıkları bu biçimdedir (yol haritası Evre 5, madde 2).
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
	out := mustRun(t, `değişken y = tensör_yükle("`+fpath+`")
yazdır(y.anahtarlar())
yazdır(y["a"].değer, y["a"].tür, y["b"].değer, y["b"].tür, y["c"].değer, y["c"].tür, y["boş"].şekil)`)
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
		{"desteklenmeyen tür", safetensorsBytes(entry(`{"dtype":"I64","shape":[1],"data_offsets":[0,8]}`), make([]byte, 8)), messages.H6643},
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
			if limit > interp.MaxCollectionSize || n != len(g.data) {
				t.Fatalf("%q: şekil %v, %d eleman", g.ad, g.shape, len(g.data))
			}
			if g.dtype == tensor.Float32 {
				tensor.New(g.data, g.shape...)
			} else {
				tensor.NewOf(g.dtype, g.data, g.shape...)
			}
		}
	})
}

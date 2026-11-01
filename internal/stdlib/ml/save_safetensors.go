package ml

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// safetensors biçiminin yazıcısı (dil tanımı 15.47a). Düzen okuyucudaki gibidir
// (load_safetensors.go): 8 baytlık başlık boyu, boşlukla 8 baytın katına tamamlanmış JSON başlık,
// veri tamponu. Tensörler hizaya göre yazılır: önce geniş türler, aynı genişlikte sözlüğün
// sırasıyla (Python'un safetensors'ı aynı genişlikte ada göre sıralar; TR sıralamaz). Başlık
// __metadata__ ile başlar ({"format": "pt"}), sonra tensörler veri sırasıyla gelir. CRC yoktur.

// safetensorsSuffix bu uzantıyla biten yol safetensors yazar (15.46); öbürleri TR biçimini.
const safetensorsSuffix = ".safetensors"

// safetensorsMetadataKey başlığın tensör olmayan anahtarıdır; tensör adı olamaz.
const safetensorsMetadataKey = "__metadata__"

// safetensorsNames çekirdek türlerinin safetensors adları; safetensorsDTypes'ın tersi.
var safetensorsNames = map[tensor.DType]string{tensor.Float32: "F32", tensor.BFloat16: "BF16", tensor.Float16: "F16"}

func isSafetensorsPath(fpath string) bool { return strings.HasSuffix(fpath, safetensorsSuffix) }

// checkSafetensorsEntries yazmadan önce girdilerin safetensors'ta yazılabildiğini denetler:
// nicemlenmiş tensörün safetensors'ta türü yoktur (15.55), __metadata__ başlığın ayrılmış
// anahtarıdır.
func checkSafetensorsEntries(fn any, inputs []fileEntry) error {
	for _, g := range inputs {
		if g.ad == safetensorsMetadataKey {
			return messages.New(messages.H6951, fn, g.ad)
		}
		if _, ok := safetensorsNames[g.dtype]; !ok {
			return messages.New(messages.H6950, fn, g.ad, g.dtype.String())
		}
		if !utf8.ValidString(g.ad) {
			return messages.Internal("safetensors: tensor name is not valid UTF-8")
		}
	}
	return nil
}

// writeSafetensorsFile girdileri yol'a safetensors olarak atomik yazar (18.13).
func writeSafetensorsFile(fpath string, inputs []fileEntry) error {
	return interp.WriteAtomic(fpath, func(f io.Writer) error { return writeSafetensors(f, inputs) })
}

// safetensorsOrder girdilerin yazılma sırasıdır: geniş türler önce, aynı genişlikte verilen sıra.
func safetensorsOrder(inputs []fileEntry) []fileEntry {
	out := slices.Clone(inputs)
	slices.SortStableFunc(out, func(a, b fileEntry) int { return b.dtype.Bits() - a.dtype.Bits() })
	return out
}

// writeSafetensors girdileri f'ye safetensors olarak yazar; girdiler checkSafetensorsEntries'ten
// geçmiş olmalıdır.
func writeSafetensors(f io.Writer, inputs []fileEntry) error {
	ordered := safetensorsOrder(inputs)
	header, err := safetensorsHeaderBytes(ordered)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<16)
	var nb [8]byte
	binary.LittleEndian.PutUint64(nb[:], uint64(len(header)))
	bw.Write(nb[:])
	bw.Write(header)
	buffer := make([]byte, 0, 1<<16)
	for _, g := range ordered {
		if err := writeValues(bw, g.dtype, g.data, buffer); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// safetensorsHeaderBytes başlığı veri sırasıyla kurar ve boşlukla 8 baytın katına tamamlar.
func safetensorsHeaderBytes(ordered []fileEntry) ([]byte, error) {
	var h bytes.Buffer
	h.WriteString(`{"` + safetensorsMetadataKey + `":{"format":"pt"}`)
	offset := uint64(0)
	for _, g := range ordered {
		name, err := jsonText(g.ad)
		if err != nil {
			return nil, err
		}
		size := uint64(len(g.data)) * uint64(g.dtype.Bits()/8)
		h.WriteByte(',')
		h.Write(name)
		h.WriteString(`:{"dtype":"` + safetensorsNames[g.dtype] + `","shape":[`)
		for i, d := range g.shape {
			if i > 0 {
				h.WriteByte(',')
			}
			h.WriteString(strconv.Itoa(d))
		}
		h.WriteString(`],"data_offsets":[` + strconv.FormatUint(offset, 10) + "," + strconv.FormatUint(offset+size, 10) + "]}")
		offset += size
	}
	h.WriteByte('}')
	for h.Len()%8 != 0 {
		h.WriteByte(' ')
	}
	return h.Bytes(), nil
}

// jsonText metni JSON dizgisi olarak yazar; HTML kaçışı yapılmaz (ad olduğu gibi kalır).
func jsonText(s string) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// writeValues değerleri dtype türünde küçük sonlu yazar (16 bitlik türlerde bit deseni);
// buffer yazma tamponudur.
func writeValues(w io.Writer, dtype tensor.DType, data []float32, buffer []byte) error {
	on6 := dtype == tensor.BFloat16 || dtype == tensor.Float16
	for i := 0; i < len(data); {
		t := buffer[:0]
		for ; i < len(data) && len(t) < cap(t); i++ {
			if on6 {
				t = binary.LittleEndian.AppendUint16(t, tensor.Bits16(dtype, data[i]))
			} else {
				t = binary.LittleEndian.AppendUint32(t, math.Float32bits(data[i]))
			}
		}
		if _, err := w.Write(t); err != nil {
			return err
		}
	}
	return nil
}

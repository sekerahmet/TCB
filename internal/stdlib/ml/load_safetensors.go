package ml

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// safetensors biçiminin okuyucusu (dil tanımı 15.47). Düzen
// (https://github.com/huggingface/safetensors#format): 8 baytlık little-endian başlık boyu N,
// N baytlık UTF-8 JSON başlık ({ ile başlar, sonu boşlukla doldurulabilir), veri tamponu.
// Başlık ad → {dtype, shape, data_offsets} ve isteğe bağlı __metadata__ (metin → metin) tutar;
// data_offsets tamponun başına göredir, [BEGIN, END). Dosya güvenilmeyen girdidir; denetimler
// safetensors/src/tensor.rs'teki validate ve read_metadata ile aynıdır (başlık sınırı 100 MB,
// ardışık boşluksuz örtüşmesiz aralıklar, END − BEGIN = eleman × tür boyu, tamponun tamamı
// kullanılır), ek olarak yinelenen anahtar reddedilir (README: "not all parsers may respect
// this") ve TR'nin eleman, eksen ve ad sınırları uygulanır. Bütün denetimler veri okunmadan
// yapılır; dosya boyunu aşan ayırma olmaz.

// maxSafetensorsHeader başlığın en büyük boyudur (tensor.rs MAX_HEADER_SIZE).
const maxSafetensorsHeader = 100_000_000

// safetensorsDTypes desteklenen veri türleri ve eleman boyları.
var safetensorsDTypes = map[string]tensor.DType{"F32": tensor.Float32, "BF16": tensor.BFloat16, "F16": tensor.Float16}

// decodeTensorFile dosyanın biçimini uzantıdan değil içerikten tanır: TRTENSOR ile başlıyorsa
// TR biçimi, 9. baytı { ise (başlık boyundan sonra JSON başlık; belirtim: "MUST begin with a {")
// safetensors; ikisi de değilse ikisini de anan bir hata.
func decodeTensorFile(f io.Reader, size int64) ([]fileEntry, error) {
	br := bufio.NewReaderSize(f, 1<<16)
	p, _ := br.Peek(9)
	switch {
	case len(p) >= len(fileMagic) && string(p[:len(fileMagic)]) == fileMagic:
		return decodeFile(br, size)
	case len(p) == 9 && p[8] == '{':
		return decodeSafetensors(br, size)
	}
	return nil, messages.New(messages.H6610, fileMagic, codes.DisplayName(codes.T700))
}

// safetensorsEntry başlıktaki bir tensörün kaydıdır.
type safetensorsEntry struct {
	Dtype       *string  `json:"dtype"`
	Shape       []uint64 `json:"shape"`
	DataOffsets []uint64 `json:"data_offsets"`

	name string
}

// decodeSafetensors boyutu size bayt olan bir safetensors dosyasını r'den çözer; tensörler
// veri tamponundaki sırayla döner.
func decodeSafetensors(r io.Reader, size int64) ([]fileEntry, error) {
	var nb [8]byte
	if _, err := io.ReadFull(r, nb[:]); err != nil {
		return nil, messages.New(messages.H6610, fileMagic, codes.DisplayName(codes.T700))
	}
	n := binary.LittleEndian.Uint64(nb[:])
	if n > maxSafetensorsHeader || n > uint64(size-8) {
		return nil, messages.New(messages.H6639, n, size-8, maxSafetensorsHeader)
	}
	header := make([]byte, n)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, messages.New(messages.H6639, n, size-8, maxSafetensorsHeader)
	}
	if !utf8.Valid(header) {
		return nil, messages.New(messages.H6640)
	}
	entries, err := safetensorsHeader(header)
	if err != nil {
		return nil, err
	}
	buffer := uint64(size-8) - n
	if err := checkSafetensorsLayout(entries, buffer); err != nil {
		return nil, err
	}
	out := make([]fileEntry, 0, len(entries))
	for _, e := range entries {
		dt := safetensorsDTypes[*e.Dtype]
		shape := make([]int, len(e.Shape))
		count := 1
		for i, d := range e.Shape {
			shape[i] = int(d)
			count *= int(d)
		}
		data, err := readValues(r, dt, count)
		if err != nil {
			return nil, messages.New(messages.H6653, e.name, err)
		}
		out = append(out, fileEntry{ad: e.name, shape: shape, dtype: dt, data: data})
	}
	return out, nil
}

// safetensorsHeader başlığı sırayla çözer: yinelenen anahtar, bilinmeyen alan, eksik alan ve
// başlıktan sonra gelen fazladan JSON hatadır; __metadata__ yalnızca metin değerler tutar.
func safetensorsHeader(header []byte) ([]*safetensorsEntry, error) {
	d := json.NewDecoder(bytes.NewReader(header))
	if _, err := d.Token(); err != nil { // '{' (header[0] denetlendi)
		return nil, messages.New(messages.H6641, err)
	}
	seen := map[string]bool{}
	var entries []*safetensorsEntry
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			return nil, messages.New(messages.H6641, err)
		}
		name, _ := tok.(string)
		if seen[name] {
			return nil, messages.New(messages.H6642, name)
		}
		seen[name] = true
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, messages.New(messages.H6641, err)
		}
		if name == "__metadata__" {
			if err := checkSafetensorsMetadata(raw); err != nil {
				return nil, err
			}
			continue
		}
		e, err := safetensorsEntryOf(name, raw)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if _, err := d.Token(); err != nil { // '}'
		return nil, messages.New(messages.H6641, err)
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, messages.New(messages.H6651)
	}
	return entries, nil
}

// checkSafetensorsMetadata __metadata__'nın metin → metin bir nesne olduğunu denetler.
func checkSafetensorsMetadata(raw json.RawMessage) error {
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(raw, &meta); err != nil {
		return messages.New(messages.H6641, err)
	}
	for k, v := range meta {
		var s string
		if json.Unmarshal(v, &s) != nil {
			return messages.New(messages.H6647, k)
		}
	}
	return nil
}

// safetensorsEntryOf bir tensör kaydını katı çözer ve tür, eksen, eleman ve ad sınırlarını
// denetler.
func safetensorsEntryOf(name string, raw json.RawMessage) (*safetensorsEntry, error) {
	if len(name) > maxNameLen {
		return nil, messages.New(messages.H6650, len(name), maxNameLen)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	e := &safetensorsEntry{name: name}
	if err := d.Decode(e); err != nil {
		return nil, messages.New(messages.H6641, err)
	}
	if e.Dtype == nil || e.Shape == nil || len(e.DataOffsets) != 2 {
		return nil, messages.New(messages.H6652, name)
	}
	if _, ok := safetensorsDTypes[*e.Dtype]; !ok {
		return nil, messages.New(messages.H6643, name, *e.Dtype)
	}
	if len(e.Shape) > maxDims {
		return nil, messages.New(messages.H6648, name, len(e.Shape), maxDims)
	}
	limit := uint64(1)
	for _, d := range e.Shape {
		if d > uint64(interp.MaxCollectionSize) || (d > 0 && limit > uint64(interp.MaxCollectionSize)/d) {
			return nil, messages.New(messages.H6649, name, d, interp.MaxCollectionSize)
		}
		limit *= max(d, 1)
	}
	return e, nil
}

// checkSafetensorsLayout aralıkları veri sırasına dizer ve denetler: ilki 0'da başlar, her biri
// öncekinin bittiği yerde başlar, boyu şeklin istediği kadardır ve son aralık tamponun sonunda
// biter (tensor.rs validate: "if s != start || e < s", "if e - s != size",
// MetadataIncompleteBuffer).
func checkSafetensorsLayout(entries []*safetensorsEntry, buffer uint64) error {
	slices.SortStableFunc(entries, func(a, b *safetensorsEntry) int {
		return compareU64(a.DataOffsets[0], b.DataOffsets[0])
	})
	expected := uint64(0)
	for _, e := range entries {
		begin, end := e.DataOffsets[0], e.DataOffsets[1]
		if begin != expected || end < begin {
			return messages.New(messages.H6644, e.name, begin, end, expected)
		}
		count := uint64(1)
		shape := make([]int, len(e.Shape))
		for i, d := range e.Shape {
			count *= d
			shape[i] = int(d)
		}
		want := count * uint64(safetensorsDTypes[*e.Dtype].Bits()/8)
		if end-begin != want {
			return messages.New(messages.H6645, e.name, shapeText(shape), want, end-begin)
		}
		expected = end
	}
	if expected != buffer {
		return messages.New(messages.H6646, buffer, expected)
	}
	return nil
}

func compareU64(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// readValues n elemanı dt türünde okur ve float32'ye kayıpsız açar.
func readValues(r io.Reader, dt tensor.DType, n int) ([]float32, error) {
	data := make([]float32, n)
	eb := dt.Bits() / 8
	buffer := make([]byte, 1<<16)
	for i := 0; i < n; {
		part := min(len(buffer)/eb, n-i)
		if _, err := io.ReadFull(r, buffer[:part*eb]); err != nil {
			return nil, err
		}
		for j := 0; j < part; j++ {
			if eb == 2 {
				data[i+j] = tensor.FromBits16(dt, binary.LittleEndian.Uint16(buffer[j*2:]))
			} else {
				data[i+j] = math.Float32frombits(binary.LittleEndian.Uint32(buffer[j*4:]))
			}
		}
		i += part
	}
	return data, nil
}

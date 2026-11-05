// Package gguf GGUF sürüm 3 dosyasının başlığını çözer: üstveri ve tensör kayıtları (dil tanımı
// 15.55, 16.1). Dosya güvenilmeyen girdidir: bütün sayılar, boylar, hizalama, kaymalar ve
// boyutlar taşma denetimli hesaplanır, bildirilen veri dosyada olmalıdır, denetimler ayırmadan
// önce yapılır. Tensör verisini okumak çağıranın işidir: ml ile tokenizer bu paketi ortak kullanır.
//
// Layer: compute core (internal/compute); yalnızca ileti kataloğunu içe aktarır.
package gguf

import (
	"io"
	"math"
	"math/bits"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

const (
	// Magic dosyanın ilk dört baytıdır.
	Magic = "GGUF"
	// version okunan tek sürümdür (gguf.h GGUF_VERSION).
	version = 3
	// defaultAlignment general.alignment yoksa geçerlidir (gguf.h GGUF_DEFAULT_ALIGNMENT).
	defaultAlignment = 32
	// alignmentKey hizalamanın üstveri anahtarıdır.
	alignmentKey = "general.alignment"
	// maxTensorName tensör adının en büyük bayt boyudur (ggml.h GGML_MAX_NAME 64, sonu NUL).
	maxTensorName = 63
	// maxDims tensörün en çok eksen sayısıdır (ggml.h GGML_MAX_DIMS).
	maxDims = 4
	// minKV ve minTensor bir kaydın dosyada tutabileceği en az bayttır: sayıların dev
	// ayırmaya yol açmasını önler. Anahtar: boy 8, tür 4, değer 1; tensör: boy 8, ad 1,
	// eksen sayısı 4, tür 4, kayma 8.
	minKV     = 13
	minTensor = 25
)

// Value bir üstveri değeridir. Data tek değerde uint8, int8, uint16, int16, uint32, int32,
// float32, bool, string, uint64, int64 ya da float64'tür; dizide (Type == Array) Elem'in
// dilimidir ([]string, []int32 ...).
type Value struct {
	Type ValueType
	Elem ValueType
	Data any
}

// Uint tek tam sayı değeridir; tam sayı olmayan ya da eksi değerde ok yanlıştır.
func (v Value) Uint() (n uint64, ok bool) {
	switch x := v.Data.(type) {
	case uint8:
		return uint64(x), true
	case uint16:
		return uint64(x), true
	case uint32:
		return uint64(x), true
	case uint64:
		return x, true
	case int8:
		return uint64(x), x >= 0
	case int16:
		return uint64(x), x >= 0
	case int32:
		return uint64(x), x >= 0
	case int64:
		return uint64(x), x >= 0
	}
	return 0, false
}

// Tensor dosyadaki bir tensörün kaydıdır.
type Tensor struct {
	Name   string
	Dims   []uint64 // dosyadaki sıra: Dims[0] en iç eksendir (son eksen)
	Type   TensorType
	Offset uint64 // veri bölümünün başına göre
	Size   uint64 // veri baytı
}

// Shape TR şeklidir: dosyadakinin tersi, dosyanın ilk boyutu son eksendir (15.55).
func (t Tensor) Shape() []uint64 {
	shape := make([]uint64, len(t.Dims))
	for i, d := range t.Dims {
		shape[len(t.Dims)-1-i] = d
	}
	return shape
}

// File çözülmüş başlıktır.
type File struct {
	Keys      []string // üstveri anahtarları, dosyadaki sırayla
	Values    map[string]Value
	Tensors   []Tensor // dosyadaki sırayla; kaymaları artandır
	HeaderEnd int64    // Decode'un okuduğu son bayttan sonrası
	DataStart int64    // veri bölümünün dosyadaki yeri
}

// Decode size bayt uzunluğundaki dosyanın başlığını r'den çözer ve denetler; r tensör
// kayıtlarının sonunda (HeaderEnd) kalır, küçük okumalar yaptığı için tamponlu olmalıdır. Akış
// size'dan uzun olsa da çözücü size'ı aşmaz: kalan bayt eksiye inseydi boy kapıları açılırdı
// (boyu 0 görünen dosya, okunurken büyüyen dosya).
func Decode(r io.Reader, size int64) (*File, error) {
	d := &decoder{r: io.LimitReader(r, size), remaining: size}
	tensors, keys, err := d.header()
	if err != nil {
		return nil, err
	}
	f := &File{Values: map[string]Value{}}
	for range keys {
		if err := d.keyValue(f); err != nil {
			return nil, err
		}
	}
	alignment, err := f.alignment()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for range tensors {
		t, err := d.tensor(seen)
		if err != nil {
			return nil, err
		}
		f.Tensors = append(f.Tensors, t)
	}
	f.HeaderEnd = d.offset
	if err := f.layout(alignment, uint64(size)); err != nil {
		return nil, err
	}
	return f, nil
}

// messageLimit iletiye yazılan ad ve anahtarın en büyük bayt boyudur: dosya 32 MB'lık ad
// bildirebilir, ileti onu taşımaz.
const messageLimit = 64

// Clip adı iletiye sığacak boya kırpar.
func Clip(s string) string {
	if len(s) <= messageLimit {
		return s
	}
	return strings.ToValidUTF8(s[:messageLimit], "") + "…"
}

// header sihirli baytları, sürümü ve sayıları denetler: tensör ve anahtar sayısını verir.
func (d *decoder) header() (tensors, keys uint64, err error) {
	var magic [4]byte
	if !d.bytes("header", magic[:]) {
		return 0, 0, d.err
	}
	if string(magic[:]) != Magic {
		return 0, 0, messages.New(messages.H6676, Magic)
	}
	v, ok := d.u32("header")
	if !ok {
		return 0, 0, d.err
	}
	if v != version {
		// Ters uçlu dosyada sürüm küçük uçlu okununca alt 16 bit sıfır olur (gguf.cpp:506).
		if v != 0 && v&0xFFFF == 0 {
			return 0, 0, messages.New(messages.H6656, bits.ReverseBytes32(v), "big")
		}
		return 0, 0, messages.New(messages.H6656, v, "little")
	}
	tensors, ok = d.u64("header")
	if !ok {
		return 0, 0, d.err
	}
	keys, ok = d.u64("header")
	if !ok {
		return 0, 0, d.err
	}
	if !fits(keys, minKV, tensors, minTensor, uint64(d.remaining)) {
		return 0, 0, messages.New(messages.H6657, keys, tensors, d.remaining)
	}
	return tensors, keys, nil
}

// fits a·x + b·y taşmadan sınıra sığıyor mu?
func fits(a, x, b, y, limit uint64) bool {
	hi1, lo1 := bits.Mul64(a, x)
	hi2, lo2 := bits.Mul64(b, y)
	sum, carry := bits.Add64(lo1, lo2, 0)
	return hi1 == 0 && hi2 == 0 && carry == 0 && sum <= limit
}

// keyValue bir anahtarı ve değerini okur; yinelenen anahtar hatadır.
func (d *decoder) keyValue(f *File) error {
	key, ok := d.text("key")
	if !ok {
		return d.err
	}
	if _, dup := f.Values[key]; dup {
		return messages.New(messages.H6664, "key", Clip(key))
	}
	v, err := d.value(key)
	if err != nil {
		return err
	}
	f.Keys = append(f.Keys, key)
	f.Values[key] = v
	return nil
}

// alignment hizalamayı verir: anahtar yoksa varsayılan, varsa ikinin kuvveti bir uint32.
func (f *File) alignment() (uint64, error) {
	v, ok := f.Values[alignmentKey]
	if !ok {
		return defaultAlignment, nil
	}
	a, ok := v.Data.(uint32)
	if !ok {
		return 0, messages.New(messages.H6665, v.Type.String())
	}
	if a == 0 || a&(a-1) != 0 {
		return 0, messages.New(messages.H6665, a)
	}
	return uint64(a), nil
}

// tensor bir tensör kaydını okur ve denetler; kayma layout'ta denetlenir.
func (d *decoder) tensor(seen map[string]bool) (Tensor, error) {
	name, ok := d.text("tensor")
	if !ok {
		return Tensor{}, d.err
	}
	if len(name) == 0 || len(name) > maxTensorName {
		return Tensor{}, messages.New(messages.H6666, Clip(name), len(name))
	}
	if seen[name] {
		return Tensor{}, messages.New(messages.H6664, "tensor", name)
	}
	seen[name] = true
	count, ok := d.u32("tensor")
	if !ok {
		return Tensor{}, d.err
	}
	if count > maxDims {
		return Tensor{}, messages.New(messages.H6667, name, count)
	}
	t := Tensor{Name: name, Dims: make([]uint64, count)}
	for i := range t.Dims {
		if t.Dims[i], ok = d.u64("tensor"); !ok {
			return Tensor{}, d.err
		}
		if t.Dims[i] > math.MaxInt64 {
			return Tensor{}, messages.New(messages.H6668, name, int64(t.Dims[i]))
		}
	}
	kind, ok := d.u32("tensor")
	if !ok {
		return Tensor{}, d.err
	}
	t.Type = TensorType(kind)
	if t.Offset, ok = d.u64("tensor"); !ok {
		return Tensor{}, d.err
	}
	return t, t.measure()
}

// measure türü ve şekli denetler, bayt boyunu hesaplar. Sıfır eksenler taşmada 1 sayılır
// (sıfır eksenli tensör veri tutmaz, ama öbür eksenleri işlemlerin şeklini belirler).
func (t *Tensor) measure() error {
	info, known := typeTable[t.Type]
	if !known {
		return messages.New(messages.H6670, t.Name, t.Type.String())
	}
	last := uint64(1) // eksensiz tensör tek öğedir
	if len(t.Dims) > 0 {
		last = t.Dims[0]
	}
	if last%info.block != 0 {
		return messages.New(messages.H6671, t.Name, last, t.Type.String(), info.block)
	}
	count, limit := uint64(1), uint64(1)
	for _, n := range t.Dims {
		hi, lo := bits.Mul64(limit, max(n, 1))
		if hi != 0 || lo > math.MaxInt64 {
			return messages.New(messages.H6669, t.Name)
		}
		limit, count = lo, count*n
	}
	hi, size := bits.Mul64(count/info.block, info.bytes)
	if hi != 0 || size > math.MaxInt64 {
		return messages.New(messages.H6669, t.Name)
	}
	t.Size = size
	return nil
}

// layout veri bölümünü denetler: tensörler sırayla, hizalı ve aralıksız dizilir (gguf.cpp'nin
// "ti.offset != ctx->size" denetimi), her birinin verisi dosyada olmalıdır. Bölüm tensör
// varsa hizalanır. Boylar dosya boyuyla sınırlı olduğu için toplamlar taşmaz.
func (f *File) layout(alignment, size uint64) error {
	f.DataStart = f.HeaderEnd
	if len(f.Tensors) == 0 {
		return nil
	}
	start := pad(uint64(f.HeaderEnd), alignment)
	f.DataStart = int64(start)
	expected := uint64(0)
	for _, t := range f.Tensors {
		if t.Offset != expected {
			return messages.New(messages.H6672, t.Name, t.Offset, expected)
		}
		end, carry := bits.Add64(start+expected, t.Size, 0)
		if carry != 0 || end > size {
			return messages.New(messages.H6673, t.Name, end, size)
		}
		expected += pad(t.Size, alignment)
	}
	return nil
}

// pad n'yi hizalamanın katına yuvarlar; hizalama ikinin kuvvetidir.
func pad(n, alignment uint64) uint64 {
	return (n + alignment - 1) &^ (alignment - 1)
}

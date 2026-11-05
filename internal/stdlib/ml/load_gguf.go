package ml

import (
	"bufio"
	"io"
	"slices"

	"github.com/sekerahmet/TCB/internal/compute/gguf"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// GGUF dosyasının tensörleri (dil tanımı 15.55). Başlığı internal/compute/gguf çözer ve
// denetler; burada TR'nin okuduğu türler, eleman sınırı ve veri okuması vardır. Şekil dosyadakinin
// tersidir: dosyanın ilk boyutu son eksendir.

// ggufSplitKey bölünmüş dosyanın parça sayısı anahtarıdır.
const ggufSplitKey = "split.count"

// ggufDTypes okunan türlerin TR karşılıklarıdır; H6674'ün metni bu listeyi sayar. Nicemli türler
// depo geldiğinde buraya girer.
var ggufDTypes = map[gguf.TensorType]tensor.DType{gguf.TypeF32: tensor.Float32, gguf.TypeF16: tensor.Float16,
	gguf.TypeBF16: tensor.BFloat16}

// decodeGGUF boyutu size bayt olan GGUF dosyasını r'den çözer; bütün denetimler veri
// okunmadan yapılır, tensörler dosyadaki sırayla döner.
func decodeGGUF(r *bufio.Reader, size int64) ([]fileEntry, error) {
	f, err := gguf.Decode(r, size)
	if err != nil {
		return nil, err
	}
	if v, ok := f.Values[ggufSplitKey]; ok {
		if n, ok := v.Data.(uint16); ok && n > 1 {
			return nil, messages.New(messages.H6675, n)
		}
	}
	for _, t := range f.Tensors {
		if err := checkGGUFTensor(t); err != nil {
			return nil, err
		}
	}
	out := make([]fileEntry, 0, len(f.Tensors))
	at := f.HeaderEnd
	for _, t := range f.Tensors {
		start := f.DataStart + int64(t.Offset)
		if _, err := io.CopyN(io.Discard, r, start-at); err != nil {
			return nil, messages.New(messages.H6653, t.Name, err)
		}
		g, err := readGGUFTensor(r, t)
		if err != nil {
			return nil, messages.New(messages.H6653, t.Name, err)
		}
		out = append(out, g)
		at = start + int64(t.Size)
	}
	return out, nil
}

// checkGGUFTensor türü ve eleman sınırını denetler.
func checkGGUFTensor(t gguf.Tensor) error {
	if _, ok := ggufDTypes[t.Type]; !ok {
		return messages.New(messages.H6674, t.Name, t.Type.String())
	}
	shape := t.Shape()
	if i, bound := tensorLimit(shape, slices.Contains(shape, 0)); i >= 0 {
		return messages.New(messages.H6649, t.Name, shape[i], bound)
	}
	return nil
}

// readGGUFTensor tensörün verisini okur; boy başlıkta dosyayla denetlenmiştir.
func readGGUFTensor(r io.Reader, t gguf.Tensor) (fileEntry, error) {
	dtype := ggufDTypes[t.Type]
	g := fileEntry{ad: t.Name, dtype: dtype, shape: make([]int, len(t.Dims))}
	count := 1
	for i, d := range t.Shape() {
		g.shape[i] = int(d)
		count *= int(d)
	}
	var err error
	if dtype == tensor.Float32 {
		g.data, err = readValues(r, count)
	} else {
		g.bits, err = readBits16(r, count)
	}
	return g, err
}

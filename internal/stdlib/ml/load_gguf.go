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

// ggufDTypes okunan türlerin TR karşılıklarıdır; H6674'ün metni bu listeyi sayar. Q4_0 ("q4")
// henüz yoktur.
var ggufDTypes = map[gguf.TensorType]tensor.DType{gguf.TypeF32: tensor.Float32, gguf.TypeF16: tensor.Float16,
	gguf.TypeBF16: tensor.BFloat16, gguf.TypeQ8_0: tensor.Q8, gguf.TypeQ4_K: tensor.Q4K, gguf.TypeQ6_K: tensor.Q6K}

// decodeGGUF boyutu size bayt olan GGUF dosyasını r'den çözer; bütün denetimler veri
// okunmadan yapılır, tensörler dosyadaki sırayla döner.
func decodeGGUF(r *bufio.Reader, size int64) ([]fileEntry, error) {
	f, err := gguf.Decode(r, size)
	if err != nil {
		return nil, err
	}
	if v, ok := f.Values[ggufSplitKey]; ok {
		n, isCount := v.Uint()
		if !isCount {
			return nil, messages.New(messages.H6693, ggufSplitKey)
		}
		if n > 1 {
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
		if messages.Has(err, messages.H6686) {
			return nil, err
		}
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
	switch {
	case dtype.IsQuantized():
		// Dosyanın bitleriyle gelir, açılmaz (15.55); sıfır ölçek geçerlidir.
		if g.block = tensor.NewHostBlock(int(t.Size)); g.block != nil {
			g.raw = g.block.Bytes()
		} else {
			g.raw = make([]byte, t.Size)
		}
		if _, err = io.ReadFull(r, g.raw); err == nil {
			if b := tensor.CheckQuantScales(dtype, g.raw); b >= 0 {
				return g, messages.New(messages.H6686, t.Name, b)
			}
		}
	case dtype == tensor.Float32:
		err = g.readFloats(r, count)
	default:
		err = g.readBits(r, count)
	}
	return g, err
}

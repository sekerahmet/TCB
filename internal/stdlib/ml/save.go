package ml

import (
	"bufio"
	"encoding/binary"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"os"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
)

// TR tensör dosyası biçimi (sürüm 2). Tüm tam sayılar küçük sonlu (little-endian):
//
//	sihirli    8 bayt   "TRTENSOR"
//	sürüm      uint16   2
//	girdi      uint32   girdi sayısı
//	her girdi:
//	  ad_uzun  uint32   adın bayt uzunluğu (en fazla 65535)
//	  ad       UTF-8
//	  tür      uint8    veri türü kodu: 1 = float32, 2 = bf16, 3 = fp16
//	  boyut    uint8    eksen sayısı (en fazla 32)
//	  şekil    uint64 × boyut
//	  veri     eleman sayısı × tür boyu bayt (satır öncelikli; float32 4 bayt,
//	           bf16/fp16 2 baytlık bit deseni)
//	sağlama    uint32   önceki tüm baytların CRC-32 (IEEE) değeri
//
// Sürüm 1 aynı düzendedir ama yalnızca tür kodu 1'i (float32) içerir; artık okunmaz.
// Yüklemede (kayit_oku.go) başlık, sürüm, tür, boyutlar, eleman sınırı, dosya uzunluğu ve
// sağlama toplamı denetlenir; bozuk ya da eksik dosya açık bir hatayla reddedilir.

const (
	fileMagic    = "TRTENSOR"
	fileVersion  = 2
	dtypeFloat32 = 1
	dtypeBF16    = 2
	dtypeFP16    = 3
	maxNameLen   = 65535
	maxDims      = 32
)

// dtypeCode çekirdek türünün dosyadaki kodu.
func dtypeCode(dt tensor.DType) byte {
	switch dt {
	case tensor.BFloat16:
		return dtypeBF16
	case tensor.Float16:
		return dtypeFP16
	}
	return dtypeFloat32
}

// dtypeFromCode dosyadaki tür kodunu çekirdek türüne çevirir.
func dtypeFromCode(code byte) (tensor.DType, bool) {
	switch code {
	case dtypeFloat32:
		return tensor.Float32, true
	case dtypeBF16:
		return tensor.BFloat16, true
	case dtypeFP16:
		return tensor.Float16, true
	}
	return 0, false
}

type fileEntry struct {
	ad    string
	shape []int
	dtype tensor.DType
	data  []float32 // 16 bitlik türlerde de tensörün (kayıpsız açılmış) değerleri
}

func yolArg(fn string, v interp.Value) (string, error) {
	fpath, ok, err := interp.PathArg(fn, v)
	if !ok {
		return "", messages.New(messages.H6602, fn, typeName(v))
	}
	if err != nil {
		return "", err
	}
	if fpath == "" {
		return "", messages.New(messages.H6603, fn)
	}
	return fpath, nil
}

// tensorSave adları tensör olan bir sözlüğü dosyaya yazar: tensör_kaydet(yol, sözlük).
func tensorSave(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T700)
	if len(args) != 2 {
		return nil, messages.New(messages.H6604, fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named); err != nil {
		return nil, err
	}
	fpath, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	dict, ok := args[1].(*interp.DictVal)
	if !ok {
		return nil, messages.New(messages.H6605, fn, typeName(args[1]))
	}
	// Veri yorumlayıcı kilidi altında kopyalanır; yazma kilit bırakılarak yapılır.
	inputs := make([]fileEntry, 0, len(dict.Order))
	for _, k := range dict.Order {
		if _, text := dict.KeyValue(k).(*interp.StringVal); !text {
			return nil, messages.New(messages.H6606, fn, dict.DisplayKey(k))
		}
		if len(k) > maxNameLen {
			return nil, messages.New(messages.H6607, fn, len(k), maxNameLen)
		}
		d, ok := dict.Pairs[k].(*TensorValue)
		if !ok {
			return nil, messages.New(messages.H6608, fn, k, typeName(dict.Pairs[k]))
		}
		sh := d.t.Shape()
		if len(sh) > maxDims {
			return nil, messages.New(messages.H6609, fn, k, maxDims, shapeText(sh))
		}
		inputs = append(inputs, fileEntry{ad: k, shape: sh, dtype: d.t.DType(), data: d.t.Data()})
	}
	// Göreli yol TR'nin tek yol kuralıyla çözülür (bkz. interp.(*VM).ResolvePath).
	decoded := bind.ResolvePath(m, fpath)
	var writeErr error
	m.Blocking(func() { writeErr = writeToFile(decoded, inputs) })
	if writeErr != nil {
		return nil, turkish.FileError(fn, fpath, messages.H2101, writeErr)
	}
	return interp.Null, nil
}

// writeToFile girdileri yol'a atomik olarak yazar (bkz. interp.WriteAtomic): yazma yarıda kalırsa
// önceki kontrol noktası bozulmaz.
func writeToFile(fpath string, inputs []fileEntry) error {
	return interp.WriteAtomic(fpath, func(f io.Writer) error { return writeEntries(f, inputs) })
}

func writeEntries(f io.Writer, inputs []fileEntry) error {
	y := NewTensorWriter(f, len(inputs))
	for _, g := range inputs {
		if err := y.Write(g.ad, g.shape, g.dtype, g.data); err != nil {
			return err
		}
	}
	return y.Finish()
}

// TensorWriter tensör dosyasını (biçim yukarıda) girdi girdi yazar: kontrol noktası büyük
// modelleri bütün veriyi bellekte toplamadan, tensör tensör konağa indirip bununla yazar.
// Girdi sayısı baştan bilinir; Bitir sağlama toplamını yazar.
type TensorWriter struct {
	bw        *bufio.Writer
	crc       hash.Hash32
	w         io.Writer
	remaining int
	b         [8]byte
	buffer    []byte
	failure   error
}

// NewTensorWriter f'ye entryCount girdilik bir tensör dosyası yazmaya başlar (başlık).
func NewTensorWriter(f io.Writer, entryCount int) *TensorWriter {
	y := &TensorWriter{bw: bufio.NewWriterSize(f, 1<<16), crc: crc32.NewIEEE(), remaining: entryCount, buffer: make([]byte, 0, 1<<16)}
	y.w = io.MultiWriter(y.bw, y.crc)
	y.write([]byte(fileMagic))
	y.u16(fileVersion)
	y.u32(uint32(entryCount))
	return y
}

func (y *TensorWriter) write(p []byte) {
	if y.failure == nil {
		_, y.failure = y.w.Write(p)
	}
}
func (y *TensorWriter) u16(x uint16) { binary.LittleEndian.PutUint16(y.b[:2], x); y.write(y.b[:2]) }
func (y *TensorWriter) u32(x uint32) { binary.LittleEndian.PutUint32(y.b[:4], x); y.write(y.b[:4]) }
func (y *TensorWriter) u64(x uint64) { binary.LittleEndian.PutUint64(y.b[:8], x); y.write(y.b[:8]) }

// Write bir girdiyi yazar: veri satır öncelikli değerlerdir (16 bitlik türlerde o türden
// açılmış değerler; geri çevirme kayıpsızdır).
func (y *TensorWriter) Write(ad string, shape []int, dtype tensor.DType, data []float32) error {
	switch {
	case y.failure != nil:
		return y.failure
	case y.remaining <= 0:
		return messages.New(messages.H6618, ad)
	case len(ad) > maxNameLen:
		return messages.New(messages.H6619, ad[:64], len(ad), maxNameLen)
	case len(shape) > maxDims:
		return messages.New(messages.H6620, ad, maxDims, shapeText(shape))
	}
	y.remaining--
	y.u32(uint32(len(ad)))
	y.write([]byte(ad))
	y.write([]byte{dtypeCode(dtype), byte(len(shape))})
	for _, d := range shape {
		y.u64(uint64(d))
	}
	on6 := dtype == tensor.BFloat16 || dtype == tensor.Float16
	for i := 0; i < len(data); {
		t := y.buffer[:0]
		for ; i < len(data) && len(t) < cap(t); i++ {
			if on6 {
				t = binary.LittleEndian.AppendUint16(t, tensor.Bits16(dtype, data[i]))
			} else {
				t = binary.LittleEndian.AppendUint32(t, math.Float32bits(data[i]))
			}
		}
		y.write(t)
	}
	return y.failure
}

// Finish sağlama toplamını yazar ve tamponu boşaltır; bütün girdiler yazılmış olmalıdır.
func (y *TensorWriter) Finish() error {
	if y.failure != nil {
		return y.failure
	}
	if y.remaining != 0 {
		return messages.New(messages.H6622, y.remaining)
	}
	binary.LittleEndian.PutUint32(y.b[:4], y.crc.Sum32())
	if _, err := y.bw.Write(y.b[:4]); err != nil {
		return err
	}
	return y.bw.Flush()
}

// tensorLoad tensör_kaydet ile yazılmış dosyayı sözlük olarak okur.
func tensorLoad(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.DisplayName(codes.T701)
	if len(args) != 1 {
		return nil, messages.New(messages.H5608, fn, len(args))
	}
	if err := bind.CheckNamed(fn+"()", named, nameRequiresGrad); err != nil {
		return nil, err
	}
	fpath, err := yolArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	watch, err := requiresGrad(fn, named, nil, false)
	if err != nil {
		return nil, err
	}
	decoded, tried := bind.ResolvePathTried(m, fpath)
	var inputs []fileEntry
	var readErr, osErr error
	m.Blocking(func() { inputs, readErr, osErr = readFromFile(decoded) })
	if osErr != nil {
		return nil, interp.ReadError(fn, fpath, messages.H2107, osErr, tried)
	}
	if readErr != nil {
		return nil, messages.New(messages.H6626, fn, fpath, readErr)
	}
	result := interp.NewDictVal()
	for _, g := range inputs {
		var t *tensor.Tensor
		if g.dtype == tensor.Float32 {
			t = tensor.New(g.data, g.shape...)
		} else {
			t = tensor.NewOf(g.dtype, g.data, g.shape...)
		}
		t.SetRequiresGrad(watch)
		result.Set(g.ad, New(t))
	}
	return result, nil
}

// readFromFile dosyayı çözer. İlk hata dosya biçimi hatası, ikincisi işletim sistemi
// hatasıdır.
func readFromFile(fpath string) ([]fileEntry, error, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, messages.New(messages.H6629), nil
	}
	inputs, err := decodeTensorFile(f, info.Size())
	return inputs, err, nil
}

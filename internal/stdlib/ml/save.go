package ml

import (
	"bufio"
	"encoding/binary"
	"hash"
	"hash/crc32"
	"io"
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
//	  tür      uint8    veri türü kodu: 1 = float32, 2 = bf16, 3 = fp16, 4 = float64,
//	                    5 = int64, 6 = int32, 7 = int16, 8 = int8, 9 = uint32, 10 = uint16,
//	                    11 = uint8, 12 = mantık (15.46)
//	  boyut    uint8    eksen sayısı (en fazla 32)
//	  şekil    uint64 × boyut
//	  veri     eleman sayısı × tür boyu bayt (satır öncelikli, küçük uçlu; float32 4 bayt,
//	           bf16/fp16 2 baytlık bit deseni, mantık 1 bayt 0/1)
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

// dtypeCode çekirdek türünün dosyadaki kodu; kodu olmayan türde (nicemlenmiş Int8, Int4) ok
// yanlıştır: o tür float32 koduyla yazılsaydı dosya sessizce yanlış olurdu (15.55: her nicemli
// hassasiyetin kendi tür kodu vardır).
func dtypeCode(dt tensor.DType) (byte, bool) {
	for code := 1; code < len(fileDTypes); code++ {
		if fileDTypes[code] == dt {
			return byte(code), true
		}
	}
	return 0, false
}

// fileDTypes tür kodlarının türleri (dizin = kod; 0 kod değildir, yeri boştur). 1–3 sürüm 2'nin ilk
// kodlarıdır, öbürleri cinsli türlerin (15.46); düzen bütün türlerde aynıdır.
var fileDTypes = []tensor.DType{0, tensor.Float32, tensor.BFloat16, tensor.Float16, tensor.Float64,
	tensor.Int64, tensor.Int32, tensor.Int16, tensor.Int8, tensor.UInt32, tensor.UInt16, tensor.UInt8, tensor.Bool}

// dtypeFromCode dosyadaki tür kodunu çekirdek türüne çevirir.
func dtypeFromCode(code byte) (tensor.DType, bool) {
	if code == 0 || int(code) >= len(fileDTypes) {
		return 0, false
	}
	return fileDTypes[code], true
}

type fileEntry struct {
	ad    string
	shape []int
	dtype tensor.DType
	data  []float32 // float32 tensörün değerleri
	raw   []byte    // cinsli türlerde (float64, TR_Int, TR_Bool) küçük uçlu ham baytlar
	bits  []uint16  // 16 bitlik ondalıklar: tensörün ve dosyanın bitleri, açılmadan

	block *tensor.HostBlock // data, raw ya da bits öbek dışındaysa sahipleri (15.53)
}

// newFileEntry tensörün verisini konağa alır: hesap ondalığı değer, öbürü ham bayt olarak.
func newFileEntry(ad string, t *tensor.Tensor) fileEntry {
	g := fileEntry{ad: ad, shape: t.Shape(), dtype: t.DType()}
	switch {
	case g.dtype.IsTyped():
		g.raw = t.Raw()
	case g.dtype.Bits() == 16:
		g.bits = t.Bits16s()
	default:
		g.data = t.Data()
	}
	return g
}

// tensor dosyadan okunan girdinin tensörüdür (varsayılan cihazda).
func (g fileEntry) tensor() *tensor.Tensor {
	switch {
	case g.block != nil:
		return tensor.FromHostBlock(g.block, g.dtype, g.shape...)
	case g.dtype.IsQuantized():
		return tensor.FromQuantized(g.dtype, g.raw, g.shape...)
	case g.dtype.IsTyped():
		return tensor.FromRawOn(tensor.DefaultDevice(), g.dtype, g.raw, g.shape...)
	case g.bits != nil:
		return tensor.FromBits16On(tensor.DefaultDevice(), g.dtype, g.bits, g.shape...)
	case g.dtype == tensor.Float32:
		return tensor.New(g.data, g.shape...)
	}
	return tensor.NewOf(g.dtype, g.data, g.shape...)
}

// count girdinin öğe sayısı.
func (g fileEntry) count() int {
	if elems, size := g.dtype.QuantBlock(); elems > 0 {
		return len(g.raw) / size * elems
	}
	if g.dtype.IsTyped() {
		return len(g.raw) / (g.dtype.Bits() / 8)
	}
	if g.bits != nil {
		return len(g.bits)
	}
	return len(g.data)
}

func yolArg(fn any, v interp.Value) (string, error) {
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
// `.safetensors` ile biten yol safetensors (save_safetensors.go), öbürleri TR biçimini yazar (15.46).
func tensorSave(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.NameRef(codes.FnSaveTensors)
	if len(args) != 2 {
		return nil, messages.New(messages.H6604, fn, len(args))
	}
	if err := bind.CheckNamed(fn.Called(), named); err != nil {
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
			return nil, messages.New(messages.H6606, fn, dict.KeyText(k))
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
			return nil, messages.New(messages.H6609, fn, k, maxDims, tensor.ShapeText(sh))
		}
		inputs = append(inputs, newFileEntry(k, d.t))
	}
	// Göreli yol yalnız çalışma klasörüne göre çözülür (18.16, interp.(*VM).ResolveWritePath).
	write := writeToFile
	if isSafetensorsPath(fpath) {
		if err := checkSafetensorsEntries(fn, inputs); err != nil {
			return nil, err
		}
		write = writeSafetensorsFile
	}
	decoded, _ := m.ResolveWritePath(fpath)
	var writeErr error
	m.Blocking(func() { writeErr = write(decoded, inputs) })
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
		var err error
		switch {
		case g.dtype.IsTyped():
			err = y.WriteRaw(g.ad, g.shape, g.dtype, g.raw)
		case g.bits != nil:
			err = y.WriteBits(g.ad, g.shape, g.dtype, g.bits)
		default:
			err = y.Write(g.ad, g.shape, g.dtype, g.data)
		}
		if err != nil {
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

// Write bir hesap ondalığı girdisini yazar: veri satır öncelikli değerlerdir (16 bitlik
// türlerde o türden açılmış değerler; geri çevirme kayıpsızdır).
func (y *TensorWriter) Write(ad string, shape []int, dtype tensor.DType, data []float32) error {
	if err := y.entryHeader(ad, shape, dtype); err != nil {
		return err
	}
	y.failure = writeValues(y.w, dtype, data, y.buffer)
	return y.failure
}

// WriteBits 16 bitlik bir girdiyi bitleriyle yazar.
func (y *TensorWriter) WriteBits(ad string, shape []int, dtype tensor.DType, bits []uint16) error {
	if err := y.entryHeader(ad, shape, dtype); err != nil {
		return err
	}
	y.failure = writeBits16(y.w, bits, y.buffer)
	return y.failure
}

// WriteRaw cinsli bir girdiyi (float64, TR_Int, TR_Bool) satır öncelikli küçük uçlu ham
// baytlarıyla yazar.
func (y *TensorWriter) WriteRaw(ad string, shape []int, dtype tensor.DType, raw []byte) error {
	if err := y.entryHeader(ad, shape, dtype); err != nil {
		return err
	}
	y.write(raw)
	return y.failure
}

// entryHeader girdinin adını, tür kodunu ve şeklini yazar.
func (y *TensorWriter) entryHeader(ad string, shape []int, dtype tensor.DType) error {
	switch {
	case y.failure != nil:
		return y.failure
	case y.remaining <= 0:
		return messages.New(messages.H6618, ad)
	case len(ad) > maxNameLen:
		return messages.New(messages.H6619, ad[:64], len(ad), maxNameLen)
	case len(shape) > maxDims:
		return messages.New(messages.H6620, ad, maxDims, tensor.ShapeText(shape))
	}
	code, ok := dtypeCode(dtype)
	if !ok {
		return messages.New(messages.H6959, ad, dtype.String())
	}
	y.remaining--
	y.u32(uint32(len(ad)))
	y.write([]byte(ad))
	y.write([]byte{code, byte(len(shape))})
	for _, d := range shape {
		y.u64(uint64(d))
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
	fn := codes.NameRef(codes.FnLoadTensors)
	if len(args) != 1 {
		return nil, messages.New(messages.H5608, fn, len(args))
	}
	if err := bind.CheckNamed(fn.Called(), named, nameRequiresGrad); err != nil {
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
	// Dosyadan yükleme programın durumunu değiştirmez: durdurma onu bırakır (1.5).
	if waitErr := m.Abandonable(func() { inputs, readErr, osErr = readFromFile(decoded) }); waitErr != nil {
		return nil, waitErr
	}
	if osErr != nil {
		return nil, interp.ReadError(fn, fpath, messages.H2107, osErr, tried)
	}
	if readErr != nil {
		return nil, messages.New(messages.H6626, fn, fpath, readErr)
	}
	result := interp.NewDictVal()
	for _, g := range inputs {
		t, err := safe(g.tensor)
		if err != nil {
			return nil, err
		}
		// Türev nicemlenmemiş TR_Float tensörlerde açılır, float64 dahil; TR_Int, TR_Bool ve
		// nicemlenmiş tensörler türevsiz kalır (15.46). float64'te türev yolu yok: çizgeye
		// girince H6040 açık hatadır (tensor.CheckFloat64Grad).
		t.SetRequiresGrad(watch && (g.dtype.IsFloat() || g.dtype == tensor.Float64))
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

package ml

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Tensör dosyasının (biçim: kayit.go) çözücüsü. Dosya güvenilmeyen girdidir (indirilen
// kontrol noktası): her uzunluk alanı, ayırmadan önce dosyada kalan bayt sayısıyla ve
// eleman sınırıyla sınanır; sağlama toplamı en sonda denetlenir.

// fileReader tensör dosyasını sırayla okur.
type fileReader struct {
	br        *bufio.Reader
	r         io.Reader // br'den okur, okuduğunu sağlama toplamına da yazar
	crc       hash.Hash32
	remaining int64 // dosyada henüz okunmamış bayt sayısı
	b         [8]byte
	failure   error // ilk okuma hatası (sonraki okumalar yapılmaz)
}

// read n baytı (en fazla 8) okur; hata olursa k.hata dolar.
func (k *fileReader) read(n int) []byte {
	if k.failure != nil {
		return k.b[:n]
	}
	if _, err := io.ReadFull(k.r, k.b[:n]); err != nil {
		k.failure = err
	}
	k.remaining -= int64(n)
	return k.b[:n]
}

// integer p'yi tümüyle doldurur; dosya biterse ya da okunamazsa yanlış döner.
func (k *fileReader) integer(p []byte) bool {
	if k.failure != nil {
		return false
	}
	if _, err := io.ReadFull(k.r, p); err != nil {
		k.failure = err
		return false
	}
	k.remaining -= int64(len(p))
	return true
}

// missing ne okunurken oluşan okuma hatasını anlatır.
func (k *fileReader) missing(ne string) error {
	if k.failure != nil && !errors.Is(k.failure, io.EOF) && !errors.Is(k.failure, io.ErrUnexpectedEOF) {
		return messages.New(messages.H6600, ne, k.failure)
	}
	return messages.New(messages.H6601, ne)
}

// decodeFile boyut bayt uzunluğundaki bir tensör dosyasının içeriğini f'den çözer.
func decodeFile(f io.Reader, dim int64) ([]fileEntry, error) {
	var inputs []fileEntry
	err := walkFile(f, dim, func(g fileEntry) error {
		inputs = append(inputs, g)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inputs, nil
}

// walkFile boyut bayt uzunluğundaki bir tensör dosyasını f'den sırayla çözer ve her girdiyi
// her'e verir; her hata dönerse durur. Sağlama toplamı en sonda denetlenir: her'in gördüğü
// veriler walkFile hatasız dönene kadar güvenilir sayılmamalıdır.
func walkFile(f io.Reader, dim int64, each func(fileEntry) error) error {
	k := &fileReader{br: bufio.NewReaderSize(f, 1<<16), crc: crc32.NewIEEE(), remaining: dim}
	k.r = io.TeeReader(k.br, k.crc)
	number, err := k.header()
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for gi := 0; gi < int(number); gi++ {
		g, err := k.input(gi, names)
		if err != nil {
			return err
		}
		if err := each(g); err != nil {
			return err
		}
	}
	return k.checksum()
}

// TensorEntry tensör dosyasındaki bir girdidir: ad, şekil, veri türü ve (16 bitlik
// türlerde kayıpsız açılmış) değerler.
type TensorEntry struct {
	Ad    string
	Sekil []int
	Tur   tensor.DType
	Veri  []float32
}

// WalkTensorFile tensör_kaydet biçimindeki bir dosyayı (boyut bayt) r'den sırayla çözer
// ve her girdiyi her'e verir; büyük dosyalar bütün veri bellekte toplanmadan okunur. Bütün
// sınırlar ve denetimler tensör_yükle'ninkilerdir; sağlama toplamı en sonda denetlenir, bu
// yüzden her'in gördüğü veriler dönüş hatasız olana kadar güvenilir sayılmamalıdır (kontrol
// noktası dosyayı önce yalnızca denetler, sonra yükler). Hata dosya biçimi hatasıdır.
func WalkTensorFile(r io.Reader, dim int64, each func(TensorEntry) error) error {
	return walkFile(r, dim, func(g fileEntry) error {
		return each(TensorEntry{Ad: g.ad, Sekil: g.shape, Tur: g.dtype, Veri: g.data})
	})
}

// header sihirli baytları ve sürümü denetler, girdi sayısını verir.
func (k *fileReader) header() (uint32, error) {
	if string(k.read(8)) != fileMagic || k.failure != nil {
		return 0, messages.New(messages.H6610, fileMagic, codes.Name(codes.T700))
	}
	version := binary.LittleEndian.Uint16(k.read(2))
	if k.failure != nil {
		return 0, k.missing(messages.Text(messages.H6630))
	}
	if version != fileVersion {
		return 0, messages.New(messages.H6611, version, fileVersion, codes.Name(codes.T700))
	}
	number := binary.LittleEndian.Uint32(k.read(4))
	if k.failure != nil {
		return 0, k.missing(messages.Text(messages.H6631))
	}
	// Her girdi en az 4+1+1 bayt tutar; bozuk sayının dev ayırmaya yol açmasını önler.
	if int64(number)*6 > k.remaining {
		return 0, messages.New(messages.H6612, number, k.remaining)
	}
	return number, nil
}

// input gi. girdiyi (ad, tür, şekil, veri) okur; adlar aynı adın iki kez geçmesini yakalar.
func (k *fileReader) input(gi int, names map[string]bool) (fileEntry, error) {
	order := messages.Text(messages.H6636, gi+1)
	nameLen := binary.LittleEndian.Uint32(k.read(4))
	if k.failure != nil {
		return fileEntry{}, k.missing(messages.Text(messages.H6632, order))
	}
	if nameLen > maxNameLen || int64(nameLen) > k.remaining {
		return fileEntry{}, messages.New(messages.H6613, order, nameLen)
	}
	nameB := make([]byte, nameLen)
	if !k.integer(nameB) {
		return fileEntry{}, k.missing(messages.Text(messages.H6632, order))
	}
	ad := string(nameB)
	if !utf8.ValidString(ad) {
		return fileEntry{}, messages.New(messages.H6614, order)
	}
	if names[ad] {
		return fileEntry{}, messages.New(messages.H6615, ad)
	}
	names[ad] = true
	order = messages.Text(messages.H6637, gi+1, ad)
	tb := k.read(2)
	if k.failure != nil {
		return fileEntry{}, k.missing(messages.Text(messages.H6633, order))
	}
	code, dim := tb[0], int(tb[1])
	dtype, known := dtypeFromCode(code)
	if !known {
		return fileEntry{}, messages.New(messages.H6616, order, code, dtypeFloat32, dtypeBF16, dtypeFP16)
	}
	if dim > maxDims {
		return fileEntry{}, messages.New(messages.H6617, order, dim, maxDims)
	}
	shape, n, err := k.shape(order, dim)
	if err != nil {
		return fileEntry{}, err
	}
	data, err := k.data(order, dtype, shape, n)
	if err != nil {
		return fileEntry{}, err
	}
	return fileEntry{ad: ad, shape: shape, dtype: dtype, data: data}, nil
}

// shape boyut eksenli şekli ve eleman sayısını okur. Eleman sınırı sıfır uzunluklu eksenler
// 1 sayılarak uygulanır: sıfır eksenli bir tensör veri tutmaz, ama öbür eksenleri
// işlemlerin çıktı şeklini belirler (ör. [4, 0] @ [0, N] → [4, N]). Sınır yalnızca gerçek
// eleman sayısına uygulansaydı yüz baytlık bir dosya dev eksenler bildirip ilk işlemde
// belleği tüketebilirdi (güvenlik denetimi 2026-10-18).
func (k *fileReader) shape(order string, dim int) ([]int, int64, error) {
	shape := make([]int, dim)
	n, limit := int64(1), int64(1)
	for i := range shape {
		d := binary.LittleEndian.Uint64(k.read(8))
		if k.failure != nil {
			return nil, 0, k.missing(messages.Text(messages.H6634, order))
		}
		if d > uint64(interp.MaxCollectionSize) || (d > 0 && limit > int64(interp.MaxCollectionSize)/int64(d)) {
			return nil, 0, messages.New(messages.H6621, order, d, interp.MaxCollectionSize)
		}
		shape[i] = int(d)
		n *= int64(d)
		limit *= max(int64(d), 1)
	}
	return shape, n, nil
}

// data n elemanlık veriyi okur ve float32'ye açar.
func (k *fileReader) data(order string, dtype tensor.DType, shape []int, n int64) ([]float32, error) {
	size := int64(dtype.Bits() / 8) // eleman başına bayt
	if n*size > k.remaining-4 {
		return nil, messages.New(messages.H6623, order, shapeText(shape), n*size, max(k.remaining-4, 0))
	}
	data := make([]float32, n)
	buffer := make([]byte, 1<<16)
	eb := int(size)
	for i := 0; i < len(data); {
		part := min(len(buffer)/eb, len(data)-i)
		if !k.integer(buffer[:part*eb]) {
			return nil, k.missing(messages.Text(messages.H6635, order))
		}
		for j := 0; j < part; j++ {
			if eb == 2 {
				data[i+j] = tensor.FromBits16(dtype, binary.LittleEndian.Uint16(buffer[j*2:]))
			} else {
				data[i+j] = math.Float32frombits(binary.LittleEndian.Uint32(buffer[j*4:]))
			}
		}
		i += part
	}
	return data, nil
}

// checksum dosyanın sonundaki sağlama toplamını denetler; sonrasında veri kalmamalıdır.
func (k *fileReader) checksum() error {
	expected := k.crc.Sum32()
	if _, err := io.ReadFull(k.br, k.b[:4]); err != nil {
		return messages.New(messages.H6625)
	}
	if binary.LittleEndian.Uint32(k.b[:4]) != expected {
		return messages.New(messages.H6627)
	}
	if _, err := k.br.ReadByte(); err != io.EOF {
		return messages.New(messages.H6628)
	}
	return nil
}

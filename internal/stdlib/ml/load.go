package ml

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"math"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
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
		return fmt.Errorf("%s okunamadı: %v", ne, k.failure)
	}
	return fmt.Errorf("dosya bozuk ya da eksik: %s okunurken dosya bitti", ne)
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
		return 0, fmt.Errorf("bir TR tensör dosyası değil (başlık %q bulunamadı; dosya %s ile yazılmalı)", fileMagic, codes.Name(codes.T700))
	}
	version := binary.LittleEndian.Uint16(k.read(2))
	if k.failure != nil {
		return 0, k.missing("sürüm")
	}
	if version != fileVersion {
		return 0, fmt.Errorf("dosya biçimi sürümü %d; bu TR sürümü yalnızca sürüm %d okur (dosyayı %s ile yeniden yazın)", version, fileVersion, codes.Name(codes.T700))
	}
	number := binary.LittleEndian.Uint32(k.read(4))
	if k.failure != nil {
		return 0, k.missing("girdi sayısı")
	}
	// Her girdi en az 4+1+1 bayt tutar; bozuk sayının dev ayırmaya yol açmasını önler.
	if int64(number)*6 > k.remaining {
		return 0, fmt.Errorf("dosya bozuk: %d girdi bildiriliyor ama dosyada yalnızca %d bayt kaldı", number, k.remaining)
	}
	return number, nil
}

// input gi. girdiyi (ad, tür, şekil, veri) okur; adlar aynı adın iki kez geçmesini yakalar.
func (k *fileReader) input(gi int, names map[string]bool) (fileEntry, error) {
	order := fmt.Sprintf("%d. girdi", gi+1)
	nameLen := binary.LittleEndian.Uint32(k.read(4))
	if k.failure != nil {
		return fileEntry{}, k.missing(order + " adı")
	}
	if nameLen > maxNameLen || int64(nameLen) > k.remaining {
		return fileEntry{}, fmt.Errorf("dosya bozuk: %s adının uzunluğu %d geçersiz", order, nameLen)
	}
	nameB := make([]byte, nameLen)
	if !k.integer(nameB) {
		return fileEntry{}, k.missing(order + " adı")
	}
	ad := string(nameB)
	if !utf8.ValidString(ad) {
		return fileEntry{}, fmt.Errorf("dosya bozuk: %s adı geçerli UTF-8 değil", order)
	}
	if names[ad] {
		return fileEntry{}, fmt.Errorf("dosya bozuk: %q adı iki kez geçiyor", ad)
	}
	names[ad] = true
	order = fmt.Sprintf("%d. girdi (%q)", gi+1, ad)
	tb := k.read(2)
	if k.failure != nil {
		return fileEntry{}, k.missing(order + " türü")
	}
	code, dim := tb[0], int(tb[1])
	dtype, known := dtypeFromCode(code)
	if !known {
		return fileEntry{}, fmt.Errorf("%s: desteklenmeyen veri türü kodu %d (bu TR sürümü float32 = %d, bf16 = %d, fp16 = %d okuyabilir)", order, code, dtypeFloat32, dtypeBF16, dtypeFP16)
	}
	if dim > maxDims {
		return fileEntry{}, fmt.Errorf("dosya bozuk: %s %d eksenli bildiriliyor (en fazla %d)", order, dim, maxDims)
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
			return nil, 0, k.missing(order + " şekli")
		}
		if d > uint64(interp.MaxCollectionSize) || (d > 0 && limit > int64(interp.MaxCollectionSize)/int64(d)) {
			return nil, 0, fmt.Errorf("dosya bozuk ya da tensör çok büyük: %s ekseni %d (en fazla %d eleman; sıfır uzunluklu eksenler 1 sayılır)", order, d, interp.MaxCollectionSize)
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
		return nil, fmt.Errorf("dosya bozuk ya da eksik: %s %s şeklinde %d bayt veri bekliyor, dosyada %d bayt kaldı", order, shapeText(shape), n*size, max(k.remaining-4, 0))
	}
	data := make([]float32, n)
	buffer := make([]byte, 1<<16)
	eb := int(size)
	for i := 0; i < len(data); {
		part := min(len(buffer)/eb, len(data)-i)
		if !k.integer(buffer[:part*eb]) {
			return nil, k.missing(order + " verisi")
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
		return fmt.Errorf("dosya bozuk ya da eksik: sağlama toplamı okunurken dosya bitti")
	}
	if binary.LittleEndian.Uint32(k.b[:4]) != expected {
		return fmt.Errorf("dosya bozuk: sağlama toplamı tutmuyor (dosya değiştirilmiş ya da yarım yazılmış olabilir)")
	}
	if _, err := k.br.ReadByte(); err != io.EOF {
		return fmt.Errorf("dosya bozuk: sağlama toplamından sonra fazladan veri var")
	}
	return nil
}

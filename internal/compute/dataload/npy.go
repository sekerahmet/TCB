// Package dataload, dil modeli eğitimini besleyen veri yükleyicinin çekirdeğidir.
//
// Katmanlar (aşağıdan yukarıya):
//
//   - .npy başlık okuyucu/yazıcı (BaslikOku, BaslikCoz, BaslikYaz)
//   - Parca: tek bir .npy dosyasına bellek eşlemeli (ya da ReadAt ile) erişim
//   - Kume: bir dizindeki parçalar ve isteğe bağlı meta.json
//   - Ornekleyici: tohumlu, tekrar üretilebilir rastgele pencere örnekleme
//   - SiraliOkuyucu: karıştırmasız, sıralı doğrulama okuması
//   - OnYukleyici: arka planda sonraki toplu işleri hazırlayan çoklu tampon
//   - Onbellek: yavaş kaynaktaki parçaları yerel diske kopyalayan kotalı katman
//
// Paket yalnızca standart kütüphaneyi kullanır.
//
// Layer: compute core (internal/compute); yalnızca internal/compute paketlerini içe aktarır.
package dataload

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// DType, .npy dosyasındaki elemanların türüdür.
type DType uint8

const (
	// DTypeInvalid tanınmayan türdür.
	DTypeInvalid DType = iota
	// DTypeU16 küçük uçlu işaretsiz 16 bit ('<u2').
	DTypeU16
	// DTypeU32 küçük uçlu işaretsiz 32 bit ('<u4').
	DTypeU32
	// DTypeI32 küçük uçlu işaretli 32 bit ('<i4').
	DTypeI32
)

// Size, bir elemanın bayt cinsinden boyutudur.
func (t DType) Size() int {
	switch t {
	case DTypeU16:
		return 2
	case DTypeU32, DTypeI32:
		return 4
	}
	return 0
}

// String, türün .npy 'descr' gösterimidir.
func (t DType) String() string {
	switch t {
	case DTypeU16:
		return "<u2"
	case DTypeU32:
		return "<u4"
	case DTypeI32:
		return "<i4"
	}
	return messages.Text(messages.H5092)
}

func parseDType(descr string) (DType, error) {
	switch descr {
	case "<u2":
		return DTypeU16, nil
	case "<u4":
		return DTypeU32, nil
	case "<i4":
		return DTypeI32, nil
	}
	if len(descr) == 3 && descr[0] == '>' {
		return DTypeInvalid, messages.New(messages.H5028, descr)
	}
	return DTypeInvalid, messages.New(messages.H5029, descr)
}

// Header, çözümlenmiş bir .npy başlığıdır.
type Header struct {
	Version      uint8 // ana sürüm: 1, 2 ya da 3
	MinorVersion uint8
	DType        DType
	Length       int64 // eleman (token) sayısı
	DataOffset   int64 // verinin dosya başından itibaren bayt konumu
}

// DataSize, veri bölümünün bayt cinsinden boyutudur.
func (b Header) DataSize() int64 { return b.Length * int64(b.DType.Size()) }

// FormatError, geçersiz ya da desteklenmeyen bir .npy başlığını bildirir.
type FormatError struct {
	Path   string // biliniyorsa dosya yolu
	Reason error  // ileti kodlu neden
}

func (h *FormatError) Error() string { return h.message().Error() }

// MessageText hatanın lang dilindeki metnidir (messages.Localized).
func (h *FormatError) MessageText(lang string) string { return messages.ErrorIn(h.message(), lang) }

// message hatanın kodlu iletisidir.
func (h *FormatError) message() *messages.Error {
	if h.Path != "" {
		return messages.New(messages.H5090, h.Path, h.Reason)
	}
	return messages.New(messages.H5091, h.Reason)
}

// Unwrap nedeni verir; böylece nedenin ileti kodu hata zincirinde aranabilir.
func (h *FormatError) Unwrap() error { return h.Reason }

// formatError code iletisiyle bir .npy biçim hatası kurar.
func formatError(code messages.Code, a ...any) error {
	return &FormatError{Reason: messages.New(code, a...)}
}

const (
	npyMagic       = "\x93NUMPY"
	maxHeader      = 1 << 20 // makul olmayan büyüklükteki başlıkları reddet
	npyAlign       = 64
	headerPrefixV1 = 10 // büyü(6) + sürüm(2) + uzunluk(2)
	headerPrefixV2 = 12 // büyü(6) + sürüm(2) + uzunluk(4)
	minFileStart   = headerPrefixV1
)

// readHeaderPath ReadHeader gibidir; biçim hatasına dosya yolunu ekler.
func readHeaderPath(r io.ReaderAt, path string) (Header, error) {
	bs, err := ReadHeader(r)
	var bh *FormatError
	if errors.As(err, &bh) {
		bh.Path = path
	}
	return bs, err
}

// headerExtent b'nin (en az minFileStart bayt) sürümünden başlık metninin uzunluğunu (bl) ve ön ek
// boyunu (prefix) çıkarır. magicFirst doğruysa büyü baytları önce denetlenir (H5053); değilse yalnızca
// bilinmeyen sürümde (ReadHeader: yanlış dosyada daha anlamlı hata). shortCode sürüm 2/3'te ön ek
// 12 bayttan kısaysa verilen hatadır (ReadHeader H5052, ParseHeader H5058).
func headerExtent(b []byte, magicFirst bool, shortCode messages.Code) (bl, prefix int, err error) {
	if magicFirst && string(b[:6]) != npyMagic {
		return 0, 0, formatError(messages.H5053)
	}
	switch b[6] {
	case 1:
		bl, prefix = int(binary.LittleEndian.Uint16(b[8:10])), headerPrefixV1
	case 2, 3:
		if len(b) < headerPrefixV2 {
			return 0, 0, formatError(shortCode, b[6])
		}
		bl, prefix = int(binary.LittleEndian.Uint32(b[8:12])), headerPrefixV2
	default:
		if string(b[:6]) != npyMagic {
			return 0, 0, formatError(messages.H5053)
		}
		return 0, 0, formatError(messages.H5054, b[6], b[7])
	}
	if bl > maxHeader {
		return 0, 0, formatError(messages.H5055, bl)
	}
	return bl, prefix, nil
}

// ReadHeader, r'nin başından .npy başlığını okur ve çözümler. Yalnızca
// başlık baytları okunur; veri bölümüne dokunulmaz.
func ReadHeader(r io.ReaderAt) (Header, error) {
	var on [headerPrefixV2]byte
	n, err := r.ReadAt(on[:], 0)
	if n < minFileStart {
		if err == nil || errors.Is(err, io.EOF) {
			return Header{}, formatError(messages.H5051, n)
		}
		return Header{}, messages.New(messages.H5030, err)
	}
	bl, prefix, err := headerExtent(on[:n], false, messages.H5052)
	if err != nil {
		return Header{}, err
	}
	exact := make([]byte, prefix+bl)
	copy(exact, on[:n])
	if prefix+bl > n {
		m, err := r.ReadAt(exact[n:], int64(n))
		if m < len(exact)-n {
			if err == nil || errors.Is(err, io.EOF) {
				return Header{}, formatError(messages.H5056, prefix+bl)
			}
			return Header{}, messages.New(messages.H5030, err)
		}
	}
	return ParseHeader(exact)
}

// ParseHeader, bellekteki baytlardan (en az tüm başlık) .npy başlığını çözümler.
func ParseHeader(b []byte) (Header, error) {
	if len(b) < minFileStart {
		return Header{}, formatError(messages.H5057, len(b))
	}
	if string(b[:6]) != npyMagic {
		return Header{}, formatError(messages.H5053)
	}
	var bs Header
	bs.Version, bs.MinorVersion = b[6], b[7]
	bl, prefix, err := headerExtent(b, true, messages.H5058)
	if err != nil {
		return Header{}, err
	}
	if len(b) < prefix+bl {
		return Header{}, formatError(messages.H5059, prefix+bl, len(b))
	}
	text := b[prefix : prefix+bl]
	if bs.Version < 3 {
		for _, c := range text {
			if c >= 0x80 {
				return Header{}, formatError(messages.H5060, bs.Version)
			}
		}
	}
	dict, err := parseDict(string(text))
	if err != nil {
		return Header{}, formatError(messages.H5061, err)
	}
	if err := fillHeader(&bs, dict); err != nil {
		return Header{}, err
	}
	bs.DataOffset = int64(prefix + bl)
	return bs, nil
}

func fillHeader(bs *Header, s map[string]any) error {
	for k := range s {
		switch k {
		case "descr", "fortran_order", "shape":
		default:
			return formatError(messages.H5062, k)
		}
	}
	d, ok := s["descr"]
	if !ok {
		return formatError(messages.H5063)
	}
	ds, ok := d.(string)
	if !ok {
		return formatError(messages.H5064)
	}
	dtype, err := parseDType(ds)
	if err != nil {
		return &FormatError{Reason: err}
	}
	bs.DType = dtype

	f, ok := s["fortran_order"]
	if !ok {
		return formatError(messages.H5065)
	}
	fb, ok := f.(bool)
	if !ok {
		return formatError(messages.H5066)
	}
	if fb {
		return formatError(messages.H5067)
	}

	sh, ok := s["shape"]
	if !ok {
		return formatError(messages.H5068)
	}
	dims, ok := sh.([]int64)
	if !ok {
		return formatError(messages.H5069)
	}
	if len(dims) != 1 {
		return formatError(messages.H5070, len(dims))
	}
	if dims[0] < 0 {
		return formatError(messages.H5071, dims[0])
	}
	if dims[0] > (1<<62)/int64(dtype.Size()) {
		return formatError(messages.H5072, dims[0])
	}
	bs.Length = dims[0]
	return nil
}

// ---------------------------------------------------------------------------
// Başlık sözlüğü için küçük bir sözdizimi çözümleyici. Biçim, tırnaklı
// anahtarlı bir sözlük değişmezidir:
// {'descr': '<u2', 'fortran_order': False, 'shape': (N,), }
// ---------------------------------------------------------------------------

type dictParser struct {
	s string
	i int
}

func parseDict(s string) (map[string]any, error) {
	a := &dictParser{s: s}
	a.pad()
	if !a.al('{') {
		return nil, messages.New(messages.H5031)
	}
	result := map[string]any{}
	for {
		a.pad()
		if a.al('}') {
			break
		}
		a.pad()
		key, err := a.text()
		if err != nil {
			return nil, messages.New(messages.H5032, err)
		}
		a.pad()
		if !a.al(':') {
			return nil, messages.New(messages.H5033, key)
		}
		a.pad()
		d, err := a.value()
		if err != nil {
			return nil, messages.New(messages.H5034, key, err)
		}
		if _, var_ := result[key]; var_ {
			return nil, messages.New(messages.H5035, key)
		}
		result[key] = d
		a.pad()
		if a.al(',') {
			continue
		}
		a.pad()
		if a.al('}') {
			break
		}
		return nil, messages.New(messages.H5036, a.i)
	}
	a.pad()
	if a.i != len(a.s) {
		return nil, messages.New(messages.H5037)
	}
	return result, nil
}

func (a *dictParser) pad() {
	for a.i < len(a.s) {
		switch a.s[a.i] {
		case ' ', '\t', '\n', '\r', '\x00':
			a.i++
		default:
			return
		}
	}
}

func (a *dictParser) al(c byte) bool {
	if a.i < len(a.s) && a.s[a.i] == c {
		a.i++
		return true
	}
	return false
}

func (a *dictParser) text() (string, error) {
	if a.i >= len(a.s) {
		return "", messages.New(messages.H5038)
	}
	quote := a.s[a.i]
	if quote != '\'' && quote != '"' {
		return "", messages.New(messages.H5039, a.i)
	}
	j := strings.IndexByte(a.s[a.i+1:], quote)
	if j < 0 {
		return "", messages.New(messages.H5040)
	}
	m := a.s[a.i+1 : a.i+1+j]
	if strings.IndexByte(m, '\\') >= 0 {
		return "", messages.New(messages.H5041)
	}
	a.i += j + 2
	return m, nil
}

func (a *dictParser) value() (any, error) {
	if a.i >= len(a.s) {
		return nil, messages.New(messages.H5042)
	}
	switch c := a.s[a.i]; {
	case c == '\'' || c == '"':
		return a.text()
	case c == '(':
		a.i++
		var bundle []int64
		for {
			a.pad()
			if a.al(')') {
				return bundle, nil
			}
			n, err := a.integer()
			if err != nil {
				return nil, err
			}
			bundle = append(bundle, n)
			a.pad()
			if a.al(',') {
				continue
			}
			if a.al(')') {
				return bundle, nil
			}
			return nil, messages.New(messages.H5043)
		}
	case c == '[' || c == '{':
		return nil, messages.New(messages.H5044)
	case c >= '0' && c <= '9' || c == '-':
		return a.integer()
	default:
		j := a.i
		for j < len(a.s) && (a.s[j] >= 'A' && a.s[j] <= 'Z' || a.s[j] >= 'a' && a.s[j] <= 'z') {
			j++
		}
		ad := a.s[a.i:j]
		a.i = j
		switch ad {
		case "True":
			return true, nil
		case "False":
			return false, nil
		case "None":
			return nil, nil
		}
		return nil, messages.New(messages.H5045, ad)
	}
}

func (a *dictParser) integer() (int64, error) {
	j := a.i
	if j < len(a.s) && a.s[j] == '-' {
		j++
	}
	for j < len(a.s) && a.s[j] >= '0' && a.s[j] <= '9' {
		j++
	}
	n, err := strconv.ParseInt(a.s[a.i:j], 10, 64)
	if err != nil {
		return 0, messages.New(messages.H5046, a.s[a.i:j])
	}
	a.i = j
	// Eski yazıcıların uzun tamsayı soneki.
	if a.i < len(a.s) && (a.s[a.i] == 'L' || a.s[a.i] == 'l') {
		a.i++
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Yazma
// ---------------------------------------------------------------------------

// WriteHeader, uzunluk elemanlı tek boyutlu bir dizi için .npy başlığını w'ye
// yazar. surum 1 ya da 2 olmalıdır. Başlık toplamı 64 bayta hizalanır.
func WriteHeader(w io.Writer, dtype DType, length int64, version int) error {
	if dtype.Size() == 0 {
		return messages.New(messages.H5047)
	}
	if length < 0 {
		return messages.New(messages.H5048)
	}
	dict := fmt.Sprintf("{'descr': '%s', 'fortran_order': False, 'shape': (%d,), }", dtype, length)
	var prefix int
	switch version {
	case 1:
		prefix = headerPrefixV1
	case 2:
		prefix = headerPrefixV2
	default:
		return messages.New(messages.H5049, version)
	}
	total := prefix + len(dict) + 1
	padding := (npyAlign - total%npyAlign) % npyAlign
	bl := len(dict) + padding + 1
	if version == 1 && bl > 0xFFFF {
		return messages.New(messages.H5050)
	}
	var buf bytes.Buffer
	buf.WriteString(npyMagic)
	buf.WriteByte(byte(version))
	buf.WriteByte(0)
	if version == 1 {
		_ = binary.Write(&buf, binary.LittleEndian, uint16(bl))
	} else {
		_ = binary.Write(&buf, binary.LittleEndian, uint32(bl))
	}
	buf.WriteString(dict)
	buf.WriteString(strings.Repeat(" ", padding))
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

// WriteNpyU16, veriyi '<u2' türünde sürüm 1 .npy olarak w'ye yazar.
func WriteNpyU16(w io.Writer, data []uint16) error {
	if err := WriteHeader(w, DTypeU16, int64(len(data)), 1); err != nil {
		return err
	}
	b := make([]byte, 2*len(data))
	for i, v := range data {
		binary.LittleEndian.PutUint16(b[2*i:], v)
	}
	_, err := w.Write(b)
	return err
}

// WriteNpyI32, veriyi '<i4' türünde sürüm 1 .npy olarak w'ye yazar.
func WriteNpyI32(w io.Writer, data []int32) error {
	if err := WriteHeader(w, DTypeI32, int64(len(data)), 1); err != nil {
		return err
	}
	b := make([]byte, 4*len(data))
	for i, v := range data {
		binary.LittleEndian.PutUint32(b[4*i:], uint32(v))
	}
	_, err := w.Write(b)
	return err
}

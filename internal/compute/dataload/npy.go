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
)

// Tur, .npy dosyasındaki elemanların türüdür.
type DType uint8

const (
	// TurGecersiz tanınmayan türdür.
	DTypeInvalid DType = iota
	// TurU16 küçük uçlu işaretsiz 16 bit ('<u2').
	DTypeU16
	// TurU32 küçük uçlu işaretsiz 32 bit ('<u4').
	DTypeU32
	// TurI32 küçük uçlu işaretli 32 bit ('<i4').
	DTypeI32
)

// Boyut, bir elemanın bayt cinsinden boyutudur.
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
	return "geçersiz"
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
		return DTypeInvalid, fmt.Errorf("büyük uçlu tür %q desteklenmiyor (yalnızca küçük uçlu '<u2', '<u4', '<i4')", descr)
	}
	return DTypeInvalid, fmt.Errorf("desteklenmeyen eleman türü %q (yalnızca '<u2', '<u4', '<i4')", descr)
}

// Baslik, çözümlenmiş bir .npy başlığıdır.
type Header struct {
	Version      uint8 // ana sürüm: 1, 2 ya da 3
	MinorVersion uint8
	DType        DType
	Length       int64 // eleman (token) sayısı
	DataOffset   int64 // verinin dosya başından itibaren bayt konumu
}

// VeriBoyutu, veri bölümünün bayt cinsinden boyutudur.
func (b Header) DataSize() int64 { return b.Length * int64(b.DType.Size()) }

// BicimHatasi, geçersiz ya da desteklenmeyen bir .npy başlığını bildirir.
type FormatError struct {
	Path   string // biliniyorsa dosya yolu
	Reason string
}

func (h *FormatError) Error() string {
	if h.Path != "" {
		return fmt.Sprintf("veri: %s: geçersiz .npy dosyası: %s", h.Path, h.Reason)
	}
	return "veri: geçersiz .npy dosyası: " + h.Reason
}

func formatError(format string, a ...any) error {
	return &FormatError{Reason: fmt.Sprintf(format, a...)}
}

const (
	npyMagic       = "\x93NUMPY"
	maxHeader      = 1 << 20 // makul olmayan büyüklükteki başlıkları reddet
	npyAlign       = 64
	headerPrefixV1 = 10 // büyü(6) + sürüm(2) + uzunluk(2)
	headerPrefixV2 = 12 // büyü(6) + sürüm(2) + uzunluk(4)
	minFileStart   = headerPrefixV1
)

// BaslikOku, r'nin başından .npy başlığını okur ve çözümler. Yalnızca
// başlık baytları okunur; veri bölümüne dokunulmaz.
func ReadHeader(r io.ReaderAt) (Header, error) {
	var on [headerPrefixV2]byte
	n, err := r.ReadAt(on[:], 0)
	if n < minFileStart {
		if err == nil || errors.Is(err, io.EOF) {
			return Header{}, formatError("dosya çok kısa (%d bayt), .npy başlığı yok", n)
		}
		return Header{}, fmt.Errorf("veri: başlık okunamadı: %w", err)
	}
	base := on[6]
	var bl, prefix int
	switch base {
	case 1:
		bl, prefix = int(binary.LittleEndian.Uint16(on[8:10])), headerPrefixV1
	case 2, 3:
		if n < headerPrefixV2 {
			return Header{}, formatError("dosya çok kısa, sürüm %d başlığı eksik", base)
		}
		bl, prefix = int(binary.LittleEndian.Uint32(on[8:12])), headerPrefixV2
	default:
		// Önce büyü baytlarını denetle ki yanlış dosyada daha anlamlı hata verelim.
		if string(on[:6]) != npyMagic {
			return Header{}, formatError("büyü baytları eksik (dosya .npy değil)")
		}
		return Header{}, formatError("desteklenmeyen sürüm %d.%d (yalnızca 1.x, 2.x, 3.x)", base, on[7])
	}
	if bl > maxHeader {
		return Header{}, formatError("başlık uzunluğu çok büyük (%d bayt)", bl)
	}
	exact := make([]byte, prefix+bl)
	copy(exact, on[:n])
	if prefix+bl > n {
		m, err := r.ReadAt(exact[n:], int64(n))
		if m < len(exact)-n {
			if err == nil || errors.Is(err, io.EOF) {
				return Header{}, formatError("başlık kesik: %d bayt beklendi", prefix+bl)
			}
			return Header{}, fmt.Errorf("veri: başlık okunamadı: %w", err)
		}
	}
	return ParseHeader(exact)
}

// BaslikCoz, bellekteki baytlardan (en az tüm başlık) .npy başlığını çözümler.
func ParseHeader(b []byte) (Header, error) {
	if len(b) < minFileStart {
		return Header{}, formatError("başlık çok kısa (%d bayt)", len(b))
	}
	if string(b[:6]) != npyMagic {
		return Header{}, formatError("büyü baytları eksik (dosya .npy değil)")
	}
	var bs Header
	bs.Version, bs.MinorVersion = b[6], b[7]
	var bl, prefix int
	switch bs.Version {
	case 1:
		bl, prefix = int(binary.LittleEndian.Uint16(b[8:10])), headerPrefixV1
	case 2, 3:
		if len(b) < headerPrefixV2 {
			return Header{}, formatError("sürüm %d başlığı eksik", bs.Version)
		}
		bl, prefix = int(binary.LittleEndian.Uint32(b[8:12])), headerPrefixV2
	default:
		return Header{}, formatError("desteklenmeyen sürüm %d.%d (yalnızca 1.x, 2.x, 3.x)", bs.Version, bs.MinorVersion)
	}
	if bl > maxHeader {
		return Header{}, formatError("başlık uzunluğu çok büyük (%d bayt)", bl)
	}
	if len(b) < prefix+bl {
		return Header{}, formatError("başlık kesik: %d bayt beklendi, %d var", prefix+bl, len(b))
	}
	text := b[prefix : prefix+bl]
	if bs.Version < 3 {
		for _, c := range text {
			if c >= 0x80 {
				return Header{}, formatError("sürüm %d başlığında ASCII dışı bayt", bs.Version)
			}
		}
	}
	dict, err := parseDict(string(text))
	if err != nil {
		return Header{}, formatError("başlık sözlüğü çözümlenemedi: %v", err)
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
			return formatError("başlıkta beklenmeyen anahtar %q", k)
		}
	}
	d, ok := s["descr"]
	if !ok {
		return formatError("başlıkta 'descr' anahtarı yok")
	}
	ds, ok := d.(string)
	if !ok {
		return formatError("'descr' bir metin değil (yapılandırılmış türler desteklenmiyor)")
	}
	dtype, err := parseDType(ds)
	if err != nil {
		return formatError("%v", err)
	}
	bs.DType = dtype

	f, ok := s["fortran_order"]
	if !ok {
		return formatError("başlıkta 'fortran_order' anahtarı yok")
	}
	fb, ok := f.(bool)
	if !ok {
		return formatError("'fortran_order' True/False değil")
	}
	if fb {
		return formatError("sütun öncelikli diziler desteklenmiyor (yalnızca satır öncelikli)")
	}

	sh, ok := s["shape"]
	if !ok {
		return formatError("başlıkta 'shape' anahtarı yok")
	}
	dims, ok := sh.([]int64)
	if !ok {
		return formatError("'shape' bir tamsayı demeti değil")
	}
	if len(dims) != 1 {
		return formatError("yalnızca tek boyutlu diziler destekleniyor, şekil %d boyutlu", len(dims))
	}
	if dims[0] < 0 {
		return formatError("negatif uzunluk %d", dims[0])
	}
	if dims[0] > (1<<62)/int64(dtype.Size()) {
		return formatError("uzunluk çok büyük (%d)", dims[0])
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
		return nil, fmt.Errorf("'{' bekleniyordu")
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
			return nil, fmt.Errorf("anahtar: %w", err)
		}
		a.pad()
		if !a.al(':') {
			return nil, fmt.Errorf("%q anahtarından sonra ':' bekleniyordu", key)
		}
		a.pad()
		d, err := a.value()
		if err != nil {
			return nil, fmt.Errorf("%q değeri: %w", key, err)
		}
		if _, var_ := result[key]; var_ {
			return nil, fmt.Errorf("%q anahtarı iki kez geçiyor", key)
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
		return nil, fmt.Errorf("konum %d: ',' ya da '}' bekleniyordu", a.i)
	}
	a.pad()
	if a.i != len(a.s) {
		return nil, fmt.Errorf("sözlükten sonra fazladan karakter")
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
		return "", fmt.Errorf("metin bekleniyordu, başlık bitti")
	}
	quote := a.s[a.i]
	if quote != '\'' && quote != '"' {
		return "", fmt.Errorf("konum %d: tırnaklı metin bekleniyordu", a.i)
	}
	j := strings.IndexByte(a.s[a.i+1:], quote)
	if j < 0 {
		return "", fmt.Errorf("kapanmayan tırnak")
	}
	m := a.s[a.i+1 : a.i+1+j]
	if strings.IndexByte(m, '\\') >= 0 {
		return "", fmt.Errorf("kaçış dizili metin desteklenmiyor")
	}
	a.i += j + 2
	return m, nil
}

func (a *dictParser) value() (any, error) {
	if a.i >= len(a.s) {
		return nil, fmt.Errorf("değer bekleniyordu, başlık bitti")
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
			return nil, fmt.Errorf("demette ',' ya da ')' bekleniyordu")
		}
	case c == '[' || c == '{':
		return nil, fmt.Errorf("iç içe liste/sözlük desteklenmiyor")
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
		return nil, fmt.Errorf("tanınmayan değer %q", ad)
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
		return 0, fmt.Errorf("geçersiz tamsayı %q", a.s[a.i:j])
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

// BaslikYaz, uzunluk elemanlı tek boyutlu bir dizi için .npy başlığını w'ye
// yazar. surum 1 ya da 2 olmalıdır. Başlık toplamı 64 bayta hizalanır.
func WriteHeader(w io.Writer, dtype DType, length int64, version int) error {
	if dtype.Size() == 0 {
		return fmt.Errorf("veri: geçersiz tür")
	}
	if length < 0 {
		return fmt.Errorf("veri: negatif uzunluk")
	}
	dict := fmt.Sprintf("{'descr': '%s', 'fortran_order': False, 'shape': (%d,), }", dtype, length)
	var prefix int
	switch version {
	case 1:
		prefix = headerPrefixV1
	case 2:
		prefix = headerPrefixV2
	default:
		return fmt.Errorf("veri: yazma için desteklenmeyen sürüm %d", version)
	}
	total := prefix + len(dict) + 1
	padding := (npyAlign - total%npyAlign) % npyAlign
	bl := len(dict) + padding + 1
	if version == 1 && bl > 0xFFFF {
		return fmt.Errorf("veri: başlık sürüm 1 için çok uzun")
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

// NpyYazU16, veriyi '<u2' türünde sürüm 1 .npy olarak w'ye yazar.
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

// NpyYazI32, veriyi '<i4' türünde sürüm 1 .npy olarak w'ye yazar.
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

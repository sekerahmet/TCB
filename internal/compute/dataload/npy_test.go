package dataload

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// manualHeader, verilen sözlük metniyle ham bir başlık üretir (dolgu ve
// satır sonu dahil, 64 bayta hizalı).
func manualHeader(version int, dict string) []byte {
	prefix := 10
	if version >= 2 {
		prefix = 12
	}
	total := prefix + len(dict) + 1
	padding := (64 - total%64) % 64
	text := dict + strings.Repeat(" ", padding) + "\n"
	var b bytes.Buffer
	b.WriteString("\x93NUMPY")
	b.WriteByte(byte(version))
	b.WriteByte(0)
	if version == 1 {
		_ = binary.Write(&b, binary.LittleEndian, uint16(len(text)))
	} else {
		_ = binary.Write(&b, binary.LittleEndian, uint32(len(text)))
	}
	b.WriteString(text)
	return b.Bytes()
}

func TestHeaderVersions(t *testing.T) {
	cases := []struct {
		version int
		dict    string
		dtype   DType
		n       int64
	}{
		{1, "{'descr': '<u2', 'fortran_order': False, 'shape': (250000000,), }", DTypeU16, 250000000},
		{2, "{'descr': '<u4', 'fortran_order': False, 'shape': (7,), }", DTypeU32, 7},
		{3, "{'descr': '<i4', 'fortran_order': False, 'shape': (0,), }", DTypeI32, 0},
		{1, `{"shape": (12L,), "descr": "<u2", "fortran_order": False}`, DTypeU16, 12},
		{1, "{'descr':'<u2','fortran_order':False,'shape':(5,)}", DTypeU16, 5},
	}
	for _, d := range cases {
		raw := manualHeader(d.version, d.dict)
		bs, err := ParseHeader(raw)
		if err != nil {
			t.Fatalf("%q: %v", d.dict, err)
		}
		if int(bs.Version) != d.version || bs.DType != d.dtype || bs.Length != d.n || bs.DataOffset != int64(len(raw)) {
			t.Fatalf("%q: yanlış başlık %+v", d.dict, bs)
		}
		// ReaderAt yolu da aynı sonucu vermeli.
		bs2, err := ReadHeader(bytes.NewReader(append(raw, make([]byte, 16)...)))
		if err != nil || bs2 != bs {
			t.Fatalf("BaslikOku farklı: %+v %v", bs2, err)
		}
	}
}

func TestWriteHeaderRoundTrip(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, dtype := range []DType{DTypeU16, DTypeU32, DTypeI32} {
			var b bytes.Buffer
			if err := WriteHeader(&b, dtype, 123456789, version); err != nil {
				t.Fatal(err)
			}
			if b.Len()%64 != 0 {
				t.Fatalf("başlık 64 bayta hizalı değil: %d", b.Len())
			}
			bs, err := ParseHeader(b.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if bs.DType != dtype || bs.Length != 123456789 || int(bs.Version) != version || bs.DataOffset != int64(b.Len()) {
				t.Fatalf("gidiş-dönüş bozuk: %+v", bs)
			}
		}
	}
}

func TestHeaderErrors(t *testing.T) {
	cases := []struct {
		ad   string
		raw  []byte
		code messages.Code
	}{
		{"büyü", append([]byte("XNUMPY\x01\x00\x10\x00"), make([]byte, 16)...), messages.H5053},
		{"boş", nil, messages.H5057},
		{"büyük uçlu", manualHeader(1, "{'descr': '>u2', 'fortran_order': False, 'shape': (5,), }"), messages.H5028},
		{"tür", manualHeader(1, "{'descr': '<f4', 'fortran_order': False, 'shape': (5,), }"), messages.H5029},
		{"iki boyut", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5, 3), }"), messages.H5070},
		{"skaler", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (), }"), messages.H5070},
		{"fortran", manualHeader(1, "{'descr': '<u2', 'fortran_order': True, 'shape': (5,), }"), messages.H5067},
		{"eksik anahtar", manualHeader(1, "{'descr': '<u2', 'shape': (5,), }"), messages.H5065},
		{"fazla anahtar", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), 'x': 1}"), messages.H5062},
		{"yapılı tür", manualHeader(1, "{'descr': [('a', '<u2')], 'fortran_order': False, 'shape': (5,), }"), messages.H5044},
		{"bozuk sözlük", manualHeader(1, "{'descr' '<u2'}"), messages.H5033},
		{"kesik", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), }")[:30], messages.H5059},
	}
	for _, d := range cases {
		_, err := ParseHeader(d.raw)
		if err == nil {
			t.Fatalf("%s: hata bekleniyordu", d.ad)
		}
		var bh *FormatError
		if !errors.As(err, &bh) {
			t.Fatalf("%s: BicimHatasi bekleniyordu: %T %v", d.ad, err, err)
		}
		if !messages.Has(err, d.code) {
			t.Fatalf("%s: hata %q, %s kodunu içermeli", d.ad, err, d.code)
		}
	}
	// Desteklenmeyen sürüm.
	raw := manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), }")
	raw[6] = 9
	if _, err := ParseHeader(raw); !messages.Has(err, messages.H5054) {
		t.Fatalf("sürüm hatası bekleniyordu: %v", err)
	}
	if _, err := ReadHeader(bytes.NewReader(raw)); !messages.Has(err, messages.H5054) {
		t.Fatalf("BaslikOku sürüm hatası bekleniyordu: %v", err)
	}
}

// Kural: ReadHeader ve ParseHeader başlık ön ekini aynı yardımcıyla çözer ama hata sırası
// korunur: ReadHeader sürümü büyüden önce denetler (büyüsü yanlış ama sürümü geçerli bir dosyada
// kısa ön ek H5052 verir; bilinmeyen sürümde büyü yanlışsa H5053), ParseHeader büyüyü önce
// denetler (H5053) ve kısa ön ekte H5058 verir.
func TestHeaderPrefixErrorOrder(t *testing.T) {
	wrongMagic := func(version byte, n int) []byte {
		b := make([]byte, n)
		copy(b, "XNUMPY")
		b[6] = version
		return b
	}
	for _, c := range []struct {
		name         string
		b            []byte
		read, parsed messages.Code
	}{
		{"büyü yanlış, sürüm 2, kısa ön ek", wrongMagic(2, 10), messages.H5052, messages.H5053},
		{"büyü yanlış, bilinmeyen sürüm", wrongMagic(9, 12), messages.H5053, messages.H5053},
		{"büyü doğru, bilinmeyen sürüm", append([]byte(npyMagic), 9, 0, 0, 0, 0, 0), messages.H5054, messages.H5054},
		{"büyü doğru, sürüm 2, kısa ön ek", append([]byte(npyMagic), 2, 0, 0, 0), messages.H5052, messages.H5058},
	} {
		if _, err := ReadHeader(bytes.NewReader(c.b)); !messages.Has(err, c.read) {
			t.Errorf("%s: ReadHeader %v, beklenen %s", c.name, err, c.read)
		}
		if _, err := ParseHeader(c.b); !messages.Has(err, c.parsed) {
			t.Errorf("%s: ParseHeader %v, beklenen %s", c.name, err, c.parsed)
		}
	}
}

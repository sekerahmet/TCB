package dataload

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// elleBaslik, verilen sözlük metniyle ham bir başlık üretir (dolgu ve
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
		ad       string
		raw      []byte
		contains string
	}{
		{"büyü", append([]byte("XNUMPY\x01\x00\x10\x00"), make([]byte, 16)...), "büyü"},
		{"boş", nil, "kısa"},
		{"büyük uçlu", manualHeader(1, "{'descr': '>u2', 'fortran_order': False, 'shape': (5,), }"), "büyük uçlu"},
		{"tür", manualHeader(1, "{'descr': '<f4', 'fortran_order': False, 'shape': (5,), }"), "desteklenmeyen eleman türü"},
		{"iki boyut", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5, 3), }"), "tek boyutlu"},
		{"skaler", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (), }"), "tek boyutlu"},
		{"fortran", manualHeader(1, "{'descr': '<u2', 'fortran_order': True, 'shape': (5,), }"), "sütun öncelikli"},
		{"eksik anahtar", manualHeader(1, "{'descr': '<u2', 'shape': (5,), }"), "fortran_order"},
		{"fazla anahtar", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), 'x': 1}"), "beklenmeyen anahtar"},
		{"yapılı tür", manualHeader(1, "{'descr': [('a', '<u2')], 'fortran_order': False, 'shape': (5,), }"), "iç içe"},
		{"bozuk sözlük", manualHeader(1, "{'descr' '<u2'}"), "':'"},
		{"kesik", manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), }")[:30], "kesik"},
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
		if !strings.Contains(err.Error(), d.contains) {
			t.Fatalf("%s: hata %q, %q içermeli", d.ad, err, d.contains)
		}
	}
	// Desteklenmeyen sürüm.
	raw := manualHeader(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), }")
	raw[6] = 9
	if _, err := ParseHeader(raw); err == nil || !strings.Contains(err.Error(), "sürüm 9") {
		t.Fatalf("sürüm hatası bekleniyordu: %v", err)
	}
	if _, err := ReadHeader(bytes.NewReader(raw)); err == nil || !strings.Contains(err.Error(), "sürüm 9") {
		t.Fatalf("BaslikOku sürüm hatası bekleniyordu: %v", err)
	}
}

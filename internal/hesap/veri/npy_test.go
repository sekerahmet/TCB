package veri

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// elleBaslik, verilen sözlük metniyle ham bir başlık üretir (dolgu ve
// satır sonu dahil, 64 bayta hizalı).
func elleBaslik(surum int, sozluk string) []byte {
	onEk := 10
	if surum >= 2 {
		onEk = 12
	}
	toplam := onEk + len(sozluk) + 1
	dolgu := (64 - toplam%64) % 64
	metin := sozluk + strings.Repeat(" ", dolgu) + "\n"
	var b bytes.Buffer
	b.WriteString("\x93NUMPY")
	b.WriteByte(byte(surum))
	b.WriteByte(0)
	if surum == 1 {
		_ = binary.Write(&b, binary.LittleEndian, uint16(len(metin)))
	} else {
		_ = binary.Write(&b, binary.LittleEndian, uint32(len(metin)))
	}
	b.WriteString(metin)
	return b.Bytes()
}

func TestBaslikSurumler(t *testing.T) {
	durumlar := []struct {
		surum  int
		sozluk string
		tur    Tur
		n      int64
	}{
		{1, "{'descr': '<u2', 'fortran_order': False, 'shape': (250000000,), }", TurU16, 250000000},
		{2, "{'descr': '<u4', 'fortran_order': False, 'shape': (7,), }", TurU32, 7},
		{3, "{'descr': '<i4', 'fortran_order': False, 'shape': (0,), }", TurI32, 0},
		{1, `{"shape": (12L,), "descr": "<u2", "fortran_order": False}`, TurU16, 12},
		{1, "{'descr':'<u2','fortran_order':False,'shape':(5,)}", TurU16, 5},
	}
	for _, d := range durumlar {
		ham := elleBaslik(d.surum, d.sozluk)
		bs, err := BaslikCoz(ham)
		if err != nil {
			t.Fatalf("%q: %v", d.sozluk, err)
		}
		if int(bs.Surum) != d.surum || bs.Tur != d.tur || bs.Uzunluk != d.n || bs.VeriOfseti != int64(len(ham)) {
			t.Fatalf("%q: yanlış başlık %+v", d.sozluk, bs)
		}
		// ReaderAt yolu da aynı sonucu vermeli.
		bs2, err := BaslikOku(bytes.NewReader(append(ham, make([]byte, 16)...)))
		if err != nil || bs2 != bs {
			t.Fatalf("BaslikOku farklı: %+v %v", bs2, err)
		}
	}
}

func TestBaslikYazGidisDonus(t *testing.T) {
	for _, surum := range []int{1, 2} {
		for _, tur := range []Tur{TurU16, TurU32, TurI32} {
			var b bytes.Buffer
			if err := BaslikYaz(&b, tur, 123456789, surum); err != nil {
				t.Fatal(err)
			}
			if b.Len()%64 != 0 {
				t.Fatalf("başlık 64 bayta hizalı değil: %d", b.Len())
			}
			bs, err := BaslikCoz(b.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if bs.Tur != tur || bs.Uzunluk != 123456789 || int(bs.Surum) != surum || bs.VeriOfseti != int64(b.Len()) {
				t.Fatalf("gidiş-dönüş bozuk: %+v", bs)
			}
		}
	}
}

func TestBaslikHatalari(t *testing.T) {
	durumlar := []struct {
		ad     string
		ham    []byte
		icerir string
	}{
		{"büyü", append([]byte("XNUMPY\x01\x00\x10\x00"), make([]byte, 16)...), "büyü"},
		{"boş", nil, "kısa"},
		{"büyük uçlu", elleBaslik(1, "{'descr': '>u2', 'fortran_order': False, 'shape': (5,), }"), "büyük uçlu"},
		{"tür", elleBaslik(1, "{'descr': '<f4', 'fortran_order': False, 'shape': (5,), }"), "desteklenmeyen eleman türü"},
		{"iki boyut", elleBaslik(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5, 3), }"), "tek boyutlu"},
		{"skaler", elleBaslik(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (), }"), "tek boyutlu"},
		{"fortran", elleBaslik(1, "{'descr': '<u2', 'fortran_order': True, 'shape': (5,), }"), "Fortran"},
		{"eksik anahtar", elleBaslik(1, "{'descr': '<u2', 'shape': (5,), }"), "fortran_order"},
		{"fazla anahtar", elleBaslik(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), 'x': 1}"), "beklenmeyen anahtar"},
		{"yapılı tür", elleBaslik(1, "{'descr': [('a', '<u2')], 'fortran_order': False, 'shape': (5,), }"), "iç içe"},
		{"bozuk sözlük", elleBaslik(1, "{'descr' '<u2'}"), "':'"},
		{"kesik", elleBaslik(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), }")[:30], "kesik"},
	}
	for _, d := range durumlar {
		_, err := BaslikCoz(d.ham)
		if err == nil {
			t.Fatalf("%s: hata bekleniyordu", d.ad)
		}
		var bh *BicimHatasi
		if !errors.As(err, &bh) {
			t.Fatalf("%s: BicimHatasi bekleniyordu: %T %v", d.ad, err, err)
		}
		if !strings.Contains(err.Error(), d.icerir) {
			t.Fatalf("%s: hata %q, %q içermeli", d.ad, err, d.icerir)
		}
	}
	// Desteklenmeyen sürüm.
	ham := elleBaslik(1, "{'descr': '<u2', 'fortran_order': False, 'shape': (5,), }")
	ham[6] = 9
	if _, err := BaslikCoz(ham); err == nil || !strings.Contains(err.Error(), "sürüm 9") {
		t.Fatalf("sürüm hatası bekleniyordu: %v", err)
	}
	if _, err := BaslikOku(bytes.NewReader(ham)); err == nil || !strings.Contains(err.Error(), "sürüm 9") {
		t.Fatalf("BaslikOku sürüm hatası bekleniyordu: %v", err)
	}
}

func TestParcaKesikVeri(t *testing.T) {
	d := t.TempDir()
	var b bytes.Buffer
	if err := BaslikYaz(&b, TurU16, 100, 1); err != nil {
		t.Fatal(err)
	}
	b.Write(make([]byte, 150)) // 200 bayt gerekli
	yol := filepath.Join(d, "kesik.npy")
	if err := os.WriteFile(yol, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParcaAc(yol, ParcaSecenekleri{})
	if err == nil || !strings.Contains(err.Error(), "kesik") || !strings.Contains(err.Error(), "kesik.npy") {
		t.Fatalf("kesik dosya hatası bekleniyordu: %v", err)
	}
	// Hiç .npy olmayan dosya.
	yol2 := filepath.Join(d, "metin.npy")
	_ = os.WriteFile(yol2, []byte("merhaba dünya, bu bir npy değil"), 0o644)
	if _, err := ParcaAc(yol2, ParcaSecenekleri{}); err == nil || !strings.Contains(err.Error(), "büyü") {
		t.Fatalf("büyü hatası bekleniyordu: %v", err)
	}
	// Boş dosya.
	yol3 := filepath.Join(d, "bos.npy")
	_ = os.WriteFile(yol3, nil, 0o644)
	if _, err := ParcaAc(yol3, ParcaSecenekleri{}); err == nil || !strings.Contains(err.Error(), "kısa") {
		t.Fatalf("kısa dosya hatası bekleniyordu: %v", err)
	}
}

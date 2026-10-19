package dataload

import (
	"bytes"
	"testing"
)

// FuzzNpyBaslik: .npy veri dosyaları güvenilmeyen girdidir (indirilen veri kümesi).
// Başlık çözücü hiçbir baytta çökmemeli; dosyadan (ReadAt) ve bellekten okuma aynı
// sonucu vermeli; kabul edilen başlığın veri konumu ve boyu taşmasız olmalı ve
// bildirilen veri dosyaya sığıyorsa her eleman çözülebilmelidir. Derin arama:
//
//	go test ./internal/compute/dataload -run '^$' -fuzz FuzzNpyBaslik -fuzztime 5m
func FuzzNpyHeader(f *testing.F) {
	for _, dtype := range []DType{DTypeU16, DTypeU32, DTypeI32} {
		for _, version := range []int{1, 2} {
			var b bytes.Buffer
			if err := WriteHeader(&b, dtype, 3, version); err != nil {
				f.Fatal(err)
			}
			b.Write(make([]byte, 3*dtype.Size()))
			f.Add(b.Bytes())
		}
	}
	f.Add([]byte("\x93NUMPY\x03\x00\x20\x00\x00\x00{'descr': '<u2', 'shape': (5L,), }"))
	f.Add([]byte("\x93NUMPY\x01\x00\x40\x00{'descr':'<i4','fortran_order':False,'shape':(9223372036854775807,)}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		bs, err := ReadHeader(bytes.NewReader(data))
		bs2, err2 := ParseHeader(data)
		if (err == nil) != (err2 == nil) || bs != bs2 {
			t.Fatalf("dosyadan ve bellekten okuma farklı: %+v %v / %+v %v", bs, err, bs2, err2)
		}
		if err != nil {
			return
		}
		if bs.DType.Size() == 0 || bs.Length < 0 || bs.DataOffset < 0 || bs.DataOffset > int64(len(data)) {
			t.Fatalf("geçersiz başlık kabul edildi: %+v (dosya %d bayt)", bs, len(data))
		}
		needed := bs.DataOffset + bs.DataSize()
		if needed < bs.DataOffset {
			t.Fatalf("veri boyu taştı: %+v", bs)
		}
		if needed > int64(len(data)) {
			return // ParcaAc bunu "dosya kesik" diye reddeder
		}
		dst := make([]int32, bs.Length)
		decodeI32(dst, data[bs.DataOffset:needed], bs.DType)
	})
}

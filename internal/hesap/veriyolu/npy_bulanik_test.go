package veriyolu

import (
	"bytes"
	"testing"
)

// FuzzNpyBaslik: .npy veri dosyaları güvenilmeyen girdidir (indirilen veri kümesi).
// Başlık çözücü hiçbir baytta çökmemeli; dosyadan (ReadAt) ve bellekten okuma aynı
// sonucu vermeli; kabul edilen başlığın veri konumu ve boyu taşmasız olmalı ve
// bildirilen veri dosyaya sığıyorsa her eleman çözülebilmelidir. Derin arama:
//
//	go test ./internal/hesap/veriyolu -run '^$' -fuzz FuzzNpyBaslik -fuzztime 5m
func FuzzNpyBaslik(f *testing.F) {
	for _, tur := range []Tur{TurU16, TurU32, TurI32} {
		for _, surum := range []int{1, 2} {
			var b bytes.Buffer
			if err := BaslikYaz(&b, tur, 3, surum); err != nil {
				f.Fatal(err)
			}
			b.Write(make([]byte, 3*tur.Boyut()))
			f.Add(b.Bytes())
		}
	}
	f.Add([]byte("\x93NUMPY\x03\x00\x20\x00\x00\x00{'descr': '<u2', 'shape': (5L,), }"))
	f.Add([]byte("\x93NUMPY\x01\x00\x40\x00{'descr':'<i4','fortran_order':False,'shape':(9223372036854775807,)}"))

	f.Fuzz(func(t *testing.T, veri []byte) {
		bs, err := BaslikOku(bytes.NewReader(veri))
		bs2, err2 := BaslikCoz(veri)
		if (err == nil) != (err2 == nil) || bs != bs2 {
			t.Fatalf("dosyadan ve bellekten okuma farklı: %+v %v / %+v %v", bs, err, bs2, err2)
		}
		if err != nil {
			return
		}
		if bs.Tur.Boyut() == 0 || bs.Uzunluk < 0 || bs.VeriOfseti < 0 || bs.VeriOfseti > int64(len(veri)) {
			t.Fatalf("geçersiz başlık kabul edildi: %+v (dosya %d bayt)", bs, len(veri))
		}
		gereken := bs.VeriOfseti + bs.VeriBoyutu()
		if gereken < bs.VeriOfseti {
			t.Fatalf("veri boyu taştı: %+v", bs)
		}
		if gereken > int64(len(veri)) {
			return // ParcaAc bunu "dosya kesik" diye reddeder
		}
		dst := make([]int32, bs.Uzunluk)
		cozI32(dst, veri[bs.VeriOfseti:gereken], bs.Tur)
	})
}

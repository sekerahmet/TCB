package ml

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// FuzzTensorKaydiCoz: tensör dosyası (.trt) güvenilmeyen girdidir (indirilen kontrol
// noktası). Çözücü hiçbir baytta çökmemeli, dosya boyunu aşan ayırma yapmamalı; kabul
// ettiği her dosyadan tensör kurulabilmeli ve yeniden yazılan dosya aynı biçimde
// okunabilmelidir. Normal `go test` yalnızca tohumları koşar; derin arama için:
//
//	go test ./internal/stdlib/ml -run '^$' -fuzz FuzzTensorKaydiCoz -fuzztime 5m
func FuzzTensorKaydiCoz(f *testing.F) {
	f.Add(kayitBaytlari(f, nil))
	f.Add(kayitBaytlari(f, []kayitGirdisi{{ad: "ağırlık", sekil: []int{2, 3}, tur: tensor.Float32, veri: []float32{1, 2, 3, 4, 5, 6}}}))
	f.Add(kayitBaytlari(f, []kayitGirdisi{
		{ad: "a", sekil: []int{}, tur: tensor.Float32, veri: []float32{7}},
		{ad: "b", sekil: []int{2}, tur: tensor.BFloat16, veri: []float32{1, -2}},
		{ad: "c", sekil: []int{0, 4}, tur: tensor.Float16, veri: []float32{}},
	}))
	// Sağlaması doğru ama girdi sayısı dev olan dosya.
	bozuk := []byte(dosyaSihirli)
	bozuk = binary.LittleEndian.AppendUint16(bozuk, dosyaSurumu)
	bozuk = binary.LittleEndian.AppendUint32(bozuk, 1<<31)
	bozuk = binary.LittleEndian.AppendUint32(bozuk, crc32.ChecksumIEEE(bozuk))
	f.Add(bozuk)

	f.Fuzz(func(t *testing.T, veri []byte) {
		girdiler, err := kayitCoz(bytes.NewReader(veri), int64(len(veri)))
		if err != nil {
			return
		}
		toplam := 0
		for _, g := range girdiler {
			n, sinir := 1, 1
			for _, d := range g.sekil {
				n *= d
				sinir *= max(d, 1)
			}
			if sinir > interp.MaxCollectionSize {
				t.Fatalf("%q: şekil %v eleman sınırını aşıyor (sıfır eksenler 1 sayılır)", g.ad, g.sekil)
			}
			if n != len(g.veri) {
				t.Fatalf("%q: şekil %v ile %d eleman tutmuyor", g.ad, g.sekil, len(g.veri))
			}
			toplam += len(g.veri)
			// tensör_yükle'nin yaptığı gibi tensör kurulur.
			var tt *tensor.Tensor
			if g.tur == tensor.Float32 {
				tt = tensor.New(g.veri, g.sekil...)
			} else {
				tt = tensor.NewOf(g.tur, g.veri, g.sekil...)
			}
			if tt.Numel() != len(g.veri) {
				t.Fatalf("%q: tensör %d eleman, veri %d", g.ad, tt.Numel(), len(g.veri))
			}
		}
		if toplam*2 > len(veri) {
			t.Fatalf("%d bayttan %d eleman okundu: veri dosyadan büyük", len(veri), toplam)
		}
		yeni := kayitBaytlari(t, girdiler)
		ikinci, err := kayitCoz(bytes.NewReader(yeni), int64(len(yeni)))
		if err != nil {
			t.Fatalf("yeniden yazılan dosya okunamadı: %v", err)
		}
		if len(ikinci) != len(girdiler) {
			t.Fatalf("yeniden okumada %d girdi, önce %d", len(ikinci), len(girdiler))
		}
	})
}

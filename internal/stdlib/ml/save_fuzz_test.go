package ml

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// FuzzTensorFileDecode: tensör dosyası (.trt) güvenilmeyen girdidir (indirilen kontrol
// noktası). Çözücü hiçbir baytta çökmemeli, dosya boyunu aşan ayırma yapmamalı; kabul
// ettiği her dosyadan tensör kurulabilmeli ve yeniden yazılan dosya aynı biçimde
// okunabilmelidir. Normal `go test` yalnızca tohumları koşar; derin arama için:
//
//	go test ./internal/stdlib/ml -run '^$' -fuzz FuzzTensorFileDecode -fuzztime 5m
func FuzzTensorFileDecode(f *testing.F) {
	f.Add(fileBytes(f, nil))
	f.Add(fileBytes(f, []fileEntry{{ad: "ağırlık", shape: []int{2, 3}, dtype: tensor.Float32, data: []float32{1, 2, 3, 4, 5, 6}}}))
	f.Add(fileBytes(f, []fileEntry{
		{ad: "a", shape: []int{}, dtype: tensor.Float32, data: []float32{7}},
		{ad: "b", shape: []int{2}, dtype: tensor.BFloat16, data: []float32{1, -2}},
		{ad: "c", shape: []int{0, 4}, dtype: tensor.Float16, data: []float32{}},
	}))
	// Sağlaması doğru ama girdi sayısı dev olan dosya.
	broken := []byte(fileMagic)
	broken = binary.LittleEndian.AppendUint16(broken, fileVersion)
	broken = binary.LittleEndian.AppendUint32(broken, 1<<31)
	broken = binary.LittleEndian.AppendUint32(broken, crc32.ChecksumIEEE(broken))
	f.Add(broken)

	f.Fuzz(func(t *testing.T, data []byte) {
		inputs, err := decodeFile(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		total := 0
		for _, g := range inputs {
			n, limit := 1, 1
			for _, d := range g.shape {
				n *= d
				limit *= max(d, 1)
			}
			if limit > interp.MaxCollectionSize {
				t.Fatalf("%q: şekil %v eleman sınırını aşıyor (sıfır eksenler 1 sayılır)", g.ad, g.shape)
			}
			if n != len(g.data) {
				t.Fatalf("%q: şekil %v ile %d eleman tutmuyor", g.ad, g.shape, len(g.data))
			}
			total += len(g.data)
			// tensör_yükle'nin yaptığı gibi tensör kurulur.
			var tt *tensor.Tensor
			if g.dtype == tensor.Float32 {
				tt = tensor.New(g.data, g.shape...)
			} else {
				tt = tensor.NewOf(g.dtype, g.data, g.shape...)
			}
			if tt.Numel() != len(g.data) {
				t.Fatalf("%q: tensör %d eleman, veri %d", g.ad, tt.Numel(), len(g.data))
			}
		}
		if total*2 > len(data) {
			t.Fatalf("%d bayttan %d eleman okundu: veri dosyadan büyük", len(data), total)
		}
		fresh := fileBytes(t, inputs)
		second, err := decodeFile(bytes.NewReader(fresh), int64(len(fresh)))
		if err != nil {
			t.Fatalf("yeniden yazılan dosya okunamadı: %v", err)
		}
		if len(second) != len(inputs) {
			t.Fatalf("yeniden okumada %d girdi, önce %d", len(second), len(inputs))
		}
	})
}

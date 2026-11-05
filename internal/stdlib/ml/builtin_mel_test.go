package ml

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// melFilterBound Whisper'ın dosyasıyla en büyük farkın sınırıdır (23.4); ölçülen 3,7e-9. Dosya float32 hesaplanmıştır, TR
// float64'te hesaplayıp bir kez yuvarlar.
const melFilterBound = 1e-8

// npyFloats npz içindeki bir .npy dosyasının float32 öğeleridir.
func npyFloats(t *testing.T, z *zip.Reader, name string) []float32 {
	t.Helper()
	f, err := z.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("\x93NUMPY\x01")) {
		t.Fatalf("%s: npy değil", name)
	}
	body := raw[10+int(binary.LittleEndian.Uint16(raw[8:])):]
	out := make([]float32, len(body)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(body[4*i:]))
	}
	return out
}

// Whisper'ın mel_filters.npz'i librosa'nın süzgeçleridir: TR aynı değerleri verir (23.4). Dosyanın
// kaynağı ve lisansı: testdata/mel_filters.KAYNAK.md.
func TestMelFiltersMatchWhisper(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "mel_filters.npz"))
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]int{"mel_80.npy": 80, "mel_128.npy": 128} {
		want := npyFloats(t, z, name)
		got, empty := melWeights(melSpec{rate: 16000, nFFT: 400, nMels: m, slaney: true, fMax: 8000})
		if empty >= 0 {
			t.Fatalf("%s: %d. süzgeç boş", name, empty)
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %d öğe, beklenen %d", name, len(got), len(want))
		}
		worst := 0.0
		for i := range got {
			worst = max(worst, math.Abs(float64(got[i])-float64(want[i])))
		}
		if worst > melFilterBound {
			t.Errorf("%s: en büyük fark %g, sınır %g", name, worst, melFilterBound)
		}
	}
}

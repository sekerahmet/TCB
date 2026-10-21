package ml

import (
	"bytes"
	"testing"
)

// Kural (dil tanımı 15.47): veri aralıkları (başlangıç, bitiş) ikilisiyle sıralanır; aynı yerde
// başlayan boş tensör doludan önce gelir (tensor.rs). (Gemini gerileme denetimi, Bulgu 3.)
func TestSafetensorsEmptyTensorSortsBeforeFullAtSameOffset(t *testing.T) {
	header := `{"a":{"dtype":"F32","shape":[1],"data_offsets":[0,4]},"bos":{"dtype":"F32","shape":[0],"data_offsets":[0,0]}}`
	file := safetensorsBytes(header, f32Bytes(1.0))
	entries, err := decodeTensorFile(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatalf("geçerli safetensors dosyası (dolu tensörden sonra aynı ofsette boş tensör) reddedildi: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("2 tensör bekleniyordu, alınan: %d", len(entries))
	}
}

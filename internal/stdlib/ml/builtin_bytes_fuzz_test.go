package ml

import (
	"errors"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// fuzzFormats 22.1'in bütün hassasiyetleri, iki bayt sırasıyla.
func fuzzFormats() []byteFormat {
	var out []byteFormat
	for _, dt := range []tensor.DType{tensor.Float32, tensor.Float64, tensor.BFloat16, tensor.Float16,
		tensor.Int64, tensor.Int32, tensor.Int16, tensor.Int8, tensor.UInt32, tensor.UInt16, tensor.UInt8, tensor.Bool} {
		for _, big := range []bool{false, true} {
			out = append(out, byteFormat{dt: dt, name: dt.Name(), size: tensor.ElemSize(dt), big: big})
		}
	}
	return out
}

// FuzzFromBytes: TR_from_bytes'ın baytları güvenilmeyen girdidir. Her bayt ya tensör ya H kodlu hata
// verir, panik vermez; kurulan tensör aynı baytlara geri yazılır. Normal `go test` tohumları koşar.
func FuzzFromBytes(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 0, 255, 255})
	f.Add([]byte{0, 0, 192, 127, 0, 0, 128, 255})
	f.Add([]byte{2})
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, form := range fuzzFormats() {
			v, err := decodeTensor("from_bytes", data, form)
			if err != nil {
				requireCode(t, err)
				continue
			}
			got := v.(*TensorValue).t.Bytes(form.big)
			if string(got) != string(data) {
				t.Fatalf("%s: geri yazılan baytlar farklı: %v, %v", form.name, data, got)
			}
		}
	})
}

// FuzzReadNumber: .TR_read_int ve .TR_read_float her konumda ya sayı ya H kodlu hata verir.
func FuzzReadNumber(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8}, int64(0))
	f.Add([]byte{1}, int64(-1))
	f.Add([]byte{}, int64(0))
	f.Add([]byte{1, 2}, int64(1<<63-1))
	f.Fuzz(func(t *testing.T, data []byte, pos int64) {
		for _, form := range fuzzFormats() {
			raw, err := slot("read_int", data, pos, form.size)
			if err != nil {
				requireCode(t, err)
				continue
			}
			if len(raw) != form.size {
				t.Fatalf("%s: %d bayt verildi, %d bekleniyordu", form.name, len(raw), form.size)
			}
			if form.dt.Elem() == tensor.ElemInt {
				decodeInt(raw, form)
			} else if form.dt.Elem() == tensor.ElemFloat {
				decodeFloat(raw, form)
			}
		}
	})
}

// requireCode hatanın H kodlu bir ileti olduğunu sınar.
func requireCode(t *testing.T, err error) {
	t.Helper()
	var e *messages.Error
	if !errors.As(err, &e) {
		t.Fatalf("H kodu taşımayan hata: %v", err)
	}
}

package interp

import (
	"bytes"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kural (3.3a, 3.14): bayt dizisinin metni onu yeniden kuran çağrıdır (Bytes([104, 105])); uzun
// dizide boyu ve ilk 16 baytı yazılır, metin dizinin boyuyla büyümez.
func TestBytesTextIsTheCallThatBuildsIt(t *testing.T) {
	if got := NewBytes([]byte("hi")).Inspect(); got != "Bytes([104, 105])" {
		t.Errorf("Inspect = %q", got)
	}
	if got := NewBytes(nil).Inspect(); got != "Bytes([])" {
		t.Errorf("empty Inspect = %q", got)
	}
	long := NewBytes(bytes.Repeat([]byte{7}, 1000)).Inspect()
	want := "Bytes(length 1000, [7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, …])"
	if long != want {
		t.Errorf("long Inspect = %q; want %q", long, want)
	}
	if got := NewBytes(bytes.Repeat([]byte{1}, bytesTextLimit)).Inspect(); len(got) != len("Bytes([])")+bytesTextLimit*3-2 {
		t.Errorf("a %d-byte array is written whole: %q", bytesTextLimit, got)
	}
}

// Kural (3.3a): geçersiz UTF-8'in hatası ilk geçersiz baytın indeksini söyler.
func TestBytesTextNamesTheFirstInvalidByte(t *testing.T) {
	cases := map[string]int{"\xff": 0, "ab\xc5": 2, "şe\x80x": 3}
	for in, want := range cases {
		_, err := bytesText(NewBytes([]byte(in)))
		if !messages.Has(err, messages.H2656) {
			t.Fatalf("%q: %v; want H2656", in, err)
		}
		if got := err.(*messages.Error).Args[0]; got != want {
			t.Errorf("%q: offset %v; want %d", in, got, want)
		}
	}
}

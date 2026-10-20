package messages

import (
	"fmt"
	"testing"
)

// Konum hatanın alanıdır: zincirde (sarılmış da olsa) bulunur, metinden okunmaz.
func TestPosError(t *testing.T) {
	err := fmt.Errorf("sarılı: %w", At(3, 7, New(H7004, "bozuk")))
	if l, c, ok := Pos(err); !ok || l != 3 || c != 7 {
		t.Fatalf("Pos: %d %d %v", l, c, ok)
	}
	if got := At(3, 7, New(H7004, "x")).Error(); got != "Satır 3, Sütun 7: CSV okuma hatası: x" {
		t.Fatalf("sütunlu metin: %s", got)
	}
	if got := At(3, 0, New(H7004, "x")).Error(); got != "Satır 3: CSV okuma hatası: x" {
		t.Fatalf("sütunsuz metin: %s", got)
	}
	if _, _, ok := Pos(New(H7004, "x")); ok || !Has(err, H7004) {
		t.Fatal("konumsuz hata konum vermemeli; kod zincirde bulunmalı")
	}
}

// Konumu bilinmeyen (satır 0) hata konumlu sayılmaz: "Satır 0" yazılmaz, Pos zincirde aramayı sürdürür.
func TestPosErrorWithoutLine(t *testing.T) {
	err := At(0, 0, New(H2000, "f", "x"))
	if got := err.Error(); got != Text(H2000, "f", "x") {
		t.Errorf("metin: %q", got)
	}
	if _, _, ok := Pos(err); ok {
		t.Error("satırı 0 olan hata konumlu sayıldı")
	}
	if l, _, ok := Pos(At(0, 0, At(3, 0, New(H2000, "f", "x")))); !ok || l != 3 {
		t.Errorf("içteki konum bulunmadı: %d %v", l, ok)
	}
}

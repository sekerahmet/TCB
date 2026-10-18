package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// docs/KODLAR.md kod listesinden üretilenle aynı olmalı: bir kod, açıklama ya da tanım
// değiştiğinde `go run ./araclar/kod_belgesi` çalıştırılmadıysa bu test düşer.
func TestBelgeGuncel(t *testing.T) {
	yeni, err := Uret()
	if err != nil {
		t.Fatal(err)
	}
	eski, err := os.ReadFile(filepath.Join("..", "..", belgeYolu))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(eski, []byte("\r\n"), []byte("\n")), yeni) {
		t.Fatalf("%s güncel değil; depo kökünde `go run ./araclar/kod_belgesi` çalıştırın", belgeYolu)
	}
}

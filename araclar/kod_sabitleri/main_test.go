package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Üretilen sabit dosyası sözlükten üretilenle aynı olmalı: bir kod eklendiğinde,
// kaldırıldığında ya da taşındığında `go run ./araclar/kod_sabitleri` çalıştırılmadıysa
// bu test düşer.
func TestSabitlerGuncel(t *testing.T) {
	kok := filepath.Join("..", "..")
	yeni, err := Uret(kok)
	if err != nil {
		t.Fatal(err)
	}
	eski, err := os.ReadFile(filepath.Join(kok, ciktiYolu))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(eski, []byte("\r\n"), []byte("\n")), yeni) {
		t.Fatalf("%s güncel değil; depo kökünde `go run ./araclar/kod_sabitleri` çalıştırın", ciktiYolu)
	}
}

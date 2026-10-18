//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package veri

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Eşlenmiş bir parça dışarıdan kısaltılırsa okuma SIGBUS ile süreci
// çökertmez, açık hata döner (Windows'ta eşlenmiş dosya kısaltılamaz).
func TestEslemeKisaltilincaCokmez(t *testing.T) {
	v := make([]int32, 100_000)
	for i := range v {
		v[i] = int32(i)
	}
	var b bytes.Buffer
	if err := NpyYazI32(&b, v); err != nil {
		t.Fatal(err)
	}
	yol := filepath.Join(t.TempDir(), "p.npy")
	if err := os.WriteFile(yol, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ParcaAc(yol, ParcaSecenekleri{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Kapat()
	if !p.Eslemeli() {
		t.Skip("eşleme kullanılamadı:", p.EslemeHatasi())
	}
	dst := make([]int32, 8)
	if err := p.Oku(dst, 90_000); err != nil || dst[0] != 90_000 {
		t.Fatalf("okuma: %v %v", dst, err)
	}
	if err := os.Truncate(yol, 4096); err != nil {
		t.Fatal(err)
	}
	err = p.Oku(dst, 90_000)
	if err == nil || !strings.Contains(err.Error(), "bellek hatası") {
		t.Fatalf("kısaltılmış eşlemede açık hata bekleniyordu: %v", err)
	}
}

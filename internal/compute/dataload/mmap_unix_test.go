//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package dataload

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Eşlenmiş bir parça dışarıdan kısaltılırsa okuma SIGBUS ile süreci
// çökertmez, açık hata döner (Windows'ta eşlenmiş dosya kısaltılamaz).
func TestMmapTruncatedDoesNotCrash(t *testing.T) {
	v := make([]int32, 100_000)
	for i := range v {
		v[i] = int32(i)
	}
	var b bytes.Buffer
	if err := WriteNpyI32(&b, v); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "p.npy")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := OpenShard(path, ShardOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.Mmapped() {
		t.Skip("eşleme kullanılamadı:", p.MmapError())
	}
	dst := make([]int32, 8)
	if err := p.Read(dst, 90_000); err != nil || dst[0] != 90_000 {
		t.Fatalf("okuma: %v %v", dst, err)
	}
	if err := os.Truncate(path, 4096); err != nil {
		t.Fatal(err)
	}
	err = p.Read(dst, 90_000)
	if !messages.Has(err, messages.H5088) {
		t.Fatalf("kısaltılmış eşlemede açık hata bekleniyordu: %v", err)
	}
}

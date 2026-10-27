package dataload

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

func TestShardMmapAndReadAtSameData(t *testing.T) {
	checkShardLeak(t)
	d := t.TempDir()
	u := fakeU16(100_003, 7)
	pathU := writeU16(t, d, "u.npy", u)
	iv := make([]int32, 50_001)
	for i := range iv {
		iv[i] = int32(i*31 - 20_000)
	}
	pathI := writeI32(t, d, "i.npy", iv)

	for _, closed := range []bool{false, true} {
		p, err := OpenShard(pathU, ShardOptions{NoMmap: closed})
		if err != nil {
			t.Fatal(err)
		}
		if closed && p.Mmapped() {
			t.Fatal("eşleme kapalıyken eşlemeli açıldı")
		}
		if !closed && mmapSupported && !p.Mmapped() {
			t.Fatalf("eşleme bekleniyordu: %v", p.MmapError())
		}
		if p.Length() != int64(len(u)) || p.DType() != DTypeU16 {
			t.Fatalf("yanlış başlık: %+v", p.Header())
		}
		// Tüm dosya, ReadAt tamponundan büyük tek okuma ve küçük okumalar.
		all := make([]int32, len(u))
		if err := p.Read(all, 0); err != nil {
			t.Fatal(err)
		}
		for i := range u {
			if all[i] != int32(u[i]) {
				t.Fatalf("eşleme=%v: %d konumunda %d != %d", !closed, i, all[i], u[i])
			}
		}
		k := make([]uint16, 17)
		if err := p.ReadU16(k, 99_986); err != nil {
			t.Fatal(err)
		}
		for i := range k {
			if k[i] != u[99_986+i] {
				t.Fatal("OkuU16 yanlış")
			}
		}
		if err := p.Read(make([]int32, 4), int64(len(u))-3); !messages.Has(err, messages.H5085) {
			t.Fatalf("sınır hatası bekleniyordu: %v", err)
		}
		if err := p.Read(make([]int32, 1), -1); err == nil {
			t.Fatal("negatif konum kabul edildi")
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}

		pi, err := OpenShard(pathI, ShardOptions{NoMmap: closed})
		if err != nil {
			t.Fatal(err)
		}
		g := make([]int32, 1000)
		if err := pi.Read(g, 49_001); err != nil {
			t.Fatal(err)
		}
		for i := range g {
			if g[i] != iv[49_001+i] {
				t.Fatalf("i4 %d: %d != %d", i, g[i], iv[49_001+i])
			}
		}
		if err := pi.ReadU16(make([]uint16, 1), 0); err == nil {
			t.Fatal("i4 parçada OkuU16 kabul edildi")
		}
		_ = pi.Close()
	}
}

func TestShardCloseAndLeak(t *testing.T) {
	checkShardLeak(t)
	d := t.TempDir()
	path := writeU16(t, d, "k.npy", fakeU16(5000, 1))
	for _, closed := range []bool{false, true} {
		baseOff := openShardCount.Load()
		p, err := OpenShard(path, ShardOptions{NoMmap: closed})
		if err != nil {
			t.Fatal(err)
		}
		if openShardCount.Load() != baseOff+1 {
			t.Fatal("açık parça sayacı artmadı")
		}
		// Eşzamanlı okumalar sırasında kapatma güvenli olmalı.
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				b := make([]int32, 64)
				for i := 0; i < 200; i++ {
					err := p.Read(b, int64((g*97+i*13)%4900))
					if err != nil && !errors.Is(err, ErrClosed) {
						t.Error(err)
						return
					}
				}
			}(g)
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		if err := p.Close(); err != nil {
			t.Fatal("ikinci Kapat hata verdi:", err)
		}
		if openShardCount.Load() != baseOff {
			t.Fatal("açık parça sayacı geri dönmedi")
		}
		if err := p.Read(make([]int32, 1), 0); !errors.Is(err, ErrClosed) {
			t.Fatalf("kapalı parçada ErrKapali bekleniyordu: %v", err)
		}
	}
	// Tüm tutamaklar ve eşlemeler bırakıldıysa dosya silinebilmeli
	// (bazı platformlarda açık ya da eşlenmiş dosya silinemez).
	if err := os.Remove(path); err != nil {
		t.Fatalf("kapatılan parça silinemedi (kaynak sızıntısı?): %v", err)
	}
}

func TestShardEmptyFile(t *testing.T) {
	checkShardLeak(t)
	d := t.TempDir()
	path := writeU16(t, d, "bos.npy", nil)
	p, err := OpenShard(path, ShardOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Length() != 0 || p.Mmapped() {
		t.Fatal("boş parça yanlış")
	}
	if err := p.Read(nil, 0); err != nil {
		t.Fatal(err)
	}
}

// TestShardU32Overflow int32'ye sığmayan bir '<u4' değerinin (eksiye dönüp çapraz entropide
// yok sayılacak hedef) hata verdiğini, öbür aralıkların okunduğunu denetler.
func TestShardU32Overflow(t *testing.T) {
	checkShardLeak(t)
	var b bytes.Buffer
	if err := WriteHeader(&b, DTypeU32, 4, 1); err != nil {
		t.Fatal(err)
	}
	for _, v := range []uint32{7, math.MaxInt32, 1 << 31, 9} {
		b.Write(binary.LittleEndian.AppendUint32(nil, v))
	}
	path := filepath.Join(t.TempDir(), "u4.npy")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, noMmap := range []bool{false, true} {
		p, err := OpenShard(path, ShardOptions{NoMmap: noMmap})
		if err != nil {
			t.Fatal(err)
		}
		ok := make([]int32, 2)
		if err := p.Read(ok, 0); err != nil || ok[1] != math.MaxInt32 {
			t.Fatalf("eşlemesiz=%v: sığan aralık okunamadı: %v %v", noMmap, ok, err)
		}
		if err := p.Read(make([]int32, 2), 1); !messages.Has(err, messages.H5086) {
			t.Fatalf("eşlemesiz=%v: 2^31 hata vermedi: %v", noMmap, err)
		}
		p.Close()
	}
}

func TestShardTruncatedData(t *testing.T) {
	d := t.TempDir()
	var b bytes.Buffer
	if err := WriteHeader(&b, DTypeU16, 100, 1); err != nil {
		t.Fatal(err)
	}
	b.Write(make([]byte, 150)) // 200 bayt gerekli
	path := filepath.Join(d, "kesik.npy")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := OpenShard(path, ShardOptions{})
	if !messages.Has(err, messages.H5084) || !strings.Contains(err.Error(), "kesik.npy") {
		t.Fatalf("kesik dosya hatası bekleniyordu: %v", err)
	}
	// Hiç .npy olmayan dosya.
	path2 := filepath.Join(d, "TR_str.npy")
	_ = os.WriteFile(path2, []byte("merhaba dünya, bu bir npy değil"), 0o644)
	if _, err := OpenShard(path2, ShardOptions{}); !messages.Has(err, messages.H5053) {
		t.Fatalf("büyü hatası bekleniyordu: %v", err)
	}
	// Boş dosya.
	path3 := filepath.Join(d, "bos.npy")
	_ = os.WriteFile(path3, nil, 0o644)
	if _, err := OpenShard(path3, ShardOptions{}); !messages.Has(err, messages.H5051) {
		t.Fatalf("kısa dosya hatası bekleniyordu: %v", err)
	}
}

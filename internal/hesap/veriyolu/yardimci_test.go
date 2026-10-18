package veriyolu

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// u16Yaz, dizinde ad adlı '<u2' .npy dosyası oluşturur.
func u16Yaz(t testing.TB, dizin, ad string, veri []uint16) string {
	t.Helper()
	var b bytes.Buffer
	if err := NpyYazU16(&b, veri); err != nil {
		t.Fatal(err)
	}
	yol := filepath.Join(dizin, ad)
	if err := os.WriteFile(yol, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return yol
}

// i32Yaz, dizinde ad adlı '<i4' .npy dosyası oluşturur.
func i32Yaz(t testing.TB, dizin, ad string, veri []int32) string {
	t.Helper()
	var b bytes.Buffer
	if err := NpyYazI32(&b, veri); err != nil {
		t.Fatal(err)
	}
	yol := filepath.Join(dizin, ad)
	if err := os.WriteFile(yol, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return yol
}

// kodluKume, her tokenın değeri parça*1e6 + konum olan '<i4' parçalardan
// bir dizin kurar. Böylece her token hangi parçadan ve konumdan geldiğini
// kendisi söyler.
func kodluKume(t testing.TB, uzunluklar ...int) string {
	t.Helper()
	d := t.TempDir()
	for p, n := range uzunluklar {
		v := make([]int32, n)
		for i := range v {
			v[i] = int32(p*1_000_000 + i)
		}
		i32Yaz(t, d, parcaAdi(p), v)
	}
	return d
}

func parcaAdi(p int) string {
	return "p_" + string(rune('a'+p/26)) + string(rune('a'+p%26)) + ".npy"
}

// sozdeU16, tohumdan belirlenimli sahte token dizisi üretir (< 40000).
func sozdeU16(n int, tohum uint64) []uint16 {
	v := make([]uint16, n)
	x := tohum*0x9E3779B97F4A7C15 + 1
	for i := range v {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		v[i] = uint16(x % 40000)
	}
	return v
}

// gorutinBekle, gorutin sayısı taban'a inene kadar bekler; inmezse hata.
func gorutinBekle(t *testing.T, taban int) {
	t.Helper()
	son := time.Now().Add(3 * time.Second)
	for {
		n := runtime.NumGoroutine()
		if n <= taban {
			return
		}
		if time.Now().After(son) {
			buf := make([]byte, 1<<16)
			buf = buf[:runtime.Stack(buf, true)]
			t.Fatalf("gorutin sızıntısı: %d > %d\n%s", n, taban, buf)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// parcaSizintiDenetle, test sonunda açık parça kalmadığını denetler.
func parcaSizintiDenetle(t *testing.T) {
	t.Helper()
	taban := acikParcaSayisi.Load()
	t.Cleanup(func() {
		if n := acikParcaSayisi.Load(); n != taban {
			t.Errorf("parça sızıntısı: açık parça %d, beklenen %d", n, taban)
		}
	})
}

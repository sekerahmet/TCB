package dataload

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// u16Yaz, dizinde ad adlı '<u2' .npy dosyası oluşturur.
func writeU16(t testing.TB, dir, ad string, data []uint16) string {
	t.Helper()
	var b bytes.Buffer
	if err := WriteNpyU16(&b, data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ad)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// i32Yaz, dizinde ad adlı '<i4' .npy dosyası oluşturur.
func writeI32(t testing.TB, dir, ad string, data []int32) string {
	t.Helper()
	var b bytes.Buffer
	if err := WriteNpyI32(&b, data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ad)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// kodluKume, her tokenın değeri parça*1e6 + konum olan '<i4' parçalardan
// bir dizin kurar. Böylece her token hangi parçadan ve konumdan geldiğini
// kendisi söyler.
func encodedDataset(t testing.TB, lengths ...int) string {
	t.Helper()
	d := t.TempDir()
	for p, n := range lengths {
		v := make([]int32, n)
		for i := range v {
			v[i] = int32(p*1_000_000 + i)
		}
		writeI32(t, d, shardName(p), v)
	}
	return d
}

func shardName(p int) string {
	return "p_" + string(rune('a'+p/26)) + string(rune('a'+p%26)) + ".npy"
}

// sozdeU16, tohumdan belirlenimli sahte token dizisi üretir (< 40000).
func fakeU16(n int, seed uint64) []uint16 {
	v := make([]uint16, n)
	x := seed*0x9E3779B97F4A7C15 + 1
	for i := range v {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		v[i] = uint16(x % 40000)
	}
	return v
}

// gorutinBekle, gorutin sayısı taban'a inene kadar bekler; inmezse hata.
func waitGoroutine(t *testing.T, baseOff int) {
	t.Helper()
	last := time.Now().Add(3 * time.Second)
	for {
		n := runtime.NumGoroutine()
		if n <= baseOff {
			return
		}
		if time.Now().After(last) {
			buf := make([]byte, 1<<16)
			buf = buf[:runtime.Stack(buf, true)]
			t.Fatalf("gorutin sızıntısı: %d > %d\n%s", n, baseOff, buf)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// gorutinBeklemede, yığınında islev geçen bir gorutin kanal beklemesinde (select ya da
// kanaldan alma) durana kadar bekler. Uyku yerine kullanılır: "Al beklerken Kapat" gibi
// sınamalar beklemenin gerçekten başladığından emin olur, yoksa sınadığını sınamadan da
// geçebilirdi. Süre sınırı yalnızca takılmaya karşıdır.
func goroutineWaiting(t *testing.T, function string) {
	t.Helper()
	last := time.Now().Add(5 * time.Second)
	buf := make([]byte, 1<<20)
	for {
		stack := string(buf[:runtime.Stack(buf, true)])
		for _, g := range strings.Split(stack, "\n\n") {
			header, _, _ := strings.Cut(g, "\n")
			if (strings.Contains(header, "[select") || strings.Contains(header, "[chan receive")) && strings.Contains(g, function) {
				return
			}
		}
		if time.Now().After(last) {
			t.Fatalf("%s beklemeye girmedi:\n%s", function, stack)
		}
		time.Sleep(time.Millisecond)
	}
}

// sinyalBekle kanaldan bir sinyal gelene kadar bekler; süre sınırı yalnızca takılmaya karşıdır.
func waitSignal(t *testing.T, c <-chan struct{}, ne string) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s olmadı", ne)
	}
}

// parcaSizintiDenetle, test sonunda açık parça kalmadığını denetler.
func checkShardLeak(t *testing.T) {
	t.Helper()
	baseOff := openShardCount.Load()
	t.Cleanup(func() {
		if n := openShardCount.Load(); n != baseOff {
			t.Errorf("parça sızıntısı: açık parça %d, beklenen %d", n, baseOff)
		}
	})
}

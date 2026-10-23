package dataload

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeNpyFile veriyi writeFn ile .npy olarak yazar ve dizinde ad adlı dosyaya koyar.
func writeNpyFile[T any](t testing.TB, dir, ad string, data []T, writeFn func(io.Writer, []T) error) string {
	t.Helper()
	var b bytes.Buffer
	if err := writeFn(&b, data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ad)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeU16, dizinde ad adlı '<u2' .npy dosyası oluşturur.
func writeU16(t testing.TB, dir, ad string, data []uint16) string {
	t.Helper()
	return writeNpyFile(t, dir, ad, data, WriteNpyU16)
}

// writeI32, dizinde ad adlı '<i4' .npy dosyası oluşturur.
func writeI32(t testing.TB, dir, ad string, data []int32) string {
	t.Helper()
	return writeNpyFile(t, dir, ad, data, WriteNpyI32)
}

// encodedDataset, her tokenın değeri parça*1e6 + konum olan '<i4' parçalardan
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

// fakeU16, tohumdan belirlenimli sahte token dizisi üretir (< 40000).
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

// waitGoroutine, gorutin sayısı taban'a inene kadar bekler; inmezse hata.
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

// waitSignal kanaldan bir sinyal gelene kadar bekler; süre sınırı yalnızca takılmaya karşıdır.
func waitSignal(t *testing.T, c <-chan struct{}, ne string) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s olmadı", ne)
	}
}

// checkShardLeak, test sonunda açık parça kalmadığını denetler.
func checkShardLeak(t *testing.T) {
	t.Helper()
	baseOff := openShardCount.Load()
	t.Cleanup(func() {
		if n := openShardCount.Load(); n != baseOff {
			t.Errorf("parça sızıntısı: açık parça %d, beklenen %d", n, baseOff)
		}
	})
}

// runningWorkers cekirdekAc'ın başlattığı işçi gorutinlerini sayar ("created by" satırı
// gorutin henüz çalışmaya başlamamışken de vardır; giriş fonksiyonunun adı o sırada farklı
// görünür). Toplam gorutin sayısıyla karşılaştırmak kararsızdı: önceki testlerden kalan
// gorutinler bu arada bitince sayı düşüyor, işçiler başlamış olsa da test kalıyordu.
func runningWorkers() int {
	b := make([]byte, 1<<20)
	return strings.Count(string(b[:runtime.Stack(b, true)]), "created by github.com/sekerahmet/TCB/internal/compute/dataload.openCore ")
}

// waitWorkers işçi sayısı n olana kadar (en çok 3 sn) bekler ve son sayıyı verir. Tek bir
// sayım kararsızdı: kapanan önceki önbelleğin işçisi wg.Done'dan sonra bir an daha yığın
// dökümünde görünebilir (yarış dedektörüyle CI'da 4 yerine 5 sayıldı).
func waitWorkers(n int) int {
	last := time.Now().Add(3 * time.Second)
	for {
		if c := runningWorkers(); c == n || time.Now().After(last) {
			return c
		}
		time.Sleep(5 * time.Millisecond)
	}
}

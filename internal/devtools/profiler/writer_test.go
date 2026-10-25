package profiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/reporoot"
)

// Yerine koymanın geçici hatası (Windows'ta dosyayı o an okuyan varken) kısa yeniden
// denemeyle aşılır: uyarı yok, doğrudan yazmaya geçilmez.
func TestReplaceTransientErrorRetried(t *testing.T) {
	old := replace
	remaining := len(replaceWaits) // ilk deneme ve son bekleme dışındakiler başarısız
	replace = func(a, b string) error {
		if remaining > 0 {
			remaining--
			return os.ErrPermission
		}
		return old(a, b)
	}
	defer func() { replace = old }()
	dir := t.TempDir()
	y := &statusWriter{fpath: filepath.Join(dir, StatusFileName), warning: func(m string) { t.Errorf("beklenmeyen uyarı: %s", m) }}
	if err := y.write(&Snapshot{Format: FormatName, Seconds: 7}); err != nil {
		t.Fatal(err)
	}
	if y.faulty != 0 || y.direct {
		t.Errorf("geçici hata kalıcı sayıldı: %d hata, doğrudan %v", y.faulty, y.direct)
	}
	if inputs, _ := reporoot.ReadDir(dir); len(inputs) != 1 {
		t.Errorf("geçici dosya kaldı: %v", inputs)
	}
}

// Atomik yazma: yazıcı dosyayı art arda yenilerken okuyan hiçbir zaman yarım ya da bozuk
// bir dosya görmez (her okuma geçerli JSON'dur ve bir görüntünün tamamıdır).
func TestStatusFileReaderSeesNoPartialFile(t *testing.T) {
	fpath := filepath.Join(t.TempDir(), StatusFileName)
	y := &statusWriter{fpath: fpath, warning: func(m string) { t.Errorf("beklenmeyen uyarı: %s", m) }}
	// Büyük bir görüntü: tek yazmada bitmesin.
	g := &Snapshot{Format: FormatName, Version: FormatVersion}
	for i := 0; i < 400; i++ {
		g.Tensor.Ops = append(g.Tensor.Ops, OpInfo{Code: "matris_carpimi", Name: strings.Repeat("ç", 40), Calls: int64(i)})
	}
	if err := y.write(g); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			time.Sleep(5 * time.Millisecond) // gerçek yazıcı saniyede bir yazar
			g.Seconds = float64(i)
			if err := y.write(g); err != nil {
				t.Errorf("yazma: %v", err)
				return
			}
		}
	}()
	readVal := 0
	for readVal < 100 {
		data, err := os.ReadFile(fpath)
		if err != nil {
			time.Sleep(time.Millisecond)
			continue // Windows'ta yerine koyma anında dosya kısa süre açılamayabilir
		}
		var d Snapshot
		if err := json.Unmarshal(data, &d); err != nil {
			t.Fatalf("okuyan yarım ya da bozuk dosya gördü (%d bayt): %v", len(data), err)
		}
		if len(d.Tensor.Ops) != 400 {
			t.Fatalf("görüntü eksik: %d işlem", len(d.Tensor.Ops))
		}
		readVal++
		time.Sleep(10 * time.Millisecond) // okuyan araçlar saniyede bir okur; sıkı döngü Windows'ta yerine koymayı (yük altında 250 ms boyunca) engelliyordu
	}
	close(stop)
	wg.Wait()
	if y.direct {
		t.Error("yerine koyma bu sürücüde çalışmalıydı")
	}
}

// Yerine koyma bir aralıkta bütün denemelerde olmazsa (Windows'ta okuyan dosyayı tutuyor) o
// aralık atlanır: önceki tam dosya kalır, uyarı yok. Son rapor atlanmaz, doğrudan yazılır.
func TestReplaceFailureSkipsInterval(t *testing.T) {
	dir := t.TempDir()
	y := &statusWriter{fpath: filepath.Join(dir, StatusFileName), warning: func(m string) { t.Errorf("beklenmeyen uyarı: %s", m) }}
	if err := y.write(&Snapshot{Format: FormatName, Seconds: 1}); err != nil {
		t.Fatal(err)
	}
	old := replace
	replace = func(string, string) error { return os.ErrPermission }
	defer func() { replace = old }()
	if err := y.write(&Snapshot{Format: FormatName, Seconds: 2}); err != nil {
		t.Fatal(err)
	}
	read := func() float64 {
		var g Snapshot
		data, _ := os.ReadFile(y.fpath)
		if err := json.Unmarshal(data, &g); err != nil {
			t.Fatalf("dosya bozuk: %v", err)
		}
		return g.Seconds
	}
	if s := read(); s != 1 {
		t.Errorf("atlanan aralıkta dosya değişti: %v", s)
	}
	if inputs, _ := reporoot.ReadDir(dir); len(inputs) != 1 {
		t.Errorf("geçici dosya kaldı: %v", inputs)
	}
	y.warning = func(string) {} // son rapor doğrudan yazılır ve bir kez uyarılır
	if err := y.writeFinal(&Snapshot{Format: FormatName, Seconds: 3}); err != nil {
		t.Fatal(err)
	}
	if s := read(); s != 3 {
		t.Errorf("son rapor yazılmadı: %v", s)
	}
}

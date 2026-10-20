package profiler

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Çıktı dosyasına yazılamazsa (ör. disk dolu) boru boşaltılmaya devam eder: programın
// yazdır'ı donmaz, son satırlar tutulmayı sürdürür.
func TestCaptureKeepsDrainingAfterFileError(t *testing.T) {
	y, err := StartCapture(filepath.Join(t.TempDir(), "çıktı.txt"))
	if err != nil {
		t.Fatal(err)
	}
	y.file.Close() // bundan sonra dosyaya her yazma hata verir
	done := make(chan struct{})
	go func() {
		defer close(done)
		line := strings.Repeat("x", 1023) + "\n"
		for i := 0; i < 4096; i++ { // borunun tamponundan çok büyük (4 MB)
			if _, err := y.Writer().WriteString(line); err != nil {
				return
			}
		}
		y.Writer().WriteString("son satır\n")
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("dosya hatasından sonra boru boşaltılmadı: yazma dondu")
	}
	y.Close()
	if last := y.LastLines(1); len(last) != 1 || last[0] != "son satır" {
		t.Fatalf("son satırlar tutulmalıydı: %q", last)
	}
	if y.WriteError() == nil {
		t.Fatal("dosyaya yazma hatası bildirilmeli")
	}
}

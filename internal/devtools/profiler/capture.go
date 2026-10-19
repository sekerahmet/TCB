package profiler

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
)

// Yakalama, canlı tablo terminali kullanırken programın kendi çıktısını (yazdır, ilerleme
// çubuğu, hata mesajları) bir boruya alır: çıktının tamamı bir dosyaya yazılır, son
// satırları tablonun altında gösterilir. Böylece program çıktısı tabloyla karışmaz ve
// kaybolmaz. Boru bir terminal olmadığından ilerleme çubuğu düz satır kipine geçer.

// lastLineCount bellekte tutulan son satır sayısıdır.
const lastLineCount = 50

// controlSeq çıktıdaki terminal denetim dizileridir (renk, imleç); son satırlar
// gösterilirken tabloyu bozmasınlar diye silinir (dosyaya olduğu gibi yazılır).
var controlSeq = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

// Capture programın yakalanan çıktısıdır.
type Capture struct {
	fpath string
	file  *os.File
	r, w  *os.File

	mu    sync.Mutex
	lines []string
	done  chan struct{}
}

// StartCapture çıktıyı yol dosyasına (üzerine) yazan bir yakalama başlatır.
func StartCapture(fpath string) (*Capture, error) {
	d, err := os.Create(fpath)
	if err != nil {
		return nil, err
	}
	r, w, err := os.Pipe()
	if err != nil {
		d.Close()
		return nil, err
	}
	y := &Capture{fpath: fpath, file: d, r: r, w: w, done: make(chan struct{})}
	go y.read()
	return y, nil
}

// Writer programın standart çıkışının ve hatasının yerine geçecek dosyadır (borunun ucu).
func (y *Capture) Writer() *os.File { return y.w }

// Path çıktının yazıldığı dosyadır.
func (y *Capture) Path() string { return y.fpath }

func (y *Capture) read() {
	defer close(y.done)
	s := bufio.NewReader(io.TeeReader(y.r, y.file))
	for {
		line, err := s.ReadString('\n')
		if line != "" {
			y.add(line)
		}
		if err != nil {
			return
		}
	}
}

// add bir satırı son satırlara ekler (satır başına dönüşte son parça kalır).
func (y *Capture) add(line string) {
	line = strings.TrimRight(line, "\r\n")
	if i := strings.LastIndexByte(line, '\r'); i >= 0 {
		line = line[i+1:]
	}
	line = controlSeq.ReplaceAllString(line, "")
	y.mu.Lock()
	y.lines = append(y.lines, line)
	if len(y.lines) > lastLineCount {
		y.lines = y.lines[len(y.lines)-lastLineCount:]
	}
	y.mu.Unlock()
}

// LastLines son n satırı verir.
func (y *Capture) LastLines(n int) []string {
	y.mu.Lock()
	defer y.mu.Unlock()
	if n > len(y.lines) {
		n = len(y.lines)
	}
	return append([]string(nil), y.lines[len(y.lines)-n:]...)
}

// Close borunun yazma ucunu kapatır, kalan çıktının okunmasını bekler ve dosyayı kapatır.
// Çağırandan önce standart çıkış ve hata yerine konmuş olmalıdır.
func (y *Capture) Close() {
	y.w.Close()
	<-y.done
	y.r.Close()
	y.file.Close()
}

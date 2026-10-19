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

// sonSatirSayisi bellekte tutulan son satır sayısıdır.
const sonSatirSayisi = 50

// denetimDizisi çıktıdaki terminal denetim dizileridir (renk, imleç); son satırlar
// gösterilirken tabloyu bozmasınlar diye silinir (dosyaya olduğu gibi yazılır).
var denetimDizisi = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

// Yakalama programın yakalanan çıktısıdır.
type Yakalama struct {
	yol   string
	dosya *os.File
	r, w  *os.File

	mu       sync.Mutex
	satirlar []string
	bitti    chan struct{}
}

// YakalamaBaslat çıktıyı yol dosyasına (üzerine) yazan bir yakalama başlatır.
func YakalamaBaslat(yol string) (*Yakalama, error) {
	d, err := os.Create(yol)
	if err != nil {
		return nil, err
	}
	r, w, err := os.Pipe()
	if err != nil {
		d.Close()
		return nil, err
	}
	y := &Yakalama{yol: yol, dosya: d, r: r, w: w, bitti: make(chan struct{})}
	go y.oku()
	return y, nil
}

// Yazici programın standart çıkışının ve hatasının yerine geçecek dosyadır (borunun ucu).
func (y *Yakalama) Yazici() *os.File { return y.w }

// Yol çıktının yazıldığı dosyadır.
func (y *Yakalama) Yol() string { return y.yol }

func (y *Yakalama) oku() {
	defer close(y.bitti)
	s := bufio.NewReader(io.TeeReader(y.r, y.dosya))
	for {
		satir, err := s.ReadString('\n')
		if satir != "" {
			y.ekle(satir)
		}
		if err != nil {
			return
		}
	}
}

// ekle bir satırı son satırlara ekler (satır başına dönüşte son parça kalır).
func (y *Yakalama) ekle(satir string) {
	satir = strings.TrimRight(satir, "\r\n")
	if i := strings.LastIndexByte(satir, '\r'); i >= 0 {
		satir = satir[i+1:]
	}
	satir = denetimDizisi.ReplaceAllString(satir, "")
	y.mu.Lock()
	y.satirlar = append(y.satirlar, satir)
	if len(y.satirlar) > sonSatirSayisi {
		y.satirlar = y.satirlar[len(y.satirlar)-sonSatirSayisi:]
	}
	y.mu.Unlock()
}

// SonSatirlar son n satırı verir.
func (y *Yakalama) SonSatirlar(n int) []string {
	y.mu.Lock()
	defer y.mu.Unlock()
	if n > len(y.satirlar) {
		n = len(y.satirlar)
	}
	return append([]string(nil), y.satirlar[len(y.satirlar)-n:]...)
}

// Kapat borunun yazma ucunu kapatır, kalan çıktının okunmasını bekler ve dosyayı kapatır.
// Çağırandan önce standart çıkış ve hata yerine konmuş olmalıdır.
func (y *Yakalama) Kapat() {
	y.w.Close()
	<-y.bitti
	y.r.Close()
	y.dosya.Close()
}

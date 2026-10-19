package profiler

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/stdlib/logging"
)

// Canlı gösterim: terminalde ekranın üstünde saniyede bir yeniden çizilen tablo; terminal
// değilse (boru, dosya, not defteri hücresi) 10 saniyede bir düz özet satırı.
//
// Terminalde tablo ekranın başından çizilir (imleç başa, her satırın sonu silinir, tablonun
// altı silinir). Ayrı ekran tamponu kullanılmaz: program ikinci kesme isteğiyle hemen
// çıkarsa terminal ayrı tamponda kalmasın; ekranda yalnızca son tablo durur.

// plainInterval terminal değilken özet satırının aralığıdır (testler kısaltır).
var plainInterval = 10 * time.Second

// Live canlı gösterimdir.
type Live struct {
	w        *os.File
	terminal bool
	capture  *Capture
	restore  func() // terminal kipini geri alır

	mu        sync.Mutex
	lastPlain time.Time
	closed    bool
}

// NewLive w'ye (gerçek standart hata) yazan canlı gösterimi kurar. terminal istenmişse
// terminalin denetim dizileri açılmaya çalışılır; açılamazsa düz satırlara düşülür. yak
// programın yakalanan çıktısıdır (yoksa nil).
func NewLive(w *os.File, terminal bool, capture *Capture) *Live {
	c := &Live{w: w, capture: capture, lastPlain: time.Now()}
	if terminal {
		if restore, ok := enableControlSeqs(w); ok {
			c.terminal, c.restore = true, restore
			w.WriteString("\x1b[2J\x1b[H")
		}
	}
	return c
}

// Capture programın yakalanan çıktısını tablonun altında gösterir.
func (c *Live) Capture(y *Capture) {
	c.mu.Lock()
	c.capture = y
	c.mu.Unlock()
}

// Terminal canlı gösterimin terminal kipinde olup olmadığıdır.
func (c *Live) Terminal() bool { return c.terminal }

// Show bir görüntüyü gösterir (profilin saniyelik döngüsünden çağrılır).
func (c *Live) Show(g *Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	if !c.terminal {
		if time.Since(c.lastPlain) >= plainInterval {
			c.lastPlain = time.Now()
			c.w.WriteString(SummaryLine(g) + "\n")
		}
		return
	}
	e := Screen{Width: logging.TerminalWidth(c.w) - 1}
	if c.capture != nil {
		e.OutputFile, e.LastOutput = c.capture.Path(), c.capture.LastLines(8)
	}
	var b strings.Builder
	b.WriteString("\x1b[H")
	for _, s := range ScreenLines(g, e) {
		b.WriteString(s)
		b.WriteString("\x1b[K\n")
	}
	b.WriteString("\x1b[J")
	c.w.WriteString(b.String())
}

// Close canlı gösterimi bitirir: terminal kipini geri alır. Son tablo ekranda kalır; sonra
// yazılanlar onun altına gelir.
func (c *Live) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.restore != nil {
		c.restore()
		c.restore = nil
	}
	c.closed = true
}

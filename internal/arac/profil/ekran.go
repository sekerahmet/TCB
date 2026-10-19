package profil

import (
	"os"
	"strings"
	"sync"
	"time"

	"tr_dili/internal/motor/gunluk"
)

// Canlı gösterim: terminalde ekranın üstünde saniyede bir yeniden çizilen tablo; terminal
// değilse (boru, dosya, not defteri hücresi) 10 saniyede bir düz özet satırı.
//
// Terminalde tablo ekranın başından çizilir (imleç başa, her satırın sonu silinir, tablonun
// altı silinir). Ayrı ekran tamponu kullanılmaz: program ikinci kesme isteğiyle hemen
// çıkarsa terminal ayrı tamponda kalmasın; ekranda yalnızca son tablo durur.

// duzAralik terminal değilken özet satırının aralığıdır (testler kısaltır).
var duzAralik = 10 * time.Second

// Canli canlı gösterimdir.
type Canli struct {
	w        *os.File
	terminal bool
	yak      *Yakalama
	geri     func() // terminal kipini geri alır

	mu      sync.Mutex
	sonDuz  time.Time
	kapandi bool
}

// YeniCanli w'ye (gerçek standart hata) yazan canlı gösterimi kurar. terminal istenmişse
// terminalin denetim dizileri açılmaya çalışılır; açılamazsa düz satırlara düşülür. yak
// programın yakalanan çıktısıdır (yoksa nil).
func YeniCanli(w *os.File, terminal bool, yak *Yakalama) *Canli {
	c := &Canli{w: w, yak: yak, sonDuz: time.Now()}
	if terminal {
		if geri, ok := denetimDizileriniAc(w); ok {
			c.terminal, c.geri = true, geri
			w.WriteString("\x1b[2J\x1b[H")
		}
	}
	return c
}

// Yakala programın yakalanan çıktısını tablonun altında gösterir.
func (c *Canli) Yakala(y *Yakalama) {
	c.mu.Lock()
	c.yak = y
	c.mu.Unlock()
}

// Terminal canlı gösterimin terminal kipinde olup olmadığıdır.
func (c *Canli) Terminal() bool { return c.terminal }

// Goster bir görüntüyü gösterir (profilin saniyelik döngüsünden çağrılır).
func (c *Canli) Goster(g *Goruntu) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.kapandi {
		return
	}
	if !c.terminal {
		if time.Since(c.sonDuz) >= duzAralik {
			c.sonDuz = time.Now()
			c.w.WriteString(OzetSatiri(g) + "\n")
		}
		return
	}
	e := Ekran{Genislik: gunluk.TerminalGenisligi(c.w) - 1}
	if c.yak != nil {
		e.CiktiDosyasi, e.SonCikti = c.yak.Yol(), c.yak.SonSatirlar(8)
	}
	var b strings.Builder
	b.WriteString("\x1b[H")
	for _, s := range EkranSatirlari(g, e) {
		b.WriteString(s)
		b.WriteString("\x1b[K\n")
	}
	b.WriteString("\x1b[J")
	c.w.WriteString(b.String())
}

// Kapat canlı gösterimi bitirir: terminal kipini geri alır. Son tablo ekranda kalır; sonra
// yazılanlar onun altına gelir.
func (c *Canli) Kapat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.geri != nil {
		c.geri()
		c.geri = nil
	}
	c.kapandi = true
}

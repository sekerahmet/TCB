package dataload

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// OnYukleyici, bir Kaynak'tan sonraki toplu işleri arka plandaki tek bir
// gorutinde hazırlar. derinlik adet tampon dolaşımdadır: üretici boş
// tamponları doldurup kuyruğa koyar, Al dolu bir tamponu çağıranın
// TopluIs'i ile takas eder (kopyalama yok) ve eskisini üreticiye geri verir.
// Toplu işlerin sırası kaynağın üretim sırasıyla aynıdır.
//
// Kapat üreticiyi durdurur, gorutinin bitmesini bekler ve ANCAK bundan
// sonra, kaynak bir Kapat() error ya da Close() error yöntemine sahipse
// kaynağı kapatır; böylece kaynak, üretici onu kullanırken kapatılmaz.
// Verilen bağlamın iptali de üreticiyi durdurur (kaynağı kapatmaz).
// Takılan okumalara karşı KapatZamanli kullanılabilir.
type OnYukleyici struct {
	kaynak Kaynak
	bos    chan *TopluIs
	dolu   chan sonucTopluIs
	ctx    context.Context
	iptal  context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex
	kaliciHt error // kaynak hatası ya da kapatma; sonraki Al'lar bunu döner
	kapandi  bool

	kaynakBir sync.Once
	kaynakHt  error
}

// kapatici, kendi Kapat yöntemi olan kaynaklardır (Ornekleyici, SiraliOkuyucu).
type kapatici interface{ Kapat() error }

type sonucTopluIs struct {
	ti  *TopluIs
	hat error
}

// OnYukle, kaynaktan önden yüklemeye başlar. derinlik < 1 ise 2 kullanılır.
// ctx iptal edilirse üretici durur ve Al bağlam hatasını döner.
func OnYukle(ctx context.Context, k Kaynak, derinlik int) *OnYukleyici {
	if derinlik < 1 {
		derinlik = 2
	}
	c, iptal := context.WithCancel(ctx)
	o := &OnYukleyici{
		kaynak: k,
		bos:    make(chan *TopluIs, derinlik),
		dolu:   make(chan sonucTopluIs, derinlik),
		ctx:    c,
		iptal:  iptal,
	}
	for i := 0; i < derinlik; i++ {
		o.bos <- &TopluIs{}
	}
	o.wg.Add(1)
	go o.uret()
	return o
}

func (o *OnYukleyici) uret() {
	defer o.wg.Done()
	for {
		var ti *TopluIs
		select {
		case <-o.ctx.Done():
			return
		case ti = <-o.bos:
		}
		err := o.kaynak.Doldur(o.ctx, ti)
		if o.ctx.Err() != nil {
			// Kapatma sırasında yarım kalan iş tüketiciye verilmez.
			return
		}
		select {
		case o.dolu <- sonucTopluIs{ti: ti, hat: err}:
		case <-o.ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

// Al, sonraki toplu işi dst'ye yerleştirir. dst'nin eski tamponu yeniden
// kullanılmak üzere üreticiye verilir; bu yüzden önceki Al'dan dönen veri
// dilimleri Al'dan sonra geçersizdir (saklanacaksa Kopya alınmalı).
// Kaynak bittiğinde io.EOF, kapatıldıktan sonra ErrKapali döner.
func (o *OnYukleyici) Al(dst *TopluIs) error {
	o.mu.Lock()
	if o.kaliciHt != nil {
		err := o.kaliciHt
		o.mu.Unlock()
		return err
	}
	o.mu.Unlock()
	var s sonucTopluIs
	select {
	case s = <-o.dolu:
	case <-o.ctx.Done():
		// Kuyrukta hazır iş varsa önce onu ver (bağlam iptali dışında).
		select {
		case s = <-o.dolu:
		default:
			return o.sonHata(o.kapatmaHatasi())
		}
	}
	if s.hat != nil {
		// Tamponu geri vermeye gerek yok; üretici durdu.
		return o.sonHata(s.hat)
	}
	eski := dst.Veri
	*dst = *s.ti
	s.ti.Veri = eski[:0]
	s.ti.B, s.ti.T = 0, 0
	// bos kapasitesi derinlik; dolaşımdaki tampon sayısı hiç artmaz.
	select {
	case o.bos <- s.ti:
	default:
	}
	return nil
}

func (o *OnYukleyici) kapatmaHatasi() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.kapandi {
		return ErrKapali
	}
	return o.ctx.Err()
}

func (o *OnYukleyici) sonHata(err error) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.kaliciHt == nil {
		o.kaliciHt = err
	}
	return o.kaliciHt
}

// Kapat, üreticiyi durdurur, bitmesini bekler ve ardından kaynağı (kapatılabilir
// ise) kapatır. Birden çok kez çağrılabilir; kaynak yalnızca bir kez kapatılır.
func (o *OnYukleyici) Kapat() error { return o.kapat(0) }

// KapatZamanli, Kapat gibidir ama üretici sure içinde durmazsa (ör. ağ
// okuması takıldıysa) beklemeyi bırakıp hata döner. Bu durumda kaynak
// KAPATILMAZ (üretici hâlâ onu kullanıyor olabilir); üretici gorutini,
// takılan okuma dönünce kendiliğinden biter. Sonraki bir Kapat çağrısı
// üreticiyi yeniden bekler ve kaynağı kapatır.
func (o *OnYukleyici) KapatZamanli(sure time.Duration) error {
	if sure <= 0 {
		return o.kapat(0)
	}
	return o.kapat(sure)
}

func (o *OnYukleyici) kapat(sure time.Duration) error {
	o.mu.Lock()
	o.kapandi = true
	o.kaliciHt = ErrKapali
	o.mu.Unlock()
	o.iptal()
	if sure > 0 {
		bitti := make(chan struct{})
		go func() { o.wg.Wait(); close(bitti) }()
		zaman := time.NewTimer(sure)
		defer zaman.Stop()
		select {
		case <-bitti:
		case <-zaman.C:
			return fmt.Errorf("veri: önden yükleyici %v içinde durmadı (takılan okuma?); kaynak kapatılmadı", sure)
		}
	} else {
		o.wg.Wait()
	}
	o.kaynakBir.Do(func() {
		switch k := o.kaynak.(type) {
		case kapatici:
			o.kaynakHt = k.Kapat()
		case io.Closer:
			o.kaynakHt = k.Close()
		}
	})
	return o.kaynakHt
}

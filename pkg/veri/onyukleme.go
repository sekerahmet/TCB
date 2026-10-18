package veri

import (
	"context"
	"sync"
)

// OnYukleyici, bir Kaynak'tan sonraki toplu işleri arka plandaki tek bir
// gorutinde hazırlar. derinlik adet tampon dolaşımdadır: üretici boş
// tamponları doldurup kuyruğa koyar, Al dolu bir tamponu çağıranın
// TopluIs'i ile takas eder (kopyalama yok) ve eskisini üreticiye geri verir.
// Toplu işlerin sırası kaynağın üretim sırasıyla aynıdır.
//
// Kapat (ya da verilen bağlamın iptali) üreticiyi durdurur ve gorutinin
// bitmesini bekler. Kaynağın kendisini kapatmak çağıranın işidir.
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
}

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

// Kapat, üreticiyi durdurur ve bitmesini bekler. Birden çok kez çağrılabilir.
func (o *OnYukleyici) Kapat() error {
	o.mu.Lock()
	o.kapandi = true
	o.mu.Unlock()
	o.iptal()
	o.wg.Wait()
	o.mu.Lock()
	o.kaliciHt = ErrKapali
	o.mu.Unlock()
	return nil
}

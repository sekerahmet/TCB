package dataload

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Prefetcher, bir Kaynak'tan sonraki toplu işleri arka plandaki tek bir
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
type Prefetcher struct {
	src    Source
	empty  chan *Batch
	full   chan resultBatch
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu            sync.Mutex
	persistentErr error // kaynak hatası ya da kapatma; sonraki Al'lar bunu döner
	wasClosed     bool

	srcOne sync.Once
	srcErr error
}

// closer, kendi Kapat yöntemi olan kaynaklardır (Ornekleyici, SiraliOkuyucu).
type closer interface{ Close() error }

type resultBatch struct {
	ti    *Batch
	fault error
}

// Prefetch, kaynaktan önden yüklemeye başlar. derinlik < 1 ise 2 kullanılır.
// ctx iptal edilirse üretici durur ve Al bağlam hatasını döner.
func Prefetch(ctx context.Context, k Source, depth int) *Prefetcher {
	if depth < 1 {
		depth = 2
	}
	c, cancel := context.WithCancel(ctx)
	o := &Prefetcher{
		src:    k,
		empty:  make(chan *Batch, depth),
		full:   make(chan resultBatch, depth),
		ctx:    c,
		cancel: cancel,
	}
	for i := 0; i < depth; i++ {
		o.empty <- &Batch{}
	}
	o.wg.Add(1)
	go o.produce()
	return o
}

func (o *Prefetcher) produce() {
	defer o.wg.Done()
	for {
		var ti *Batch
		select {
		case <-o.ctx.Done():
			return
		case ti = <-o.empty:
		}
		err := o.src.Fill(o.ctx, ti)
		if o.ctx.Err() != nil {
			// Kapatma sırasında yarım kalan iş tüketiciye verilmez.
			return
		}
		select {
		case o.full <- resultBatch{ti: ti, fault: err}:
		case <-o.ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

// Take, sonraki toplu işi dst'ye yerleştirir. dst'nin eski tamponu yeniden
// kullanılmak üzere üreticiye verilir; bu yüzden önceki Al'dan dönen veri
// dilimleri Al'dan sonra geçersizdir (saklanacaksa Kopya alınmalı).
// Kaynak bittiğinde io.EOF, kapatıldıktan sonra ErrKapali döner.
func (o *Prefetcher) Take(dst *Batch) error {
	o.mu.Lock()
	if o.persistentErr != nil {
		err := o.persistentErr
		o.mu.Unlock()
		return err
	}
	o.mu.Unlock()
	var s resultBatch
	select {
	case s = <-o.full:
	case <-o.ctx.Done():
		// Kuyrukta hazır iş varsa önce onu ver (bağlam iptali dışında).
		select {
		case s = <-o.full:
		default:
			return o.lastErr(o.closeError())
		}
	}
	if s.fault != nil {
		// Tamponu geri vermeye gerek yok; üretici durdu.
		return o.lastErr(s.fault)
	}
	old := dst.Data
	*dst = *s.ti
	s.ti.Data = old[:0]
	s.ti.B, s.ti.T = 0, 0
	// bos kapasitesi derinlik; dolaşımdaki tampon sayısı hiç artmaz.
	select {
	case o.empty <- s.ti:
	default:
	}
	return nil
}

func (o *Prefetcher) closeError() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.wasClosed {
		return ErrClosed
	}
	return o.ctx.Err()
}

func (o *Prefetcher) lastErr(err error) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.persistentErr == nil {
		o.persistentErr = err
	}
	return o.persistentErr
}

// Close, üreticiyi durdurur, bitmesini bekler ve ardından kaynağı (kapatılabilir
// ise) kapatır. Birden çok kez çağrılabilir; kaynak yalnızca bir kez kapatılır.
func (o *Prefetcher) Close() error { return o.closeFn(0) }

// CloseTimeout, Kapat gibidir ama üretici sure içinde durmazsa (ör. ağ
// okuması takıldıysa) beklemeyi bırakıp hata döner. Bu durumda kaynak
// KAPATILMAZ (üretici hâlâ onu kullanıyor olabilir); üretici gorutini,
// takılan okuma dönünce kendiliğinden biter. Sonraki bir Kapat çağrısı
// üreticiyi yeniden bekler ve kaynağı kapatır.
func (o *Prefetcher) CloseTimeout(duration time.Duration) error {
	if duration <= 0 {
		return o.closeFn(0)
	}
	return o.closeFn(duration)
}

func (o *Prefetcher) closeFn(duration time.Duration) error {
	o.mu.Lock()
	o.wasClosed = true
	o.persistentErr = ErrClosed
	o.mu.Unlock()
	o.cancel()
	if duration > 0 {
		finished := make(chan struct{})
		go func() { o.wg.Wait(); close(finished) }()
		when := time.NewTimer(duration)
		defer when.Stop()
		select {
		case <-finished:
		case <-when.C:
			return fmt.Errorf("veri: önden yükleyici %v içinde durmadı (takılan okuma?); kaynak kapatılmadı", duration)
		}
	} else {
		o.wg.Wait()
	}
	o.srcOne.Do(func() {
		switch k := o.src.(type) {
		case closer:
			o.srcErr = k.Close()
		case io.Closer:
			o.srcErr = k.Close()
		}
	})
	return o.srcErr
}

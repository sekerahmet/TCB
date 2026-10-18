package veri

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// sayacKaynak, sırayla 0,1,2,... değerli toplu işler üretir; sinir'e
// ulaşınca io.EOF döner. gecikme verilirse her Doldur o kadar bekler;
// durSonra verilirse o kadar çağrıdan sonraki Doldur'lar bağlam iptal
// edilene kadar takılır.
type sayacKaynak struct {
	sinir    int
	i        int
	gecikme  time.Duration
	durSonra int
	cagri    atomic.Int64
}

func (s *sayacKaynak) Doldur(ctx context.Context, ti *TopluIs) error {
	if n := s.cagri.Add(1); s.durSonra > 0 && n > int64(s.durSonra) {
		<-ctx.Done()
		return ctx.Err()
	}
	if s.gecikme > 0 {
		select {
		case <-time.After(s.gecikme):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.sinir > 0 && s.i >= s.sinir {
		return io.EOF
	}
	ti.hazirla(2, 3)
	for j := range ti.Veri {
		ti.Veri[j] = int32(s.i)
	}
	ti.Durum = Durum{Pencere: int64(s.i)}
	s.i++
	return nil
}

func TestOnYukleyiciSiraVeEOF(t *testing.T) {
	taban := runtime.NumGoroutine()
	o := OnYukle(context.Background(), &sayacKaynak{sinir: 100}, 3)
	var ti TopluIs
	var oncekiVeri []int32
	for i := 0; i < 100; i++ {
		if err := o.Al(&ti); err != nil {
			t.Fatal(i, err)
		}
		if ti.Veri[0] != int32(i) || ti.Veri[len(ti.Veri)-1] != int32(i) || ti.Durum.Pencere != int64(i) {
			t.Fatalf("sıra bozuk: %d, %v", i, ti.Veri)
		}
		if oncekiVeri != nil && &oncekiVeri[0] == &ti.Veri[0] {
			t.Fatal("ardışık Al'lar aynı tamponu döndürdü")
		}
		oncekiVeri = ti.Veri
	}
	for i := 0; i < 2; i++ {
		if err := o.Al(&ti); err != io.EOF {
			t.Fatalf("io.EOF bekleniyordu: %v", err)
		}
	}
	_ = o.Kapat()
	if err := o.Al(&ti); !errors.Is(err, ErrKapali) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	gorutinBekle(t, taban)
}

func TestOnYukleyiciKapatmaSizintisiz(t *testing.T) {
	taban := runtime.NumGoroutine()
	// Tüketilmeden kapatma: üretici dolu kuyruğa yazmaya çalışırken bloke.
	k := &sayacKaynak{}
	o := OnYukle(context.Background(), k, 2)
	deadline := time.Now().Add(2 * time.Second)
	for k.cagri.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	_ = o.Kapat()
	_ = o.Kapat()
	gorutinBekle(t, taban)

	// Yavaş kaynak sürerken kapatma: Doldur bağlamla kesilmeli.
	o = OnYukle(context.Background(), &sayacKaynak{gecikme: time.Hour}, 2)
	time.Sleep(10 * time.Millisecond)
	bitti := make(chan struct{})
	go func() { _ = o.Kapat(); close(bitti) }()
	select {
	case <-bitti:
	case <-time.After(3 * time.Second):
		t.Fatal("Kapat yavaş kaynakta takıldı")
	}
	gorutinBekle(t, taban)

	// Bekleyen Al, Kapat ile uyanmalı.
	o = OnYukle(context.Background(), &sayacKaynak{gecikme: time.Hour}, 2)
	hata := make(chan error, 1)
	go func() {
		var ti TopluIs
		hata <- o.Al(&ti)
	}()
	time.Sleep(10 * time.Millisecond)
	_ = o.Kapat()
	if err := <-hata; !errors.Is(err, ErrKapali) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	gorutinBekle(t, taban)

	// Üst bağlamın iptali de üreticiyi durdurur.
	ctx, iptal := context.WithCancel(context.Background())
	o = OnYukle(ctx, &sayacKaynak{gecikme: time.Hour}, 2)
	iptal()
	var ti TopluIs
	if err := o.Al(&ti); !errors.Is(err, context.Canceled) {
		t.Fatalf("context.Canceled bekleniyordu: %v", err)
	}
	gorutinBekle(t, taban) // Kapat çağrılmadan da gorutin bitmeli
	_ = o.Kapat()
	gorutinBekle(t, taban)
}

func TestOnYukleyiciOrnekleyiciIleAyniSira(t *testing.T) {
	parcaSizintiDenetle(t)
	taban := runtime.NumGoroutine()
	d := kodluKume(t, 3000, 1000, 2500)
	k, _ := KumeAc(d, KumeSecenekleri{})
	sec := OrnekleyiciSecenekleri{B: 4, T: 32, Tohum: 11}
	dogrudan, _ := k.Ornekleyici(sec)
	beklenen := toplulariAl(t, dogrudan, 120)
	_ = dogrudan.Kapat()

	ork, _ := k.Ornekleyici(sec)
	o := OnYukle(context.Background(), ork, 4)
	var ti TopluIs
	var alinan [][]int32
	for i := 0; i < 120; i++ {
		if err := o.Al(&ti); err != nil {
			t.Fatal(err)
		}
		alinan = append(alinan, append([]int32(nil), ti.Veri...))
	}
	_ = o.Kapat()
	_ = ork.Kapat()
	if !reflect.DeepEqual(alinan, beklenen) {
		t.Fatal("önden yükleme sırayı bozdu")
	}
	gorutinBekle(t, taban)
}

func TestOnYukleyiciBeklemez(t *testing.T) {
	// Üretici tüketiciden hızlıysa, tüketici hazır kuyruktan beklemeden
	// almalı. Duvar saatiyle ölçmek yavaş CI makinelerinde ve Windows'un
	// kaba zamanlayıcısında kararsızdı; bunun yerine kaynak 4 toplu işten
	// sonra takılır. Kuyruk doluyken 4 Al ancak kuyruktan alınırsa döner
	// (kaynağı bekleseydi takılırdı).
	k := &sayacKaynak{durSonra: 4}
	o := OnYukle(context.Background(), k, 4)
	defer o.Kapat()
	son := time.Now().Add(5 * time.Second)
	for len(o.dolu) < 4 {
		if time.Now().After(son) {
			t.Fatalf("kuyruk dolmadı: %d/4", len(o.dolu))
		}
		time.Sleep(time.Millisecond)
	}
	bitti := make(chan error, 1)
	go func() {
		var ti TopluIs
		for i := 0; i < 4; i++ {
			if err := o.Al(&ti); err != nil {
				bitti <- err
				return
			}
			if ti.Veri[0] != int32(i) {
				bitti <- fmt.Errorf("sıra bozuk: %d, %v", i, ti.Veri)
				return
			}
		}
		bitti <- nil
	}()
	select {
	case err := <-bitti:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("hazır toplu işler için kaynak beklendi")
	}
}

// izliKaynak, Doldur sürerken Kapat çağrılırsa bunu kaydeder.
type izliKaynak struct {
	sayacKaynak
	calisiyor atomic.Bool
	kapatma   atomic.Int64
	yaris     atomic.Bool
}

func (k *izliKaynak) Doldur(ctx context.Context, ti *TopluIs) error {
	k.calisiyor.Store(true)
	defer k.calisiyor.Store(false)
	return k.sayacKaynak.Doldur(ctx, ti)
}

func (k *izliKaynak) Kapat() error {
	if k.calisiyor.Load() {
		k.yaris.Store(true)
	}
	k.kapatma.Add(1)
	return nil
}

func TestOnYukleyiciKaynagiUreticidenSonraKapatir(t *testing.T) {
	taban := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		k := &izliKaynak{sayacKaynak: sayacKaynak{gecikme: time.Millisecond}}
		o := OnYukle(context.Background(), k, 2)
		var ti TopluIs
		_ = o.Al(&ti)
		_ = o.Kapat()
		_ = o.Kapat()
		if k.yaris.Load() {
			t.Fatal("kaynak, üretici Doldur içindeyken kapatıldı")
		}
		if k.kapatma.Load() != 1 {
			t.Fatalf("kaynak %d kez kapatıldı, 1 bekleniyordu", k.kapatma.Load())
		}
	}
	gorutinBekle(t, taban)
}

// takilanKaynak, bağlamı dinlemeyen ve birak kapanana kadar takılan bir
// okumayı taklit eder.
type takilanKaynak struct {
	birak     chan struct{}
	kapatildi atomic.Bool
}

func (k *takilanKaynak) Doldur(ctx context.Context, ti *TopluIs) error {
	<-k.birak
	return io.EOF
}

func (k *takilanKaynak) Kapat() error { k.kapatildi.Store(true); return nil }

func TestOnYukleyiciZamanliKapatma(t *testing.T) {
	taban := runtime.NumGoroutine()
	k := &takilanKaynak{birak: make(chan struct{})}
	o := OnYukle(context.Background(), k, 2)
	time.Sleep(5 * time.Millisecond)
	bas := time.Now()
	err := o.KapatZamanli(30 * time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "durmadı") {
		t.Fatalf("zaman aşımı hatası bekleniyordu: %v", err)
	}
	if gecen := time.Since(bas); gecen > time.Second {
		t.Fatalf("zamanlı kapatma çok uzun sürdü: %v", gecen)
	}
	if k.kapatildi.Load() {
		t.Fatal("üretici çalışırken kaynak kapatıldı")
	}
	var ti TopluIs
	if err := o.Al(&ti); !errors.Is(err, ErrKapali) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	close(k.birak) // takılan okuma döner
	if err := o.Kapat(); err != nil {
		t.Fatal(err)
	}
	if !k.kapatildi.Load() {
		t.Fatal("kaynak sonunda kapatılmadı")
	}
	gorutinBekle(t, taban)
}

func TestOrnekleyiciEszamanliKapatma(t *testing.T) {
	parcaSizintiDenetle(t)
	d := kodluKume(t, 5000, 5000, 5000)
	k, _ := KumeAc(d, KumeSecenekleri{})
	for i := 0; i < 20; i++ {
		o, _ := k.Ornekleyici(OrnekleyiciSecenekleri{B: 16, T: 31, Tohum: uint64(i)})
		bitti := make(chan error, 1)
		go func() {
			var ti TopluIs
			for {
				if err := o.Sonraki(&ti); err != nil {
					bitti <- err
					return
				}
			}
		}()
		time.Sleep(time.Duration(i%3) * time.Millisecond)
		if err := o.Kapat(); err != nil {
			t.Fatal(err)
		}
		if err := <-bitti; !errors.Is(err, ErrKapali) {
			t.Fatalf("ErrKapali bekleniyordu: %v", err)
		}
	}
	// Önden yükleyici örnekleyiciyi kendisi kapatır.
	o, _ := k.Ornekleyici(OrnekleyiciSecenekleri{B: 4, T: 31})
	y := OnYukle(context.Background(), o, 3)
	var ti TopluIs
	if err := y.Al(&ti); err != nil {
		t.Fatal(err)
	}
	_ = y.Kapat()
	if err := o.Sonraki(&ti); !errors.Is(err, ErrKapali) {
		t.Fatalf("önden yükleyici örnekleyiciyi kapatmadı: %v", err)
	}
}

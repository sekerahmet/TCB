package veri

import (
	"context"
	"errors"
	"io"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// sayacKaynak, sırayla 0,1,2,... değerli toplu işler üretir; sinir'e
// ulaşınca io.EOF döner. gecikme verilirse her Doldur o kadar bekler.
type sayacKaynak struct {
	sinir   int
	i       int
	gecikme time.Duration
	cagri   atomic.Int64
}

func (s *sayacKaynak) Doldur(ctx context.Context, ti *TopluIs) error {
	s.cagri.Add(1)
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
	// Üretici tüketiciden hızlıysa, tüketici hazır kuyruktan anında almalı.
	k := &sayacKaynak{gecikme: 2 * time.Millisecond}
	o := OnYukle(context.Background(), k, 4)
	defer o.Kapat()
	time.Sleep(50 * time.Millisecond) // kuyruk dolsun
	var ti TopluIs
	bas := time.Now()
	for i := 0; i < 4; i++ {
		if err := o.Al(&ti); err != nil {
			t.Fatal(err)
		}
	}
	if gecen := time.Since(bas); gecen > 20*time.Millisecond {
		t.Fatalf("hazır toplu işler için bekleme oldu: %v", gecen)
	}
}

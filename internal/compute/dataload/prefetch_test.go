package dataload

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
// edilene kadar takılır. basladi verilirse her Doldur başlarken ona (doluysa
// atlayarak) bir sinyal gönderir.
type countingSource struct {
	limit     int
	i         int
	delay     time.Duration
	stopAfter int
	calls     atomic.Int64
	started   chan struct{}
}

func (s *countingSource) Fill(ctx context.Context, ti *Batch) error {
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if n := s.calls.Add(1); s.stopAfter > 0 && n > int64(s.stopAfter) {
		<-ctx.Done()
		return ctx.Err()
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.limit > 0 && s.i >= s.limit {
		return io.EOF
	}
	ti.prepare(2, 3)
	for j := range ti.Data {
		ti.Data[j] = int32(s.i)
	}
	ti.State = State{Window: int64(s.i)}
	s.i++
	return nil
}

func TestPrefetcherOrderAndEOF(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	o := Prefetch(context.Background(), &countingSource{limit: 100}, 3)
	var ti Batch
	var prevData []int32
	for i := 0; i < 100; i++ {
		if err := o.Take(&ti); err != nil {
			t.Fatal(i, err)
		}
		if ti.Data[0] != int32(i) || ti.Data[len(ti.Data)-1] != int32(i) || ti.State.Window != int64(i) {
			t.Fatalf("sıra bozuk: %d, %v", i, ti.Data)
		}
		if prevData != nil && &prevData[0] == &ti.Data[0] {
			t.Fatal("ardışık Al'lar aynı tamponu döndürdü")
		}
		prevData = ti.Data
	}
	for i := 0; i < 2; i++ {
		if err := o.Take(&ti); err != io.EOF {
			t.Fatalf("io.EOF bekleniyordu: %v", err)
		}
	}
	_ = o.Close()
	if err := o.Take(&ti); !errors.Is(err, ErrClosed) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	waitGoroutine(t, baseOff)
}

func TestPrefetcherCloseNoLeak(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	// Tüketilmeden kapatma: üretici dolu kuyruğa yazmaya çalışırken bloke.
	k := &countingSource{}
	o := Prefetch(context.Background(), k, 2)
	deadline := time.Now().Add(2 * time.Second)
	for k.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	_ = o.Close()
	_ = o.Close()
	waitGoroutine(t, baseOff)

	// Yavaş kaynak sürerken kapatma: Doldur bağlamla kesilmeli.
	slow := &countingSource{delay: time.Hour, started: make(chan struct{}, 1)}
	o = Prefetch(context.Background(), slow, 2)
	waitSignal(t, slow.started, "Doldur başlaması")
	finished := make(chan struct{})
	go func() { _ = o.Close(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Kapat yavaş kaynakta takıldı")
	}
	waitGoroutine(t, baseOff)

	// Bekleyen Al, Kapat ile uyanmalı.
	o = Prefetch(context.Background(), &countingSource{delay: time.Hour}, 2)
	failure := make(chan error, 1)
	go func() {
		var ti Batch
		failure <- o.Take(&ti)
	}()
	goroutineWaiting(t, "(*Prefetcher).Take")
	_ = o.Close()
	if err := <-failure; !errors.Is(err, ErrClosed) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	waitGoroutine(t, baseOff)

	// Üst bağlamın iptali de üreticiyi durdurur.
	ctx, cancel := context.WithCancel(context.Background())
	o = Prefetch(ctx, &countingSource{delay: time.Hour}, 2)
	cancel()
	var ti Batch
	if err := o.Take(&ti); !errors.Is(err, context.Canceled) {
		t.Fatalf("context.Canceled bekleniyordu: %v", err)
	}
	waitGoroutine(t, baseOff) // Kapat çağrılmadan da gorutin bitmeli
	_ = o.Close()
	waitGoroutine(t, baseOff)
}

func TestPrefetcherSameOrderAsSampler(t *testing.T) {
	checkShardLeak(t)
	baseOff := runtime.NumGoroutine()
	d := encodedDataset(t, 3000, 1000, 2500)
	k, _ := OpenDataset(d, DatasetOptions{})
	pick := SamplerOptions{B: 4, T: 32, Seed: 11}
	direct, _ := k.Sampler(pick)
	expected := takeBatches(t, direct, 120)
	_ = direct.Close()

	ork, _ := k.Sampler(pick)
	o := Prefetch(context.Background(), ork, 4)
	var ti Batch
	var taken [][]int32
	for i := 0; i < 120; i++ {
		if err := o.Take(&ti); err != nil {
			t.Fatal(err)
		}
		taken = append(taken, append([]int32(nil), ti.Data...))
	}
	_ = o.Close()
	_ = ork.Close()
	if !reflect.DeepEqual(taken, expected) {
		t.Fatal("önden yükleme sırayı bozdu")
	}
	waitGoroutine(t, baseOff)
}

func TestPrefetcherDoesNotBlock(t *testing.T) {
	// Üretici tüketiciden hızlıysa, tüketici hazır kuyruktan beklemeden
	// almalı. Duvar saatiyle ölçmek yavaş CI makinelerinde ve Windows'un
	// kaba zamanlayıcısında kararsızdı; bunun yerine kaynak 4 toplu işten
	// sonra takılır. Kuyruk doluyken 4 Al ancak kuyruktan alınırsa döner
	// (kaynağı bekleseydi takılırdı).
	k := &countingSource{stopAfter: 4}
	o := Prefetch(context.Background(), k, 4)
	defer o.Close()
	last := time.Now().Add(5 * time.Second)
	for len(o.full) < 4 {
		if time.Now().After(last) {
			t.Fatalf("kuyruk dolmadı: %d/4", len(o.full))
		}
		time.Sleep(time.Millisecond)
	}
	finished := make(chan error, 1)
	go func() {
		var ti Batch
		for i := 0; i < 4; i++ {
			if err := o.Take(&ti); err != nil {
				finished <- err
				return
			}
			if ti.Data[0] != int32(i) {
				finished <- fmt.Errorf("sıra bozuk: %d, %v", i, ti.Data)
				return
			}
		}
		finished <- nil
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("hazır toplu işler için kaynak beklendi")
	}
}

// izliKaynak, Doldur sürerken Kapat çağrılırsa bunu kaydeder.
type trackedSource struct {
	countingSource
	running atomic.Bool
	closing atomic.Int64
	race    atomic.Bool
}

func (k *trackedSource) Fill(ctx context.Context, ti *Batch) error {
	k.running.Store(true)
	defer k.running.Store(false)
	return k.countingSource.Fill(ctx, ti)
}

func (k *trackedSource) Close() error {
	if k.running.Load() {
		k.race.Store(true)
	}
	k.closing.Add(1)
	return nil
}

func TestPrefetcherClosesSourceAfterProducer(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		k := &trackedSource{countingSource: countingSource{delay: time.Millisecond}}
		o := Prefetch(context.Background(), k, 2)
		var ti Batch
		_ = o.Take(&ti)
		_ = o.Close()
		_ = o.Close()
		if k.race.Load() {
			t.Fatal("kaynak, üretici Doldur içindeyken kapatıldı")
		}
		if k.closing.Load() != 1 {
			t.Fatalf("kaynak %d kez kapatıldı, 1 bekleniyordu", k.closing.Load())
		}
	}
	waitGoroutine(t, baseOff)
}

// takilanKaynak, bağlamı dinlemeyen ve birak kapanana kadar takılan bir
// okumayı taklit eder; okumaya girince basladi kapanır.
type stuckSource struct {
	release  chan struct{}
	started  chan struct{}
	isClosed atomic.Bool
}

func (k *stuckSource) Fill(ctx context.Context, ti *Batch) error {
	close(k.started)
	<-k.release
	return io.EOF
}

func (k *stuckSource) Close() error { k.isClosed.Store(true); return nil }

func TestPrefetcherCloseTimeout(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	k := &stuckSource{release: make(chan struct{}), started: make(chan struct{})}
	o := Prefetch(context.Background(), k, 2)
	// Üretici takılan okumaya girmeden kapatılsaydı bağlamı görüp hemen durur, zaman aşımı
	// sınanmamış olurdu.
	waitSignal(t, k.started, "takılan okumanın başlaması")
	begin := time.Now()
	err := o.CloseTimeout(30 * time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "durmadı") {
		t.Fatalf("zaman aşımı hatası bekleniyordu: %v", err)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Fatalf("zamanlı kapatma çok uzun sürdü: %v", elapsed)
	}
	if k.isClosed.Load() {
		t.Fatal("üretici çalışırken kaynak kapatıldı")
	}
	var ti Batch
	if err := o.Take(&ti); !errors.Is(err, ErrClosed) {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	close(k.release) // takılan okuma döner
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	if !k.isClosed.Load() {
		t.Fatal("kaynak sonunda kapatılmadı")
	}
	waitGoroutine(t, baseOff)
}

func TestSamplerConcurrentClose(t *testing.T) {
	checkShardLeak(t)
	d := encodedDataset(t, 5000, 5000, 5000)
	k, _ := OpenDataset(d, DatasetOptions{})
	for i := 0; i < 20; i++ {
		o, _ := k.Sampler(SamplerOptions{B: 16, T: 31, Seed: uint64(i)})
		finished := make(chan error, 1)
		go func() {
			var ti Batch
			for {
				if err := o.Next(&ti); err != nil {
					finished <- err
					return
				}
			}
		}()
		time.Sleep(time.Duration(i%3) * time.Millisecond)
		if err := o.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-finished; !errors.Is(err, ErrClosed) {
			t.Fatalf("ErrKapali bekleniyordu: %v", err)
		}
	}
	// Önden yükleyici örnekleyiciyi kendisi kapatır.
	o, _ := k.Sampler(SamplerOptions{B: 4, T: 31})
	y := Prefetch(context.Background(), o, 3)
	var ti Batch
	if err := y.Take(&ti); err != nil {
		t.Fatal(err)
	}
	_ = y.Close()
	if err := o.Next(&ti); !errors.Is(err, ErrClosed) {
		t.Fatalf("önden yükleyici örnekleyiciyi kapatmadı: %v", err)
	}
}

func BenchmarkPrefetcher(b *testing.B) {
	const B, T = 32, 1024
	k := benchDataset(b, true)
	o, err := k.Sampler(SamplerOptions{B: B, T: T, Seed: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer o.Close()
	y := Prefetch(context.Background(), o, 4)
	defer y.Close()
	var ti Batch
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := y.Take(&ti); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N)*B*T/b.Elapsed().Seconds(), "token/s")
}

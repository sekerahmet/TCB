package dataload

import (
	"context"
	"os"
	"testing"
	"time"
)

// İki işçide eski kuşağın listesinden en son dönen işçi de bekleyen işi bitmiş saymalı: yeni
// liste öbür işçi tarafından işlendiyse Wait döner (eskiden pending hep doğru kalır, Wait
// asılırdı).
func TestCacheWaitAfterStaleWorker(t *testing.T) {
	src := t.TempDir()
	a, size := sizedFile(t, src, "a.npy", 1000, 1)
	b, _ := sizedFile(t, src, "b.npy", 1000, 2)
	started, release := make(chan struct{}), make(chan struct{})
	copyHook = func(k string, prefetch bool) {
		if prefetch && k == a {
			close(started)
			<-release
		}
	}
	defer func() { copyHook = nil }()
	o, err := OpenCache(CacheOptions{Dir: t.TempDir(), Quota: 4 * size, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	c := o.c
	o.Prepare([]string{a}) // kuşak 1: bir işçi a'da takılır, diğeri a'yı atlar
	<-started
	o.Prepare([]string{b}) // kuşak 2: iki uyarı; boştaki işçi ikisini de tüketir
	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.Lock()
		idle := len(c.warning) == 0 && c.active == 1
		c.mu.Unlock()
		if idle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("hızlı işçi boşa düşmedi")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release) // bayat işçi en son biter
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = o.Wait(ctx)
	c.mu.Lock()
	pending := c.pending
	c.mu.Unlock()
	if err != nil {
		t.Fatalf("Wait asıldı: %v (pending %v)", err, pending)
	}
}

// Kural: bir kopyalama hatası sessizce yutulmaz ve önbellekte iz bırakmaz: ön planda okuyan
// çağrı hatayı kendisi alır, sonraki çağrı kaynak düzelince yeniden kopyalar. Arka plan
// kopyalamasının hatası bu yüzden ayrıca saklanmaz (LastError kaldırıldı).
func TestCacheCopyFailureReachesTheForegroundAndLeavesNoRecord(t *testing.T) {
	src, local := t.TempDir(), t.TempDir()
	a, size := sizedFile(t, src, "a.npy", 1000, 1)
	o, err := OpenCache(CacheOptions{Dir: local, Quota: 3 * size})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	data, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	copyHook = func(k string, prefetch bool) { _ = os.Remove(k) } // Stat'tan sonra kaynak kaybolur
	_, _, err = o.Fetch(context.Background(), a)
	copyHook = nil
	if err == nil {
		t.Fatal("kopyalama hatası ön plandaki çağrıya ulaşmadı")
	}
	if err := os.WriteFile(a, data, 0o644); err != nil {
		t.Fatal(err)
	}
	y, release, err := o.Fetch(context.Background(), a)
	if err != nil {
		t.Fatalf("kaynak düzelince yeniden kopyalanmadı: %v", err)
	}
	release()
	if !isLocal(o, y) {
		t.Fatalf("yerel kopya bekleniyordu: %s", y)
	}
}

package dataset

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ----------------------------------------------------------------------------
// Kaynak yönetimi
// ----------------------------------------------------------------------------
//
// TR'de açık bir kapatma adı yoktur. Dosya tutamakları, bellek eşlemeleri ve arka plan
// gorutinleri üç yolla serbest bırakılır:
//
//  1. Sıralı okuyucu sona erdiğinde (sonraki() ilk kez yok döndürdüğünde) hemen.
//  2. TR nesnesi erişilemez olunca çöp toplayıcının sonlandırıcısıyla (runtime.SetFinalizer).
//     Kapatma ayrı bir gorutinde yapılır; takılan bir okuma diğer temizlikleri bekletmez.
//  3. Program bitince CloseAll ile (cmd/tr ana programı internal/stdlib.Close üzerinden çağırır).
//
// Kayıt defteri yalnızca Go nesnelerini (önden yükleyici, önbellek) tutar, TR nesnelerini
// tutmaz; böylece TR nesnesinin erişilemez olması engellenmez.

// level kapatma sırasıdır: önce okuyucular (önbelleği kullanırlar), sonra önbellekler.
type level int

const (
	levelReader level = iota
	levelCache
)

type resource struct {
	level   level
	closeFn func(timeout time.Duration) error
	prev    sync.Once
	err     error
	// next, sahibin kaynağı değişince (okuyucunun durumu_yükle'si) yerine geçen kaynaktır:
	// sahibin sonlandırıcısı zinciri izleyip o anki kaynağı da kapatır.
	next atomic.Pointer[resource]
}

var (
	registryMu sync.Mutex
	registry   = map[*resource]struct{}{}
)

// addResource bir kaynağı kayıt defterine ekler; sahip erişilemez olunca kapatılır.
// Sonlandırıcı yalnızca k'yi yakalar, sahibi yakalamaz (aksi hâlde sahip hiç toplanmazdı).
func addResource[T any](owner *T, s level, closeFn func(timeout time.Duration) error) *resource {
	k := registerResource(s, closeFn)
	runtime.SetFinalizer(owner, func(*T) { go k.closeChain() })
	return k
}

// registerResource bir kaynağı sonlandırıcı kurmadan kayıt defterine ekler.
func registerResource(s level, closeFn func(timeout time.Duration) error) *resource {
	k := &resource{level: s, closeFn: closeFn}
	registryMu.Lock()
	registry[k] = struct{}{}
	registryMu.Unlock()
	return k
}

// replace k'nin yerine yeni bir kaynak kaydeder ve k'yi kapatır; sahibin sonlandırıcısı
// (k'den başlayan zincir) yenisini de kapatır. Yeni kaynağı döndürür.
func (k *resource) replace(closeFn func(timeout time.Duration) error) *resource {
	fresh := registerResource(k.level, closeFn)
	k.next.Store(fresh)
	_ = k.Close(0)
	return fresh
}

// closeChain kaynağı ve yerine geçen bütün kaynakları kapatır.
func (k *resource) closeChain() {
	for c := k; c != nil; c = c.next.Load() {
		_ = c.Close(0)
	}
}

// Close kaynağı bir kez kapatır. sure > 0 ise takılan bir okumada en çok o kadar bekler.
func (k *resource) Close(timeout time.Duration) error {
	k.prev.Do(func() {
		k.err = k.closeFn(timeout)
		registryMu.Lock()
		delete(registry, k)
		registryMu.Unlock()
	})
	return k.err
}

// OpenResources kapatılmamış kaynak sayısıdır (testler için).
func OpenResources() int {
	registryMu.Lock()
	defer registryMu.Unlock()
	return len(registry)
}

// CloseAll açık tüm veri kaynaklarını (önden yükleyiciler, okuyucular, önbellekler)
// kapatır. Takılan bir okumada en çok sure kadar bekler; süre dolarsa kalanlar süreç
// bitince işletim sistemince serbest bırakılır. Birden çok kez çağrılabilir.
func CloseAll(timeout time.Duration) {
	last := time.Now().Add(timeout)
	for _, s := range []level{levelReader, levelCache} {
		registryMu.Lock()
		var list []*resource
		for k := range registry {
			if k.level == s {
				list = append(list, k)
			}
		}
		registryMu.Unlock()
		if len(list) == 0 {
			continue
		}
		left := time.Until(last)
		if left <= 0 {
			return
		}
		var wg sync.WaitGroup
		for _, k := range list {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = k.Close(left)
			}()
		}
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(left):
			return
		}
	}
}

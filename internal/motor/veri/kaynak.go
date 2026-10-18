package veri

import (
	"runtime"
	"sync"
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
//  3. Program bitince TumunuKapat ile (cmd/tr ana programı internal/motor.Kapat üzerinden çağırır).
//
// Kayıt defteri yalnızca Go nesnelerini (önden yükleyici, önbellek) tutar, TR nesnelerini
// tutmaz; böylece TR nesnesinin erişilemez olması engellenmez.

// seviye kapatma sırasıdır: önce okuyucular (önbelleği kullanırlar), sonra önbellekler.
type seviye int

const (
	seviyeOkuyucu seviye = iota
	seviyeOnbellek
)

type kaynak struct {
	seviye seviye
	kapat  func(sure time.Duration) error
	once   sync.Once
	err    error
}

var (
	kayitMu sync.Mutex
	kayit   = map[*kaynak]struct{}{}
)

// kaynakEkle bir kaynağı kayıt defterine ekler; sahip erişilemez olunca kapatılır.
// Sonlandırıcı yalnızca k'yi yakalar, sahibi yakalamaz (aksi hâlde sahip hiç toplanmazdı).
func kaynakEkle[T any](sahip *T, s seviye, kapat func(sure time.Duration) error) *kaynak {
	k := &kaynak{seviye: s, kapat: kapat}
	kayitMu.Lock()
	kayit[k] = struct{}{}
	kayitMu.Unlock()
	runtime.SetFinalizer(sahip, func(*T) { go k.Kapat(0) })
	return k
}

// Kapat kaynağı bir kez kapatır. sure > 0 ise takılan bir okumada en çok o kadar bekler.
func (k *kaynak) Kapat(sure time.Duration) error {
	k.once.Do(func() {
		k.err = k.kapat(sure)
		kayitMu.Lock()
		delete(kayit, k)
		kayitMu.Unlock()
	})
	return k.err
}

// AcikKaynakSayisi kapatılmamış kaynak sayısıdır (testler için).
func AcikKaynakSayisi() int {
	kayitMu.Lock()
	defer kayitMu.Unlock()
	return len(kayit)
}

// TumunuKapat açık tüm veri kaynaklarını (önden yükleyiciler, okuyucular, önbellekler)
// kapatır. Takılan bir okumada en çok sure kadar bekler; süre dolarsa kalanlar süreç
// bitince işletim sistemince serbest bırakılır. Birden çok kez çağrılabilir.
func TumunuKapat(sure time.Duration) {
	son := time.Now().Add(sure)
	for _, s := range []seviye{seviyeOkuyucu, seviyeOnbellek} {
		kayitMu.Lock()
		var liste []*kaynak
		for k := range kayit {
			if k.seviye == s {
				liste = append(liste, k)
			}
		}
		kayitMu.Unlock()
		if len(liste) == 0 {
			continue
		}
		kalan := time.Until(son)
		if kalan <= 0 {
			return
		}
		var wg sync.WaitGroup
		for _, k := range liste {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = k.Kapat(kalan)
			}()
		}
		bitti := make(chan struct{})
		go func() { wg.Wait(); close(bitti) }()
		select {
		case <-bitti:
		case <-time.After(kalan):
			return
		}
	}
}

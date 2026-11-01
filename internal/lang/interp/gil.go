package interp

import (
	"sync"
	"time"
)

// --- YORUMLAYICI KİLİDİ (10.2) ---
//
// Aynı anda yalnız bir görev TR kodu çalıştırır. Kilit bir bekleme kuyruğudur: kilidi bekleyen
// görevler onu geldikleri sırayla alır; bırakan görev kilidi doğrudan kuyruğun başındakine verir
// (Go'nun sync.Mutex'i sıra vermez). Kuyruk boşsa kilit serbest kalır, ilk gelen alır.
//
// Devir süreyledir: bekleyen bir görev varsa kilidi tutan görev, kilidi aldığından bu yana
// lockHold geçtikten sonraki ilk deyim sınırında onu devreder (yieldPoint). Deyim sınırları:
// blokta iki deyimin arası, döngü turunun başı ve kullanıcı fonksiyonunun girişi. Bekleyen yokken
// sınırın bedeli tek bir atomik okumadır; saat yalnız bekleyen varken okunur.

// lockHold, bekleyen varken kilidin bir görevde en çok kaldığı süredir (10.2: 5 ms).
const lockHold = 5 * time.Millisecond

// clockBase tekdüze saatin başlangıcıdır; monotonic ondan bu yana geçen süredir (time.Since
// tekdüze saati okur, duvar saatinin sıçramalarından etkilenmez).
var clockBase = time.Now()

func monotonic() time.Duration { return time.Since(clockBase) }

// gilQueue kilidin durumudur: tutuluyor mu ve bekleyenler, geliş sırasıyla.
type gilQueue struct {
	mu     sync.Mutex
	held   bool
	queue  []gilWaiter
	ticket uint64 // sıradaki bekleyenin numarası (testler sırayı bununla sınar)
}

// gilWaiter kuyruktaki bir görevdir: bırakan görev wake'e yazarak kilidi ona verir.
type gilWaiter struct {
	wake   chan struct{}
	ticket uint64
	ahead  int           // kuyruğa girdiğinde önünde bekleyen görev sayısı
	since  time.Duration // kuyruğa girdiği an (monotonic)
}

// acquireGIL kilidi alır: serbestse hemen, değilse kuyruğun sonunda bekleyerek.
func (v *VM) acquireGIL() {
	g := &v.gil
	g.mu.Lock()
	if !g.held {
		g.held = true
		g.mu.Unlock()
	} else {
		w := gilWaiter{wake: make(chan struct{}, 1), ticket: g.ticket, ahead: len(g.queue), since: monotonic()}
		g.ticket++
		g.queue = append(g.queue, w)
		v.pending.Add(1)
		g.mu.Unlock()
		<-w.wake // kilit bize devredildi (held açık kaldı)
		if h := v.gilWaitHook; h != nil {
			h(w.ahead, w.ticket, monotonic()-w.since)
		}
	}
	v.gilSince = monotonic()
	v.holdsGIL = true
	for _, f := range v.taskHooks {
		f(v)
	}
}

// releaseGIL kilidi bırakır: bekleyen varsa kuyruğun başındakine verir.
func (v *VM) releaseGIL() {
	v.holdsGIL = false
	g := &v.gil
	g.mu.Lock()
	if len(g.queue) == 0 {
		g.held = false
		g.mu.Unlock()
		return
	}
	w := g.queue[0]
	g.queue[0] = gilWaiter{}
	g.queue = g.queue[1:]
	v.pending.Add(-1)
	g.mu.Unlock()
	w.wake <- struct{}{}
}

// yieldPoint deyim sınırıdır: kilidi bekleyen bir görev varsa ve bu görev kilidi lockHold'dan
// uzun tuttuysa kilit devredilir. Bir deyimin okuma ve yazma adımları ('s += 1') arasında
// çağrılmaz; bu yüzden bölünmez.
func (v *VM) yieldPoint() {
	if v.pending.Load() != 0 {
		v.yieldSlow(false)
	}
}

// callBoundary kullanıcı fonksiyonunun girişidir: bir deyim sınırıdır ve uygulanmış durdurma
// burada da fark edilir (Interrupt sayacı artırdığından buraya girilir).
func (v *VM) callBoundary() {
	if v.pending.Load() != 0 {
		v.yieldSlow(true)
	}
}

//go:noinline
func (v *VM) yieldSlow(stopPoint bool) {
	if !v.holdsGIL || v.exclusive != nil {
		return
	}
	if stopPoint {
		v.checkInterrupt()
	}
	if monotonic()-v.gilSince < lockHold {
		return
	}
	v.handOffLock()
}

// loopTurn döngü turunun başındaki güvenli noktadır: bir deyim sınırıdır (yieldPoint) ve
// uygulanmış durdurma her turda fark edilir (1.5).
func (v *VM) loopTurn() {
	v.yieldPoint()
	v.checkInterrupt()
}

// handOffLock kilidi kuyruğun başındaki göreve verir ve kuyruğun sonuna geçer. Bırakma kilidi
// doğrudan bekleyene verdiği için ayrıca işlemciyi bırakmak (runtime.Gosched) gerekmez.
func (v *VM) handOffLock() {
	v.releaseGIL()
	v.acquireGIL()
	v.checkInterrupt()
}

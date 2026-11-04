package interp

import (
	"sync"
	"sync/atomic"
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
// blokta iki deyimin arası, döngü turunun başı ve kullanıcı fonksiyonunun girişi.
//
// Süreyi saat değil zamanlayıcı ölçer: kuyruğun başına bir görev geçince, tutanın lockHold'u
// dolduğu ana bir zamanlayıcı kurulur; süre dolunca zamanlayıcı tutanın kuşağını (kilidin kaçıncı
// alınışı) expired'a yazar. Deyim sınırında yalnız bu sayı okunur: saati her sınırda okumak
// Linux'ta (time.Now ~59 ns) sıkı döngünün süresinin dörtte birine varıyordu. Kuşak, eski tutan
// için kurulmuş, geç kalan bir zamanlayıcının yeni tutanı devretmemesi içindir. Zamanlayıcı yük
// altında geç kalabilir (çekirdekler doluyken 15–35 ms ölçüldü); yedek olarak bekleyen varken saat
// her lockClock.every sınırda bir de okunur (en çok clockEveryMax). Bekleyen yokken sınırın bedeli
// tek bir atomik okumadır.

// lockHold, bekleyen varken kilidin bir görevde en çok kaldığı süredir (10.2: 5 ms).
const lockHold = 5 * time.Millisecond

// clockGrain yedek saatin iki okuması arasında hedeflenen süredir; clockEveryMax iki okuma
// arasındaki deyim sınırı sayısının tavanıdır. Tavan, hızlı deyimlerden yavaşlara geçişte saatin
// en çok 64 yavaş deyim gecikmesini sağlar (sayı geçmiş deyimlerin hızından ayarlanır).
const (
	clockGrain    = 100 * time.Microsecond
	clockEveryMax = 64
)

// clockBase tekdüze saatin başlangıcıdır; monotonic ondan bu yana geçen süredir (time.Since
// tekdüze saati okur, duvar saatinin sıçramalarından etkilenmez).
var clockBase = time.Now()

func monotonic() time.Duration { return time.Since(clockBase) }

// gilQueue kilidin durumudur: tutuluyor mu, bekleyenler (geliş sırasıyla) ve tutanın süresi.
type gilQueue struct {
	mu     sync.Mutex
	held   bool
	queue  []gilWaiter
	ticket uint64 // sıradaki bekleyenin numarası (testler sırayı bununla sınar)
	// gen kilidin kaçıncı alınışıdır (kuşak), since alındığı an (monotonic); timer tutanın
	// süresinin zamanlayıcısıdır. Üçü de mu ile yazılır.
	gen   uint64
	since time.Duration
	timer *time.Timer
	// expired süresi dolan en son kuşaktır: zamanlayıcı yazar, kilidi tutan okur.
	expired atomic.Uint64
	// timerOff ve clockOff yalnız testler içindir: zamanlayıcıyı ya da yedek saati kapatır.
	timerOff, clockOff bool
	// afterFunc yalnız testler içindir: boş değilse zamanlayıcıyı time.AfterFunc yerine o kurar
	// (test kurulan fonksiyonu tutup istediği anda çağırır).
	afterFunc func(time.Duration, func()) *time.Timer
}

// gilWaiter kuyruktaki bir görevdir: bırakan görev wake'e yazarak kilidi ona verir.
type gilWaiter struct {
	wake   chan struct{}
	ticket uint64
	ahead  int           // kuyruğa girdiğinde önünde bekleyen görev sayısı
	since  time.Duration // kuyruğa girdiği an (monotonic)
}

// lockClock yedek saatin görevdeki durumudur (yalnız görevin kendisi okur ve yazar): saat her
// every sınırda bir okunur; skip sonraki okumaya kalan sınır, gap son iki okuma arasındaki
// sınırlar, last saatin son ilerlediği an, since o andan bu yana geçilen sınırlar.
type lockClock struct {
	every, skip, gap, since int
	last                    time.Duration
}

// acquireGIL kilidi alır: serbestse hemen, değilse kuyruğun sonunda bekleyerek.
func (v *VM) acquireGIL() {
	g := &v.gil
	g.mu.Lock()
	if !g.held {
		g.held = true
		g.take()
		g.mu.Unlock()
	} else {
		w := gilWaiter{wake: make(chan struct{}, 1), ticket: g.ticket, ahead: len(g.queue), since: monotonic()}
		g.ticket++
		g.queue = append(g.queue, w)
		if len(g.queue) == 1 {
			g.arm() // kuyruğun başı oldu: tutanın süresi ölçülür
		}
		v.pending.Add(1)
		g.mu.Unlock()
		<-w.wake // kilit bize devredildi (held açık kaldı; take bırakan görevde çağrıldı)
		if h := v.gilWaitHook; h != nil {
			h(w.ahead, w.ticket, monotonic()-w.since)
		}
	}
	// Kuşak ve an, kilit bu göreve geçerken yazıldı; bırakılana kadar değişmez.
	v.gilGen, v.gilSince = g.gen, g.since
	v.clock.last, v.clock.since, v.clock.skip = v.gilSince, 0, 0
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
		g.stopTimer()
		g.mu.Unlock()
		return
	}
	w := g.queue[0]
	g.queue[0] = gilWaiter{}
	g.queue = g.queue[1:]
	g.take()
	v.pending.Add(-1)
	g.mu.Unlock()
	w.wake <- struct{}{}
}

// take kilidin yeni tutanının kuşağını başlatır (mu tutulurken): önceki tutanın zamanlayıcısı
// durur; kuyrukta bekleyen varsa yeninin zamanlayıcısı kurulur.
func (g *gilQueue) take() {
	g.gen++
	g.since = monotonic()
	g.stopTimer()
	if len(g.queue) > 0 {
		g.arm()
	}
}

// arm tutanın lockHold'u dolduğu ana zamanlayıcıyı kurar (mu tutulurken).
func (g *gilQueue) arm() {
	if g.timerOff || g.timer != nil {
		return
	}
	gen := g.gen
	after := time.AfterFunc
	if g.afterFunc != nil {
		after = g.afterFunc
	}
	g.timer = after(lockHold-(monotonic()-g.since), func() { g.expire(gen) })
}

func (g *gilQueue) stopTimer() {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
}

// expire gen kuşağının süresinin dolduğunu yazar. Durdurulamayan (o anda çalışan) eski bir
// zamanlayıcı yeni kuşağın kaydını geri almasın diye sayı yalnız büyür.
func (g *gilQueue) expire(gen uint64) {
	for old := g.expired.Load(); old < gen; old = g.expired.Load() {
		if g.expired.CompareAndSwap(old, gen) {
			return
		}
	}
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
	if v.gil.expired.Load() != v.gilGen && !v.clockExpired() {
		return
	}
	v.handOffLock()
}

// clockExpired yedek saattir: her clock.every sınırda bir saati okur ve kilit lockHold'dan uzun
// tutulduysa doğru döner. every, iki okuma arası yaklaşık clockGrain olacak biçimde ayarlanır:
// sınır başına süre kısaysa büyür (okumada en çok iki katına, en çok clockEveryMax), uzunsa 1'e
// iner. Süre saatin son ilerlediği andan ölçülür: adımı kaba saatte (Windows'ta 0,3–10 ms,
// ortancası 0,5–2 ms ölçüldü) ilerlemeyen okuma ölçü vermez. Saat ilerleyince ortalama, adım
// boyunca sayılan binlerce hızlı sınırı da içerir ve hızlıdan yavaşa geçişi gizler; bu yüzden
// sonraki okuma, geçen sürenin hepsi son okumadan beri geçen sınırlarda geçmiş sayılarak da
// clockGrain içinde tutulur (every değişmez: hızlı evrede bu, saatin adımı başına bir okumadır).
func (v *VM) clockExpired() bool {
	c := &v.clock
	c.since++
	if c.skip > 0 {
		c.skip--
		return false
	}
	if v.gil.clockOff {
		return false
	}
	now := monotonic()
	c.skip = max(c.every, 1) - 1
	if now > c.last {
		next := 2 * max(c.every, 1)
		if per := (now - c.last) / time.Duration(c.since); per > 0 {
			next = min(next, int(clockGrain/per))
		}
		c.every = min(max(next, 1), clockEveryMax)
		c.skip = c.every - 1
		gap := c.since // görevin ilk okuması: önceki okuma yok
		if c.gap > 0 {
			gap = min(gap, c.gap)
		}
		if recent := (now - c.last) / time.Duration(gap); recent > 0 {
			c.skip = min(c.every, max(int(clockGrain/recent), 1)) - 1
		}
		c.last, c.since = now, 0
	}
	c.gap = c.skip + 1
	return now-v.gilSince >= lockHold
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

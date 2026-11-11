package tensor

import (
	"runtime"
	"sync"
	"time"
	"weak"
)

// ============================================================================
// Yüklemeden önce bekleme: bırakılan blokların temizleyicileri (status C satırı)
// ============================================================================
//
// Bırakılan modelin blokları bir sonraki çöp toplamaya ve onu izleyen temizleyicilere kadar geri
// verilmez (host_block.go, "Ömür"). Model değiştirirken yeni ağırlıklar eskilerin üstüne ayrılınca
// tepe iki modelin toplamı olur. Bu yüzden yükleyici büyük bir dosyadan önce toplar ve ölü blokların
// temizleyicilerini bekler.
//
// Süreç sayacına (hostMemory.live) bakılmaz. Temizleyiciler programın goroutine'leriyle aynı anda
// koşar; bekleme sürerken başka bir yol da blok ayırabilir. Sayaca bakan bir koşul bu iki hâlde
// yersiz beklerdi. Onun yerine her blok bir kayıtta durur: zayıf gösterici ve boy. Toplamadan sonra
// zayıf göstericisi nil olan kayıtlar ölü kümedir; bekleme yalnız o kümenin kayıtları silinene kadar
// sürer. Temizleyici önce belleği geri verir ve sayacı düşürür, sonra kaydı siler: silinmiş kayıt
// geri verilmiş bellek demektir.

// hostSettleLimit beklemenin sınırıdır. Sınır aşılırsa yükleme sürer, çağıran uyarır (H6730). Değer,
// ölçülen temizleyici süresinin (~0,9 sn; Linux, 2,4 GB blok) on katıdır.
const hostSettleLimit = 10 * time.Second

// hostSettlePoll ölü kümenin denetim aralığıdır.
const hostSettlePoll = 5 * time.Millisecond

type hostRecord struct {
	block weak.Pointer[HostBlock]
	size  int64
}

// hostRecords NewHostBlock'un verdiği, temizleyicisi henüz bitmemiş bloklardır. Sayıları bütçeyle
// sınırlıdır (hostBudgetLeft).
var hostRecords = struct {
	sync.Mutex
	next uint64
	m    map[uint64]hostRecord
}{m: map[uint64]hostRecord{}}

// hostCleanup bloğun temizleyicisinin değiştirgesidir: blok değil, boyu ve kaydın anahtarı.
type hostCleanup struct {
	size int64
	id   uint64
}

// hostRegister bloğu kayda ekler ve kaydın anahtarını verir.
func hostRegister(b *HostBlock, size int) uint64 {
	hostRecords.Lock()
	defer hostRecords.Unlock()
	hostRecords.next++
	hostRecords.m[hostRecords.next] = hostRecord{block: weak.Make(b), size: int64(size)}
	return hostRecords.next
}

// hostUnregister kaydı siler. Temizleyici belleği geri verdikten sonra çağırır.
func hostUnregister(id uint64) {
	hostRecords.Lock()
	delete(hostRecords.m, id)
	hostRecords.Unlock()
}

// HostSettle beklemenin sonucudur: ölü kümenin kayıt sayısı ve baytı, sınırda henüz silinmemiş
// kayıtların baytı, süre ve sınırın aşılıp aşılmadığı.
type HostSettle struct {
	Dead      int
	DeadBytes int64
	Waiting   int64
	Elapsed   time.Duration
	TimedOut  bool
}

// SettleHostBlocks dosyadan size baytlık bir yüklemeden önce çağrılır. Dosya bloğa girecek kadar
// büyükse ve öbek dışında blok varsa toplar ve ölü blokların temizleyicilerini en çok
// hostSettleLimit kadar bekler. Öbür hâllerde (küçük dosya, blok yok, bloksuz platform) hiçbir şey
// yapmaz.
func SettleHostBlocks(size int64) HostSettle {
	if !hostSettleNeeded(size, hostMemory.live.Load()) {
		return HostSettle{}
	}
	return settleHost(hostSettleLimit)
}

// hostSettleNeeded size baytlık dosyadan önce, live bayt blok canlıyken beklemek gerekir mi.
func hostSettleNeeded(size, live int64) bool {
	return size > hostBlockMin && live > 0
}

// settleHost toplar, ölü kümeyi seçer ve kümenin kayıtları silinene ya da limit dolana kadar
// bekler. Kilit yalnız tarama ve denetim anında tutulur, uykuda tutulmaz: tutulsaydı temizleyici
// kaydı silemez, her bekleme sınıra varırdı.
func settleHost(limit time.Duration) HostSettle {
	start := time.Now()
	runtime.GC()
	dead, bytes := hostDeadSet()
	r := HostSettle{Dead: len(dead), DeadBytes: bytes}
	for len(dead) > 0 {
		dead, r.Waiting = hostPending(dead)
		if r.Waiting == 0 {
			break
		}
		if time.Since(start) >= limit {
			r.TimedOut = true
			break
		}
		time.Sleep(hostSettlePoll)
	}
	r.Elapsed = time.Since(start)
	return r
}

// hostDeadSet zayıf göstericisi nil olan kayıtları (ölü küme) ve boylarının toplamını verir.
func hostDeadSet() ([]uint64, int64) {
	hostRecords.Lock()
	defer hostRecords.Unlock()
	var dead []uint64
	var bytes int64
	for id, rec := range hostRecords.m {
		if rec.block.Value() == nil {
			dead = append(dead, id)
			bytes += rec.size
		}
	}
	return dead, bytes
}

// hostPending kümenin henüz silinmemiş kayıtlarını ve boylarının toplamını verir.
func hostPending(dead []uint64) ([]uint64, int64) {
	hostRecords.Lock()
	defer hostRecords.Unlock()
	left := dead[:0]
	var bytes int64
	for _, id := range dead {
		if rec, ok := hostRecords.m[id]; ok {
			left = append(left, id)
			bytes += rec.size
		}
	}
	return left, bytes
}

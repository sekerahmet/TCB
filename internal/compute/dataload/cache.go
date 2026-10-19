package dataload

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// CacheOptions, yerel önbelleğin ayarlarıdır.
type CacheOptions struct {
	// Dir, kopyaların konacağı yerel dizindir (zorunlu). Önbellek bu
	// dizinde yalnızca kendi adlandırdığı dosyalara dokunur.
	Dir string
	// Quota, önbelleğin kullanabileceği en çok bayt sayısıdır (zorunlu, > 0).
	// Örnekleyiciyle kullanıldığında en az (aynı anda açık parça sayısı +
	// ileri bakış) × en büyük parça boyutu olmalıdır; örnekleyici kurulurken
	// denetlenir.
	Quota int64
	// Workers, arka planda aynı anda kopyalayan gorutin sayısıdır (varsayılan 2).
	Workers int
}

// CacheStats, önbelleğin sayaçlarıdır.
type CacheStats struct {
	Hits        int64 // yerel kopyadan karşılanan Getir çağrıları
	Misses      int64 // Getir sırasında eşzamanlı kopyalanan dosyalar
	Skipped     int64 // yer olmadığından reddedilen Getir çağrıları
	Evicted     int64 // kota için silinen dosyalar
	CopiedBytes int64
	Usage       int64 // şu anki toplam bayt (süren kopyalar dahil)
	FileCount   int
}

// Cache, yavaş bir kaynak dizindeki (ör. ağ sürücüsü) dosyaları yerel
// bir dizine kopyalayan, disk kotası olan bir katmandır. Kota dolduğunda en
// uzun süredir kullanılmayan ve o an açık olmayan dosyalar silinir. Hazirla
// ile bildirilen sıradaki dosyalar arka planda kopyalanır. Yer açılamazsa
// Getir açıklayıcı bir hata döner; yavaş kaynaktan sessizce doğrudan
// okumaya geri dönülmez.
//
// Aynı dizin için süreç içinde tek bir yönetici (obCekirdek) vardır: aynı
// dizinle açılan her Onbellek (ör. eğitim ve doğrulama kümeleri) onu
// paylaşır; kota, kayıtlar ve LRU sırası ortaktır. Her Onbellek kendi
// Hazirla listesini bildirir; listeler sırayla harmanlanır. Dizin ayrıca
// süreçler arası bir kilit dosyasıyla korunur: başka bir süreç aynı dizini
// açmaya çalışırsa açık bir hata alır (birbirinin kopyalarını silmesinler).
type Cache struct {
	c      *cacheCore
	mu     sync.Mutex
	closed bool
}

// cacheCore bir önbellek dizininin süreç içindeki tek yöneticisidir.
type cacheCore struct {
	dir   string
	key   string   // kayıt anahtarı (mutlak, temiz yol)
	quota int64    // paylaşılan kota (en son açılışta verilen); mu ile korunur
	lock  *os.File // süreçler arası kilit dosyası (açık kaldıkça kilitli)
	// clients açık Onbellek'lerdir (açılış sırasıyla); listeler her
	// birinin son Hazirla listesidir.
	clients  []*Cache
	lists    map[*Cache][]string
	reserved map[*Cache]int64 // Ayir ile bildirilen yer

	mu            sync.Mutex
	record        map[string]*cacheEntry // yerel ad → girdi
	usage         int64
	counter       uint64
	requested     []string
	requestedName map[string]bool
	generation    uint64
	pending       bool // Hazirla listesi henüz işlenip bitmedi
	// handled sonuna kadar işlenen (ya da kota yetmediği için bırakılan) en son liste
	// kuşağıdır; pending, son kuşak işlenmiş ve çalışan işçi kalmamışsa düşer. Hangi işçinin
	// en son bittiği önemli değildir (eski kuşağın listesinden dönen işçi de düşürebilir).
	handled  uint64
	active   int // listeyi işlemekte olan işçi sayısı
	worker   int
	closed   bool
	statsNow CacheStats
	lastErr  error

	warning chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type cacheEntry struct {
	ad    string
	dim   int64
	when  time.Time     // kaynağın değişiklik zamanı (yerel dosyaya da yazılır)
	last  uint64        // son kullanım sayacı (LRU)
	pin   int           // açık kullanıcı sayısı; > 0 iken silinmez
	ready bool          // dosya tamamen kopyalandı
	dup   chan struct{} // nil değilse kopyalanıyor; bitince kapanır
	// prefetch, süren kopyayı arka plan işçisinin (Hazirla) başlattığını
	// bildirir. Böyle bir kopya bitince iğnelenmez, silinebilir hale gelir.
	prefetch bool
}

var (
	// ErrCacheInsufficient, önbellek kotasında dosya için yer açılamadığını
	// bildirir (kota küçük ya da açık dosyalar yer kaplıyor).
	ErrCacheInsufficient = errors.New("veri: önbellek kotası yetersiz")

	// ErrCacheLocked, önbellek dizinini başka bir sürecin kullandığını
	// bildirir.
	ErrCacheLocked = errors.New("veri: önbellek dizini kilitli")

	// copyHook, testlerde kopyalamayı yavaşlatmak için kopyadan hemen
	// önce (kilit tutulmadan) çağrılır; üretimde nil'dir.
	copyHook func(src string, prefetch bool)

	managedName = regexp.MustCompile(`\.[0-9a-f]{16}\.onb(\.[^.]*)?$`)
	tempSuffix  = ".gecici"
)

// lockPath, önbellek dizinindeki süreçler arası kilit dosyasının adıdır
// (yönetilen adlara uymaz; silinmez).
const lockPath = ".onbellek.kilit"

// cacheRecord, süreç içinde açık önbellek dizinleridir (anahtar: dizinAnahtari).
var cacheRecord = struct {
	mu sync.Mutex
	m  map[string]*cacheCore
}{m: map[string]*cacheCore{}}

// dirKey dizinin süreç içi kayıt anahtarıdır: bağlantıları çözülmüş,
// mutlak ve temiz yol (Windows'ta küçük harf).
func dirKey(dir string) string {
	exact, err := filepath.Abs(dir)
	if err != nil {
		exact = dir
	}
	if g, err := filepath.EvalSymlinks(exact); err == nil {
		exact = g
	}
	exact = filepath.Clean(exact)
	if runtime.GOOS == "windows" {
		exact = strings.ToLower(exact)
	}
	return exact
}

// OpenCache, önbelleği açar; dizinde önceki çalıştırmadan kalan kopyalar
// varsa benimsenir (kaynakla boyut ve zaman eşleşirse kullanılır). Dizin bu
// süreçte zaten açıksa yöneticisi paylaşılır (tek kota; en son verilen
// geçerlidir); başka bir süreç dizini kullanıyorsa hata döner.
func OpenCache(s CacheOptions) (*Cache, error) {
	if s.Dir == "" {
		return nil, fmt.Errorf("veri: önbellek dizini verilmedi")
	}
	if s.Quota <= 0 {
		return nil, fmt.Errorf("veri: önbellek kotası pozitif olmalı")
	}
	if s.Workers <= 0 {
		s.Workers = 2
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("veri: önbellek dizini oluşturulamadı: %w", err)
	}
	key := dirKey(s.Dir)
	cacheRecord.mu.Lock()
	defer cacheRecord.mu.Unlock()
	c := cacheRecord.m[key]
	if c != nil {
		// Paylaşılan tek kota vardır; en son verilen geçerlidir (küçüldüyse
		// fazlası hemen silinir).
		c.mu.Lock()
		c.quota = s.Quota
		c.makeRoom(0, false)
		c.mu.Unlock()
	} else {
		var err error
		if c, err = openCore(s); err != nil {
			return nil, err
		}
		c.key = key
		cacheRecord.m[key] = c
	}
	h := &Cache{c: c}
	c.mu.Lock()
	c.clients = append(c.clients, h)
	c.mu.Unlock()
	return h, nil
}

// openCore dizinin yöneticisini kurar: süreçler arası kilidi alır,
// önceki çalıştırmadan kalan kopyaları benimser ve işçileri başlatır.
func openCore(s CacheOptions) (*cacheCore, error) {
	lock, err := os.OpenFile(filepath.Join(s.Dir, lockPath), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("veri: önbellek kilit dosyası açılamadı: %w", err)
	}
	if err := lockFile(lock); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("%w: önbellek dizini %s başka bir program (süreç) tarafından kullanılıyor; "+
			"iki program aynı önbellek klasörünü aynı anda kullanamaz, her birine ayrı bir klasör verin", ErrCacheLocked, s.Dir)
	}
	inputs, err := os.ReadDir(s.Dir)
	if err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("veri: önbellek dizini okunamadı: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	o := &cacheCore{
		dir: s.Dir, quota: s.Quota, lock: lock, lists: map[*Cache][]string{}, reserved: map[*Cache]int64{},
		record: map[string]*cacheEntry{}, requestedName: map[string]bool{},
		warning: make(chan struct{}, s.Workers), ctx: ctx, cancel: cancel, worker: s.Workers,
	}
	for _, g := range inputs {
		ad := g.Name()
		if g.IsDir() {
			continue
		}
		if strings.HasSuffix(ad, tempSuffix) && managedName.MatchString(strings.TrimSuffix(ad, tempSuffix)) {
			_ = os.Remove(filepath.Join(s.Dir, ad)) // yarım kalmış kopya
			continue
		}
		if !managedName.MatchString(ad) {
			continue
		}
		info, err := g.Info()
		if err != nil {
			continue
		}
		if !copyValid(filepath.Join(s.Dir, ad), info.Size()) {
			_ = os.Remove(filepath.Join(s.Dir, ad)) // bozuk ya da kesik kopya
			continue
		}
		o.record[ad] = &cacheEntry{ad: ad, dim: info.Size(), when: info.ModTime(), ready: true}
		o.usage += info.Size()
	}
	if o.usage > o.quota {
		o.mu.Lock()
		o.makeRoom(0, false)
		o.mu.Unlock()
	}
	o.wg.Add(o.worker)
	for i := 0; i < o.worker; i++ {
		go o.run()
	}
	return o, nil
}

// copyValid, benimsenecek kopya .npy ise başlığının geçerli ve verinin
// tam olduğunu denetler. Diğer dosyalarda yalnızca boyut (Getir sırasında
// kaynakla) karşılaştırılır.
func copyValid(path string, dim int64) bool {
	if !strings.EqualFold(filepath.Ext(path), ".npy") {
		return true
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	bs, err := ReadHeader(f)
	if err != nil {
		return false
	}
	return bs.DataOffset+bs.DataSize() <= dim
}

// pinnedBytes, açık (iğneli) dosyaların toplam boyutudur; kilit tutulur.
func (o *cacheCore) pinnedBytes() int64 {
	var t int64
	for _, g := range o.record {
		if g.pin > 0 {
			t += g.dim
		}
	}
	return t
}

// localName, kaynak yolu için önbellekteki dosya adını üretir. Ad, kaynağın
// tam yolunun özetini içerir; farklı dizinlerdeki aynı adlı dosyalar
// çakışmaz.
func localName(src string) string {
	exact, err := filepath.Abs(src)
	if err != nil {
		exact = src
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(filepath.Clean(exact)))
	baseOff := filepath.Base(src)
	uz := filepath.Ext(baseOff)
	return fmt.Sprintf("%s.%016x.onb%s", strings.TrimSuffix(baseOff, uz), h.Sum64(), uz)
}

// sameTime önbellekteki kopyanın zamanı (cached) kaynağınkiyle (source) aynı mı bildirir. Kopya
// kaynağın zamanını alır, ama dosya sistemi onu yuvarlayabilir: NTFS 100 ns'ye, exFAT 10 ms'ye,
// FAT 2 saniyeye, bazıları tam saniyeye. Kopyanın zamanında saniye kesri varsa en çok 10 ms
// yuvarlanmıştır; yoksa ±2 sn tolerans gerekir. O durumda aynı boyutta kalıp 2 saniye içinde
// yeniden yazılan bir kaynak değişmemiş sayılabilir (yeni kopya için önbellek klasörü silinir).
func sameTime(cached, source time.Time) bool {
	tolerance := 2 * time.Second
	if cached.Nanosecond() != 0 {
		tolerance = 10 * time.Millisecond
	}
	d := cached.Sub(source)
	return d < tolerance && d > -tolerance
}

func (o *cacheCore) fetch(ctx context.Context, src string, prefetch bool) (string, func(), error) {
	st, err := os.Stat(src)
	if err != nil {
		return "", nil, fmt.Errorf("veri: önbellek kaynağı: %w", err)
	}
	ad := localName(src)
	local := filepath.Join(o.dir, ad)
	copied := false

	o.mu.Lock()
	for {
		if o.closed {
			o.mu.Unlock()
			return "", nil, ErrClosed
		}
		g := o.record[ad]
		if g != nil && g.dup != nil {
			if prefetch {
				o.mu.Unlock()
				return "", nil, nil // başkası kopyalıyor
			}
			c := g.dup
			o.mu.Unlock()
			select {
			case <-c:
			case <-ctx.Done():
				return "", nil, ctx.Err()
			}
			o.mu.Lock()
			continue
		}
		if g != nil && g.ready {
			if g.dim == st.Size() && sameTime(g.when, st.ModTime()) {
				o.counter++
				g.last = o.counter
				if prefetch {
					o.mu.Unlock()
					return local, nil, nil
				}
				g.pin++
				if !copied {
					o.statsNow.Hits++
				}
				o.mu.Unlock()
				return local, o.releaser(g), nil
			}
			// Kaynak değişmiş: kopya bayat.
			if g.pin > 0 || !o.remove(g) {
				if prefetch {
					o.mu.Unlock()
					return "", nil, nil
				}
				o.statsNow.Skipped++
				o.mu.Unlock()
				return "", nil, fmt.Errorf("veri: %s: kaynak değişmiş ama eski yerel kopya hâlâ kullanımda; yenilenemiyor", src)
			}
		}
		if st.Size() > o.quota || !o.makeRoom(st.Size(), prefetch) {
			if prefetch {
				o.mu.Unlock()
				return "", nil, ErrCacheInsufficient
			}
			// Yeri geçici olarak arka planda süren önden yükleme kopyaları
			// tutuyorsa (ör. kapanmış bir örnekleyicinin istediği sıradaki
			// parça) onlar bitince silinebilir; hata vermeden önce birini
			// bekleyip yeniden dene.
			if st.Size() <= o.quota {
				if c := o.ongoingPrefetch(st.Size()); c != nil {
					o.mu.Unlock()
					select {
					case <-c:
					case <-ctx.Done():
						return "", nil, ctx.Err()
					}
					o.mu.Lock()
					continue
				}
			}
			o.statsNow.Skipped++
			failure := fmt.Errorf("%w: %s için %d bayt gerekli; kota %d bayt, kullanımda %d bayt (açık dosyalar %d bayt). "+
				"Yavaş kaynaktan doğrudan okumamak için kopya alınmadı; kotayı büyütün",
				ErrCacheInsufficient, filepath.Base(src), st.Size(), o.quota, o.usage, o.pinnedBytes())
			o.mu.Unlock()
			return "", nil, failure
		}
		g = &cacheEntry{ad: ad, dim: st.Size(), when: st.ModTime(), dup: make(chan struct{}), prefetch: prefetch}
		o.record[ad] = g
		o.usage += g.dim
		if !prefetch {
			o.statsNow.Misses++
		}
		o.mu.Unlock()

		if copyHook != nil {
			copyHook(src, prefetch)
		}
		err := copyFile(ctx, src, local, st.Size(), st.ModTime())

		o.mu.Lock()
		close(g.dup)
		g.dup = nil
		if err != nil {
			delete(o.record, ad)
			o.usage -= g.dim
			o.mu.Unlock()
			return "", nil, err
		}
		g.ready = true
		o.statsNow.CopiedBytes += g.dim
		copied = true
	}
}

func (o *cacheCore) releaser(g *cacheEntry) func() {
	var one sync.Once
	return func() {
		one.Do(func() {
			o.mu.Lock()
			g.pin--
			o.counter++
			g.last = o.counter
			o.mu.Unlock()
		})
	}
}

// remove, girdinin dosyasını siler; kilit tutulur. Silinemezse false döner.
func (o *cacheCore) remove(g *cacheEntry) bool {
	err := os.Remove(filepath.Join(o.dir, g.ad))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false
	}
	delete(o.record, g.ad)
	o.usage -= g.dim
	o.statsNow.Evicted++
	return true
}

// makeRoom, gerekli bayt için kotada yer açar; kilit tutulur. Silinecek
// adaylar: hazır, açık olmayan ve kopyalanmayan dosyalar, en eski
// kullanılan önce. Önden yüklemede, hâlâ istenen dosyalar silinmez; eşzamanlı
// Getir'de ise istenenler en son silinir. Yer açılamayacaksa hiçbir şey
// silinmez ve false döner.
func (o *cacheCore) makeRoom(required int64, prefetch bool) bool {
	if o.usage+required <= o.quota {
		return true
	}
	var candidates []*cacheEntry
	var free int64
	for _, g := range o.record {
		if !g.ready || g.pin > 0 || g.dup != nil {
			continue
		}
		if prefetch && o.requestedName[g.ad] {
			continue
		}
		candidates = append(candidates, g)
		free += g.dim
	}
	if o.usage-free+required > o.quota {
		return false
	}
	sort.Slice(candidates, func(i, j int) bool {
		ai, aj := o.requestedName[candidates[i].ad], o.requestedName[candidates[j].ad]
		if ai != aj {
			return !ai
		}
		if candidates[i].last != candidates[j].last {
			return candidates[i].last < candidates[j].last
		}
		return candidates[i].ad < candidates[j].ad
	})
	for _, g := range candidates {
		if o.usage+required <= o.quota {
			break
		}
		o.remove(g)
	}
	return o.usage+required <= o.quota
}

// ongoingPrefetch, gerekli bayt için şu an yer açılamıyor ama süren önden
// yükleme kopyaları bitip silinebilir hale gelince açılabilecekse bu
// kopyalardan birinin bitiş kanalını döner; yoksa nil. Kilit tutulur.
// Önden yükleme kopyası bitince iğnelenmez, bu yüzden bekleyen eşzamanlı
// Getir onu silerek yer açabilir; aksi halde Getir, kota yalnızca geçici
// olarak doluyken yanlışlıkla ErrOnbellekYetersiz dönerdi.
func (o *cacheCore) ongoingPrefetch(required int64) chan struct{} {
	var free int64
	var wait chan struct{}
	for _, g := range o.record {
		switch {
		case g.dup != nil && g.prefetch:
			free += g.dim
			if wait == nil {
				wait = g.dup
			}
		case g.ready && g.pin == 0 && g.dup == nil:
			free += g.dim
		}
	}
	if wait == nil || o.usage-free+required > o.quota {
		return nil
	}
	return wait
}

// copyFile, kaynağı geçici bir dosyaya kopyalar, yazılan bayt sayısını
// beklenenle karşılaştırır, diske yazılmasını (Sync) sağlar, değişiklik
// zamanını kaynakla eşitler ve ancak bundan sonra hedef adına taşır. Böylece
// hedef adında yalnızca tam ve kalıcı kopyalar bulunur.
func copyFile(ctx context.Context, src, target string, expected int64, when time.Time) (err error) {
	k, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	defer k.Close()
	temp := target + tempSuffix
	h, err := os.Create(temp)
	if err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	defer func() {
		if err != nil {
			_ = h.Close()
			_ = os.Remove(temp)
		}
	}()
	buf := make([]byte, 4<<20)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := k.Read(buf)
		if n > 0 {
			if _, werr := h.Write(buf[:n]); werr != nil {
				return fmt.Errorf("veri: önbellek kopyası: %w", werr)
			}
			written += int64(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("veri: önbellek kopyası: %w", rerr)
		}
	}
	if written != expected {
		return fmt.Errorf("veri: %s: önbellek kopyası eksik ya da fazla: %d bayt yazıldı, %d bekleniyordu",
			filepath.Base(src), written, expected)
	}
	if err := h.Sync(); err != nil {
		return fmt.Errorf("veri: önbellek kopyası diske yazılamadı: %w", err)
	}
	if err := h.Close(); err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	if err := os.Chtimes(temp, when, when); err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	if err := os.Rename(temp, target); err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	return nil
}

// Prepare, sıradaki kaynak dosyaları (öncelik sırasıyla) bildirir; arka
// plandaki işçiler bunları sırayla ve aynı anda en çok Isci dosya olmak
// üzere, kotanın izin verdiği kadar kopyalar. Her çağrı bu Onbellek'in
// önceki listesinin yerini alır; dizini paylaşan diğer Onbellek'lerin
// listeleriyle sırayla (her birinden birer dosya) harmanlanır.
func (h *Cache) Prepare(sources []string) {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if !closed {
		h.c.prepare(h, sources)
	}
}

func (o *cacheCore) prepare(h *Cache, sources []string) {
	o.mu.Lock()
	o.lists[h] = append([]string(nil), sources...)
	o.interleaveLists()
	o.pending = true
	o.mu.Unlock()
	// Tüm işçileri uyandır; meşgul olanlar listeyi bitirince yeniden bakar.
	for i := 0; i < o.worker; i++ {
		select {
		case o.warning <- struct{}{}:
		default:
		}
	}
}

func (o *cacheCore) run() {
	defer o.wg.Done()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-o.warning:
		}
		o.mu.Lock()
		list, generation := o.requested, o.generation
		o.active++
		o.mu.Unlock()
		completed := o.fetchList(list, generation)
		o.mu.Lock()
		o.active--
		if completed && generation > o.handled {
			o.handled = generation
		}
		if o.handled == o.generation && o.active == 0 {
			o.pending = false
		}
		o.mu.Unlock()
		if o.ctx.Err() != nil {
			return
		}
	}
}

// fetchList kuşağın listesini önbelleğe alır; liste sonuna kadar işlendiyse (ya da kota
// yetmediği için bırakıldıysa) doğru, yeni bir liste geldiği ya da önbellek kapandığı için
// yarıda kaldıysa yanlış döner.
func (o *cacheCore) fetchList(list []string, generation uint64) bool {
	for _, k := range list {
		if o.ctx.Err() != nil {
			return false
		}
		o.mu.Lock()
		changed := o.generation != generation
		o.mu.Unlock()
		if changed {
			return false // uyarı kanalında yeni liste bekliyor
		}
		_, _, err := o.fetch(o.ctx, k, true)
		if errors.Is(err, ErrCacheInsufficient) {
			return true
		}
		if err != nil && o.ctx.Err() == nil {
			o.mu.Lock()
			o.lastErr = err
			o.mu.Unlock()
		}
	}
	return true
}

func (o *cacheCore) wait(ctx context.Context) error {
	for {
		o.mu.Lock()
		finished := !o.pending || o.closed
		o.mu.Unlock()
		if finished {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (o *cacheCore) stats() CacheStats {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := o.statsNow
	s.Usage = o.usage
	s.FileCount = len(o.record)
	return s
}

func (o *cacheCore) takeLastErr() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.lastErr
}

// interleaveLists istemcilerin listelerini sırayla (her birinden birer
// dosya) birleştirir ve listenin değiştiğini işçilere bildirir; kilit tutulur.
func (o *cacheCore) interleaveLists() {
	o.requested = nil
	o.requestedName = map[string]bool{}
	for i := 0; ; i++ {
		remaining := false
		for _, h := range o.clients {
			l := o.lists[h]
			if i >= len(l) {
				continue
			}
			remaining = true
			if ad := localName(l[i]); !o.requestedName[ad] {
				o.requestedName[ad] = true
				o.requested = append(o.requested, l[i])
			}
		}
		if !remaining {
			break
		}
	}
	o.generation++
}

// Close, bu Onbellek'i kapatır. Dizini paylaşan başka açık Onbellek yoksa
// arka plan kopyalamasını durdurur, gorutinlerin bitmesini bekler ve dizin
// kilidini bırakır. Yerel kopyalar diskte kalır ve sonraki açılışta
// benimsenir. Birden çok kez çağrılabilir.
func (h *Cache) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()
	o := h.c
	cacheRecord.mu.Lock()
	o.mu.Lock()
	delete(o.lists, h)
	delete(o.reserved, h)
	for i, x := range o.clients {
		if x == h {
			o.clients = append(o.clients[:i], o.clients[i+1:]...)
			break
		}
	}
	last := len(o.clients) == 0
	if last {
		o.closed = true
		if cacheRecord.m[o.key] == o {
			delete(cacheRecord.m, o.key)
		}
	} else {
		o.interleaveLists()
	}
	o.mu.Unlock()
	cacheRecord.mu.Unlock()
	if !last {
		return nil
	}
	o.cancel()
	o.wg.Wait()
	return o.lock.Close() // kilit, tutamak kapanınca bırakılır
}

// Fetch, kaynağın yerel kopyasının yolunu döner; kopya yoksa kopyalar (ya
// da arka planda süren kopyayı bekler). Dönen birak işlevi çağrılana kadar
// dosya silinmez. Yer yalnızca arka planda süren önden yükleme kopyaları
// yüzünden geçici olarak doluysa onların bitmesi beklenir. Kotada yer
// açılamıyorsa ErrOnbellekYetersiz'i saran açıklayıcı bir hata döner
// (kaynaktan doğrudan okumaya geri dönülmez).
func (h *Cache) Fetch(ctx context.Context, src string) (string, func(), error) {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		return "", nil, ErrClosed
	}
	return h.c.fetch(ctx, src, false)
}

// Reserve, bu Onbellek'in (ör. bir örnekleyicinin) aynı anda yerelde tutacağı
// bayt sayısını bildirir; önceki ayırmanın yerini alır. Dizini paylaşan
// Onbellek'lerin ayırdıklarıyla birlikte kotayı aşarsa ErrOnbellekYetersiz'i
// saran açıklayıcı bir hata döner (ayırma değişmez).
func (h *Cache) Reserve(needed int64) error {
	o := h.c
	o.mu.Lock()
	defer o.mu.Unlock()
	var other int64
	for _, x := range o.clients {
		if x != h {
			other += o.reserved[x]
		}
	}
	if other+needed > o.quota {
		return fmt.Errorf("%w: bu önbellek klasörünü paylaşan diğer kümeler %d bayt ayırdı, bu küme %d bayt daha istiyor; "+
			"kota %d bayt. Aynı klasörü kullanan kümeler kotayı paylaşır: kotayı en az %d bayt yapın ya da kümelere ayrı klasörler verin",
			ErrCacheInsufficient, other, needed, o.quota, other+needed)
	}
	o.reserved[h] = needed
	return nil
}

// Quota, önbelleğin (dizini paylaşanlarla ortak) bayt kotasıdır.
func (h *Cache) Quota() int64 {
	h.c.mu.Lock()
	defer h.c.mu.Unlock()
	return h.c.quota
}

// Dir, önbellek dizinidir.
func (h *Cache) Dir() string { return h.c.dir }

// Stats, dizinin (paylaşılan) sayaçlarının anlık kopyasıdır.
func (h *Cache) Stats() CacheStats { return h.c.stats() }

// LastError, arka plan kopyalamasında görülen son hatadır (yoksa nil).
func (h *Cache) LastError() error { return h.c.takeLastErr() }

// Wait, Hazirla ile istenen dosyaların arka plan kopyası bitene (ya da
// kota yüzünden durana) kadar bekler. Çoğunlukla testler içindir.
func (h *Cache) Wait(ctx context.Context) error { return h.c.wait(ctx) }

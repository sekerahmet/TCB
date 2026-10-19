package tensor

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// Paralel yürütme: kalıcı işçi havuzu
// ============================================================================
//
// Eğitimde aynı işlemler saniyede binlerce kez, çoğu onlarca mikrosaniye süren
// küçük işler olarak gelir. Her çağrıda goroutine açmak, uyuyan işletim sistemi
// iş parçacıklarının uyandırılması demektir: Windows'ta tek uyandırma 10-30 µs
// sürer ve Go zamanlayıcısı işçileri sırayla (zincirleme) uyandırır. 20 iş
// parçacıklı bir makinede bu gecikme işin kendisini aşıyordu (karakter modeli:
// GOMAXPROCS=2'de 13,4, sınırsızda 17,4 ms/adım).
//
// Bu yüzden kalıcı işçiler tembel başlatılır. Bir iş, tek bir işaretçi
// yazılarak yayımlanır; dönen işçiler onu hemen görür, uyuyanlar kendi
// kanallarından uyandırılır. Çağıran da işin parçalarını yürütür ve tüm
// parçalar bitene kadar bekler. Parçalar atomik bir sayaçla dağıtılır (dinamik
// yük dengesi: başarım ve verimlilik çekirdekleri karışık işlemcilerde yavaş
// çekirdek daha az parça alır).
//
// Dönerek bekleme ucuz değildir: çöp toplayıcı her döngüde dünyayı iki kez
// durdurur ve çalışan her P'nin durmasını bekler. Bütün işçiler dönerken bu
// duraklamalar milisaniyelere çıkıyordu (modelde 100 adımda 7 ms yerine 210 ms).
// Bu yüzden yalnızca ilk donenIsci işçi küçük işler arasında döner; küçük
// işler (isciSayisi) en çok bu kadar yardımcıyla yürütülür. Büyük işlerde
// bütün işçiler katılır; uyandırma gecikmesi orada önemsizdir.
//
// Havuz aynı anda tek bir çağırana hizmet eder; iç içe ya da eşzamanlı
// çağrılar (havuz meşgulken) geçici goroutine'lerle yürütülür.
//
// Belirlenimcilik: havuz yalnızca parçaların HANGİ işçide yürütüleceğini
// belirler; parça sınırları çağıranlarca seçilir. Toplamlar işçi sayısından
// bağımsız sabit parçalarla yapıldığından (indirgemeParca) sonuçlar değişmez.

// paralelEsik eleman bazlı işlerde bir işçiye düşen en az iş (eleman × göreli
// maliyet). Toplam iş bunun altındaysa doğrudan çağıranda yürütülür.
//
// Ölçüm (i7-13700H): 2^15 birimlik iş (ör. 32 bin elemanlık toplama, çıktı
// ayırma dahil) tek çekirdekte 10-25 µs sürer; dağıtım ve bekleme dönen bir
// işçiyle ~1 µs, uyuyan bir işçiyle 10-30 µs'dir. Daha küçük parçalarda ek
// işçi kazandırmıyordu.
const parallelThreshold = 1 << 15

// elemanBuyukIs bu kadar birimden büyük eleman bazlı işler bütün işçileri
// kullanabilir; daha küçükleri en çok donenIsci+1 katılımcıyla yürütülür.
// 2^22 birim (ör. 256 bin elemanlık GELU) tek çekirdekte ~0,5 ms sürer;
// uyuyan işçileri uyandırmanın (~20 µs/işçi, zincirleme) karşılığı bu
// büyüklükten sonra alınıyordu.
const elemBigJob = 1 << 22

// donenIsci küçük işler arasında dönerek bekleyen (hemen hazır) işçi sayısı.
// Ölçüm (karakter modeli, 20 iş parçacığı): 3, 1 ve 2'den belirgin iyiydi; 4-5
// ölçüm gürültüsü içinde aynıydı. Bütün işçilerin dönmesi ise GC duraklamaları
// yüzünden adımı ~%25 yavaşlatıyordu.
const returningWorker = 3

// havuzBekleme dönen işçinin iş bittikten sonra yeni iş için beklediği süre.
// Eğitim adımında ardışık tensör işlemleri arasında yorumlayıcı birkaç ile
// birkaç on µs harcar; işçi bu sürede uyumazsa uyandırma maliyeti olmaz. Daha
// uzun boşluklarda (veri hazırlama, yazdırma) işçiler uyur, CPU tüketmez.
// 200 µs ile fark ölçülmedi; kısa tutmak dizüstü işlemcilerde güç bütçesini
// çalışan çekirdeklere bırakır.
const poolWait = 50 * time.Microsecond

// workers kullanılabilecek en çok işçi sayısı (GOMAXPROCS; çağıran dahil).
func workers() int {
	w := runtime.GOMAXPROCS(0)
	if w < 1 {
		w = 1
	}
	return w
}

// isciSayisi is birimlik bir iş için katılımcı sayısını (çağıran dahil)
// seçer: her katılımcıya en az enAz birim düşer; is < buyuk ise en çok
// donenIsci+1 katılımcı kullanılır.
func workerCount(is, minN, large int) int {
	w := workers()
	if lim := is / minN; w > lim {
		w = max(lim, 1)
	}
	if is < large && w > returningWorker+1 {
		w = returningWorker + 1
	}
	return w
}

// gorev havuza yayımlanan bir iş: [0, parca) parçalarının her biri için
// fn(katilimci, parca) çağrılır. katilimci 0 çağırandır, 1..yardimci işçilerdir;
// aynı katilimci numarası aynı anda tek goroutine'de çalışır (katılımcı başına
// geçici tampon tutmak için).
type task struct {
	fn       func(participant, c int)
	part     int64
	helper   int
	seq      uint64
	_        [64]byte // sayaçlar salt okunur alanlardan ayrı önbellek satırında
	next     atomic.Int64
	_        [56]byte
	finished atomic.Int64
	// panik, bir parçada oluşan ilk paniktir; çağıran bütün parçalar bitince onu kendi
	// goroutine'inde yeniden fırlatır (işçide kurtarılmayan panik süreci çökertirdi).
	panicVal atomic.Pointer[partPanic]
}

// parcaPanigi bir parçanın kurtarılmış panik değeri.
type partPanic struct{ value any }

// parcaYurut bir parçayı yürütür; panik oluşursa kaydeder (ilki kalır).
func runPart(p *atomic.Pointer[partPanic], fn func(participant, c int), participant, c int) {
	defer func() {
		if r := recover(); r != nil {
			p.CompareAndSwap(nil, &partPanic{value: r})
		}
	}()
	fn(participant, c)
}

// yurut parçalar bitene kadar sayaçtan parça alıp yürütür. Bir parça panik verdiyse
// kalan parçalar yürütülmeden bitmiş sayılır.
func (g *task) runFn(participant int) {
	for {
		c := g.next.Add(1) - 1
		if c >= g.part {
			return
		}
		if g.panicVal.Load() == nil {
			runPart(&g.panicVal, g.fn, participant, int(c))
		}
		g.finished.Add(1)
	}
}

type worker struct {
	sleeping atomic.Bool
	wake     chan struct{} // kapasite 1: uyandırma jetonu
	_        [48]byte      // komşu işçilerin bayrakları ayrı önbellek satırında
}

var pool struct {
	mu      sync.Mutex // havuzu kullanan tek çağıran
	order   uint64     // son yayımlanan işin sıra numarası (mu altında)
	workers []*worker  // yalnızca mu tutulurken büyütülür
	task    atomic.Pointer[task]
}

// iscileriHazirla en az n işçinin çalıştığını güvence eder (havuz.mu tutulur).
func prepareWorkers(n int) {
	for len(pool.workers) < n {
		i := &worker{wake: make(chan struct{}, 1)}
		no := len(pool.workers)
		pool.workers = append(pool.workers, i)
		go i.run(no)
	}
}

// calis işçi döngüsü: no, işçinin sırasıdır (katılımcı numarası no+1).
func (i *worker) run(no int) {
	var last uint64
	shouldReturn := false
	for {
		g := i.wait(no, last, shouldReturn)
		last = g.seq
		g.runFn(no + 1)
		// Yalnızca ilk donenIsci işçi döner; büyük işlere katılan diğerleri
		// hemen uyur. (Büyük işlerden sonra da dönmek, 512² ve 1024²
		// çarpımlarda GC duraklamaları yüzünden %12-50 kaybettiriyordu.)
		shouldReturn = no < returningWorker
	}
}

// bekle sırası son'dan farklı ve bu işçinin katılacağı bir iş gelene kadar
// bekler: donsun ise önce havuzBekleme süresince döner, sonra uyur.
func (i *worker) wait(no int, last uint64, shouldReturn bool) *task {
	for {
		if shouldReturn {
			begin := time.Now()
			for k := 0; ; k++ {
				if g := pool.task.Load(); g != nil && g.seq != last {
					last = g.seq
					if no < g.helper {
						return g
					}
				}
				if k&15 == 15 && time.Since(begin) > poolWait {
					break
				}
				cpuPause()
			}
		}
		// Uyu: önce bayrağı kaldır, sonra işi yeniden denetle (yayımlayan,
		// işi yazdıktan sonra bayrağa bakar; ikisinden biri diğerini görür).
		i.sleeping.Store(true)
		if g := pool.task.Load(); g != nil && g.seq != last && no < g.helper {
			if !i.sleeping.CompareAndSwap(true, false) {
				// Yayımlayan bayrağı indirip jeton gönderdi; jetonu tüket.
				<-i.wake
			}
			return g
		}
		<-i.wake
		shouldReturn = false
	}
}

// dagit [0, parca) parçalarını en çok w katılımcıyla (çağıran dahil) yürütür.
// fn(katilimci, c) için katilimci [0, w) aralığındadır. w <= 1 ise ya da tek
// parça varsa her şey çağıranda yürütülür.
func distribute(w, part int, fn func(participant, c int)) {
	if w > part {
		w = part
	}
	if w <= 1 {
		for c := 0; c < part; c++ {
			fn(0, c)
		}
		return
	}
	if !pool.mu.TryLock() {
		// Havuz meşgul (iç içe ya da eşzamanlı çağrı): geçici goroutine'ler.
		distributeTemp(w, part, fn)
		return
	}
	defer pool.mu.Unlock()
	prepareWorkers(w - 1)
	pool.order++
	g := &task{fn: fn, part: int64(part), helper: w - 1, seq: pool.order}
	pool.task.Store(g)
	for _, i := range pool.workers[:w-1] {
		if i.sleeping.Load() && i.sleeping.CompareAndSwap(true, false) {
			i.wake <- struct{}{}
		}
	}
	g.runFn(0)
	// Kalan parçaları yürüten işçileri bekle: önce dönerek, uzun sürerse
	// işlemciyi başka goroutine'lere bırakarak.
	for k := 0; g.finished.Load() < g.part; k++ {
		if k < 1<<12 {
			cpuPause()
		} else {
			runtime.Gosched()
		}
	}
	// İşin kapanışını (ve yakaladığı tamponları) bırak; aynı sıra numarasıyla
	// boş bir iş yayımlanır, işçiler onu yeni iş saymaz.
	pool.task.Store(&task{fn: emptyJob, seq: g.seq})
	// Bir parçada panik olduysa (bütün katılımcılar durduktan sonra) çağıranda yeniden
	// fırlatılır; böylece kurtarma (ör. yorumlayıcının hata dönüşümü) çalışır.
	if p := g.panicVal.Load(); p != nil {
		panic(p.value)
	}
}

func emptyJob(int, int) {}

// dagitGecici havuz kullanılamadığında her çağrıda goroutine açan yedek yol.
func distributeTemp(w, part int, fn func(participant, c int)) {
	var next atomic.Int64
	var panicVal atomic.Pointer[partPanic]
	var wg sync.WaitGroup
	wg.Add(w)
	for k := 0; k < w; k++ {
		go func() {
			defer wg.Done()
			for panicVal.Load() == nil {
				c := int(next.Add(1) - 1)
				if c >= part {
					return
				}
				runPart(&panicVal, fn, k, c)
			}
		}()
	}
	wg.Wait()
	if p := panicVal.Load(); p != nil {
		panic(p.value)
	}
}

// parallelFor [0, n) aralığını parçalara bölüp fn(lo, hi) çağrılarını işçilere
// dağıtır. cost, bir birimin yaklaşık eleman maliyetidir; işçi sayısı toplam
// işe göre seçilir (isciSayisi).
func parallelFor(n, cost int, fn func(lo, hi int)) {
	if n <= 0 {
		return
	}
	is := n * cost
	if n == 1 || is < 2*parallelThreshold {
		fn(0, n)
		return
	}
	w := workerCount(is, parallelThreshold, elemBigJob)
	if w == 1 {
		fn(0, n)
		return
	}
	// Yönlendirmeli parçalama: her katılımcı kalan işin 1/(2w)'sini alır, parçalar
	// sona doğru küçülür (en az yonluEnAz birimlik iş). Geç başlayan (uyandırılan)
	// ya da yavaş çekirdekteki bir katılımcının son parçası böylece kısa kalır ve
	// çağıran, kendi işi bitince onu daha az bekler. Parça sınırları zamanlamaya
	// bağlıdır: parallelFor çağıranları sınırlardan bağımsız sonuç üretir (eleman
	// ya da satır bazlı işler; toplamlar sabit parçalı parallelSum ile).
	minN := int64(max(1, directedMin/max(cost, 1)))
	numer := int64(2 * w)
	var next atomic.Int64
	distribute(w, w, func(_, _ int) {
		for {
			begin := next.Load()
			rest := int64(n) - begin
			if rest <= 0 {
				return
			}
			al := max(rest/numer, minN)
			if !next.CompareAndSwap(begin, begin+al) {
				continue
			}
			fn(int(begin), int(min(begin+al, int64(n))))
		}
	})
}

// yonluEnAz parallelFor'un yönlendirmeli parçalarında bir parçanın en az işi
// (eleman × göreli maliyet; ~2-3 µs). Daha küçük parçalarda sayaç erişimi ve
// çağrı maliyeti öne çıkar.
const directedMin = parallelThreshold / 4

// indirgemeParca belirlenimci paralel toplamlarda sabit parça uzunluğu. Parça
// sınırları yalnızca eleman sayısına bağlıdır (işçi sayısına değil); kısmi
// toplamlar parça sırasıyla birleştirildiği için sonuç her çalıştırmada ve her
// makinede bit düzeyinde aynıdır.
const reducePart = 1 << 14

// parallelSum [0, n) aralığını indirgemeParca uzunluklu parçalara böler,
// fn(lo, hi) kısmi toplamlarını paralel hesaplar ve sabit sırayla toplar.
func parallelSum(n, cost int, fn func(lo, hi int) float64) float64 {
	if n <= reducePart {
		return fn(0, n)
	}
	np := (n + reducePart - 1) / reducePart
	partial := make([]float64, np)
	parallelFor(np, reducePart*cost, func(lo, hi int) {
		for c := lo; c < hi; c++ {
			a := c * reducePart
			partial[c] = fn(a, min(a+reducePart, n))
		}
	})
	var s float64
	for _, v := range partial {
		s += v
	}
	return s
}

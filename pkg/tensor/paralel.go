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
const paralelEsik = 1 << 15

// elemanBuyukIs bu kadar birimden büyük eleman bazlı işler bütün işçileri
// kullanabilir; daha küçükleri en çok donenIsci+1 katılımcıyla yürütülür.
// 2^22 birim (ör. 256 bin elemanlık GELU) tek çekirdekte ~0,5 ms sürer;
// uyuyan işçileri uyandırmanın (~20 µs/işçi, zincirleme) karşılığı bu
// büyüklükten sonra alınıyordu.
const elemanBuyukIs = 1 << 22

// donenIsci küçük işler arasında dönerek bekleyen (hemen hazır) işçi sayısı.
// Ölçüm (karakter modeli, 20 iş parçacığı): 3, 1 ve 2'den belirgin iyiydi; 4-5
// ölçüm gürültüsü içinde aynıydı. Bütün işçilerin dönmesi ise GC duraklamaları
// yüzünden adımı ~%25 yavaşlatıyordu.
const donenIsci = 3

// havuzBekleme dönen işçinin iş bittikten sonra yeni iş için beklediği süre.
// Eğitim adımında ardışık tensör işlemleri arasında yorumlayıcı birkaç ile
// birkaç on µs harcar; işçi bu sürede uyumazsa uyandırma maliyeti olmaz. Daha
// uzun boşluklarda (veri hazırlama, yazdırma) işçiler uyur, CPU tüketmez.
// 200 µs ile fark ölçülmedi; kısa tutmak dizüstü işlemcilerde güç bütçesini
// çalışan çekirdeklere bırakır.
const havuzBekleme = 50 * time.Microsecond

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
func isciSayisi(is, enAz, buyuk int) int {
	w := workers()
	if lim := is / enAz; w > lim {
		w = max(lim, 1)
	}
	if is < buyuk && w > donenIsci+1 {
		w = donenIsci + 1
	}
	return w
}

// gorev havuza yayımlanan bir iş: [0, parca) parçalarının her biri için
// fn(katilimci, parca) çağrılır. katilimci 0 çağırandır, 1..yardimci işçilerdir;
// aynı katilimci numarası aynı anda tek goroutine'de çalışır (katılımcı başına
// geçici tampon tutmak için).
type gorev struct {
	fn       func(katilimci, c int)
	parca    int64
	yardimci int
	seq      uint64
	_        [64]byte // sayaçlar salt okunur alanlardan ayrı önbellek satırında
	sonraki  atomic.Int64
	_        [56]byte
	biten    atomic.Int64
}

// yurut parçalar bitene kadar sayaçtan parça alıp yürütür.
func (g *gorev) yurut(katilimci int) {
	for {
		c := g.sonraki.Add(1) - 1
		if c >= g.parca {
			return
		}
		g.fn(katilimci, int(c))
		g.biten.Add(1)
	}
}

type isci struct {
	uyuyor atomic.Bool
	uyan   chan struct{} // kapasite 1: uyandırma jetonu
	_      [48]byte      // komşu işçilerin bayrakları ayrı önbellek satırında
}

var havuz struct {
	mu      sync.Mutex // havuzu kullanan tek çağıran
	sira    uint64     // son yayımlanan işin sıra numarası (mu altında)
	isciler []*isci    // yalnızca mu tutulurken büyütülür
	gorev   atomic.Pointer[gorev]
}

// iscileriHazirla en az n işçinin çalıştığını güvence eder (havuz.mu tutulur).
func iscileriHazirla(n int) {
	for len(havuz.isciler) < n {
		i := &isci{uyan: make(chan struct{}, 1)}
		no := len(havuz.isciler)
		havuz.isciler = append(havuz.isciler, i)
		go i.calis(no)
	}
}

// calis işçi döngüsü: no, işçinin sırasıdır (katılımcı numarası no+1).
func (i *isci) calis(no int) {
	var son uint64
	donsun := false
	for {
		g := i.bekle(no, son, donsun)
		son = g.seq
		g.yurut(no + 1)
		// Yalnızca ilk donenIsci işçi döner; büyük işlere katılan diğerleri
		// hemen uyur. (Büyük işlerden sonra da dönmek, 512² ve 1024²
		// çarpımlarda GC duraklamaları yüzünden %12-50 kaybettiriyordu.)
		donsun = no < donenIsci
	}
}

// bekle sırası son'dan farklı ve bu işçinin katılacağı bir iş gelene kadar
// bekler: donsun ise önce havuzBekleme süresince döner, sonra uyur.
func (i *isci) bekle(no int, son uint64, donsun bool) *gorev {
	for {
		if donsun {
			bas := time.Now()
			for k := 0; ; k++ {
				if g := havuz.gorev.Load(); g != nil && g.seq != son {
					son = g.seq
					if no < g.yardimci {
						return g
					}
				}
				if k&15 == 15 && time.Since(bas) > havuzBekleme {
					break
				}
				islemciBekle()
			}
		}
		// Uyu: önce bayrağı kaldır, sonra işi yeniden denetle (yayımlayan,
		// işi yazdıktan sonra bayrağa bakar; ikisinden biri diğerini görür).
		i.uyuyor.Store(true)
		if g := havuz.gorev.Load(); g != nil && g.seq != son && no < g.yardimci {
			if !i.uyuyor.CompareAndSwap(true, false) {
				// Yayımlayan bayrağı indirip jeton gönderdi; jetonu tüket.
				<-i.uyan
			}
			return g
		}
		<-i.uyan
		donsun = false
	}
}

// dagit [0, parca) parçalarını en çok w katılımcıyla (çağıran dahil) yürütür.
// fn(katilimci, c) için katilimci [0, w) aralığındadır. w <= 1 ise ya da tek
// parça varsa her şey çağıranda yürütülür.
func dagit(w, parca int, fn func(katilimci, c int)) {
	if w > parca {
		w = parca
	}
	if w <= 1 {
		for c := 0; c < parca; c++ {
			fn(0, c)
		}
		return
	}
	if !havuz.mu.TryLock() {
		// Havuz meşgul (iç içe ya da eşzamanlı çağrı): geçici goroutine'ler.
		dagitGecici(w, parca, fn)
		return
	}
	defer havuz.mu.Unlock()
	iscileriHazirla(w - 1)
	havuz.sira++
	g := &gorev{fn: fn, parca: int64(parca), yardimci: w - 1, seq: havuz.sira}
	havuz.gorev.Store(g)
	for _, i := range havuz.isciler[:w-1] {
		if i.uyuyor.Load() && i.uyuyor.CompareAndSwap(true, false) {
			i.uyan <- struct{}{}
		}
	}
	g.yurut(0)
	// Kalan parçaları yürüten işçileri bekle: önce dönerek, uzun sürerse
	// işlemciyi başka goroutine'lere bırakarak.
	for k := 0; g.biten.Load() < g.parca; k++ {
		if k < 1<<12 {
			islemciBekle()
		} else {
			runtime.Gosched()
		}
	}
	// İşin kapanışını (ve yakaladığı tamponları) bırak; aynı sıra numarasıyla
	// boş bir iş yayımlanır, işçiler onu yeni iş saymaz.
	havuz.gorev.Store(&gorev{fn: bosIs, seq: g.seq})
}

func bosIs(int, int) {}

// dagitGecici havuz kullanılamadığında her çağrıda goroutine açan yedek yol.
func dagitGecici(w, parca int, fn func(katilimci, c int)) {
	var next atomic.Int64
	var wg sync.WaitGroup
	wg.Add(w)
	for k := 0; k < w; k++ {
		go func() {
			defer wg.Done()
			for {
				c := int(next.Add(1) - 1)
				if c >= parca {
					return
				}
				fn(k, c)
			}
		}()
	}
	wg.Wait()
}

// parallelFor [0, n) aralığını parçalara bölüp fn(lo, hi) çağrılarını işçilere
// dağıtır. cost, bir birimin yaklaşık eleman maliyetidir; işçi sayısı toplam
// işe göre seçilir (isciSayisi).
func parallelFor(n, cost int, fn func(lo, hi int)) {
	if n <= 0 {
		return
	}
	is := n * cost
	if n == 1 || is < 2*paralelEsik {
		fn(0, n)
		return
	}
	w := isciSayisi(is, paralelEsik, elemanBuyukIs)
	if w == 1 {
		fn(0, n)
		return
	}
	// Yük dengesi için işçi başına birkaç parça (yavaş çekirdekler daha az
	// parça alır).
	chunks := w * 4
	if chunks > n {
		chunks = n
	}
	size := (n + chunks - 1) / chunks
	chunks = (n + size - 1) / size
	dagit(w, chunks, func(_, c int) {
		lo := c * size
		fn(lo, min(lo+size, n))
	})
}

// indirgemeParca belirlenimci paralel toplamlarda sabit parça uzunluğu. Parça
// sınırları yalnızca eleman sayısına bağlıdır (işçi sayısına değil); kısmi
// toplamlar parça sırasıyla birleştirildiği için sonuç her çalıştırmada ve her
// makinede bit düzeyinde aynıdır.
const indirgemeParca = 1 << 14

// parallelSum [0, n) aralığını indirgemeParca uzunluklu parçalara böler,
// fn(lo, hi) kısmi toplamlarını paralel hesaplar ve sabit sırayla toplar.
func parallelSum(n, cost int, fn func(lo, hi int) float64) float64 {
	if n <= indirgemeParca {
		return fn(0, n)
	}
	np := (n + indirgemeParca - 1) / indirgemeParca
	kismi := make([]float64, np)
	parallelFor(np, indirgemeParca*cost, func(lo, hi int) {
		for c := lo; c < hi; c++ {
			a := c * indirgemeParca
			kismi[c] = fn(a, min(a+indirgemeParca, n))
		}
	})
	var s float64
	for _, v := range kismi {
		s += v
	}
	return s
}

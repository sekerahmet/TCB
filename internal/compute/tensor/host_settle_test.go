package tensor

import (
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

// hostRecordOf b'nin kaydının anahtarını verir; kayıt yoksa 0.
func hostRecordOf(b *HostBlock) uint64 {
	hostRecords.Lock()
	defer hostRecords.Unlock()
	for id, rec := range hostRecords.m {
		if rec.block.Value() == b {
			return id
		}
	}
	return 0
}

// hostHasRecord kaydın hâlâ durup durmadığını söyler.
func hostHasRecord(id uint64) bool {
	hostRecords.Lock()
	defer hostRecords.Unlock()
	_, ok := hostRecords.m[id]
	return ok
}

// hostRecordCount kayıtların sayısıdır.
func hostRecordCount() int {
	hostRecords.Lock()
	defer hostRecords.Unlock()
	return len(hostRecords.m)
}

// releasedBlock n baytlık bir blok ayırır, kaydının anahtarını verip bloğu bırakır.
func releasedBlock(t *testing.T, n int) uint64 {
	t.Helper()
	id := hostRecordOf(needBlock(t, n))
	if id == 0 {
		t.Fatal("blok kayda girmedi")
	}
	return id
}

// fakeDeadRecord temizleyicisi olmayan, erişilmez bir bloğun kaydını ekler: kayıt kendiliğinden
// silinmez, bekleme ona takılır. Test bitince silinir.
func fakeDeadRecord(t *testing.T, size int) uint64 {
	t.Helper()
	id := hostRegister(&HostBlock{}, size)
	t.Cleanup(func() { hostUnregister(id) })
	return id
}

// Bırakılan bloğun kaydı ölü kümeye girer ya da temizleyici onu taramadan önce silmiştir (toplama
// dönmeden temizleyici koşabilir); iki hâlde de dönüşte kayıt silinmiştir, bekleme sınıra varmaz.
// Kümeye girişi belirli olarak TestHostSettleLimitReportsWhatIsLeft sınar.
func TestHostSettleWaitsForTheReleasedBlock(t *testing.T) {
	id := releasedBlock(t, hostSize*4)
	r := settleHost(hostSettleLimit)
	if r.TimedOut || r.Waiting != 0 {
		t.Fatalf("bekleme sınıra vardı: %+v", r)
	}
	if r.Dead == 0 {
		t.Logf("kayıt taramadan önce silinmişti: %+v", r)
	}
	if hostHasRecord(id) {
		t.Fatal("bekleme bitti, bırakılan bloğun kaydı duruyor")
	}
}

// Canlı blok ölü kümeye girmez, beklenmez; kaydı yerinde kalır.
func TestHostSettleSkipsALiveBlock(t *testing.T) {
	b := needBlock(t, hostSize*4)
	id := hostRecordOf(b)
	runtime.GC()
	dead, _ := hostDeadSet()
	if slices.Contains(dead, id) {
		t.Fatal("canlı blok ölü kümede")
	}
	if r := settleHost(hostSettleLimit); r.TimedOut {
		t.Fatalf("canlı blokla bekleme sınıra vardı: %+v", r)
	}
	if !hostHasRecord(id) {
		t.Fatal("canlı bloğun kaydı silindi")
	}
	runtime.KeepAlive(b)
}

// Bekleme yalnız bloğa girecek dosyadan önce ve blok varken yapılır.
func TestHostSettleOnlyForLargeFilesWithBlocks(t *testing.T) {
	cases := []struct {
		size, live int64
		want       bool
	}{
		{hostBlockMin + 1, 1, true},
		{hostBlockMin, 1, false},
		{hostBlockMin - 1, 1 << 30, false},
		{0, 1, false},
		{-1, 1, false},
		{hostBlockMin + 1, 0, false},
		{1 << 40, 0, false},
	}
	for _, c := range cases {
		if got := hostSettleNeeded(c.size, c.live); got != c.want {
			t.Errorf("dosya %d, canlı %d: %v, beklenen %v", c.size, c.live, got, c.want)
		}
	}
	if r := SettleHostBlocks(hostBlockMin); r != (HostSettle{}) {
		t.Fatalf("küçük dosyada bekleme yapıldı: %+v", r)
	}
}

// Sınır dolunca bekleme "aşıldı" der ve silinmemiş baytı verir; yükleme kararı çağıranındır.
func TestHostSettleLimitReportsWhatIsLeft(t *testing.T) {
	id := fakeDeadRecord(t, 123)
	r := settleHost(0)
	if !r.TimedOut || r.Waiting < 123 {
		t.Fatalf("sınır 0, silinmeyen kayıt: %+v", r)
	}
	if !hostHasRecord(id) {
		t.Fatal("bekleme kaydı sildi")
	}
}

// Bekleme sürerken başka bir goroutine blok ayırır: yeni blok kümede değildir, bekleme kümenin kaydı
// silinince biter, sınıra varmaz (DENETİM 458 Ş1 (b)). Yarış dedektörüyle koşar; -short'ta atlanmaz.
func TestHostSettleIgnoresBlocksAllocatedMeanwhile(t *testing.T) {
	needBlock(t, hostSize*2)
	id := hostRegister(&HostBlock{}, 1<<20)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		var kept []*HostBlock
		for {
			select {
			case <-stop:
				runtime.KeepAlive(kept)
				return
			default:
			}
			if len(kept) < 8 {
				kept = append(kept, NewHostBlock(hostSize*2))
			}
			time.Sleep(time.Millisecond)
		}
	})
	time.AfterFunc(50*time.Millisecond, func() { hostUnregister(id) })
	r := settleHost(5 * time.Second)
	close(stop)
	wg.Wait()
	if r.TimedOut {
		t.Fatalf("yeni bloklar beklemeyi uzattı: %+v", r)
	}
	if r.Elapsed < 50*time.Millisecond {
		t.Fatalf("bekleme ölü kümenin kaydı silinmeden bitti: %+v", r)
	}
}

// Temizleyiciler taramayla aynı anda koşar: tarama sırasında silinen kayıt kümeye girmez, silmek
// isteyen temizleyici kilidi bekler; bekleme biter, sınıra varmaz (DENETİM 458 Ş1 (a)).
func TestHostSettleWithConcurrentCleanups(t *testing.T) {
	var ids []uint64
	for range 8 {
		ids = append(ids, releasedBlock(t, hostSize*2))
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 4 {
			runtime.GC()
		}
	})
	r := settleHost(hostSettleLimit)
	wg.Wait()
	if r.TimedOut {
		t.Fatalf("temizleyicilerle aynı anda bekleme sınıra vardı: %+v", r)
	}
	for _, id := range ids {
		if hostHasRecord(id) {
			t.Fatalf("kayıt %d temizleyiciden sonra duruyor", id)
		}
	}
}

// Aynı anda iki yükleme: her biri kendi ölü kümesini bekler, ikisi de sınıra varmaz.
func TestHostSettleTwoAtOnce(t *testing.T) {
	releasedBlock(t, hostSize*2)
	releasedBlock(t, hostSize*2)
	var wg sync.WaitGroup
	results := make([]HostSettle, 2)
	for i := range results {
		wg.Go(func() { results[i] = settleHost(hostSettleLimit) })
	}
	wg.Wait()
	for i, r := range results {
		if r.TimedOut {
			t.Fatalf("%d. bekleme sınıra vardı: %+v", i, r)
		}
	}
}

// Kayda yalnız verilen bloklar girer: eşiğin altındaki ve bütçe dolunca verilmeyen blok girmez.
func TestHostRegistryHoldsOnlyGivenBlocks(t *testing.T) {
	b := needBlock(t, hostBlockMin)
	if hostRecordOf(b) == 0 {
		t.Fatal("eşikteki blok kayda girmedi")
	}
	runtime.KeepAlive(b)
	before := hostRecordCount()
	if NewHostBlock(hostBlockMin-1) != nil {
		t.Fatal("eşiğin altında blok verildi")
	}
	withHostLimits(t, hostMemory.blocks.Load(), hostAddressSpace)
	if NewHostBlock(hostSize*2) != nil {
		t.Fatal("bütçe dolu, blok verildi")
	}
	if got := hostRecordCount(); got > before {
		t.Fatalf("verilmeyen blok kayda girdi: %d → %d", before, got)
	}
}

// BenchmarkHostSettle yüklemeden önceki beklemenin bedelidir (tasarım §11.2): blok yokken, ölü bir
// 64 MiB blokla, canlı bir blokla ve 1 GiB canlı Go verisiyle. Yalnız Colab'da koşar.
func BenchmarkHostSettle(b *testing.B) {
	const big = 64 << 20
	b.Run("live0", func(b *testing.B) {
		settleHost(hostSettleLimit)
		if hostMemory.live.Load() != 0 {
			b.Skip("öbek dışında blok kaldı")
		}
		for b.Loop() {
			SettleHostBlocks(big)
		}
	})
	b.Run("dead64MiB", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			if NewHostBlock(big) == nil {
				b.Skip("blok yok")
			}
			b.StartTimer()
			SettleHostBlocks(big)
		}
	})
	b.Run("live64MiB", func(b *testing.B) {
		keep := NewHostBlock(big)
		if keep == nil {
			b.Skip("blok yok")
		}
		for b.Loop() {
			SettleHostBlocks(big)
		}
		runtime.KeepAlive(keep)
	})
	b.Run("live64MiBHeap1GiB", func(b *testing.B) {
		keep := NewHostBlock(big)
		if keep == nil {
			b.Skip("blok yok")
		}
		x := new(int64)
		heap := make([]*int64, 1<<27)
		for i := range heap {
			heap[i] = x
		}
		for b.Loop() {
			SettleHostBlocks(big)
		}
		runtime.KeepAlive(keep)
		runtime.KeepAlive(heap)
	})
}

// BenchmarkHostRegistry kaydın her bloğa eklediği bedeldir: ekleme ve silme. Gerçek blok
// ayırmaz: verilen adres geri gelmez, döngü bütçeyi bitirirdi.
func BenchmarkHostRegistry(b *testing.B) {
	blk := &HostBlock{}
	for b.Loop() {
		hostUnregister(hostRegister(blk, hostBlockMin))
	}
}

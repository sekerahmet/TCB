package dataload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// sizedFile, verilen boyutta (bayt) sahte .npy dosyası yazar.
func sizedFile(t *testing.T, dir, ad string, token int, seed uint64) (string, int64) {
	t.Helper()
	path := writeU16(t, dir, ad, fakeU16(token, seed))
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, st.Size()
}

func isLocal(o *Cache, path string) bool {
	return strings.HasPrefix(path, o.Dir())
}

func TestCacheQuotaAndLRU(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	src, local := t.TempDir(), t.TempDir()
	a, size := sizedFile(t, src, "a.npy", 1000, 1)
	b, _ := sizedFile(t, src, "b.npy", 1000, 2)
	c, _ := sizedFile(t, src, "c.npy", 1000, 3)
	o, err := OpenCache(CacheOptions{Dir: local, Quota: size*2 + size/2})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	al := func(k string) (string, func()) {
		t.Helper()
		y, release, err := o.Fetch(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
		return y, release
	}
	ya, ba := al(a)
	if !isLocal(o, ya) {
		t.Fatalf("a yerelde değil: %s", ya)
	}
	ba()
	_, bb := al(b)
	bb()
	// a'yı yeniden kullan: artık b daha eski.
	_, ba = al(a)
	ba()
	_, bc := al(c) // kota 2.5 dosya: b silinmeli
	bc()
	statsNow := o.Stats()
	if statsNow.Evicted != 1 || statsNow.FileCount != 2 || statsNow.Usage > size*2+size/2 {
		t.Fatalf("kota uygulanmadı: %+v", statsNow)
	}
	if _, err := os.Stat(filepath.Join(local, localName(b))); !os.IsNotExist(err) {
		t.Fatal("en eski kullanılan (b) silinmedi")
	}
	if _, err := os.Stat(filepath.Join(local, localName(a))); err != nil {
		t.Fatal("yakın zamanda kullanılan (a) silindi")
	}
	if statsNow.Hits != 1 || statsNow.Misses != 3 {
		t.Fatalf("isabet/ıska yanlış: %+v", statsNow)
	}

	// Açık (iğneli) dosyalar silinmez: a ve c açıkken b için yer yok →
	// açık hata (kaynağa sessizce geri dönülmez).
	_, ba = al(a)
	_, bc = al(c)
	if y, _, err := o.Fetch(ctx, b); !errors.Is(err, ErrCacheInsufficient) || y != "" ||
		!strings.Contains(err.Error(), "kota") {
		t.Fatalf("yer yokken ErrOnbellekYetersiz bekleniyordu: %q %v", y, err)
	}
	ba()
	bc()
	if o.Stats().Skipped != 1 {
		t.Fatal("atlanan sayılmadı")
	}
	// Artık yer var: b alınabilmeli.
	_, bb = al(b)
	bb()
	// Kotadan büyük dosya hiç kopyalanmaz ve hata verir.
	large, _ := sizedFile(t, src, "buyuk.npy", 4000, 4)
	if _, _, err := o.Fetch(ctx, large); !errors.Is(err, ErrCacheInsufficient) {
		t.Fatalf("kotadan büyük dosyada hata bekleniyordu: %v", err)
	}
	_ = o.Close()
	if _, _, err := o.Fetch(ctx, a); err != ErrClosed {
		t.Fatalf("ErrKapali bekleniyordu: %v", err)
	}
	waitGoroutine(t, baseOff)
}

func TestCachePrefetchStaleAdoption(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	src, local := t.TempDir(), t.TempDir()
	var paths []string
	var size int64
	for i := 0; i < 4; i++ {
		y, b := sizedFile(t, src, shardName(i), 2000, uint64(i))
		paths, size = append(paths, y), b
	}
	o, err := OpenCache(CacheOptions{Dir: local, Quota: 3 * size})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	o.Prepare(paths) // 4 istendi, kotaya 3 sığar
	if err := o.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	statsNow := o.Stats()
	if statsNow.FileCount != 3 || statsNow.CopiedBytes != 3*size || statsNow.Evicted != 0 {
		t.Fatalf("önden yükleme kotayı aştı ya da istenenleri sildi: %+v", statsNow)
	}
	for i := 0; i < 3; i++ {
		y, release, err := o.Fetch(ctx, paths[i])
		if err != nil || !isLocal(o, y) {
			t.Fatalf("önden yüklenen dosya yerelde değil: %v", err)
		}
		release()
	}
	if o.Stats().Hits != 3 || o.Stats().Misses != 0 {
		t.Fatalf("önden yükleme isabet vermedi: %+v", o.Stats())
	}
	// İçerik aynı olmalı.
	y0, release, _ := o.Fetch(ctx, paths[0])
	k0, _ := os.ReadFile(paths[0])
	l0, _ := os.ReadFile(y0)
	release()
	if string(k0) != string(l0) {
		t.Fatal("yerel kopya kaynakla aynı değil")
	}

	// Kaynak değişince kopya bayat sayılır ve yeniden kopyalanır.
	writeU16(t, src, shardName(0), fakeU16(2500, 99))
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(paths[0], future, future)
	y0, release, err = o.Fetch(ctx, paths[0])
	if err != nil {
		t.Fatal(err)
	}
	l0, _ = os.ReadFile(y0)
	k0, _ = os.ReadFile(paths[0])
	release()
	if string(k0) != string(l0) {
		t.Fatal("bayat kopya yenilenmedi")
	}
	_ = o.Close()
	waitGoroutine(t, baseOff)

	// Yeniden açılışta mevcut kopyalar benimsenir; yarım kalmış geçici
	// dosyalar silinir; yabancı dosyalara dokunulmaz.
	temp := filepath.Join(local, localName(paths[3])+tempSuffix)
	_ = os.WriteFile(temp, []byte("yarım"), 0o644)
	foreign := filepath.Join(local, "benim_notum.txt")
	_ = os.WriteFile(foreign, []byte("dokunma"), 0o644)
	o2, err := OpenCache(CacheOptions{Dir: local, Quota: 4 * size})
	if err != nil {
		t.Fatal(err)
	}
	defer o2.Close()
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatal("geçici dosya silinmedi")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("yabancı dosya silindi")
	}
	// yollar[1] bayat kopya yenilenirken kota için silindi (en eski); [2] durmalı.
	y1, release, err := o2.Fetch(ctx, paths[2])
	if err != nil || !isLocal(o2, y1) {
		t.Fatal(err)
	}
	release()
	if statsNow := o2.Stats(); statsNow.Hits != 1 || statsNow.Misses != 0 {
		t.Fatalf("mevcut kopya benimsenmedi: %+v", statsNow)
	}
}

func TestCachedSamplerSameData(t *testing.T) {
	checkShardLeak(t)
	baseOff := runtime.NumGoroutine()
	d := encodedDataset(t, 3000, 2000, 2500, 1800, 2200)
	k, _ := OpenDataset(d, DatasetOptions{})
	pick := SamplerOptions{B: 8, T: 64, Seed: 5, GroupSize: 2, Lookahead: 1}
	o, _ := k.Sampler(pick)
	expected := takeBatches(t, o, 60)
	_ = o.Close()

	st, _ := os.Stat(k.Shards()[0].Path)
	// (GrupBoyu + IleriBak) × en büyük parça gerekli; 5 parça sığmaz,
	// eski gruplar silinmeli.
	ob, err := OpenCache(CacheOptions{Dir: t.TempDir(), Quota: 3 * st.Size()})
	if err != nil {
		t.Fatal(err)
	}
	kc, _ := OpenDataset(d, DatasetOptions{Cache: ob})
	oc, _ := kc.Sampler(pick)
	yk := Prefetch(context.Background(), oc, 3)
	var ti Batch
	var taken [][]int32
	for i := 0; i < 60; i++ {
		if err := yk.Take(&ti); err != nil {
			t.Fatal(err)
		}
		taken = append(taken, append([]int32(nil), ti.Data...))
	}
	_ = yk.Close()
	_ = oc.Close()
	_ = ob.Close()
	if !reflect.DeepEqual(taken, expected) {
		t.Fatal("önbellekli örnekleyici farklı veri verdi")
	}
	statsNow := ob.Stats()
	if statsNow.Usage > 3*st.Size() || statsNow.CopiedBytes == 0 || statsNow.Skipped != 0 || statsNow.Evicted == 0 {
		t.Fatalf("önbellek kullanılmadı ya da kota aşıldı: %+v", statsNow)
	}
	waitGoroutine(t, baseOff)
}

func TestCacheQuotaCheckNoSilentFallback(t *testing.T) {
	checkShardLeak(t)
	d := encodedDataset(t, 3000, 2000, 2500)
	st, _ := os.Stat(filepath.Join(d, shardName(0))) // en büyük parça
	ob, err := OpenCache(CacheOptions{Dir: t.TempDir(), Quota: 2 * st.Size()})
	if err != nil {
		t.Fatal(err)
	}
	defer ob.Close()
	k, _ := OpenDataset(d, DatasetOptions{Cache: ob})
	// Varsayılanlarla (grup 4 → 3 parça, ileri bakış 3) tüm parçalar
	// gerekir → kurulmaz; grup 2 + ileri bakış 0 da 2'yi aşmaz ama grup 2 +
	// ileri bakış 1 aşar.
	for _, s := range []SamplerOptions{{B: 2, T: 8}, {B: 2, T: 8, GroupSize: 2, Lookahead: 1}} {
		if _, err := k.Sampler(s); !errors.Is(err, ErrCacheInsufficient) {
			t.Fatalf("yetersiz kota için hata bekleniyordu: %+v %v", s, err)
		}
	}
	if o, err := k.Sampler(SamplerOptions{B: 2, T: 8, GroupSize: 2, Lookahead: -1}); err != nil {
		t.Fatal(err)
	} else {
		_ = o.Close()
	}
	// Grup 1 + ileri bakış 1 ile 2 parça yeter.
	o, err := k.Sampler(SamplerOptions{B: 2, T: 8, GroupSize: 1, Lookahead: 1})
	if err != nil {
		t.Fatal(err)
	}
	var ti Batch
	if err := o.Next(&ti); err != nil {
		t.Fatal(err)
	}
	_ = o.Close()
	// Kotayı iğneli dosyalarla doldurunca parça açma kaynağa dönmez, hata verir.
	p0, err := k.OpenShard(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	p1, err := k.OpenShard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !isLocal(ob, p0.Path()) || !isLocal(ob, p1.Path()) {
		t.Fatal("parçalar yerel kopyadan açılmadı")
	}
	if p, err := k.OpenShard(context.Background(), 2); !errors.Is(err, ErrCacheInsufficient) {
		if p != nil {
			_ = p.Close()
		}
		t.Fatalf("yer yokken parça açma hata vermeli: %v", err)
	}
	_ = p0.Close()
	_ = p1.Close()
	// Sıralı okuyucu için tek parça yeter; daha küçük kota reddedilir.
	ob2, _ := OpenCache(CacheOptions{Dir: t.TempDir(), Quota: st.Size() - 1})
	defer ob2.Close()
	k2, _ := OpenDataset(d, DatasetOptions{Cache: ob2})
	if _, err := k2.Sequential(SequentialOptions{B: 1, T: 8}); !errors.Is(err, ErrCacheInsufficient) {
		t.Fatalf("sıralı okuyucu yetersiz kotayı kabul etti: %v", err)
	}
}

func TestCacheCopyValidation(t *testing.T) {
	src, local := t.TempDir(), t.TempDir()
	a, size := sizedFile(t, src, "a.npy", 1000, 1)
	st, _ := os.Stat(a)
	target := filepath.Join(local, localName(a))
	// Beklenenden kısa kopya (kaynak kopya sırasında kısalmış gibi) reddedilir
	// ve geride ne hedef ne geçici dosya kalır.
	err := copyFile(context.Background(), a, target, size+10, st.ModTime())
	if err == nil || !strings.Contains(err.Error(), "eksik") {
		t.Fatalf("kısa kopya hatası bekleniyordu: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("kısa kopya hedef adına taşındı")
	}
	if _, err := os.Stat(target + tempSuffix); !os.IsNotExist(err) {
		t.Fatal("geçici dosya kaldı")
	}
	if err := copyFile(context.Background(), a, target, size, st.ModTime()); err != nil {
		t.Fatal(err)
	}

	// Açılışta bozuk başlıklı ya da kesik kopyalar benimsenmez ve silinir.
	b, _ := sizedFile(t, src, "b.npy", 1000, 2)
	c, _ := sizedFile(t, src, "c.npy", 1000, 3)
	corrupt := filepath.Join(local, localName(b))
	_ = os.WriteFile(corrupt, []byte("bu bir npy değil, bozuk kopya"), 0o644)
	truncated := filepath.Join(local, localName(c))
	raw, _ := os.ReadFile(c)
	_ = os.WriteFile(truncated, raw[:len(raw)/2], 0o644)
	o, err := OpenCache(CacheOptions{Dir: local, Quota: 10 * size})
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if statsNow := o.Stats(); statsNow.FileCount != 1 {
		t.Fatalf("yalnızca geçerli kopya benimsenmeli: %+v", statsNow)
	}
	for _, y := range []string{corrupt, truncated} {
		if _, err := os.Stat(y); !os.IsNotExist(err) {
			t.Fatalf("bozuk kopya silinmedi: %s", y)
		}
	}
	// Geçerli kopya isabet verir; bozuklar yeniden kopyalanır.
	for _, k := range []string{a, b, c} {
		y, release, err := o.Fetch(context.Background(), k)
		if err != nil || !isLocal(o, y) {
			t.Fatal(err)
		}
		release()
	}
	if statsNow := o.Stats(); statsNow.Hits != 1 || statsNow.Misses != 2 {
		t.Fatalf("isabet/ıska yanlış: %+v", statsNow)
	}
}

// runningWorkers cekirdekAc'ın başlattığı işçi gorutinlerini sayar ("created by" satırı
// gorutin henüz çalışmaya başlamamışken de vardır; giriş fonksiyonunun adı o sırada farklı
// görünür). Toplam gorutin sayısıyla karşılaştırmak kararsızdı: önceki testlerden kalan
// gorutinler bu arada bitince sayı düşüyor, işçiler başlamış olsa da test kalıyordu.
func runningWorkers() int {
	b := make([]byte, 1<<20)
	return strings.Count(string(b[:runtime.Stack(b, true)]), "created by github.com/sekerahmet/TCB/internal/compute/dataload.openCore ")
}

func TestCacheWorkerCounts(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	src := t.TempDir()
	var paths []string
	var size int64
	for i := 0; i < 6; i++ {
		y, b := sizedFile(t, src, shardName(i), 3000, uint64(i))
		paths, size = append(paths, y), b
	}
	for _, worker := range []int{1, 2, 4} {
		o, err := OpenCache(CacheOptions{Dir: t.TempDir(), Quota: 6 * size, Workers: worker})
		if err != nil {
			t.Fatal(err)
		}
		if n := runningWorkers(); n != worker {
			t.Fatalf("%d işçi gorutini bekleniyordu, %d çalışıyor", worker, n)
		}
		o.Prepare(paths)
		if err := o.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
		// Bekle dönmeden önce son işçi bitmiş olabilir; tüm dosyalar yerelde olmalı.
		for _, y := range paths {
			yl, release, err := o.Fetch(context.Background(), y)
			if err != nil || !isLocal(o, yl) {
				t.Fatal(err)
			}
			release()
		}
		statsNow := o.Stats()
		if statsNow.CopiedBytes != 6*size || statsNow.Hits != 6 {
			t.Fatalf("işçi %d: önden kopya eksik ya da tekrarlı: %+v", worker, statsNow)
		}
		_ = o.Close()
		waitGoroutine(t, baseOff)
	}
}

// Kota yalnızca süren bir önden yükleme kopyası yüzünden geçici olarak
// doluyken eşzamanlı Getir hata vermemeli; kopyanın bitmesini bekleyip onu
// silerek yer açmalı. (CI'da kapanan örnekleyicinin istediği sıradaki
// parçanın kopyası sürerken parça açma ErrOnbellekYetersiz dönüyordu.)
func TestCacheWaitsOngoingPrefetch(t *testing.T) {
	baseOff := runtime.NumGoroutine()
	src := t.TempDir()
	a, size := sizedFile(t, src, "a.npy", 1000, 1)
	b, _ := sizedFile(t, src, "b.npy", 1000, 2)
	c, _ := sizedFile(t, src, "c.npy", 1000, 3)
	started, release := make(chan struct{}), make(chan struct{})
	copyHook = func(k string, prefetch bool) {
		if prefetch && k == b {
			close(started)
			<-release
		}
	}
	defer func() { copyHook = nil }()
	o, err := OpenCache(CacheOptions{Dir: t.TempDir(), Quota: 2 * size, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, ba, err := o.Fetch(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	o.Prepare([]string{b})
	<-started // b'nin arka plan kopyası sürüyor: kota a + b ile dolu

	// a açıkken c için yer ancak b bitip silinince açılır: Getir beklemeli.
	type result struct {
		path    string
		release func()
		err     error
	}
	sc := make(chan result, 1)
	go func() {
		y, bb, err := o.Fetch(ctx, c)
		sc <- result{y, bb, err}
	}()
	select {
	case s := <-sc:
		t.Fatalf("Getir önden yükleme bitmeden döndü: %v", s.err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	s := <-sc
	if s.err != nil || !isLocal(o, s.path) {
		t.Fatalf("süren önden yükleme beklenmedi: %v", s.err)
	}
	s.release()
	ba()
	if statsNow := o.Stats(); statsNow.Evicted != 1 || statsNow.Usage > 2*size {
		t.Fatalf("b silinerek yer açılmadı: %+v", statsNow)
	}

	// Süren önden yükleme yokken ve açık dosyalar yer kaplarken beklemeden
	// hata döner.
	copyHook = nil
	_, ba, _ = o.Fetch(ctx, a)
	_, bc, _ := o.Fetch(ctx, c)
	if _, _, err := o.Fetch(ctx, b); !errors.Is(err, ErrCacheInsufficient) {
		t.Fatalf("yer yokken hata bekleniyordu: %v", err)
	}
	ba()
	bc()
	_ = o.Close()
	waitGoroutine(t, baseOff)
}

// Aynı dizinle açılan iki Onbellek tek yöneticiyi paylaşır: kota ortaktır,
// ikinci açılış birincinin süren kopyasına (geçici dosya) dokunmaz.
func TestCacheSharedDir(t *testing.T) {
	src, local := t.TempDir(), t.TempDir()
	var paths []string
	var size int64
	for i := 0; i < 6; i++ {
		y, b := sizedFile(t, src, fmt.Sprintf("p%d.npy", i), 2000, uint64(i))
		paths, size = append(paths, y), b
	}
	a, err := OpenCache(CacheOptions{Dir: local, Quota: 3 * size})
	if err != nil {
		t.Fatal(err)
	}
	// Birincinin yarım kalmış gibi görünen (süren) kopyası.
	temp := filepath.Join(local, localName(paths[5])+tempSuffix)
	if err := os.WriteFile(temp, []byte("süren kopya"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := OpenCache(CacheOptions{Dir: filepath.Join(local, "."), Quota: 3 * size})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(temp); err != nil {
		t.Fatal("ikinci açılış birincinin geçici dosyasını sildi")
	}
	if a.c != b.c {
		t.Fatal("aynı dizin için iki ayrı yönetici kuruldu")
	}
	ctx := context.Background()
	a.Prepare(paths[:3])
	b.Prepare(paths[3:5])
	if err := a.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	for _, y := range paths[:5] {
		path, release, err := a.Fetch(ctx, y)
		if err != nil {
			t.Fatal(err)
		}
		if !isLocal(a, path) {
			t.Fatal("kopya yerelde değil")
		}
		release()
		if k := a.Stats().Usage; k > a.Quota() {
			t.Fatalf("paylaşılan kota aşıldı: %d > %d", k, a.Quota())
		}
	}
	// Ayırmalar toplanır.
	if err := a.Reserve(2 * size); err != nil {
		t.Fatal(err)
	}
	if err := b.Reserve(2 * size); err == nil || !errors.Is(err, ErrCacheInsufficient) {
		t.Fatalf("paylaşılan ayırma kotayı aşmalıydı: %v", err)
	}
	if err := b.Reserve(size); err != nil {
		t.Fatal(err)
	}
	// Biri kapanınca öteki çalışmayı sürdürür; sonuncusu kapanınca dizin
	// yeniden (yeni kotayla) açılabilir.
	_ = a.Close()
	if _, release, err := b.Fetch(ctx, paths[0]); err != nil {
		t.Fatalf("paylaşan kapandıktan sonra: %v", err)
	} else {
		release()
	}
	if _, _, err := a.Fetch(ctx, paths[0]); !errors.Is(err, ErrClosed) {
		t.Fatalf("kapalı Onbellek: %v", err)
	}
	_ = b.Close()
	c, err := OpenCache(CacheOptions{Dir: local, Quota: 4 * size})
	if err != nil {
		t.Fatal(err)
	}
	if c.c == a.c || c.Quota() != 4*size {
		t.Fatal("kapanan yönetici yeniden kullanıldı")
	}
	_ = c.Close()
}

// Dizin başka bir tutamakta (başka bir süreçte) kilitliyse açılış açık
// hatayla reddedilir.
func TestCacheCrossProcessLock(t *testing.T) {
	local := t.TempDir()
	f, err := os.OpenFile(filepath.Join(local, lockPath), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := lockFile(f); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenCache(CacheOptions{Dir: local, Quota: 1 << 20}); !errors.Is(err, ErrCacheLocked) {
		_ = f.Close()
		t.Fatalf("kilitli dizin reddedilmeliydi: %v", err)
	}
	_ = f.Close() // kilit bırakılır
	o, err := OpenCache(CacheOptions{Dir: local, Quota: 1 << 20})
	if err != nil {
		t.Fatalf("kilit bırakılınca açılmalı: %v", err)
	}
	_ = o.Close()
}

// TestCacheSameTime saniye kesri olan kopya zamanının 10 ms, kesirsiz (saniyeye yuvarlanmış)
// zamanın ±2 sn toleransla karşılaştırıldığını denetler.
func TestCacheSameTime(t *testing.T) {
	base := time.Date(2026, 10, 19, 12, 0, 10, 0, time.UTC)
	precise := base.Add(300 * time.Millisecond)
	for _, d := range []struct {
		cached, source time.Time
		want           bool
	}{
		{precise, precise, true},
		{precise, precise.Add(80 * time.Nanosecond), true},   // NTFS'nin 100 ns yuvarlaması
		{precise, precise.Add(20 * time.Millisecond), false}, // kesirli zaman: değişiklik görülür
		{base, base.Add(1900 * time.Millisecond), true},      // yuvarlanmış zaman (FAT): tolerans
		{base, base.Add(2 * time.Second), false},
		{base, base.Add(-1500 * time.Millisecond), true},
	} {
		if got := sameTime(d.cached, d.source); got != d.want {
			t.Errorf("sameTime(%v, %v) = %v, %v bekleniyordu", d.cached, d.source, got, d.want)
		}
	}
}

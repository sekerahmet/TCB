package gpu

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Kaydedilip oynatılan iş, aynı girdiyle kayıtsız çalıştırmayla bit düzeyinde aynıdır: girdi
// kayıttan sonra yerinde değiştirilir, oynatma yeni girdiyi okur.
func TestGraphReplayMatchesEager(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	rng := rand.New(rand.NewSource(41))
	x := tensor.NewOn(gpuDevice, random(rng, 4*40, 1), 4, 40)
	w := tensor.NewOn(gpuDevice, random(rng, 40*24, 1), 40, 24)
	v := tensor.NewOn(gpuDevice, random(rng, 24, 1), 1, 24)
	f := func() *tensor.Tensor { return tensor.Add(tensor.MatMul(x, w).Relu(), v) }
	f() // ısınma: çekirdekler derlenir
	g.BeginCapture()
	out := f()
	gr := g.EndCapture()
	defer gr.Release()
	for i := 0; i < 3; i++ {
		x.CopyFrom(tensor.NewOn(gpuDevice, random(rng, 4*40, 1), 4, 40))
		want := f().Data()
		gr.Replay()
		got := out.Data()
		for j := range want {
			if got[j] != want[j] {
				t.Fatalf("oynatma %d, [%d]: %v, kayıtsız %v", i, j, got[j], want[j])
			}
		}
	}
}

// Kayıt sırasında cihazdan okuma, konaktan yükleme ve iç içe kayıt kodlu hatadır; kayıt
// bırakılınca arka uç normal çalışır.
func TestGraphForbiddenOps(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	x := tensor.NewOn(gpuDevice, []float32{1, 2, 3}, 3)
	x.Relu().Data()
	for _, c := range []struct {
		code messages.Code
		f    func()
	}{
		{messages.H4065, func() { x.Relu().Data() }},
		{messages.H4066, func() { tensor.NewOn(gpuDevice, []float32{4, 5}, 2) }},
		{messages.H4068, func() { g.BeginCapture() }},
	} {
		g.BeginCapture()
		err := catchPanic(c.f)
		g.AbortCapture()
		if !messages.Has(err, c.code) {
			t.Fatalf("%s bekleniyordu, %v", c.code, err)
		}
	}
	if got := x.Relu().Data(); got[2] != 3 {
		t.Fatalf("kayıt bırakıldıktan sonra sonuç %v", got)
	}
}

// Kayıt içinde bırakılan blok kayıt kapanana kadar kayıt dışındaki işlere verilmez (oynatma
// ona yazardı); kayıt bırakılınca genel havuza döner.
func TestGraphPrivatePool(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	const n = 1000
	g.BeginCapture()
	tmp := g.storageOf(g.AllocUninit(n, tensor.Float32))
	addr := tmp.addr
	g.Release(tmp)
	gr := g.EndCapture()
	var live []tensor.Storage
	for i := 0; i < 8; i++ {
		s := g.AllocUninit(n, tensor.Float32)
		live = append(live, s)
		if g.storageOf(s).addr == addr {
			t.Fatalf("kaydın bloğu kayıt sürerken başka işe verildi")
		}
	}
	gr.Release()
	found := false
	for i := 0; i < 64 && !found; i++ {
		s := g.AllocUninit(n, tensor.Float32)
		live = append(live, s)
		found = g.storageOf(s).addr == addr
	}
	if !found {
		t.Fatalf("kayıt bırakılınca blok genel havuza dönmedi")
	}
	for _, s := range live {
		g.Release(s)
	}
}

func catchPanic(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err, _ = r.(error)
		}
	}()
	f()
	return nil
}

// Kayıt içinde indeksi cihazdan okuyan gömme ve dilim yazma: her oynatma indeks tensörünün o
// anki değerini okur; sınır dışı indeks kırpılır (gömmede [0, V−1], dilimde [0, Td−Ts]).
func TestGraphIndexedOps(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	rng := rand.New(rand.NewSource(42))
	const V, D, B, Td, Ts, I = 7, 5, 2, 6, 2, 3
	tablo := random(rng, V*D, 1)
	w := tensor.NewOn(gpuDevice, tablo, V, D)
	ids := tensor.NewOn(gpuDevice, []float32{0, 0}, 2)
	pos := tensor.NewOn(gpuDevice, []float32{0}) // 0 boyutlu
	cache := tensor.NewOn(gpuDevice, make([]float32, B*Td*I), B, Td, I)
	src := tensor.NewOn(gpuDevice, random(rng, B*Ts*I, 1), B, Ts, I)
	step := func() *tensor.Tensor {
		tensor.UpdateSliceAt(cache, src, pos, 1)
		return tensor.EmbeddingAt(w, ids)
	}
	step()
	g.BeginCapture()
	out := step()
	gr := g.EndCapture()
	defer gr.Release()
	srcData := src.Data()
	want := cache.Data() // ısınma çağrısı 0. konuma yazdı
	for _, c := range []struct {
		ids [2]float32
		pos float32
	}{{[2]float32{3, 6}, 1}, {[2]float32{-4, 99}, 9}, {[2]float32{2, 1}, 0}} {
		ids.CopyFrom(tensor.NewOn(gpuDevice, c.ids[:], 2))
		pos.CopyFrom(tensor.NewOn(gpuDevice, []float32{c.pos}))
		gr.Replay()
		got := out.Data()
		for r, id := range c.ids {
			row := min(max(int(id), 0), V-1)
			for j := 0; j < D; j++ {
				if got[r*D+j] != tablo[row*D+j] {
					t.Fatalf("gömme indeks %v: [%d,%d] = %v, beklenen %v", id, r, j, got[r*D+j], tablo[row*D+j])
				}
			}
		}
		s := min(max(int(c.pos), 0), Td-Ts)
		for b := 0; b < B; b++ {
			for a := 0; a < Ts; a++ {
				copy(want[(b*Td+s+a)*I:(b*Td+s+a+1)*I], srcData[(b*Ts+a)*I:(b*Ts+a+1)*I])
			}
		}
		checkDiff(t, "dilim yazma", cache.Data(), want, 0)
	}
}

// Kayıttan önce ayrılıp kayda giren ve kayıt sırasında bırakılan blok (oynatma onu okur) kayıt
// bırakılana kadar başka işe verilmez; kayda girmeden bırakılan blok genel havuza döner.
func TestGraphKeepsBlocksFreedDuringCapture(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	const n = 1000
	used := g.storageOf(g.AllocUninit(n, tensor.Float32))
	unused := g.storageOf(g.AllocUninit(n, tensor.Float32))
	usedAddr, unusedAddr := used.addr, unused.addr
	g.BeginCapture()
	g.Fill(used, 1) // kayda girer
	g.Release(used)
	g.Release(unused) // kayda girmedi: genel havuza döner
	gr := g.EndCapture()
	var live []tensor.Storage
	reused := false
	for i := 0; i < 8; i++ {
		s := g.AllocUninit(n, tensor.Float32)
		live = append(live, s)
		switch g.storageOf(s).addr {
		case usedAddr:
			t.Fatalf("kaydın okuduğu, kayıt sırasında bırakılan blok kayıt yaşarken başka işe verildi")
		case unusedAddr:
			reused = true
		}
	}
	if !reused {
		t.Errorf("kayda girmeden bırakılan blok genel havuza dönmedi")
	}
	gr.Release()
	for _, s := range live {
		g.Release(s)
	}
}

// Kural: kayıttan önce ayrılmış, kayda girmiş ve kayıt sırasında bırakılmış blok aynı kayıt
// içinde yeniden verilmez: verilseydi sonraki iş onun verisini ezer, ikinci oynatma ezilmiş
// veriyi okurdu (Gemini kayıt denetimi, bulgu 1A).
func TestGraphHeldBlockNotReusedInsideCapture(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	old := tensor.NewOn(gpuDevice, []float32{10, 20, 30, 40}, 4)
	x := tensor.NewOn(gpuDevice, []float32{1, 2, 3, 4}, 4)
	tensor.Mul(old, x).Drop() // ısınma: çekirdekler derlenir
	x.AddScalar(100).Drop()
	g.BeginCapture()
	out := tensor.Mul(old, x)
	old.Drop()
	tmp := x.AddScalar(100) // aynı boy: old'un bloğunu almamalı
	gr := g.EndCapture()
	defer gr.Release()
	defer out.Drop()
	defer tmp.Drop()
	want := []float32{10, 40, 90, 160}
	for i := 0; i < 2; i++ {
		gr.Replay()
		if got := out.Data(); !reflect.DeepEqual(got, want) {
			t.Fatalf("%d. oynatma %v, beklenen %v", i+1, got, want)
		}
	}
}

// Kural: kayda girmiş bir depolamayı kayıt yaşadıkça kayıt tutar: başlığı ulaşılamaz olsa da
// çöp toplayıcı bloğu genel havuza döndürmez (bulgu 1B).
func TestGraphHoldsStoragesAgainstCollection(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	w := tensor.NewOn(gpuDevice, []float32{1, 2, 3, 4}, 4)
	w.AddScalar(1).Drop() // ısınma
	w.Drop()
	var out *tensor.Tensor
	func() {
		old := tensor.NewOn(gpuDevice, []float32{5, 6, 7, 8}, 4)
		g.BeginCapture()
		out = old.AddScalar(1) // old bundan sonra ulaşılamaz (açık bırakma yok)
	}()
	gr := g.EndCapture()
	defer gr.Release()
	defer out.Drop()
	var others []*tensor.Tensor
	defer func() {
		for _, r := range others {
			r.Drop()
		}
	}()
	want := []float32{6, 7, 8, 9}
	for i := 0; i < 20; i++ {
		runtime.GC()
		for j := 0; j < 8; j++ {
			others = append(others, tensor.NewOn(gpuDevice, []float32{999, 999, 999, 999}, 4))
		}
		gr.Replay()
		if got := out.Data(); !reflect.DeepEqual(got, want) {
			t.Fatalf("kaydın okuduğu blok kayıt yaşarken başka işe verildi: %v, beklenen %v", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

// Kural: kayıtta konağa okuma (H4065) ve konaktan yükleme (H4066) veriden bağımsız hatadır:
// konak gölgesi olan tensörden okuma da, boş tensör de denetimden geçer; geçseydi okunan değer
// kayda sabitlenir, sonraki değişiklik oynatmaya yansımazdı (bulgu 5).
func TestGraphHostTransfersForbiddenEvenFromShadow(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	x := tensor.NewOn(gpuDevice, []float32{10}, 1) // konak gölgesi var
	empty := tensor.NewOn(gpuDevice, []float32{}, 0)
	g.BeginCapture()
	errShadow := catchPanic(func() { _ = x.Item() })
	errRead := catchPanic(func() { _ = empty.Data() })
	errLoad := catchPanic(func() { _ = tensor.NewOn(gpuDevice, []float32{}, 0) })
	g.AbortCapture()
	if !messages.Has(errShadow, messages.H4065) || !messages.Has(errRead, messages.H4065) || !messages.Has(errLoad, messages.H4066) {
		t.Fatalf("gölge %v, boş okuma %v, boş yükleme %v", errShadow, errRead, errLoad)
	}
}

// Kural: kaydın havuzunda yeniden kullanılan blok konum izlemesinde bir kez sayılır; kayıt
// bırakılınca konum başına canlı bellek sıfıra döner (bulgu 9A).
func TestGraphPoolReuseCountedOnce(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	site := uint64(1)
	SetAllocSite(func() uint64 { return site })
	defer SetAllocSite(nil)
	g.BeginCapture()
	for i := uint64(1); i <= 4; i++ {
		site = i
		g.Release(g.AllocUninit(64, tensor.Float32)) // 256 bayt, aynı anda tek blok canlı
	}
	g.EndCapture().Release()
	_, peak := PeakLiveBySite()
	g.pool.mu.Lock()
	live := g.pool.siteLive
	g.pool.mu.Unlock()
	if peak != 256 || live != 0 {
		t.Fatalf("tepe %d (beklenen 256), canlı %d (beklenen 0)", peak, live)
	}
}

// Kural: kayıt sürerken çöp toplayıcının bıraktığı, kayda girmemiş blok genel havuza döner ve
// collectGarbage onu verebilir (bulgu 9B).
func TestGraphCollectGarbageDuringCapture(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	const class = 64 << 10
	g.pool.mu.Lock()
	g.pool.empty[class] = nil
	g.pool.mu.Unlock()
	func() { _ = g.AllocUninit(class/4, tensor.Float32) }() // ulaşılamaz
	g.BeginCapture()
	defer g.AbortCapture()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := g.collectGarbage(class); ok {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("collectGarbage kayıt sürerken boşalan bloğu bulamadı")
}

// Kural: sabit tablo sınırı dolunca (kayıt dışında) tablolar bırakılıp baştan doldurulur:
// ısınmada yüklenen tablo kayıtta bulunur, H4066 olmaz (bulgu 9C).
func TestGraphConstIntsRefillAfterLimit(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	g.enter()
	saved := g.ints
	g.ints = make(map[string]*storage, constIntLimit)
	for i := 0; i < constIntLimit; i++ {
		g.ints[fmt.Sprintf("dolu:%d", i)] = &storage{g: g}
	}
	gt := g.temp()
	_ = g.constInts(gt, []int{1, 2, 3, 4})
	gt.finish()
	g.exit()
	defer func() {
		g.enter()
		g.ints = saved
		g.exit()
	}()
	g.BeginCapture()
	err := catchPanic(func() {
		g.enter()
		defer g.exit()
		gt := g.temp()
		defer gt.finish()
		_ = g.constInts(gt, []int{1, 2, 3, 4})
	})
	g.AbortCapture()
	if err != nil {
		t.Fatalf("ısınmada yüklenen tablo kayıtta bulunmadı: %v", err)
	}
}

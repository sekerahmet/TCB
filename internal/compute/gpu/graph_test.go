package gpu

import (
	"math/rand"
	"testing"

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

// Kayıttan önce ayrılıp kayıt sırasında bırakılan blok (kaydedilen iş okumuş olabilir) kayıt
// bırakılana kadar başka işe verilmez.
func TestGraphKeepsBlocksFreedDuringCapture(t *testing.T) {
	g := gpuBackend()
	if !g.CaptureSupported() {
		t.Skip("sürücü kaydı desteklemiyor")
	}
	const n = 1000
	old := g.storageOf(g.AllocUninit(n, tensor.Float32))
	addr := old.addr
	g.BeginCapture()
	g.Release(old)
	gr := g.EndCapture()
	var live []tensor.Storage
	for i := 0; i < 8; i++ {
		s := g.AllocUninit(n, tensor.Float32)
		live = append(live, s)
		if g.storageOf(s).addr == addr {
			t.Fatalf("kayıt sırasında bırakılan blok kayıt sürerken başka işe verildi")
		}
	}
	gr.Release()
	for _, s := range live {
		g.Release(s)
	}
}

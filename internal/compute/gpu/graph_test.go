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

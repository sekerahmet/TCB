package gpu

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Benzetimin hata yakalama yetenekleri ve arka ucun hata mesajları. Her sınama kendi
// benzetim arka ucunu kurar (yapışkan hata diğerlerini etkilemesin).

// Eşzamanlı kip (TR_GPU_ESZAMANLI=1) bu arka uçlarda kapalıdır: hatayı başlatmada değil
// eşzamanlamada beklerler; ortam değişkeninden bağımsız olmalıdırlar.
func newSimBackend(memory uint64) *backend {
	g := newBackend(tensor.GPU(7), newSim(memory))
	g.concurrent = false
	return g
}

// runErr, govde'nin yazdığı tek bloklu (4 iş parçacığı) çekirdeği 16 baytlık bir
// tamponla başlatır ve eşzamanlama hatasının mesajını verir.
func runErr(t *testing.T, ad string, body func(k *emitter, x reg)) error {
	t.Helper()
	g := newSimBackend(1 << 20)
	g.enter()
	defer g.exit()
	d := g.allocate(4, tensor.Float32, true)
	c := g.takeKernel(ad, func() *emitter {
		k := newEmitter(ad)
		body(k, k.written("x"))
		return k
	})
	g.launch(c, [3]uint32{1, 1, 1}, [3]uint32{4, 1, 1}, g.param().write(d, 0))
	return panicError(g.equalize)
}

func TestSimCatchesRace(t *testing.T) {
	err := runErr(t, "yaris", func(k *emitter, x reg) {
		sm := k.shared("sm", 16)
		tid := k.special("%tid.x")
		v := k.f()
		k.e("cvt.rn.f32.u32", v, tid)
		k.e("st.shared.f32", mem(sm, 0), v) // dört iş parçacığı aynı sözcüğe yazar
	})
	if !messages.Has(err, messages.H4101) || !strings.Contains(err.Error(), "CUDA_ERROR_ILLEGAL_ADDRESS") {
		t.Fatalf("paylaşımlı bellek yarışı yakalanmadı: %v", err)
	}
	// Engelle ayrılmış yazma ve okuma yarış değildir.
	err = runErr(t, "engelli", func(k *emitter, x reg) {
		sm := k.shared("sm", 16)
		tid := k.special("%tid.x")
		addr := k.r()
		k.e("mad.lo.s32", addr, tid, 4, sm)
		v := k.f()
		k.e("cvt.rn.f32.u32", v, tid)
		k.e("st.shared.f32", mem(addr, 0), v)
		k.barrier()
		r := k.f()
		k.e("ld.shared.f32", r, mem(sm, 4))
		k.store(tensor.Float32, k.address(x, tid, 4), r)
	})
	if err != nil {
		t.Fatalf("engelli erişim hata verdi: %v", err)
	}
}

func TestSimDivergentBarrier(t *testing.T) {
	err := runErr(t, "daginik", func(k *emitter, x reg) {
		tid := k.special("%tid.x")
		last := k.label()
		p := k.p()
		k.e("setp.ne.u32", p, tid, 0)
		k.jumpIf(p, last)
		k.barrier()
		k.put(last)
	})
	if !messages.Has(err, messages.H4108) {
		t.Fatalf("dağınık engel yakalanmadı: %v", err)
	}
}

func TestSimOutOfBounds(t *testing.T) {
	err := runErr(t, "sinir_disi", func(k *emitter, x reg) {
		tid := k.special("%tid.x")
		i := k.r()
		// 1000..1003: tamponun boyut sınıfının (256 bayt) da dışı; benzetim yalnızca ayrılmış
		// bölgenin (sınıf boyu) dışını yakalar, donanım gibi.
		k.e("add.s32", i, tid, 1000)
		k.store(tensor.Float32, k.address(x, i, 4), k.constant(1))
	})
	if !messages.Has(err, messages.H4100) || !messages.Has(err, messages.H4009) && !messages.Has(err, messages.H4008) || !strings.Contains(err.Error(), "sinir_disi") {
		t.Fatalf("sınır dışı yazma yakalanmadı: %v", err)
	}
}

func TestPTXCompileErrorLog(t *testing.T) {
	old := ptxDumpDir
	ptxDumpDir = t.TempDir()
	t.Cleanup(func() { ptxDumpDir = old })
	g := newSimBackend(1 << 20)
	g.enter()
	defer g.exit()
	err := panicError(func() {
		g.takeKernel("bozuk", func() *emitter {
			k := newEmitter("bozuk")
			k.e("bilinmeyen.f32", k.f(), k.f())
			return k
		})
	})
	if !messages.Has(err, messages.H4011) || !messages.Has(err, messages.H4012) || !messages.Has(err, messages.H4013) || !strings.Contains(err.Error(), "CUDA_ERROR_INVALID_PTX") {
		t.Fatalf("derleme hatası mesajı eksik: %v", err)
	}
}

// TestParamDeclChecked yazan çekirdeğe "oku" ile verilen depolamanın iç hata
// olduğunu denetler (gölge silinmesinin unutulamaması bunun üzerine kuruludur).
func TestParamDeclChecked(t *testing.T) {
	g := newSimBackend(1 << 20)
	g.enter()
	defer g.exit()
	d := g.allocate(4, tensor.Float32, true)
	c := g.takeKernel("doldur:float32x", fillKernel(tensor.Float32))
	err := panicError(func() { g.launch(c, [3]uint32{1, 1, 1}, [3]uint32{4, 1, 1}, g.param().read(d, 0).u32(4).f32(1)) })
	if !messages.Has(err, messages.H9000) || !strings.Contains(internalDetail(err), "does not match the signature") {
		t.Fatalf("yanlış bildirim yakalanmadı: %v", err)
	}
}

// TestNestedParamChain, bir başlatmanın parametreleri yazılırken ikinci bir başlatma
// hazırlanmasının (dış zinciri silerdi) iç hata verdiğini ve sonraki girişin temiz
// başladığını denetler.
func TestNestedParamChain(t *testing.T) {
	g := newSimBackend(1 << 20)
	g.enter()
	err := panicError(func() { g.param().u32(1); g.param() })
	g.exit()
	if !messages.Has(err, messages.H9000) || !strings.Contains(internalDetail(err), "nested param()") {
		t.Fatalf("iç içe zincir yakalanmadı: %v", err)
	}
	g.enter()
	defer g.exit()
	if m := panicText(func() { g.param() }); m != "" {
		t.Fatalf("yeni girişte zincir kapalı olmalıydı: %q", m)
	}
}

func TestOutOfMemory(t *testing.T) {
	g := newSimBackend(1 << 20)
	err := panicError(func() {
		var hold []tensor.Storage
		for i := 0; i < 8; i++ {
			hold = append(hold, g.Alloc(64<<10, tensor.Float32)) // 256 KB
		}
		_ = hold
	})
	if !messages.Has(err, messages.H4014) || !strings.Contains(err.Error(), "CUDA_ERROR_OUT_OF_MEMORY") {
		t.Fatalf("bellek yetmezliği mesajı: %v", err)
	}
	// Cihaz belleğinden büyük tek ayırma, ayırmadan önce reddedilir.
	place := tensor.PlaceGPU
	err = panicError(func() { g.Alloc(1<<20, tensor.Float32) })
	if !messages.Has(err, messages.H3083) || limitPlace(err) != place {
		t.Fatalf("cihaz sınırı mesajı: %v", err)
	}
	// Cihazdan bağımsız katman şekli GPU belleğiyle denetler (ayırmadan önce). Satır sayısı
	// GPU belleğini az farkla aşar; konağın belleği GPU'nunkinden küçükse önce konak sınırı
	// devreye girer (gerçek GPU'da), o da aynı biçimde Türkçe ve yakalanabilirdir.
	row := int(gpuBackend().MemoryLimit()/16) + 1
	err = panicError(func() { tensor.ZerosOn(gpuDevice, row, 4) })
	me, _ := messages.As(err)
	if !messages.Has(err, messages.H3084) || me.Args[1] != fmt.Sprintf("[%d, 4]", row) ||
		!(limitPlace(err) == place || limitPlace(err) == tensor.PlaceHost) {
		t.Fatalf("tensör katmanının GPU sınırı mesajı: %v", err)
	}
	if inSim() && limitPlace(err) != place {
		t.Fatalf("benzetimde GPU sınırı devreye girmeliydi: %v", err)
	}
}

// limitPlace bellek sınırı iletisinin yer parametresidir (H3083, H3084...: dördüncü parametre).
func limitPlace(err error) string {
	m, ok := messages.As(err)
	if !ok || len(m.Args) < 4 {
		return ""
	}
	p, _ := m.Args[3].(string)
	return p
}

func TestUnsupported(t *testing.T) {
	g := gpuBackend()
	if err := panicError(func() { g.Alloc(4, tensor.Int8) }); !messages.Has(err, messages.H4050) {
		t.Fatalf("int8 mesajı: %v", err)
	}
	a := tensor.NewOn(tensor.CPU, []float32{1, 2}, 2)
	b := tensor.NewOn(gpuDevice, []float32{1, 2}, 2)
	if err := panicError(func() { tensor.Add(a, b) }); !messages.Has(err, messages.H3072) {
		t.Fatalf("farklı cihaz mesajı: %v", err)
	}
}

// TestNoDriverGraceful NVIDIA sürücüsü olmayan makinede (Linux'ta dlopen, Windows'ta
// DLL yükleme hatası) paketin paniklemeden "GPU yok" dediğini ve tensörlerin CPU'da
// kaldığını denetler. Gerçek GPU'lu makinede atlanır.
func TestNoDriverGraceful(t *testing.T) {
	if !inSim() {
		t.Skip("gerçek GPU var")
	}
	if n := deviceCount(); n != 0 {
		t.Fatalf("sürücü yokken cihaz sayısı %d", n)
	}
	if !strings.HasPrefix(Status(), "GPU yok: ") {
		t.Fatalf("durum: %q", Status())
	}
	if tensor.HasDevice(tensor.GPU(1)) {
		t.Fatal("sürücü yokken gpu:1 görünüyor")
	}
	if d := tensor.NewOn(tensor.CPU, []float32{1}, 1).Device(); d != tensor.CPU {
		t.Fatalf("tensör CPU'da değil: %s", d)
	}
}

// fromDevice depolamanın içeriğini gölgeye bakmadan cihazdan okur.
func fromDevice(g *backend, s tensor.Storage) []float32 {
	g.enter()
	defer g.exit()
	return lockedDownload(g, g.storageOf(s))
}

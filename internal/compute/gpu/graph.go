package gpu

import (
	"sync"

	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Kayıt ve oynatma (CUDA Graph)
// ============================================================================
//
// Bir iş dizisi akışta kaydedilir (cuStreamBeginCapture, RELAXED kip), çalıştırılabilir
// çizgeye kurulur (cuGraphInstantiateWithFlags) ve her oynatmada tek çağrıyla sıraya konur
// (cuGraphLaunch): kernel başlatmalarının konak yükü kalkar. Kayıt işi çalıştırmaz; oynatma
// her seferinde aynı kernelleri aynı parametrelerle ve aynı adreslerle yürütür (PyTorch
// torch.cuda.graph ile aynı anlam: https://docs.pytorch.org/docs/2.14/notes/cuda.html).
//
// Kurallar:
//   - Kayıt sırasında ayrılan bellek kayda özel havuzdan verilir: kayıt içinde bırakılan
//     blok kayıt içinde yeniden kullanılabilir (sıra her oynatmada aynıdır), ama kayıt
//     kapanana kadar kayıt dışındaki işlere verilmez (oynatma onlara yazardı). Kayıt
//     kapanınca boştaki bloklar genel havuza döner, kayıttan sonra yaşayan tensörlerin
//     blokları da öldüklerinde genel havuza döner. Kaydın havuzunda blok yoksa genel havuzdan
//     ya da sürücüden alınır (RELAXED kipte ayırmaya izin vardır).
//   - Kayıt sırasında cihazdan okuma (H4065), konaktan yükleme (H4066) ve eşzamanlama
//     (H4067) hatadır: kaydedilen iş henüz çalışmamıştır, yüklenen veri her oynatmada aynı
//     kalırdı. Bellek yetmeyip havuz sürücüye iade edilmek istenirse de eşzamanlama gerektiği
//     için H4067 olur. Profil olayları kayda konmaz.
//   - Aynı anda tek kayıt (H4068). Sürücü kaydı desteklemiyorsa (işlevler yok ya da akış
//     oluşturulamadı) CaptureSupported yanlış döner; çağıran işi doğrudan çalıştırır.

// privatePool bir kaydın bellek havuzu: kayıt içinde bırakılan bloklar istek boyuna göre
// tutulur (küçük bloklarda sınıf, büyüklerde largeSize).
type privatePool struct {
	mu     sync.Mutex
	h      *pool
	free   map[uint64][]privateBlock
	closed bool
}

type privateBlock struct{ addr, class uint64 }

func (p *privatePool) take(want uint64) (uint64, uint64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := p.free[want]
	if len(l) == 0 {
		return 0, 0, false
	}
	b := l[len(l)-1]
	p.free[want] = l[:len(l)-1]
	return b.addr, b.class, true
}

// give bloğu kaydın havuzuna (kayıt kapandıysa genel havuza) koyar.
func (p *privatePool) give(want, addr, class uint64) {
	p.mu.Lock()
	if !p.closed {
		p.free[want] = append(p.free[want], privateBlock{addr, class})
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	p.h.release2(addr, class)
}

// close boştaki blokları genel havuza döndürür; sonra bırakılanlar doğrudan oraya gider.
func (p *privatePool) close() {
	p.mu.Lock()
	free := p.free
	p.free, p.closed = nil, true
	p.mu.Unlock()
	for _, l := range free {
		for _, b := range l {
			p.h.release2(b.addr, b.class)
		}
	}
}

// Graph kaydedilmiş, oynatılabilir bir iş dizisi.
type Graph struct {
	g           *backend
	graph, exec uintptr
	priv        *privatePool
}

// CaptureSupported sürücünün kaydı destekleyip desteklemediği.
func (g *backend) CaptureSupported() bool { return g.sr.captureSupported() }

// BeginCapture akıştaki işleri kaydetmeye başlar (sıradaki işler önce biter).
func (g *backend) BeginCapture() {
	g.enter()
	defer g.exit()
	if g.capturing != nil {
		panic(messages.New(messages.H4068))
	}
	g.equalize()
	g.try_("cuStreamBeginCapture", g.sr.beginCapture())
	g.capturing = &privatePool{h: &g.pool, free: map[uint64][]privateBlock{}}
}

// EndCapture kaydı bitirir ve oynatılabilir çizgeyi kurar. Kayıt bir sürücü kuralıyla
// bozulduysa (çizge yok) H4069 hatası verir; havuzu kapatılır.
func (g *backend) EndCapture() *Graph {
	g.enter()
	defer g.exit()
	priv := g.capturing
	if priv == nil {
		panic(internalf("EndCapture without BeginCapture"))
	}
	g.capturing = nil
	graph, s := g.sr.endCapture()
	if s != succeeded || graph == 0 {
		priv.close()
		panic(messages.New(messages.H4069, g.sr.errName(s)))
	}
	exec, s := g.sr.instantiate(graph)
	if s != succeeded {
		g.sr.destroyGraph(graph, 0)
		priv.close()
		panic(messages.New(messages.H4069, g.sr.errName(s)))
	}
	return &Graph{g: g, graph: graph, exec: exec, priv: priv}
}

// AbortCapture süren kaydı (varsa) bırakır: akış eski hâline döner, havuz kapanır. Kayıt
// sırasında hata çıkınca çağrılır.
func (g *backend) AbortCapture() {
	g.enter()
	defer g.exit()
	priv := g.capturing
	if priv == nil {
		return
	}
	g.capturing = nil
	if graph, s := g.sr.endCapture(); s == succeeded && graph != 0 {
		g.sr.destroyGraph(graph, 0)
	}
	priv.close()
}

// Replay kaydı sıraya koyar (beklemez).
func (c *Graph) Replay() {
	g := c.g
	g.enter()
	defer g.exit()
	if c.exec == 0 {
		panic(internalf("replay of a released graph"))
	}
	if g.capturing != nil {
		panic(messages.New(messages.H4068))
	}
	ev := g.measureStart()
	g.try_("cuGraphLaunch", g.sr.launchGraph(c.exec))
	g.measureEnd("kayit_oynat", ev)
	if g.concurrent && g.capturing == nil {
		g.equalize()
	}
}

// Release çizgeyi yok eder ve havuzunu genel havuza döndürür (sıradaki işler önce biter).
func (c *Graph) Release() {
	g := c.g
	g.enter()
	defer g.exit()
	if c.exec == 0 {
		return
	}
	g.equalize()
	g.sr.destroyGraph(c.graph, c.exec)
	c.graph, c.exec = 0, 0
	c.priv.close()
}

// noCapture kayıt sürerken yapılamayan bir işlemde iletisiyle hata verir.
func (g *backend) noCapture(code messages.Code) {
	if g.capturing != nil {
		panic(messages.New(code))
	}
}

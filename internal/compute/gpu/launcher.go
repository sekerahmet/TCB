package gpu

import (
	"encoding/binary"
	"math"
	"os"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// ============================================================================
// Çekirdek önbelleği ve başlatma
// ============================================================================

// kernel derlenmiş bir PTX giriş noktası.
type kernel struct {
	ad        string
	fn        uintptr
	signature []paramType
	// sharedMem devingen paylaşımlı bellek (bayt; 0: statik diziler).
	sharedMem uint32
}

// takeKernel anahtarın çekirdeğini verir; önbellekte yoksa üretir ve sürücüye derletir.
// Anahtar, üreteci belirleyen bütün ayarları (tür, eksen sayısı, blok boyu) içermelidir.
func (g *backend) takeKernel(key string, generate func() *emitter) *kernel {
	if c, ok := g.kernels[key]; ok {
		return c
	}
	k := generate()
	text := k.text()
	mod, log, s := g.sr.module(text)
	if s != succeeded {
		panic(g.compileError(k.ad, text, log, s))
	}
	fn, s := g.sr.fn(mod, k.ad)
	g.try_("cuModuleGetFunction("+k.ad+")", s)
	c := &kernel{ad: k.ad, fn: fn, signature: k.signature}
	if k.dynamic {
		c.sharedMem = uint32(k.sharedMemBytes)
		g.try_("cuFuncSetAttribute("+k.ad+")", g.sr.sharedMemOptIn(fn, k.sharedMemBytes))
	}
	g.kernels[key] = c
	return c
}

// ptxDumpDir derlenemeyen PTX metninin teşhis için yazıldığı klasördür ("": işletim
// sisteminin geçici klasörü). Sınamalar kendi geçici klasörlerine yönlendirir; yoksa her koşu
// geçici klasörde bir dosya bırakırdı.
var ptxDumpDir = ""

// compileError PTX derleme hatasını, derleyici günlüğünü ve PTX'in yazıldığı geçici
// dosyanın yolunu içeren bir hataya çevirir.
func (g *backend) compileError(ad, text, log string, s result) error {
	var err error = messages.New(messages.H4011, ad, g.sr.errName(s), int32(s))
	if log != "" {
		err = messages.New(messages.H4012, err, log)
	}
	if f, ferr := os.CreateTemp(ptxDumpDir, "tr_gpu_"+ad+"_*.ptx"); ferr == nil {
		_, _ = f.WriteString(text)
		_ = f.Close()
		err = messages.New(messages.H4013, err, f.Name())
	}
	return err
}

// Parametre çeşitleri (başlatma yerinin bildirdiği).
const (
	pValue = iota // sayı ya da dizi
	pRead         // çekirdeğin okuduğu depolama
	pWrite        // çekirdeğin yazdığı depolama
)

// paramWriter bir başlatmanın parametrelerini sırayla biriktirir (yeniden kullanılır).
// Göstericiler yalnızca depolamayla verilir (oku/yaz); ham adres verilemez. Böylece
// başlatıcı yazılan her depolamayı bilir: konak gölgesini siler ve bildirimi çekirdeğin
// imzasıyla karşılaştırır (yazan çekirdeğe "oku" ile verilen depolama iç hatadır).
type paramWriter struct {
	buf     []byte
	limit   []int
	variant []int
	writer  []*storage
	partOf  [][]byte
	// open, param() ile başlayıp baslat ile biten bir zincirin sürdüğünü gösterir; zincir
	// açıkken yeni bir param() (ör. zincirin ortasında kopya çekirdeği başlatan bir
	// gecici.oku) dış zincirin parametrelerini silerdi: iç hata.
	open bool
	// capture süren kayıt (yoksa nil): göstericisi verilen depolama kayda girer (graph.go).
	capture *privatePool
}

func (p *paramWriter) reset() *paramWriter {
	p.buf, p.limit, p.variant, p.writer = p.buf[:0], p.limit[:0], p.variant[:0], p.writer[:0]
	return p
}

func (p *paramWriter) addVariant(b []byte, c int) *paramWriter {
	p.buf = append(p.buf, b...)
	p.limit = append(p.limit, len(p.buf))
	p.variant = append(p.variant, c)
	return p
}

func (p *paramWriter) add(b []byte) *paramWriter { return p.addVariant(b, pValue) }

func (p *paramWriter) indicator(d *storage, offset int, c int) *paramWriter {
	if p.capture != nil {
		p.capture.hold(d)
	}
	addr := d.addr + uint64(offset*dtypeSize(d.dtype))
	if c == pWrite {
		p.writer = append(p.writer, d)
	}
	var b [8]byte // yığıtta: ekleCesit kopyalar (başlatma başına ayırma olmasın)
	binary.LittleEndian.PutUint64(b[:], addr)
	return p.addVariant(b[:], c)
}

// read çekirdeğin yalnızca okuduğu depolama (ofset eleman cinsinden).
func (p *paramWriter) read(d *storage, offset int) *paramWriter { return p.indicator(d, offset, pRead) }

// write çekirdeğin yazdığı depolama.
func (p *paramWriter) write(d *storage, offset int) *paramWriter {
	return p.indicator(d, offset, pWrite)
}

// readV ve yazV görünümün ilk elemanından başlayan gösterici.
func (p *paramWriter) readV(v tensor.View) *paramWriter {
	return p.read(v.S.(*storage), v.Offset)
}

func (p *paramWriter) writeV(v tensor.View) *paramWriter {
	return p.write(v.S.(*storage), v.Offset)
}

// empty, çekirdeğin bu başlatmada kullanmadığı bir okuma göstericisi (0 adres); bosYaz
// aynısının yazılan parametreler için olanı (çekirdek o yola hiç girmez).
func (p *paramWriter) empty() *paramWriter {
	var b [8]byte
	return p.addVariant(b[:], pRead)
}

func (p *paramWriter) writeEmpty() *paramWriter {
	var b [8]byte
	return p.addVariant(b[:], pWrite)
}

// readFrom d nil değilse okuma göstericisi, nil ise boş gösterici ekler; yazYa aynısının
// yazılan parametreler için olanı.
func (p *paramWriter) readFrom(d *storage) *paramWriter {
	if d == nil {
		return p.empty()
	}
	return p.read(d, 0)
}

// readMask dikkat maskesinin göstericisi: görünümün başlangıcından (MaskBase); maske yoksa boş.
func (p *paramWriter) readMask(d *storage, a tensor.AttentionParams) *paramWriter {
	if d == nil {
		return p.empty()
	}
	return p.read(d, a.MaskBase)
}

func (p *paramWriter) writeTo(d *storage) *paramWriter {
	if d == nil {
		return p.writeEmpty()
	}
	return p.write(d, 0)
}

func (p *paramWriter) u32(v int) *paramWriter {
	if v < 0 || v > math.MaxUint32 {
		panic(internalf("parameter %d does not fit in 32 bits", v))
	}
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	return p.add(b[:])
}

func (p *paramWriter) f32(v float32) *paramWriter {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	return p.add(b[:])
}

// arr n elemanlık 32 bitlik tam sayı dizisi (eksik elemanlar 0).
func (p *paramWriter) arr(n int, v ...int) *paramWriter {
	begin := len(p.buf)
	for i := 0; i < 4*max(n, 1); i++ {
		p.buf = append(p.buf, 0)
	}
	for i, x := range v {
		if x < math.MinInt32 || x > math.MaxUint32 {
			panic(internalf("array parameter %d does not fit in 32 bits", x))
		}
		binary.LittleEndian.PutUint32(p.buf[begin+4*i:], uint32(x))
	}
	p.limit = append(p.limit, len(p.buf))
	p.variant = append(p.variant, pValue)
	return p
}

func (p *paramWriter) parts() [][]byte {
	p.partOf = p.partOf[:0]
	begin := 0
	for _, s := range p.limit {
		p.partOf = append(p.partOf, p.buf[begin:s])
		begin = s
	}
	return p.partOf
}

// param yeni bir başlatmanın parametre yazıcısını verir.
func (g *backend) param() *paramWriter {
	if g.prm.open {
		panic(internalf("new launch prepared while a parameter chain is open (nested param())"))
	}
	g.prm.open = true
	g.prm.capture = g.capturing
	return g.prm.reset()
}

// launch çekirdeği verilen ızgara ve blokla sıraya ekler. Parametreler imzayla
// denetlenir (sayı ve bayt boyu).
func (g *backend) launch(c *kernel, grid, block [3]uint32, p *paramWriter) {
	p.open = false
	part := p.parts()
	if len(part) != len(c.signature) {
		panic(internalf("kernel %s given %d parameters, expected %d", c.ad, len(part), len(c.signature)))
	}
	for i, t := range c.signature {
		if len(part[i]) != t.byteN {
			panic(internalf("parameter %s of kernel %s must be %d bytes, got %d", t.ad, c.ad, t.byteN, len(part[i])))
		}
		variant := p.variant[i]
		if t.pointer != (variant != pValue) || (t.writable && variant != pWrite) || (!t.writable && variant == pWrite) {
			panic(internalf("read/write declaration of parameter %s of kernel %s does not match the signature", t.ad, c.ad))
		}
	}
	for i := 0; i < 3; i++ {
		if grid[i] == 0 {
			return // boş iş
		}
	}
	for _, d := range p.writer {
		d.host = nil
	}
	g.lastKernel = c.ad
	ev := g.measureStart()
	if s := g.sr.launch(c.fn, grid, block, c.sharedMem, part); s != succeeded {
		g.try_("cuLaunchKernel("+c.ad+")", s) // metin yalnızca hatada kurulur
	}
	g.measureEnd(c.ad, ev)
	if g.concurrent && g.capturing == nil {
		g.equalize()
	}
}

// blocks n işi b'lik bloklara bölen ızgara boyu.
func blocks(n, b int) uint32 { return uint32((n + b - 1) / b) }

// plain bir boyutlu ızgara/blok.
func plain(n, b int) ([3]uint32, [3]uint32) {
	return [3]uint32{blocks(n, b), 1, 1}, [3]uint32{uint32(b), 1, 1}
}

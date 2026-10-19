package gpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
)

// ============================================================================
// Çekirdek önbelleği ve başlatma
// ============================================================================

// cekirdek derlenmiş bir PTX giriş noktası.
type kernel struct {
	ad        string
	fn        uintptr
	signature []paramType
	// paylasim devingen paylaşımlı bellek (bayt; 0: statik diziler).
	sharedMem uint32
}

// cekirdekAl anahtarın çekirdeğini verir; önbellekte yoksa üretir ve sürücüye derletir.
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

// ptxDokumKlasoru derlenemeyen PTX metninin teşhis için yazıldığı klasördür ("": işletim
// sisteminin geçici klasörü). Sınamalar kendi geçici klasörlerine yönlendirir; yoksa her koşu
// geçici klasörde bir dosya bırakırdı.
var ptxDumpDir = ""

// derlemeHatasi PTX derleme hatasını, derleyici günlüğünü ve PTX'in yazıldığı geçici
// dosyanın yolunu içeren bir mesaja çevirir.
func (g *backend) compileError(ad, text, log string, s result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "tensor: GPU: %s çekirdeği derlenemedi: %s (%d)", ad, g.sr.errName(s), int32(s))
	if log != "" {
		sb.WriteString("\nPTX derleyici günlüğü:\n")
		sb.WriteString(log)
	}
	if f, err := os.CreateTemp(ptxDumpDir, "tr_gpu_"+ad+"_*.ptx"); err == nil {
		_, _ = f.WriteString(text)
		_ = f.Close()
		sb.WriteString("\nPTX metni: " + f.Name())
	}
	return sb.String()
}

// Parametre çeşitleri (başlatma yerinin bildirdiği).
const (
	pValue = iota // sayı ya da dizi
	pRead         // çekirdeğin okuduğu depolama
	pWrite        // çekirdeğin yazdığı depolama
)

// paramYazici bir başlatmanın parametrelerini sırayla biriktirir (yeniden kullanılır).
// Göstericiler yalnızca depolamayla verilir (oku/yaz); ham adres verilemez. Böylece
// başlatıcı yazılan her depolamayı bilir: konak gölgesini siler ve bildirimi çekirdeğin
// imzasıyla karşılaştırır (yazan çekirdeğe "oku" ile verilen depolama iç hatadır).
type paramWriter struct {
	buf     []byte
	limit   []int
	variant []int
	writer  []*storage
	partOf  [][]byte
	// acik, param() ile başlayıp baslat ile biten bir zincirin sürdüğünü gösterir; zincir
	// açıkken yeni bir param() (ör. zincirin ortasında kopya çekirdeği başlatan bir
	// gecici.oku) dış zincirin parametrelerini silerdi: iç hata.
	open bool
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
	addr := d.addr + uint64(offset*dtypeSize(d.dtype))
	if c == pWrite {
		p.writer = append(p.writer, d)
	}
	var b [8]byte // yığıtta: ekleCesit kopyalar (başlatma başına ayırma olmasın)
	binary.LittleEndian.PutUint64(b[:], addr)
	return p.addVariant(b[:], c)
}

// oku çekirdeğin yalnızca okuduğu depolama (ofset eleman cinsinden).
func (p *paramWriter) read(d *storage, offset int) *paramWriter { return p.indicator(d, offset, pRead) }

// yaz çekirdeğin yazdığı depolama.
func (p *paramWriter) write(d *storage, offset int) *paramWriter {
	return p.indicator(d, offset, pWrite)
}

// okuV ve yazV görünümün ilk elemanından başlayan gösterici.
func (p *paramWriter) readV(v tensor.View) *paramWriter {
	return p.read(v.S.(*storage), v.Offset)
}

func (p *paramWriter) writeV(v tensor.View) *paramWriter {
	return p.write(v.S.(*storage), v.Offset)
}

// bos, çekirdeğin bu başlatmada kullanmadığı bir okuma göstericisi (0 adres); bosYaz
// aynısının yazılan parametreler için olanı (çekirdek o yola hiç girmez).
func (p *paramWriter) empty() *paramWriter {
	var b [8]byte
	return p.addVariant(b[:], pRead)
}

func (p *paramWriter) writeEmpty() *paramWriter {
	var b [8]byte
	return p.addVariant(b[:], pWrite)
}

// okuYa d nil değilse okuma göstericisi, nil ise boş gösterici ekler; yazYa aynısının
// yazılan parametreler için olanı.
func (p *paramWriter) readFrom(d *storage) *paramWriter {
	if d == nil {
		return p.empty()
	}
	return p.read(d, 0)
}

func (p *paramWriter) writeTo(d *storage) *paramWriter {
	if d == nil {
		return p.writeEmpty()
	}
	return p.write(d, 0)
}

func (p *paramWriter) u32(v int) *paramWriter {
	if v < 0 || v > math.MaxUint32 {
		panic(fmt.Sprintf("tensor: GPU: parametre %d 32 bite sığmıyor (iç hata)", v))
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

// dizi n elemanlık 32 bitlik tam sayı dizisi (eksik elemanlar 0).
func (p *paramWriter) arr(n int, v ...int) *paramWriter {
	begin := len(p.buf)
	for i := 0; i < 4*max(n, 1); i++ {
		p.buf = append(p.buf, 0)
	}
	for i, x := range v {
		if x < math.MinInt32 || x > math.MaxUint32 {
			panic(fmt.Sprintf("tensor: GPU: dizi parametresi %d 32 bite sığmıyor (iç hata)", x))
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
		panic("tensor: GPU: parametre zinciri açıkken yeni başlatma hazırlandı (iç içe param(); iç hata)")
	}
	g.prm.open = true
	return g.prm.reset()
}

// baslat çekirdeği verilen ızgara ve blokla sıraya ekler. Parametreler imzayla
// denetlenir (sayı ve bayt boyu).
func (g *backend) launch(c *kernel, grid, block [3]uint32, p *paramWriter) {
	p.open = false
	part := p.parts()
	if len(part) != len(c.signature) {
		panic(fmt.Sprintf("tensor: GPU: %s çekirdeğine %d parametre verildi, %d bekleniyordu (iç hata)", c.ad, len(part), len(c.signature)))
	}
	for i, t := range c.signature {
		if len(part[i]) != t.byteN {
			panic(fmt.Sprintf("tensor: GPU: %s çekirdeğinin %s parametresi %d bayt olmalı, %d verildi (iç hata)", c.ad, t.ad, t.byteN, len(part[i])))
		}
		variant := p.variant[i]
		if t.pointer != (variant != pValue) || (t.writable && variant != pWrite) || (!t.writable && variant == pWrite) {
			panic(fmt.Sprintf("tensor: GPU: %s çekirdeğinin %s parametresi için okuma/yazma bildirimi imzayla uyuşmuyor (iç hata)", c.ad, t.ad))
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
	if g.concurrent {
		g.equalize()
	}
}

// bloklar n işi b'lik bloklara bölen ızgara boyu.
func blocks(n, b int) uint32 { return uint32((n + b - 1) / b) }

// duz bir boyutlu ızgara/blok.
func plain(n, b int) ([3]uint32, [3]uint32) {
	return [3]uint32{blocks(n, b), 1, 1}, [3]uint32{uint32(b), 1, 1}
}

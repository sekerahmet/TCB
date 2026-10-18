package gpu

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"

	"tr_dili/internal/hesap/tensor"
)

// ============================================================================
// Çekirdek önbelleği ve başlatma
// ============================================================================

// cekirdek derlenmiş bir PTX giriş noktası.
type cekirdek struct {
	ad   string
	fn   uintptr
	imza []paramTuru
}

// cekirdekAl anahtarın çekirdeğini verir; önbellekte yoksa üretir ve sürücüye derletir.
// Anahtar, üreteci belirleyen bütün ayarları (tür, eksen sayısı, blok boyu) içermelidir.
func (g *arkaUc) cekirdekAl(anahtar string, uret func() *yazici) *cekirdek {
	if c, ok := g.cekirdekler[anahtar]; ok {
		return c
	}
	k := uret()
	metin := k.metin()
	mod, gunluk, s := g.sr.modul(metin)
	if s != basarili {
		panic(g.derlemeHatasi(k.ad, metin, gunluk, s))
	}
	fn, s := g.sr.islev(mod, k.ad)
	g.dene("cuModuleGetFunction("+k.ad+")", s)
	c := &cekirdek{ad: k.ad, fn: fn, imza: k.imza}
	g.cekirdekler[anahtar] = c
	return c
}

// derlemeHatasi PTX derleme hatasını, derleyici günlüğünü ve PTX'in yazıldığı geçici
// dosyanın yolunu içeren bir mesaja çevirir.
func (g *arkaUc) derlemeHatasi(ad, metin, gunluk string, s sonuc) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "tensor: GPU: %s çekirdeği derlenemedi: %s (%d)", ad, g.sr.hataAdi(s), int32(s))
	if gunluk != "" {
		sb.WriteString("\nPTX derleyici günlüğü:\n")
		sb.WriteString(gunluk)
	}
	if f, err := os.CreateTemp("", "tr_gpu_"+ad+"_*.ptx"); err == nil {
		_, _ = f.WriteString(metin)
		_ = f.Close()
		sb.WriteString("\nPTX metni: " + f.Name())
	}
	return sb.String()
}

// Parametre çeşitleri (başlatma yerinin bildirdiği).
const (
	pDeger = iota // sayı ya da dizi
	pOku          // çekirdeğin okuduğu depolama
	pYaz          // çekirdeğin yazdığı depolama
)

// paramYazici bir başlatmanın parametrelerini sırayla biriktirir (yeniden kullanılır).
// Göstericiler yalnızca depolamayla verilir (oku/yaz); ham adres verilemez. Böylece
// başlatıcı yazılan her depolamayı bilir: konak gölgesini siler ve bildirimi çekirdeğin
// imzasıyla karşılaştırır (yazan çekirdeğe "oku" ile verilen depolama iç hatadır).
type paramYazici struct {
	buf    []byte
	sinir  []int
	cesit  []int
	yazan  []*depo
	parcal [][]byte
	// acik, param() ile başlayıp baslat ile biten bir zincirin sürdüğünü gösterir; zincir
	// açıkken yeni bir param() (ör. zincirin ortasında kopya çekirdeği başlatan bir
	// gecici.oku) dış zincirin parametrelerini silerdi: iç hata.
	acik bool
}

func (p *paramYazici) sifirla() *paramYazici {
	p.buf, p.sinir, p.cesit, p.yazan = p.buf[:0], p.sinir[:0], p.cesit[:0], p.yazan[:0]
	return p
}

func (p *paramYazici) ekleCesit(b []byte, c int) *paramYazici {
	p.buf = append(p.buf, b...)
	p.sinir = append(p.sinir, len(p.buf))
	p.cesit = append(p.cesit, c)
	return p
}

func (p *paramYazici) ekle(b []byte) *paramYazici { return p.ekleCesit(b, pDeger) }

func (p *paramYazici) gosterge(d *depo, ofset int, c int) *paramYazici {
	adr := d.adr + uint64(ofset*turBoyu(d.tur))
	if c == pYaz {
		p.yazan = append(p.yazan, d)
	}
	return p.ekleCesit(binary.LittleEndian.AppendUint64(nil, adr), c)
}

// oku çekirdeğin yalnızca okuduğu depolama (ofset eleman cinsinden).
func (p *paramYazici) oku(d *depo, ofset int) *paramYazici { return p.gosterge(d, ofset, pOku) }

// yaz çekirdeğin yazdığı depolama.
func (p *paramYazici) yaz(d *depo, ofset int) *paramYazici { return p.gosterge(d, ofset, pYaz) }

// okuV ve yazV görünümün ilk elemanından başlayan gösterici.
func (p *paramYazici) okuV(v tensor.View) *paramYazici {
	return p.oku(v.S.(*depo), v.Offset)
}

func (p *paramYazici) yazV(v tensor.View) *paramYazici {
	return p.yaz(v.S.(*depo), v.Offset)
}

// bos, çekirdeğin bu başlatmada kullanmadığı bir okuma göstericisi (0 adres); bosYaz
// aynısının yazılan parametreler için olanı (çekirdek o yola hiç girmez).
func (p *paramYazici) bos() *paramYazici {
	return p.ekleCesit(make([]byte, 8), pOku)
}

func (p *paramYazici) bosYaz() *paramYazici {
	return p.ekleCesit(make([]byte, 8), pYaz)
}

// okuYa d nil değilse okuma göstericisi, nil ise boş gösterici ekler; yazYa aynısının
// yazılan parametreler için olanı.
func (p *paramYazici) okuYa(d *depo) *paramYazici {
	if d == nil {
		return p.bos()
	}
	return p.oku(d, 0)
}

func (p *paramYazici) yazYa(d *depo) *paramYazici {
	if d == nil {
		return p.bosYaz()
	}
	return p.yaz(d, 0)
}

func (p *paramYazici) u32(v int) *paramYazici {
	if v < 0 || v > math.MaxUint32 {
		panic(fmt.Sprintf("tensor: GPU: parametre %d 32 bite sığmıyor (iç hata)", v))
	}
	return p.ekle(binary.LittleEndian.AppendUint32(nil, uint32(v)))
}

func (p *paramYazici) f32(v float32) *paramYazici {
	return p.ekle(binary.LittleEndian.AppendUint32(nil, math.Float32bits(v)))
}

// dizi n elemanlık 32 bitlik tam sayı dizisi (eksik elemanlar 0).
func (p *paramYazici) dizi(n int, v ...int) *paramYazici {
	b := make([]byte, 4*max(n, 1))
	for i, x := range v {
		if x < math.MinInt32 || x > math.MaxUint32 {
			panic(fmt.Sprintf("tensor: GPU: dizi parametresi %d 32 bite sığmıyor (iç hata)", x))
		}
		binary.LittleEndian.PutUint32(b[4*i:], uint32(x))
	}
	return p.ekle(b)
}

func (p *paramYazici) parcalar() [][]byte {
	p.parcal = p.parcal[:0]
	bas := 0
	for _, s := range p.sinir {
		p.parcal = append(p.parcal, p.buf[bas:s])
		bas = s
	}
	return p.parcal
}

// param yeni bir başlatmanın parametre yazıcısını verir.
func (g *arkaUc) param() *paramYazici {
	if g.prm.acik {
		panic("tensor: GPU: parametre zinciri açıkken yeni başlatma hazırlandı (iç içe param(); iç hata)")
	}
	g.prm.acik = true
	return g.prm.sifirla()
}

// baslat çekirdeği verilen ızgara ve blokla sıraya ekler. Parametreler imzayla
// denetlenir (sayı ve bayt boyu).
func (g *arkaUc) baslat(c *cekirdek, izgara, blok [3]uint32, p *paramYazici) {
	p.acik = false
	parca := p.parcalar()
	if len(parca) != len(c.imza) {
		panic(fmt.Sprintf("tensor: GPU: %s çekirdeğine %d parametre verildi, %d bekleniyordu (iç hata)", c.ad, len(parca), len(c.imza)))
	}
	for i, t := range c.imza {
		if len(parca[i]) != t.bayt {
			panic(fmt.Sprintf("tensor: GPU: %s çekirdeğinin %s parametresi %d bayt olmalı, %d verildi (iç hata)", c.ad, t.ad, t.bayt, len(parca[i])))
		}
		cesit := p.cesit[i]
		if t.gosterici != (cesit != pDeger) || (t.yazilir && cesit != pYaz) || (!t.yazilir && cesit == pYaz) {
			panic(fmt.Sprintf("tensor: GPU: %s çekirdeğinin %s parametresi için okuma/yazma bildirimi imzayla uyuşmuyor (iç hata)", c.ad, t.ad))
		}
	}
	for i := 0; i < 3; i++ {
		if izgara[i] == 0 {
			return // boş iş
		}
	}
	for _, d := range p.yazan {
		d.konak = nil
	}
	g.sonCekirdek = c.ad
	g.dene("cuLaunchKernel("+c.ad+")", g.sr.baslat(c.fn, izgara, blok, parca))
	if g.eszamanli {
		g.esitle()
	}
}

// bloklar n işi b'lik bloklara bölen ızgara boyu.
func bloklar(n, b int) uint32 { return uint32((n + b - 1) / b) }

// duz bir boyutlu ızgara/blok.
func duz(n, b int) ([3]uint32, [3]uint32) {
	return [3]uint32{bloklar(n, b), 1, 1}, [3]uint32{uint32(b), 1, 1}
}

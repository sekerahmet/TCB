package gunluk

import (
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/ortak/baglama"
)

// Yenileme sıklıkları: terminalde çubuk en çok saniyede 10 kez çizilir; terminal değilse
// (dosya, boru, not defteri hücresi) en çok 10 saniyede bir düz satır yazılır.
const (
	terminalAraligi = 100 * time.Millisecond
	duzSatirAraligi = 10 * time.Second
	// hizPenceresi adım hızının üstel ortalamasının zaman sabitidir (kalan süre bundan).
	hizPenceresi = 30.0 // saniye
	// hizOrnekAraligi hız ölçümleri arasındaki en kısa süredir (çok hızlı adımlarda
	// gürültüyü azaltır).
	hizOrnekAraligi = 0.5 // saniye
)

// cubukSirasi çubukların açılış sırasıdır (program bitince sonra açılan önce biter).
var cubukSirasi atomic.Int64

// olcu çubukta gösterilen ve günlüğe yazılan bir ölçüdür.
type olcu struct {
	ad    string
	deger yurut.Value
}

// Ilerleme TR'de açılmış bir ilerleme çubuğudur (tip İlerleme).
type Ilerleme struct {
	m        *yurut.VM
	d        *durum
	w        io.Writer
	terminal bool
	genislik int
	gunluk   *Gunluk
	isYolu   string // arka plan işinde ilerleme.json (yoksa boş)
	sira     int64

	mu        sync.Mutex
	toplam    int64 // 0: toplamsız sayaç
	adim      int64
	baslangic time.Time
	olculer   []olcu
	durdu     bool

	hiz      float64 // adım/sn (üstel ortalama; 0: henüz ölçülmedi)
	hizAdim  int64
	hizZaman time.Time

	sonCizim time.Time
	sonDuz   time.Time
	sonIs    time.Time
	satir    string // terminalde çizilen satır (çıktı kilidi altında okunur)
	kirli    bool   // satır son güncellemeden eski
	onceki   *Ilerleme
}

func (c *Ilerleme) Type() yurut.ValueType { return IlerlemeTipi }
func (c *Ilerleme) IsTruthy() bool        { return true }
func (c *Ilerleme) Inspect() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ek := ""
	if c.durdu {
		ek = ", bitti"
	}
	return fmt.Sprintf("%s(%s%s)", IlerlemeTipi, c.adimYazimi(), ek)
}

func (c *Ilerleme) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	switch kd {
	case kod.Y010:
		return &yurut.BuiltinVal{Name: kod.Ad(kod.Y010), Fn: func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
			return baglama.Guvenli(yurut.CagriAdi(kod.Y010), func() (yurut.Value, error) { return c.guncelle(m, args, named) })
		}}, nil
	case kod.Y011:
		return &yurut.BuiltinVal{Name: kod.Ad(kod.Y011), AdliAlmaz: true, Fn: func(m *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			if len(args) != 0 {
				return nil, yurut.ArgumanSayisiHatasi(kod.Y011, "0", len(args))
			}
			return yurut.BoolToVal(c.bitir(saat())), nil
		}}, nil
	}
	return nil, uyeHatasi(string(IlerlemeTipi), prop, kod.Y010, kod.Y011)
}

// ilerlemeAc ilerleme(toplam, günlük: g) yerleşiğidir.
func (d *durum) ilerlemeAc(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.G811)
	if err := baglama.AdliDenetle(fn, named, adGunluk); err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, yurut.ArgumanSayisiHatasi(kod.G811, "0 ya da 1", len(args), "", "toplam_adım")
	}
	var toplam int64
	if len(args) == 1 && args[0] != yurut.NULL {
		n, ok := baglama.TamSayi(args[0])
		if !ok {
			return nil, yurut.ArgumanTipHatasi(kod.G811, 1, args[0], kod.A200, kod.A204)
		}
		if n <= 0 {
			return nil, fmt.Errorf("%s: toplam adım sayısı pozitif olmalı, %d verildi (toplamsız bir sayaç için %s verin ya da argümansız çağırın)", fn, n, kod.Ad(kod.A042))
		}
		toplam = n
	}
	c := &Ilerleme{m: m, d: d, toplam: toplam, baslangic: saat(), sira: cubukSirasi.Add(1)}
	if v, ok := baglama.AdliAl(named, adGunluk); ok && v != yurut.NULL {
		g, ok := v.(*Gunluk)
		if !ok {
			return nil, fmt.Errorf("%s: %s: bir %s olmalı (%s(...) ile açılır), %s verildi", fn, adGunluk.TR, GunlukTipi, kod.Ad(kod.G810), yurut.TipVeOzet(v))
		}
		c.gunluk = g
		g.toplamAyarla(toplam)
	}
	c.cikisAyarla()
	c.sonDuz, c.hizZaman = c.baslangic, c.baslangic
	acikMu.Lock()
	acikIlerleme[c] = struct{}{}
	acikMu.Unlock()
	if c.terminal {
		c.onceki = d.etkin
		d.etkin = c
		m.CanliSatirAyarla(c)
	}
	return c, nil
}

// cikisAyarla çubuğun yazacağı yeri ve kipini seçer: standart hata (VM.Err) terminalse canlı
// satır, değilse düz satırlar. Arka plan işinde (TR_İŞ) her zaman düz satır ve ilerleme.json.
func (c *Ilerleme) cikisAyarla() {
	c.w = c.m.Err
	if c.w == nil {
		c.w = os.Stderr
	}
	c.isYolu = isDurumuYolu()
	if c.isYolu == "" {
		c.terminal, c.genislik = terminalSecimi(c.w)
	}
}

// terminalSecimi çıkışın terminal olup olmadığını ve genişliğini söyler (testler değiştirir).
var terminalSecimi = func(w io.Writer) (bool, int) {
	if f, ok := w.(*os.File); ok && terminalMi(f) {
		return true, terminalGenisligi(f)
	}
	return false, 0
}

// guncelle İlerleme.güncelle([ölçüler], adım: n) metodudur.
func (c *Ilerleme) guncelle(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.Y010)
	if err := baglama.AdliDenetle(fn, named, adAdim); err != nil {
		return nil, err
	}
	if len(args) > 1 {
		return nil, yurut.ArgumanSayisiHatasi(kod.Y010, "0 ya da 1", len(args), "", "{ölçü: değer, ...}")
	}
	var olculer []olcu
	if len(args) == 1 && args[0] != yurut.NULL {
		s, ok := args[0].(*yurut.DictVal)
		if !ok {
			return nil, yurut.ArgumanTipHatasi(kod.Y010, 1, args[0], kod.A206)
		}
		kayit := yurut.NewDictVal()
		if err := olculeriEkle(fn, kayit, s, aTur, aZaman, aAdim, aHiz); err != nil {
			return nil, err
		}
		for _, k := range kayit.Order {
			olculer = append(olculer, olcu{ad: k, deger: kayit.Pairs[k]})
		}
	}
	yeniAdim := int64(-1)
	if v, ok := baglama.AdliAl(named, adAdim); ok {
		n, ok := baglama.TamSayi(v)
		if !ok || n < 0 {
			return nil, fmt.Errorf("%s: %s: 0 ya da pozitif bir %s olmalı (tamamlanan adım sayısı), %s verildi", fn, adAdim.TR, kod.Ad(kod.A200), yurut.TipVeOzet(v))
		}
		yeniAdim = n
	}
	simdi := saat()
	bitti, err := c.ilerle(fn, simdi, yeniAdim, olculer)
	if err != nil {
		return nil, err
	}
	if k := adimKancasi.Load(); k != nil {
		(*k)(AdimSiniri{VM: m, Adim: c.adim, Toplam: c.toplam, Zaman: simdi})
	}
	if bitti {
		c.bitir(simdi)
	}
	return yurut.TRUE, nil
}

// ilerle sayacı ilerletir, hızı ölçer, günlüğe yazar ve çubuğu (sıklık sınırıyla) çizer.
// Toplama ulaşıldıysa bitti doğrudur.
func (c *Ilerleme) ilerle(fn string, simdi time.Time, yeniAdim int64, olculer []olcu) (bool, error) {
	c.mu.Lock()
	if c.durdu {
		c.mu.Unlock()
		return false, fmt.Errorf("%s: çubuk bitti (%s); devam etmek için yeni bir %s(...) açın", fn, c.adimYazimi(), kod.Ad(kod.G811))
	}
	switch {
	case yeniAdim < 0:
		c.adim++
	case c.toplam > 0 && yeniAdim > c.toplam:
		c.mu.Unlock()
		return false, fmt.Errorf("%s: %s: %d toplam adım sayısından (%d) büyük olamaz", fn, adAdim.TR, yeniAdim, c.toplam)
	case yeniAdim != c.adim+1:
		// Sıçrama (kaldığı yerden sürdürme): hız sıçramadan sonra yeniden ölçülür.
		c.adim, c.hiz, c.hizAdim, c.hizZaman = yeniAdim, 0, yeniAdim, simdi
	default:
		c.adim = yeniAdim
	}
	if olculer != nil {
		c.olculer = olculer
	}
	c.hizOlc(simdi)
	bitti := c.toplam > 0 && c.adim >= c.toplam
	kayit := c.gunlukKaydi(simdi)
	c.mu.Unlock()
	if kayit != nil {
		if err := c.gunluk.yaz(kayit); err != nil {
			return false, err
		}
	}
	if !bitti {
		c.goster(simdi)
	}
	return bitti, nil
}

// hizOlc adım hızının üstel ortalamasını günceller; ölçümler en az hizOrnekAraligi
// arayla alınır (ilk ölçümden önce hız bilinmez: 0). c.mu tutulur.
func (c *Ilerleme) hizOlc(simdi time.Time) {
	dt := simdi.Sub(c.hizZaman).Seconds()
	if dt < hizOrnekAraligi {
		return
	}
	anlik := float64(c.adim-c.hizAdim) / dt
	if c.hiz == 0 {
		c.hiz = anlik
	} else {
		a := 1 - math.Exp(-dt/hizPenceresi)
		c.hiz += a * (anlik - c.hiz)
	}
	c.hizAdim, c.hizZaman = c.adim, simdi
}

// gunlukKaydi bağlı günlüğe yazılacak ölçü kaydıdır (günlük yoksa nil). c.mu tutulur.
func (c *Ilerleme) gunlukKaydi(simdi time.Time) *yurut.DictVal {
	if c.gunluk == nil {
		return nil
	}
	k := yurut.NewDictVal()
	k.Set(aZaman, zamanDegeri(simdi))
	k.Set(aAdim, yurut.NewInt(c.adim))
	for _, o := range c.olculer {
		k.Set(o.ad, o.deger)
	}
	if h := c.hiz; h > 0 {
		k.Set(aHiz, &yurut.FloatVal{Val: yuvarla(h)})
	}
	return k
}

// bitir çubuğu bitirir: son satırı yazar, günlüğe bitiş kaydını ekler. İlk çağrıda doğru.
func (c *Ilerleme) bitir(simdi time.Time) bool {
	c.mu.Lock()
	if c.durdu {
		c.mu.Unlock()
		return false
	}
	c.durdu = true
	son := c.sonSatir(simdi)
	var kayit *yurut.DictVal
	if c.gunluk != nil {
		kayit = yurut.NewDictVal()
		kayit.Set(aTur, &yurut.StringVal{Val: turBitis})
		kayit.Set(aZaman, zamanDegeri(simdi))
		kayit.Set(aAdim, yurut.NewInt(c.adim))
		kayit.Set(aSure, &yurut.FloatVal{Val: yuvarla(simdi.Sub(c.baslangic).Seconds())})
	}
	c.mu.Unlock()
	acikMu.Lock()
	delete(acikIlerleme, c)
	acikMu.Unlock()
	c.sonuYaz(son)
	if kayit != nil {
		_ = c.gunluk.yaz(kayit) // program biterken de çağrılır; yazılamazsa sessiz kalır
	}
	c.isDurumunuYaz(simdi, true)
	return true
}

// yuvarla bir ölçümü günlük için 3 ondalık basamağa yuvarlar.
func yuvarla(f float64) float64 { return math.Round(f*1000) / 1000 }

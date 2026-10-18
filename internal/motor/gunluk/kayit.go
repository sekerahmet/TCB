package gunluk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/ortak/baglama"
	"tr_dili/internal/motor/ortak/jsonyaz"
)

// gunlukUzantisi günlük dosyalarının uzantısıdır (JSON satırları).
const gunlukUzantisi = ".jsonl"

// Gunluk TR'de açılmış bir eğitim günlüğüdür (tip Günlük). Kayıtlar dosyanın sonuna, her
// biri tek bir yazma çağrısıyla eklenir. Kontrol noktası gibi başka motorlar Go tarafından
// GunlukMu, Uzunluk ve DevamYaz ile kullanır.
type Gunluk struct {
	m       *yurut.VM // gizli değer süzgeci ve program adı (paylaşılan durum)
	d       *durum
	yol     string // kullanıcının verdiği yol
	tamYol  string // çözülmüş yol
	program string
	acilis  time.Time // başlangıç kaydının zamanı

	mu        sync.Mutex
	dosya     *os.File // nil: kapalı (yeniden yazılınca açılır)
	baslangic bool     // başlangıç kaydı yazıldı mı
	toplam    int64    // bağlı çubuğun toplamı (başlangıç kaydı için; 0: yok)
	kayitlar  int64    // bu açılıştan beri yazılan kayıt sayısı
}

func (g *Gunluk) Type() yurut.ValueType { return GunlukTipi }
func (g *Gunluk) IsTruthy() bool        { return true }
func (g *Gunluk) Inspect() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return fmt.Sprintf("%s(%s, %d kayıt)", GunlukTipi, g.yol, g.kayitlar)
}

// Yol günlük dosyasının çözülmüş yoludur.
func (g *Gunluk) Yol() string { return g.tamYol }

// GunlukMu değer bir günlük mü? Kontrol noktası gibi motorlar günlüğü bununla tanır.
func GunlukMu(v yurut.Value) (*Gunluk, bool) {
	g, ok := v.(*Gunluk)
	return g, ok
}

func (g *Gunluk) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	if kd == kod.Y004 {
		return &yurut.BuiltinVal{Name: kod.Ad(kod.Y004), AdliAlmaz: true, Fn: func(m *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			return baglama.Guvenli(yurut.CagriAdi(kod.Y004), func() (yurut.Value, error) { return g.ekle(args) })
		}}, nil
	}
	return nil, uyeHatasi(string(GunlukTipi), prop, kod.Y004)
}

// gunlukAc günlük(yol) yerleşiğidir.
func (d *durum) gunlukAc(m *yurut.VM, args []yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.G810)
	if len(args) != 1 {
		return nil, yurut.ArgumanSayisiHatasi(kod.G810, "1", len(args), "dosya")
	}
	yol, ok, err := yurut.YolArgumani(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, yurut.ArgumanTipHatasi(kod.G810, 1, args[0], kod.A202)
	}
	if !strings.EqualFold(filepath.Ext(yol), gunlukUzantisi) {
		return nil, fmt.Errorf("%s: %q bir günlük dosyası adı değil; günlük %s uzantılı olmalı (her satırı bir JSON kaydı), ör. %q",
			fn, yol, gunlukUzantisi, strings.TrimSuffix(yol, filepath.Ext(yol))+gunlukUzantisi)
	}
	g := &Gunluk{m: m, d: d, yol: yol, tamYol: baglama.YolCoz(m, yol), program: filepath.Base(m.SourceFile), acilis: saat()}
	if err := os.MkdirAll(filepath.Dir(g.tamYol), 0o755); err != nil {
		return nil, turkce.DosyaHatasi(fn, yol, "oluşturulamadı", err)
	}
	g.mu.Lock()
	err = g.ac()
	g.mu.Unlock()
	if err != nil {
		return nil, turkce.DosyaHatasi(fn, yol, "açılamadı", err)
	}
	return g, nil
}

// ac dosyayı ekleme kipinde açar; son satır yarım kalmışsa (önceki koşu çöktüyse) yeni
// kayıtlar yeni bir satırda başlasın diye satır sonu ekler. g.mu tutulur.
func (g *Gunluk) ac() error {
	f, err := os.OpenFile(g.tamYol, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if yarimSatir(g.tamYol) {
		if _, err := f.Write([]byte("\n")); err != nil {
			f.Close()
			return err
		}
	}
	g.dosya = f
	acikMu.Lock()
	acikGunluk[g] = struct{}{}
	acikMu.Unlock()
	return nil
}

// yarimSatir dosya boş değilse ve satır sonuyla bitmiyorsa doğrudur.
func yarimSatir(yol string) bool {
	f, err := os.Open(yol)
	if err != nil {
		return false
	}
	defer f.Close()
	son, err := f.Seek(-1, io.SeekEnd)
	if err != nil || son < 0 {
		return false
	}
	b := make([]byte, 1)
	n, _ := f.Read(b)
	return n == 1 && b[0] != '\n'
}

// kapat dosyayı kapatır (sonraki yazmada yeniden açılır). Hiç kayıt yazılmadıysa önce
// başlangıç kaydı yazılır: her açılışın bir başlangıç kaydı olur.
func (g *Gunluk) kapat() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.dosya != nil {
		if !g.baslangic {
			if satir, err := g.satir(g.baslangicKaydi()); err == nil {
				if _, err := g.dosya.Write(satir); err == nil {
					g.baslangic = true
				}
			}
		}
		g.dosya.Close()
		g.dosya = nil
	}
	acikMu.Lock()
	delete(acikGunluk, g)
	acikMu.Unlock()
}

// ekle Günlük.ekle(kayıt) metodudur: Sözlük bir kayıttır, Metin {"mesaj": ...} olur.
func (g *Gunluk) ekle(args []yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.Y004)
	if len(args) != 1 {
		return nil, yurut.ArgumanSayisiHatasi(kod.Y004, "1", len(args), "{ad: değer, ...}", "ileti")
	}
	kayit := yurut.NewDictVal()
	kayit.Set(aZaman, zamanDegeri(saat()))
	switch x := args[0].(type) {
	case *yurut.StringVal:
		kayit.Set(aMesaj, x)
	case *yurut.DictVal:
		if err := olculeriEkle(fn, kayit, x, aTur); err != nil {
			return nil, err
		}
	default:
		return nil, yurut.ArgumanTipHatasi(kod.Y004, 1, args[0], kod.A206, kod.A202)
	}
	if err := g.yaz(kayit); err != nil {
		return nil, err
	}
	return yurut.TRUE, nil
}

// olculeriEkle kullanıcının sözlüğündeki ölçüleri kayda ekler: anahtarlar Metin olmalı,
// ayrılmış anahtarlar hata verir, tek elemanlı tensörler sayı olur. Kullanıcı "zaman"
// verirse kaydın zamanı onunla değişir.
func olculeriEkle(fn string, kayit, sozluk *yurut.DictVal, ayrilmis ...string) error {
	for _, k := range sozluk.Order {
		v, ok := sozluk.Pairs[k]
		if !ok {
			continue
		}
		if a := sozluk.KeyValue(k); a != nil {
			if _, metin := a.(*yurut.StringVal); !metin {
				return fmt.Errorf("%s: ölçü adları %s olmalıdır, %s verildi", fn, kod.Ad(kod.A202), yurut.TipVeOzet(a))
			}
		}
		for _, r := range ayrilmis {
			if k == r {
				return fmt.Errorf("%s: '%s' adı günlük biçimine ayrılmış (%s); ölçüye başka bir ad verin", fn, k, ayrilmisNeden(k))
			}
		}
		d, err := olcuDegeri(fn, k, v)
		if err != nil {
			return err
		}
		kayit.Set(k, d)
	}
	return nil
}

// ayrilmisNeden bir anahtarın neden ayrılmış olduğunu söyler.
func ayrilmisNeden(k string) string {
	switch k {
	case aTur:
		return "kaydın türünü söyler: başlangıç, bitiş, devam"
	case aZaman:
		return "kaydın zamanını çubuk kendisi yazar"
	case aAdim:
		return "tamamlanan adımı çubuk kendisi yazar"
	}
	return "adım hızını çubuk kendisi yazar"
}

// yaz bir kaydı (gerekirse önce başlangıç kaydını) tek bir yazma çağrısıyla ekler.
func (g *Gunluk) yaz(kayit *yurut.DictVal) error {
	satir, err := g.satir(kayit)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.baslangic {
		bas, err := g.satir(g.baslangicKaydi())
		if err != nil {
			return err
		}
		satir = append(bas, satir...)
	}
	if g.dosya == nil {
		if err := g.ac(); err != nil {
			return turkce.DosyaHatasi(yurut.Cagri(kod.G810), g.yol, "açılamadı", err)
		}
	}
	if _, err := g.dosya.Write(satir); err != nil {
		return turkce.DosyaHatasi(yurut.Cagri(kod.G810), g.yol, "yazılamadı", err)
	}
	g.baslangic = true
	g.kayitlar++
	return nil
}

// satir kaydı JSON satırına çevirir: NaN ve sonsuz metin olarak, gizli değerler *** olarak.
func (g *Gunluk) satir(kayit *yurut.DictVal) ([]byte, error) {
	b, err := jsonyaz.YazSecenekli(kayit, jsonyaz.Secenekler{SayiDisiMetin: true, Metin: g.m.Maskele})
	if err != nil {
		return nil, fmt.Errorf("%s: kayıt JSON'a yazılamadı: %v", yurut.Cagri(kod.Y004), err)
	}
	return append(b, '\n'), nil
}

// baslangicKaydi programın adını, (varsa) bağlı çubuğun toplamını ve komut satırı
// ayarlarını taşır. g.mu tutulur.
func (g *Gunluk) baslangicKaydi() *yurut.DictVal {
	k := yurut.NewDictVal()
	k.Set(aTur, &yurut.StringVal{Val: turBaslangic})
	k.Set(aZaman, zamanDegeri(g.acilis))
	k.Set(aSurum, yurut.NewInt(bicimSurumu))
	k.Set(aProgram, &yurut.StringVal{Val: g.program})
	if g.toplam > 0 {
		k.Set(aToplam, yurut.NewInt(g.toplam))
	}
	if g.d != nil && g.d.surec != nil {
		if a := g.d.surec.Ayarlar(); a != nil {
			k.Set(aAyarlar, a)
		}
	}
	return k
}

// toplamAyarla bağlanan çubuğun toplamını başlangıç kaydı için saklar (kayıt henüz
// yazılmadıysa).
func (g *Gunluk) toplamAyarla(n int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.baslangic {
		g.toplam = n
	}
}

// Uzunluk dosyanın şimdiki bayt uzunluğudur. Kayıtlar tek yazma çağrısıyla eklendiğinden
// bu her zaman bir satır sınırıdır. Kontrol noktası kaydederken saklar.
func (g *Gunluk) Uzunluk() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	b, err := os.Stat(g.tamYol)
	if err != nil {
		return 0, err
	}
	return b.Size(), nil
}

// DevamYaz kontrol noktasından sürdürülen eğitimin {"tür":"devam","adım":N} kaydını yazar.
// N kaydedilen noktanın tamamlanmış adım sayısıdır. Okuyucu kuralı: bir devam kaydından önce
// gelen ve adımı N'den büyük olan ölçü kayıtları geçersizdir (kesilmeden önce yazılmış,
// yeniden hesaplanacak adımlar silinmez; adımı N olan kayıt noktaya dahildir).
func (g *Gunluk) DevamYaz(adim int64) error {
	k := yurut.NewDictVal()
	k.Set(aTur, &yurut.StringVal{Val: turDevam})
	k.Set(aZaman, zamanDegeri(saat()))
	k.Set(aAdim, yurut.NewInt(adim))
	return g.yaz(k)
}

// zamanDegeri kayıt zamanının metnidir.
func zamanDegeri(t time.Time) *yurut.StringVal {
	return &yurut.StringVal{Val: t.Format(zamanBicimi)}
}

// uyeHatasi tipin bilinmeyen bir üyesini, en yakın üyeyi önererek bildirir.
func uyeHatasi(tip, prop string, uyeler ...kod.Kod) error {
	adlar := kod.Adlar(uyeler...)
	oneri := fmt.Sprintf(" (üyeler: %s)", strings.Join(adlar, ", "))
	if o := turkce.EnYakin(prop, adlar); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", tip, prop, oneri)
}

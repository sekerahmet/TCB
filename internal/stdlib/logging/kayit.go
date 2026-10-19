package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/common/jsonwrite"
)

// gunlukUzantisi günlük dosyalarının uzantısıdır (JSON satırları).
const gunlukUzantisi = ".jsonl"

// Gunluk TR'de açılmış bir eğitim günlüğüdür (tip Günlük). Kayıtlar dosyanın sonuna, her
// biri tek bir yazma çağrısıyla eklenir. Kontrol noktası gibi başka motorlar Go tarafından
// GunlukMu, Uzunluk ve DevamYaz ile kullanır.
type Gunluk struct {
	m       *interp.VM // gizli değer süzgeci ve program adı (paylaşılan durum)
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

func (g *Gunluk) Type() interp.ValueType { return GunlukTipi }
func (g *Gunluk) IsTruthy() bool         { return true }
func (g *Gunluk) Inspect() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return fmt.Sprintf("%s(%s, %d kayıt)", GunlukTipi, g.yol, g.kayitlar)
}

// Yol günlük dosyasının çözülmüş yoludur.
func (g *Gunluk) Yol() string { return g.tamYol }

// GunlukMu değer bir günlük mü? Kontrol noktası gibi motorlar günlüğü bununla tanır.
func GunlukMu(v interp.Value) (*Gunluk, bool) {
	g, ok := v.(*Gunluk)
	return g, ok
}

func (g *Gunluk) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	if kd == codes.Y004 {
		return &interp.BuiltinVal{Name: codes.Name(codes.Y004), NoNamedArgs: true, Fn: func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return bind.Safe(interp.CallName(codes.Y004), func() (interp.Value, error) { return g.ekle(args) })
		}}, nil
	}
	return nil, uyeHatasi(string(GunlukTipi), prop, codes.Y004)
}

// gunlukAc günlük(yol) yerleşiğidir.
func (d *durum) gunlukAc(m *interp.VM, args []interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.G810)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.G810, "1", len(args), "dosya")
	}
	yol, ok, err := interp.PathArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, interp.ArgTypeError(codes.G810, 1, args[0], codes.A202)
	}
	if !strings.EqualFold(filepath.Ext(yol), gunlukUzantisi) {
		return nil, fmt.Errorf("%s: %q bir günlük dosyası adı değil; günlük %s uzantılı olmalı (her satırı bir JSON kaydı), ör. %q",
			fn, yol, gunlukUzantisi, strings.TrimSuffix(yol, filepath.Ext(yol))+gunlukUzantisi)
	}
	g := &Gunluk{m: m, d: d, yol: yol, tamYol: bind.ResolvePath(m, yol), program: filepath.Base(m.SourceFile), acilis: saat()}
	if err := os.MkdirAll(filepath.Dir(g.tamYol), 0o755); err != nil {
		return nil, turkish.FileError(fn, yol, "oluşturulamadı", err)
	}
	g.mu.Lock()
	err = g.ac()
	g.mu.Unlock()
	if err != nil {
		return nil, turkish.FileError(fn, yol, "açılamadı", err)
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
func (g *Gunluk) ekle(args []interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.Y004)
	if len(args) != 1 {
		return nil, interp.ArgCountError(codes.Y004, "1", len(args), "{ad: değer, ...}", "ileti")
	}
	kayit := interp.NewDictVal()
	kayit.Set(aZaman, zamanDegeri(saat()))
	switch x := args[0].(type) {
	case *interp.StringVal:
		kayit.Set(aMesaj, x)
	case *interp.DictVal:
		if err := olculeriEkle(fn, kayit, x, aTur); err != nil {
			return nil, err
		}
	default:
		return nil, interp.ArgTypeError(codes.Y004, 1, args[0], codes.A206, codes.A202)
	}
	if err := g.yaz(kayit); err != nil {
		return nil, err
	}
	return interp.TRUE, nil
}

// olculeriEkle kullanıcının sözlüğündeki ölçüleri kayda ekler: anahtarlar Metin olmalı,
// ayrılmış anahtarlar hata verir, tek elemanlı tensörler sayı olur. Kullanıcı "zaman"
// verirse kaydın zamanı onunla değişir.
func olculeriEkle(fn string, kayit, sozluk *interp.DictVal, ayrilmis ...string) error {
	for _, k := range sozluk.Order {
		v, ok := sozluk.Pairs[k]
		if !ok {
			continue
		}
		if a := sozluk.KeyValue(k); a != nil {
			if _, metin := a.(*interp.StringVal); !metin {
				return fmt.Errorf("%s: ölçü adları %s olmalıdır, %s verildi", fn, codes.Name(codes.A202), interp.TypeAndSummary(a))
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
func (g *Gunluk) yaz(kayit *interp.DictVal) error {
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
			return turkish.FileError(interp.CallPrefix(codes.G810), g.yol, "açılamadı", err)
		}
	}
	if _, err := g.dosya.Write(satir); err != nil {
		return turkish.FileError(interp.CallPrefix(codes.G810), g.yol, "yazılamadı", err)
	}
	g.baslangic = true
	g.kayitlar++
	return nil
}

// satir kaydı JSON satırına çevirir: NaN ve sonsuz metin olarak, gizli değerler *** olarak.
func (g *Gunluk) satir(kayit *interp.DictVal) ([]byte, error) {
	b, err := jsonwrite.WriteOptions(kayit, jsonwrite.Options{NonFiniteAsText: true, TextFilter: g.m.Mask})
	if err != nil {
		return nil, fmt.Errorf("%s: kayıt JSON'a yazılamadı: %v", interp.CallPrefix(codes.Y004), err)
	}
	return append(b, '\n'), nil
}

// baslangicKaydi programın adını, (varsa) bağlı çubuğun toplamını ve komut satırı
// ayarlarını taşır. g.mu tutulur.
func (g *Gunluk) baslangicKaydi() *interp.DictVal {
	k := interp.NewDictVal()
	k.Set(aTur, &interp.StringVal{Val: turBaslangic})
	k.Set(aZaman, zamanDegeri(g.acilis))
	k.Set(aSurum, interp.NewInt(bicimSurumu))
	k.Set(aProgram, &interp.StringVal{Val: g.program})
	if g.toplam > 0 {
		k.Set(aToplam, interp.NewInt(g.toplam))
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
	k := interp.NewDictVal()
	k.Set(aTur, &interp.StringVal{Val: turDevam})
	k.Set(aZaman, zamanDegeri(saat()))
	k.Set(aAdim, interp.NewInt(adim))
	return g.yaz(k)
}

// zamanDegeri kayıt zamanının metnidir.
func zamanDegeri(t time.Time) *interp.StringVal {
	return &interp.StringVal{Val: t.Format(zamanBicimi)}
}

// uyeHatasi tipin bilinmeyen bir üyesini, en yakın üyeyi önererek bildirir.
func uyeHatasi(tip, prop string, uyeler ...codes.Code) error {
	adlar := codes.Names(uyeler...)
	oneri := fmt.Sprintf(" (üyeler: %s)", strings.Join(adlar, ", "))
	if o := turkish.Closest(prop, adlar); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", tip, prop, oneri)
}

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*Gunluk)(nil)
)

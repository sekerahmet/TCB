package veri

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// OnbellekSecenekleri, yerel önbelleğin ayarlarıdır.
type OnbellekSecenekleri struct {
	// Dizin, kopyaların konacağı yerel dizindir (zorunlu). Önbellek bu
	// dizinde yalnızca kendi adlandırdığı dosyalara dokunur.
	Dizin string
	// Kota, önbelleğin kullanabileceği en çok bayt sayısıdır (zorunlu, > 0).
	// Örnekleyiciyle kullanıldığında en az (aynı anda açık parça sayısı +
	// ileri bakış) × en büyük parça boyutu olmalıdır; örnekleyici kurulurken
	// denetlenir.
	Kota int64
}

// OnbellekIstatistik, önbelleğin sayaçlarıdır.
type OnbellekIstatistik struct {
	Isabet         int64 // yerel kopyadan karşılanan Getir çağrıları
	Iska           int64 // Getir sırasında eşzamanlı kopyalanan dosyalar
	Atlanan        int64 // yer olmadığından reddedilen Getir çağrıları
	Silinen        int64 // kota için silinen dosyalar
	KopyalananBayt int64
	Kullanim       int64 // şu anki toplam bayt (süren kopyalar dahil)
	DosyaSayisi    int
}

// Onbellek, yavaş bir kaynak dizindeki (ör. ağ sürücüsü) dosyaları yerel
// bir dizine kopyalayan, disk kotası olan bir katmandır. Kota dolduğunda en
// uzun süredir kullanılmayan ve o an açık olmayan dosyalar silinir. Hazirla
// ile bildirilen sıradaki dosyalar arka planda kopyalanır. Yer açılamazsa
// Getir açıklayıcı bir hata döner; yavaş kaynaktan sessizce doğrudan
// okumaya geri dönülmez.
//
// Önbellek tek bir sıralı okuyucuya (eğitim örnekleyicisi) göre
// tasarlanmıştır: Hazirla her çağrıda bekleyen listeyi değiştirir. Aynı
// dizini iki Onbellek örneği aynı anda kullanmamalıdır.
type Onbellek struct {
	dizin string
	kota  int64

	mu        sync.Mutex
	kayit     map[string]*obGirdi // yerel ad → girdi
	kullanim  int64
	sayac     uint64
	istenen   []string
	istenenAd map[string]bool
	nesil     uint64
	bekleyen  bool // Hazirla listesi henüz işlenip bitmedi
	kapali    bool
	ist       OnbellekIstatistik
	sonHata   error

	uyari chan struct{}
	ctx   context.Context
	iptal context.CancelFunc
	wg    sync.WaitGroup
}

type obGirdi struct {
	ad    string
	boyut int64
	zaman time.Time     // kaynağın değişiklik zamanı (yerel dosyaya da yazılır)
	son   uint64        // son kullanım sayacı (LRU)
	pin   int           // açık kullanıcı sayısı; > 0 iken silinmez
	hazir bool          // dosya tamamen kopyalandı
	kopya chan struct{} // nil değilse kopyalanıyor; bitince kapanır
}

var (
	// ErrOnbellekYetersiz, önbellek kotasında dosya için yer açılamadığını
	// bildirir (kota küçük ya da açık dosyalar yer kaplıyor).
	ErrOnbellekYetersiz = errors.New("veri: önbellek kotası yetersiz")

	yonetilenAd  = regexp.MustCompile(`\.[0-9a-f]{16}\.onb(\.[^.]*)?$`)
	geciciSoneki = ".gecici"
)

// OnbellekAc, önbelleği açar; dizinde önceki çalıştırmadan kalan kopyalar
// varsa benimsenir (kaynakla boyut ve zaman eşleşirse kullanılır).
func OnbellekAc(s OnbellekSecenekleri) (*Onbellek, error) {
	if s.Dizin == "" {
		return nil, fmt.Errorf("veri: önbellek dizini verilmedi")
	}
	if s.Kota <= 0 {
		return nil, fmt.Errorf("veri: önbellek kotası pozitif olmalı")
	}
	if err := os.MkdirAll(s.Dizin, 0o755); err != nil {
		return nil, fmt.Errorf("veri: önbellek dizini oluşturulamadı: %w", err)
	}
	girdiler, err := os.ReadDir(s.Dizin)
	if err != nil {
		return nil, fmt.Errorf("veri: önbellek dizini okunamadı: %w", err)
	}
	ctx, iptal := context.WithCancel(context.Background())
	o := &Onbellek{
		dizin: s.Dizin, kota: s.Kota,
		kayit: map[string]*obGirdi{}, istenenAd: map[string]bool{},
		uyari: make(chan struct{}, 1), ctx: ctx, iptal: iptal,
	}
	for _, g := range girdiler {
		ad := g.Name()
		if g.IsDir() {
			continue
		}
		if strings.HasSuffix(ad, geciciSoneki) && yonetilenAd.MatchString(strings.TrimSuffix(ad, geciciSoneki)) {
			_ = os.Remove(filepath.Join(s.Dizin, ad)) // yarım kalmış kopya
			continue
		}
		if !yonetilenAd.MatchString(ad) {
			continue
		}
		bilgi, err := g.Info()
		if err != nil {
			continue
		}
		o.kayit[ad] = &obGirdi{ad: ad, boyut: bilgi.Size(), zaman: bilgi.ModTime(), hazir: true}
		o.kullanim += bilgi.Size()
	}
	if o.kullanim > o.kota {
		o.mu.Lock()
		o.yerAc(0, false)
		o.mu.Unlock()
	}
	o.wg.Add(1)
	go o.calis()
	return o, nil
}

// Kota, önbelleğin bayt kotasıdır.
func (o *Onbellek) Kota() int64 { return o.kota }

// pinliBayt, açık (iğneli) dosyaların toplam boyutudur; kilit tutulur.
func (o *Onbellek) pinliBayt() int64 {
	var t int64
	for _, g := range o.kayit {
		if g.pin > 0 {
			t += g.boyut
		}
	}
	return t
}

// yerelAd, kaynak yolu için önbellekteki dosya adını üretir. Ad, kaynağın
// tam yolunun özetini içerir; farklı dizinlerdeki aynı adlı dosyalar
// çakışmaz.
func yerelAd(kaynak string) string {
	tam, err := filepath.Abs(kaynak)
	if err != nil {
		tam = kaynak
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(filepath.Clean(tam)))
	taban := filepath.Base(kaynak)
	uz := filepath.Ext(taban)
	return fmt.Sprintf("%s.%016x.onb%s", strings.TrimSuffix(taban, uz), h.Sum64(), uz)
}

func ayniZaman(a, b time.Time) bool {
	d := a.Sub(b)
	return d < 2*time.Second && d > -2*time.Second
}

// Getir, kaynağın yerel kopyasının yolunu döner; kopya yoksa kopyalar (ya
// da arka planda süren kopyayı bekler). Dönen birak işlevi çağrılana kadar
// dosya silinmez. Kotada yer açılamıyorsa ErrOnbellekYetersiz'i saran
// açıklayıcı bir hata döner (kaynaktan doğrudan okumaya geri dönülmez).
func (o *Onbellek) Getir(ctx context.Context, kaynak string) (string, func(), error) {
	return o.getir(ctx, kaynak, false)
}

func (o *Onbellek) getir(ctx context.Context, kaynak string, onYukleme bool) (string, func(), error) {
	st, err := os.Stat(kaynak)
	if err != nil {
		return "", nil, fmt.Errorf("veri: önbellek kaynağı: %w", err)
	}
	ad := yerelAd(kaynak)
	yerel := filepath.Join(o.dizin, ad)
	kopyaladik := false

	o.mu.Lock()
	for {
		if o.kapali {
			o.mu.Unlock()
			return "", nil, ErrKapali
		}
		g := o.kayit[ad]
		if g != nil && g.kopya != nil {
			if onYukleme {
				o.mu.Unlock()
				return "", nil, nil // başkası kopyalıyor
			}
			c := g.kopya
			o.mu.Unlock()
			select {
			case <-c:
			case <-ctx.Done():
				return "", nil, ctx.Err()
			}
			o.mu.Lock()
			continue
		}
		if g != nil && g.hazir {
			if g.boyut == st.Size() && ayniZaman(g.zaman, st.ModTime()) {
				o.sayac++
				g.son = o.sayac
				if onYukleme {
					o.mu.Unlock()
					return yerel, nil, nil
				}
				g.pin++
				if !kopyaladik {
					o.ist.Isabet++
				}
				o.mu.Unlock()
				return yerel, o.birakici(g), nil
			}
			// Kaynak değişmiş: kopya bayat.
			if g.pin > 0 || !o.sil(g) {
				if onYukleme {
					o.mu.Unlock()
					return "", nil, nil
				}
				o.ist.Atlanan++
				o.mu.Unlock()
				return "", nil, fmt.Errorf("veri: %s: kaynak değişmiş ama eski yerel kopya hâlâ kullanımda; yenilenemiyor", kaynak)
			}
		}
		if st.Size() > o.kota || !o.yerAc(st.Size(), onYukleme) {
			if onYukleme {
				o.mu.Unlock()
				return "", nil, ErrOnbellekYetersiz
			}
			o.ist.Atlanan++
			hata := fmt.Errorf("%w: %s için %d bayt gerekli; kota %d bayt, kullanımda %d bayt (açık dosyalar %d bayt). "+
				"Yavaş kaynaktan doğrudan okumamak için kopya alınmadı; kotayı büyütün",
				ErrOnbellekYetersiz, filepath.Base(kaynak), st.Size(), o.kota, o.kullanim, o.pinliBayt())
			o.mu.Unlock()
			return "", nil, hata
		}
		g = &obGirdi{ad: ad, boyut: st.Size(), zaman: st.ModTime(), kopya: make(chan struct{})}
		o.kayit[ad] = g
		o.kullanim += g.boyut
		if !onYukleme {
			o.ist.Iska++
		}
		o.mu.Unlock()

		err := kopyala(ctx, kaynak, yerel, st.ModTime())

		o.mu.Lock()
		close(g.kopya)
		g.kopya = nil
		if err != nil {
			delete(o.kayit, ad)
			o.kullanim -= g.boyut
			o.mu.Unlock()
			return "", nil, err
		}
		g.hazir = true
		o.ist.KopyalananBayt += g.boyut
		kopyaladik = true
	}
}

func (o *Onbellek) birakici(g *obGirdi) func() {
	var bir sync.Once
	return func() {
		bir.Do(func() {
			o.mu.Lock()
			g.pin--
			o.sayac++
			g.son = o.sayac
			o.mu.Unlock()
		})
	}
}

// sil, girdinin dosyasını siler; kilit tutulur. Silinemezse false döner.
func (o *Onbellek) sil(g *obGirdi) bool {
	err := os.Remove(filepath.Join(o.dizin, g.ad))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false
	}
	delete(o.kayit, g.ad)
	o.kullanim -= g.boyut
	o.ist.Silinen++
	return true
}

// yerAc, gerekli bayt için kotada yer açar; kilit tutulur. Silinecek
// adaylar: hazır, açık olmayan ve kopyalanmayan dosyalar, en eski
// kullanılan önce. Önden yüklemede, hâlâ istenen dosyalar silinmez; eşzamanlı
// Getir'de ise istenenler en son silinir. Yer açılamayacaksa hiçbir şey
// silinmez ve false döner.
func (o *Onbellek) yerAc(gerekli int64, onYukleme bool) bool {
	if o.kullanim+gerekli <= o.kota {
		return true
	}
	var adaylar []*obGirdi
	var serbest int64
	for _, g := range o.kayit {
		if !g.hazir || g.pin > 0 || g.kopya != nil {
			continue
		}
		if onYukleme && o.istenenAd[g.ad] {
			continue
		}
		adaylar = append(adaylar, g)
		serbest += g.boyut
	}
	if o.kullanim-serbest+gerekli > o.kota {
		return false
	}
	sort.Slice(adaylar, func(i, j int) bool {
		ai, aj := o.istenenAd[adaylar[i].ad], o.istenenAd[adaylar[j].ad]
		if ai != aj {
			return !ai
		}
		if adaylar[i].son != adaylar[j].son {
			return adaylar[i].son < adaylar[j].son
		}
		return adaylar[i].ad < adaylar[j].ad
	})
	for _, g := range adaylar {
		if o.kullanim+gerekli <= o.kota {
			break
		}
		o.sil(g)
	}
	return o.kullanim+gerekli <= o.kota
}

// kopyala, kaynağı geçici bir dosyaya kopyalar, değişiklik zamanını
// kaynakla eşitler ve hedef adına taşır.
func kopyala(ctx context.Context, kaynak, hedef string, zaman time.Time) (err error) {
	k, err := os.Open(kaynak)
	if err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	defer k.Close()
	gecici := hedef + geciciSoneki
	h, err := os.Create(gecici)
	if err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	defer func() {
		if err != nil {
			_ = h.Close()
			_ = os.Remove(gecici)
		}
	}()
	buf := make([]byte, 4<<20)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := k.Read(buf)
		if n > 0 {
			if _, werr := h.Write(buf[:n]); werr != nil {
				return fmt.Errorf("veri: önbellek kopyası: %w", werr)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("veri: önbellek kopyası: %w", rerr)
		}
	}
	if err := h.Close(); err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	if err := os.Chtimes(gecici, zaman, zaman); err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	if err := os.Rename(gecici, hedef); err != nil {
		return fmt.Errorf("veri: önbellek kopyası: %w", err)
	}
	return nil
}

// Hazirla, sıradaki kaynak dosyaları (öncelik sırasıyla) bildirir; arka
// plandaki gorutin bunları kotanın izin verdiği kadar kopyalar. Her çağrı
// önceki listenin yerini alır.
func (o *Onbellek) Hazirla(kaynaklar []string) {
	o.mu.Lock()
	o.istenen = append([]string(nil), kaynaklar...)
	o.istenenAd = make(map[string]bool, len(kaynaklar))
	for _, k := range kaynaklar {
		o.istenenAd[yerelAd(k)] = true
	}
	o.nesil++
	o.bekleyen = true
	o.mu.Unlock()
	select {
	case o.uyari <- struct{}{}:
	default:
	}
}

func (o *Onbellek) calis() {
	defer o.wg.Done()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-o.uyari:
		}
		o.mu.Lock()
		liste, nesil := o.istenen, o.nesil
		o.mu.Unlock()
		for _, k := range liste {
			if o.ctx.Err() != nil {
				return
			}
			o.mu.Lock()
			degisti := o.nesil != nesil
			o.mu.Unlock()
			if degisti {
				break // uyarı kanalında yeni liste bekliyor
			}
			_, _, err := o.getir(o.ctx, k, true)
			if errors.Is(err, ErrOnbellekYetersiz) {
				break
			}
			if err != nil && o.ctx.Err() == nil {
				o.mu.Lock()
				o.sonHata = err
				o.mu.Unlock()
			}
		}
		o.mu.Lock()
		if o.nesil == nesil {
			o.bekleyen = false
		}
		o.mu.Unlock()
	}
}

// Bekle, Hazirla ile istenen dosyaların arka plan kopyası bitene (ya da
// kota yüzünden durana) kadar bekler. Çoğunlukla testler içindir.
func (o *Onbellek) Bekle(ctx context.Context) error {
	for {
		o.mu.Lock()
		bitti := !o.bekleyen || o.kapali
		o.mu.Unlock()
		if bitti {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// Istatistik, sayaçların anlık kopyasıdır.
func (o *Onbellek) Istatistik() OnbellekIstatistik {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := o.ist
	s.Kullanim = o.kullanim
	s.DosyaSayisi = len(o.kayit)
	return s
}

// SonHata, arka plan kopyalamasında görülen son hatadır (yoksa nil).
func (o *Onbellek) SonHata() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.sonHata
}

// Dizin, önbellek dizinidir.
func (o *Onbellek) Dizin() string { return o.dizin }

// Kapat, arka plan kopyalamasını durdurur ve gorutinin bitmesini bekler.
// Yerel kopyalar diskte kalır ve sonraki açılışta benimsenir.
func (o *Onbellek) Kapat() error {
	o.mu.Lock()
	o.kapali = true
	o.mu.Unlock()
	o.iptal()
	o.wg.Wait()
	return nil
}

package dataset

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/interp"
)

// Okuyucu durumu: ö.durum (Y502) okuyucunun kaldığı yeri düz değerli bir sözlük olarak verir,
// ö.durumu_yükle(sözlük) (Y503) aynı ayarla kurulmuş bir okuyucuyu o konuma getirir. Konum
// tüketilen son toplu işten sonraki konumdur (önden yükleme hattında bekleyen toplu işler
// sayılmaz); yüklemede önden yükleme hattı boşaltılıp okuyucu konumdan yeniden kurulur. Böylece
// sürdürülen eğitim kesintisizle aynı toplu işleri aynı sırayla alır. Kontrol noktası
// (internal/stdlib/checkpoint) okuyucuyu bu iki Go yöntemiyle kaydeder (DurumSozlugu, DurumuYukle).

const (
	uyeDurum       = codes.Y502
	uyeDurumuYukle = codes.Y503
)

// Durum sözlüğünün anahtarları ve okuyucu türleri dosya biçiminin (kontrol noktasının
// durum.json'u) parçasıdır: yazım dilinden bağımsızdır ve çevrilmez (günlük biçimi gibi).
var durumAdlari = strings.Fields("tür toplu bağlam tohum küme dönem sıra pencere sayaç bitti örnekleyici sıralı")

var (
	dTur, dToplu, dBaglam, dTohum  = durumAdlari[0], durumAdlari[1], durumAdlari[2], durumAdlari[3]
	dKume, dDonem, dSira, dPencere = durumAdlari[4], durumAdlari[5], durumAdlari[6], durumAdlari[7]
	dSayac, dBitti                 = durumAdlari[8], durumAdlari[9]
	turOrnekleyici, turSirali      = durumAdlari[10], durumAdlari[11]
)

// kumeOzeti veri kümesinin kimliğidir: veri dosyalarının adları ve parça sayılarının özeti.
// Veri klasörü taşınsa da (yerel disk → bulut sürücüsü) aynı kalır; dosya eklenir, çıkarılır
// ya da değişirse farklılaşır.
func kumeOzeti(k *dataload.Kume) string {
	h := sha256.New()
	var b [8]byte
	for _, p := range k.Parcalar() {
		h.Write([]byte(p.Ad))
		h.Write([]byte{0})
		binary.LittleEndian.PutUint64(b[:], uint64(p.Token))
		h.Write(b[:])
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

func (o *Okuyucu) turAdi() string {
	if o.sirali {
		return turSirali
	}
	return turOrnekleyici
}

// DurumSozlugu okuyucunun durumudur (TR'de ö.durum): tür, toplu, bağlam, (örnekleyicide)
// tohum, küme özeti, konum (dönem, sıra, pencere), okunan toplu iş sayısı ve (sıralıda)
// bitti. Değerler düz değerlerdir; sözlük JSON'a yazılabilir.
func (o *Okuyucu) DurumSozlugu(*interp.VM) (*interp.DictVal, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := interp.NewDictVal()
	s.Set(dTur, &interp.StringVal{Val: o.turAdi()})
	s.Set(dToplu, interp.NewInt(int64(o.b)))
	s.Set(dBaglam, interp.NewInt(int64(o.t)))
	if !o.sirali {
		s.Set(dTohum, interp.NewInt(int64(o.tohum)))
	}
	s.Set(dKume, &interp.StringVal{Val: kumeOzeti(o.kume.k)})
	if !o.sirali {
		s.Set(dDonem, interp.NewInt(int64(o.konum.Donem)))
	}
	s.Set(dSira, interp.NewInt(int64(o.konum.Sira)))
	s.Set(dPencere, interp.NewInt(o.konum.Pencere))
	s.Set(dSayac, interp.NewInt(int64(o.sayac)))
	if o.sirali {
		s.Set(dBitti, interp.BoolToVal(o.bitti))
	}
	return s, nil
}

// durumuYukleTR ö.durumu_yükle(sözlük) metodudur.
func (o *Okuyucu) durumuYukleTR(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	if len(named) > 0 || len(args) != 1 {
		if len(named) > 0 {
			return nil, fmt.Errorf("%s adlı argüman almaz", interp.Cagri(uyeDurumuYukle))
		}
		return nil, interp.ArgumanSayisiHatasi(uyeDurumuYukle, "1", len(args), "ö."+codes.Ad(uyeDurum))
	}
	s, ok := args[0].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgumanTipHatasi(uyeDurumuYukle, 1, args[0], codes.A206)
	}
	if err := o.DurumuYukle(m, s); err != nil {
		return nil, err
	}
	return interp.TRUE, nil
}

// DurumuYukle durumu (DurumSozlugu biçiminde) yükler: tür, toplu, bağlam, tohum ve küme
// okuyucununkilerle aynı olmalıdır, değilse yol gösterici bir hata verir ve okuyucuyu
// değiştirmez. Önden yükleme hattı durdurulur, okuyucu kaydedilen konumdan yeniden kurulur.
func (o *Okuyucu) DurumuYukle(m *interp.VM, s *interp.DictVal) error {
	fn := interp.Cagri(uyeDurumuYukle)
	d := durumOkuyucu{fn: fn, s: s}
	tur := d.metin(dTur)
	if d.hata == nil && tur != o.turAdi() {
		d.hata = fmt.Errorf("%s: kayıt bir %s okuyucusunun durumu, bu okuyucu %s", fn, tur, o.turAdi())
	}
	d.ayni(dToplu, adToplu.TR, int64(o.b))
	d.ayni(dBaglam, adBaglam.TR, int64(o.t))
	if !o.sirali {
		d.ayni(dTohum, adTohum.TR, int64(o.tohum))
	}
	if k := d.metin(dKume); d.hata == nil && k != kumeOzeti(o.kume.k) {
		d.hata = fmt.Errorf("%s: kayıt başka bir veri kümesine ait (veri dosyalarının adları ya da boyları farklı: %q); aynı veriyle sürdürün ya da yeni bir koşu klasörü verin", fn, o.kume.yol)
	}
	var konum dataload.Durum
	if !o.sirali {
		konum.Donem = int(d.tam(dDonem))
	}
	konum.Sira, konum.Pencere = int(d.tam(dSira)), d.tam(dPencere)
	sayac := int(d.tam(dSayac))
	bitti := o.sirali && d.mantik(dBitti)
	if d.hata != nil {
		return d.hata
	}
	return o.konumaGetir(m, fn, konum, sayac, bitti)
}

// konumaGetir okuyucuyu verilen konumdan yeniden kurar ve eski önden yükleme hattını kapatır.
func (o *Okuyucu) konumaGetir(m *interp.VM, fn string, konum dataload.Durum, sayac int, bitti bool) error {
	var kaynakGo dataload.Kaynak
	var err error
	m.Blocking(func() {
		if o.sirali {
			kaynakGo, err = o.kume.k.Sirali(dataload.SiraliSecenekleri{B: o.b, T: o.t, Baslangic: &konum})
		} else {
			kaynakGo, err = o.kume.k.Ornekleyici(dataload.OrnekleyiciSecenekleri{B: o.b, T: o.t, Tohum: o.tohum, Baslangic: &konum})
		}
	})
	if err != nil {
		return fmt.Errorf("%s: kayıttaki konum bu veri kümesinde geçersiz (%s); kayıt bozuk ya da başka bir veriye ait: %v", fn, o.kume.yol, temizle(err.Error()))
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	oy := dataload.OnYukle(context.Background(), kaynakGo, onYuklemeDerinligi)
	o.kaynak = o.kaynak.degistir(onYukleyiciKapatici(oy))
	o.oy, o.ti, o.konum = oy, dataload.TopluIs{}, konum
	o.sayac, o.donem, o.bitti = sayac, konum.Donem, bitti
	if bitti {
		_ = o.kaynak.Kapat(0)
	}
	return nil
}

// durumOkuyucu durum sözlüğünden alan okur; ilk hatayı tutar (sonraki okumalar boş döner).
type durumOkuyucu struct {
	fn   string
	s    *interp.DictVal
	hata error
}

func (d *durumOkuyucu) al(ad string) (interp.Value, bool) {
	if d.hata != nil {
		return nil, false
	}
	v, ok := d.s.Pairs[ad]
	if !ok {
		d.hata = fmt.Errorf("%s: durumda %q alanı yok; sözlük okuyucunun %s'u olmalı", d.fn, ad, codes.Ad(uyeDurum))
	}
	return v, ok
}

func (d *durumOkuyucu) metin(ad string) string {
	v, ok := d.al(ad)
	if !ok {
		return ""
	}
	m, ok := v.(*interp.StringVal)
	if !ok {
		d.hata = fmt.Errorf("%s: durumun %q alanı %s olmalı, %s verildi", d.fn, ad, codes.Ad(codes.A202), interp.TipVeOzet(v))
		return ""
	}
	return m.Val
}

func (d *durumOkuyucu) tam(ad string) int64 {
	v, ok := d.al(ad)
	if !ok {
		return 0
	}
	n, ok := v.(*interp.IntVal)
	if !ok || n.Val < 0 {
		d.hata = fmt.Errorf("%s: durumun %q alanı 0 ya da pozitif bir %s olmalı, %s verildi", d.fn, ad, codes.Ad(codes.A200), interp.TipVeOzet(v))
		return 0
	}
	return n.Val
}

func (d *durumOkuyucu) mantik(ad string) bool {
	v, ok := d.al(ad)
	if !ok {
		return false
	}
	b, ok := v.(*interp.BoolVal)
	if !ok {
		d.hata = fmt.Errorf("%s: durumun %q alanı %s olmalı, %s verildi", d.fn, ad, codes.Ad(codes.A203), interp.TipVeOzet(v))
		return false
	}
	return b.Val
}

// ayni durumdaki bir ayarın okuyucununkiyle aynı olmasını ister.
func (d *durumOkuyucu) ayni(ad, gorunen string, simdi int64) {
	if n := d.tam(ad); d.hata == nil && n != simdi {
		d.hata = fmt.Errorf("%s: kayıttaki okuyucunun %s değeri %d, bu okuyucununki %d; aynı ayarla sürdürün ya da yeni bir koşu klasörü verin", d.fn, gorunen, n, simdi)
	}
}

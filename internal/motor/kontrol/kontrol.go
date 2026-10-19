// Package kontrol uzun eğitimin kontrol noktasıdır: kontrol_noktası(koşu, {ad: nesne, ...})
// (T702; tip KontrolNoktası, A258) eğitimin bütün durumunu koşu klasörüne atomik kaydeder ve
// yeniden başlatılan programı kaldığı adımdan, kesintisiz eğitimle bit düzeyinde aynı
// sürecek biçimde devam ettirir. Kayıt adım sınırında yapılır: kn.güncelle(adım) (Y010)
// her_dakika süresi dolunca ya da durdurma istenince (Ctrl+C), kn.kaydet() (Y521) hemen.
//
// Klasör düzeni (biçim sürümü 2; bkz. bicim.go):
//
//	<koşu>/kontrol_noktalari/
//	  son.json              {"sürüm":2,"adım":1500,"klasör":"adim_0000001500"} (araçlar için)
//	  adim_0000001500/
//	    tensorler.trt       modellerin tensörleri (türev izlemeyenler dahil), iyileştirici durumları
//	    durum.json          adım, program, ayarlar, okuyucu durumları, düz değerler, günlük boyları
//	    ozet.json           dosyaların bayt sayısı ve SHA-256 özeti (en son yazılır)
//	  adim_0000001000/      tut: 2 → bir önceki de durur
//
// Yazma: nokta önce adim_N.tmp-<süreç> klasörüne yazılıp diske işlenir, sonra tek bir
// yeniden adlandırmayla yerine konur, son.json atomik güncellenir ve tut'u aşan eskiler
// silinir; her aşamada kesilme önceki geçerli noktayı bozmaz. Aralıkla kayıtta yazma hatası
// eğitimi durdurmaz (uyarı, sonraki aralıkta yeniden deneme; art arda 3 başarısızlıkta açık
// hata). Yükleme: en yeni nokta klasörü özetleriyle doğrulanır, bütün nesnelerin uyumu
// hiçbir şey yüklenmeden denetlenir, sonra okuyucular, parametreler (yerinde), iyileştirici
// durumları (parametrelerin cihazına), düz değerler (yerinde) yüklenir ve günlüklere devam
// kaydı yazılır. Bozuk bir nokta açık hatadır ve bir önceki geçerli noktayı önerir.
//
// Kontrol noktası kurulunca ilk kesme isteği programı hemen durdurmaz (yurut.VM.KesmeIste):
// sıradaki adım sınırında nokta kaydedilir ve program nazikçe (çıkış kodu 3) durur.
//
// Katman: motor (internal/motor); dil ve hesap çekirdeklerini ve öbür motorları içe aktarır.
package kontrol

import (
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"tr_dili/internal/dil/kod"

	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/motor/dosya"
	"tr_dili/internal/motor/ogrenme"
	"tr_dili/internal/motor/ortak/baglama"
	"tr_dili/internal/motor/surec"
)

// KontrolTipi KontrolNoktası değerlerinin tip adıdır.
var KontrolTipi = yurut.TipAdi(kod.A258)

// Adlı argümanlar; adları kod listesindedir (internal/dil/kod).
var (
	adHerDakika = baglama.KodAd(kod.T984)
	adTut       = baglama.KodAd(kod.T985)
	adTohum     = baglama.KodAd(kod.T903)
)

// Varsayılanlar ve sınırlar.
const (
	varsayilanDakika = 15
	varsayilanTut    = 2
	enFazlaDakika    = 1e6
	// enFazlaBasarisiz art arda bu kadar aralıklı kayıt başarısız olursa eğitim durur.
	enFazlaBasarisiz = 3
)

// saat şimdiki zamandır; testler aralığı sınamak için değiştirir.
var saat = time.Now

// Kaydet kontrol_noktası yerleşiğini VM'ye kaydeder. s programın süreç durumudur (komut
// satırı ayarları noktaya yazılır); nil olabilir.
func Kaydet(v *yurut.VM, s *surec.Durum) {
	v.RegisterBuiltinKod(kod.T702, func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		return baglama.Guvenli(kod.Ad(kod.T702), func() (yurut.Value, error) { return kur(m, s, args, named) })
	})
}

// KontrolNoktasi TR'de kurulmuş bir kontrol noktasıdır (tip KontrolNoktası). Alanları
// yorumlayıcı kilidi altında değişir.
type KontrolNoktasi struct {
	gorunen  string // kullanıcıya gösterilen klasör: <koşu>/kontrol_noktalari
	klasor   string // çözülmüş klasör
	nesneler []*nesne
	aralik   time.Duration
	tut      int
	surec    *surec.Durum

	kaldigi    int64     // yüklenen noktanın adımı (yeni koşuda 0)
	adim       int64     // son güncelle(adım)
	sonKayit   time.Time // son kayıt ya da kayıt denemesi (aralık buradan sayılır)
	basarisiz  int       // art arda başarısız aralıklı kayıt
	sonGecerli int64     // son geçerli noktanın adımı (-1: yok)
}

func (kn *KontrolNoktasi) Type() yurut.ValueType { return KontrolTipi }
func (kn *KontrolNoktasi) IsTruthy() bool        { return true }
func (kn *KontrolNoktasi) Inspect() string {
	return fmt.Sprintf("%s(%s, adım %d, %d nesne)", KontrolTipi, kn.gorunen, kn.adim, len(kn.nesneler))
}

// Klasor noktaların çözülmüş klasörüdür (<koşu>/kontrol_noktalari).
func (kn *KontrolNoktasi) Klasor() string { return kn.klasor }

func (kn *KontrolNoktasi) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	switch kd {
	case kod.Y520:
		return yurut.NewInt(kn.kaldigi), nil
	case kod.Y010:
		return &yurut.BuiltinVal{Name: kod.Ad(kod.Y010), AdliAlmaz: true, Fn: func(m *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			return baglama.Guvenli(yurut.CagriAdi(kod.Y010), func() (yurut.Value, error) { return kn.guncelle(m, args) })
		}}, nil
	case kod.Y521:
		return &yurut.BuiltinVal{Name: kod.Ad(kod.Y521), AdliAlmaz: true, Fn: func(m *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			return baglama.Guvenli(yurut.CagriAdi(kod.Y521), func() (yurut.Value, error) { return kn.kaydetTR(m, args) })
		}}, nil
	}
	return nil, baglama.UyeHatasi(string(KontrolTipi), prop, kod.Adlar(kod.Y010, kod.Y520, kod.Y521))
}

// kur kontrol_noktası(koşu, nesneler, her_dakika:, tut:) yerleşiğidir.
func kur(m *yurut.VM, s *surec.Durum, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := yurut.Cagri(kod.T702)
	if err := baglama.AdliDenetle(fn, named, adHerDakika, adTut); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, yurut.ArgumanSayisiHatasi(kod.T702, "2", len(args), "koşu_klasörü, {model: model, op: op, ...}")
	}
	yol, ok, err := yurut.YolArgumani(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !ok || yol == "" {
		return nil, yurut.ArgumanTipHatasi(kod.T702, 1, args[0], kod.A202, kod.A250)
	}
	sozluk, ok := args[1].(*yurut.DictVal)
	if !ok {
		return nil, yurut.ArgumanTipHatasi(kod.T702, 2, args[1], kod.A206)
	}
	aralik, tut, err := secenekler(fn, named)
	if err != nil {
		return nil, err
	}
	nesneler, err := nesneleriAyir(fn, sozluk)
	if err != nil {
		return nil, err
	}
	kosu := baglama.YolCoz(m, yol)
	if b, err := os.Stat(kosu); err == nil && !b.IsDir() {
		return nil, fmt.Errorf("%s: %q bir dosya; koşu klasörü olmalı (noktalar %s/ altına yazılır)", fn, yol, klasorAdi)
	}
	kn := &KontrolNoktasi{gorunen: path.Join(filepath.ToSlash(yol), klasorAdi), klasor: filepath.Join(kosu, klasorAdi),
		nesneler: nesneler, aralik: aralik, tut: tut, surec: s, sonGecerli: -1}
	if err := kn.yukle(m, fn); err != nil {
		return nil, err
	}
	kn.sonKayit = saat()
	m.KesmeyiErtele()
	tohumsuzUyarisiKur(m)
	return kn, nil
}

// secenekler her_dakika: ve tut: adlı argümanlarını okur.
func secenekler(fn string, named map[string]yurut.Value) (time.Duration, int, error) {
	dakika := float64(varsayilanDakika)
	if v, ok := baglama.AdliAl(named, adHerDakika); ok {
		f, sayi := yurut.ToFloat(v)
		if !sayi || math.IsNaN(f) || f < 0 || f > enFazlaDakika {
			return 0, 0, fmt.Errorf("%s: %s 0 ile %g arasında bir sayı (dakika) olmalı, %s verildi", fn, adHerDakika.TR, float64(enFazlaDakika), yurut.TipVeOzet(v))
		}
		dakika = f
	}
	tut := varsayilanTut
	if v, ok := baglama.AdliAl(named, adTut); ok {
		n, tam := v.(*yurut.IntVal)
		if !tam || n.Val < 1 || n.Val > 1000 {
			return 0, 0, fmt.Errorf("%s: %s 1 ile 1000 arasında bir %s olmalı (saklanacak nokta sayısı), %s verildi", fn, adTut.TR, kod.Ad(kod.A200), yurut.TipVeOzet(v))
		}
		tut = int(n.Val)
	}
	return time.Duration(dakika * float64(time.Minute)), tut, nil
}

// tohumsuzUyarisiKur tohumsuz rastgelelik kullanılınca bir kez uyarır: sürdürülen eğitim
// kesintisizle aynı olmayacaktır (paylaşılan üretecin durumu kaydedilemez).
func tohumsuzUyarisiKur(m *yurut.VM) {
	var bir sync.Once
	ogrenme.TohumsuzKancasiAyarla(func() {
		bir.Do(func() {
			m.CiktiYaz(m.Err, fmt.Sprintf("Uyarı: %s kurulduktan sonra tohumsuz rastgelelik kullanıldı (%s ya da %s, %s: verilmeden); sürdürülen eğitim kesintisizle aynı olmayacak. Rastgeleliği %s: ile verin.\n",
				kod.Ad(kod.T702), kod.Ad(kod.T103), kod.Ad(kod.T611), adTohum.TR, adTohum.TR))
		})
	})
}

// adimArg güncelle'nin tamamlanan adım argümanını okur.
func adimArg(kd kod.Kod, args []yurut.Value) (int64, error) {
	if len(args) != 1 {
		return 0, yurut.ArgumanSayisiHatasi(kd, "1", len(args), "tamamlanan_adım")
	}
	n, ok := args[0].(*yurut.IntVal)
	if !ok || n.Val < 0 {
		return 0, fmt.Errorf("%s: tamamlanan adım sayısı 0 ya da pozitif bir %s olmalı, %s verildi", yurut.Cagri(kd), kod.Ad(kod.A200), yurut.TipVeOzet(args[0]))
	}
	return n.Val, nil
}

// guncelle kn.güncelle(adım) metodudur: adım sınırı. Durdurma istendiyse kaydeder ve
// programı durdurur; her_dakika süresi dolduysa kaydeder (kaydettiyse doğru).
func (kn *KontrolNoktasi) guncelle(m *yurut.VM, args []yurut.Value) (yurut.Value, error) {
	n, err := adimArg(kod.Y010, args)
	if err != nil {
		return nil, err
	}
	kn.adim = n
	if m.DurdurmaIstendi() {
		kn.durdur(m)
	}
	if saat().Sub(kn.sonKayit) < kn.aralik {
		return yurut.FALSE, nil
	}
	return kn.araliklaKaydet(m)
}

// araliklaKaydet aralıklı kaydı yapar: yazma hatası eğitimi durdurmaz (uyarı, sonraki
// aralıkta yeniden deneme), art arda enFazlaBasarisiz başarısızlık açık hatadır.
func (kn *KontrolNoktasi) araliklaKaydet(m *yurut.VM) (yurut.Value, error) {
	_, err := kn.kaydet(m)
	kn.sonKayit = saat()
	if err == nil {
		kn.basarisiz = 0
		return yurut.TRUE, nil
	}
	if _, toplama := err.(toplamaHatasi); toplama {
		return nil, err
	}
	kn.basarisiz++
	if kn.basarisiz >= enFazlaBasarisiz {
		return nil, fmt.Errorf("%v; kontrol noktası art arda %d kez kaydedilemedi, eğitim durduruldu. %s", err, kn.basarisiz, kn.sonGecerliMetni())
	}
	ne := "sonraki " + kod.Ad(kod.Y010) + "'de"
	if kn.aralik > 0 {
		ne = sureMetni(kn.aralik) + " sonra"
	}
	m.CiktiYaz(m.Err, fmt.Sprintf("Uyarı: %v (%d/%d; eğitim sürüyor, %s yeniden denenecek). %s\n",
		err, kn.basarisiz, enFazlaBasarisiz, ne, kn.sonGecerliMetni()))
	return yurut.FALSE, nil
}

// kaydetTR kn.kaydet() metodudur: hemen kaydeder; hata açık hatadır.
func (kn *KontrolNoktasi) kaydetTR(m *yurut.VM, args []yurut.Value) (yurut.Value, error) {
	if len(args) != 0 {
		return nil, yurut.ArgumanSayisiHatasi(kod.Y521, "0", len(args))
	}
	yol, err := kn.kaydet(m)
	kn.sonKayit = saat()
	if err != nil {
		return nil, err
	}
	kn.basarisiz = 0
	if m.DurdurmaIstendi() {
		kn.durdur(m)
	}
	return dosya.YeniYol(yol), nil
}

// durdur ertelenen kesme isteğini karşılar: noktayı kaydeder (hata uyarıdır) ve programı
// nazikçe durdurur (dönmez).
func (kn *KontrolNoktasi) durdur(m *yurut.VM) {
	if yol, err := kn.kaydet(m); err != nil {
		m.CiktiYaz(m.Err, fmt.Sprintf("Uyarı: program durdurulurken kontrol noktası kaydedilemedi (%v). %s\n", err, kn.sonGecerliMetni()))
	} else {
		m.CiktiYaz(m.Err, fmt.Sprintf("Kontrol noktası kaydedildi: adım %d (%s). Sürdürmek için programı aynı komutla yeniden çalıştırın.\n", kn.adim, yol))
	}
	m.SimdiKes()
}

// sonGecerliMetni son geçerli noktayı söyler (uyarı ve hata mesajları için).
func (kn *KontrolNoktasi) sonGecerliMetni() string {
	if kn.sonGecerli < 0 {
		return "Henüz geçerli bir kontrol noktası yok"
	}
	return fmt.Sprintf("Son geçerli nokta: adım %d (%s/%s)", kn.sonGecerli, kn.gorunen, noktaAdi(kn.sonGecerli))
}

// sureMetni bir aralığı kısaca yazar: 15 dk, 30 sn.
func sureMetni(d time.Duration) string {
	if d >= time.Minute {
		return strings.Replace(strconv.FormatFloat(d.Minutes(), 'g', 3, 64), ".", ",", 1) + " dk"
	}
	return strings.Replace(strconv.FormatFloat(d.Seconds(), 'g', 3, 64), ".", ",", 1) + " sn"
}

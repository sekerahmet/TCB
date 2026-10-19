// Package checkpoint uzun eğitimin kontrol noktasıdır: kontrol_noktası(koşu, {ad: nesne, ...})
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
// Layer: standard library (internal/stdlib); dil ve hesap çekirdeklerini ve öbür motorları içe aktarır.
package checkpoint

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

	"github.com/sekerahmet/TCB/internal/lang/codes"

	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/files"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
	"github.com/sekerahmet/TCB/internal/stdlib/process"
)

// KontrolTipi KontrolNoktası değerlerinin tip adıdır.
var KontrolTipi = interp.TypeName(codes.A258)

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	adHerDakika = bind.KodAd(codes.T984)
	adTut       = bind.KodAd(codes.T985)
	adTohum     = bind.KodAd(codes.T903)
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
func Kaydet(v *interp.VM, s *process.Durum) {
	v.RegisterBuiltinCode(codes.T702, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(codes.Name(codes.T702), func() (interp.Value, error) { return kur(m, s, args, named) })
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
	surec    *process.Durum

	kaldigi    int64     // yüklenen noktanın adımı (yeni koşuda 0)
	adim       int64     // son güncelle(adım)
	sonKayit   time.Time // son kayıt ya da kayıt denemesi (aralık buradan sayılır)
	basarisiz  int       // art arda başarısız aralıklı kayıt
	sonGecerli int64     // son geçerli noktanın adımı (-1: yok)
}

func (kn *KontrolNoktasi) Type() interp.ValueType { return KontrolTipi }
func (kn *KontrolNoktasi) IsTruthy() bool         { return true }
func (kn *KontrolNoktasi) Inspect() string {
	return fmt.Sprintf("%s(%s, adım %d, %d nesne)", KontrolTipi, kn.gorunen, kn.adim, len(kn.nesneler))
}

// Klasor noktaların çözülmüş klasörüdür (<koşu>/kontrol_noktalari).
func (kn *KontrolNoktasi) Klasor() string { return kn.klasor }

func (kn *KontrolNoktasi) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case codes.Y520:
		return interp.NewInt(kn.kaldigi), nil
	case codes.Y010:
		return &interp.BuiltinVal{Name: codes.Name(codes.Y010), NoNamedArgs: true, Fn: func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return bind.Guvenli(interp.CallName(codes.Y010), func() (interp.Value, error) { return kn.guncelle(m, args) })
		}}, nil
	case codes.Y521:
		return &interp.BuiltinVal{Name: codes.Name(codes.Y521), NoNamedArgs: true, Fn: func(m *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			return bind.Guvenli(interp.CallName(codes.Y521), func() (interp.Value, error) { return kn.kaydetTR(m, args) })
		}}, nil
	}
	return nil, bind.UyeHatasi(string(KontrolTipi), prop, codes.Names(codes.Y010, codes.Y520, codes.Y521))
}

// kur kontrol_noktası(koşu, nesneler, her_dakika:, tut:) yerleşiğidir.
func kur(m *interp.VM, s *process.Durum, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := interp.CallPrefix(codes.T702)
	if err := bind.AdliDenetle(fn, named, adHerDakika, adTut); err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, interp.ArgCountError(codes.T702, "2", len(args), "koşu_klasörü, {model: model, op: op, ...}")
	}
	yol, ok, err := interp.PathArg(fn, args[0])
	if err != nil {
		return nil, err
	}
	if !ok || yol == "" {
		return nil, interp.ArgTypeError(codes.T702, 1, args[0], codes.A202, codes.A250)
	}
	sozluk, ok := args[1].(*interp.DictVal)
	if !ok {
		return nil, interp.ArgTypeError(codes.T702, 2, args[1], codes.A206)
	}
	aralik, tut, err := secenekler(fn, named)
	if err != nil {
		return nil, err
	}
	nesneler, err := nesneleriAyir(fn, sozluk)
	if err != nil {
		return nil, err
	}
	kosu := bind.YolCoz(m, yol)
	if b, err := os.Stat(kosu); err == nil && !b.IsDir() {
		return nil, fmt.Errorf("%s: %q bir dosya; koşu klasörü olmalı (noktalar %s/ altına yazılır)", fn, yol, klasorAdi)
	}
	kn := &KontrolNoktasi{gorunen: path.Join(filepath.ToSlash(yol), klasorAdi), klasor: filepath.Join(kosu, klasorAdi),
		nesneler: nesneler, aralik: aralik, tut: tut, surec: s, sonGecerli: -1}
	if err := kn.yukle(m, fn); err != nil {
		return nil, err
	}
	kn.sonKayit = saat()
	m.DeferInterrupts()
	tohumsuzUyarisiKur(m)
	return kn, nil
}

// secenekler her_dakika: ve tut: adlı argümanlarını okur.
func secenekler(fn string, named map[string]interp.Value) (time.Duration, int, error) {
	dakika := float64(varsayilanDakika)
	if v, ok := bind.AdliAl(named, adHerDakika); ok {
		f, sayi := interp.ToFloat(v)
		if !sayi || math.IsNaN(f) || f < 0 || f > enFazlaDakika {
			return 0, 0, fmt.Errorf("%s: %s 0 ile %g arasında bir sayı (dakika) olmalı, %s verildi", fn, adHerDakika.TR, float64(enFazlaDakika), interp.TypeAndSummary(v))
		}
		dakika = f
	}
	tut := varsayilanTut
	if v, ok := bind.AdliAl(named, adTut); ok {
		n, tam := v.(*interp.IntVal)
		if !tam || n.Val < 1 || n.Val > 1000 {
			return 0, 0, fmt.Errorf("%s: %s 1 ile 1000 arasında bir %s olmalı (saklanacak nokta sayısı), %s verildi", fn, adTut.TR, codes.Name(codes.A200), interp.TypeAndSummary(v))
		}
		tut = int(n.Val)
	}
	return time.Duration(dakika * float64(time.Minute)), tut, nil
}

// tohumsuzUyarisiKur tohumsuz rastgelelik kullanılınca bir kez uyarır: sürdürülen eğitim
// kesintisizle aynı olmayacaktır (paylaşılan üretecin durumu kaydedilemez).
func tohumsuzUyarisiKur(m *interp.VM) {
	var bir sync.Once
	ml.TohumsuzKancasiAyarla(func() {
		bir.Do(func() {
			m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %s kurulduktan sonra tohumsuz rastgelelik kullanıldı (%s ya da %s, %s: verilmeden); sürdürülen eğitim kesintisizle aynı olmayacak. Rastgeleliği %s: ile verin.\n",
				codes.Name(codes.T702), codes.Name(codes.T103), codes.Name(codes.T611), adTohum.TR, adTohum.TR))
		})
	})
}

// adimArg güncelle'nin tamamlanan adım argümanını okur.
func adimArg(kd codes.Code, args []interp.Value) (int64, error) {
	if len(args) != 1 {
		return 0, interp.ArgCountError(kd, "1", len(args), "tamamlanan_adım")
	}
	n, ok := args[0].(*interp.IntVal)
	if !ok || n.Val < 0 {
		return 0, fmt.Errorf("%s: tamamlanan adım sayısı 0 ya da pozitif bir %s olmalı, %s verildi", interp.CallPrefix(kd), codes.Name(codes.A200), interp.TypeAndSummary(args[0]))
	}
	return n.Val, nil
}

// guncelle kn.güncelle(adım) metodudur: adım sınırı. Durdurma istendiyse kaydeder ve
// programı durdurur; her_dakika süresi dolduysa kaydeder (kaydettiyse doğru).
func (kn *KontrolNoktasi) guncelle(m *interp.VM, args []interp.Value) (interp.Value, error) {
	n, err := adimArg(codes.Y010, args)
	if err != nil {
		return nil, err
	}
	kn.adim = n
	if m.StopRequested() {
		kn.durdur(m)
	}
	if saat().Sub(kn.sonKayit) < kn.aralik {
		return interp.FALSE, nil
	}
	return kn.araliklaKaydet(m)
}

// araliklaKaydet aralıklı kaydı yapar: yazma hatası eğitimi durdurmaz (uyarı, sonraki
// aralıkta yeniden deneme), art arda enFazlaBasarisiz başarısızlık açık hatadır.
func (kn *KontrolNoktasi) araliklaKaydet(m *interp.VM) (interp.Value, error) {
	_, err := kn.kaydet(m)
	kn.sonKayit = saat()
	if err == nil {
		kn.basarisiz = 0
		return interp.TRUE, nil
	}
	if _, toplama := err.(toplamaHatasi); toplama {
		return nil, err
	}
	kn.basarisiz++
	if kn.basarisiz >= enFazlaBasarisiz {
		return nil, fmt.Errorf("%v; kontrol noktası art arda %d kez kaydedilemedi, eğitim durduruldu. %s", err, kn.basarisiz, kn.sonGecerliMetni())
	}
	ne := "sonraki " + codes.Name(codes.Y010) + "'de"
	if kn.aralik > 0 {
		ne = sureMetni(kn.aralik) + " sonra"
	}
	m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: %v (%d/%d; eğitim sürüyor, %s yeniden denenecek). %s\n",
		err, kn.basarisiz, enFazlaBasarisiz, ne, kn.sonGecerliMetni()))
	return interp.FALSE, nil
}

// kaydetTR kn.kaydet() metodudur: hemen kaydeder; hata açık hatadır.
func (kn *KontrolNoktasi) kaydetTR(m *interp.VM, args []interp.Value) (interp.Value, error) {
	if len(args) != 0 {
		return nil, interp.ArgCountError(codes.Y521, "0", len(args))
	}
	yol, err := kn.kaydet(m)
	kn.sonKayit = saat()
	if err != nil {
		return nil, err
	}
	kn.basarisiz = 0
	if m.StopRequested() {
		kn.durdur(m)
	}
	return files.YeniYol(yol), nil
}

// durdur ertelenen kesme isteğini karşılar: noktayı kaydeder (hata uyarıdır) ve programı
// nazikçe durdurur (dönmez).
func (kn *KontrolNoktasi) durdur(m *interp.VM) {
	if yol, err := kn.kaydet(m); err != nil {
		m.WriteOutput(m.Err, fmt.Sprintf("Uyarı: program durdurulurken kontrol noktası kaydedilemedi (%v). %s\n", err, kn.sonGecerliMetni()))
	} else {
		m.WriteOutput(m.Err, fmt.Sprintf("Kontrol noktası kaydedildi: adım %d (%s). Sürdürmek için programı aynı komutla yeniden çalıştırın.\n", kn.adim, yol))
	}
	m.InterruptNow()
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

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter = (*KontrolNoktasi)(nil)
)

// Package dataset, .npy token parçaları için veri yükleyiciyi (github.com/sekerahmet/TCB/internal/compute/dataload) TR diline
// bağlar: veri_kümesi(dizin, önbellek:, kota_gb:), kümenin .örnekleyici(...) ve
// .sıralı(...) okuyucuları ve okuyucuların .sonraki() metodu.
//
// .sonraki() [girdi, hedef] listesi döndürür; ikisi de [toplu, bağlam] şeklinde, tam sayı
// değerli tensörlerdir ve gömme/çapraz_entropi'ye doğrudan verilebilir. Önden yükleme her
// zaman açıktır: sonraki toplu işler arka plandaki bir Go gorutininde hazırlanır. Bu
// gorutin TR kodu çalıştırmaz, yorumlayıcı kilidini hiç tutmaz; .sonraki() beklerken
// kilit bırakılır.
//
// Katman: motor (internal/stdlib); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package dataset

import (
	"context"
	"errors"
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sekerahmet/TCB/internal/compute/dataload"
	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
	"github.com/sekerahmet/TCB/internal/stdlib/ml"
)

// Tip adları.
var (
	KumeTipi        = string(interp.TipAdi(codes.A231)) // VeriKümesi
	OrnekleyiciTipi = string(interp.TipAdi(codes.A232)) // Örnekleyici
	SiraliTipi      = string(interp.TipAdi(codes.A233)) // SıralıOkuyucu
)

// onYuklemeDerinligi arka planda hazır tutulan toplu iş sayısıdır.
const onYuklemeDerinligi = 4

// enBuyukTamFloat32 float32'nin tam gösterebildiği en büyük tam sayıdır (2^24).
const enBuyukTamFloat32 = 1 << 24

// Adlı argümanlar; adları kod listesindedir (internal/lang/codes).
var (
	adOnbellek = bind.KodAd(codes.V900)
	adKota     = bind.KodAd(codes.V901)
	adToplu    = bind.KodAd(codes.V902)
	adBaglam   = bind.KodAd(codes.V903)
	adTohum    = bind.KodAd(codes.T903)
)

// Kaydet veri yerleşiklerini VM'ye kaydeder.
func Kaydet(v *interp.VM) {
	v.RegisterBuiltinKod(codes.V100, func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(codes.Ad(codes.V100), func() (interp.Value, error) { return kumeAc(m, args, named) })
	})
}

// ----------------------------------------------------------------------------
// Küme
// ----------------------------------------------------------------------------

// Kume TR'de açılmış bir veri kümesidir.
type Kume struct {
	k      *dataload.Kume
	yol    string // kullanıcının verdiği yol
	ob     *dataload.Onbellek
	kaynak *kaynak // yalnızca önbellek varsa
}

// GoKumesi sarılı Go kümesini verir.
func (k *Kume) GoKumesi() *dataload.Kume { return k.k }

func (k *Kume) Type() interp.ValueType { return interp.ValueType(KumeTipi) }
func (k *Kume) IsTruthy() bool         { return true }

func (k *Kume) ozet() string {
	return fmt.Sprintf("%d veri dosyası, %s parça", k.k.ParcaSayisi(), bind.KisaSayi(k.k.ToplamToken()))
}

func (k *Kume) Inspect() string {
	ek := ""
	if k.ob != nil {
		ek = fmt.Sprintf(", önbellek %s", bind.BaytMetni(k.ob.Kota()))
	}
	return fmt.Sprintf("%s(%s%s)", KumeTipi, k.ozet(), ek)
}

// IzlemeTuru İzleme Masası'nda ikon seçimi için türdür.
func (k *Kume) IzlemeTuru() string { return "diger" }

// IzlemeEtiketi tip sütununda gösterilen özettir.
func (k *Kume) IzlemeEtiketi() string { return "Veri kümesi (" + k.ozet() + ")" }

// TabloVerisi parçaları (ad, token, boyut) tablo olarak verir.
func (k *Kume) TabloVerisi() ([]string, [][]string, error) {
	const sinir = 10000
	var satirlar [][]string
	for i, p := range k.k.Parcalar() {
		if i >= sinir {
			break
		}
		satirlar = append(satirlar, []string{p.Ad, fmt.Sprint(p.Token), bind.BaytMetni(p.Bayt)})
	}
	return []string{"Veri dosyası", "Parça", "Boyut"}, satirlar, nil
}

// Veri kümesi ve okuyucu üyeleri (adları kod listesinde, internal/lang/codes Y60x).
const (
	uyeOrnekleyici = codes.Y600
	uyeSirali      = codes.Y601
	uyeSonraki     = codes.Y602
)

var kumeUyeleri = codes.Adlar(uyeOrnekleyici, uyeSirali)

func (k *Kume) GetProperty(kd codes.Kod, prop string) (interp.Value, error) {
	switch kd {
	case uyeOrnekleyici:
		return metot(codes.Ad(uyeOrnekleyici), k.ornekleyici), nil
	case uyeSirali:
		return metot(codes.Ad(uyeSirali), k.sirali), nil
	}
	return nil, bind.UyeHatasi("Veri kümesi", prop, kumeUyeleri)
}

func metot(ad string, fn interp.BuiltinFunc) *interp.BuiltinVal {
	return &interp.BuiltinVal{Name: ad, Fn: func(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
		return bind.Guvenli(ad, func() (interp.Value, error) { return fn(m, args, named) })
	}}
}

// onbellekDizini önbellek: adlı argümanının (verildiyse) çözülmüş klasör yoludur.
func onbellekDizini(m *interp.VM, fn string, named map[string]interp.Value) (string, error) {
	v, ok := bind.AdliAl(named, adOnbellek)
	if !ok {
		return "", nil
	}
	d, ok, err := interp.YolArgumani(fn, v)
	if !ok || d == "" {
		return "", fmt.Errorf("%s: %s yerel bir klasör yolu (metin) olmalıdır, ör. %s: \"onbellek/veri\"", fn, adOnbellek.TR, adOnbellek.TR)
	}
	if err != nil {
		return "", err
	}
	// Göreli yol, diğer yollar gibi tek kuralla çözülür (16.8): önce çalışma
	// klasörü, orada yoksa programın klasörü.
	return bind.YolCoz(m, d), nil
}

func kumeAc(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(codes.V100)
	if err := bind.AdliDenetle(fn+"()", named, adOnbellek, adKota); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(dizin) 1 parametre bekler (.npy veri dosyalarının klasörü ya da tek bir .npy dosyası), %d verildi", fn, len(args))
	}
	ham, ok, yolHata := interp.YolArgumani(fn, args[0])
	if !ok {
		return nil, fmt.Errorf("%s: dizin bir metin olmalıdır, %s verildi", fn, args[0].Type())
	}
	if yolHata != nil {
		return nil, yolHata
	}
	yol, bakilanlar := bind.YolCozBakilan(m, ham)

	obDizin, obHata := onbellekDizini(m, fn, named)
	if obHata != nil {
		return nil, obHata
	}
	var kota int64
	kotaV, kotaVar := bind.AdliAl(named, adKota)
	if kotaVar {
		if obDizin == "" {
			return nil, fmt.Errorf("%s: %s yalnızca %s: ile birlikte kullanılır (önbellek klasörünün en çok kaç GB kullanacağı)", fn, adKota.TR, adOnbellek.TR)
		}
		gb, ok := sayi(kotaV)
		if !ok || gb <= 0 || math.IsInf(gb, 0) || math.IsNaN(gb) {
			return nil, fmt.Errorf("%s: %s pozitif bir sayı olmalıdır (ör. %s: 20), %s verildi", fn, adKota.TR, adKota.TR, kotaV.Inspect())
		}
		// int64 bayt sayısına sığmayan kota (≈ 8,6 milyar GB) taşıp negatif olmasın.
		if gb >= float64(math.MaxInt64)/(1<<30) {
			return nil, fmt.Errorf("%s: %s çok büyük (%s); en çok %d GB verilebilir", fn, adKota.TR, kotaV.Inspect(), int64(math.MaxInt64>>30))
		}
		kota = max(int64(gb*(1<<30)), 1)
	}

	var k *dataload.Kume
	var ob *dataload.Onbellek
	var err error
	m.Blocking(func() {
		// Önbelleksiz okumada parçalar belleğe eşlenmez: her pencere ReadAt ile okunur.
		// Ağ ya da akışlı bulut sürücülerinde yalnızca istenen baytlar okunur; yerel
		// diskte de bir toplu iş başına birkaç düzine küçük okuma eğitim adımının yanında
		// önemsizdir. Önbellekteki yerel kopyalar eşlenerek okunur.
		sec := dataload.KumeSecenekleri{Parca: dataload.ParcaSecenekleri{EslemeKapali: obDizin == ""}}
		if obDizin != "" && kotaVar {
			ob, err = dataload.OnbellekAc(dataload.OnbellekSecenekleri{Dizin: obDizin, Kota: kota})
			if err != nil {
				return
			}
			sec.Onbellek = ob
		}
		k, err = dataload.KumeAc(yol, sec)
		if err != nil && ob != nil {
			_ = ob.Kapat()
			ob = nil
		}
	})
	if err != nil {
		return nil, veriHatasi(fn, ham, err, interp.BakilanYerler(bakilanlar))
	}
	if obDizin != "" && !kotaVar {
		gereken := enBuyukParca(k) * int64(dataload.VarsayilanGrupBoyu+dataload.VarsayilanIleriBak)
		return nil, fmt.Errorf("%s: %s: verildiğinde %s: de verilmelidir (önbellek klasörünün kullanabileceği en çok disk alanı, GB). "+
			"Bu kümenin örnekleyicisi aynı anda %d veri dosyasını yerelde tutar; en büyük veri dosyası %s olduğundan en az %s: %d gerekir",
			fn, adOnbellek.TR, adKota.TR, dataload.VarsayilanGrupBoyu+dataload.VarsayilanIleriBak, bind.BaytMetni(enBuyukParca(k)), adKota.TR, int64(math.Ceil(float64(gereken)/(1<<30))))
	}
	tk := &Kume{k: k, yol: ham, ob: ob}
	if ob != nil {
		tk.kaynak = kaynakEkle(tk, seviyeOnbellek, func(time.Duration) error { return ob.Kapat() })
	}
	return tk, nil
}

func enBuyukParca(k *dataload.Kume) int64 {
	var b int64
	for _, p := range k.Parcalar() {
		b = max(b, p.Bayt)
	}
	return b
}

func sayi(v interp.Value) (float64, bool) {
	switch n := v.(type) {
	case *interp.IntVal:
		return float64(n.Val), true
	case *interp.FloatVal:
		return n.Val, true
	}
	return 0, false
}

// veriHatasi Go veri paketinin hatasını yol gösterici bir TR hatasına çevirir; ekler
// küme bulunamadığında mesaja eklenir (ör. bakılan yerler).
func veriHatasi(fn, yol string, err error, ekler ...string) error {
	var bh *dataload.BicimHatasi
	switch {
	case errors.As(err, &bh):
		dosya := bh.Yol
		if dosya == "" {
			dosya = yol
		}
		return fmt.Errorf("%s: %q geçerli bir .npy veri dosyası değil: %s. Veri dosyaları standart .npy biçiminde tek boyutlu, küçük uçlu uint16, uint32 ya da int32 dizileri olmalıdır",
			fn, filepath.ToSlash(dosya), bh.Neden)
	case errors.Is(err, dataload.ErrOnbellekYetersiz):
		return fmt.Errorf("%s: önbellek kotası yetersiz: %s", fn, temizle(strings.TrimPrefix(err.Error(), dataload.ErrOnbellekYetersiz.Error()+": ")))
	case errors.Is(err, dataload.ErrKapali):
		return fmt.Errorf("%s: okuyucu kapatılmış (program sonlanıyor); yeni bir okuyucu oluşturun", fn)
	case errors.Is(err, dataload.ErrOnbellekKilitli):
		return fmt.Errorf("%s: %s", fn, temizle(strings.TrimPrefix(err.Error(), dataload.ErrOnbellekKilitli.Error()+": ")))
	case bind.YokMu(err) && strings.Contains(err.Error(), "küme açılamadı"):
		return turkish.DosyaHatasi(fn, yol, "açılamadı", err,
			append([]string{".npy veri dosyalarının bulunduğu klasörün (ya da tek bir .npy dosyasının) yolunu verin"}, ekler...)...)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		// İzin, kilitli dosya, disk hatası...: işletim sisteminin metni yerine Türkçe neden
		return turkish.DosyaHatasi(fn, filepath.ToSlash(pe.Path), "okunamadı", err)
	}
	msg := temizle(err.Error())
	if strings.Contains(msg, "desenine uyan veri dosyası yok") {
		return fmt.Errorf("%s: %q klasöründe .npy veri dosyası yok; klasör .npy uzantılı veri dosyaları içermelidir", fn, yol)
	}
	return fmt.Errorf("%s: %s", fn, msg)
}

// temizle Go paketinin "veri: " öneklerini kaldırır.
func temizle(s string) string { return strings.ReplaceAll(s, "veri: ", "") }

// ----------------------------------------------------------------------------
// Okuyucular (örnekleyici ve sıralı)
// ----------------------------------------------------------------------------

// Okuyucu .sonraki() ile [girdi, hedef] tensörleri veren örnekleyici ya da sıralı
// okuyucudur. Önden yükleyici her zaman açıktır.
type Okuyucu struct {
	sirali bool
	kume   *Kume // önbelleğin okuyucudan önce kapatılmaması için canlı tutulur
	b, t   int
	tohum  uint64

	mu     sync.Mutex
	oy     *dataload.OnYukleyici
	ti     dataload.TopluIs
	konum  dataload.Durum // tüketilen son toplu işten sonraki konum (durum, durum.go)
	sayac  int
	donem  int
	bitti  bool
	kaynak *kaynak
}

func (o *Okuyucu) Type() interp.ValueType {
	if o.sirali {
		return interp.ValueType(SiraliTipi)
	}
	return interp.ValueType(OrnekleyiciTipi)
}

func (o *Okuyucu) IsTruthy() bool { return true }

func (o *Okuyucu) Inspect() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.sirali {
		durum := fmt.Sprintf("%d toplu iş", o.sayac)
		if o.bitti {
			durum += ", bitti"
		}
		return fmt.Sprintf("%s(%s: %d, %s: %d, %s)", SiraliTipi, adToplu.TR, o.b, adBaglam.TR, o.t, durum)
	}
	return fmt.Sprintf("%s(%s: %d, %s: %d, %s: %d, dönem: %d, %d toplu iş)", OrnekleyiciTipi, adToplu.TR, o.b, adBaglam.TR, o.t, adTohum.TR, o.tohum, o.donem, o.sayac)
}

// IzlemeTuru İzleme Masası'nda ikon seçimi için türdür.
func (o *Okuyucu) IzlemeTuru() string { return "diger" }

// IzlemeEtiketi tip sütununda gösterilen özettir.
func (o *Okuyucu) IzlemeEtiketi() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.sirali {
		if o.bitti {
			return fmt.Sprintf("Sıralı okuyucu (%d×%d, bitti)", o.b, o.t)
		}
		return fmt.Sprintf("Sıralı okuyucu (%d×%d, %d toplu iş okundu)", o.b, o.t, o.sayac)
	}
	return fmt.Sprintf("%s (%d×%d, dönem %d, %d toplu iş)", OrnekleyiciTipi, o.b, o.t, o.donem, o.sayac)
}

// TabloVerisi okuyucuyu tablo olarak açmaz; toplu işler .sonraki() ile alınır.
func (o *Okuyucu) TabloVerisi() ([]string, [][]string, error) {
	return nil, nil, fmt.Errorf("okuyucu tablo olarak açılamaz; .%s() ile alınan girdi ve hedef tensörlerini inceleyin", codes.Ad(uyeSonraki))
}

func (o *Okuyucu) GetProperty(kd codes.Kod, prop string) (interp.Value, error) {
	switch kd {
	case uyeSonraki:
		return metot(codes.Ad(uyeSonraki), o.sonraki), nil
	case uyeDurum:
		return o.DurumSozlugu(nil)
	case uyeDurumuYukle:
		return metot(codes.Ad(uyeDurumuYukle), o.durumuYukleTR), nil
	}
	ne := OrnekleyiciTipi
	if o.sirali {
		ne = "Sıralı okuyucu"
	}
	return nil, bind.UyeHatasi(ne, prop, codes.Adlar(uyeSonraki, uyeDurum, uyeDurumuYukle))
}

// okuyucuArgumanlari toplu, bağlam ve (örnekleyicide) tohum argümanlarını okur. Konumlu
// olarak da aynı sırayla verilebilirler.
func okuyucuArgumanlari(fn string, args []interp.Value, named map[string]interp.Value, tohumlu bool) (b, t int, tohum uint64, err error) {
	adlar := []bind.Ad{adToplu, adBaglam}
	if tohumlu {
		adlar = append(adlar, adTohum)
	}
	if err = bind.AdliDenetle(fn+"()", named, adlar...); err != nil {
		return
	}
	if len(args) > len(adlar) {
		err = fmt.Errorf("%s() en çok %d parametre alır, %d verildi", fn, len(adlar), len(args))
		return
	}
	degerler := make([]interp.Value, len(adlar))
	for i, a := range adlar {
		v, adli := bind.AdliAl(named, a)
		if i < len(args) {
			if adli {
				err = fmt.Errorf("%s: %s hem konumlu hem adlı verildi", fn, a.TR)
				return
			}
			v = args[i]
		}
		degerler[i] = v
	}
	ornek := fmt.Sprintf("%s(%s: 32, %s: 1024)", fn, adToplu.TR, adBaglam.TR)
	if tohumlu {
		ornek = fmt.Sprintf("%s(%s: 32, %s: 1024, %s: 1)", fn, adToplu.TR, adBaglam.TR, adTohum.TR)
	}
	pozitif := func(ad string, v interp.Value) (int, error) {
		if v == nil {
			return 0, fmt.Errorf("%s: %s: verilmedi; ör. %s", fn, ad, ornek)
		}
		n, ok := bind.TamSayi(v)
		if !ok || n <= 0 || n > 1<<24 {
			return 0, fmt.Errorf("%s: %s pozitif bir tam sayı olmalıdır, %s verildi", fn, ad, v.Inspect())
		}
		return int(n), nil
	}
	if b, err = pozitif(adToplu.TR, degerler[0]); err != nil {
		return
	}
	if t, err = pozitif(adBaglam.TR, degerler[1]); err != nil {
		return
	}
	if int64(b)*int64(t+1) > int64(interp.MaxCollectionSize) {
		err = fmt.Errorf("%s: %s × %s (%d × %d) çok büyük; en fazla %d parça", fn, adToplu.TR, adBaglam.TR, b, t, interp.MaxCollectionSize)
		return
	}
	if tohumlu && degerler[2] != nil {
		n, ok := bind.TamSayi(degerler[2])
		if !ok || n < 0 {
			err = fmt.Errorf("%s: %s negatif olmayan bir tam sayı olmalıdır, %s verildi", fn, adTohum.TR, degerler[2].Inspect())
			return
		}
		tohum = uint64(n)
	}
	return
}

func (k *Kume) ornekleyici(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(uyeOrnekleyici)
	b, t, tohum, err := okuyucuArgumanlari(fn, args, named, true)
	if err != nil {
		return nil, err
	}
	var ok *dataload.Ornekleyici
	m.Blocking(func() {
		ok, err = k.k.Ornekleyici(dataload.OrnekleyiciSecenekleri{B: b, T: t, Tohum: tohum})
	})
	if err != nil {
		return nil, okuyucuHatasi(fn, k, t, err)
	}
	return k.okuyucuKur(false, b, t, tohum, ok), nil
}

func (k *Kume) sirali(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(uyeSirali)
	b, t, _, err := okuyucuArgumanlari(fn, args, named, false)
	if err != nil {
		return nil, err
	}
	var so *dataload.SiraliOkuyucu
	m.Blocking(func() {
		so, err = k.k.Sirali(dataload.SiraliSecenekleri{B: b, T: t})
	})
	if err != nil {
		return nil, okuyucuHatasi(fn, k, t, err)
	}
	return k.okuyucuKur(true, b, t, 0, so), nil
}

func okuyucuHatasi(fn string, k *Kume, t int, err error) error {
	if strings.Contains(err.Error(), "parçadan uzun değil") {
		var enUzun int64
		for _, p := range k.k.Parcalar() {
			enUzun = max(enUzun, p.Token)
		}
		return fmt.Errorf("%s: %s: %d için her pencere %d parça ister, ama en uzun veri dosyası %d parça; %s değerini küçültün", fn, adBaglam.TR, t, t+1, enUzun, adBaglam.TR)
	}
	if errors.Is(err, dataload.ErrOnbellekYetersiz) {
		return fmt.Errorf("%s: önbellek kotası yetersiz: %s", fn, temizle(strings.TrimPrefix(err.Error(), dataload.ErrOnbellekYetersiz.Error()+": ")))
	}
	return veriHatasi(fn, k.yol, err)
}

func (k *Kume) okuyucuKur(sirali bool, b, t int, tohum uint64, kaynakGo dataload.Kaynak) *Okuyucu {
	oy := dataload.OnYukle(context.Background(), kaynakGo, onYuklemeDerinligi)
	o := &Okuyucu{sirali: sirali, kume: k, b: b, t: t, tohum: tohum, oy: oy}
	o.kaynak = kaynakEkle(o, seviyeOkuyucu, onYukleyiciKapatici(oy))
	return o
}

// onYukleyiciKapatici önden yükleyicinin kaynak kaydındaki kapatıcısıdır: önden yükleyici,
// gorutini durduktan sonra örnekleyiciyi/okuyucuyu (ve açık parçaları) kapatır.
func onYukleyiciKapatici(oy *dataload.OnYukleyici) func(time.Duration) error {
	return func(sure time.Duration) error {
		if sure > 0 {
			return oy.KapatZamanli(sure)
		}
		return oy.Kapat()
	}
}

// sonraki bir sonraki toplu işi [girdi, hedef] olarak verir; sıralı okuyucu bitince yok.
func (o *Okuyucu) sonraki(m *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Ad(uyeSonraki)
	if err := bind.AdliDenetle(fn+"()", named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("%s() parametre almaz, %d verildi", fn, len(args))
	}
	var girdi, hedef []float32
	var b, t int
	var err error
	son := false
	// Bekleme ve dönüşüm yorumlayıcı kilidi bırakılarak yapılır; aynı okuyucuyu kullanan
	// eşzamanlı görevler okuyucunun kendi kilidiyle sıralanır.
	m.Blocking(func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.bitti {
			son = true
			return
		}
		err = o.oy.Al(&o.ti)
		if errors.Is(err, io.EOF) {
			o.bitti, son, err = true, true, nil
			_ = o.kaynak.Kapat(0)
			return
		}
		if err != nil {
			return
		}
		o.sayac++
		o.donem = o.ti.Durum.Donem
		o.konum = o.ti.Durum
		b, t = o.ti.B, o.ti.T
		girdi, hedef, err = ayir(&o.ti)
	})
	if err != nil {
		return nil, veriHatasi(fn, o.kume.yol, err)
	}
	if son {
		return interp.NULL, nil
	}
	gt := ml.YeniTam(tensor.New(girdi, b, t))
	ht := ml.YeniTam(tensor.New(hedef, b, t))
	return &interp.ListVal{Elements: []interp.Value{gt, ht}}, nil
}

var _ interp.Akis = (*Okuyucu)(nil)

// Sonraki sıralı okuyucuyu 'döngü iş içinde okuyucu { ... }' ile gezdirir (yurut.Akis):
// her tur .sonraki()'nin verdiği [girdi, hedef] listesidir, veri bitince döngü biter.
// Örnekleyici sonsuz ürettiği için döngüyle gezilmez.
func (o *Okuyucu) Sonraki(m *interp.VM) (interp.Value, bool, error) {
	if !o.sirali {
		return nil, false, fmt.Errorf("örnekleyici sonsuz toplu iş üretir, döngüyle gezilemez; adım sayısıyla döngü kurup .%s() kullanın (ya da .%s(...) ile sonlu bir okuyucu açın)", codes.Ad(uyeSonraki), codes.Ad(uyeSirali))
	}
	d, err := o.sonraki(m, nil, nil)
	if err != nil || d == interp.NULL {
		return nil, false, err
	}
	return d, true, nil
}

// ayir [B, T+1] toplu işini [B, T] girdi ve bir kaydırılmış [B, T] hedef dizilerine
// (float32) böler.
func ayir(ti *dataload.TopluIs) ([]float32, []float32, error) {
	n := ti.B * ti.T
	girdi := make([]float32, n)
	hedef := make([]float32, n)
	for i := 0; i < ti.B; i++ {
		satir := ti.Satir(i)
		for j := 0; j < ti.T; j++ {
			a, h := satir[j], satir[j+1]
			if a < 0 || a >= enBuyukTamFloat32 || h < 0 || h >= enBuyukTamFloat32 {
				x := a
				if a >= 0 && a < enBuyukTamFloat32 {
					x = h
				}
				return nil, nil, fmt.Errorf("parça kimliği %d tensörde tam olarak gösterilemez (kimlikler 0 ile %d arasında olmalı)", x, enBuyukTamFloat32-1)
			}
			girdi[i*ti.T+j] = float32(a)
			hedef[i*ti.T+j] = float32(h)
		}
	}
	return girdi, hedef, nil
}

// Package veri, .npy token parçaları için veri yükleyiciyi (tr_dili/internal/hesap/veriyolu) TR diline
// bağlar: veri_kümesi(dizin, önbellek:, kota_gb:), kümenin .örnekleyici(...) ve
// .sıralı(...) okuyucuları ve okuyucuların .sonraki() metodu.
//
// .sonraki() [girdi, hedef] listesi döndürür; ikisi de [toplu, bağlam] şeklinde, tam sayı
// değerli tensörlerdir ve gömme/çapraz_entropi'ye doğrudan verilebilir. Önden yükleme her
// zaman açıktır: sonraki toplu işler arka plandaki bir Go gorutininde hazırlanır. Bu
// gorutin TR kodu çalıştırmaz, yorumlayıcı kilidini hiç tutmaz; .sonraki() beklerken
// kilit bırakılır.
//
// Katman: motor (internal/motor); dil ve hesap çekirdeklerini içe aktarır, araçları bilmez.
package veri

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"tr_dili/internal/dil/kod"

	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/hesap/veriyolu"
	"tr_dili/internal/motor/ogrenme"
	"tr_dili/internal/motor/ortak/baglama"
)

// Tip adları.
var (
	KumeTipi        = string(yurut.TipAdi(kod.A231)) // VeriKümesi
	OrnekleyiciTipi = string(yurut.TipAdi(kod.A232)) // Örnekleyici
	SiraliTipi      = string(yurut.TipAdi(kod.A233)) // SıralıOkuyucu
)

// onYuklemeDerinligi arka planda hazır tutulan toplu iş sayısıdır.
const onYuklemeDerinligi = 4

// enBuyukTamFloat32 float32'nin tam gösterebildiği en büyük tam sayıdır (2^24).
const enBuyukTamFloat32 = 1 << 24

// Adlı argümanlar; adları kod listesindedir (internal/dil/kod).
var (
	adOnbellek = baglama.KodAd(kod.V900)
	adKota     = baglama.KodAd(kod.V901)
	adToplu    = baglama.KodAd(kod.V902)
	adBaglam   = baglama.KodAd(kod.V903)
	adTohum    = baglama.KodAd(kod.T903)
)

// Kaydet veri yerleşiklerini VM'ye kaydeder.
func Kaydet(v *yurut.VM) {
	v.RegisterBuiltinKod(kod.V100, func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		return baglama.Guvenli(kod.Ad(kod.V100), func() (yurut.Value, error) { return kumeAc(m, args, named) })
	})
}

// ----------------------------------------------------------------------------
// Küme
// ----------------------------------------------------------------------------

// Kume TR'de açılmış bir veri kümesidir.
type Kume struct {
	k      *veriyolu.Kume
	yol    string // kullanıcının verdiği yol
	ob     *veriyolu.Onbellek
	kaynak *kaynak // yalnızca önbellek varsa
}

// GoKumesi sarılı Go kümesini verir.
func (k *Kume) GoKumesi() *veriyolu.Kume { return k.k }

func (k *Kume) Type() yurut.ValueType { return yurut.ValueType(KumeTipi) }
func (k *Kume) IsTruthy() bool        { return true }

func (k *Kume) ozet() string {
	return fmt.Sprintf("%d veri dosyası, %s parça", k.k.ParcaSayisi(), baglama.KisaSayi(k.k.ToplamToken()))
}

func (k *Kume) Inspect() string {
	ek := ""
	if k.ob != nil {
		ek = fmt.Sprintf(", önbellek %s", baglama.BaytMetni(k.ob.Kota()))
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
		satirlar = append(satirlar, []string{p.Ad, fmt.Sprint(p.Token), baglama.BaytMetni(p.Bayt)})
	}
	return []string{"Veri dosyası", "Parça", "Boyut"}, satirlar, nil
}

// Veri kümesi ve okuyucu üyeleri (adları kod listesinde, internal/dil/kod Y60x).
const (
	uyeOrnekleyici = kod.Y600
	uyeSirali      = kod.Y601
	uyeSonraki     = kod.Y602
)

var kumeUyeleri = kod.Adlar(uyeOrnekleyici, uyeSirali)

func (k *Kume) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	switch kd {
	case uyeOrnekleyici:
		return metot(kod.Ad(uyeOrnekleyici), k.ornekleyici), nil
	case uyeSirali:
		return metot(kod.Ad(uyeSirali), k.sirali), nil
	}
	return nil, baglama.UyeHatasi("Veri kümesi", prop, kumeUyeleri)
}

func metot(ad string, fn yurut.BuiltinFunc) *yurut.BuiltinVal {
	return &yurut.BuiltinVal{Name: ad, Fn: func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		return baglama.Guvenli(ad, func() (yurut.Value, error) { return fn(m, args, named) })
	}}
}

// onbellekDizini önbellek: adlı argümanının (verildiyse) çözülmüş klasör yoludur.
func onbellekDizini(m *yurut.VM, fn string, named map[string]yurut.Value) (string, error) {
	v, ok := baglama.AdliAl(named, adOnbellek)
	if !ok {
		return "", nil
	}
	d, ok, err := yurut.YolArgumani(fn, v)
	if !ok || d == "" {
		return "", fmt.Errorf("%s: %s yerel bir klasör yolu (metin) olmalıdır, ör. %s: \"onbellek/veri\"", fn, adOnbellek.TR, adOnbellek.TR)
	}
	if err != nil {
		return "", err
	}
	// Göreli yol, diğer yollar gibi tek kuralla çözülür (16.8): önce çalışma
	// klasörü, orada yoksa programın klasörü.
	return baglama.YolCoz(m, d), nil
}

func kumeAc(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.V100)
	if err := baglama.AdliDenetle(fn+"()", named, adOnbellek, adKota); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(dizin) 1 parametre bekler (.npy veri dosyalarının klasörü ya da tek bir .npy dosyası), %d verildi", fn, len(args))
	}
	ham, ok, yolHata := yurut.YolArgumani(fn, args[0])
	if !ok {
		return nil, fmt.Errorf("%s: dizin bir metin olmalıdır, %s verildi", fn, args[0].Type())
	}
	if yolHata != nil {
		return nil, yolHata
	}
	yol, bakilanlar := baglama.YolCozBakilan(m, ham)

	obDizin, obHata := onbellekDizini(m, fn, named)
	if obHata != nil {
		return nil, obHata
	}
	var kota int64
	kotaV, kotaVar := baglama.AdliAl(named, adKota)
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

	var k *veriyolu.Kume
	var ob *veriyolu.Onbellek
	var err error
	m.Blocking(func() {
		// Önbelleksiz okumada parçalar belleğe eşlenmez: her pencere ReadAt ile okunur.
		// Ağ ya da akışlı bulut sürücülerinde yalnızca istenen baytlar okunur; yerel
		// diskte de bir toplu iş başına birkaç düzine küçük okuma eğitim adımının yanında
		// önemsizdir. Önbellekteki yerel kopyalar eşlenerek okunur.
		sec := veriyolu.KumeSecenekleri{Parca: veriyolu.ParcaSecenekleri{EslemeKapali: obDizin == ""}}
		if obDizin != "" && kotaVar {
			ob, err = veriyolu.OnbellekAc(veriyolu.OnbellekSecenekleri{Dizin: obDizin, Kota: kota})
			if err != nil {
				return
			}
			sec.Onbellek = ob
		}
		k, err = veriyolu.KumeAc(yol, sec)
		if err != nil && ob != nil {
			_ = ob.Kapat()
			ob = nil
		}
	})
	if err != nil {
		return nil, veriHatasi(fn, ham, err, yurut.BakilanYerler(bakilanlar))
	}
	if obDizin != "" && !kotaVar {
		gereken := enBuyukParca(k) * int64(veriyolu.VarsayilanGrupBoyu+veriyolu.VarsayilanIleriBak)
		return nil, fmt.Errorf("%s: %s: verildiğinde %s: de verilmelidir (önbellek klasörünün kullanabileceği en çok disk alanı, GB). "+
			"Bu kümenin örnekleyicisi aynı anda %d veri dosyasını yerelde tutar; en büyük veri dosyası %s olduğundan en az %s: %d gerekir",
			fn, adOnbellek.TR, adKota.TR, veriyolu.VarsayilanGrupBoyu+veriyolu.VarsayilanIleriBak, baglama.BaytMetni(enBuyukParca(k)), adKota.TR, int64(math.Ceil(float64(gereken)/(1<<30))))
	}
	tk := &Kume{k: k, yol: ham, ob: ob}
	if ob != nil {
		tk.kaynak = kaynakEkle(tk, seviyeOnbellek, func(time.Duration) error { return ob.Kapat() })
	}
	return tk, nil
}

func enBuyukParca(k *veriyolu.Kume) int64 {
	var b int64
	for _, p := range k.Parcalar() {
		b = max(b, p.Bayt)
	}
	return b
}

func sayi(v yurut.Value) (float64, bool) {
	switch n := v.(type) {
	case *yurut.IntVal:
		return float64(n.Val), true
	case *yurut.FloatVal:
		return n.Val, true
	}
	return 0, false
}

// veriHatasi Go veri paketinin hatasını yol gösterici bir TR hatasına çevirir; ekler
// küme bulunamadığında mesaja eklenir (ör. bakılan yerler).
func veriHatasi(fn, yol string, err error, ekler ...string) error {
	var bh *veriyolu.BicimHatasi
	switch {
	case errors.As(err, &bh):
		dosya := bh.Yol
		if dosya == "" {
			dosya = yol
		}
		return fmt.Errorf("%s: %q geçerli bir .npy veri dosyası değil: %s. Veri dosyaları standart .npy biçiminde tek boyutlu, küçük uçlu uint16, uint32 ya da int32 dizileri olmalıdır",
			fn, filepath.ToSlash(dosya), bh.Neden)
	case errors.Is(err, veriyolu.ErrOnbellekYetersiz):
		return fmt.Errorf("%s: önbellek kotası yetersiz: %s", fn, temizle(strings.TrimPrefix(err.Error(), veriyolu.ErrOnbellekYetersiz.Error()+": ")))
	case errors.Is(err, veriyolu.ErrKapali):
		return fmt.Errorf("%s: okuyucu kapatılmış (program sonlanıyor); yeni bir okuyucu oluşturun", fn)
	case errors.Is(err, veriyolu.ErrOnbellekKilitli):
		return fmt.Errorf("%s: %s", fn, temizle(strings.TrimPrefix(err.Error(), veriyolu.ErrOnbellekKilitli.Error()+": ")))
	case baglama.YokMu(err) && strings.Contains(err.Error(), "küme açılamadı"):
		return turkce.DosyaHatasi(fn, yol, "açılamadı", err,
			append([]string{".npy veri dosyalarının bulunduğu klasörün (ya da tek bir .npy dosyasının) yolunu verin"}, ekler...)...)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		// İzin, kilitli dosya, disk hatası...: işletim sisteminin metni yerine Türkçe neden
		return turkce.DosyaHatasi(fn, filepath.ToSlash(pe.Path), "okunamadı", err)
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
	oy     *veriyolu.OnYukleyici
	ti     veriyolu.TopluIs
	sayac  int
	donem  int
	bitti  bool
	kaynak *kaynak
}

func (o *Okuyucu) Type() yurut.ValueType {
	if o.sirali {
		return yurut.ValueType(SiraliTipi)
	}
	return yurut.ValueType(OrnekleyiciTipi)
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
	return nil, nil, fmt.Errorf("okuyucu tablo olarak açılamaz; .%s() ile alınan girdi ve hedef tensörlerini inceleyin", kod.Ad(uyeSonraki))
}

func (o *Okuyucu) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	if kd == uyeSonraki {
		return metot(kod.Ad(uyeSonraki), o.sonraki), nil
	}
	ne := OrnekleyiciTipi
	if o.sirali {
		ne = "Sıralı okuyucu"
	}
	return nil, baglama.UyeHatasi(ne, prop, kod.Adlar(uyeSonraki))
}

// okuyucuArgumanlari toplu, bağlam ve (örnekleyicide) tohum argümanlarını okur. Konumlu
// olarak da aynı sırayla verilebilirler.
func okuyucuArgumanlari(fn string, args []yurut.Value, named map[string]yurut.Value, tohumlu bool) (b, t int, tohum uint64, err error) {
	adlar := []baglama.Ad{adToplu, adBaglam}
	if tohumlu {
		adlar = append(adlar, adTohum)
	}
	if err = baglama.AdliDenetle(fn+"()", named, adlar...); err != nil {
		return
	}
	if len(args) > len(adlar) {
		err = fmt.Errorf("%s() en çok %d parametre alır, %d verildi", fn, len(adlar), len(args))
		return
	}
	degerler := make([]yurut.Value, len(adlar))
	for i, a := range adlar {
		v, adli := baglama.AdliAl(named, a)
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
	pozitif := func(ad string, v yurut.Value) (int, error) {
		if v == nil {
			return 0, fmt.Errorf("%s: %s: verilmedi; ör. %s", fn, ad, ornek)
		}
		n, ok := baglama.TamSayi(v)
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
	if int64(b)*int64(t+1) > int64(yurut.MaxCollectionSize) {
		err = fmt.Errorf("%s: %s × %s (%d × %d) çok büyük; en fazla %d parça", fn, adToplu.TR, adBaglam.TR, b, t, yurut.MaxCollectionSize)
		return
	}
	if tohumlu && degerler[2] != nil {
		n, ok := baglama.TamSayi(degerler[2])
		if !ok || n < 0 {
			err = fmt.Errorf("%s: %s negatif olmayan bir tam sayı olmalıdır, %s verildi", fn, adTohum.TR, degerler[2].Inspect())
			return
		}
		tohum = uint64(n)
	}
	return
}

func (k *Kume) ornekleyici(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(uyeOrnekleyici)
	b, t, tohum, err := okuyucuArgumanlari(fn, args, named, true)
	if err != nil {
		return nil, err
	}
	var ok *veriyolu.Ornekleyici
	m.Blocking(func() {
		ok, err = k.k.Ornekleyici(veriyolu.OrnekleyiciSecenekleri{B: b, T: t, Tohum: tohum})
	})
	if err != nil {
		return nil, okuyucuHatasi(fn, k, t, err)
	}
	return k.okuyucuKur(false, b, t, tohum, ok), nil
}

func (k *Kume) sirali(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(uyeSirali)
	b, t, _, err := okuyucuArgumanlari(fn, args, named, false)
	if err != nil {
		return nil, err
	}
	var so *veriyolu.SiraliOkuyucu
	m.Blocking(func() {
		so, err = k.k.Sirali(veriyolu.SiraliSecenekleri{B: b, T: t})
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
	if errors.Is(err, veriyolu.ErrOnbellekYetersiz) {
		return fmt.Errorf("%s: önbellek kotası yetersiz: %s", fn, temizle(strings.TrimPrefix(err.Error(), veriyolu.ErrOnbellekYetersiz.Error()+": ")))
	}
	return veriHatasi(fn, k.yol, err)
}

func (k *Kume) okuyucuKur(sirali bool, b, t int, tohum uint64, kaynakGo veriyolu.Kaynak) *Okuyucu {
	oy := veriyolu.OnYukle(context.Background(), kaynakGo, onYuklemeDerinligi)
	o := &Okuyucu{sirali: sirali, kume: k, b: b, t: t, tohum: tohum, oy: oy}
	o.kaynak = kaynakEkle(o, seviyeOkuyucu, func(sure time.Duration) error {
		// Önden yükleyici, gorutini durduktan sonra örnekleyiciyi/okuyucuyu (ve açık
		// parçaları) kapatır.
		if sure > 0 {
			return oy.KapatZamanli(sure)
		}
		return oy.Kapat()
	})
	return o
}

// sonraki bir sonraki toplu işi [girdi, hedef] olarak verir; sıralı okuyucu bitince yok.
func (o *Okuyucu) sonraki(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(uyeSonraki)
	if err := baglama.AdliDenetle(fn+"()", named); err != nil {
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
		b, t = o.ti.B, o.ti.T
		girdi, hedef, err = ayir(&o.ti)
	})
	if err != nil {
		return nil, veriHatasi(fn, o.kume.yol, err)
	}
	if son {
		return yurut.NULL, nil
	}
	gt := ogrenme.YeniTam(tensor.New(girdi, b, t))
	ht := ogrenme.YeniTam(tensor.New(hedef, b, t))
	return &yurut.ListVal{Elements: []yurut.Value{gt, ht}}, nil
}

var _ yurut.Akis = (*Okuyucu)(nil)

// Sonraki sıralı okuyucuyu 'döngü iş içinde okuyucu { ... }' ile gezdirir (yurut.Akis):
// her tur .sonraki()'nin verdiği [girdi, hedef] listesidir, veri bitince döngü biter.
// Örnekleyici sonsuz ürettiği için döngüyle gezilmez.
func (o *Okuyucu) Sonraki(m *yurut.VM) (yurut.Value, bool, error) {
	if !o.sirali {
		return nil, false, fmt.Errorf("örnekleyici sonsuz toplu iş üretir, döngüyle gezilemez; adım sayısıyla döngü kurup .%s() kullanın (ya da .%s(...) ile sonlu bir okuyucu açın)", kod.Ad(uyeSonraki), kod.Ad(uyeSirali))
	}
	d, err := o.sonraki(m, nil, nil)
	if err != nil || d == yurut.NULL {
		return nil, false, err
	}
	return d, true, nil
}

// ayir [B, T+1] toplu işini [B, T] girdi ve bir kaydırılmış [B, T] hedef dizilerine
// (float32) böler.
func ayir(ti *veriyolu.TopluIs) ([]float32, []float32, error) {
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

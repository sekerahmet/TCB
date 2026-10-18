// Package veri, .npy token parçaları için veri yükleyiciyi (tr_dili/pkg/veri) TR diline
// bağlar: veri_kümesi(dizin, önbellek:, kota_gb:), kümenin .örnekleyici(...) ve
// .sıralı(...) okuyucuları ve okuyucuların .sonraki() metodu.
//
// .sonraki() [girdi, hedef] listesi döndürür; ikisi de [toplu, bağlam] şeklinde, tam sayı
// değerli tensörlerdir ve gömme/çapraz_entropi'ye doğrudan verilebilir. Önden yükleme her
// zaman açıktır: sonraki toplu işler arka plandaki bir Go gorutininde hazırlanır. Bu
// gorutin TR kodu çalıştırmaz, yorumlayıcı kilidini hiç tutmaz; .sonraki() beklerken
// kilit bırakılır.
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
	"tr_dili/pkg/kod"

	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/motor/tensor"
	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/turkce"
	dv "tr_dili/pkg/veri"
	"tr_dili/pkg/vm"
)

// Tip adları.
var (
	KumeTipi        = string(vm.TipAdi(kod.A231)) // VeriKümesi
	OrnekleyiciTipi = string(vm.TipAdi(kod.A232)) // Örnekleyici
	SiraliTipi      = string(vm.TipAdi(kod.A233)) // SıralıOkuyucu
)

// onYuklemeDerinligi arka planda hazır tutulan toplu iş sayısıdır.
const onYuklemeDerinligi = 4

// enBuyukTamFloat32 float32'nin tam gösterebildiği en büyük tam sayıdır (2^24).
const enBuyukTamFloat32 = 1 << 24

// Adlı argümanlar; adları kod listesindedir (pkg/kod).
var (
	adOnbellek = baglama.KodAd(kod.V900)
	adKota     = baglama.KodAd(kod.V901)
	adToplu    = baglama.KodAd(kod.V902)
	adBaglam   = baglama.KodAd(kod.V903)
	adTohum    = baglama.KodAd(kod.T903)
)

// Kaydet veri yerleşiklerini VM'ye kaydeder.
func Kaydet(v *vm.VM) {
	v.RegisterBuiltinKod(kod.V100, func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		return baglama.Guvenli("veri_kümesi", func() (vm.Value, error) { return kumeAc(m, args, named) })
	})
}

// ----------------------------------------------------------------------------
// Küme
// ----------------------------------------------------------------------------

// Kume TR'de açılmış bir veri kümesidir.
type Kume struct {
	k      *dv.Kume
	yol    string // kullanıcının verdiği yol
	ob     *dv.Onbellek
	kaynak *kaynak // yalnızca önbellek varsa
}

// GoKumesi sarılı Go kümesini verir.
func (k *Kume) GoKumesi() *dv.Kume { return k.k }

func (k *Kume) Type() vm.ValueType { return vm.ValueType(KumeTipi) }
func (k *Kume) IsTruthy() bool     { return true }

func (k *Kume) ozet() string {
	return fmt.Sprintf("%d veri dosyası, %s parça", k.k.ParcaSayisi(), baglama.KisaSayi(k.k.ToplamToken()))
}

func (k *Kume) Inspect() string {
	ek := ""
	if k.ob != nil {
		ek = fmt.Sprintf(", önbellek %s", baglama.BaytMetni(k.ob.Kota()))
	}
	return fmt.Sprintf("VeriKümesi(%s%s)", k.ozet(), ek)
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

// Veri kümesi ve okuyucu üyeleri (adları kod listesinde, pkg/kod Y60x).
const (
	uyeOrnekleyici = kod.Y600
	uyeSirali      = kod.Y601
	uyeSonraki     = kod.Y602
)

var kumeUyeleri = kod.Adlar(uyeOrnekleyici, uyeSirali)

func (k *Kume) GetProperty(kd kod.Kod, prop string) (vm.Value, error) {
	switch kd {
	case uyeOrnekleyici:
		return metot(kod.Ad(uyeOrnekleyici), k.ornekleyici), nil
	case uyeSirali:
		return metot(kod.Ad(uyeSirali), k.sirali), nil
	}
	return nil, uyeHatasi("Veri kümesi", prop, kumeUyeleri)
}

func uyeHatasi(ne, prop string, uyeler []string) error {
	oneri := ""
	if o := turkce.EnYakin(prop, uyeler); o != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", o)
	} else {
		oneri = fmt.Sprintf(" (üyeler: %s)", strings.Join(uyeler, ", "))
	}
	return fmt.Errorf("%s üzerinde %q özelliği ya da metodu bulunamadı%s", ne, prop, oneri)
}

func metot(ad string, fn vm.BuiltinFunc) *vm.BuiltinVal {
	return &vm.BuiltinVal{Name: ad, Fn: func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		return baglama.Guvenli(ad, func() (vm.Value, error) { return fn(m, args, named) })
	}}
}

func kumeAc(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const fn = "veri_kümesi"
	if err := baglama.AdliDenetle(fn+"()", named, adOnbellek, adKota); err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(dizin) 1 parametre bekler (.npy veri dosyalarının klasörü ya da tek bir .npy dosyası), %d verildi", fn, len(args))
	}
	s, ok := args[0].(*vm.StringVal)
	if !ok {
		return nil, fmt.Errorf("%s: dizin bir metin olmalıdır, %s verildi", fn, args[0].Type())
	}
	yol, bakilanlar := baglama.YolCozBakilan(m, s.Val)

	obDizin := ""
	if v, ok := baglama.AdliAl(named, adOnbellek); ok {
		d, ok := v.(*vm.StringVal)
		if !ok || d.Val == "" {
			return nil, fmt.Errorf("%s: önbellek yerel bir klasör yolu (metin) olmalıdır, ör. önbellek: \"onbellek/veri\"", fn)
		}
		// Göreli yol, diğer yollar gibi tek kuralla çözülür (16.8): önce çalışma
		// klasörü, orada yoksa programın klasörü.
		obDizin = baglama.YolCoz(m, d.Val)
	}
	var kota int64
	kotaV, kotaVar := baglama.AdliAl(named, adKota)
	if kotaVar {
		if obDizin == "" {
			return nil, fmt.Errorf("%s: kota_gb yalnızca önbellek: ile birlikte kullanılır (önbellek klasörünün en çok kaç GB kullanacağı)", fn)
		}
		gb, ok := sayi(kotaV)
		if !ok || gb <= 0 || math.IsInf(gb, 0) || math.IsNaN(gb) {
			return nil, fmt.Errorf("%s: kota_gb pozitif bir sayı olmalıdır (ör. kota_gb: 20), %s verildi", fn, kotaV.Inspect())
		}
		// int64 bayt sayısına sığmayan kota (≈ 8,6 milyar GB) taşıp negatif olmasın.
		if gb >= float64(math.MaxInt64)/(1<<30) {
			return nil, fmt.Errorf("%s: kota_gb çok büyük (%s); en çok %d GB verilebilir", fn, kotaV.Inspect(), int64(math.MaxInt64>>30))
		}
		kota = max(int64(gb*(1<<30)), 1)
	}

	var k *dv.Kume
	var ob *dv.Onbellek
	var err error
	m.Blocking(func() {
		// Önbelleksiz okumada parçalar belleğe eşlenmez: her pencere ReadAt ile okunur.
		// Ağ ya da akışlı bulut sürücülerinde yalnızca istenen baytlar okunur; yerel
		// diskte de bir toplu iş başına birkaç düzine küçük okuma eğitim adımının yanında
		// önemsizdir. Önbellekteki yerel kopyalar eşlenerek okunur.
		sec := dv.KumeSecenekleri{Parca: dv.ParcaSecenekleri{EslemeKapali: obDizin == ""}}
		if obDizin != "" && kotaVar {
			ob, err = dv.OnbellekAc(dv.OnbellekSecenekleri{Dizin: obDizin, Kota: kota})
			if err != nil {
				return
			}
			sec.Onbellek = ob
		}
		k, err = dv.KumeAc(yol, sec)
		if err != nil && ob != nil {
			_ = ob.Kapat()
			ob = nil
		}
	})
	if err != nil {
		return nil, veriHatasi(fn, s.Val, err, vm.BakilanYerler(bakilanlar))
	}
	if obDizin != "" && !kotaVar {
		gereken := enBuyukParca(k) * int64(dv.VarsayilanGrupBoyu+dv.VarsayilanIleriBak)
		return nil, fmt.Errorf("%s: önbellek: verildiğinde kota_gb: de verilmelidir (önbellek klasörünün kullanabileceği en çok disk alanı, GB). "+
			"Bu kümenin örnekleyicisi aynı anda %d veri dosyasını yerelde tutar; en büyük veri dosyası %s olduğundan en az kota_gb: %d gerekir",
			fn, dv.VarsayilanGrupBoyu+dv.VarsayilanIleriBak, baglama.BaytMetni(enBuyukParca(k)), int64(math.Ceil(float64(gereken)/(1<<30))))
	}
	tk := &Kume{k: k, yol: s.Val, ob: ob}
	if ob != nil {
		tk.kaynak = kaynakEkle(tk, seviyeOnbellek, func(time.Duration) error { return ob.Kapat() })
	}
	return tk, nil
}

func enBuyukParca(k *dv.Kume) int64 {
	var b int64
	for _, p := range k.Parcalar() {
		b = max(b, p.Bayt)
	}
	return b
}

func sayi(v vm.Value) (float64, bool) {
	switch n := v.(type) {
	case *vm.IntVal:
		return float64(n.Val), true
	case *vm.FloatVal:
		return n.Val, true
	}
	return 0, false
}

// veriHatasi Go veri paketinin hatasını yol gösterici bir TR hatasına çevirir; ekler
// küme bulunamadığında mesaja eklenir (ör. bakılan yerler).
func veriHatasi(fn, yol string, err error, ekler ...string) error {
	var bh *dv.BicimHatasi
	switch {
	case errors.As(err, &bh):
		dosya := bh.Yol
		if dosya == "" {
			dosya = yol
		}
		return fmt.Errorf("%s: %q geçerli bir .npy veri dosyası değil: %s. Veri dosyaları standart .npy biçiminde tek boyutlu, küçük uçlu uint16, uint32 ya da int32 dizileri olmalıdır",
			fn, filepath.ToSlash(dosya), bh.Neden)
	case errors.Is(err, dv.ErrOnbellekYetersiz):
		return fmt.Errorf("%s: önbellek kotası yetersiz: %s", fn, temizle(strings.TrimPrefix(err.Error(), dv.ErrOnbellekYetersiz.Error()+": ")))
	case errors.Is(err, dv.ErrKapali):
		return fmt.Errorf("%s: okuyucu kapatılmış (program sonlanıyor); yeni bir okuyucu oluşturun", fn)
	case errors.Is(err, dv.ErrOnbellekKilitli):
		return fmt.Errorf("%s: %s", fn, temizle(strings.TrimPrefix(err.Error(), dv.ErrOnbellekKilitli.Error()+": ")))
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
	oy     *dv.OnYukleyici
	ti     dv.TopluIs
	sayac  int
	donem  int
	bitti  bool
	kaynak *kaynak
}

func (o *Okuyucu) Type() vm.ValueType {
	if o.sirali {
		return vm.ValueType(SiraliTipi)
	}
	return vm.ValueType(OrnekleyiciTipi)
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
		return fmt.Sprintf("SıralıOkuyucu(toplu: %d, bağlam: %d, %s)", o.b, o.t, durum)
	}
	return fmt.Sprintf("Örnekleyici(toplu: %d, bağlam: %d, tohum: %d, dönem: %d, %d toplu iş)", o.b, o.t, o.tohum, o.donem, o.sayac)
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
	return fmt.Sprintf("Örnekleyici (%d×%d, dönem %d, %d toplu iş)", o.b, o.t, o.donem, o.sayac)
}

// TabloVerisi okuyucuyu tablo olarak açmaz; toplu işler .sonraki() ile alınır.
func (o *Okuyucu) TabloVerisi() ([]string, [][]string, error) {
	return nil, nil, fmt.Errorf("okuyucu tablo olarak açılamaz; .sonraki() ile alınan girdi ve hedef tensörlerini inceleyin")
}

func (o *Okuyucu) GetProperty(kd kod.Kod, prop string) (vm.Value, error) {
	if kd == uyeSonraki {
		return metot(kod.Ad(uyeSonraki), o.sonraki), nil
	}
	ne := "Örnekleyici"
	if o.sirali {
		ne = "Sıralı okuyucu"
	}
	return nil, uyeHatasi(ne, prop, kod.Adlar(uyeSonraki))
}

// okuyucuArgumanlari toplu, bağlam ve (örnekleyicide) tohum argümanlarını okur. Konumlu
// olarak da aynı sırayla verilebilirler.
func okuyucuArgumanlari(fn string, args []vm.Value, named map[string]vm.Value, tohumlu bool) (b, t int, tohum uint64, err error) {
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
	degerler := make([]vm.Value, len(adlar))
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
	ornek := fmt.Sprintf("%s(toplu: 32, bağlam: 1024)", fn)
	if tohumlu {
		ornek = fmt.Sprintf("%s(toplu: 32, bağlam: 1024, tohum: 1)", fn)
	}
	pozitif := func(ad string, v vm.Value) (int, error) {
		if v == nil {
			return 0, fmt.Errorf("%s: %s: verilmedi; ör. %s", fn, ad, ornek)
		}
		n, ok := baglama.TamSayi(v)
		if !ok || n <= 0 || n > 1<<24 {
			return 0, fmt.Errorf("%s: %s pozitif bir tam sayı olmalıdır, %s verildi", fn, ad, v.Inspect())
		}
		return int(n), nil
	}
	if b, err = pozitif("toplu", degerler[0]); err != nil {
		return
	}
	if t, err = pozitif("bağlam", degerler[1]); err != nil {
		return
	}
	if int64(b)*int64(t+1) > int64(vm.MaxCollectionSize) {
		err = fmt.Errorf("%s: toplu × bağlam (%d × %d) çok büyük; en fazla %d parça", fn, b, t, vm.MaxCollectionSize)
		return
	}
	if tohumlu && degerler[2] != nil {
		n, ok := baglama.TamSayi(degerler[2])
		if !ok || n < 0 {
			err = fmt.Errorf("%s: tohum negatif olmayan bir tam sayı olmalıdır, %s verildi", fn, degerler[2].Inspect())
			return
		}
		tohum = uint64(n)
	}
	return
}

func (k *Kume) ornekleyici(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const fn = "örnekleyici"
	b, t, tohum, err := okuyucuArgumanlari(fn, args, named, true)
	if err != nil {
		return nil, err
	}
	var ok *dv.Ornekleyici
	m.Blocking(func() {
		ok, err = k.k.Ornekleyici(dv.OrnekleyiciSecenekleri{B: b, T: t, Tohum: tohum})
	})
	if err != nil {
		return nil, okuyucuHatasi(fn, k, t, err)
	}
	return k.okuyucuKur(false, b, t, tohum, ok), nil
}

func (k *Kume) sirali(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const fn = "sıralı"
	b, t, _, err := okuyucuArgumanlari(fn, args, named, false)
	if err != nil {
		return nil, err
	}
	var so *dv.SiraliOkuyucu
	m.Blocking(func() {
		so, err = k.k.Sirali(dv.SiraliSecenekleri{B: b, T: t})
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
		return fmt.Errorf("%s: bağlam: %d için her pencere %d parça ister, ama en uzun veri dosyası %d parça; bağlamı küçültün", fn, t, t+1, enUzun)
	}
	if errors.Is(err, dv.ErrOnbellekYetersiz) {
		return fmt.Errorf("%s: önbellek kotası yetersiz: %s", fn, temizle(strings.TrimPrefix(err.Error(), dv.ErrOnbellekYetersiz.Error()+": ")))
	}
	return veriHatasi(fn, k.yol, err)
}

func (k *Kume) okuyucuKur(sirali bool, b, t int, tohum uint64, kaynakGo dv.Kaynak) *Okuyucu {
	oy := dv.OnYukle(context.Background(), kaynakGo, onYuklemeDerinligi)
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
func (o *Okuyucu) sonraki(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if err := baglama.AdliDenetle("sonraki()", named); err != nil {
		return nil, err
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("sonraki() parametre almaz, %d verildi", len(args))
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
		fn := "sonraki"
		return nil, veriHatasi(fn, o.kume.yol, err)
	}
	if son {
		return vm.NULL, nil
	}
	gt := tensor.YeniTam(ct.New(girdi, b, t))
	ht := tensor.YeniTam(ct.New(hedef, b, t))
	return &vm.ListVal{Elements: []vm.Value{gt, ht}}, nil
}

// ayir [B, T+1] toplu işini [B, T] girdi ve bir kaydırılmış [B, T] hedef dizilerine
// (float32) böler.
func ayir(ti *dv.TopluIs) ([]float32, []float32, error) {
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

package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"tr_dili/pkg/motor/internal/baglama"
	"tr_dili/pkg/sozluk"
	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/vm"
)

// Kaydet tensör yerleşiklerini VM'ye kaydeder. Adlar sözlüktedir (pkg/sozluk); burada
// yalnızca kodlar geçer. Aynı adlı çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama,
// min, maks, karekök) sarılır: argüman tensörse tensör yolu, değilse eski yerleşik çalışır.
func Kaydet(v *vm.VM) {
	// --- Oluşturucular ---
	sar(v, sozluk.T+100, yeniYerlesik(tensorYerlesik))
	sar(v, sozluk.T+101, yeniYerlesik(sekilliOlusturucu(sozluk.T+101, true, func(sh []int, _ map[string]vm.Value) (*ct.Tensor, error) {
		return ct.Zeros(sh...), nil
	})))
	sar(v, sozluk.T+102, yeniYerlesik(sekilliOlusturucu(sozluk.T+102, true, func(sh []int, _ map[string]vm.Value) (*ct.Tensor, error) {
		return ct.Ones(sh...), nil
	})))
	sar(v, sozluk.T+103, yeniYerlesik(sekilliOlusturucu(sozluk.T+103, false, rastgeleNormal)))

	// --- Matris (2 boyutlu tensör) oluşturucusu (matris.go) ---
	sar(v, sozluk.T+111, yeniYerlesik(birimMatris))

	// --- Görünüm ---
	sar(v, sozluk.T+120, yeniYerlesik(devrikYerlesik))
	sar(v, sozluk.T+121, yeniYerlesik(birlestirYerlesik)) // model_araclari.go

	// --- Eleman bazlı matematik ---
	birli(v, sozluk.T+200, (*ct.Tensor).Relu, relu64)
	birli(v, sozluk.T+201, (*ct.Tensor).Sigmoid, sigmoid64)
	birli(v, sozluk.T+202, (*ct.Tensor).Tanh, math.Tanh)
	birli(v, sozluk.T+203, (*ct.Tensor).Gelu, gelu64)
	birli(v, sozluk.G+404, (*ct.Tensor).Sqrt, nil) // karekök
	birli(v, sozluk.T+220, (*ct.Tensor).Exp, math.Exp)
	birli(v, sozluk.T+221, (*ct.Tensor).Log, math.Log)
	birli(v, sozluk.T+222, (*ct.Tensor).Sin, math.Sin)
	birli(v, sozluk.T+223, (*ct.Tensor).Cos, math.Cos)

	// --- Softmax ---
	softmaxKaydet(v, sozluk.T+210, false)
	softmaxKaydet(v, sozluk.T+211, true)

	// --- İndirgemeler (toplam, ortalama, maks, min) ---
	indirgeme(v, sozluk.G+400, func(t *ct.Tensor, e int, tum, koru bool) *ct.Tensor {
		if tum {
			return t.SumAll()
		}
		return t.Sum(e, koru)
	})
	indirgeme(v, sozluk.G+410, func(t *ct.Tensor, e int, tum, koru bool) *ct.Tensor {
		if tum {
			return t.MeanAll()
		}
		return t.Mean(e, koru)
	})
	indirgeme(v, sozluk.G+402, func(t *ct.Tensor, e int, tum, koru bool) *ct.Tensor {
		if tum {
			return t.Reshape(-1).Max(0, false)
		}
		return t.Max(e, koru)
	})
	indirgeme(v, sozluk.G+401, func(t *ct.Tensor, e int, tum, koru bool) *ct.Tensor {
		if tum {
			return enKucukTum(t)
		}
		return t.Neg().Max(e, koru).Neg()
	})

	// --- Sinir ağı ---
	sar(v, sozluk.T+500, yeniYerlesik(gomme))
	sar(v, sozluk.T+510, yeniYerlesik(caprazEntropi))
	sar(v, sozluk.T+501, yeniYerlesik(katmanNormu))
	sar(v, sozluk.T+502, yeniYerlesik(rmsNormu))
	sar(v, sozluk.T+511, yeniYerlesik(kayipMSE))
	sar(v, sozluk.T+503, yeniYerlesik(dikkatYerlesik)) // model_araclari.go
	sar(v, sozluk.T+530, yeniYerlesik(parametrelerYerlesik))
	sar(v, sozluk.T+611, yeniYerlesik(orneklemeYerlesik))

	// --- Hazır sinir ağı modeli (sinir_agi.go) ---
	sar(v, sozluk.T+520, yeniYerlesik(sinirAgiKur))
	sar(v, sozluk.T+521, yeniYerlesik(modelYukle))

	// --- Türev ---
	sar(v, sozluk.T+300, yeniYerlesik(geriYayilim))
	sar(v, sozluk.T+301, yeniYerlesik(turevsiz))

	// --- Veri türleri (veri_turu.go) ---
	sar(v, sozluk.T+710, yeniYerlesik(karisikHassasiyet))

	// --- İyileştiriciler ---
	sar(v, sozluk.T+400, yeniYerlesik(adamWKur))
	sar(v, sozluk.T+401, yeniYerlesik(sgdKur))
	sar(v, sozluk.T+402, yeniYerlesik(gradyanKirp))

	// --- İndeks, kaydetme ve yükleme ---
	sar(v, sozluk.T+230, yeniYerlesik(maksIndeks))
	sar(v, sozluk.T+700, yeniYerlesik(tensorKaydet))
	sar(v, sozluk.T+701, yeniYerlesik(tensorYukle))
}

// enKucukTum tüm elemanların en küçüğünü 0 boyutlu tensör olarak verir. Çekirdekte Min
// işlemi yoktur; türev gerekiyorsa -maks(-x) ile (gradyan doğru elemana gider) hesaplanır.
// Türev gerekmiyorsa ara tensör ayırmadan konakta tek geçişle bulunur.
func enKucukTum(t *ct.Tensor) *ct.Tensor {
	if t.RequiresGrad() && ct.GradEnabled() {
		return t.Neg().Reshape(-1).Max(0, false).Neg()
	}
	veri := t.Data()
	m := veri[0]
	for _, x := range veri[1:] {
		if x < m || x != x { // NaN yayılır (maks ile tutarlı)
			m = x
			if x != x {
				break
			}
		}
	}
	return ct.Scalar(m).To(t.Device())
}

// sar kodun bütün yazımlarına (ad ve sözlüğe göre Türkçe karaktersiz biçimi) aynı yeni
// yerleşiği bağlar. Eski yerleşik (varsa) bu koddan okunur ve yeni fonksiyona verilir.
func sar(v *vm.VM, kod sozluk.Kod, kur func(eski vm.BuiltinFunc) vm.BuiltinFunc) {
	var eski vm.BuiltinFunc
	if b := v.Yerlesik(kod); b != nil {
		eski = b.Fn
	}
	v.YerlesikTanimla(kod, &vm.BuiltinVal{Name: sozluk.Ad(kod), Fn: kur(eski)})
}

// eskiCagir eski yerleşiği çağırır; yoksa argümanın tensör olmadığını bildirir.
func eskiCagir(ad string, eski vm.BuiltinFunc, m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if eski != nil {
		return eski(m, args, named)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("%s() en az 1 parametre bekler", ad)
	}
	return nil, fmt.Errorf("%s() bir tensör bekler, %s verildi", ad, tipAdi(args[0]))
}

// yeniYerlesik eski karşılığı olmayan bir yerleşik kurar.
func yeniYerlesik(fn vm.BuiltinFunc) func(vm.BuiltinFunc) vm.BuiltinFunc {
	return func(vm.BuiltinFunc) vm.BuiltinFunc { return fn }
}

func tensorMu(args []vm.Value) bool {
	for _, a := range args {
		if _, ok := a.(*Deger); ok {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// Oluşturucular
// ----------------------------------------------------------------------------

// turevIzle türev_izle adlı argümanını (ya da verilen konumlu değeri) okur.
func turevIzle(named map[string]vm.Value, konumlu vm.Value, varsayilan bool) bool {
	if konumlu != nil {
		varsayilan = konumlu.IsTruthy()
	}
	if v, ok := baglama.AdliAl(named, adTurevIzle); ok {
		return v.IsTruthy()
	}
	return varsayilan
}

// tensorYerlesik tensör(veri, türev_izle: yanlış, tür: "float32") yerleşiğidir. Veri tek
// sayı (0 boyutlu tensör), iç içe sayı listesi ya da tensör olabilir. tür verilmezse
// listeden float32, tensörden kaynağın türünde tensör oluşur.
func tensorYerlesik(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("tensör(veri, türev_izle: doğru) en az 1 parametre bekler")
	}
	if len(args) > 2 {
		return nil, fmt.Errorf("tensör(veri, türev_izle: doğru) en fazla 2 parametre alır, %d verildi", len(args))
	}
	if err := baglama.AdliDenetle("tensör()", named, adTurevIzle, adTur); err != nil {
		return nil, err
	}
	var konumlu vm.Value
	if len(args) == 2 {
		konumlu = args[1]
	}
	izle := turevIzle(named, konumlu, false)
	var t *ct.Tensor
	if d, ok := args[0].(*Deger); ok {
		dt, err := adliTur("tensör", named, d.t.DType())
		if err != nil {
			return nil, err
		}
		// Var olan tensörden çizgeden bağımsız, kendi verisine sahip yeni bir yaprak.
		t, err = guvenli(func() *ct.Tensor {
			if dt == ct.Float32 {
				return ct.NewOn(d.t.Device(), d.t.Data(), d.t.Shape()...)
			}
			return ct.NewOf(dt, d.t.Data(), d.t.Shape()...).To(d.t.Device())
		})
		if err != nil {
			return nil, err
		}
	} else {
		dt, err := adliTur("tensör", named, ct.Float32)
		if err != nil {
			return nil, err
		}
		t, err = tensoreCevir("tensör", args[0])
		if err != nil {
			return nil, err
		}
		if t, err = tureCevir(t, dt); err != nil {
			return nil, err
		}
	}
	t.SetRequiresGrad(izle)
	return Yeni(t), nil
}

// sekilliOlusturucu şekli liste olarak alan oluşturucular (sıfırlar([2, 3]) ...) içindir.
// matrisBicimi doğruysa tam sayılar eski matris biçimidir: sıfırlar(2, 3) 2 boyutlu,
// sıfırlar(n) [1, n] şeklinde tensör verir. Değilse tam sayılar da şekil sayılır:
// rastgele_normal(2, 3, 4).
func sekilliOlusturucu(kod sozluk.Kod, matrisBicimi bool, olustur func(sh []int, named map[string]vm.Value) (*ct.Tensor, error)) vm.BuiltinFunc {
	ad := sozluk.Ad(kod)
	return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s([2, 3]) şekli liste olarak bekler", ad)
		}
		izinli := []baglama.Ad{adTurevIzle, adTur}
		if kod == sozluk.T+103 { // rastgele_normal
			izinli = append(izinli, adTohum)
		}
		var sh []int
		var err error
		if _, liste := args[0].(*vm.ListVal); !liste && matrisBicimi {
			// Eski biçim (sıfırlar(2, 3)) adlı argüman almaz; sessizce yok saymak yerine
			// şekil listeli biçime yönlendir.
			if len(named) > 0 {
				return nil, fmt.Errorf("%s(%s) eski matris biçimidir ve adlı argüman almaz; %s([%s], ...) yazın",
					ad, argMetni(args), ad, argMetni(args))
			}
			sh, err = matrisSekli(kod, args)
		} else {
			if !liste {
				if _, tam := args[0].(*vm.IntVal); !tam {
					return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", ad, tipAdi(args[0]))
				}
				args = []vm.Value{&vm.ListVal{Elements: args}}
			}
			if len(args) != 1 {
				return nil, fmt.Errorf("%s([2, 3]) tek parametre (şekil listesi) bekler, %d verildi", ad, len(args))
			}
			if err := baglama.AdliDenetle(ad+"()", named, izinli...); err != nil {
				return nil, err
			}
			sh, err = sekilListesi(ad, args[0])
		}
		if err != nil {
			return nil, err
		}
		dt, err := adliTur(ad, named, ct.Float32)
		if err != nil {
			return nil, err
		}
		t, err := olustur(sh, named)
		if err != nil {
			return nil, err
		}
		// Yeni tensör türev izlemediği için dönüşüm çizgeye kaydedilmez; sonuç yapraktır.
		if t, err = tureCevir(t, dt); err != nil {
			return nil, err
		}
		t.SetRequiresGrad(turevIzle(named, nil, false))
		return Yeni(t), nil
	}
}

var (
	rastgeleMu  sync.Mutex
	rastgeleKay = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func rastgeleNormal(sh []int, named map[string]vm.Value) (*ct.Tensor, error) {
	if s, ok := baglama.AdliAl(named, adTohum); ok {
		tohum, ok := s.(*vm.IntVal)
		if !ok {
			return nil, fmt.Errorf("rastgele_normal: tohum tam sayı olmalıdır, %s verildi", tipAdi(s))
		}
		return ct.Randn(rand.New(rand.NewSource(tohum.Val)), sh...), nil
	}
	rastgeleMu.Lock()
	defer rastgeleMu.Unlock()
	return ct.Randn(rastgeleKay, sh...), nil
}

// ----------------------------------------------------------------------------
// Eleman bazlı fonksiyonlar
// ----------------------------------------------------------------------------

func gelu64(x float64) float64 {
	return 0.5 * x * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(x+0.044715*x*x*x)))
}

func relu64(x float64) float64 {
	if x > 0 {
		return x
	}
	return 0
}

func sigmoid64(x float64) float64 { return 1.0 / (1.0 + math.Exp(-x)) }

// birli tek argümanlı eleman bazlı bir fonksiyonu kaydeder. Tensör (matris dahil)
// girdisinde tensör yolu çalışır. Tensör olmayan girdide eski yerleşiği varsa (karekök)
// o çağrılır; yoksa sonuç girdinin biçimindedir: sayı → Ondalık (sayi64 ile), liste → aynı
// yapıda liste.
func birli(v *vm.VM, kod sozluk.Kod, fn func(*ct.Tensor) *ct.Tensor, sayi64 func(float64) float64) {
	ad := sozluk.Ad(kod)
	sar(v, kod, func(eski vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) != 1 || !tensorMu(args) {
				if eski != nil {
					return eski(m, args, named)
				}
				if len(args) != 1 {
					return nil, fmt.Errorf("%s(x) 1 parametre bekler, %d verildi", ad, len(args))
				}
				return eskiBicimdeUygula(ad, args[0], sayi64)
			}
			if err := baglama.AdliDenetle(ad+"()", named); err != nil {
				return nil, err
			}
			t, err := tensoreCevir(ad, args[0])
			if err != nil {
				return nil, err
			}
			r, err := guvenli(func() *ct.Tensor { return fn(t) })
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}

// eskiBicimdeUygula eleman bazlı fonksiyonu tensör olmayan bir değere, değerin kendi
// biçimini koruyarak (float64 ile) uygular.
func eskiBicimdeUygula(ad string, v vm.Value, f func(float64) float64) (vm.Value, error) {
	if x, ok := v.(*vm.ListVal); ok {
		el := make([]vm.Value, len(x.Elements))
		for i, e := range x.Elements {
			r, err := eskiBicimdeUygula(ad, e, f)
			if err != nil {
				return nil, err
			}
			el[i] = r
		}
		return &vm.ListVal{Elements: el}, nil
	}
	if s, ok := vm.ToFloat(v); ok {
		return &vm.FloatVal{Val: f(s)}, nil
	}
	return nil, fmt.Errorf("%s: sayı, liste ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
}

// softmaxListe softmax'ı (logKip ise log_softmax'ı) düz sayı listesine float64 ile uygular
// ve liste döndürür.
func softmaxListe(ad string, v vm.Value, logKip bool) (vm.Value, error) {
	x, ok := v.(*vm.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: liste ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
	}
	if len(x.Elements) == 0 {
		return nil, fmt.Errorf("%s: boş liste", ad)
	}
	sayilar := make([]float64, len(x.Elements))
	enb := math.Inf(-1)
	for i, e := range x.Elements {
		f, ok := vm.ToFloat(e)
		if !ok {
			return nil, fmt.Errorf("%s: liste düz sayı listesi olmalıdır (%d. eleman %s); iç içe veri için %s(tensör(...), eksen: -1) yazın", ad, i+1, tipAdi(e), ad)
		}
		sayilar[i] = f
		enb = math.Max(enb, f)
	}
	ustel := make([]float64, len(sayilar))
	toplam := 0.0
	for i, f := range sayilar {
		ustel[i] = math.Exp(f - enb)
		toplam += ustel[i]
	}
	el := make([]vm.Value, len(sayilar))
	for i := range sayilar {
		if logKip {
			el[i] = &vm.FloatVal{Val: sayilar[i] - (enb + math.Log(toplam))}
		} else {
			el[i] = &vm.FloatVal{Val: ustel[i] / toplam}
		}
	}
	return &vm.ListVal{Elements: el}, nil
}

// eksenArg eksen adlı argümanını ya da konumlu ikinci argümanı okur.
func eksenArg(fn string, args []vm.Value, named map[string]vm.Value) (int, bool, error) {
	var ev vm.Value
	if len(args) >= 2 {
		ev = args[1]
	}
	if v, ok := baglama.AdliAl(named, adEksen); ok {
		if ev != nil {
			return 0, false, fmt.Errorf("%s: eksen hem konumlu hem adlı verildi", fn)
		}
		ev = v
	}
	if ev == nil || ev == vm.NULL {
		return 0, false, nil
	}
	e, ok := tamSayi(ev)
	if !ok {
		return 0, false, fmt.Errorf("%s: eksen tam sayı olmalıdır, %s verildi", fn, ev.Inspect())
	}
	return e, true, nil
}

// softmaxKaydet softmax/log_softmax'ı kaydeder: tensörde (matris dahil) verilen eksende
// (varsayılan son eksen) çalışır; düz sayı listesinde liste döndürür.
func softmaxKaydet(v *vm.VM, kod sozluk.Kod, logKip bool) {
	ad := sozluk.Ad(kod)
	sar(v, kod, func(vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) == 0 || !tensorMu(args[:1]) {
				if len(args) == 0 {
					return nil, fmt.Errorf("%s(x, eksen: -1) en az 1 parametre bekler", ad)
				}
				if len(args) > 1 || len(named) > 0 {
					return nil, fmt.Errorf("%s: eksen yalnızca tensörlerde kullanılır; %s(tensör(...), eksen: -1) yazın", ad, ad)
				}
				return softmaxListe(ad, args[0], logKip)
			}
			if len(args) > 2 {
				return nil, fmt.Errorf("%s(x, eksen: -1) en fazla 2 parametre alır, %d verildi", ad, len(args))
			}
			if err := baglama.AdliDenetle(ad+"()", named, adEksen); err != nil {
				return nil, err
			}
			t, err := tensoreCevir(ad, args[0])
			if err != nil {
				return nil, err
			}
			if t, err = yuksekHassasiyet(m, t); err != nil {
				return nil, err
			}
			e, verildi, err := eksenArg(ad, args, named)
			if err != nil {
				return nil, err
			}
			if !verildi {
				e = -1
			}
			e, err = eksenCoz(ad, e, t.Shape())
			if err != nil {
				return nil, err
			}
			r, err := guvenli(func() *ct.Tensor {
				if logKip {
					return t.LogSoftmax(e)
				}
				return t.Softmax(e)
			})
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}

// indirgeme toplam/ortalama/maks/min'i kaydeder. Eksen verilmezse tüm elemanlar
// indirgenir ve 0 boyutlu (skaler) tensör döner; eksen verilirse o eksen kaldırılır.
// boyutu_koru: doğru indirgenen ekseni (eksen verilmezse bütün eksenleri) 1 uzunlukta
// korur: [B, T, D] → [B, T, 1].
func indirgeme(v *vm.VM, kod sozluk.Kod, fn func(t *ct.Tensor, eksen int, tum, koru bool) *ct.Tensor) {
	ad := sozluk.Ad(kod)
	maksMin := kod == sozluk.G+402 || kod == sozluk.G+401 // maks, min
	sar(v, kod, func(eski vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) == 0 {
				return eskiCagir(ad, eski, m, args, named)
			}
			if _, ok := args[0].(*Deger); !ok {
				return tensorOlmayanIndirgeme(kod, eski, m, args, named)
			}
			if maksMin && len(args) >= 2 {
				// maks(x, 0) eksen mi, eleman bazlı sınır mı belirsiz; sessizce eksen saymak yerine yol göster.
				return nil, fmt.Errorf("%s(x, %s): eksen için %s(x, eksen: %s) yazın; eleman bazlı sınır için %s kullanın",
					ad, args[1].Inspect(), ad, args[1].Inspect(), map[bool]string{true: "relu(x - c) + c (en az c)", false: "relu(x * -1 + c) * -1 + c (en çok c)"}[kod == sozluk.G+402])
			}
			if len(args) > 2 {
				return nil, fmt.Errorf("%s(x, eksen: ...) en fazla 2 parametre alır, %d verildi", ad, len(args))
			}
			if err := baglama.AdliDenetle(ad+"()", named, adEksen, adBoyutuKoru); err != nil {
				return nil, err
			}
			koru, err := adliMantik(ad, named, adBoyutuKoru, false)
			if err != nil {
				return nil, err
			}
			t := args[0].(*Deger).t
			if !maksMin {
				// Karışık hassasiyet bloğunda toplam/ortalama float32'de yapılır.
				if t, err = yuksekHassasiyet(m, t); err != nil {
					return nil, err
				}
			}
			e, verildi, err := eksenArg(ad, args, named)
			if err != nil {
				return nil, err
			}
			sh := t.Shape()
			if verildi {
				if e, err = eksenCoz(ad, e, sh); err != nil {
					return nil, err
				}
				if maksMin && sh[e] == 0 {
					return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş", ad, sekilMetni(sh), e)
				}
			} else if maksMin && t.Numel() == 0 {
				return nil, fmt.Errorf("%s: boş tensörün (%s) en büyük/küçük elemanı yoktur", ad, sekilMetni(sh))
			}
			r, err := guvenli(func() *ct.Tensor {
				r := fn(t, e, !verildi, koru)
				if koru && !verildi {
					birler := make([]int, len(sh))
					for i := range birler {
						birler[i] = 1
					}
					r = r.Reshape(birler...)
				}
				return r
			})
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}

// maksIndeks verilen eksende en büyük elemanın indeksini verir: maks_indeks(x, eksen: -1).
// Sonuç her zaman tam sayı değerli bir tensördür (o eksen kaldırılmış şekilde; 1 boyutlu
// girdide 0 boyutlu) ve türev izlemez. Tensör tam sayılı olarak işaretlenir: .değer Tam
// (ya da Tam listesi) verir, böylece liste indeksi olarak doğrudan kullanılabilir; sonuç
// gömme(...) ve çapraz_entropi(...) indeksi olarak da verilebilir.
// Eşitlikte ilk en büyük eleman seçilir.
func maksIndeks(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const ad = "maks_indeks"
	if len(args) != 1 {
		if len(args) == 2 {
			return nil, fmt.Errorf("%s(x, eksen: %s): eksen adlı verilir", ad, args[1].Inspect())
		}
		return nil, fmt.Errorf("%s(x, eksen: -1) 1 konumlu parametre bekler, %d verildi", ad, len(args))
	}
	if err := baglama.AdliDenetle(ad+"()", named, adEksen); err != nil {
		return nil, err
	}
	d, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("%s: bir tensör bekler, %s verildi; liste için %s(tensör(liste)) yazın", ad, tipAdi(args[0]), ad)
	}
	t := d.t
	e, verildi, err := eksenArg(ad, args, named)
	if err != nil {
		return nil, err
	}
	if !verildi {
		e = -1
	}
	sh := t.Shape()
	if e, err = eksenCoz(ad, e, sh); err != nil {
		return nil, err
	}
	if sh[e] == 0 {
		return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş", ad, sekilMetni(sh), e)
	}
	r, err := guvenli(func() *ct.Tensor {
		idx := t.ArgMax(e)
		veri := make([]float32, len(idx))
		for i, x := range idx {
			veri[i] = float32(x)
		}
		cikti := append(append([]int{}, sh[:e]...), sh[e+1:]...)
		return ct.NewOn(t.Device(), veri, cikti...)
	})
	if err != nil {
		return nil, err
	}
	sonuc := Yeni(r)
	sonuc.tamSayili = true
	return sonuc, nil
}

// tensorOlmayanIndirgeme ilk argümanı tensör olmayan indirgeme çağrılarını işler. Eski
// yerleşikler iç içe listeleri ve listedeki tensörleri sessizce atladığı için (toplam([[1, 2]])
// → 0) bu durumlar burada yakalanır: toplam/ortalama listedeki tensörleri hesap çizgesini
// koruyarak katlar, diğer durumlar açık hata verir.
func tensorOlmayanIndirgeme(kod sozluk.Kod, eski vm.BuiltinFunc, m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	ad := sozluk.Ad(kod)
	if _, ok := baglama.AdliAl(named, adBoyutuKoru); ok {
		return nil, fmt.Errorf("%s: boyutu_koru: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(tensör(...), eksen: 0, boyutu_koru: doğru)", ad, tipAdi(args[0]), ad)
	}
	if _, ok := baglama.AdliAl(named, adEksen); ok {
		return nil, fmt.Errorf("%s: eksen: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(tensör(...), eksen: 0)", ad, tipAdi(args[0]), ad)
	}
	if l, ok := args[0].(*vm.ListVal); ok && len(args) == 1 {
		var tensorVar bool
		for _, el := range l.Elements {
			switch el.(type) {
			case *vm.ListVal:
				return nil, fmt.Errorf("%s: iç içe liste indirgenemez (eski liste davranışı yalnızca düz sayı listesi alır); tensöre çevirin: %s(tensör(liste)) ya da eksen için %s(tensör(liste), eksen: 0)", ad, ad, ad)
			case *Deger:
				tensorVar = true
			}
		}
		if tensorVar {
			if kod != sozluk.G+400 && kod != sozluk.G+410 { // toplam, ortalama
				return nil, fmt.Errorf("%s: tensör listesinin en %s elemanı tanımsız; tek bir tensör verin: %s(x) ya da %s(x, eksen: 0)", ad, map[bool]string{true: "büyük", false: "küçük"}[kod == sozluk.G+402], ad, ad)
			}
			return tensorListesiTopla(ad, kod == sozluk.G+410, l.Elements)
		}
	}
	for _, a := range args[1:] {
		if _, ok := a.(*Deger); ok {
			return nil, fmt.Errorf("%s(x, eksen: ...) ilk parametre olarak tensör bekler, %s verildi", ad, tipAdi(args[0]))
		}
	}
	return eskiCagir(ad, eski, m, args, named)
}

// tensorListesiTopla sayı ve tensörlerden oluşan bir listeyi yayınlamalı toplamla katlar;
// ortalama için eleman sayısına böler. Sonuç türev izlenen girdilere bağlı kalır.
func tensorListesiTopla(ad string, ortalama bool, el []vm.Value) (vm.Value, error) {
	var acc *ct.Tensor
	sabit := 0.0
	for i, e := range el {
		switch x := e.(type) {
		case *Deger:
			if acc == nil {
				acc = x.t
				continue
			}
			if err := yayinDenetle("+", acc, x.t); err != nil {
				return nil, fmt.Errorf("%s: %d. eleman eklenemiyor: %v", ad, i+1, err)
			}
			a := acc
			r, err := guvenli(func() *ct.Tensor { return ct.Add(a, x.t) })
			if err != nil {
				return nil, err
			}
			acc = r
		default:
			f, ok := vm.ToFloat(e)
			if !ok {
				return nil, fmt.Errorf("%s: listenin %d. elemanı sayı ya da tensör olmalıdır, %s verildi", ad, i+1, tipAdi(e))
			}
			sabit += f
		}
	}
	r, err := guvenli(func() *ct.Tensor {
		out := acc
		if sabit != 0 {
			out = out.AddScalar(float32(sabit))
		}
		if ortalama {
			out = ct.Div(out, ct.Scalar(float32(len(el))).To(out.Device()))
		}
		return out
	})
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// ----------------------------------------------------------------------------
// Sinir ağı fonksiyonları
// ----------------------------------------------------------------------------

// tamIndeksler indeks verisini (tam sayı, iç içe liste ya da tensör) düz tam sayı
// dizisine ve şekline çevirir.
func tamIndeksler(fn string, v vm.Value) ([]int, []int, error) {
	if n, ok := tamSayi(v); ok {
		return []int{n}, nil, nil
	}
	var veri []float32
	var sh []int
	switch x := v.(type) {
	case *Deger:
		veri, sh = x.t.Data(), x.t.Shape()
	case *vm.ListVal:
		var err error
		veri, sh, err = listeVerisi(fn, x)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("%s: indeksler tam sayı, tam sayı listesi ya da tensör olmalıdır, %s verildi", fn, tipAdi(v))
	}
	out := make([]int, len(veri))
	for i, f := range veri {
		if float32(int(f)) != f {
			return nil, nil, fmt.Errorf("%s: indeksler tam sayı olmalıdır, %v verildi", fn, f)
		}
		out[i] = int(f)
	}
	return out, sh, nil
}

func gomme(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("gömme(tablo, indeksler) 2 parametre bekler, %d verildi", len(args))
	}
	if err := baglama.AdliDenetle("gömme()", named); err != nil {
		return nil, err
	}
	tablo, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("gömme: tablo bir tensör olmalıdır, %s verildi", tipAdi(args[0]))
	}
	tsh := tablo.t.Shape()
	if len(tsh) != 2 {
		return nil, fmt.Errorf("gömme: tablo 2 boyutlu (satır sayısı × boyut) olmalıdır, %s verildi", sekilMetni(tsh))
	}
	idx, ish, err := tamIndeksler("gömme", args[1])
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		if i < 0 || i >= tsh[0] {
			return nil, fmt.Errorf("gömme: indeks %d, %s şeklindeki tablonun dışında; 0 ile %d arasında olmalı", i, sekilMetni(tsh), tsh[0]-1)
		}
	}
	r, err := guvenli(func() *ct.Tensor {
		if len(ish) == 0 {
			// Tek indeks: sonuç tek bir satırdır, şekli [boyut].
			return ct.Embedding(tablo.t, idx).Reshape(tsh[1])
		}
		return ct.Embedding(tablo.t, idx, ish...)
	})
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func caprazEntropi(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("çapraz_entropi(çıktı, hedef) 2 parametre bekler, %d verildi", len(args))
	}
	if err := baglama.AdliDenetle("çapraz_entropi()", named); err != nil {
		return nil, err
	}
	cikti, err := tensoreCevir("çapraz_entropi", args[0])
	if err != nil {
		return nil, err
	}
	if cikti, err = yuksekHassasiyet(m, cikti); err != nil {
		return nil, err
	}
	sh := cikti.Shape()
	if len(sh) == 0 {
		return nil, fmt.Errorf("çapraz_entropi: çıktı en az 1 boyutlu olmalıdır (son eksen sınıflar)")
	}
	hedef, _, err := tamIndeksler("çapraz_entropi", args[1])
	if err != nil {
		return nil, err
	}
	C := sh[len(sh)-1]
	satir := 0
	if C > 0 {
		satir = cikti.Numel() / C
	}
	if len(hedef) != satir && C > 1 && len(hedef) == cikti.Numel() && sifirBirMi(hedef) {
		return nil, fmt.Errorf("çapraz_entropi: hedef sınıf indeksleri olmalı (ör. [2, 0, 1]); one-hot verdiniz gibi görünüyor (%s şeklindeki çıktıyla aynı sayıda 0/1); her satır için 1'in konumunu verin", sekilMetni(sh))
	}
	if len(hedef) != satir {
		return nil, fmt.Errorf("çapraz_entropi: %s şeklindeki çıktı için %d hedef bekleniyor, %d verildi", sekilMetni(sh), satir, len(hedef))
	}
	for _, h := range hedef {
		if h >= C {
			return nil, fmt.Errorf("çapraz_entropi: hedef sınıf %d, %d sınıflı çıktının dışında; 0 ile %d arasında olmalı (negatif hedefler yok sayılır)", h, C, C-1)
		}
	}
	r, err := guvenli(func() *ct.Tensor { return ct.CrossEntropy(cikti, hedef) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func sifirBirMi(x []int) bool {
	for _, v := range x {
		if v != 0 && v != 1 {
			return false
		}
	}
	return true
}

// normParam ağırlık/sapma argümanını okur (yok verilebilir) ve şeklini denetler.
func normParam(fn, ad string, v vm.Value, D int) (*ct.Tensor, error) {
	if v == nil || v == vm.NULL {
		return nil, nil
	}
	t, err := tensoreCevir(fn, v)
	if err != nil {
		return nil, err
	}
	if sh := t.Shape(); len(sh) != 1 || sh[0] != D {
		return nil, fmt.Errorf("%s: %s [%d] şeklinde olmalı (x'in son ekseni %d), %s verildi", fn, ad, D, D, sekilMetni(sh))
	}
	return t, nil
}

// normArgs x, ağırlık, sapma ve epsilon argümanlarını konumlu ya da adlı olarak okur.
func normArgs(fn string, args []vm.Value, named map[string]vm.Value, adlar []baglama.Ad) (*ct.Tensor, []vm.Value, float32, error) {
	if err := baglama.AdliDenetle(fn+"()", named, append([]baglama.Ad{adEpsilon}, adlar...)...); err != nil {
		return nil, nil, 0, err
	}
	if len(args) < 1 || len(args) > 1+len(adlar) {
		return nil, nil, 0, fmt.Errorf("%s 1 ile %d arasında konumlu parametre alır, %d verildi", fn, 1+len(adlar), len(args))
	}
	x, err := tensoreCevir(fn, args[0])
	if err != nil {
		return nil, nil, 0, err
	}
	if len(x.Shape()) == 0 {
		return nil, nil, 0, fmt.Errorf("%s: x en az 1 boyutlu olmalıdır", fn)
	}
	degerler := make([]vm.Value, len(adlar))
	for i, a := range adlar {
		if 1+i < len(args) {
			degerler[i] = args[1+i]
		}
		if v, ok := baglama.AdliAl(named, a); ok {
			if degerler[i] != nil {
				return nil, nil, 0, fmt.Errorf("%s: %s hem konumlu hem adlı verildi", fn, a.TR)
			}
			degerler[i] = v
		}
	}
	eps := float32(1e-5)
	if v, ok := baglama.AdliAl(named, adEpsilon); ok {
		f, ok := vm.ToFloat(v)
		if !ok || f <= 0 {
			return nil, nil, 0, fmt.Errorf("%s: epsilon pozitif bir sayı olmalıdır, %s verildi", fn, v.Inspect())
		}
		eps = float32(f)
	}
	return x, degerler, eps, nil
}

func katmanNormu(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	x, p, eps, err := normArgs("katman_normu", args, named, []baglama.Ad{adAgirlik, adSapma})
	if err != nil {
		return nil, err
	}
	if x, err = yuksekHassasiyet(m, x); err != nil {
		return nil, err
	}
	sh := x.Shape()
	D := sh[len(sh)-1]
	w, err := normParam("katman_normu", "ağırlık", p[0], D)
	if err != nil {
		return nil, err
	}
	b, err := normParam("katman_normu", "sapma", p[1], D)
	if err != nil {
		return nil, err
	}
	if w, err = yuksekHassasiyet(m, w); err != nil {
		return nil, err
	}
	if b, err = yuksekHassasiyet(m, b); err != nil {
		return nil, err
	}
	r, err := guvenli(func() *ct.Tensor { return ct.LayerNorm(x, w, b, eps) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func rmsNormu(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	x, p, eps, err := normArgs("rms_normu", args, named, []baglama.Ad{adAgirlik})
	if err != nil {
		return nil, err
	}
	sh := x.Shape()
	w, err := normParam("rms_normu", "ağırlık", p[0], sh[len(sh)-1])
	if err != nil {
		return nil, err
	}
	if x, err = yuksekHassasiyet(m, x); err != nil {
		return nil, err
	}
	if w, err = yuksekHassasiyet(m, w); err != nil {
		return nil, err
	}
	r, err := guvenli(func() *ct.Tensor { return ct.RMSNorm(x, w, eps) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

// ----------------------------------------------------------------------------
// Türev
// ----------------------------------------------------------------------------

// geriYayilim geri_yayılım(kayıp) yerleşiğidir: tek elemanlı kayıptan geri yayılım yapar;
// yaprak tensörlerin gradyanları birikir.
func geriYayilim(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("geri_yayılım(kayıp) 1 parametre bekler, %d verildi", len(args))
	}
	if err := baglama.AdliDenetle("geri_yayılım()", named); err != nil {
		return nil, err
	}
	d, ok := args[0].(*Deger)
	if !ok {
		return nil, fmt.Errorf("geri_yayılım() bir tensör bekler, %s verildi", tipAdi(args[0]))
	}
	t := d.t
	if t.Numel() != 1 {
		return nil, fmt.Errorf("geri_yayılım tek elemanlı (skaler) bir kayıp ister, %s şeklinde tensör verildi; önce toplam(...) ya da ortalama(...) ile indirgeyin", sekilMetni(t.Shape()))
	}
	if !t.RequiresGrad() {
		return nil, fmt.Errorf("geri_yayılım: kayıp türev izlenen hiçbir tensöre bağlı değil; parametreleri tensör(..., türev_izle: doğru) ile oluşturun (türevsiz bloğu içinde hesaplanan değerler izlenmez)")
	}
	if _, err := guvenli(func() bool { t.Backward(); return true }); err != nil {
		return nil, err
	}
	return vm.TRUE, nil
}

// turevsiz iki biçimde çalışır: türevsiz { ... } bloğu türev izlemeden çalıştırır ve
// bloğun değerini döndürür; türevsiz(x) tensörü hesap çizgesinden ayırır.
func turevsiz(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("türevsiz { ... } bir blok ya da türevsiz(x) bir tensör bekler, %d parametre verildi", len(args))
	}
	if err := baglama.AdliDenetle("türevsiz()", named); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case *Deger:
		return Yeni(x.t.Detach()), nil
	case *vm.FuncVal, *vm.BuiltinVal:
		var sonuc vm.Value
		var err error
		ct.NoGrad(func() {
			sonuc, err = m.ApplyFunction(x, nil, nil, 0)
		})
		if err != nil {
			return nil, err
		}
		if sonuc == nil {
			sonuc = vm.NULL
		}
		return sonuc, nil
	}
	return nil, fmt.Errorf("türevsiz { ... } bir blok ya da türevsiz(x) bir tensör bekler, %s verildi", tipAdi(args[0]))
}

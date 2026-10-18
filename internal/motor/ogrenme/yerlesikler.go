package ogrenme

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"tr_dili/internal/dil/kod"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// Kaydet tensör yerleşiklerini VM'ye kaydeder. Adlar kod listesindedir (internal/dil/kod); burada
// yalnızca kodlar geçer. Aynı adlı çekirdek ve yapay zekâ yerleşikleri (toplam, ortalama,
// min, maks, karekök) sarılır: argüman tensörse tensör yolu, değilse eski yerleşik çalışır.
func Kaydet(v *yurut.VM) {
	// Türevsiz kip görev başınadır (gorev.go).
	v.GorevKancasiEkle(gorevKancasi)

	// --- Oluşturucular ---
	sar(v, kod.T100, yeniYerlesik(tensorYerlesik))
	sar(v, kod.T101, yeniYerlesik(sekilliOlusturucu(kod.T101, func(sh []int, _ map[string]yurut.Value) (*tensor.Tensor, error) {
		return tensor.Zeros(sh...), nil
	})))
	sar(v, kod.T102, yeniYerlesik(sekilliOlusturucu(kod.T102, func(sh []int, _ map[string]yurut.Value) (*tensor.Tensor, error) {
		return tensor.Ones(sh...), nil
	})))
	sar(v, kod.T103, yeniYerlesik(sekilliOlusturucu(kod.T103, rastgeleNormal)))

	// --- Matris (2 boyutlu tensör) oluşturucusu (matris.go) ---
	sar(v, kod.T111, yeniYerlesik(birimMatris))

	// --- Görünüm ---
	sar(v, kod.T121, yeniYerlesik(birlestirYerlesik)) // model_araclari.go

	// --- Eleman bazlı matematik ---
	birli(v, kod.T200, (*tensor.Tensor).Relu, relu64)
	birli(v, kod.T201, (*tensor.Tensor).Sigmoid, sigmoid64)
	birli(v, kod.T202, (*tensor.Tensor).Tanh, math.Tanh)
	birli(v, kod.T203, (*tensor.Tensor).Gelu, gelu64)
	birli(v, kod.G404, (*tensor.Tensor).Sqrt, nil) // karekök
	birli(v, kod.T220, (*tensor.Tensor).Exp, math.Exp)
	birli(v, kod.T221, (*tensor.Tensor).Log, math.Log)
	birli(v, kod.T222, (*tensor.Tensor).Sin, math.Sin)
	birli(v, kod.T223, (*tensor.Tensor).Cos, math.Cos)

	// --- Softmax ---
	softmaxKaydet(v, kod.T210, false)
	softmaxKaydet(v, kod.T211, true)

	// --- İndirgemeler (toplam, ortalama, maks, min) ---
	indirgeme(v, kod.G400, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.SumAll()
		}
		return t.Sum(e, koru)
	})
	indirgeme(v, kod.G410, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.MeanAll()
		}
		return t.Mean(e, koru)
	})
	indirgeme(v, kod.G402, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return t.Reshape(-1).Max(0, false)
		}
		return t.Max(e, koru)
	})
	indirgeme(v, kod.G401, func(t *tensor.Tensor, e int, tum, koru bool) *tensor.Tensor {
		if tum {
			return enKucukTum(t)
		}
		return t.Neg().Max(e, koru).Neg()
	})

	// --- Sinir ağı ---
	sar(v, kod.T500, yeniYerlesik(gomme))
	sar(v, kod.T510, yeniYerlesik(caprazEntropi))
	sar(v, kod.T501, yeniYerlesik(katmanNormu))
	sar(v, kod.T502, yeniYerlesik(rmsNormu))
	sar(v, kod.T503, yeniYerlesik(dikkatYerlesik)) // model_araclari.go
	sar(v, kod.T530, yeniYerlesik(parametrelerYerlesik))
	sar(v, kod.T611, yeniYerlesik(orneklemeYerlesik))

	// --- Türev ---
	sar(v, kod.T300, yeniYerlesik(geriYayilim))
	sar(v, kod.T301, yeniYerlesik(turevsiz))
	ozelTurevKaydet(v) // özel_türev, türev_denetle (ozel_turev.go)

	// --- Veri türleri (veri_turu.go) ---
	sar(v, kod.T710, yeniYerlesik(karisikHassasiyet))

	// --- İyileştiriciler ---
	sar(v, kod.T400, yeniYerlesik(adamWKur))
	sar(v, kod.T401, yeniYerlesik(sgdKur))
	sar(v, kod.T402, yeniYerlesik(gradyanKirp))

	// --- İndeks, kaydetme ve yükleme ---
	sar(v, kod.T230, yeniYerlesik(maksIndeks))
	sar(v, kod.T700, yeniYerlesik(tensorKaydet))
	sar(v, kod.T701, yeniYerlesik(tensorYukle))
}

// enKucukTum tüm elemanların en küçüğünü 0 boyutlu tensör olarak verir. Çekirdekte Min
// işlemi yoktur; türev gerekiyorsa -maks(-x) ile (gradyan doğru elemana gider) hesaplanır.
// Türev gerekmiyorsa ara tensör ayırmadan konakta tek geçişle bulunur.
func enKucukTum(t *tensor.Tensor) *tensor.Tensor {
	if t.RequiresGrad() && tensor.GradEnabled() {
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
	return tensor.Scalar(m).To(t.Device())
}

// sar kodun bütün yazımlarına (ad ve sözlüğe göre Türkçe karaktersiz biçimi) aynı yeni
// yerleşiği bağlar. Eski yerleşik (varsa) bu koddan okunur ve yeni fonksiyona verilir.
func sar(v *yurut.VM, kd kod.Kod, kur func(eski yurut.BuiltinFunc) yurut.BuiltinFunc) {
	var eski yurut.BuiltinFunc
	if b := v.Yerlesik(kd); b != nil {
		eski = b.Fn
	}
	v.YerlesikTanimla(kd, &yurut.BuiltinVal{Name: kod.Ad(kd), Fn: kur(eski)})
}

// eskiCagir eski yerleşiği çağırır; yoksa argümanın tensör olmadığını bildirir.
func eskiCagir(ad string, eski yurut.BuiltinFunc, m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if eski != nil {
		return eski(m, args, named)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("%s() en az 1 parametre bekler", ad)
	}
	return nil, fmt.Errorf("%s() bir tensör bekler, %s verildi", ad, tipAdi(args[0]))
}

// yeniYerlesik eski karşılığı olmayan bir yerleşik kurar.
func yeniYerlesik(fn yurut.BuiltinFunc) func(yurut.BuiltinFunc) yurut.BuiltinFunc {
	return func(yurut.BuiltinFunc) yurut.BuiltinFunc { return fn }
}

func tensorMu(args []yurut.Value) bool {
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
func turevIzle(named map[string]yurut.Value, konumlu yurut.Value, varsayilan bool) bool {
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
func tensorYerlesik(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("tensör(veri, türev_izle: doğru) en az 1 parametre bekler")
	}
	if len(args) > 2 {
		return nil, fmt.Errorf("tensör(veri, türev_izle: doğru) en fazla 2 parametre alır, %d verildi", len(args))
	}
	if err := baglama.AdliDenetle("tensör()", named, adTurevIzle, adTur); err != nil {
		return nil, err
	}
	var konumlu yurut.Value
	if len(args) == 2 {
		konumlu = args[1]
	}
	izle := turevIzle(named, konumlu, false)
	var t *tensor.Tensor
	if d, ok := args[0].(*Deger); ok {
		dt, err := adliTur("tensör", named, d.t.DType())
		if err != nil {
			return nil, err
		}
		// Var olan tensörden çizgeden bağımsız, kendi verisine sahip yeni bir yaprak.
		t, err = guvenli(func() *tensor.Tensor {
			if dt == tensor.Float32 {
				return tensor.NewOn(d.t.Device(), d.t.Data(), d.t.Shape()...)
			}
			return tensor.NewOf(dt, d.t.Data(), d.t.Shape()...).To(d.t.Device())
		})
		if err != nil {
			return nil, err
		}
	} else {
		dt, err := adliTur("tensör", named, tensor.Float32)
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
// Şeklin tek yazımı listedir: sıfırlar(2, 3) biçimi kaldırıldı, yazılınca liste biçimini
// önerir.
func sekilliOlusturucu(kd kod.Kod, olustur func(sh []int, named map[string]yurut.Value) (*tensor.Tensor, error)) yurut.BuiltinFunc {
	ad := kod.Ad(kd)
	return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s([2, 3]) şekli liste olarak bekler", ad)
		}
		izinli := []baglama.Ad{adTurevIzle, adTur}
		if kd == kod.T103 { // rastgele_normal
			izinli = append(izinli, adTohum)
		}
		if _, liste := args[0].(*yurut.ListVal); !liste {
			if _, tam := args[0].(*yurut.IntVal); tam {
				return nil, fmt.Errorf("%s(%s): şekil liste olarak verilir; %s([%s]) yazın", ad, argMetni(args), ad, argMetni(args))
			}
			return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", ad, tipAdi(args[0]))
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("%s([2, 3]) tek parametre (şekil listesi) bekler, %d verildi", ad, len(args))
		}
		if err := baglama.AdliDenetle(ad+"()", named, izinli...); err != nil {
			return nil, err
		}
		sh, err := sekilListesi(ad, args[0])
		if err != nil {
			return nil, err
		}
		dt, err := adliTur(ad, named, tensor.Float32)
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

func rastgeleNormal(sh []int, named map[string]yurut.Value) (*tensor.Tensor, error) {
	if s, ok := baglama.AdliAl(named, adTohum); ok {
		tohum, ok := s.(*yurut.IntVal)
		if !ok {
			return nil, fmt.Errorf("rastgele_normal: tohum tam sayı olmalıdır, %s verildi", tipAdi(s))
		}
		return tensor.Randn(rand.New(rand.NewSource(tohum.Val)), sh...), nil
	}
	rastgeleMu.Lock()
	defer rastgeleMu.Unlock()
	return tensor.Randn(rastgeleKay, sh...), nil
}

// ----------------------------------------------------------------------------
// Eleman bazlı fonksiyonlar
// ----------------------------------------------------------------------------

func gelu64(x float64) float64 {
	return 0.5 * x * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(x+0.044715*x*x*x)))
}

func relu64(x float64) float64 {
	if x > 0 || x != x { // NaN yayılır (tensör relu'suyla aynı)
		return x
	}
	return 0
}

func sigmoid64(x float64) float64 { return 1.0 / (1.0 + math.Exp(-x)) }

// birli tek argümanlı eleman bazlı bir fonksiyonu kaydeder. Tensör (matris dahil)
// girdisinde tensör yolu çalışır. Tensör olmayan girdide eski yerleşiği varsa (karekök)
// o çağrılır; yoksa sonuç girdinin biçimindedir: sayı → Ondalık (sayi64 ile), liste → aynı
// yapıda liste.
func birli(v *yurut.VM, kd kod.Kod, fn func(*tensor.Tensor) *tensor.Tensor, sayi64 func(float64) float64) {
	ad := kod.Ad(kd)
	sar(v, kd, func(eski yurut.BuiltinFunc) yurut.BuiltinFunc {
		return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
			if len(args) != 1 || !tensorMu(args) {
				if err := baglama.AdliDenetle(ad+"()", named); err != nil {
					return nil, err
				}
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
			r, err := guvenli(func() *tensor.Tensor { return fn(t) })
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}

// eskiBicimdeUygula eleman bazlı fonksiyonu tensör olmayan bir değere, değerin kendi
// biçimini koruyarak (float64 ile) uygular.
func eskiBicimdeUygula(ad string, v yurut.Value, f func(float64) float64) (yurut.Value, error) {
	if x, ok := v.(*yurut.ListVal); ok {
		el := make([]yurut.Value, len(x.Elements))
		for i, e := range x.Elements {
			r, err := eskiBicimdeUygula(ad, e, f)
			if err != nil {
				return nil, err
			}
			el[i] = r
		}
		return &yurut.ListVal{Elements: el}, nil
	}
	if s, ok := yurut.ToFloat(v); ok {
		return &yurut.FloatVal{Val: f(s)}, nil
	}
	return nil, fmt.Errorf("%s: sayı, liste ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
}

// softmaxListe softmax'ı (logKip ise log_softmax'ı) düz sayı listesine float64 ile uygular
// ve liste döndürür.
func softmaxListe(ad string, v yurut.Value, logKip bool) (yurut.Value, error) {
	x, ok := v.(*yurut.ListVal)
	if !ok {
		return nil, fmt.Errorf("%s: liste ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
	}
	if len(x.Elements) == 0 {
		return nil, fmt.Errorf("%s: boş liste", ad)
	}
	sayilar := make([]float64, len(x.Elements))
	enb := math.Inf(-1)
	for i, e := range x.Elements {
		f, ok := yurut.ToFloat(e)
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
	el := make([]yurut.Value, len(sayilar))
	for i := range sayilar {
		if logKip {
			el[i] = &yurut.FloatVal{Val: sayilar[i] - (enb + math.Log(toplam))}
		} else {
			el[i] = &yurut.FloatVal{Val: ustel[i] / toplam}
		}
	}
	return &yurut.ListVal{Elements: el}, nil
}

// eksenArg eksen adlı argümanını ya da konumlu ikinci argümanı okur.
func eksenArg(fn string, args []yurut.Value, named map[string]yurut.Value) (int, bool, error) {
	var ev yurut.Value
	if len(args) >= 2 {
		ev = args[1]
	}
	if v, ok := baglama.AdliAl(named, adEksen); ok {
		if ev != nil {
			return 0, false, fmt.Errorf("%s: eksen hem konumlu hem adlı verildi", fn)
		}
		ev = v
	}
	if ev == nil || ev == yurut.NULL {
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
func softmaxKaydet(v *yurut.VM, kd kod.Kod, logKip bool) {
	ad := kod.Ad(kd)
	sar(v, kd, func(yurut.BuiltinFunc) yurut.BuiltinFunc {
		return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
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
			r, err := guvenli(func() *tensor.Tensor {
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
func indirgeme(v *yurut.VM, kd kod.Kod, fn func(t *tensor.Tensor, eksen int, tum, koru bool) *tensor.Tensor) {
	ad := kod.Ad(kd)
	maksMin := kd == kod.G402 || kd == kod.G401 // maks, min
	sar(v, kd, func(eski yurut.BuiltinFunc) yurut.BuiltinFunc {
		return func(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
			if len(args) == 0 {
				if err := baglama.AdliDenetle(ad+"()", named, adEksen, adBoyutuKoru); err != nil {
					return nil, err
				}
				return eskiCagir(ad, eski, m, args, named)
			}
			if _, ok := args[0].(*Deger); !ok {
				return tensorOlmayanIndirgeme(kd, eski, m, args, named)
			}
			if maksMin && len(args) >= 2 {
				// maks(x, 0) eksen mi, eleman bazlı sınır mı belirsiz; sessizce eksen saymak yerine yol göster.
				return nil, fmt.Errorf("%s(x, %s): eksen için %s(x, eksen: %s) yazın; eleman bazlı sınır için %s kullanın",
					ad, args[1].Inspect(), ad, args[1].Inspect(), map[bool]string{true: "relu(x - c) + c (en az c)", false: "relu(x * -1 + c) * -1 + c (en çok c)"}[kd == kod.G402])
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
			if kd == kod.G410 { // ortalama: boş kümenin ortalaması tanımsızdır
				if !verildi && t.Numel() == 0 {
					return nil, fmt.Errorf("%s: boş tensörün ortalaması tanımsız (şekil %s)", ad, sekilMetni(sh))
				}
				if verildi && sh[e] == 0 {
					cikis := 1
					for i, d := range sh {
						if i != e {
							cikis *= d
						}
					}
					if cikis > 0 {
						return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş; boş kümenin ortalaması tanımsız", ad, sekilMetni(sh), e)
					}
				}
			}
			r, err := guvenli(func() *tensor.Tensor {
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
func maksIndeks(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
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
	r, err := guvenli(func() *tensor.Tensor {
		idx := t.ArgMax(e)
		veri := make([]float32, len(idx))
		for i, x := range idx {
			veri[i] = float32(x)
		}
		cikti := append(append([]int{}, sh[:e]...), sh[e+1:]...)
		return tensor.NewOn(t.Device(), veri, cikti...)
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
func tensorOlmayanIndirgeme(kd kod.Kod, eski yurut.BuiltinFunc, m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	ad := kod.Ad(kd)
	if err := baglama.AdliDenetle(ad+"()", named, adEksen, adBoyutuKoru); err != nil {
		return nil, err
	}
	if _, ok := baglama.AdliAl(named, adBoyutuKoru); ok {
		return nil, fmt.Errorf("%s: boyutu_koru: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(tensör(...), eksen: 0, boyutu_koru: doğru)", ad, tipAdi(args[0]), ad)
	}
	if _, ok := baglama.AdliAl(named, adEksen); ok {
		return nil, fmt.Errorf("%s: eksen: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(tensör(...), eksen: 0)", ad, tipAdi(args[0]), ad)
	}
	if l, ok := args[0].(*yurut.ListVal); ok && len(args) == 1 {
		var tensorVar bool
		for _, el := range l.Elements {
			switch el.(type) {
			case *yurut.ListVal:
				return nil, fmt.Errorf("%s: iç içe liste indirgenemez (eski liste davranışı yalnızca düz sayı listesi alır); tensöre çevirin: %s(tensör(liste)) ya da eksen için %s(tensör(liste), eksen: 0)", ad, ad, ad)
			case *Deger:
				tensorVar = true
			}
		}
		if tensorVar {
			if kd != kod.G400 && kd != kod.G410 { // toplam, ortalama
				return nil, fmt.Errorf("%s: tensör listesinin en %s elemanı tanımsız; tek bir tensör verin: %s(x) ya da %s(x, eksen: 0)", ad, map[bool]string{true: "büyük", false: "küçük"}[kd == kod.G402], ad, ad)
			}
			return tensorListesiTopla(ad, kd == kod.G410, l.Elements)
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
func tensorListesiTopla(ad string, ortalama bool, el []yurut.Value) (yurut.Value, error) {
	var acc *tensor.Tensor
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
			r, err := guvenli(func() *tensor.Tensor { return tensor.Add(a, x.t) })
			if err != nil {
				return nil, err
			}
			acc = r
		default:
			f, ok := yurut.ToFloat(e)
			if !ok {
				return nil, fmt.Errorf("%s: listenin %d. elemanı sayı ya da tensör olmalıdır, %s verildi", ad, i+1, tipAdi(e))
			}
			sabit += f
		}
	}
	r, err := guvenli(func() *tensor.Tensor {
		out := acc
		if sabit != 0 {
			out = out.AddScalar(float32(sabit))
		}
		if ortalama {
			out = tensor.Div(out, tensor.Scalar(float32(len(el))).To(out.Device()))
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
func tamIndeksler(fn string, v yurut.Value) ([]int, []int, error) {
	if n, ok := tamSayi(v); ok {
		return []int{n}, nil, nil
	}
	var veri []float32
	var sh []int
	switch x := v.(type) {
	case *Deger:
		veri, sh = x.t.Data(), x.t.Shape()
	case *yurut.ListVal:
		// Liste değerleri tam sayı olarak okunur (float32'den geçmez).
		return listeTamSayilari(fn, x)
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

func gomme(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
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
	r, err := guvenli(func() *tensor.Tensor {
		if len(ish) == 0 {
			// Tek indeks: sonuç tek bir satırdır, şekli [boyut].
			return tensor.Embedding(tablo.t, idx).Reshape(tsh[1])
		}
		return tensor.Embedding(tablo.t, idx, ish...)
	})
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func caprazEntropi(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
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
	gecerli := 0
	for _, h := range hedef {
		if h >= C {
			return nil, fmt.Errorf("çapraz_entropi: hedef sınıf %d, %d sınıflı çıktının dışında; 0 ile %d arasında olmalı (negatif hedefler yok sayılır)", h, C, C-1)
		}
		if h >= 0 {
			gecerli++
		}
	}
	if gecerli == 0 {
		return nil, fmt.Errorf("çapraz_entropi: bütün hedefler yok sayıldı (%d satırın hepsi negatif ya da hiç satır yok); ortalama kayıp tanımsız. Yalnızca dolgudan oluşan partiyi atlayın", len(hedef))
	}
	r, err := guvenli(func() *tensor.Tensor { return tensor.CrossEntropy(cikti, hedef) })
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
func normParam(fn, ad string, v yurut.Value, D int) (*tensor.Tensor, error) {
	if v == nil || v == yurut.NULL {
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
func normArgs(fn string, args []yurut.Value, named map[string]yurut.Value, adlar []baglama.Ad) (*tensor.Tensor, []yurut.Value, float32, error) {
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
	degerler := make([]yurut.Value, len(adlar))
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
		f, ok := yurut.ToFloat(v)
		if !ok || f <= 0 {
			return nil, nil, 0, fmt.Errorf("%s: epsilon pozitif bir sayı olmalıdır, %s verildi", fn, v.Inspect())
		}
		eps = float32(f)
	}
	return x, degerler, eps, nil
}

func katmanNormu(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
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
	r, err := guvenli(func() *tensor.Tensor { return tensor.LayerNorm(x, w, b, eps) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func rmsNormu(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
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
	r, err := guvenli(func() *tensor.Tensor { return tensor.RMSNorm(x, w, eps) })
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
func geriYayilim(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
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
	// Dış düğümlerin (özel_türev) geri hesapları bu görevle çalışır.
	if err := geriYay(m, t, nil); err != nil {
		return nil, err
	}
	return yurut.TRUE, nil
}

// turevsiz iki biçimde çalışır: türevsiz { ... } bloğu türev izlemeden çalıştırır ve
// bloğun değerini döndürür; türevsiz(x) tensörü hesap çizgesinden ayırır.
func turevsiz(m *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("türevsiz { ... } bir blok ya da türevsiz(x) bir tensör bekler, %d parametre verildi", len(args))
	}
	if err := baglama.AdliDenetle("türevsiz()", named); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case *Deger:
		return Yeni(x.t.Detach()), nil
	case *yurut.FuncVal, *yurut.BuiltinVal:
		var sonuc yurut.Value
		var err error
		tensor.NoGrad(func() {
			sonuc, err = m.ApplyFunction(x, nil, nil, 0)
		})
		if err != nil {
			return nil, err
		}
		if sonuc == nil {
			sonuc = yurut.NULL
		}
		return sonuc, nil
	}
	return nil, fmt.Errorf("türevsiz { ... } bir blok ya da türevsiz(x) bir tensör bekler, %s verildi", tipAdi(args[0]))
}

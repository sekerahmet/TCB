package tensor

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"tr_dili/pkg/motor/yz"
	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/vm"
)

// Kaydet tensör yerleşiklerini VM'ye kaydeder. Aynı adlı eski yerleşikler sarılır:
// argüman tensörse tensör yolu, değilse eski yerleşik çalışır.
func Kaydet(v *vm.VM) {
	// --- Oluşturucular ---
	sar(v, []string{"tensör", "tensor"}, tensorYerlesik)
	sar(v, []string{"sıfırlar", "sifirlar"}, sekilliOlusturucu("sıfırlar", func(sh []int, _ map[string]vm.Value) (*ct.Tensor, error) {
		return ct.Zeros(sh...), nil
	}))
	sar(v, []string{"birler"}, sekilliOlusturucu("birler", func(sh []int, _ map[string]vm.Value) (*ct.Tensor, error) {
		return ct.Ones(sh...), nil
	}))
	sar(v, []string{"rastgele_normal"}, sekilliOlusturucu("rastgele_normal", rastgeleNormal))

	// --- Görünüm ---
	sar(v, []string{"devrik"}, func(eski vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) == 1 {
				if d, ok := args[0].(*Deger); ok {
					return devrik(d.t)
				}
			}
			return eskiCagir("devrik", eski, m, args, named)
		}
	})

	// --- Eleman bazlı matematik ---
	birli(v, []string{"relu"}, (*ct.Tensor).Relu, nil)
	birli(v, []string{"sigmoid"}, (*ct.Tensor).Sigmoid, nil)
	birli(v, []string{"tanh"}, (*ct.Tensor).Tanh, nil)
	birli(v, []string{"gelu"}, (*ct.Tensor).Gelu, gelu64)
	birli(v, []string{"karekök", "karekok"}, (*ct.Tensor).Sqrt, nil)
	birli(v, []string{"üstel", "ustel"}, (*ct.Tensor).Exp, math.Exp)
	birli(v, []string{"log"}, (*ct.Tensor).Log, math.Log)

	// --- Softmax ---
	softmaxKaydet(v, []string{"softmax"}, false)
	softmaxKaydet(v, []string{"log_softmax"}, true)

	// --- İndirgemeler ---
	indirgeme(v, []string{"toplam"}, "toplam", func(t *ct.Tensor, e int, tum bool) *ct.Tensor {
		if tum {
			return t.SumAll()
		}
		return t.Sum(e, false)
	})
	indirgeme(v, []string{"ortalama"}, "ortalama", func(t *ct.Tensor, e int, tum bool) *ct.Tensor {
		if tum {
			return t.MeanAll()
		}
		return t.Mean(e, false)
	})
	indirgeme(v, []string{"maks", "en_büyük"}, "maks", func(t *ct.Tensor, e int, tum bool) *ct.Tensor {
		if tum {
			return t.Reshape(-1).Max(0, false)
		}
		return t.Max(e, false)
	})
	indirgeme(v, []string{"min", "en_küçük"}, "min", func(t *ct.Tensor, e int, tum bool) *ct.Tensor {
		if tum {
			return t.Neg().Reshape(-1).Max(0, false).Neg()
		}
		return t.Neg().Max(e, false).Neg()
	})

	// --- Sinir ağı ---
	sar(v, []string{"gömme", "gomme"}, yeniYerlesik("gömme", gomme))
	sar(v, []string{"çapraz_entropi", "capraz_entropi"}, yeniYerlesik("çapraz_entropi", caprazEntropi))
	sar(v, []string{"katman_normu"}, yeniYerlesik("katman_normu", katmanNormu))
	sar(v, []string{"rms_normu"}, yeniYerlesik("rms_normu", rmsNormu))

	// --- Türev ---
	sar(v, []string{"geri_yayılım", "geri_yayilim"}, geriYayilim)
	sar(v, []string{"türevsiz", "turevsiz"}, yeniYerlesik("türevsiz", turevsiz))

	// --- İyileştiriciler ---
	sar(v, []string{"AdamW"}, yeniYerlesik("AdamW", adamWKur))
	sar(v, []string{"SGD"}, yeniYerlesik("SGD", sgdKur))
}

// sar adların hepsine aynı yeni yerleşiği bağlar. Eski yerleşik ilk addan okunur
// (yoksa nil'dir) ve yeni fonksiyona verilir.
func sar(v *vm.VM, adlar []string, kur func(eski vm.BuiltinFunc) vm.BuiltinFunc) {
	var eski vm.BuiltinFunc
	if onceki, ok := v.GlobalEnv.Get(adlar[0]); ok {
		if b, ok := onceki.(*vm.BuiltinVal); ok {
			eski = b.Fn
		}
	}
	b := &vm.BuiltinVal{Name: adlar[0], Fn: kur(eski)}
	for _, ad := range adlar {
		v.GlobalEnv.Define(ad, b, true)
	}
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
func yeniYerlesik(_ string, fn vm.BuiltinFunc) func(vm.BuiltinFunc) vm.BuiltinFunc {
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
	if v, ok := adliAl(named, "türev_izle", "turev_izle"); ok {
		return v.IsTruthy()
	}
	return varsayilan
}

func tensorYerlesik(eski vm.BuiltinFunc) vm.BuiltinFunc {
	return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("tensör(veri, türev_izle: doğru) en az 1 parametre bekler")
		}
		switch args[0].(type) {
		case *vm.ListVal, *vm.MatrixVal, *Deger:
		default:
			// Tek sayı: eski tek sayılık otomatik türev değeri (geriye uyumluluk).
			return eskiCagir("tensör", eski, m, args, named)
		}
		if len(args) > 2 {
			return nil, fmt.Errorf("tensör(veri, türev_izle: doğru) en fazla 2 parametre alır, %d verildi", len(args))
		}
		if err := adliDenetle("tensör()", named, "türev_izle", "turev_izle"); err != nil {
			return nil, err
		}
		var konumlu vm.Value
		if len(args) == 2 {
			konumlu = args[1]
		}
		izle := turevIzle(named, konumlu, false)
		var t *ct.Tensor
		if d, ok := args[0].(*Deger); ok {
			// Var olan tensörden çizgeden bağımsız, kendi verisine sahip yeni bir yaprak.
			t = ct.NewOn(d.t.Device(), d.t.Data(), d.t.Shape()...)
		} else {
			var err error
			t, err = tensoreCevir("tensör", args[0])
			if err != nil {
				return nil, err
			}
		}
		t.SetRequiresGrad(izle)
		return Yeni(t), nil
	}
}

// sekilliOlusturucu şekli liste olarak alan oluşturucular (sıfırlar([2, 3]) ...) içindir.
// Şekil liste değilse eski yerleşik (sıfırlar(2, 3) → Matris) çağrılır.
func sekilliOlusturucu(ad string, olustur func(sh []int, named map[string]vm.Value) (*ct.Tensor, error)) func(vm.BuiltinFunc) vm.BuiltinFunc {
	return func(eski vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("%s([2, 3]) şekli liste olarak bekler", ad)
			}
			if _, liste := args[0].(*vm.ListVal); !liste {
				if eski != nil {
					// Eski biçim (sıfırlar(2, 3)) adlı argüman almaz; sessizce yok saymak
					// yerine tensör biçimine yönlendir.
					if len(named) > 0 {
						return nil, fmt.Errorf("%s(%s) Matris döndürür ve adlı argüman almaz; tensör için %s([%s], ...) yazın",
							ad, argMetni(args), ad, argMetni(args))
					}
					return eski(m, args, named)
				}
				if _, tam := args[0].(*vm.IntVal); !tam {
					return nil, fmt.Errorf("%s: şekil [2, 3] gibi bir tam sayı listesi olmalıdır, %s verildi", ad, tipAdi(args[0]))
				}
				// Eski karşılığı olmayanlarda tam sayılar da şekil sayılır: rastgele_normal(2, 3).
				args = []vm.Value{&vm.ListVal{Elements: args}}
			}
			if len(args) != 1 {
				return nil, fmt.Errorf("%s([2, 3]) tek parametre (şekil listesi) bekler, %d verildi", ad, len(args))
			}
			izinli := []string{"türev_izle", "turev_izle"}
			if ad == "rastgele_normal" {
				izinli = append(izinli, "tohum")
			}
			if err := adliDenetle(ad+"()", named, izinli...); err != nil {
				return nil, err
			}
			sh, err := sekilListesi(ad, args[0])
			if err != nil {
				return nil, err
			}
			t, err := olustur(sh, named)
			if err != nil {
				return nil, err
			}
			t.SetRequiresGrad(turevIzle(named, nil, false))
			return Yeni(t), nil
		}
	}
}

var (
	rastgeleMu  sync.Mutex
	rastgeleKay = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func rastgeleNormal(sh []int, named map[string]vm.Value) (*ct.Tensor, error) {
	if s, ok := named["tohum"]; ok {
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

// birli tek argümanlı eleman bazlı bir fonksiyonu kaydeder. Eski yerleşiği varsa tensör
// olmayan argümanlar ona gider; yoksa sayılar sayi64 ile hesaplanır, listeler/matrisler
// tensöre çevrilir.
func birli(v *vm.VM, adlar []string, fn func(*ct.Tensor) *ct.Tensor, sayi64 func(float64) float64) {
	ad := adlar[0]
	sar(v, adlar, func(eski vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) != 1 || !tensorMu(args) {
				if eski != nil {
					return eski(m, args, named)
				}
				if len(args) != 1 {
					return nil, fmt.Errorf("%s(x) 1 parametre bekler, %d verildi", ad, len(args))
				}
				// Eski karşılığı olmayan yeni adlar da tensör olmayan girdide eski biçimi
				// izler: sayı → Ondalık, liste → aynı yapıda liste, matris → Matris.
				return eskiBicimdeUygula(ad, args[0], sayi64)
			}
			if err := adliDenetle(ad+"()", named); err != nil {
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
	switch x := v.(type) {
	case *vm.ListVal:
		el := make([]vm.Value, len(x.Elements))
		for i, e := range x.Elements {
			r, err := eskiBicimdeUygula(ad, e, f)
			if err != nil {
				return nil, err
			}
			el[i] = r
		}
		return &vm.ListVal{Elements: el}, nil
	case *vm.MatrixVal:
		m := vm.NewMatrix(x.Rows, x.Cols)
		for i := range x.Data {
			for j, e := range x.Data[i] {
				m.Data[i][j] = f(e)
			}
		}
		return m, nil
	case *yz.TensorVal:
		return nil, fmt.Errorf("%s: tek sayılık türev değeri (tensör(3.0)) desteklenmez; tensör([...]) ile çok boyutlu tensör kullanın", ad)
	}
	if s, ok := vm.ToFloat(v); ok {
		return &vm.FloatVal{Val: f(s)}, nil
	}
	return nil, fmt.Errorf("%s: sayı, liste, matris ya da tensör bekleniyor, %s verildi", ad, tipAdi(v))
}

// logSoftmaxListe log_softmax'ı düz sayı listesine (liste döner) ya da matrisin
// satırlarına (Matris döner) eski softmax'ın biçimiyle uygular.
func logSoftmaxListe(v vm.Value) (vm.Value, error) {
	satir := func(x []float64) []float64 {
		enb := math.Inf(-1)
		for _, e := range x {
			enb = math.Max(enb, e)
		}
		s := 0.0
		for _, e := range x {
			s += math.Exp(e - enb)
		}
		lse := enb + math.Log(s)
		out := make([]float64, len(x))
		for i, e := range x {
			out[i] = e - lse
		}
		return out
	}
	switch x := v.(type) {
	case *vm.MatrixVal:
		m := vm.NewMatrix(x.Rows, x.Cols)
		for i := range x.Data {
			copy(m.Data[i], satir(x.Data[i]))
		}
		return m, nil
	case *vm.ListVal:
		sayilar := make([]float64, len(x.Elements))
		for i, e := range x.Elements {
			f, ok := vm.ToFloat(e)
			if !ok {
				return nil, fmt.Errorf("log_softmax: liste düz sayı listesi olmalıdır (%d. eleman %s); iç içe veri için log_softmax(tensör(...), eksen: -1) yazın", i+1, tipAdi(e))
			}
			sayilar[i] = f
		}
		el := make([]vm.Value, len(sayilar))
		for i, f := range satir(sayilar) {
			el[i] = &vm.FloatVal{Val: f}
		}
		return &vm.ListVal{Elements: el}, nil
	}
	return nil, fmt.Errorf("log_softmax: liste, matris ya da tensör bekleniyor, %s verildi", tipAdi(v))
}

// eksenArg eksen adlı argümanını ya da konumlu ikinci argümanı okur.
func eksenArg(fn string, args []vm.Value, named map[string]vm.Value) (int, bool, error) {
	var ev vm.Value
	if len(args) >= 2 {
		ev = args[1]
	}
	if v, ok := named["eksen"]; ok {
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

func softmaxKaydet(v *vm.VM, adlar []string, logKip bool) {
	ad := adlar[0]
	sar(v, adlar, func(eski vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) == 0 || !tensorMu(args[:1]) {
				if eski != nil {
					return eski(m, args, named)
				}
				if len(args) == 0 {
					return nil, fmt.Errorf("%s(x, eksen: -1) en az 1 parametre bekler", ad)
				}
				// log_softmax'ın eski karşılığı yok; softmax'ın liste/matris biçimini izler.
				if len(args) > 1 || len(named) > 0 {
					return nil, fmt.Errorf("%s: eksen yalnızca tensörlerde kullanılır; %s(tensör(...), eksen: -1) yazın", ad, ad)
				}
				return logSoftmaxListe(args[0])
			}
			if len(args) > 2 {
				return nil, fmt.Errorf("%s(x, eksen: -1) en fazla 2 parametre alır, %d verildi", ad, len(args))
			}
			if err := adliDenetle(ad+"()", named, "eksen"); err != nil {
				return nil, err
			}
			t, err := tensoreCevir(ad, args[0])
			if err != nil {
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
func indirgeme(v *vm.VM, adlar []string, ad string, fn func(t *ct.Tensor, eksen int, tum bool) *ct.Tensor) {
	sar(v, adlar, func(eski vm.BuiltinFunc) vm.BuiltinFunc {
		return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
			if len(args) == 0 {
				return eskiCagir(ad, eski, m, args, named)
			}
			maksMin := ad == "maks" || ad == "min"
			if _, ok := args[0].(*Deger); !ok {
				return tensorOlmayanIndirgeme(ad, eski, m, args, named)
			}
			if maksMin && len(args) >= 2 {
				// maks(x, 0) eksen mi, eleman bazlı sınır mı belirsiz; sessizce eksen saymak yerine yol göster.
				return nil, fmt.Errorf("%s(x, %s): eksen için %s(x, eksen: %s) yazın; eleman bazlı sınır için %s kullanın",
					ad, args[1].Inspect(), ad, args[1].Inspect(), map[bool]string{true: "relu(x - c) + c (en az c)", false: "relu(x * -1 + c) * -1 + c (en çok c)"}[ad == "maks"])
			}
			if len(args) > 2 {
				return nil, fmt.Errorf("%s(x, eksen: ...) en fazla 2 parametre alır, %d verildi", ad, len(args))
			}
			if err := adliDenetle(ad+"()", named, "eksen"); err != nil {
				return nil, err
			}
			t := args[0].(*Deger).t
			e, verildi, err := eksenArg(ad, args, named)
			if err != nil {
				return nil, err
			}
			sh := t.Shape()
			if verildi {
				if e, err = eksenCoz(ad, e, sh); err != nil {
					return nil, err
				}
				if (ad == "maks" || ad == "min") && sh[e] == 0 {
					return nil, fmt.Errorf("%s: %s şeklinde %d. eksen boş", ad, sekilMetni(sh), e)
				}
			} else if (ad == "maks" || ad == "min") && t.Numel() == 0 {
				return nil, fmt.Errorf("%s: boş tensörün (%s) en büyük/küçük elemanı yoktur", ad, sekilMetni(sh))
			}
			r, err := guvenli(func() *ct.Tensor { return fn(t, e, !verildi) })
			if err != nil {
				return nil, err
			}
			return Yeni(r), nil
		}
	})
}

// tensorOlmayanIndirgeme ilk argümanı tensör olmayan indirgeme çağrılarını işler. Eski
// yerleşikler iç içe listeleri ve listedeki tensörleri sessizce atladığı için (toplam([[1, 2]])
// → 0) bu durumlar burada yakalanır: toplam/ortalama listedeki tensörleri hesap çizgesini
// koruyarak katlar, diğer durumlar açık hata verir.
func tensorOlmayanIndirgeme(ad string, eski vm.BuiltinFunc, m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if _, ok := named["eksen"]; ok {
		return nil, fmt.Errorf("%s: eksen: yalnızca tensörlerde kullanılır, %s verildi; önce tensöre çevirin: %s(tensör(...), eksen: 0)", ad, tipAdi(args[0]), ad)
	}
	if l, ok := args[0].(*vm.ListVal); ok && len(args) == 1 {
		var tensorVar, eskiTensorVar bool
		for _, el := range l.Elements {
			switch el.(type) {
			case *vm.ListVal:
				return nil, fmt.Errorf("%s: iç içe liste indirgenemez (eski liste davranışı yalnızca düz sayı listesi alır); tensöre çevirin: %s(tensör(liste)) ya da eksen için %s(tensör(liste), eksen: 0)", ad, ad, ad)
			case *Deger:
				tensorVar = true
			case *yz.TensorVal:
				eskiTensorVar = true
			}
		}
		if tensorVar {
			if eskiTensorVar {
				return nil, fmt.Errorf("%s: listede tek sayılık türev değeri (tensör(3.0)) ile çok boyutlu tensör karışık; hepsini tensör([...]) ile oluşturun", ad)
			}
			if ad != "toplam" && ad != "ortalama" {
				return nil, fmt.Errorf("%s: tensör listesinin en %s elemanı tanımsız; tek bir tensör verin: %s(x) ya da %s(x, eksen: 0)", ad, map[bool]string{true: "büyük", false: "küçük"}[ad == "maks"], ad, ad)
			}
			return tensorListesiTopla(ad, l.Elements)
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
func tensorListesiTopla(ad string, el []vm.Value) (vm.Value, error) {
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
		if ad == "ortalama" {
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
	if err := adliDenetle("gömme()", named); err != nil {
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

func caprazEntropi(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("çapraz_entropi(çıktı, hedef) 2 parametre bekler, %d verildi", len(args))
	}
	if err := adliDenetle("çapraz_entropi()", named); err != nil {
		return nil, err
	}
	cikti, err := tensoreCevir("çapraz_entropi", args[0])
	if err != nil {
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
func normArgs(fn string, args []vm.Value, named map[string]vm.Value, adlar []string) (*ct.Tensor, []vm.Value, float32, error) {
	izinli := []string{"epsilon"}
	for _, a := range adlar {
		izinli = append(izinli, a, asciiAd(a))
	}
	if err := adliDenetle(fn+"()", named, izinli...); err != nil {
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
		if v, ok := adliAl(named, a, asciiAd(a)); ok {
			if degerler[i] != nil {
				return nil, nil, 0, fmt.Errorf("%s: %s hem konumlu hem adlı verildi", fn, a)
			}
			degerler[i] = v
		}
	}
	eps := float32(1e-5)
	if v, ok := named["epsilon"]; ok {
		f, ok := vm.ToFloat(v)
		if !ok || f <= 0 {
			return nil, nil, 0, fmt.Errorf("%s: epsilon pozitif bir sayı olmalıdır, %s verildi", fn, v.Inspect())
		}
		eps = float32(f)
	}
	return x, degerler, eps, nil
}

func asciiAd(s string) string {
	r := []rune(s)
	for i, c := range r {
		switch c {
		case 'ı':
			r[i] = 'i'
		case 'ğ':
			r[i] = 'g'
		case 'ü':
			r[i] = 'u'
		case 'ş':
			r[i] = 's'
		case 'ö':
			r[i] = 'o'
		case 'ç':
			r[i] = 'c'
		}
	}
	return string(r)
}

func katmanNormu(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	x, p, eps, err := normArgs("katman_normu", args, named, []string{"ağırlık", "sapma"})
	if err != nil {
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
	r, err := guvenli(func() *ct.Tensor { return ct.LayerNorm(x, w, b, eps) })
	if err != nil {
		return nil, err
	}
	return Yeni(r), nil
}

func rmsNormu(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	x, p, eps, err := normArgs("rms_normu", args, named, []string{"ağırlık"})
	if err != nil {
		return nil, err
	}
	sh := x.Shape()
	w, err := normParam("rms_normu", "ağırlık", p[0], sh[len(sh)-1])
	if err != nil {
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

func geriYayilim(eski vm.BuiltinFunc) vm.BuiltinFunc {
	return func(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
		if len(args) != 1 || !tensorMu(args) {
			return eskiCagir("geri_yayılım", eski, m, args, named)
		}
		t := args[0].(*Deger).t
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
}

// turevsiz iki biçimde çalışır: türevsiz { ... } bloğu türev izlemeden çalıştırır ve
// bloğun değerini döndürür; türevsiz(x) tensörü hesap çizgesinden ayırır.
func turevsiz(m *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("türevsiz { ... } bir blok ya da türevsiz(x) bir tensör bekler, %d parametre verildi", len(args))
	}
	if err := adliDenetle("türevsiz()", named); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case *Deger:
		return Yeni(x.t.Detach()), nil
	case *yz.TensorVal:
		return yz.NewTensor(x.Val, false), nil
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

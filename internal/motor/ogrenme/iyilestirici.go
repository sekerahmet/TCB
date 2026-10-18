package ogrenme

import (
	"fmt"
	"math"
	"strconv"
	"tr_dili/internal/dil/kod"

	"tr_dili/internal/dil/turkce"
	"tr_dili/internal/dil/yurut"
	"tr_dili/internal/hesap/tensor"
	"tr_dili/internal/motor/ortak/baglama"
)

// Iyilestirici AdamW ve SGD iyileştiricilerinin TR değeridir; ikisinin de tipi
// İyileştirici'dir (A214). Metotları .adım() (parametreleri günceller) ve .sıfırla()
// (gradyanları sıfırlar). ad oluşturucunun adıdır (AdamW, SGD; yazdırmada görünür).
type Iyilestirici struct {
	ad     string
	opt    tensor.Optimizer
	adamw  *tensor.AdamW
	sgd    *tensor.SGD
	params []*tensor.Tensor
}

func (o *Iyilestirici) Type() yurut.ValueType { return iyilestiriciTipi }
func (o *Iyilestirici) IsTruthy() bool        { return true }

func (o *Iyilestirici) Inspect() string {
	f := func(x float32) string { return strconv.FormatFloat(float64(x), 'g', -1, 32) }
	if o.adamw != nil {
		a := o.adamw
		return fmt.Sprintf("%s(parametre: %d, %s: %s, %s: %s, %s: %s, %s: %s, %s: %s, %s: %d)",
			kod.Ad(kod.T400), len(o.params), adOgrenmeOrani.TR, f(a.LR), adAgirlikCurumesi.TR, f(a.WeightDecay),
			adBeta1.TR, f(a.Beta1), adBeta2.TR, f(a.Beta2), adEpsilon.TR, f(a.Eps), kod.Ad(uyeAdim), a.StepCount())
	}
	s := o.sgd
	return fmt.Sprintf("%s(parametre: %d, %s: %s, %s: %s, %s: %s)",
		kod.Ad(kod.T401), len(o.params), adOgrenmeOrani.TR, f(s.LR), adMomentum.TR, f(s.Momentum), adAgirlikCurumesi.TR, f(s.WeightDecay))
}

// İyileştirici üyeleri (adları kod listesinde, internal/dil/kod Y50x).
const (
	uyeAdim    = kod.Y500
	uyeSifirla = kod.Y501
)

var iyilestiriciUyeleri = kod.Adlar(uyeAdim, uyeSifirla)

// iyilestiriciTipi AdamW ve SGD değerlerinin tip adıdır (tip(op) → "İyileştirici").
var iyilestiriciTipi = yurut.ValueType(yurut.TipAdi(kod.A214))

func (o *Iyilestirici) GetProperty(kd kod.Kod, prop string) (yurut.Value, error) {
	switch kd {
	case uyeAdim:
		return &yurut.BuiltinVal{Name: kod.Ad(uyeAdim), AdliAlmaz: true, Fn: func(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", kod.Ad(uyeAdim))
			}
			if _, err := guvenli(func() bool { o.opt.Step(); return true }); err != nil {
				return nil, err
			}
			return yurut.NULL, nil
		}}, nil
	case uyeSifirla:
		return &yurut.BuiltinVal{Name: kod.Ad(uyeSifirla), AdliAlmaz: true, Fn: func(_ *yurut.VM, args []yurut.Value, _ map[string]yurut.Value) (yurut.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", kod.Ad(uyeSifirla))
			}
			o.opt.ZeroGrad()
			return yurut.NULL, nil
		}}, nil
	}
	oneri := ""
	if s := turkce.EnYakin(prop, iyilestiriciUyeleri); s != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", s)
	}
	return nil, fmt.Errorf("%s üzerinde %q metodu bulunamadı%s", o.ad, prop, oneri)
}

// parametreler iyileştiriciye verilen tensör ya da tensör listesini denetler.
func parametreler(fn string, v yurut.Value) ([]*tensor.Tensor, error) {
	var liste []yurut.Value
	switch x := v.(type) {
	case *Deger:
		liste = []yurut.Value{x}
	case *yurut.ListVal:
		liste = x.Elements
	default:
		return nil, fmt.Errorf("%s: parametreler bir tensör listesi olmalıdır, %s verildi", fn, tipAdi(v))
	}
	if len(liste) == 0 {
		return nil, fmt.Errorf("%s: parametre listesi boş", fn)
	}
	out := make([]*tensor.Tensor, 0, len(liste))
	gorulen := map[*tensor.Tensor]bool{}
	for i, el := range liste {
		d, ok := el.(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %d. parametre bir tensör olmalıdır, %s verildi", fn, i+1, tipAdi(el))
		}
		t := d.t
		if !t.RequiresGrad() {
			return nil, fmt.Errorf("%s: %d. parametre (%s) türev izlemiyor; %s ile oluşturun", fn, i+1, sekilMetni(t.Shape()), turevIzleyenOrnek("..."))
		}
		if d.ara || !t.IsLeaf() {
			return nil, fmt.Errorf("%s: %d. parametre (%s) bir işlemin sonucu (yaprak değil); parametreleri doğrudan %s ile oluşturun", fn, i+1, sekilMetni(t.Shape()), turevIzleyenOrnek("..."))
		}
		if gorulen[t] {
			return nil, fmt.Errorf("%s: %d. parametre listede birden çok kez var", fn, i+1)
		}
		gorulen[t] = true
		out = append(out, t)
	}
	return out, nil
}

// adliOran adlı sayısal bir argümanı okur.
func adliOran(fn string, named map[string]yurut.Value, varsayilan float64, enAz float64, ad baglama.Ad) (float32, error) {
	v, ok := baglama.AdliAl(named, ad)
	if !ok {
		return float32(varsayilan), nil
	}
	f, ok := yurut.ToFloat(v)
	if !ok {
		return 0, fmt.Errorf("%s: %s sayı olmalıdır, %s verildi", fn, ad.TR, tipAdi(v))
	}
	// NaN ve sonsuz (float32'ye sığmayan) değerler reddedilir: tek adımda bütün
	// parametreleri NaN yapardı (f < enAz denetimi NaN için yanlıştır).
	if math.IsNaN(f) || math.IsInf(float64(float32(f)), 0) {
		return 0, fmt.Errorf("%s: %s sonlu bir sayı olmalıdır, %s verildi", fn, ad.TR, v.Inspect())
	}
	if f < enAz {
		return 0, fmt.Errorf("%s: %s en az %g olmalıdır, %g verildi", fn, ad.TR, enAz, f)
	}
	return float32(f), nil
}

// gradyanKirp parametre gradyanlarının toplam L2 normunu en_fazla değerine indirir ve
// kırpma öncesi normu Ondalık olarak döndürür: gradyan_kırp(parametreler, en_fazla: 1.0).
func gradyanKirp(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T402)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, %s: 1.0) 1 konumlu parametre bekler, %d verildi", fn, adEnFazla.TR, len(args))
	}
	if err := baglama.AdliDenetle(fn+"()", named, adEnFazla); err != nil {
		return nil, err
	}
	params, err := parametreler(fn, args[0])
	if err != nil {
		return nil, err
	}
	enFazla := 1.0
	if v, ok := baglama.AdliAl(named, adEnFazla); ok {
		f, ok := yurut.ToFloat(v)
		if !ok || !(f > 0) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("%s: %s pozitif bir sayı olmalıdır, %s verildi", fn, adEnFazla.TR, v.Inspect())
		}
		enFazla = f
	}
	norm, err := guvenli(func() float64 { return tensor.ClipGradNorm(params, enFazla) })
	if err != nil {
		return nil, err
	}
	// Gradyanlar float32 olduğundan norm da float32 duyarlığında, en kısa gösterimle verilir.
	return &yurut.FloatVal{Val: kisaFloat64(float32(norm))}, nil
}

// anaKopyaArg ana_kopya: doğru adlı argümanını okur (varsayılan doğru). Açıkken 16 bitlik
// (bf16/fp16) parametrelerin float32 ana kopyası tutulur: güncelleme ana kopyada yapılıp
// parametreye yuvarlanarak yazılır, böylece küçük adımlar 16 bitte kaybolmaz. float32
// parametreleri etkilemez.
func anaKopyaArg(fn string, named map[string]yurut.Value) (bool, error) {
	v, ok := baglama.AdliAl(named, adAnaKopya)
	if !ok {
		return true, nil
	}
	return baglama.Mantik(fn, adAnaKopya.TR, v)
}

func iyilestiriciParametreleri(fn string, args []yurut.Value) ([]*tensor.Tensor, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, %s: ...) 1 konumlu parametre bekler, %d verildi", fn, adOgrenmeOrani.TR, len(args))
	}
	return parametreler(fn, args[0])
}

func adamWKur(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T400) // AdamW
	if err := baglama.AdliDenetle(fn, named, adOgrenmeOrani, adAgirlikCurumesi, adBeta1, adBeta2, adEpsilon, adAnaKopya); err != nil {
		return nil, err
	}
	params, err := iyilestiriciParametreleri(fn, args)
	if err != nil {
		return nil, err
	}
	lr, err := adliOran(fn, named, 0.001, 0, adOgrenmeOrani)
	if err != nil {
		return nil, err
	}
	wd, err := adliOran(fn, named, 0.01, 0, adAgirlikCurumesi)
	if err != nil {
		return nil, err
	}
	b1, err := adliOran(fn, named, 0.9, 0, adBeta1)
	if err != nil {
		return nil, err
	}
	b2, err := adliOran(fn, named, 0.999, 0, adBeta2)
	if err != nil {
		return nil, err
	}
	if b1 >= 1 || b2 >= 1 {
		return nil, fmt.Errorf("%s: beta1 ve beta2 [0, 1) aralığında olmalıdır", fn)
	}
	eps, err := adliOran(fn, named, 1e-8, 0, adEpsilon)
	if err != nil {
		return nil, err
	}
	anaKopya, err := anaKopyaArg(fn, named)
	if err != nil {
		return nil, err
	}
	a, err := guvenli(func() *tensor.AdamW { return tensor.NewAdamW(params, lr, wd) })
	if err != nil {
		return nil, err
	}
	a.Beta1, a.Beta2, a.Eps = b1, b2, eps
	a.SetMasterWeights(anaKopya)
	return &Iyilestirici{ad: fn, opt: a, adamw: a, params: params}, nil
}

func sgdKur(_ *yurut.VM, args []yurut.Value, named map[string]yurut.Value) (yurut.Value, error) {
	fn := kod.Ad(kod.T401) // SGD
	if err := baglama.AdliDenetle(fn, named, adOgrenmeOrani, adMomentum, adAgirlikCurumesi, adAnaKopya); err != nil {
		return nil, err
	}
	params, err := iyilestiriciParametreleri(fn, args)
	if err != nil {
		return nil, err
	}
	lr, err := adliOran(fn, named, 0.01, 0, adOgrenmeOrani)
	if err != nil {
		return nil, err
	}
	mom, err := adliOran(fn, named, 0, 0, adMomentum)
	if err != nil {
		return nil, err
	}
	wd, err := adliOran(fn, named, 0, 0, adAgirlikCurumesi)
	if err != nil {
		return nil, err
	}
	anaKopya, err := anaKopyaArg(fn, named)
	if err != nil {
		return nil, err
	}
	s, err := guvenli(func() *tensor.SGD { return tensor.NewSGD(params, lr, mom) })
	if err != nil {
		return nil, err
	}
	s.WeightDecay = wd
	s.SetMasterWeights(anaKopya)
	return &Iyilestirici{ad: fn, opt: s, sgd: s, params: params}, nil
}

package tensor

import (
	"fmt"
	"math"
	"strconv"
	"tr_dili/pkg/kod"

	"tr_dili/pkg/motor/internal/baglama"
	ct "tr_dili/pkg/tensor"
	"tr_dili/pkg/turkce"
	"tr_dili/pkg/vm"
)

// Iyilestirici AdamW ve SGD iyileştiricilerinin TR değeridir. Metotları .adım()
// (parametreleri günceller) ve .sıfırla() (gradyanları sıfırlar).
type Iyilestirici struct {
	ad     string
	opt    ct.Optimizer
	adamw  *ct.AdamW
	sgd    *ct.SGD
	params []*ct.Tensor
}

func (o *Iyilestirici) Type() vm.ValueType { return vm.ValueType(o.ad) }
func (o *Iyilestirici) IsTruthy() bool     { return true }

func (o *Iyilestirici) Inspect() string {
	f := func(x float32) string { return strconv.FormatFloat(float64(x), 'g', -1, 32) }
	if o.adamw != nil {
		a := o.adamw
		return fmt.Sprintf("AdamW(parametre: %d, öğrenme_oranı: %s, ağırlık_çürümesi: %s, beta1: %s, beta2: %s, epsilon: %s, adım: %d)",
			len(o.params), f(a.LR), f(a.WeightDecay), f(a.Beta1), f(a.Beta2), f(a.Eps), a.StepCount())
	}
	s := o.sgd
	return fmt.Sprintf("SGD(parametre: %d, öğrenme_oranı: %s, momentum: %s, ağırlık_çürümesi: %s)",
		len(o.params), f(s.LR), f(s.Momentum), f(s.WeightDecay))
}

// İyileştirici üyeleri (adları kod listesinde, pkg/kod Y50x).
const (
	uyeAdim    = kod.Y500
	uyeSifirla = kod.Y501
)

var iyilestiriciUyeleri = kod.Adlar(uyeAdim, uyeSifirla)

func (o *Iyilestirici) GetProperty(kd kod.Kod, prop string) (vm.Value, error) {
	switch kd {
	case uyeAdim:
		return &vm.BuiltinVal{Name: kod.Ad(uyeAdim), Fn: func(_ *vm.VM, args []vm.Value, _ map[string]vm.Value) (vm.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("adım() argüman almaz")
			}
			if _, err := guvenli(func() bool { o.opt.Step(); return true }); err != nil {
				return nil, err
			}
			return vm.NULL, nil
		}}, nil
	case uyeSifirla:
		return &vm.BuiltinVal{Name: kod.Ad(uyeSifirla), Fn: func(_ *vm.VM, args []vm.Value, _ map[string]vm.Value) (vm.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("sıfırla() argüman almaz")
			}
			o.opt.ZeroGrad()
			return vm.NULL, nil
		}}, nil
	}
	oneri := ""
	if s := turkce.EnYakin(prop, iyilestiriciUyeleri); s != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", s)
	}
	return nil, fmt.Errorf("%s üzerinde %q metodu bulunamadı%s", o.ad, prop, oneri)
}

// parametreler iyileştiriciye verilen tensör ya da tensör listesini denetler.
func parametreler(fn string, v vm.Value) ([]*ct.Tensor, error) {
	var liste []vm.Value
	switch x := v.(type) {
	case *Deger:
		liste = []vm.Value{x}
	case *vm.ListVal:
		liste = x.Elements
	default:
		return nil, fmt.Errorf("%s: parametreler bir tensör listesi olmalıdır, %s verildi", fn, tipAdi(v))
	}
	if len(liste) == 0 {
		return nil, fmt.Errorf("%s: parametre listesi boş", fn)
	}
	out := make([]*ct.Tensor, 0, len(liste))
	gorulen := map[*ct.Tensor]bool{}
	for i, el := range liste {
		d, ok := el.(*Deger)
		if !ok {
			return nil, fmt.Errorf("%s: %d. parametre bir tensör olmalıdır, %s verildi", fn, i+1, tipAdi(el))
		}
		t := d.t
		if !t.RequiresGrad() {
			return nil, fmt.Errorf("%s: %d. parametre (%s) türev izlemiyor; tensör(..., türev_izle: doğru) ile oluşturun", fn, i+1, sekilMetni(t.Shape()))
		}
		if d.ara || !t.IsLeaf() {
			return nil, fmt.Errorf("%s: %d. parametre (%s) bir işlemin sonucu (yaprak değil); parametreleri doğrudan tensör(..., türev_izle: doğru) ile oluşturun", fn, i+1, sekilMetni(t.Shape()))
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
func adliOran(fn string, named map[string]vm.Value, varsayilan float64, enAz float64, ad baglama.Ad) (float32, error) {
	v, ok := baglama.AdliAl(named, ad)
	if !ok {
		return float32(varsayilan), nil
	}
	f, ok := vm.ToFloat(v)
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
func gradyanKirp(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	const fn = "gradyan_kırp"
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, en_fazla: 1.0) 1 konumlu parametre bekler, %d verildi", fn, len(args))
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
		f, ok := vm.ToFloat(v)
		if !ok || !(f > 0) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("%s: en_fazla pozitif bir sayı olmalıdır, %s verildi", fn, v.Inspect())
		}
		enFazla = f
	}
	norm, err := guvenli(func() float64 { return ct.ClipGradNorm(params, enFazla) })
	if err != nil {
		return nil, err
	}
	// Gradyanlar float32 olduğundan norm da float32 duyarlığında, en kısa gösterimle verilir.
	return &vm.FloatVal{Val: kisaFloat64(float32(norm))}, nil
}

// anaKopyaArg ana_kopya: doğru adlı argümanını okur (varsayılan doğru). Açıkken 16 bitlik
// (bf16/fp16) parametrelerin float32 ana kopyası tutulur: güncelleme ana kopyada yapılıp
// parametreye yuvarlanarak yazılır, böylece küçük adımlar 16 bitte kaybolmaz. float32
// parametreleri etkilemez.
func anaKopyaArg(fn string, named map[string]vm.Value) (bool, error) {
	v, ok := baglama.AdliAl(named, adAnaKopya)
	if !ok {
		return true, nil
	}
	return baglama.Mantik(fn, adAnaKopya.TR, v)
}

func iyilestiriciParametreleri(fn string, args []vm.Value) ([]*ct.Tensor, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, öğrenme_oranı: ...) 1 konumlu parametre bekler, %d verildi", fn, len(args))
	}
	return parametreler(fn, args[0])
}

func adamWKur(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	fn := kod.Ad(kod.T400) // AdamW (tip adı da budur)
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
	a, err := guvenli(func() *ct.AdamW { return ct.NewAdamW(params, lr, wd) })
	if err != nil {
		return nil, err
	}
	a.Beta1, a.Beta2, a.Eps = b1, b2, eps
	a.SetMasterWeights(anaKopya)
	return &Iyilestirici{ad: fn, opt: a, adamw: a, params: params}, nil
}

func sgdKur(_ *vm.VM, args []vm.Value, named map[string]vm.Value) (vm.Value, error) {
	fn := kod.Ad(kod.T401) // SGD (tip adı da budur)
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
	s, err := guvenli(func() *ct.SGD { return ct.NewSGD(params, lr, mom) })
	if err != nil {
		return nil, err
	}
	s.WeightDecay = wd
	s.SetMasterWeights(anaKopya)
	return &Iyilestirici{ad: fn, opt: s, sgd: s, params: params}, nil
}

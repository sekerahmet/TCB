package ml

import (
	"fmt"
	"github.com/sekerahmet/TCB/internal/lang/codes"
	"math"
	"strconv"

	"github.com/sekerahmet/TCB/internal/compute/tensor"
	"github.com/sekerahmet/TCB/internal/lang/interp"
	"github.com/sekerahmet/TCB/internal/lang/turkish"
	"github.com/sekerahmet/TCB/internal/stdlib/common/bind"
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

func (o *Iyilestirici) Type() interp.ValueType { return iyilestiriciTipi }
func (o *Iyilestirici) IsTruthy() bool         { return true }

func (o *Iyilestirici) Inspect() string {
	f := func(x float32) string { return strconv.FormatFloat(float64(x), 'g', -1, 32) }
	if o.adamw != nil {
		a := o.adamw
		return fmt.Sprintf("%s(parametre: %d, %s: %s, %s: %s, %s: %s, %s: %s, %s: %s, %s: %d)",
			codes.Name(codes.T400), len(o.params), adOgrenmeOrani.TR, f(a.LR), adAgirlikCurumesi.TR, f(a.WeightDecay),
			adBeta1.TR, f(a.Beta1), adBeta2.TR, f(a.Beta2), adEpsilon.TR, f(a.Eps), codes.Name(uyeAdim), a.StepCount())
	}
	s := o.sgd
	return fmt.Sprintf("%s(parametre: %d, %s: %s, %s: %s, %s: %s)",
		codes.Name(codes.T401), len(o.params), adOgrenmeOrani.TR, f(s.LR), adMomentum.TR, f(s.Momentum), adAgirlikCurumesi.TR, f(s.WeightDecay))
}

// İyileştirici üyeleri (adları kod listesinde, internal/lang/codes Y50x).
const (
	uyeAdim         = codes.Y500
	uyeSifirla      = codes.Y501
	uyeOgrenmeOrani = codes.Y504
)

var iyilestiriciUyeleri = codes.Names(uyeAdim, uyeSifirla, uyeDurum, uyeDurumuYukle, uyeOgrenmeOrani)

// oranAlani iyileştiricinin öğrenme oranı alanıdır (AdamW ya da SGD); arka uç onu her adımda
// okur, yeni oran sonraki adımda kullanılır.
func (o *Iyilestirici) oranAlani() *float32 {
	if o.adamw != nil {
		return &o.adamw.LR
	}
	return &o.sgd.LR
}

// UyeyeAta op.öğrenme_oranı = değer atamasıdır (yurut.UyeAtanabilir): yalnızca sonlu, negatif olmayan
// ve 32 bite sığan bir Tam ya da Ondalık kabul edilir.
func (o *Iyilestirici) SetMember(kd codes.Code, ad string, deger interp.Value) error {
	if kd != uyeOgrenmeOrani {
		return fmt.Errorf("%s üzerinde %q üyesine atama yapılamaz; atanabilen üye: %s", o.ad, ad, codes.Name(uyeOgrenmeOrani))
	}
	var f float64
	switch x := deger.(type) {
	case *interp.IntVal:
		f = float64(x.Val)
	case *interp.FloatVal:
		f = x.Val
	default:
		return fmt.Errorf("%s.%s bir %s ya da %s olmalıdır, %s verildi", o.ad, codes.Name(uyeOgrenmeOrani), codes.Name(codes.A200), codes.Name(codes.A201), tipAdi(deger))
	}
	// 0 geçerlidir (kurucudaki gibi): oran planları eğitimin sonunda orana 0'a iner. Sıfır olmayan
	// ama 32 bitte 0'a yuvarlanan değer reddedilir (sessizce dondurmasın).
	if !(f >= 0) || math.IsInf(float64(float32(f)), 0) || (f != 0 && float32(f) == 0) {
		return fmt.Errorf("%s.%s sonlu ve negatif olmayan bir sayı olmalıdır, %s verildi", o.ad, codes.Name(uyeOgrenmeOrani), deger.Inspect())
	}
	*o.oranAlani() = float32(f)
	return nil
}

// iyilestiriciTipi AdamW ve SGD değerlerinin tip adıdır (tip(op) → "İyileştirici").
var iyilestiriciTipi = interp.ValueType(interp.TypeName(codes.A214))

func (o *Iyilestirici) GetProperty(kd codes.Code, prop string) (interp.Value, error) {
	switch kd {
	case uyeAdim:
		return &interp.BuiltinVal{Name: codes.Name(uyeAdim), NoNamedArgs: true, Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", codes.Name(uyeAdim))
			}
			if _, err := guvenli(func() bool { o.opt.Step(); return true }); err != nil {
				return nil, err
			}
			return interp.NULL, nil
		}}, nil
	case uyeSifirla:
		return &interp.BuiltinVal{Name: codes.Name(uyeSifirla), NoNamedArgs: true, Fn: func(_ *interp.VM, args []interp.Value, _ map[string]interp.Value) (interp.Value, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%s() argüman almaz", codes.Name(uyeSifirla))
			}
			o.opt.ZeroGrad()
			return interp.NULL, nil
		}}, nil
	case uyeOgrenmeOrani:
		// Oran 32 bit saklanır; en kısa gösterimiyle verilir (0.003 → 0.003).
		return &interp.FloatVal{Val: kisaFloat64(*o.oranAlani())}, nil
	}
	if v, ok, err := o.durumUyesi(kd); ok {
		return v, err // durum, durumu_yükle (iyilestirici_durum.go)
	}
	oneri := ""
	if s := turkish.Closest(prop, iyilestiriciUyeleri); s != "" {
		oneri = fmt.Sprintf(". Bunu mu demek istediniz: '%s'?", s)
	}
	return nil, fmt.Errorf("%s üzerinde %q metodu bulunamadı%s", o.ad, prop, oneri)
}

// parametreler iyileştiriciye verilen tensör ya da tensör listesini denetler.
func parametreler(fn string, v interp.Value) ([]*tensor.Tensor, error) {
	var liste []interp.Value
	switch x := v.(type) {
	case *Deger:
		liste = []interp.Value{x}
	case *interp.ListVal:
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
func adliOran(fn string, named map[string]interp.Value, varsayilan float64, enAz float64, ad bind.Ad) (float32, error) {
	v, ok := bind.AdliAl(named, ad)
	if !ok {
		return float32(varsayilan), nil
	}
	f, ok := interp.ToFloat(v)
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
// kırpma öncesi normu 0 boyutlu tensör olarak döndürür: gradyan_kırp(parametreler, en_fazla: 1.0).
func gradyanKirp(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T402)
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, %s: 1.0) 1 konumlu parametre bekler, %d verildi", fn, adEnFazla.TR, len(args))
	}
	if err := bind.AdliDenetle(fn+"()", named, adEnFazla); err != nil {
		return nil, err
	}
	params, err := parametreler(fn, args[0])
	if err != nil {
		return nil, err
	}
	enFazla := 1.0
	if v, ok := bind.AdliAl(named, adEnFazla); ok {
		f, ok := interp.ToFloat(v)
		if !ok || !(f > 0) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("%s: %s pozitif bir sayı olmalıdır, %s verildi", fn, adEnFazla.TR, v.Inspect())
		}
		enFazla = f
	}
	// Norm 0 boyutlu tensördür (kayıp gibi): GPU'da norm ve kırpma cihazda yapılır, program
	// ancak norm okununca (.değer, yazdırma) bekler.
	norm, err := guvenli(func() *tensor.Tensor { return tensor.ClipGradNormTensor(params, enFazla) })
	if err != nil {
		return nil, err
	}
	return Yeni(norm), nil
}

// anaKopyaArg ana_kopya: doğru adlı argümanını okur (varsayılan doğru). Açıkken 16 bitlik
// (bf16/fp16) parametrelerin float32 ana kopyası tutulur: güncelleme ana kopyada yapılıp
// parametreye yuvarlanarak yazılır, böylece küçük adımlar 16 bitte kaybolmaz. float32
// parametreleri etkilemez.
func anaKopyaArg(fn string, named map[string]interp.Value) (bool, error) {
	v, ok := bind.AdliAl(named, adAnaKopya)
	if !ok {
		return true, nil
	}
	return bind.Mantik(fn, adAnaKopya.TR, v)
}

func iyilestiriciParametreleri(fn string, args []interp.Value) ([]*tensor.Tensor, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s(parametreler, %s: ...) 1 konumlu parametre bekler, %d verildi", fn, adOgrenmeOrani.TR, len(args))
	}
	return parametreler(fn, args[0])
}

func adamWKur(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T400) // AdamW
	if err := bind.AdliDenetle(fn, named, adOgrenmeOrani, adAgirlikCurumesi, adBeta1, adBeta2, adEpsilon, adAnaKopya); err != nil {
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

func sgdKur(_ *interp.VM, args []interp.Value, named map[string]interp.Value) (interp.Value, error) {
	fn := codes.Name(codes.T401) // SGD
	if err := bind.AdliDenetle(fn, named, adOgrenmeOrani, adMomentum, adAgirlikCurumesi, adAnaKopya); err != nil {
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

// Compile-time checks: the interpreter finds these hooks with type assertions, so a
// renamed or changed method would otherwise drop the hook silently.
var (
	_ interp.PropertyGetter   = (*Iyilestirici)(nil)
	_ interp.MemberAssignable = (*Iyilestirici)(nil)
)
